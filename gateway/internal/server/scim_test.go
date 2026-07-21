package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/ifahad/agentos/gateway/internal/provider"
	"github.com/ifahad/agentos/gateway/internal/rbac"
	"github.com/ifahad/agentos/gateway/internal/store"
)

const testSCIMToken = "scim-secret-token"

// newSCIMGateway builds a gateway with SCIM enabled and the default org
// bootstrapped, returning the base URL and the memory store.
func newSCIMGateway(t *testing.T) (string, *store.Memory) {
	t.Helper()
	fake := newFakeProvider(t)
	mem := store.NewMemory()
	if err := mem.EnsureOrg(context.Background(), store.DefaultOrgID, "default", 0); err != nil {
		t.Fatalf("EnsureOrg: %v", err)
	}
	router := &provider.Router{
		AnthropicBaseURL: fake.server.URL, OpenAIBaseURL: fake.server.URL, OllamaBaseURL: fake.server.URL,
		AnthropicAPIKey: "k", OpenAIAPIKey: "k",
	}
	ts := newHTTPServer(t, New(mem, router, testAdminKey, WithSCIM(testSCIMToken, "", "")).Handler())
	return ts, mem
}

func TestSCIMDisabledReturns404(t *testing.T) {
	// A gateway without WithSCIM must 404 every /scim/v2/* route.
	_, _, srv := newTestGateway(t)
	for _, path := range []string{"/scim/v2/Users", "/scim/v2/Users/usr_x", "/scim/v2/ServiceProviderConfig"} {
		resp, _ := doRawBytes(t, http.MethodGet, srv.URL+path, testSCIMToken, "")
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404 when SCIM disabled", path, resp.StatusCode)
		}
	}
}

func TestSCIMRequiresBearer(t *testing.T) {
	ts, _ := newSCIMGateway(t)
	// Missing token → 401 SCIM error.
	resp, body := doJSON(t, http.MethodGet, ts+"/scim/v2/Users", "", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no-token status = %d, want 401", resp.StatusCode)
	}
	assertSCIMError(t, body)
	// Wrong token → 401.
	resp, _ = doJSON(t, http.MethodGet, ts+"/scim/v2/Users", "wrong-token", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong-token status = %d, want 401", resp.StatusCode)
	}
}

