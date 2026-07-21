package server

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/ifahad/agentos/gateway/internal/store"
)

// oidcEnabled reports whether SSO is configured.
func (s *Server) oidcEnabled() bool { return s.oidc.Enabled() }

// handleOIDCStatus reports whether SSO is enabled (no auth). The console uses
// it to show or hide the "Sign in with SSO" button.
func (s *Server) handleOIDCStatus(w http.ResponseWriter, _ *http.Request) {
	out := map[string]any{"enabled": s.oidcEnabled()}
	if s.oidcEnabled() {
		out["issuer"] = s.oidc.Issuer()
	}
	writeJSON(w, http.StatusOK, out)
}

// handleOIDCLogin redirects the browser to the provider's authorization
// endpoint with a signed state. Disabled → 404 sso_disabled.
func (s *Server) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	if !s.oidcEnabled() {
		writeError(w, http.StatusNotFound, errSSODisabled, "SSO is not enabled")
		return
	}
	state, err := s.oidc.NewState(time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, errProviderError, "failed to build login state")
		return
	}
	http.Redirect(w, r, s.oidc.AuthCodeURL(state), http.StatusFound)
}

// handleOIDCCallback validates the state, exchanges the code, verifies the ID
// token, upserts the user in the default org, mints an agu- token, and
// redirects to the console with the token in the URL fragment. Disabled → 404;
// any validation failure → 401 sso_failed.
func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if !s.oidcEnabled() {
		writeError(w, http.StatusNotFound, errSSODisabled, "SSO is not enabled")
		return
	}
	q := r.URL.Query()
	if err := s.oidc.VerifyState(q.Get("state"), time.Now()); err != nil {
		writeError(w, http.StatusUnauthorized, errSSOFailed, "invalid or expired login state")
		return
	}
	code := q.Get("code")
	if code == "" {
		writeError(w, http.StatusUnauthorized, errSSOFailed, "missing authorization code")
		return
	}
	identity, err := s.oidc.Exchange(r.Context(), code)
	if err != nil {
		writeError(w, http.StatusUnauthorized, errSSOFailed, "authentication failed")
		return
	}

	orgID := s.oidc.DefaultOrg()
	role := s.oidc.DefaultRole()
	token, err := s.upsertSSOUser(r, orgID, identity.Email, &role)
	if err != nil {
		writeError(w, http.StatusUnauthorized, errSSOFailed, "could not provision user")
		return
	}

	redirect := s.oidc.PostLoginURL() + "#token=" + url.QueryEscape(token) +
		"&email=" + url.QueryEscape(identity.Email) +
		"&role=" + url.QueryEscape(role)
	http.Redirect(w, r, redirect, http.StatusFound)
}

// upsertSSOUser reuses an existing user with email in orgID (minting a fresh
// token and adopting its stored role) or creates one with the default role.
// role is updated to reflect the effective role for the redirect fragment.
func (s *Server) upsertSSOUser(r *http.Request, orgID, email string, role *string) (string, error) {
	ctx := r.Context()
	existing, err := s.store.UserByEmail(ctx, orgID, email)
	switch {
	case err == nil:
		*role = existing.Role
		return s.store.IssueUserToken(ctx, existing.ID)
	case errors.Is(err, store.ErrUserNotFound):
		user, token, cerr := s.store.CreateUser(ctx, orgID, email, *role)
		if cerr != nil {
			return "", cerr
		}
		*role = user.Role
		return token, nil
	default:
		return "", err
	}
}
