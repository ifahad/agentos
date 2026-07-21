package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ifahad/agentos/gateway/internal/provider"
	"github.com/ifahad/agentos/gateway/internal/store"
)

const testAdminKey = "admin-secret"

// fakeProvider is an httptest-backed upstream that records the last request
// and serves a configurable response.
type fakeProvider struct {
	server       *httptest.Server
	lastBody     map[string]any
	lastAuth     string
	responseCode int
	responseBody string
}

func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()
	f := &fakeProvider{
		responseCode: http.StatusOK,
		responseBody: `{"id":"chatcmpl-1","choices":[{"message":{"role":"assistant","content":"hi"}}],"usage":{"prompt_tokens":100,"completion_tokens":50}}`,
	}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		f.lastBody = nil
		_ = json.Unmarshal(raw, &f.lastBody)
		f.lastAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(f.responseCode)
		io.WriteString(w, f.responseBody)
	}))
	t.Cleanup(f.server.Close)
	return f
}

// newTestGateway wires a memory store and a Server whose provider base URLs
// all point at the fake provider. Returns the gateway test server too.
func newTestGateway(t *testing.T) (*fakeProvider, *store.Memory, *httptest.Server) {
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
	srv := httptest.NewServer(New(mem, router, testAdminKey).Handler())
	t.Cleanup(srv.Close)
	return fake, mem, srv
}

func doJSON(t *testing.T, method, url, bearer, body string) (*http.Response, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	var parsed map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatalf("response is not JSON: %v (%s)", err, raw)
		}
	}
	return resp, parsed
}

func errorType(t *testing.T, body map[string]any) string {
	t.Helper()
	e, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("no error object in %v", body)
	}
	typ, _ := e["type"].(string)
	return typ
}

func errorMessage(t *testing.T, body map[string]any) string {
	t.Helper()
	e, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("no error object in %v", body)
	}
	msg, _ := e["message"].(string)
	return msg
}

func createKey(t *testing.T, mem *store.Memory, name string, budget float64) string {
	t.Helper()
	secret, err := mem.CreateKey(context.Background(), name, budget)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	return secret
}

func TestHealthz(t *testing.T) {
	_, _, srv := newTestGateway(t)
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Errorf("healthz = %d %q, want 200 ok", resp.StatusCode, body)
	}
}

func TestChatCompletionsAuthAndValidation(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	secret := createKey(t, mem, "agent", 25)

	tests := []struct {
		name       string
		bearer     string
		body       string
		wantStatus int
		wantType   string
	}{
		{
			name:       "missing auth header",
			body:       `{"model":"openai/gpt-4o-mini","messages":[]}`,
			wantStatus: http.StatusUnauthorized,
			wantType:   "invalid_key",
		},
		{
			name:       "unknown key",
			bearer:     "agos-not-a-key",
			body:       `{"model":"openai/gpt-4o-mini","messages":[]}`,
			wantStatus: http.StatusUnauthorized,
			wantType:   "invalid_key",
		},
		{
			name:       "stream requested",
			bearer:     secret,
			body:       `{"model":"openai/gpt-4o-mini","messages":[],"stream":true}`,
			wantStatus: http.StatusBadRequest,
			wantType:   "unsupported",
		},
		{
			name:       "missing model",
			bearer:     secret,
			body:       `{"messages":[]}`,
			wantStatus: http.StatusBadRequest,
			wantType:   "unsupported",
		},
		{
			name:       "unknown provider prefix",
			bearer:     secret,
			body:       `{"model":"mistral/mistral-7b","messages":[]}`,
			wantStatus: http.StatusBadRequest,
			wantType:   "unsupported",
		},
		{
			name:       "invalid json",
			bearer:     secret,
			body:       `{not json`,
			wantStatus: http.StatusBadRequest,
			wantType:   "unsupported",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := doJSON(t, http.MethodPost, srv.URL+"/v1/chat/completions", tt.bearer, tt.body)
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if got := errorType(t, body); got != tt.wantType {
				t.Errorf("error type = %q, want %q", got, tt.wantType)
			}
		})
	}
}

