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
	token, err := s.upsertSSOUser(r, orgID, identity.Email, identity.Sub, &role)
	if err != nil {
		writeError(w, http.StatusUnauthorized, errSSOFailed, "could not provision user")
		return
	}

	redirect := s.oidc.PostLoginURL() + "#token=" + url.QueryEscape(token) +
		"&email=" + url.QueryEscape(identity.Email) +
		"&role=" + url.QueryEscape(role)
	http.Redirect(w, r, redirect, http.StatusFound)
}

// errSSOSubjectMismatch is returned when a verified email matches an existing
// user already bound to a different OIDC subject. Adopting that user's role
// would be an account takeover (H3), so the login is refused.
var errSSOSubjectMismatch = errors.New("sso: email belongs to a different subject")

// upsertSSOUser resolves the SSO user for orgID and returns a freshly minted
// token, updating *role to the effective role for the redirect fragment.
//
// Identity is keyed on the stable OIDC subject (sub), stored in the user's
// external_id — never on the mutable email (H3). Resolution order:
//
//  1. A user already bound to sub → returning user; reuse identity and role.
//  2. Else a user with this (verified) email:
//     - bound to a DIFFERENT sub → refuse (errSSOSubjectMismatch); a spoofed
//     email must never inherit another subject's role.
//     - not yet bound to any sub → a pre-provisioned account; link sub to it on
//     this first SSO login and adopt its role.
//  3. Else create a new user with the default role, binding sub.
func (s *Server) upsertSSOUser(r *http.Request, orgID, email, sub string, role *string) (string, error) {
	ctx := r.Context()

	// 1. Stable subject match: the canonical "returning user" path.
	if sub != "" {
		bound, err := s.store.UserByExternalID(ctx, orgID, sub)
		switch {
		case err == nil:
			*role = bound.Role
			return s.store.IssueUserToken(ctx, bound.ID)
		case errors.Is(err, store.ErrUserNotFound):
			// fall through to the email path
		default:
			return "", err
		}
	}

	// 2. Consider an existing user with this email.
	existing, err := s.store.UserByEmail(ctx, orgID, email)
	switch {
	case err == nil:
		if existing.ExternalID != "" && existing.ExternalID != sub {
			// The email is already claimed by a different subject — reject.
			return "", errSSOSubjectMismatch
		}
		if sub != "" && existing.ExternalID == "" {
			// First SSO login for a pre-provisioned account: bind the subject so
			// future logins match on sub (step 1).
			if serr := s.store.SetUserExternalID(ctx, existing.ID, sub); serr != nil {
				return "", serr
			}
		}
		*role = existing.Role
		return s.store.IssueUserToken(ctx, existing.ID)
	case errors.Is(err, store.ErrUserNotFound):
		// 3. First-time login: create with the default role, binding the subject.
		user, token, cerr := s.store.CreateUserWithExternalID(ctx, orgID, email, *role, sub)
		if cerr != nil {
			return "", cerr
		}
		*role = user.Role
		return token, nil
	default:
		return "", err
	}
}
