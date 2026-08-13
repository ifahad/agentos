package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/ifahad/agentos/gateway/internal/rbac"
	"github.com/ifahad/agentos/gateway/internal/store"
)

// caller is the resolved identity behind an /admin/* request: either the root
// superuser (adminKey) or an authenticated org user.
type caller struct {
	root bool
	user *store.User // nil when root
}

// can reports whether the caller holds the capability. Root holds everything.
func (c *caller) can(action rbac.Action) bool {
	if c.root {
		return true
	}
	return rbac.Can(c.user.Role, action)
}

// adminAuth resolves the bearer token to a caller and invokes next. The token
// is the root admin key (superuser) or an agu- user token (role-checked);
// anything else is 401. This preserves Phase 1–4 behavior for the root key.
func (s *Server) adminAuth(next func(http.ResponseWriter, *http.Request, *caller)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, errInvalidKey, "invalid admin key")
			return
		}
		if secureCompare(token, s.adminKey) {
			next(w, r, &caller{root: true})
			return
		}
		if strings.HasPrefix(token, "agu-") {
			if u, err := s.store.AuthenticateUser(r.Context(), token); err == nil {
				next(w, r, &caller{user: u})
				return
			}
		}
		writeError(w, http.StatusUnauthorized, errInvalidKey, "invalid admin key")
	}
}

// writeForbidden emits the 403 forbidden contract shape.
func writeForbidden(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusForbidden, errForbidden, msg)
}

// denyOrgAction writes the 403 AND records it. A denial that leaves no trace is
// the gap between "nothing runs unauthorized" and being able to show it: the
// refusals are what an auditor asks to see, and they were the only outcome the
// gateway did not write down.
func (s *Server) denyOrgAction(w http.ResponseWriter, r *http.Request, c *caller, orgID, msg string) {
	u := store.Usage{OrgID: orgID, Status: http.StatusForbidden, Kind: store.KindDenied}
	if c.user != nil {
		// Attribute to the user that was refused. KeyName carries the actor
		// because a denial has no virtual key behind it.
		u.KeyName = c.user.Email
		if orgID == "" {
			u.OrgID = c.user.OrgID
		}
	}
	s.recordAudit(r, u)
	writeForbidden(w, msg)
}

// The org budget check used to live here as orgBudgetExceeded, costing a second
// Org lookup plus an OrgSpend aggregate on every proxied request — on top of the
// Org lookup the rate limiter already does. It is gone: ReserveSpend now applies
// the same cap inside the admission transaction, which is both cheaper and
// atomic. Do not reintroduce a pre-flight check here; a second, non-atomic
// opinion about the budget is exactly the race that was just closed.

// admitSpend reserves budget for one in-flight request against the key and its
// org, atomically. It writes the 402 and returns ok=false when there is no room.
//
// The returned release func MUST be deferred by the caller: it frees the hold
// once the request is done, whether it succeeded, failed, or panicked. Actual
// cost is recorded separately by recordAudit/RecordUsage — releasing only
// undoes the estimate, it does not un-bill anything.
func (s *Server) admitSpend(w http.ResponseWriter, r *http.Request, key *store.Key) (release func(), ok bool) {
	id, err := s.store.ReserveSpend(r.Context(), key.SecretHash, s.reserveUSD)
	switch {
	case errors.Is(err, store.ErrBudgetExceeded):
		s.recordAudit(r, store.Usage{
			SecretHash: key.SecretHash, OrgID: key.OrgID, KeyName: key.Name,
			Status: http.StatusPaymentRequired, Kind: store.KindBudgetExceeded,
		})
		writeError(w, http.StatusPaymentRequired, errBudgetExceeded,
			fmt.Sprintf("monthly budget of $%.2f exhausted for key %q", key.MonthlyBudgetUSD, key.Name))
		return nil, false
	case errors.Is(err, store.ErrOrgBudgetExceeded):
		s.recordAudit(r, store.Usage{
			SecretHash: key.SecretHash, OrgID: key.OrgID, KeyName: key.Name,
			Status: http.StatusPaymentRequired, Kind: store.KindBudgetExceeded,
		})
		writeError(w, http.StatusPaymentRequired, errOrgBudgetExceeded,
			fmt.Sprintf("org %q monthly budget exhausted", key.OrgID))
		return nil, false
	case err != nil:
		// Reservation is a guardrail, not the request's purpose. A store blip
		// must not take traffic down, so admit and rely on the post-hoc spend
		// record — the same fail-open stance the guardrail screener takes.
		//
		// And, like the guardrail, say so. The screener records its fail-open
		// as KindGuardrailError precisely so the blind spot is on the record;
		// this path claimed the same stance while writing nothing, which made
		// the one moment a budget was NOT enforced the one moment nothing was
		// written down.
		s.recordAudit(r, store.Usage{
			SecretHash: key.SecretHash, OrgID: key.OrgID, KeyName: key.Name,
			Status: http.StatusOK, Kind: store.KindBudgetError,
		})
		return func() {}, true
	}
	return func() {
		// Detached from the request context: the release must still run after a
		// client disconnect, or the hold survives until it expires.
		_ = s.store.ReleaseSpend(context.WithoutCancel(r.Context()), id)
	}, true
}

