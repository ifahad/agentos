package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/ifahad/agentos/gateway/internal/rbac"
	"github.com/ifahad/agentos/gateway/internal/store"
)

// doRawBytes issues a request and returns the response plus the raw body, for
// endpoints that return JSON arrays (doJSON decodes into a map).
func doRawBytes(t *testing.T, method, url, bearer, body string) (*http.Response, []byte) {
	t.Helper()
	resp, raw := doRaw(t, method, url, bearer, body)
	return resp, []byte(raw)
}

// mkUser bootstraps an org and one user of the given role, returning the org id
// and the user's agu- token.
func mkUser(t *testing.T, mem *store.Memory, orgName, email, role string, budget float64) (orgID, token string) {
	t.Helper()
	ctx := context.Background()
	org, err := mem.CreateOrg(ctx, orgName, budget)
	if err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}
	_, token, err = mem.CreateUser(ctx, org.ID, email, role)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return org.ID, token
}

func TestAdminAuthResolution(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	_, viewerTok := mkUser(t, mem, "acme", "v@acme.test", rbac.RoleViewer, 0)

	tests := []struct {
		name       string
		bearer     string
		wantStatus int
	}{
		{"root admin key", testAdminKey, http.StatusOK},
		{"valid user token", viewerTok, http.StatusOK},
		{"missing token", "", http.StatusUnauthorized},
		{"unknown agu token", "agu-nope", http.StatusUnauthorized},
		{"random token", "garbage", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GET /admin/usage is allowed for every role and for root.
			resp, _ := doRawBytes(t, http.MethodGet, srv.URL+"/admin/usage", tt.bearer, "")
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestCapabilityBoundaries(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	ctx := context.Background()
	org, err := mem.CreateOrg(ctx, "acme", 0)
	if err != nil {
		t.Fatal(err)
	}
	tok := func(role string) string {
		_, tk, err := mem.CreateUser(ctx, org.ID, role+"@acme.test", role)
		if err != nil {
			t.Fatalf("CreateUser %s: %v", role, err)
		}
		return tk
	}
	ownerTok := tok(rbac.RoleOwner)
	adminTok := tok(rbac.RoleAdmin)
	memberTok := tok(rbac.RoleMember)
	viewerTok := tok(rbac.RoleViewer)

	usersPath := srv.URL + "/admin/orgs/" + org.ID + "/users"

	tests := []struct {
		name       string
		method     string
		url        string
		bearer     string
		body       string
		wantStatus int
		wantType   string // "" = no error type asserted
	}{
		// create_key: member+ yes, viewer no.
		{"viewer create key forbidden", http.MethodPost, srv.URL + "/admin/keys", viewerTok, `{"name":"k"}`, http.StatusForbidden, errForbidden},
		{"member create key ok", http.MethodPost, srv.URL + "/admin/keys", memberTok, `{"name":"k"}`, http.StatusOK, ""},

		// list_keys: viewer forbidden, member ok.
		{"viewer list keys forbidden", http.MethodGet, srv.URL + "/admin/keys", viewerTok, "", http.StatusForbidden, errForbidden},
		{"member list keys ok", http.MethodGet, srv.URL + "/admin/keys", memberTok, "", http.StatusOK, ""},

		// view_usage/audit: viewer allowed.
		{"viewer view usage ok", http.MethodGet, srv.URL + "/admin/usage", viewerTok, "", http.StatusOK, ""},
		{"viewer view audit ok", http.MethodGet, srv.URL + "/admin/audit", viewerTok, "", http.StatusOK, ""},

		// create_user: member forbidden, admin ok.
		{"member create user forbidden", http.MethodPost, usersPath, memberTok, `{"email":"x@acme.test","role":"member"}`, http.StatusForbidden, errForbidden},
		{"admin create member ok", http.MethodPost, usersPath, adminTok, `{"email":"m2@acme.test","role":"member"}`, http.StatusOK, ""},
		// admin cannot create an owner (CanManageRole).
		{"admin create owner forbidden", http.MethodPost, usersPath, adminTok, `{"email":"o2@acme.test","role":"owner"}`, http.StatusForbidden, errForbidden},
		// owner can create an owner.
		{"owner create owner ok", http.MethodPost, usersPath, ownerTok, `{"email":"o3@acme.test","role":"owner"}`, http.StatusOK, ""},

		// list_users: member allowed, viewer forbidden.
		{"member list users ok", http.MethodGet, usersPath, memberTok, "", http.StatusOK, ""},
		{"viewer list users forbidden", http.MethodGet, usersPath, viewerTok, "", http.StatusForbidden, errForbidden},

		// orgs: user tokens forbidden (root-only).
		{"member create org forbidden", http.MethodPost, srv.URL + "/admin/orgs", memberTok, `{"name":"x"}`, http.StatusForbidden, errForbidden},
		{"owner list orgs forbidden", http.MethodGet, srv.URL + "/admin/orgs", ownerTok, "", http.StatusForbidden, errForbidden},

		// secrets status: user tokens forbidden (root-only).
		{"member secrets forbidden", http.MethodGet, srv.URL + "/admin/secrets/status", memberTok, "", http.StatusForbidden, errForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, raw := doRawBytes(t, tt.method, tt.url, tt.bearer, tt.body)
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d (%s)", resp.StatusCode, tt.wantStatus, raw)
			}
			if tt.wantType != "" {
				var parsed map[string]any
				_ = json.Unmarshal(raw, &parsed)
				if got := errorType(t, parsed); got != tt.wantType {
					t.Errorf("error type = %q, want %q", got, tt.wantType)
				}
			}
		})
	}
}

func TestCrossOrgAccessForbidden(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	orgA, _ := mkUser(t, mem, "a", "a@a.test", rbac.RoleOwner, 0)
	_, tokenB := mkUser(t, mem, "b", "b@b.test", rbac.RoleOwner, 0)

	// Owner of org B cannot list users of org A.
	resp, raw := doRawBytes(t, http.MethodGet, srv.URL+"/admin/orgs/"+orgA+"/users", tokenB, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (%s)", resp.StatusCode, raw)
	}
}

func TestKeyCreationViaUserTokenLandsInCallerOrg(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	orgID, memberTok := mkUser(t, mem, "acme", "m@acme.test", rbac.RoleMember, 0)

	// Member creates a key; even if they pass a different org_id it is ignored
	// only when it matches — a mismatch is forbidden. Omit org_id here.
	resp, body := doJSON(t, http.MethodPost, srv.URL+"/admin/keys", memberTok, `{"name":"agent","monthly_budget_usd":5}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if body["org_id"] != orgID {
		t.Errorf("response org_id = %v, want %q", body["org_id"], orgID)
	}
	keys, _ := mem.Keys(context.Background())
	var found bool
	for _, k := range keys {
		if k.Name == "agent" {
			found = true
			if k.OrgID != orgID {
				t.Errorf("stored key org = %q, want %q", k.OrgID, orgID)
			}
		}
	}
	if !found {
		t.Error("created key not found in store")
	}

	// Passing a foreign org_id is forbidden.
	resp2, _ := doRawBytes(t, http.MethodPost, srv.URL+"/admin/keys", memberTok, `{"name":"x","org_id":"org_other"}`)
	if resp2.StatusCode != http.StatusForbidden {
		t.Errorf("cross-org key create status = %d, want 403", resp2.StatusCode)
	}
}

func TestUsageAndKeysScoping(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	ctx := context.Background()

	orgA, tokenA := mkUser(t, mem, "a", "a@a.test", rbac.RoleAdmin, 0)
	orgB, _ := mkUser(t, mem, "b", "b@b.test", rbac.RoleAdmin, 0)

	if _, err := mem.CreateKeyIn(ctx, "ka", 100, orgA, "root"); err != nil {
		t.Fatal(err)
	}
	if _, err := mem.CreateKeyIn(ctx, "kb", 100, orgB, "root"); err != nil {
		t.Fatal(err)
	}
	if err := mem.RecordUsage(ctx, store.Usage{KeyName: "ka", CostUSD: 1, Status: 200}); err != nil {
		t.Fatal(err)
	}
	if err := mem.RecordUsage(ctx, store.Usage{KeyName: "kb", CostUSD: 2, Status: 200}); err != nil {
		t.Fatal(err)
	}

	// Root sees both keys.
	_, rootKeys := doRawBytes(t, http.MethodGet, srv.URL+"/admin/keys", testAdminKey, "")
	var rk []store.KeyInfo
	if err := json.Unmarshal(rootKeys, &rk); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(rk) != 2 {
		t.Errorf("root keys = %d, want 2", len(rk))
	}

	// Org A admin token sees only its own key.
	_, aKeys := doRawBytes(t, http.MethodGet, srv.URL+"/admin/keys", tokenA, "")
	var ak []store.KeyInfo
	if err := json.Unmarshal(aKeys, &ak); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(ak) != 1 || ak[0].Name != "ka" {
		t.Errorf("org A keys = %+v, want just ka", ak)
	}

	// Usage scoping mirrors keys.
	_, aUsage := doRawBytes(t, http.MethodGet, srv.URL+"/admin/usage", tokenA, "")
	var au []store.KeyUsage
	if err := json.Unmarshal(aUsage, &au); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(au) != 1 || au[0].Name != "ka" {
		t.Errorf("org A usage = %+v, want just ka", au)
	}
}

func TestOrgBudgetExceeded(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	ctx := context.Background()

	// Org capped at $1; the key's own budget is generous so only the org cap bites.
	org, err := mem.CreateOrg(ctx, "acme", 1)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := mem.CreateKeyIn(ctx, "agent", 100, org.ID, "root")
	if err != nil {
		t.Fatal(err)
	}
	// Push org spend past the cap.
	if err := mem.RecordUsage(ctx, store.Usage{KeyName: "agent", CostUSD: 1.5, Status: 200}); err != nil {
		t.Fatal(err)
	}

	resp, body := doJSON(t, http.MethodPost, srv.URL+"/v1/chat/completions", secret,
		`{"model":"openai/gpt-4o-mini","messages":[]}`)
	if resp.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402", resp.StatusCode)
	}
	if got := errorType(t, body); got != errOrgBudgetExceeded {
		t.Errorf("error type = %q, want %q", got, errOrgBudgetExceeded)
	}

	// Embeddings enforces the same org cap.
	resp2, body2 := doJSON(t, http.MethodPost, srv.URL+"/v1/embeddings", secret,
		`{"model":"openai/text-embedding-3-small","input":"hi"}`)
	if resp2.StatusCode != http.StatusPaymentRequired || errorType(t, body2) != errOrgBudgetExceeded {
		t.Errorf("embeddings org budget = %d %q", resp2.StatusCode, errorType(t, body2))
	}
}

func TestOrgBudgetUnlimitedDoesNotBlock(t *testing.T) {
	fake, mem, srv := newTestGateway(t)
	ctx := context.Background()

	// Budget 0 = unlimited (the default-org semantics).
	org, err := mem.CreateOrg(ctx, "acme", 0)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := mem.CreateKeyIn(ctx, "agent", 1000, org.ID, "root")
	if err != nil {
		t.Fatal(err)
	}
	if err := mem.RecordUsage(ctx, store.Usage{KeyName: "agent", CostUSD: 500, Status: 200}); err != nil {
		t.Fatal(err)
	}
	resp, _ := doJSON(t, http.MethodPost, srv.URL+"/v1/chat/completions", secret,
		`{"model":"openai/gpt-4o-mini","messages":[]}`)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 (unlimited org)", resp.StatusCode)
	}
	if fake.lastBody == nil {
		t.Error("request was not forwarded upstream")
	}
}

func TestSecretsStatusShape(t *testing.T) {
	fakeSrc := &stubSecrets{
		present: map[string]bool{"AGENTOS_ANTHROPIC_API_KEY": true, "AGENTOS_OPENAI_API_KEY": false},
		backend: "file",
	}
	_, _, srv := newTestGateway(t, WithSecrets(fakeSrc, DefaultSecretNames))

	// Root sees the status table; values are never present.
	_, raw := doRawBytes(t, http.MethodGet, srv.URL+"/admin/secrets/status", testAdminKey, "")
	var status []secretStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, raw)
	}
	if len(status) != 2 {
		t.Fatalf("status rows = %d, want 2", len(status))
	}
	byName := map[string]secretStatus{}
	for _, s := range status {
		byName[s.Name] = s
		if s.Source != "file" {
			t.Errorf("%s source = %q, want file", s.Name, s.Source)
		}
	}
	if !byName["AGENTOS_ANTHROPIC_API_KEY"].Present {
		t.Error("anthropic key should be present")
	}
	if byName["AGENTOS_OPENAI_API_KEY"].Present {
		t.Error("openai key should be absent")
	}
	if strings.Contains(string(raw), "value") || strings.Contains(string(raw), "sk-") {
		t.Errorf("secrets status leaked a value: %s", raw)
	}
}

// stubSecrets is a controllable secret.Source for tests.
type stubSecrets struct {
	present map[string]bool
	backend string
}

func (s *stubSecrets) Get(name string) (string, bool) {
	if s.present[name] {
		return "REDACTED", true
	}
	return "", false
}

func (s *stubSecrets) Backend() string { return s.backend }
