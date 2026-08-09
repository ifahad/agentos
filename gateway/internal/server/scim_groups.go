package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/ifahad/agentos/gateway/internal/scim"
	"github.com/ifahad/agentos/gateway/internal/store"
)

// scimGroupsBaseURL returns the absolute /scim/v2/Groups prefix for meta.location.
func scimGroupsBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/scim/v2/Groups"
}

// excludesMembers reports whether the caller asked for the members attribute to
// be left out. Entra requests this on list calls to keep responses small.
func excludesMembers(r *http.Request) bool {
	for _, part := range strings.Split(r.URL.Query().Get("excludedAttributes"), ",") {
		if strings.EqualFold(strings.TrimSpace(part), "members") {
			return true
		}
	}
	return false
}

// scimGroupResource renders a store.Group plus its members as a SCIM resource.
// Passing members=nil with excluded=true omits the key entirely; otherwise an
// empty membership still renders "members": [], which Okta requires.
func scimGroupResource(r *http.Request, g *store.Group, members []store.User, excluded bool) scim.Group {
	var list []scim.Member
	if !excluded {
		list = make([]scim.Member, 0, len(members))
		userBase := scimUsersBaseURL(r)
		for i := range members {
			list = append(list, scim.Member{
				Value:   members[i].ID,
				Display: members[i].Email,
				Ref:     userBase + "/" + members[i].ID,
				Type:    "User",
			})
		}
	}
	res := scim.NewGroup(g.ID, g.ExternalID, g.DisplayName, list, scimGroupsBaseURL(r))
	if excluded {
		res.Members = nil
	}
	return res
}

// loadGroupResource fetches a group and its members and renders the resource.
func (s *Server) loadGroupResource(r *http.Request, id string) (scim.Group, error) {
	g, err := s.store.GroupByID(r.Context(), s.scimOrg, id)
	if err != nil {
		return scim.Group{}, err
	}
	excluded := excludesMembers(r)
	var members []store.User
	if !excluded {
		if members, err = s.store.GroupMembers(r.Context(), s.scimOrg, id); err != nil {
			return scim.Group{}, err
		}
	}
	return scimGroupResource(r, g, members, excluded), nil
}

// scimGroupBody is the accepted POST/PUT payload subset. `schemas` is ignored:
// Entra sends a Microsoft extension URN alongside the core one, and the User
// path already sets the precedent of not modelling it.
type scimGroupBody struct {
	DisplayName string `json:"displayName"`
	ExternalID  string `json:"externalId"`
	Members     []struct {
		Value string `json:"value"`
	} `json:"members"`
}

func (b scimGroupBody) memberIDs() []string {
	ids := make([]string, 0, len(b.Members))
	for _, m := range b.Members {
		if m.Value != "" {
			ids = append(ids, m.Value)
		}
	}
	return ids
}

// handleSCIMCreateGroup creates a group in the SCIM default org. A duplicate
// displayName is a 409 with scimType "uniqueness": Entra retries a create until
// it receives exactly that, so a generic error makes provisioning loop forever.
//
// Unlike the User path there is no reactivation branch — a Group has no active
// flag, so a duplicate name is simply a conflict.
func (s *Server) handleSCIMCreateGroup(w http.ResponseWriter, r *http.Request) {
	var body scimGroupBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.DisplayName == "" {
		writeSCIMError(w, http.StatusBadRequest, `"displayName" is required`)
		return
	}

	g, err := s.store.CreateGroup(r.Context(), s.scimOrg, body.DisplayName, body.ExternalID)
	switch {
	case errors.Is(err, store.ErrGroupExists):
		writeSCIM(w, http.StatusConflict,
			scim.NewErrorType(http.StatusConflict, "uniqueness",
				"a group with displayName "+body.DisplayName+" already exists"))
		return
	case errors.Is(err, store.ErrOrgNotFound):
		writeSCIMError(w, http.StatusInternalServerError, "SCIM default org "+s.scimOrg+" does not exist")
		return
	case err != nil:
		writeSCIMError(w, http.StatusInternalServerError, "failed to create group")
		return
	}

	// Inbound members are honoured, not ignored: echoing an empty list back
	// while the request named members is silent data loss.
	if ids := body.memberIDs(); len(ids) > 0 {
		if err := s.store.SetGroupMembers(r.Context(), s.scimOrg, g.ID, ids); err != nil {
			if errors.Is(err, store.ErrUserNotFound) {
				writeSCIM(w, http.StatusBadRequest,
					scim.NewErrorType(http.StatusBadRequest, "invalidValue",
						"members references a user that does not exist"))
				return
			}
			writeSCIMError(w, http.StatusInternalServerError, "failed to set group members")
			return
		}
	}

	res, err := s.loadGroupResource(r, g.ID)
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "failed to load group")
		return
	}
	writeSCIM(w, http.StatusCreated, res)
}

