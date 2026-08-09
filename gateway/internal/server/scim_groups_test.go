package server

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// sameSet compares membership ignoring order: the wire order of members is not
// part of the contract, only the set is.
func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x := append([]string(nil), a...)
	y := append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

// createSCIMUser provisions a user through the SCIM API and returns its id, so
// group tests exercise the same ids an identity provider would send.
func createSCIMUser(t *testing.T, ts, userName string) string {
	t.Helper()
	resp, body := doRawBytes(t, http.MethodPost, ts+"/scim/v2/Users", testSCIMToken,
		`{"userName":"`+userName+`","externalId":"ext-`+userName+`"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create user %s: status %d, body %s", userName, resp.StatusCode, body)
	}
	var u struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &u); err != nil || u.ID == "" {
		t.Fatalf("create user %s: no id in %s", userName, body)
	}
	return u.ID
}

func createSCIMGroup(t *testing.T, ts, displayName string) string {
	t.Helper()
	resp, body := doRawBytes(t, http.MethodPost, ts+"/scim/v2/Groups", testSCIMToken,
		`{"displayName":"`+displayName+`"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create group %s: status %d, body %s", displayName, resp.StatusCode, body)
	}
	var g struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &g); err != nil || g.ID == "" {
		t.Fatalf("create group %s: no id in %s", displayName, body)
	}
	return g.ID
}

// groupMemberIDs reads the member ids out of a Group resource body.
func groupMemberIDs(t *testing.T, body []byte) []string {
	t.Helper()
	var g struct {
		Members []struct {
			Value string `json:"value"`
		} `json:"members"`
	}
	if err := json.Unmarshal(body, &g); err != nil {
		t.Fatalf("decode group: %v (%s)", err, body)
	}
	ids := make([]string, 0, len(g.Members))
	for _, m := range g.Members {
		ids = append(ids, m.Value)
	}
	return ids
}

func TestSCIMGroupCreateGetList(t *testing.T) {
	ts, _ := newSCIMGateway(t)
	alice := createSCIMUser(t, ts, "alice@corp.test")

	resp, body := doRawBytes(t, http.MethodPost, ts+"/scim/v2/Groups", testSCIMToken,
		`{"displayName":"AgentOS Admins","externalId":"idp-guid-1","members":[{"value":"`+alice+`"}]}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, body %s", resp.StatusCode, body)
	}
	// Members named on create are honoured, not silently dropped — echoing an
	// empty list back while the request named members is silent data loss.
	if got := groupMemberIDs(t, body); len(got) != 1 || got[0] != alice {
		t.Errorf("created group members = %v, want [%s]", got, alice)
	}

	var created struct {
		ID   string `json:"id"`
		Meta struct {
			ResourceType string `json:"resourceType"`
			Location     string `json:"location"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(body, &created)
	if created.Meta.ResourceType != "Group" {
		t.Errorf("meta.resourceType = %q, want Group", created.Meta.ResourceType)
	}

	resp, body = doRawBytes(t, http.MethodGet, ts+"/scim/v2/Groups/"+created.ID, testSCIMToken, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d, body %s", resp.StatusCode, body)
	}

	// filter by displayName, including a name with a space — the SplitN parser
	// keeps the remainder intact where strings.Fields would truncate it.
	resp, body = doRawBytes(t, http.MethodGet,
		ts+`/scim/v2/Groups?filter=displayName+eq+%22AgentOS+Admins%22`, testSCIMToken, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("filter status = %d", resp.StatusCode)
	}
	var list struct {
		TotalResults int `json:"totalResults"`
	}
	_ = json.Unmarshal(body, &list)
	if list.TotalResults != 1 {
		t.Errorf("filter by displayName returned %d results, want 1 (%s)", list.TotalResults, body)
	}

	// An unsupported filter is an empty list, never a 400.
	resp, body = doRawBytes(t, http.MethodGet,
		ts+`/scim/v2/Groups?filter=externalId+eq+%22nope%22`, testSCIMToken, "")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("unsupported filter status = %d, want 200 with an empty list (%s)", resp.StatusCode, body)
	}
}

