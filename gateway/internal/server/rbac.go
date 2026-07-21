package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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
		if s.adminKey != "" && token == s.adminKey {
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

// orgBudgetExceeded reports whether the org's aggregate key spend has reached
// its monthly cap. A zero (or missing) cap means unlimited, so pre-existing
// keys in the default org are never blocked — preserving Phase 1–4 behavior.
func (s *Server) orgBudgetExceeded(ctx context.Context, orgID string) bool {
	org, err := s.store.Org(ctx, orgID)
	if err != nil || org.MonthlyBudgetUSD <= 0 {
		return false
	}
	spend, err := s.store.OrgSpend(ctx, orgID)
	if err != nil {
		return false
	}
	return spend >= org.MonthlyBudgetUSD
}

// scopeKeys returns all keys for a root caller, or only the caller's org's keys
// for a user caller.
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

// orgKeyNames returns the set of key names belonging to orgID.
func (s *Server) orgKeyNames(ctx context.Context, orgID string) (map[string]bool, error) {
	keys, err := s.store.Keys(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[string]bool)
	for _, k := range keys {
		if k.OrgID == orgID {
			names[k.Name] = true
		}
	}
	return names, nil
}

// filterUsage keeps only usage rows whose key belongs to the org.
func filterUsage(usage []store.KeyUsage, names map[string]bool) []store.KeyUsage {
	out := make([]store.KeyUsage, 0, len(usage))
	for _, u := range usage {
		if names[u.Name] {
			out = append(out, u)
		}
	}
	return out
}

// filterAudit keeps only audit rows whose key belongs to the org.
func filterAudit(entries []store.AuditEntry, names map[string]bool) []store.AuditEntry {
	out := make([]store.AuditEntry, 0, len(entries))
	for _, e := range entries {
		if names[e.KeyName] {
			out = append(out, e)
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
		writeError(w, http.StatusBadRequest, errUnsupported, "invalid JSON body")
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
	if !s.authorizeOrgAction(w, c, orgID, rbac.ActUpdateOrg) {
		return
	}
	var req struct {
		MonthlyBudgetUSD *float64 `json:"monthly_budget_usd"`
		RateLimitRPM     *int     `json:"rate_limit_rpm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errUnsupported, "invalid JSON body")
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
	out := make([]orgWithSpend, 0, len(orgs))
	for _, o := range orgs {
		spend, err := s.store.OrgSpend(r.Context(), o.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, errProviderError, "failed to aggregate org spend")
			return
		}
		out = append(out, orgWithSpend{Org: o, SpendUSD: spend})
	}
	writeJSON(w, http.StatusOK, out)
}

// --- User endpoints ------------------------------------------------------

// authorizeOrgAction checks that the caller may act on orgID for the given
// capability. It writes a 403 and returns false when denied.
func (s *Server) authorizeOrgAction(w http.ResponseWriter, c *caller, orgID string, action rbac.Action) bool {
	if c.root {
		return true
	}
	if c.user.OrgID != orgID {
		writeForbidden(w, "cannot act on another org")
		return false
	}
	if !c.can(action) {
		writeForbidden(w, "role lacks capability "+string(action))
		return false
	}
	return true
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request, c *caller) {
	orgID := r.PathValue("org_id")
	if !s.authorizeOrgAction(w, c, orgID, rbac.ActCreateUser) {
		return
	}
	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errUnsupported, "invalid JSON body")
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
	if !s.authorizeOrgAction(w, c, orgID, rbac.ActListUsers) {
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
	if !s.authorizeOrgAction(w, c, orgID, rbac.ActDeleteUser) {
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

func (s *Server) handleSecretsStatus(w http.ResponseWriter, _ *http.Request, c *caller) {
	if !c.root {
		writeForbidden(w, "only the root admin key may view secrets status")
		return
	}
	out := make([]secretStatus, 0, len(s.secretNames))
	for _, name := range s.secretNames {
		// "present" means a usable value: a set-but-empty var (e.g. compose
		// passing ${VAR:-}) is reported absent, matching how routing treats it.
		value, ok := s.secrets.Get(name)
		out = append(out, secretStatus{Name: name, Present: ok && value != "", Source: s.secrets.Backend()})
	}
	writeJSON(w, http.StatusOK, out)
}