// scopeOrg returns the org id used to scope usage/audit store queries: empty
// for the root caller (sees all), else the user's org. The store pushes this
// down as a WHERE org_id filter, so isolation no longer depends on the mutable
// key name (H4).
func (s *Server) scopeOrg(c *caller) string {
	if c.root {
		return ""
	}
	return c.user.OrgID
}

// scopeKeys returns all keys for a root caller, or only the caller's org's keys
// for a user caller. Keys already carry an authoritative org_id (unlike the old
// name-keyed usage/audit), so this Go-side filter is not vulnerable to the H4
// name-collision leak.
func (s *Server) scopeKeys(c *caller, keys []store.KeyInfo) []store.KeyInfo {
	if c.root {
		return keys
	}
	out := make([]store.KeyInfo, 0, len(keys))
	for _, k := range keys {
		if k.OrgID == c.user.OrgID {
			out = append(out, k)
		}
	}
	return out
}

// --- Org endpoints -------------------------------------------------------

func (s *Server) handleCreateOrg(w http.ResponseWriter, r *http.Request, c *caller) {
	if !c.can(rbac.ActCreateOrg) {
		writeForbidden(w, "only the root admin key may create orgs")
		return
	}
	var req struct {
		Name             string  `json:"name"`
		MonthlyBudgetUSD float64 `json:"monthly_budget_usd"`
		RateLimitRPM     int     `json:"rate_limit_rpm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, errUnsupported, `"name" is required`)
		return
	}
	org, err := s.store.CreateOrg(r.Context(), req.Name, req.MonthlyBudgetUSD)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errProviderError, "failed to create org")
		return
	}
	// An optional non-zero rate limit is applied post-create so CreateOrg keeps
	// its Phase 5 signature (0 = unlimited, the default).
	if req.RateLimitRPM != 0 {
		org, err = s.store.UpdateOrg(r.Context(), org.ID, nil, &req.RateLimitRPM)
		if err != nil {
			writeError(w, http.StatusInternalServerError, errProviderError, "failed to set org rate limit")
			return
		}
	}
	writeJSON(w, http.StatusOK, org)
}

// handleUpdateOrg patches an org's monthly budget and/or rate limit. Root may
// patch any org; a user must be an owner or admin of the target org (the
// update_org capability). A nil field is left unchanged.
func (s *Server) handleUpdateOrg(w http.ResponseWriter, r *http.Request, c *caller) {
	orgID := r.PathValue("org_id")
	if !s.authorizeOrgAction(w, r, c, orgID, rbac.ActUpdateOrg) {
		return
	}
	var req struct {
		MonthlyBudgetUSD *float64 `json:"monthly_budget_usd"`
		RateLimitRPM     *int     `json:"rate_limit_rpm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}
	org, err := s.store.UpdateOrg(r.Context(), orgID, req.MonthlyBudgetUSD, req.RateLimitRPM)
	if errors.Is(err, store.ErrOrgNotFound) {
		writeError(w, http.StatusNotFound, errNotFound, "unknown org "+orgID)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, errProviderError, "failed to update org")
		return
	}
	writeJSON(w, http.StatusOK, org)
}

// handleWhoami reports the authenticated caller's identity. Root → {"root":true};
// a user token → its id, org, email, and role. Unauthenticated callers are
// rejected by adminAuth with 401 invalid_key before reaching here.
func (s *Server) handleWhoami(w http.ResponseWriter, _ *http.Request, c *caller) {
	if c.root {
		writeJSON(w, http.StatusOK, map[string]any{"root": true})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"root":    false,
		"user_id": c.user.ID,
		"org_id":  c.user.OrgID,
		"email":   c.user.Email,
		"role":    c.user.Role,
	})
}

// orgWithSpend is one row of GET /admin/orgs: an org plus its aggregate spend.
type orgWithSpend struct {
	store.Org
	SpendUSD float64 `json:"spend_usd"`
}

func (s *Server) handleListOrgs(w http.ResponseWriter, r *http.Request, c *caller) {
	if !c.can(rbac.ActListOrgs) {
		writeForbidden(w, "only the root admin key may list orgs")
		return
	}
	orgs, err := s.store.Orgs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, errProviderError, "failed to list orgs")
		return
	}
	// One grouped query for every org's spend rather than one query per org:
	// this page previously cost 1+N round trips and got slower with each tenant
	// added. Orgs with no keys are absent from the map and read as 0.
	spends, err := s.store.OrgSpends(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, errProviderError, "failed to aggregate org spend")
		return
	}
	out := make([]orgWithSpend, 0, len(orgs))
	for _, o := range orgs {
		out = append(out, orgWithSpend{Org: o, SpendUSD: spends[o.ID]})
	}
	writeJSON(w, http.StatusOK, out)
}

// --- User endpoints ------------------------------------------------------