// handleSCIMGetGroup returns one group by id, with members unless excluded.
func (s *Server) handleSCIMGetGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	res, err := s.loadGroupResource(r, id)
	if errors.Is(err, store.ErrGroupNotFound) {
		writeSCIMError(w, http.StatusNotFound, "group "+id+" not found")
		return
	}
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "failed to load group")
		return
	}
	writeSCIM(w, http.StatusOK, res)
}

// handleSCIMListGroups lists groups in the SCIM default org, honouring
// filter=displayName eq "x". An unsupported filter yields an empty list rather
// than a 400, matching the User path and RFC-tolerant IdP behaviour.
func (s *Server) handleSCIMListGroups(w http.ResponseWriter, r *http.Request) {
	excluded := excludesMembers(r)

	if filter := strings.TrimSpace(r.URL.Query().Get("filter")); filter != "" {
		name, ok := parseEqFilter(filter, "displayName")
		if !ok {
			writeSCIM(w, http.StatusOK, scim.NewGroupListResponse(nil))
			return
		}
		g, err := s.store.GroupByDisplayName(r.Context(), s.scimOrg, name)
		if errors.Is(err, store.ErrGroupNotFound) {
			writeSCIM(w, http.StatusOK, scim.NewGroupListResponse(nil))
			return
		}
		if err != nil {
			writeSCIMError(w, http.StatusInternalServerError, "failed to filter groups")
			return
		}
		res, err := s.loadGroupResource(r, g.ID)
		if err != nil {
			writeSCIMError(w, http.StatusInternalServerError, "failed to load group")
			return
		}
		writeSCIM(w, http.StatusOK, scim.NewGroupListResponse([]scim.Group{res}))
		return
	}

	groups, err := s.store.Groups(r.Context(), s.scimOrg)
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "failed to list groups")
		return
	}
	resources := make([]scim.Group, 0, len(groups))
	for i := range groups {
		var members []store.User
		if !excluded {
			if members, err = s.store.GroupMembers(r.Context(), s.scimOrg, groups[i].ID); err != nil {
				writeSCIMError(w, http.StatusInternalServerError, "failed to load group members")
				return
			}
		}
		resources = append(resources, scimGroupResource(r, &groups[i], members, excluded))
	}
	writeSCIM(w, http.StatusOK, scim.NewGroupListResponse(resources))
}

// applyGroupOps folds the ordered PATCH operations over the current membership
// and returns the resulting id set plus any rename. Order matters: Okta packs a
// remove and an add into one Operations array, and a member removed by the
// first op then re-added by the second must survive.
func applyGroupOps(current []string, ops []scim.GroupOp) (members []string, rename string) {
	set := make(map[string]struct{}, len(current))
	order := append([]string(nil), current...)
	for _, id := range current {
		set[id] = struct{}{}
	}
	add := func(id string) {
		if _, ok := set[id]; !ok {
			set[id] = struct{}{}
			order = append(order, id)
		}
	}
	for _, op := range ops {
		switch op.Kind {
		case scim.GroupOpAddMembers:
			for _, id := range op.MemberIDs {
				add(id)
			}
		case scim.GroupOpRemoveMembers:
			if len(op.MemberIDs) == 0 {
				set, order = map[string]struct{}{}, nil
				continue
			}
			for _, id := range op.MemberIDs {
				delete(set, id)
			}
		case scim.GroupOpReplaceMembers:
			set, order = map[string]struct{}{}, nil
			for _, id := range op.MemberIDs {
				add(id)
			}
		case scim.GroupOpSetDisplayName:
			rename = op.DisplayName
		}
	}
	out := make([]string, 0, len(set))
	for _, id := range order {
		if _, ok := set[id]; ok {
			out = append(out, id)
		}
	}
	return out, rename
}