func TestSCIMGroupEmptyMembersKeyIsPresent(t *testing.T) {
	ts, _ := newSCIMGateway(t)
	id := createSCIMGroup(t, ts, "Empty")

	_, body := doRawBytes(t, http.MethodGet, ts+"/scim/v2/Groups/"+id, testSCIMToken, "")
	// Okta treats a missing members key on a bare GET as malformed, so an empty
	// group must render "members": [] rather than omitting it. This is what the
	// *[]Member pointer in scim.Group exists for.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := raw["members"]; !ok {
		t.Errorf("empty group omitted the members key: %s", body)
	}

	// ?excludedAttributes=members is the one case where it may be absent.
	// A FRESH map: json.Unmarshal into a populated map merges into it rather
	// than replacing it, so reusing `raw` would carry the previous members key
	// over and report a failure that is purely the test's own doing.
	var excluded map[string]json.RawMessage
	_, body = doRawBytes(t, http.MethodGet, ts+"/scim/v2/Groups/"+id+"?excludedAttributes=members", testSCIMToken, "")
	if err := json.Unmarshal(body, &excluded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := excluded["members"]; ok {
		t.Errorf("excludedAttributes=members still returned members: %s", body)
	}
}

func TestSCIMGroupDuplicateDisplayNameConflicts(t *testing.T) {
	ts, _ := newSCIMGateway(t)
	createSCIMGroup(t, ts, "Engineering")

	// Entra retries a create until it receives a 409 with scimType uniqueness;
	// any other status makes provisioning loop forever.
	resp, body := doRawBytes(t, http.MethodPost, ts+"/scim/v2/Groups", testSCIMToken,
		`{"displayName":"engineering"}`)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate create status = %d, want 409 (%s)", resp.StatusCode, body)
	}
	var e struct {
		SCIMType string `json:"scimType"`
	}
	_ = json.Unmarshal(body, &e)
	if e.SCIMType != "uniqueness" {
		t.Errorf("scimType = %q, want uniqueness (%s)", e.SCIMType, body)
	}
}

// The two identity providers do not send the same PATCH. Each form here is one
// this gateway must understand, and a table keeps them visibly distinct.
func TestSCIMGroupPatchMemberForms(t *testing.T) {
	ts, _ := newSCIMGateway(t)
	alice := createSCIMUser(t, ts, "alice@corp.test")
	bob := createSCIMUser(t, ts, "bob@corp.test")

	tests := []struct {
		name  string
		seed  []string
		patch string
		want  []string
	}{
		{
			name:  "entra add",
			patch: `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"Add","path":"members","value":[{"value":"ALICE"}]}]}`,
			want:  []string{"ALICE"},
		},
		{
			name:  "entra remove by value array",
			seed:  []string{"ALICE", "BOB"},
			patch: `{"Operations":[{"op":"Remove","path":"members","value":[{"value":"ALICE"}]}]}`,
			want:  []string{"BOB"},
		},
		{
			name:  "okta remove by path filter",
			seed:  []string{"ALICE", "BOB"},
			patch: `{"Operations":[{"op":"remove","path":"members[value eq \"ALICE\"]"}]}`,
			want:  []string{"BOB"},
		},
		{
			name:  "replace overwrites wholesale",
			seed:  []string{"ALICE"},
			patch: `{"Operations":[{"op":"replace","path":"members","value":[{"value":"BOB"}]}]}`,
			want:  []string{"BOB"},
		},
		{
			name:  "remove with no value clears all",
			seed:  []string{"ALICE", "BOB"},
			patch: `{"Operations":[{"op":"remove","path":"members"}]}`,
			want:  []string{},
		},
		{
			// Okta packs both into one array. Applying them as a set rather than
			// in order would drop ALICE, who is removed and then re-added.
			name:  "ordered remove then add in one request",
			seed:  []string{"ALICE"},
			patch: `{"Operations":[{"op":"remove","path":"members"},{"op":"add","path":"members","value":[{"value":"ALICE"},{"value":"BOB"}]}]}`,
			want:  []string{"ALICE", "BOB"},
		},
	}

	subst := func(s string) string {
		s = strings.ReplaceAll(s, "ALICE", alice)
		return strings.ReplaceAll(s, "BOB", bob)
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gid := createSCIMGroup(t, ts, "Grp"+strconv.Itoa(i))
			if len(tc.seed) > 0 {
				seed := `{"Operations":[{"op":"add","path":"members","value":[`
				for j, s := range tc.seed {
					if j > 0 {
						seed += ","
					}
					seed += `{"value":"` + s + `"}`
				}
				seed += `]}]}`
				resp, body := doRawBytes(t, http.MethodPatch, ts+"/scim/v2/Groups/"+gid, testSCIMToken, subst(seed))
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("seed status = %d (%s)", resp.StatusCode, body)
				}
			}

			resp, body := doRawBytes(t, http.MethodPatch, ts+"/scim/v2/Groups/"+gid, testSCIMToken, subst(tc.patch))
			// 200 with the resource, not 204: it keeps both PATCH handlers in
			// this package consistent and lets the assertion below read the
			// resulting membership from the same call.
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("patch status = %d, want 200 (%s)", resp.StatusCode, body)
			}
			got := groupMemberIDs(t, body)
			want := make([]string, 0, len(tc.want))
			for _, w := range tc.want {
				want = append(want, subst(w))
			}
			if !sameSet(got, want) {
				t.Errorf("members = %v, want %v (%s)", got, want, body)
			}
		})
	}
}