func TestSCIMCreateGetListFlow(t *testing.T) {
	ts, _ := newSCIMGateway(t)

	// POST create → 201 with the User resource.
	resp, body := doJSON(t, http.MethodPost, ts+"/scim/v2/Users", testSCIMToken,
		`{"userName":"alice@corp.test","externalId":"idp-alice","active":true}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (%v)", resp.StatusCode, body)
	}
	assertSchema(t, body, "urn:ietf:params:scim:schemas:core:2.0:User")
	if body["userName"] != "alice@corp.test" {
		t.Errorf("userName = %v", body["userName"])
	}
	if body["externalId"] != "idp-alice" {
		t.Errorf("externalId = %v", body["externalId"])
	}
	if active, _ := body["active"].(bool); !active {
		t.Errorf("active = %v, want true", body["active"])
	}
	id, _ := body["id"].(string)
	if id == "" {
		t.Fatal("no id in create response")
	}
	if emails, ok := body["emails"].([]any); !ok || len(emails) != 1 {
		t.Errorf("emails = %v", body["emails"])
	}

	// Duplicate active → 409.
	resp, body = doJSON(t, http.MethodPost, ts+"/scim/v2/Users", testSCIMToken,
		`{"userName":"alice@corp.test"}`)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want 409", resp.StatusCode)
	}
	assertSCIMError(t, body)

	// GET /{id} → 200.
	resp, body = doJSON(t, http.MethodGet, ts+"/scim/v2/Users/"+id, testSCIMToken, "")
	if resp.StatusCode != http.StatusOK || body["id"] != id {
		t.Fatalf("get status = %d, id = %v", resp.StatusCode, body["id"])
	}
	// Unknown id → 404.
	resp, body = doJSON(t, http.MethodGet, ts+"/scim/v2/Users/usr_missing", testSCIMToken, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("get missing status = %d, want 404", resp.StatusCode)
	}
	assertSCIMError(t, body)

	// List (no filter) → ListResponse with 1 result.
	resp, body = doJSON(t, http.MethodGet, ts+"/scim/v2/Users", testSCIMToken, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d", resp.StatusCode)
	}
	assertSchema(t, body, "urn:ietf:params:scim:api:messages:2.0:ListResponse")
	if tr, _ := body["totalResults"].(float64); tr != 1 {
		t.Errorf("totalResults = %v, want 1", body["totalResults"])
	}

	// Filtered list userName eq → 1 match; a non-matching value → 0.
	resp, body = doJSON(t, http.MethodGet, ts+`/scim/v2/Users?filter=userName%20eq%20%22alice@corp.test%22`, testSCIMToken, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("filter status = %d", resp.StatusCode)
	}
	if tr, _ := body["totalResults"].(float64); tr != 1 {
		t.Errorf("filtered totalResults = %v, want 1", body["totalResults"])
	}
	_, body = doJSON(t, http.MethodGet, ts+`/scim/v2/Users?filter=userName%20eq%20%22nobody@corp.test%22`, testSCIMToken, "")
	if tr, _ := body["totalResults"].(float64); tr != 0 {
		t.Errorf("no-match totalResults = %v, want 0", body["totalResults"])
	}
}

// TestSCIMDeactivationInvalidatesToken proves both PATCH forms deactivate a user
// and that a deactivated user's agu- token stops authenticating.
func TestSCIMDeactivationInvalidatesToken(t *testing.T) {
	for _, form := range []struct {
		name string
		body string
	}{
		{"path form", `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"active","value":false}]}`},
		{"pathless form", `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","value":{"active":false}}]}`},
		{"top-level pathless", `{"op":"replace","value":{"active":false}}`},
	} {
		t.Run(form.name, func(t *testing.T) {
			ts, mem := newSCIMGateway(t)
			ctx := context.Background()
			// Create a user with a known token via the store.
			user, token, err := mem.CreateUser(ctx, store.DefaultOrgID, "bob@corp.test", rbac.RoleMember)
			if err != nil {
				t.Fatalf("CreateUser: %v", err)
			}
			// The agu- token works on an admin endpoint before deactivation.
			if resp, _ := doJSON(t, http.MethodGet, ts+"/admin/whoami", token, ""); resp.StatusCode != http.StatusOK {
				t.Fatalf("pre-deactivation whoami = %d, want 200", resp.StatusCode)
			}

			// Deactivate via SCIM PATCH.
			resp, body := doJSON(t, http.MethodPatch, ts+"/scim/v2/Users/"+user.ID, testSCIMToken, form.body)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("patch status = %d (%v)", resp.StatusCode, body)
			}
			if active, _ := body["active"].(bool); active {
				t.Errorf("post-patch active = true, want false")
			}

			// The agu- token now stops working (401).
			if resp, _ := doJSON(t, http.MethodGet, ts+"/admin/whoami", token, ""); resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("post-deactivation whoami = %d, want 401", resp.StatusCode)
			}
		})
	}
}