// handleSCIMPatchGroup applies member add/remove/replace and rename operations.
//
// Responds 200 with the full resource. Entra's docs prefer 204 and call
// returning the member list "not advisable", but that is a payload-size hint;
// RFC 7644 §3.5.2 permits either and Okta accepts both. 200 keeps this handler
// consistent with handleSCIMPatchUser — two PATCH handlers in one package
// returning different codes is a seam somebody later "fixes" in the wrong
// direction — and lets a caller confirm the resulting membership in one call.
func (s *Server) handleSCIMPatchGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Load first so an unknown id is a 404 before the body is even parsed,
	// matching handleSCIMPatchUser.
	if _, err := s.store.GroupByID(r.Context(), s.scimOrg, id); err != nil {
		if errors.Is(err, store.ErrGroupNotFound) {
			writeSCIMError(w, http.StatusNotFound, "group "+id+" not found")
			return
		}
		writeSCIMError(w, http.StatusInternalServerError, "failed to load group")
		return
	}

	var body scim.PatchRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalid PATCH body")
		return
	}

	existing, err := s.store.GroupMembers(r.Context(), s.scimOrg, id)
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "failed to load group members")
		return
	}
	current := make([]string, 0, len(existing))
	for i := range existing {
		current = append(current, existing[i].ID)
	}

	next, rename := applyGroupOps(current, body.GroupOps())
	if rename != "" {
		if err := s.store.RenameGroup(r.Context(), s.scimOrg, id, rename); err != nil {
			if errors.Is(err, store.ErrGroupExists) {
				writeSCIM(w, http.StatusConflict,
					scim.NewErrorType(http.StatusConflict, "uniqueness",
						"a group with displayName "+rename+" already exists"))
				return
			}
			writeSCIMError(w, http.StatusInternalServerError, "failed to rename group")
			return
		}
	}
	if err := s.store.SetGroupMembers(r.Context(), s.scimOrg, id, next); err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			writeSCIM(w, http.StatusBadRequest,
				scim.NewErrorType(http.StatusBadRequest, "invalidValue",
					"members references a user that does not exist"))
			return
		}
		writeSCIMError(w, http.StatusInternalServerError, "failed to set group members")
		return
	}

	res, err := s.loadGroupResource(r, id)
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "failed to load group")
		return
	}
	writeSCIM(w, http.StatusOK, res)
}

// handleSCIMPutGroup replaces a group's displayName and membership wholesale.
func (s *Server) handleSCIMPutGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.store.GroupByID(r.Context(), s.scimOrg, id); err != nil {
		if errors.Is(err, store.ErrGroupNotFound) {
			writeSCIMError(w, http.StatusNotFound, "group "+id+" not found")
			return
		}
		writeSCIMError(w, http.StatusInternalServerError, "failed to load group")
		return
	}
	var body scimGroupBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.DisplayName == "" {
		writeSCIMError(w, http.StatusBadRequest, `"displayName" is required`)
		return
	}
	if err := s.store.RenameGroup(r.Context(), s.scimOrg, id, body.DisplayName); err != nil {
		if errors.Is(err, store.ErrGroupExists) {
			writeSCIM(w, http.StatusConflict,
				scim.NewErrorType(http.StatusConflict, "uniqueness",
					"a group with displayName "+body.DisplayName+" already exists"))
			return
		}
		writeSCIMError(w, http.StatusInternalServerError, "failed to rename group")
		return
	}
	// PUT is a full replacement, so an absent members array means "no members".
	if err := s.store.SetGroupMembers(r.Context(), s.scimOrg, id, body.memberIDs()); err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			writeSCIM(w, http.StatusBadRequest,
				scim.NewErrorType(http.StatusBadRequest, "invalidValue",
					"members references a user that does not exist"))
			return
		}
		writeSCIMError(w, http.StatusInternalServerError, "failed to set group members")
		return
	}
	res, err := s.loadGroupResource(r, id)
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "failed to load group")
		return
	}
	writeSCIM(w, http.StatusOK, res)
}

// handleSCIMDeleteGroup removes a group. Membership rows go with it; the users
// themselves are untouched.
func (s *Server) handleSCIMDeleteGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.store.DeleteGroup(r.Context(), s.scimOrg, id)
	if errors.Is(err, store.ErrGroupNotFound) {
		writeSCIMError(w, http.StatusNotFound, "group "+id+" not found")
		return
	}
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "failed to delete group")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
