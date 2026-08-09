package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/ifahad/agentos/gateway/internal/scim"
	"github.com/ifahad/agentos/gateway/internal/store"
)

// scimEnabled reports whether SCIM provisioning is configured (a token is set).
func (s *Server) scimEnabled() bool { return s.scimToken != "" }

// scimAuth guards every /scim/v2/* route: it requires Authorization: Bearer
// <AGENTOS_SCIM_TOKEN>. A missing/wrong token is a 401 SCIM error. Routes are
// only registered when SCIM is enabled, so a disabled gateway 404s these paths.
func (s *Server) scimAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok || !secureCompare(token, s.scimToken) {
			writeSCIMError(w, http.StatusUnauthorized, "missing or invalid SCIM bearer token")
			return
		}
		next(w, r)
	}
}

// scimUsersBaseURL returns the absolute /scim/v2/Users prefix for meta.location.
func scimUsersBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/scim/v2/Users"
}

// scimResource renders a store.User as a SCIM User resource.
func scimResource(r *http.Request, u *store.User) scim.User {
	return scim.NewUser(u.ID, u.ExternalID, u.Email, "", u.Active, scimUsersBaseURL(r))
}

// writeSCIMError emits the SCIM error shape with the given HTTP status.
func writeSCIMError(w http.ResponseWriter, status int, detail string) {
	writeSCIM(w, status, scim.NewError(status, detail))
}

// writeSCIM writes a SCIM JSON body with the RFC-recommended content type.
func writeSCIM(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// best-effort; the status/headers are already committed
		return
	}
}

// scimUserByID finds a SCIM-managed user (in the SCIM default org) by id.
func (s *Server) scimUserByID(r *http.Request, id string) (*store.User, error) {
	users, err := s.store.Users(r.Context(), s.scimOrg)
	if err != nil {
		return nil, err
	}
	for i := range users {
		if users[i].ID == id {
			return &users[i], nil
		}
	}
	return nil, store.ErrUserNotFound
}

// scimUserBody is the accepted POST/PUT payload subset.
type scimUserBody struct {
	UserName   string `json:"userName"`
	ExternalID string `json:"externalId"`
	Active     *bool  `json:"active"`
	Name       struct {
		Formatted string `json:"formatted"`
	} `json:"name"`
}

// handleSCIMCreateUser upserts a user by email (userName) in the SCIM default
// org: a new user is created with the default role; an existing inactive user is
// reactivated; an existing active user is a 409 conflict.
func (s *Server) handleSCIMCreateUser(w http.ResponseWriter, r *http.Request) {
	var body scimUserBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.UserName == "" {
		writeSCIMError(w, http.StatusBadRequest, `"userName" is required`)
		return
	}
	active := true
	if body.Active != nil {
		active = *body.Active
	}

	existing, err := s.store.UserByEmail(r.Context(), s.scimOrg, body.UserName)
	switch {
	case err == nil:
		if existing.Active {
			writeSCIM(w, http.StatusConflict,
				scim.NewErrorType(http.StatusConflict, "uniqueness",
					"a user with userName "+body.UserName+" already exists"))
			return
		}
		// Reactivate (or re-provision) an existing deactivated user in place.
		if serr := s.store.SetUserActive(r.Context(), existing.ID, active); serr != nil {
			writeSCIMError(w, http.StatusInternalServerError, "failed to update user")
			return
		}
		if body.ExternalID != "" {
			if serr := s.store.SetUserExternalID(r.Context(), existing.ID, body.ExternalID); serr != nil {
				writeSCIMError(w, http.StatusInternalServerError, "failed to update user")
				return
			}
		}
		fresh, ferr := s.scimUserByID(r, existing.ID)
		if ferr != nil {
			writeSCIMError(w, http.StatusInternalServerError, "failed to load user")
			return
		}
		writeSCIM(w, http.StatusCreated, scimResource(r, fresh))
		return
	case errors.Is(err, store.ErrUserNotFound):
		user, _, cerr := s.store.CreateUserWithExternalID(r.Context(), s.scimOrg, body.UserName, s.scimRole, body.ExternalID)
		if errors.Is(cerr, store.ErrOrgNotFound) {
			writeSCIMError(w, http.StatusInternalServerError, "SCIM default org "+s.scimOrg+" does not exist")
			return
		}
		if cerr != nil {
			writeSCIMError(w, http.StatusInternalServerError, "failed to create user")
			return
		}
		if !active {
			if serr := s.store.SetUserActive(r.Context(), user.ID, false); serr != nil {
				writeSCIMError(w, http.StatusInternalServerError, "failed to update user")
				return
			}
			user.Active = false
		}
		writeSCIM(w, http.StatusCreated, scimResource(r, user))
		return
	default:
		writeSCIMError(w, http.StatusInternalServerError, "failed to look up user")
	}
}

