package store

import (
	"context"
	"sort"
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

type memoryUser struct {
	user      User
	tokenHash string
}

// Memory is an in-process Store used when AGENTOS_DATABASE_URL is empty.
type Memory struct {
	mu    sync.Mutex
	keys  map[string]*memoryKey  // secret hash -> key
	usage map[string]*KeyUsage   // key name -> aggregate
	audit []AuditEntry           // oldest first
	orgs  map[string]*Org        // org id -> org
	users map[string]*memoryUser // user id -> user (with token hash)
}

// NewMemory returns an empty in-memory store.
func NewMemory() *Memory {
	return &Memory{
		keys:  make(map[string]*memoryKey),
		usage: make(map[string]*KeyUsage),
		orgs:  make(map[string]*Org),
		users: make(map[string]*memoryUser),
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
	k, ok := m.keys[hashSecret(secret)]
	if !ok {
		return nil, ErrInvalidKey
	}
	return &Key{Name: k.name, MonthlyBudgetUSD: k.budgetUSD, SpendUSD: k.spendUSD, OrgID: orgOrDefault(k.orgID)}, nil
}

func (m *Memory) RecordUsage(_ context.Context, u Usage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range m.keys {
		if k.name == u.KeyName {
			k.spendUSD += u.CostUSD
		}
	}
	agg, ok := m.usage[u.KeyName]
	if !ok {
		agg = &KeyUsage{Name: u.KeyName}
		m.usage[u.KeyName] = agg
	}
	agg.Requests++
	agg.InputTokens += u.InputTokens
	agg.OutputTokens += u.OutputTokens
	agg.SpendUSD += u.CostUSD
	m.appendAudit(u)
	return nil
}

func (m *Memory) RecordAudit(_ context.Context, u Usage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.appendAudit(u)
	return nil
}

// appendAudit stamps and stores one audit entry; callers hold m.mu.
func (m *Memory) appendAudit(u Usage) {
	m.audit = append(m.audit, AuditEntry{
		TS:           time.Now().UTC(),
		KeyName:      u.KeyName,
		Model:        u.Model,
		InputTokens:  u.InputTokens,
		OutputTokens: u.OutputTokens,
		CostUSD:      u.CostUSD,
		LatencyMS:    u.LatencyMS,
		Status:       u.Status,
		Kind:         kindOrChat(u.Kind),
	})
}

func (m *Memory) AuditList(_ context.Context, limit int) ([]AuditEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit > len(m.audit) {
		limit = len(m.audit)
	}
	if limit < 0 {
		limit = 0
	}
	out := make([]AuditEntry, 0, limit)
	for i := len(m.audit) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, m.audit[i])
	}
	return out, nil
}

func (m *Memory) Usage(_ context.Context) ([]KeyUsage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := make(map[string]bool)
	var out []KeyUsage
	for name, agg := range m.usage {
		seen[name] = true
		out = append(out, *agg)
	}
	for _, k := range m.keys {
		if !seen[k.name] {
			seen[k.name] = true
			out = append(out, KeyUsage{Name: k.name})
		}
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

func (m *Memory) CreateUser(_ context.Context, orgID, email, role string) (*User, string, error) {
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
	u := User{ID: id, OrgID: orgID, Email: email, Role: role, CreatedAt: time.Now().UTC()}
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
			cp := mu.user
			return &cp, nil
		}
	}
	return nil, ErrInvalidToken
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
	return nil
}

// Audit returns a copy of the recorded audit entries, oldest first
// (memory store only, used by tests).
func (m *Memory) Audit() []Usage {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Usage, len(m.audit))
	for i, e := range m.audit {
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
