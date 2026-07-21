package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/ifahad/agentos/gateway/internal/provider"
	"github.com/ifahad/agentos/gateway/internal/ratelimit"
	"github.com/ifahad/agentos/gateway/internal/rbac"
	"github.com/ifahad/agentos/gateway/internal/store"
)

// newHTTPServer starts an httptest.Server for handler and returns its URL.
func newHTTPServer(t *testing.T, handler http.Handler) string {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts.URL
}

// --- whoami ---

func TestWhoamiRoot(t *testing.T) {
	_, _, srv := newTestGateway(t)
	resp, body := doJSON(t, http.MethodGet, srv.URL+"/admin/whoami", testAdminKey, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if body["root"] != true {
		t.Errorf("root = %v, want true", body["root"])
	}
	if _, ok := body["user_id"]; ok {
		t.Errorf("root whoami leaked user fields: %v", body)
	}
}

func TestWhoamiUser(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	orgID, token := mkUser(t, mem, "acme", "u@acme.test", rbac.RoleMember, 0)

	resp, body := doJSON(t, http.MethodGet, srv.URL+"/admin/whoami", token, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if body["root"] != false {
		t.Errorf("root = %v, want false", body["root"])
	}
	if body["org_id"] != orgID {
		t.Errorf("org_id = %v, want %q", body["org_id"], orgID)
	}
	if body["email"] != "u@acme.test" {
		t.Errorf("email = %v, want u@acme.test", body["email"])
	}
	if body["role"] != rbac.RoleMember {
		t.Errorf("role = %v, want member", body["role"])
	}
	if _, ok := body["user_id"].(string); !ok || body["user_id"] == "" {
		t.Errorf("user_id = %v, want non-empty", body["user_id"])
	}
}

func TestWhoamiUnauthenticated(t *testing.T) {
	_, _, srv := newTestGateway(t)
	resp, body := doJSON(t, http.MethodGet, srv.URL+"/admin/whoami", "", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	if got := errorType(t, body); got != errInvalidKey {
		t.Errorf("error type = %q, want invalid_key", got)
	}
}

// --- PATCH /admin/orgs/{id} + POST rate_limit_rpm ---

func TestCreateOrgWithRateLimit(t *testing.T) {
	_, _, srv := newTestGateway(t)
	resp, body := doJSON(t, http.MethodPost, srv.URL+"/admin/orgs", testAdminKey,
		`{"name":"acme","monthly_budget_usd":50,"rate_limit_rpm":120}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if body["rate_limit_rpm"].(float64) != 120 {
		t.Errorf("rate_limit_rpm = %v, want 120", body["rate_limit_rpm"])
	}
}

func TestPatchOrgRateLimitAndBudget(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	org, err := mem.CreateOrg(context.Background(), "acme", 0)
	if err != nil {
		t.Fatal(err)
	}

	// Root patches both fields.
	resp, body := doJSON(t, http.MethodPatch, srv.URL+"/admin/orgs/"+org.ID, testAdminKey,
		`{"monthly_budget_usd":10,"rate_limit_rpm":30}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%v)", resp.StatusCode, body)
	}
	if body["rate_limit_rpm"].(float64) != 30 || body["monthly_budget_usd"].(float64) != 10 {
		t.Errorf("patched org = %v", body)
	}

	// Nil field leaves the other unchanged: patch only the budget.
	resp2, body2 := doJSON(t, http.MethodPatch, srv.URL+"/admin/orgs/"+org.ID, testAdminKey,
		`{"monthly_budget_usd":99}`)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp2.StatusCode)
	}
	if body2["rate_limit_rpm"].(float64) != 30 {
		t.Errorf("rate_limit_rpm changed unexpectedly: %v", body2["rate_limit_rpm"])
	}
	if body2["monthly_budget_usd"].(float64) != 99 {
		t.Errorf("monthly_budget_usd = %v, want 99", body2["monthly_budget_usd"])
	}
}

func TestPatchOrgOwnerAllowedMemberForbidden(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	ctx := context.Background()
	org, err := mem.CreateOrg(ctx, "acme", 0)
	if err != nil {
		t.Fatal(err)
	}
	_, ownerTok, err := mem.CreateUser(ctx, org.ID, "o@acme.test", rbac.RoleOwner)
	if err != nil {
		t.Fatal(err)
	}
	_, memberTok, err := mem.CreateUser(ctx, org.ID, "m@acme.test", rbac.RoleMember)
	if err != nil {
		t.Fatal(err)
	}

	// Owner may patch their own org.
	resp, _ := doRawBytes(t, http.MethodPatch, srv.URL+"/admin/orgs/"+org.ID, ownerTok, `{"rate_limit_rpm":15}`)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("owner patch status = %d, want 200", resp.StatusCode)
	}
	// Member lacks update_org.
	resp2, _ := doRawBytes(t, http.MethodPatch, srv.URL+"/admin/orgs/"+org.ID, memberTok, `{"rate_limit_rpm":15}`)
	if resp2.StatusCode != http.StatusForbidden {
		t.Errorf("member patch status = %d, want 403", resp2.StatusCode)
	}
}