// authorizeOrgAction checks that the caller may act on orgID for the given
// capability. It writes a 403 and returns false when denied.
func (s *Server) authorizeOrgAction(w http.ResponseWriter, r *http.Request, c *caller, orgID string, action rbac.Action) bool {
	if c.root {
		return true
	}
	if c.user.OrgID != orgID {
		s.denyOrgAction(w, r, c, orgID, "cannot act on another org")
		return false
	}
	if !c.can(action) {
		s.denyOrgAction(w, r, c, orgID, "role lacks capability "+string(action))
		return false
	}
	return true
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request, c *caller) {
	orgID := r.PathValue("org_id")
	if !s.authorizeOrgAction(w, r, c, orgID, rbac.ActCreateUser) {
		return
	}
	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, errUnsupported, `"email" is required`)
		return
	}
	if !rbac.ValidRole(req.Role) {
		writeError(w, http.StatusBadRequest, errUnsupported, `"role" must be owner, admin, member, or viewer`)
		return
	}
	// A non-root actor may not create a user above its own management scope.
	if !c.root && !rbac.CanManageRole(c.user.Role, req.Role) {
		writeForbidden(w, "role cannot create a user with role "+req.Role)
		return
	}
	user, token, err := s.store.CreateUser(r.Context(), orgID, req.Email, req.Role)
	if errors.Is(err, store.ErrOrgNotFound) {
		writeError(w, http.StatusNotFound, errNotFound, "unknown org "+orgID)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, errProviderError, "failed to create user")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":    user.ID,
		"email": user.Email,
		"role":  user.Role,
		"token": token, // shown once
	})
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request, c *caller) {
	orgID := r.PathValue("org_id")
	if !s.authorizeOrgAction(w, r, c, orgID, rbac.ActListUsers) {
		return
	}
	users, err := s.store.Users(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errProviderError, "failed to list users")
		return
	}
	if users == nil {
		users = []store.User{}
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request, c *caller) {
	orgID := r.PathValue("org_id")
	userID := r.PathValue("user_id")
	if !s.authorizeOrgAction(w, r, c, orgID, rbac.ActDeleteUser) {
		return
	}
	// A non-root actor may only delete users within its management scope.
	if !c.root {
		users, err := s.store.Users(r.Context(), orgID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, errProviderError, "failed to load users")
			return
		}
		var target *store.User
		for i := range users {
			if users[i].ID == userID {
				target = &users[i]
				break
			}
		}
		if target == nil {
			writeError(w, http.StatusNotFound, errNotFound, "unknown user "+userID)
			return
		}
		if !rbac.CanManageRole(c.user.Role, target.Role) {
			writeForbidden(w, "role cannot delete a user with role "+target.Role)
			return
		}
	}
	err := s.store.DeleteUser(r.Context(), orgID, userID)
	if errors.Is(err, store.ErrUserNotFound) {
		writeError(w, http.StatusNotFound, errNotFound, "unknown user "+userID)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, errProviderError, "failed to delete user")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": userID})
}

// --- Secrets status ------------------------------------------------------

// secretStatus is one row of GET /admin/secrets/status. Values are never
// included — only presence and the active backend.
type secretStatus struct {
	Name    string `json:"name"`
	Present bool   `json:"present"`
	Source  string `json:"source"`
}

// secretsStatus builds the GET /admin/secrets/status rows: presence and active
// backend per configured secret name. Values are never included.
func (s *Server) secretsStatus() []secretStatus {
	out := make([]secretStatus, 0, len(s.secretNames))
	for _, name := range s.secretNames {
		// "present" means a usable value: a set-but-empty var (e.g. compose
		// passing ${VAR:-}) is reported absent, matching how routing treats it.
		value, ok := s.secrets.Get(name)
		out = append(out, secretStatus{Name: name, Present: ok && value != "", Source: s.secrets.Backend()})
	}
	return out
}

func (s *Server) handleSecretsStatus(w http.ResponseWriter, _ *http.Request, c *caller) {
	if !c.root {
		writeForbidden(w, "only the root admin key may view secrets status")
		return
	}
	writeJSON(w, http.StatusOK, s.secretsStatus())
}

// handleProviders reports the configured OpenAI-compatible providers. It reports
// only whether each credential RESOLVES — never a key name's value — so the page
// can never leak a secret. Root-admin only, like the secrets status endpoint.
func (s *Server) handleProviders(w http.ResponseWriter, _ *http.Request, c *caller) {
	if !c.root {
		writeForbidden(w, "only the root admin key may view providers")
		return
	}
	type providerOut struct {
		Name       string   `json:"name"`
		BaseURL    string   `json:"base_url"`
		Enabled    bool     `json:"enabled"`
		KeyPresent bool     `json:"key_present"`
		Models     []string `json:"models"`
	}
	out := []providerOut{}
	for _, name := range s.providers.Names() {
		e, _ := s.providers.Lookup(name)
		present := e.KeyName == ""
		if !present && s.secrets != nil {
			v, ok := s.secrets.Get(e.KeyName)
			present = ok && v != ""
		}
		models := make([]string, 0, len(e.Prices))
		for m := range e.Prices {
			models = append(models, m)
		}
		sort.Strings(models)
		out = append(out, providerOut{
			Name: e.Name, BaseURL: e.BaseURL, Enabled: e.Enabled,
			KeyPresent: present, Models: models,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": out})
}