func TestSCIMGroupPatchRenameAndDelete(t *testing.T) {
	ts, _ := newSCIMGateway(t)
	id := createSCIMGroup(t, ts, "Old Name")

	resp, body := doRawBytes(t, http.MethodPatch, ts+"/scim/v2/Groups/"+id, testSCIMToken,
		`{"Operations":[{"op":"replace","path":"displayName","value":"New Name"}]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rename status = %d (%s)", resp.StatusCode, body)
	}
	var g struct {
		DisplayName string `json:"displayName"`
	}
	_ = json.Unmarshal(body, &g)
	if g.DisplayName != "New Name" {
		t.Errorf("displayName = %q, want New Name", g.DisplayName)
	}

	resp, _ = doRawBytes(t, http.MethodDelete, ts+"/scim/v2/Groups/"+id, testSCIMToken, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", resp.StatusCode)
	}
	resp, _ = doRawBytes(t, http.MethodGet, ts+"/scim/v2/Groups/"+id, testSCIMToken, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("get after delete status = %d, want 404", resp.StatusCode)
	}
}

func TestSCIMGroupUnknownIDAndMember(t *testing.T) {
	ts, _ := newSCIMGateway(t)
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{http.MethodGet, "/scim/v2/Groups/grp_nope", "", http.StatusNotFound},
		{http.MethodPatch, "/scim/v2/Groups/grp_nope", `{"Operations":[]}`, http.StatusNotFound},
		{http.MethodDelete, "/scim/v2/Groups/grp_nope", "", http.StatusNotFound},
	} {
		resp, _ := doRawBytes(t, tc.method, ts+tc.path, testSCIMToken, tc.body)
		if resp.StatusCode != tc.want {
			t.Errorf("%s %s = %d, want %d", tc.method, tc.path, resp.StatusCode, tc.want)
		}
	}

	// A member id that does not exist must be refused rather than stored, or
	// GroupMembers would later name a user nobody can resolve.
	gid := createSCIMGroup(t, ts, "Engineering")
	resp, body := doRawBytes(t, http.MethodPatch, ts+"/scim/v2/Groups/"+gid, testSCIMToken,
		`{"Operations":[{"op":"add","path":"members","value":[{"value":"usr_nope"}]}]}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown member status = %d, want 400 (%s)", resp.StatusCode, body)
	}
}

func TestSCIMDiscoveryAdvertisesGroups(t *testing.T) {
	ts, _ := newSCIMGateway(t)
	// An endpoint that is served but not advertised is one the IdP never calls.
	for _, path := range []string{"/scim/v2/ResourceTypes", "/scim/v2/Schemas"} {
		_, body := doRawBytes(t, http.MethodGet, ts+path, testSCIMToken, "")
		if !strings.Contains(string(body), "Group") {
			t.Errorf("%s does not mention Group: %s", path, body)
		}
	}
}
