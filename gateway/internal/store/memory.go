package store

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type memoryKey struct {
	name      string
	budgetUSD float64
	spendUSD  float64
	orgID     string
	createdBy string
}

// memoryReservation is cost admitted but not yet settled — one request in
// flight. Budget decisions count these alongside recorded spend so concurrent
// requests cannot each be admitted against the same pre-spend snapshot.
type memoryReservation struct {
	secretHash  string
	estimateUSD float64
	at          time.Time
}

type memoryUser struct {
	user      User
	tokenHash string
}

// memoryUsage is one per-key aggregate plus the owning org, keyed internally by
// the key's stable identity (secret_hash) so same-named keys in different orgs
// never share a row (H4).
type memoryUsage struct {
	agg   KeyUsage
	orgID string
}

// memoryAudit is one audit row plus the owning org (used only for org scoping;
// org_id is never serialized in the JSON response).
type memoryAudit struct {
	entry AuditEntry
	orgID string
}

// Memory is an in-process Store used when AGENTOS_DATABASE_URL is empty.
type Memory struct {
	mu    sync.Mutex
	keys  map[string]*memoryKey   // secret hash -> key
	usage map[string]*memoryUsage // identity (secret_hash | "name:"+name) -> aggregate
	audit []memoryAudit           // oldest first
	orgs  map[string]*Org         // org id -> org
	users map[string]*memoryUser  // user id -> user (with token hash)
	groups  map[string]*Group              // group id -> group
	members map[string]map[string]struct{} // group id -> set of user ids
	// In-flight spend reservations, keyed by the handle handed to the caller.
	reservations   map[string]*memoryReservation
	reservationSeq int64
}

// NewMemory returns an empty in-memory store.
func NewMemory() *Memory {
	return &Memory{
		keys:  make(map[string]*memoryKey),
		usage: make(map[string]*memoryUsage),
		orgs:  make(map[string]*Org),
		users: make(map[string]*memoryUser),
		groups:  make(map[string]*Group),
		members: make(map[string]map[string]struct{}),

		reservations: make(map[string]*memoryReservation),
	}
}

func (m *Memory) CreateKey(ctx context.Context, name string, budgetUSD float64) (string, error) {
	return m.CreateKeyIn(ctx, name, budgetUSD, DefaultOrgID, RootCreator)
}