func TestChatCompletionsBudgetExceeded(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	secret := createKey(t, mem, "broke", 1)
	// Push spend past the budget.
	if err := mem.RecordUsage(context.Background(), store.Usage{KeyName: "broke", CostUSD: 1.0, Status: 200}); err != nil {
		t.Fatal(err)
	}

	resp, body := doJSON(t, http.MethodPost, srv.URL+"/v1/chat/completions", secret,
		`{"model":"openai/gpt-4o-mini","messages":[]}`)
	if resp.StatusCode != http.StatusPaymentRequired {
		t.Errorf("status = %d, want 402", resp.StatusCode)
	}
	if got := errorType(t, body); got != "budget_exceeded" {
		t.Errorf("error type = %q, want budget_exceeded", got)
	}
}

func TestChatCompletionsProxySuccess(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		wantModel     string
		wantAuth      string
		respBody      string
		wantInput     int64
		wantOutput    int64
		wantSpendUSD  float64
		wantSpendZero bool
	}{
		{
			name:         "openai route with prompt/completion tokens",
			model:        "openai/gpt-4o-mini",
			wantModel:    "gpt-4o-mini",
			wantAuth:     "Bearer openai-key",
			respBody:     `{"choices":[],"usage":{"prompt_tokens":1000000,"completion_tokens":1000000}}`,
			wantInput:    1000000,
			wantOutput:   1000000,
			wantSpendUSD: 0.75,
		},
		{
			name:         "anthropic route with input/output token fallback",
			model:        "anthropic/claude-sonnet-5",
			wantModel:    "claude-sonnet-5",
			wantAuth:     "Bearer anthropic-key",
			respBody:     `{"choices":[],"usage":{"input_tokens":1000,"output_tokens":2000}}`,
			wantInput:    1000,
			wantOutput:   2000,
			wantSpendUSD: 0.033,
		},
		{
			name:          "ollama route is free and unauthenticated",
			model:         "ollama/llama3.1",
			wantModel:     "llama3.1",
			wantAuth:      "",
			respBody:      `{"choices":[],"usage":{"prompt_tokens":500,"completion_tokens":500}}`,
			wantInput:     500,
			wantOutput:    500,
			wantSpendZero: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake, mem, srv := newTestGateway(t)
			secret := createKey(t, mem, "agent", 100)
			fake.responseBody = tt.respBody

			resp, body := doJSON(t, http.MethodPost, srv.URL+"/v1/chat/completions", secret,
				`{"model":"`+tt.model+`","messages":[{"role":"user","content":"hi"}]}`)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, body %v", resp.StatusCode, body)
			}
			// Passthrough: gateway must return the provider body unchanged.
			if _, ok := body["usage"]; !ok {
				t.Errorf("provider body not passed through: %v", body)
			}
			// Prefix stripped before forwarding.
			if got := fake.lastBody["model"]; got != tt.wantModel {
				t.Errorf("forwarded model = %v, want %q", got, tt.wantModel)
			}
			if fake.lastAuth != tt.wantAuth {
				t.Errorf("forwarded auth = %q, want %q", fake.lastAuth, tt.wantAuth)
			}

			// Usage recorded with tokens, cost, latency, status.
			audit := mem.Audit()
			if len(audit) != 1 {
				t.Fatalf("audit entries = %d, want 1", len(audit))
			}
			entry := audit[0]
			if entry.KeyName != "agent" || entry.Model != tt.model || entry.Status != 200 {
				t.Errorf("audit entry = %+v", entry)
			}
			if entry.InputTokens != tt.wantInput || entry.OutputTokens != tt.wantOutput {
				t.Errorf("tokens = %d/%d, want %d/%d", entry.InputTokens, entry.OutputTokens, tt.wantInput, tt.wantOutput)
			}
			if entry.LatencyMS < 0 {
				t.Errorf("latency = %d", entry.LatencyMS)
			}
			wantCost := tt.wantSpendUSD
			if tt.wantSpendZero {
				wantCost = 0
			}
			if d := entry.CostUSD - wantCost; d > 1e-9 || d < -1e-9 {
				t.Errorf("cost = %v, want %v", entry.CostUSD, wantCost)
			}
		})
	}
}

