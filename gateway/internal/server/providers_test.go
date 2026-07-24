package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ifahad/agentos/gateway/internal/provider"
	"github.com/ifahad/agentos/gateway/internal/store"
)

// newTestServerWithProviders builds a gateway with the given registry attached
// and a secret source that resolves no provider keys, so key_present is false.
func newTestServerWithProviders(t *testing.T, reg *provider.Registry) *Server {
	t.Helper()
	mem := store.NewMemory()
	if err := mem.EnsureOrg(t.Context(), store.DefaultOrgID, "default", 0); err != nil {
		t.Fatalf("ensure org: %v", err)
	}
	return New(mem, &provider.Router{}, testAdminKey, WithProviders(reg))
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func TestHandleProviders(t *testing.T) {
	reg, errs := provider.ParseRegistry([]byte(`{"providers":[
	  {"name":"moonshot","base_url":"https://api.moonshot.ai",
	   "key_name":"AGENTOS_MOONSHOT_API_KEY","enabled":true,
	   "prices":{"kimi-k3":{"in":1,"out":4}}}]}`))
	if len(errs) != 0 {
		t.Fatalf("registry errs = %v", errs)
	}
	srv := newTestServerWithProviders(t, reg)

	req := httptest.NewRequest(http.MethodGet, "/admin/providers", nil)
	req.Header.Set("Authorization", "Bearer "+testAdminKey)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Providers []struct {
			Name       string   `json:"name"`
			BaseURL    string   `json:"base_url"`
			Enabled    bool     `json:"enabled"`
			KeyPresent bool     `json:"key_present"`
			Models     []string `json:"models"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Providers) != 1 {
		t.Fatalf("providers = %+v", out.Providers)
	}
	p := out.Providers[0]
	if p.Name != "moonshot" || p.BaseURL != "https://api.moonshot.ai" {
		t.Errorf("entry = %+v", p)
	}
	if p.KeyPresent {
		t.Error("key_present must be false when the secret is absent")
	}
	if len(p.Models) != 1 || p.Models[0] != "kimi-k3" {
		t.Errorf("models = %v", p.Models)
	}
	if body := rec.Body.String(); containsAny(body, "AGENTOS_MOONSHOT_API_KEY", "sk-") {
		t.Error("response must never contain a key name's value or a secret")
	}
}

func TestHandleProvidersRequiresAdmin(t *testing.T) {
	srv := newTestServerWithProviders(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/admin/providers", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}