func TestSCIMPutAndDelete(t *testing.T) {
	ts, mem := newSCIMGateway(t)
	ctx := context.Background()
	user, _, err := mem.CreateUserWithExternalID(ctx, store.DefaultOrgID, "carol@corp.test", rbac.RoleMember, "old-ext")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// PUT replaces active and externalId.
	resp, body := doJSON(t, http.MethodPut, ts+"/scim/v2/Users/"+user.ID, testSCIMToken,
		`{"userName":"carol@corp.test","externalId":"new-ext","active":false}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put status = %d (%v)", resp.StatusCode, body)
	}
	if body["externalId"] != "new-ext" {
		t.Errorf("externalId = %v, want new-ext", body["externalId"])
	}
	if active, _ := body["active"].(bool); active {
		t.Errorf("active = true, want false after PUT")
	}

	// DELETE → 204, then GET → 404.
	resp, _ = doJSON(t, http.MethodDelete, ts+"/scim/v2/Users/"+user.ID, testSCIMToken, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodGet, ts+"/scim/v2/Users/"+user.ID, testSCIMToken, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", resp.StatusCode)
	}
	// DELETE unknown → 404.
	resp, _ = doJSON(t, http.MethodDelete, ts+"/scim/v2/Users/usr_missing", testSCIMToken, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("delete missing = %d, want 404", resp.StatusCode)
	}
}

// TestSCIMReactivateViaPost verifies POST on a deactivated user reactivates it
// (201) rather than 409.
func TestSCIMReactivateViaPost(t *testing.T) {
	ts, mem := newSCIMGateway(t)
	ctx := context.Background()
	user, _, err := mem.CreateUser(ctx, store.DefaultOrgID, "dave@corp.test", rbac.RoleMember)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := mem.SetUserActive(ctx, user.ID, false); err != nil {
		t.Fatalf("SetUserActive: %v", err)
	}
	resp, body := doJSON(t, http.MethodPost, ts+"/scim/v2/Users", testSCIMToken,
		`{"userName":"dave@corp.test","externalId":"idp-dave"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("reactivate status = %d, want 201 (%v)", resp.StatusCode, body)
	}
	if active, _ := body["active"].(bool); !active {
		t.Errorf("reactivated active = %v, want true", body["active"])
	}
	if body["externalId"] != "idp-dave" {
		t.Errorf("externalId = %v, want idp-dave", body["externalId"])
	}
}

func TestSCIMDiscoveryStubs(t *testing.T) {
	ts, _ := newSCIMGateway(t)
	for _, tc := range []struct {
		path   string
		schema string
	}{
		{"/scim/v2/ServiceProviderConfig", "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		{"/scim/v2/ResourceTypes", "urn:ietf:params:scim:api:messages:2.0:ListResponse"},
		{"/scim/v2/Schemas", "urn:ietf:params:scim:api:messages:2.0:ListResponse"},
	} {
		resp, body := doJSON(t, http.MethodGet, ts+tc.path, testSCIMToken, "")
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s status = %d, want 200", tc.path, resp.StatusCode)
		}
		assertSchema(t, body, tc.schema)
	}
	// Discovery still requires the bearer.
	if resp, _ := doJSON(t, http.MethodGet, ts+"/scim/v2/Schemas", "", ""); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauth discovery = %d, want 401", resp.StatusCode)
	}
}

// TestAdminUsersListingIncludesActiveAndExternalID verifies the additive fields
// on the existing GET /admin/orgs/{org}/users response.
func TestAdminUsersListingIncludesActiveAndExternalID(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	ctx := context.Background()
	if err := mem.EnsureOrg(ctx, store.DefaultOrgID, "default", 0); err != nil {
		t.Fatal(err)
	}
	if _, _, err := mem.CreateUserWithExternalID(ctx, store.DefaultOrgID, "erin@corp.test", rbac.RoleMember, "ext-erin"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	resp, raw := doRawBytes(t, http.MethodGet, srv.URL+"/admin/orgs/"+store.DefaultOrgID+"/users", testAdminKey, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list users status = %d", resp.StatusCode)
	}
	var users []map[string]any
	if err := json.Unmarshal(raw, &users); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, raw)
	}
	if len(users) != 1 {
		t.Fatalf("users = %d, want 1", len(users))
	}
	if _, ok := users[0]["active"]; !ok {
		t.Error("users listing missing additive field: active")
	}
	if users[0]["external_id"] != "ext-erin" {
		t.Errorf("external_id = %v, want ext-erin", users[0]["external_id"])
	}
	if active, _ := users[0]["active"].(bool); !active {
		t.Errorf("active = %v, want true", users[0]["active"])
	}
}

// --- assertions ----------------------------------------------------------

func assertSchema(t *testing.T, body map[string]any, want string) {
	t.Helper()
	schemas, ok := body["schemas"].([]any)
	if !ok || len(schemas) == 0 {
		t.Fatalf("no schemas in %v", body)
	}
	for _, s := range schemas {
		if s == want {
			return
		}
	}
	t.Errorf("schemas = %v, want to contain %q", schemas, want)
}

func assertSCIMError(t *testing.T, body map[string]any) {
	t.Helper()
	assertSchema(t, body, "urn:ietf:params:scim:api:messages:2.0:Error")
	if _, ok := body["detail"]; !ok {
		t.Errorf("SCIM error missing detail: %v", body)
	}
	if _, ok := body["status"]; !ok {
		t.Errorf("SCIM error missing status: %v", body)
	}
}