func TestChatCompletionsProviderError(t *testing.T) {
	fake, mem, srv := newTestGateway(t)
	secret := createKey(t, mem, "agent", 100)
	fake.responseCode = http.StatusInternalServerError
	fake.responseBody = `{"error":{"message":"upstream exploded"}}`

	resp, body := doJSON(t, http.MethodPost, srv.URL+"/v1/chat/completions", secret,
		`{"model":"openai/gpt-4o-mini","messages":[]}`)
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
	if got := errorType(t, body); got != "provider_error" {
		t.Errorf("error type = %q, want provider_error", got)
	}
	if msg := errorMessage(t, body); !strings.Contains(msg, "upstream exploded") {
		t.Errorf("provider body not included in message: %q", msg)
	}
	// Audited with the provider's status.
	audit := mem.Audit()
	if len(audit) != 1 || audit[0].Status != http.StatusInternalServerError {
		t.Errorf("audit = %+v, want one entry with status 500", audit)
	}
}

func TestAdminEndpoints(t *testing.T) {
	fake, _, srv := newTestGateway(t)
	fake.responseBody = `{"choices":[],"usage":{"prompt_tokens":1000000,"completion_tokens":1000000}}`

	// Wrong admin key -> 401 invalid_key on every admin route.
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/admin/keys"},
		{http.MethodGet, "/admin/keys"},
		{http.MethodGet, "/admin/usage"},
	} {
		resp, body := doJSON(t, route.method, srv.URL+route.path, "wrong-admin", `{}`)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s status = %d, want 401", route.method, route.path, resp.StatusCode)
		}
		if got := errorType(t, body); got != "invalid_key" {
			t.Errorf("%s %s error type = %q", route.method, route.path, got)
		}
	}

	// Create a key via the admin API; the secret is returned once.
	resp, body := doJSON(t, http.MethodPost, srv.URL+"/admin/keys", testAdminKey,
		`{"name":"runtime","monthly_budget_usd":25}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create key status = %d: %v", resp.StatusCode, body)
	}
	secret, _ := body["key"].(string)
	if !strings.HasPrefix(secret, "agos-") {
		t.Fatalf("key = %q, want agos- prefix", secret)
	}
	if body["name"] != "runtime" || body["monthly_budget_usd"] != 25.0 {
		t.Errorf("create key body = %v", body)
	}

	// List keys: no secrets exposed.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/admin/keys", nil)
	req.Header.Set("Authorization", "Bearer "+testAdminKey)
	listResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer listResp.Body.Close()
	raw, _ := io.ReadAll(listResp.Body)
	var keys []map[string]any
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatalf("list keys not JSON array: %v (%s)", err, raw)
	}
	if len(keys) != 1 {
		t.Fatalf("keys = %d, want 1", len(keys))
	}
	if _, hasSecret := keys[0]["key"]; hasSecret {
		t.Error("key listing exposes secrets")
	}
	if keys[0]["name"] != "runtime" || keys[0]["monthly_budget_usd"] != 25.0 || keys[0]["spend_usd"] != 0.0 {
		t.Errorf("key listing = %v", keys[0])
	}

	// Proxy one request, then check per-key usage totals.
	if resp, _ := doJSON(t, http.MethodPost, srv.URL+"/v1/chat/completions", secret,
		`{"model":"openai/gpt-4o-mini","messages":[]}`); resp.StatusCode != http.StatusOK {
		t.Fatalf("proxy status = %d", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/admin/usage", nil)
	req.Header.Set("Authorization", "Bearer "+testAdminKey)
	usageResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer usageResp.Body.Close()
	raw, _ = io.ReadAll(usageResp.Body)
	var usage []map[string]any
	if err := json.Unmarshal(raw, &usage); err != nil {
		t.Fatalf("usage not JSON array: %v (%s)", err, raw)
	}
	if len(usage) != 1 {
		t.Fatalf("usage rows = %d, want 1", len(usage))
	}
	row := usage[0]
	if row["name"] != "runtime" || row["requests"] != 1.0 ||
		row["input_tokens"] != 1000000.0 || row["output_tokens"] != 1000000.0 || row["spend_usd"] != 0.75 {
		t.Errorf("usage row = %v", row)
	}
}