func TestPatchOrgCrossOrgForbidden(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	orgA, _ := mkUser(t, mem, "a", "a@a.test", rbac.RoleOwner, 0)
	_, tokenB := mkUser(t, mem, "b", "b@b.test", rbac.RoleOwner, 0)

	resp, _ := doRawBytes(t, http.MethodPatch, srv.URL+"/admin/orgs/"+orgA, tokenB, `{"rate_limit_rpm":5}`)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-org patch status = %d, want 403", resp.StatusCode)
	}
}

func TestPatchOrgNotFound(t *testing.T) {
	_, _, srv := newTestGateway(t)
	resp, body := doJSON(t, http.MethodPatch, srv.URL+"/admin/orgs/org_missing", testAdminKey, `{"rate_limit_rpm":5}`)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if got := errorType(t, body); got != errNotFound {
		t.Errorf("error type = %q, want not_found", got)
	}
}

// --- per-tenant rate limiting on the proxy path ---

// newRateLimitedGateway builds a gateway with an injected, frozen-clock limiter
// so token refill is deterministic during the test.
func newRateLimitedGateway(t *testing.T, now func() time.Time) (*fakeProvider, *store.Memory, string, string) {
	t.Helper()
	fake := newFakeProvider(t)
	mem := store.NewMemory()
	router := &provider.Router{
		AnthropicBaseURL: fake.server.URL,
		OpenAIBaseURL:    fake.server.URL,
		OllamaBaseURL:    fake.server.URL,
		AnthropicAPIKey:  "anthropic-key",
		OpenAIAPIKey:     "openai-key",
	}
	limiter := ratelimit.NewWithClock(now)
	srvHandler := New(mem, router, testAdminKey, WithRateLimiter(limiter)).Handler()
	ts := newHTTPServer(t, srvHandler)

	ctx := context.Background()
	org, err := mem.CreateOrg(ctx, "acme", 0)
	if err != nil {
		t.Fatal(err)
	}
	rpm := 2
	if _, err := mem.UpdateOrg(ctx, org.ID, nil, &rpm); err != nil {
		t.Fatal(err)
	}
	secret, err := mem.CreateKeyIn(ctx, "agent", 1000, org.ID, "root")
	if err != nil {
		t.Fatal(err)
	}
	return fake, mem, ts, secret
}

func TestRateLimitRejectsAndAudits(t *testing.T) {
	now := time.Unix(0, 0)
	_, mem, url, secret := newRateLimitedGateway(t, func() time.Time { return now })

	body := `{"model":"openai/gpt-4o-mini","messages":[]}`
	// Burst = rpm = 2 allowed.
	for i := 0; i < 2; i++ {
		resp, _ := doJSON(t, http.MethodPost, url+"/v1/chat/completions", secret, body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200", i, resp.StatusCode)
		}
	}
	// Third is rate limited.
	resp, respBody := doJSON(t, http.MethodPost, url+"/v1/chat/completions", secret, body)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", resp.StatusCode)
	}
	if got := errorType(t, respBody); got != errRateLimited {
		t.Errorf("error type = %q, want rate_limited", got)
	}
	ra := resp.Header.Get("Retry-After")
	if n, err := strconv.Atoi(ra); err != nil || n < 1 {
		t.Errorf("Retry-After = %q, want positive integer", ra)
	}

	// The rejection is audited with the rate_limited kind, and no spend recorded.
	var rateLimitedAudits int
	for _, u := range mem.Audit() {
		if u.Kind == store.KindRateLimited {
			rateLimitedAudits++
			if u.CostUSD != 0 {
				t.Errorf("rate_limited audit recorded spend %v", u.CostUSD)
			}
		}
	}
	if rateLimitedAudits != 1 {
		t.Errorf("rate_limited audits = %d, want 1", rateLimitedAudits)
	}
}

func TestRateLimitEnforcedOnEmbeddings(t *testing.T) {
	now := time.Unix(0, 0)
	_, _, url, secret := newRateLimitedGateway(t, func() time.Time { return now })
	body := `{"model":"openai/text-embedding-3-small","input":"hi"}`
	for i := 0; i < 2; i++ {
		if resp, _ := doJSON(t, http.MethodPost, url+"/v1/embeddings", secret, body); resp.StatusCode != http.StatusOK {
			t.Fatalf("embeddings %d status = %d, want 200", i, resp.StatusCode)
		}
	}
	resp, _ := doJSON(t, http.MethodPost, url+"/v1/embeddings", secret, body)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("embeddings status = %d, want 429", resp.StatusCode)
	}
}

func TestRateLimitRefillAllowsAgain(t *testing.T) {
	now := time.Unix(0, 0)
	clock := func() time.Time { return now }
	_, _, url, secret := newRateLimitedGateway(t, clock)
	body := `{"model":"openai/gpt-4o-mini","messages":[]}`

	for i := 0; i < 2; i++ {
		doJSON(t, http.MethodPost, url+"/v1/chat/completions", secret, body)
	}
	if resp, _ := doJSON(t, http.MethodPost, url+"/v1/chat/completions", secret, body); resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("pre-refill status = %d, want 429", resp.StatusCode)
	}
	// rpm=2 → one token every 30s; advance the clock past that.
	now = now.Add(31 * time.Second)
	if resp, _ := doJSON(t, http.MethodPost, url+"/v1/chat/completions", secret, body); resp.StatusCode != http.StatusOK {
		t.Errorf("post-refill status = %d, want 200", resp.StatusCode)
	}
}