// handleSCIMGetUser returns one user by id.
func (s *Server) handleSCIMGetUser(w http.ResponseWriter, r *http.Request) {
	u, err := s.scimUserByID(r, r.PathValue("id"))
	if errors.Is(err, store.ErrUserNotFound) {
		writeSCIMError(w, http.StatusNotFound, "user "+r.PathValue("id")+" not found")
		return
	}
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "failed to load user")
		return
	}
	writeSCIM(w, http.StatusOK, scimResource(r, u))
}

// handleSCIMListUsers lists users in the SCIM default org, honoring a
// filter=userName eq "x" query. An unsupported filter yields an empty list.
func (s *Server) handleSCIMListUsers(w http.ResponseWriter, r *http.Request) {
	filter := strings.TrimSpace(r.URL.Query().Get("filter"))
	if filter != "" {
		userName, ok := parseEqFilter(filter, "userName")
		if !ok {
			// Only userName eq is supported; anything else matches nothing.
			writeSCIM(w, http.StatusOK, scim.NewListResponse(nil))
			return
		}
		u, err := s.store.UserByEmail(r.Context(), s.scimOrg, userName)
		if errors.Is(err, store.ErrUserNotFound) {
			writeSCIM(w, http.StatusOK, scim.NewListResponse(nil))
			return
		}
		if err != nil {
			writeSCIMError(w, http.StatusInternalServerError, "failed to filter users")
			return
		}
		writeSCIM(w, http.StatusOK, scim.NewListResponse([]scim.User{scimResource(r, u)}))
		return
	}

	users, err := s.store.Users(r.Context(), s.scimOrg)
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	resources := make([]scim.User, 0, len(users))
	for i := range users {
		resources = append(resources, scimResource(r, &users[i]))
	}
	writeSCIM(w, http.StatusOK, scim.NewListResponse(resources))
}

// parseEqFilter extracts x from `<attr> eq "x"` (case-insensitive on the
// attribute and operator). ok is false for any other filter.
//
// SplitN with a limit of 3 rather than strings.Fields: the value is the
// remainder, so a Group displayName containing spaces — which is the normal
// case, unlike userName — survives intact.
func parseEqFilter(filter, attr string) (value string, ok bool) {
	fields := strings.SplitN(filter, " ", 3)
	if len(fields) != 3 {
		return "", false
	}
	if !strings.EqualFold(fields[0], attr) || !strings.EqualFold(fields[1], "eq") {
		return "", false
	}
	val := strings.TrimSpace(fields[2])
	if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
		return val[1 : len(val)-1], true
	}
	return "", false
}

// handleSCIMPatchUser applies an RFC 7644 replace of the active attribute.
func (s *Server) handleSCIMPatchUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	u, err := s.scimUserByID(r, id)
	if errors.Is(err, store.ErrUserNotFound) {
		writeSCIMError(w, http.StatusNotFound, "user "+id+" not found")
		return
	}
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "failed to load user")
		return
	}
	var patch scim.PatchRequest
	if derr := json.NewDecoder(r.Body).Decode(&patch); derr != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalid PATCH body")
		return
	}
	if active, found := patch.ActivePatch(); found {
		if serr := s.store.SetUserActive(r.Context(), id, active); serr != nil {
			writeSCIMError(w, http.StatusInternalServerError, "failed to update user")
			return
		}
		u.Active = active
	}
	writeSCIM(w, http.StatusOK, scimResource(r, u))
}