func (m *Memory) CreateKeyIn(_ context.Context, name string, budgetUSD float64, orgID, createdBy string) (string, error) {
	secret, err := newSecret()
	if err != nil {
		return "", err
	}
	if orgID == "" {
		orgID = DefaultOrgID
	}
	if createdBy == "" {
		createdBy = RootCreator
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keys[hashSecret(secret)] = &memoryKey{name: name, budgetUSD: budgetUSD, orgID: orgID, createdBy: createdBy}
	return secret, nil
}

func (m *Memory) Authenticate(_ context.Context, secret string) (*Key, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hash := hashSecret(secret)
	k, ok := m.keys[hash]
	if !ok {
		return nil, ErrInvalidKey
	}
	return &Key{Name: k.name, MonthlyBudgetUSD: k.budgetUSD, SpendUSD: k.spendUSD, OrgID: orgOrDefault(k.orgID), SecretHash: hash}, nil
}

// resolveIdentity determines the stable identity (secret_hash) and org for a
// usage/audit row. The caller-supplied values win; missing values are resolved
// from the owning key by name (legacy path). Callers hold m.mu.
func (m *Memory) resolveIdentity(u Usage) (identity, orgID string) {
	identity, orgID = u.SecretHash, u.OrgID
	if identity == "" || orgID == "" {
		for h, k := range m.keys {
			if k.name == u.KeyName {
				if identity == "" {
					identity = h
				}
				if orgID == "" {
					orgID = orgOrDefault(k.orgID)
				}
				break
			}
		}
	}
	if identity == "" {
		identity = "name:" + u.KeyName
	}
	return identity, orgID
}

// reservedLocked totals live reservations for one key. Callers must hold m.mu.
// Expired entries are ignored rather than deleted so this stays a pure read;
// reaping happens in ReserveSpend.
func (m *Memory) reservedLocked(secretHash string, now time.Time) float64 {
	var total float64
	for _, r := range m.reservations {
		if r.secretHash == secretHash && now.Sub(r.at) < ReservationTTL {
			total += r.estimateUSD
		}
	}
	return total
}

// ReserveSpend admits a request only if the key and its org both have room,
// counting cost already in flight. The single store mutex makes the check and
// the reservation one atomic step, which is the whole point.
func (m *Memory) ReserveSpend(_ context.Context, secretHash string, estimateUSD float64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	k, ok := m.keys[secretHash]
	if !ok {
		return "", ErrInvalidKey
	}
	now := time.Now()
	// Drop anything orphaned by a crashed process before deciding.
	for id, r := range m.reservations {
		if now.Sub(r.at) >= ReservationTTL {
			delete(m.reservations, id)
		}
	}

	// A budget of 0 means unlimited, matching the rest of the gateway.
	if k.budgetUSD > 0 && k.spendUSD+m.reservedLocked(secretHash, now)+estimateUSD > k.budgetUSD {
		return "", ErrBudgetExceeded
	}

	orgID := orgOrDefault(k.orgID)
	if org, ok := m.orgs[orgID]; ok && org.MonthlyBudgetUSD > 0 {
		var committed float64
		for hash, other := range m.keys {
			if orgOrDefault(other.orgID) == orgID {
				committed += other.spendUSD + m.reservedLocked(hash, now)
			}
		}
		if committed+estimateUSD > org.MonthlyBudgetUSD {
			return "", ErrOrgBudgetExceeded
		}
	}

	m.reservationSeq++
	id := strconv.FormatInt(m.reservationSeq, 10)
	m.reservations[id] = &memoryReservation{
		secretHash:  secretHash,
		estimateUSD: estimateUSD,
		at:          now,
	}
	return id, nil
}

// ReleaseSpend drops a reservation by handle. Releasing an unknown or already
// released handle is a no-op, so a double release cannot manufacture headroom.
func (m *Memory) ReleaseSpend(_ context.Context, reservationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.reservations, reservationID)
	return nil
}

func (m *Memory) RecordUsage(_ context.Context, u Usage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	identity, orgID := m.resolveIdentity(u)
	// Attribute spend by secret_hash when the caller provided one (isolated),
	// else by name for legacy callers.
	if u.SecretHash != "" {
		if k, ok := m.keys[u.SecretHash]; ok {
			k.spendUSD += u.CostUSD
		}
	} else {
		for _, k := range m.keys {
			if k.name == u.KeyName {
				k.spendUSD += u.CostUSD
			}
		}
	}
	mu, ok := m.usage[identity]
	if !ok {
		mu = &memoryUsage{agg: KeyUsage{Name: u.KeyName}, orgID: orgID}
		m.usage[identity] = mu
	}
	if orgID != "" {
		mu.orgID = orgID
	}
	mu.agg.Name = u.KeyName
	mu.agg.Requests++
	mu.agg.InputTokens += u.InputTokens
	mu.agg.OutputTokens += u.OutputTokens
	mu.agg.SpendUSD += u.CostUSD
	m.appendAudit(u, orgID)
	return nil
}

// PruneAudit drops audit entries older than the cutoff.
func (m *Memory) PruneAudit(_ context.Context, before time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	kept := m.audit[:0]
	var removed int64
	for _, a := range m.audit {
		if a.entry.TS.Before(before) {
			removed++
			continue
		}
		kept = append(kept, a)
	}
	m.audit = kept
	return removed, nil
}

func (m *Memory) RecordAudit(_ context.Context, u Usage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, orgID := m.resolveIdentity(u)
	m.appendAudit(u, orgID)
	return nil
}

// appendAudit stamps and stores one audit entry; callers hold m.mu.
func (m *Memory) appendAudit(u Usage, orgID string) {
	m.audit = append(m.audit, memoryAudit{
		entry: AuditEntry{
			TS:           time.Now().UTC(),
			KeyName:      u.KeyName,
			Model:        u.Model,
			InputTokens:  u.InputTokens,
			OutputTokens: u.OutputTokens,
			CostUSD:      u.CostUSD,
			LatencyMS:    u.LatencyMS,
			Status:       u.Status,
			Kind:         kindOrChat(u.Kind),
		},
		orgID: orgID,
	})
}