func TestNoRateLimitByDefault(t *testing.T) {
	// Default gateway (no default rpm, org rpm 0) never rate limits.
	fake, mem, srv := newTestGateway(t)
	ctx := context.Background()
	org, err := mem.CreateOrg(ctx, "acme", 0)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := mem.CreateKeyIn(ctx, "agent", 1000, org.ID, "root")
	if err != nil {
		t.Fatal(err)
	}
	body := `{"model":"openai/gpt-4o-mini","messages":[]}`
	for i := 0; i < 20; i++ {
		if resp, _ := doJSON(t, http.MethodPost, srv.URL+"/v1/chat/completions", secret, body); resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200 (no limit)", i, resp.StatusCode)
		}
	}
	if fake.lastBody == nil {
		t.Error("requests were not forwarded")
	}
}

func TestGlobalDefaultRPMApplies(t *testing.T) {
	fake := newFakeProvider(t)
	mem := store.NewMemory()
	router := &provider.Router{
		AnthropicBaseURL: fake.server.URL, OpenAIBaseURL: fake.server.URL, OllamaBaseURL: fake.server.URL,
		AnthropicAPIKey: "k", OpenAIAPIKey: "k",
	}
	now := time.Unix(0, 0)
	limiter := ratelimit.NewWithClock(func() time.Time { return now })
	// Global default rpm=1, org opts out (rpm 0) → default applies.
	ts := newHTTPServer(t, New(mem, router, testAdminKey, WithRateLimits(1), WithRateLimiter(limiter)).Handler())

	ctx := context.Background()
	org, _ := mem.CreateOrg(ctx, "acme", 0)
	secret, _ := mem.CreateKeyIn(ctx, "agent", 1000, org.ID, "root")
	body := `{"model":"openai/gpt-4o-mini","messages":[]}`

	if resp, _ := doJSON(t, http.MethodPost, ts+"/v1/chat/completions", secret, body); resp.StatusCode != http.StatusOK {
		t.Fatalf("first status = %d, want 200", resp.StatusCode)
	}
	if resp, b := doJSON(t, http.MethodPost, ts+"/v1/chat/completions", secret, body); resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("second status = %d (%v), want 429", resp.StatusCode, b)
	}
}

func TestOrgRPMOverridesGlobalDefault(t *testing.T) {
	fake := newFakeProvider(t)
	mem := store.NewMemory()
	router := &provider.Router{
		AnthropicBaseURL: fake.server.URL, OpenAIBaseURL: fake.server.URL, OllamaBaseURL: fake.server.URL,
		AnthropicAPIKey: "k", OpenAIAPIKey: "k",
	}
	now := time.Unix(0, 0)
	limiter := ratelimit.NewWithClock(func() time.Time { return now })
	// Global default rpm=1 but org sets rpm=5 → org value wins (5 allowed).
	ts := newHTTPServer(t, New(mem, router, testAdminKey, WithRateLimits(1), WithRateLimiter(limiter)).Handler())

	ctx := context.Background()
	org, _ := mem.CreateOrg(ctx, "acme", 0)
	rpm := 5
	mem.UpdateOrg(ctx, org.ID, nil, &rpm)
	secret, _ := mem.CreateKeyIn(ctx, "agent", 1000, org.ID, "root")
	body := `{"model":"openai/gpt-4o-mini","messages":[]}`

	for i := 0; i < 5; i++ {
		if resp, _ := doJSON(t, http.MethodPost, ts+"/v1/chat/completions", secret, body); resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200 (org rpm=5)", i, resp.StatusCode)
		}
	}
	if resp, _ := doJSON(t, http.MethodPost, ts+"/v1/chat/completions", secret, body); resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("6th status = %d, want 429", resp.StatusCode)
	}
}

// orgFields decodes the org JSON returned by list to confirm the field is
// surfaced in GET /admin/orgs.
func TestListOrgsSurfacesRateLimit(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	ctx := context.Background()
	org, _ := mem.CreateOrg(ctx, "acme", 0)
	rpm := 42
	mem.UpdateOrg(ctx, org.ID, nil, &rpm)

	_, raw := doRawBytes(t, http.MethodGet, srv.URL+"/admin/orgs", testAdminKey, "")
	var orgs []map[string]any
	if err := json.Unmarshal(raw, &orgs); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, raw)
	}
	var found bool
	for _, o := range orgs {
		if o["id"] == org.ID {
			found = true
			if o["rate_limit_rpm"].(float64) != 42 {
				t.Errorf("rate_limit_rpm = %v, want 42", o["rate_limit_rpm"])
			}
		}
	}
	if !found {
		t.Error("created org not in list")
	}
}