// handleSCIMPutUser replaces active and externalId on an existing user.
func (s *Server) handleSCIMPutUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	u, err := s.scimUserByID(r, id)
	if errors.Is(err, store.ErrUserNotFound) {
		writeSCIMError(w, http.StatusNotFound, "user "+id+" not found")
		return
	}
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "failed to load user")
		return
	}
	var body scimUserBody
	if derr := json.NewDecoder(r.Body).Decode(&body); derr != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	// PUT is a full replace: an omitted active means active (SCIM default).
	active := true
	if body.Active != nil {
		active = *body.Active
	}
	if serr := s.store.SetUserActive(r.Context(), id, active); serr != nil {
		writeSCIMError(w, http.StatusInternalServerError, "failed to update user")
		return
	}
	if serr := s.store.SetUserExternalID(r.Context(), id, body.ExternalID); serr != nil {
		writeSCIMError(w, http.StatusInternalServerError, "failed to update user")
		return
	}
	u.Active = active
	u.ExternalID = body.ExternalID
	writeSCIM(w, http.StatusOK, scimResource(r, u))
}

// handleSCIMDeleteUser hard-deletes a user (204).
func (s *Server) handleSCIMDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.store.DeleteUser(r.Context(), s.scimOrg, id)
	if errors.Is(err, store.ErrUserNotFound) {
		writeSCIMError(w, http.StatusNotFound, "user "+id+" not found")
		return
	}
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Discovery stubs -----------------------------------------------------

func (s *Server) handleSCIMServiceProviderConfig(w http.ResponseWriter, _ *http.Request) {
	writeSCIM(w, http.StatusOK, scim.ServiceProviderConfig())
}

func (s *Server) handleSCIMResourceTypes(w http.ResponseWriter, _ *http.Request) {
	writeSCIM(w, http.StatusOK, scim.ResourceTypes())
}

func (s *Server) handleSCIMSchemas(w http.ResponseWriter, _ *http.Request) {
	writeSCIM(w, http.StatusOK, scim.Schemas())
}

// registerSCIMRoutes wires the /scim/v2/* routes (only called when enabled).
func (s *Server) registerSCIMRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /scim/v2/Users", s.scimAuth(s.handleSCIMCreateUser))
	mux.HandleFunc("GET /scim/v2/Users", s.scimAuth(s.handleSCIMListUsers))
	mux.HandleFunc("GET /scim/v2/Users/{id}", s.scimAuth(s.handleSCIMGetUser))
	mux.HandleFunc("PATCH /scim/v2/Users/{id}", s.scimAuth(s.handleSCIMPatchUser))
	mux.HandleFunc("PUT /scim/v2/Users/{id}", s.scimAuth(s.handleSCIMPutUser))
	mux.HandleFunc("DELETE /scim/v2/Users/{id}", s.scimAuth(s.handleSCIMDeleteUser))

	// All six at once. A registered path under an unregistered method answers
	// 405 with a text/plain body BEFORE any handler wrapper runs, which a SCIM
	// client cannot parse as an Error — so a partially registered resource is
	// worse than an absent one.
	mux.HandleFunc("POST /scim/v2/Groups", s.scimAuth(s.handleSCIMCreateGroup))
	mux.HandleFunc("GET /scim/v2/Groups", s.scimAuth(s.handleSCIMListGroups))
	mux.HandleFunc("GET /scim/v2/Groups/{id}", s.scimAuth(s.handleSCIMGetGroup))
	mux.HandleFunc("PATCH /scim/v2/Groups/{id}", s.scimAuth(s.handleSCIMPatchGroup))
	mux.HandleFunc("PUT /scim/v2/Groups/{id}", s.scimAuth(s.handleSCIMPutGroup))
	mux.HandleFunc("DELETE /scim/v2/Groups/{id}", s.scimAuth(s.handleSCIMDeleteGroup))
	mux.HandleFunc("GET /scim/v2/ServiceProviderConfig", s.scimAuth(s.handleSCIMServiceProviderConfig))
	mux.HandleFunc("GET /scim/v2/ResourceTypes", s.scimAuth(s.handleSCIMResourceTypes))
	mux.HandleFunc("GET /scim/v2/Schemas", s.scimAuth(s.handleSCIMSchemas))
}