func (m *Memory) AuditList(_ context.Context, orgID string, limit int) ([]AuditEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit < 0 {
		limit = 0
	}
	out := make([]AuditEntry, 0, limit)
	// Newest first; an empty orgID returns all rows (root), else scope by org.
	for i := len(m.audit) - 1; i >= 0 && len(out) < limit; i-- {
		if orgID != "" && m.audit[i].orgID != orgID {
			continue
		}
		out = append(out, m.audit[i].entry)
	}
	return out, nil
}

func (m *Memory) Usage(_ context.Context, orgID string) ([]KeyUsage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []KeyUsage
	// Aggregates that have recorded usage, scoped by org (empty = all).
	for _, mu := range m.usage {
		if orgID != "" && mu.orgID != orgID {
			continue
		}
		out = append(out, mu.agg)
	}
	// Keys with no usage yet still appear (with zero counts), scoped by org.
	for h, k := range m.keys {
		if orgID != "" && orgOrDefault(k.orgID) != orgID {
			continue
		}
		if _, ok := m.usage[h]; ok {
			continue
		}
		if _, ok := m.usage["name:"+k.name]; ok {
			continue
		}
		out = append(out, KeyUsage{Name: k.name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *Memory) Keys(_ context.Context) ([]KeyInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]KeyInfo, 0, len(m.keys))
	for _, k := range m.keys {
		out = append(out, KeyInfo{Name: k.name, MonthlyBudgetUSD: k.budgetUSD, SpendUSD: k.spendUSD, OrgID: orgOrDefault(k.orgID)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *Memory) EnsureKey(_ context.Context, name, secret string, budgetUSD float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	h := hashSecret(secret)
	if k, ok := m.keys[h]; ok {
		k.name = name
		k.budgetUSD = budgetUSD
		if k.orgID == "" {
			k.orgID = DefaultOrgID
		}
		return nil // spend preserved
	}
	m.keys[h] = &memoryKey{name: name, budgetUSD: budgetUSD, orgID: DefaultOrgID, createdBy: RootCreator}
	return nil
}

// orgOrDefault maps an empty org id to the bootstrapped default org.
func orgOrDefault(orgID string) string {
	if orgID == "" {
		return DefaultOrgID
	}
	return orgID
}

func (m *Memory) CreateOrg(_ context.Context, name string, monthlyBudgetUSD float64) (*Org, error) {
	id, err := newID("org_")
	if err != nil {
		return nil, err
	}
	org := &Org{ID: id, Name: name, MonthlyBudgetUSD: monthlyBudgetUSD, CreatedAt: time.Now().UTC()}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orgs[id] = org
	cp := *org
	return &cp, nil
}

func (m *Memory) EnsureOrg(_ context.Context, id, name string, monthlyBudgetUSD float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if o, ok := m.orgs[id]; ok {
		o.Name = name
		o.MonthlyBudgetUSD = monthlyBudgetUSD
		return nil
	}
	m.orgs[id] = &Org{ID: id, Name: name, MonthlyBudgetUSD: monthlyBudgetUSD, CreatedAt: time.Now().UTC()}
	return nil
}

func (m *Memory) Org(_ context.Context, id string) (*Org, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orgs[id]
	if !ok {
		return nil, ErrOrgNotFound
	}
	cp := *o
	return &cp, nil
}

func (m *Memory) Orgs(_ context.Context) ([]Org, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Org, 0, len(m.orgs))
	for _, o := range m.orgs {
		out = append(out, *o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *Memory) UpdateOrg(_ context.Context, id string, monthlyBudgetUSD *float64, rateLimitRPM *int) (*Org, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orgs[id]
	if !ok {
		return nil, ErrOrgNotFound
	}
	if monthlyBudgetUSD != nil {
		o.MonthlyBudgetUSD = *monthlyBudgetUSD
	}
	if rateLimitRPM != nil {
		o.RateLimitRPM = *rateLimitRPM
	}
	cp := *o
	return &cp, nil
}

func (m *Memory) OrgSpend(_ context.Context, orgID string) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var total float64
	for _, k := range m.keys {
		if orgOrDefault(k.orgID) == orgID {
			total += k.spendUSD
		}
	}
	return total, nil
}

// OrgSpends aggregates every org's key spend in one pass.
func (m *Memory) OrgSpends(_ context.Context) (map[string]float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]float64)
	for _, k := range m.keys {
		out[orgOrDefault(k.orgID)] += k.spendUSD
	}
	return out, nil
}

func (m *Memory) CreateUser(ctx context.Context, orgID, email, role string) (*User, string, error) {
	return m.CreateUserWithExternalID(ctx, orgID, email, role, "")
}

func (m *Memory) CreateUserWithExternalID(_ context.Context, orgID, email, role, externalID string) (*User, string, error) {
	if err := validateRole(role); err != nil {
		return nil, "", err
	}
	id, err := newID("usr_")
	if err != nil {
		return nil, "", err
	}
	token, err := newUserToken()
	if err != nil {
		return nil, "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.orgs[orgID]; !ok {
		return nil, "", ErrOrgNotFound
	}
	u := User{ID: id, OrgID: orgID, Email: email, Role: role, Active: true, ExternalID: externalID, CreatedAt: time.Now().UTC()}
	m.users[id] = &memoryUser{user: u, tokenHash: hashSecret(token)}
	cp := u
	return &cp, token, nil
}

func (m *Memory) AuthenticateUser(_ context.Context, token string) (*User, error) {
	h := hashSecret(token)
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, mu := range m.users {
		if mu.tokenHash == h {
			if !mu.user.Active {
				return nil, ErrUserInactive
			}
			cp := mu.user
			return &cp, nil
		}
	}
	return nil, ErrInvalidToken
}

func (m *Memory) SetUserActive(_ context.Context, userID string, active bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mu, ok := m.users[userID]
	if !ok {
		return ErrUserNotFound
	}
	mu.user.Active = active
	return nil
}

func (m *Memory) SetUserRole(_ context.Context, userID, role string) error {
	if err := validateRole(role); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	mu, ok := m.users[userID]
	if !ok {
		return ErrUserNotFound
	}
	mu.user.Role = role
	return nil
}

func (m *Memory) SetUserExternalID(_ context.Context, userID, externalID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mu, ok := m.users[userID]
	if !ok {
		return ErrUserNotFound
	}
	mu.user.ExternalID = externalID
	return nil
}

func (m *Memory) UserByExternalID(_ context.Context, orgID, externalID string) (*User, error) {
	if externalID == "" {
		return nil, ErrUserNotFound
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, mu := range m.users {
		if mu.user.OrgID == orgID && mu.user.ExternalID == externalID {
			cp := mu.user
			return &cp, nil
		}
	}
	return nil, ErrUserNotFound
}

func (m *Memory) UserByEmail(_ context.Context, orgID, email string) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, mu := range m.users {
		if mu.user.OrgID == orgID && mu.user.Email == email {
			cp := mu.user
			return &cp, nil
		}
	}
	return nil, ErrUserNotFound
}

func (m *Memory) IssueUserToken(_ context.Context, userID string) (string, error) {
	token, err := newUserToken()
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	mu, ok := m.users[userID]
	if !ok {
		return "", ErrUserNotFound
	}
	mu.tokenHash = hashSecret(token)
	return token, nil
}

func (m *Memory) Users(_ context.Context, orgID string) ([]User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]User, 0)
	for _, mu := range m.users {
		if mu.user.OrgID == orgID {
			out = append(out, mu.user)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *Memory) DeleteUser(_ context.Context, orgID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mu, ok := m.users[userID]
	if !ok || mu.user.OrgID != orgID {
		return ErrUserNotFound
	}
	delete(m.users, userID)
	// Membership must not outlive the user, or GroupMembers would name someone
	// who no longer exists. Postgres gets this from ON DELETE CASCADE; here it
	// is explicit, and TestGroupMembershipFollowsUserDeletion pins the parity.
	for _, set := range m.members {
		delete(set, userID)
	}
	return nil
}

// Audit returns a copy of the recorded audit entries, oldest first
// (memory store only, used by tests).
func (m *Memory) Audit() []Usage {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Usage, len(m.audit))
	for i, a := range m.audit {
		e := a.entry
		out[i] = Usage{
			KeyName:      e.KeyName,
			Model:        e.Model,
			InputTokens:  e.InputTokens,
			OutputTokens: e.OutputTokens,
			CostUSD:      e.CostUSD,
			LatencyMS:    e.LatencyMS,
			Status:       e.Status,
			Kind:         e.Kind,
		}
	}
	return out
}

// --- SCIM Groups (Phase 7) ---

// memberSet returns the membership set for a group, creating it on demand.
// Callers hold m.mu.
func (m *Memory) memberSet(groupID string) map[string]struct{} {
	set, ok := m.members[groupID]
	if !ok {
		set = make(map[string]struct{})
		m.members[groupID] = set
	}
	return set
}

func (m *Memory) CreateGroup(_ context.Context, orgID, displayName, externalID string) (*Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.orgs[orgID]; !ok {
		return nil, ErrOrgNotFound
	}
	// Case-insensitive, matching the Postgres unique index on lower(name):
	// identity providers do not guarantee casing between a create and a later
	// re-sync, and two groups differing only in case would each map to the same
	// role allowlist entry.
	for _, g := range m.groups {
		if g.OrgID == orgID && strings.EqualFold(g.DisplayName, displayName) {
			return nil, ErrGroupExists
		}
	}
	id, err := newID("grp_")
	if err != nil {
		return nil, err
	}
	g := &Group{ID: id, OrgID: orgID, DisplayName: displayName, ExternalID: externalID, CreatedAt: time.Now().UTC()}
	m.groups[id] = g
	cp := *g
	return &cp, nil
}

func (m *Memory) GroupByID(_ context.Context, orgID, groupID string) (*Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[groupID]
	if !ok || g.OrgID != orgID {
		return nil, ErrGroupNotFound
	}
	cp := *g
	return &cp, nil
}

func (m *Memory) GroupByDisplayName(_ context.Context, orgID, displayName string) (*Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, g := range m.groups {
		if g.OrgID == orgID && strings.EqualFold(g.DisplayName, displayName) {
			cp := *g
			return &cp, nil
		}
	}
	return nil, ErrGroupNotFound
}

func (m *Memory) Groups(_ context.Context, orgID string) ([]Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Group{}
	for _, g := range m.groups {
		if g.OrgID == orgID {
			out = append(out, *g)
		}
	}
	// Map iteration is randomised, so without this the list order differs run to
	// run and Postgres (ORDER BY id) would disagree with Memory.
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *Memory) RenameGroup(_ context.Context, orgID, groupID, displayName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[groupID]
	if !ok || g.OrgID != orgID {
		return ErrGroupNotFound
	}
	for _, other := range m.groups {
		if other.ID != groupID && other.OrgID == orgID && strings.EqualFold(other.DisplayName, displayName) {
			return ErrGroupExists
		}
	}
	g.DisplayName = displayName
	return nil
}

func (m *Memory) DeleteGroup(_ context.Context, orgID, groupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[groupID]
	if !ok || g.OrgID != orgID {
		return ErrGroupNotFound
	}
	delete(m.groups, groupID)
	delete(m.members, groupID)
	return nil
}

func (m *Memory) SetGroupMembers(_ context.Context, orgID, groupID string, userIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[groupID]
	if !ok || g.OrgID != orgID {
		return ErrGroupNotFound
	}
	// Validate every id BEFORE writing any, so a rejected member cannot leave
	// the membership half-applied.
	for _, uid := range userIDs {
		mu, ok := m.users[uid]
		if !ok || mu.user.OrgID != orgID {
			return ErrUserNotFound
		}
	}
	set := make(map[string]struct{}, len(userIDs))
	for _, uid := range userIDs {
		set[uid] = struct{}{}
	}
	m.members[groupID] = set
	return nil
}

func (m *Memory) GroupMembers(_ context.Context, orgID, groupID string) ([]User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[groupID]
	if !ok || g.OrgID != orgID {
		return nil, ErrGroupNotFound
	}
	out := []User{}
	for uid := range m.memberSet(groupID) {
		if mu, ok := m.users[uid]; ok {
			out = append(out, mu.user)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *Memory) GroupsForUser(_ context.Context, orgID, userID string) ([]Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Group{}
	for gid, set := range m.members {
		if _, ok := set[userID]; !ok {
			continue
		}
		if g, ok := m.groups[gid]; ok && g.OrgID == orgID {
			out = append(out, *g)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
