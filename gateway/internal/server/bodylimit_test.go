package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ifahad/agentos/gateway/internal/provider"
	"github.com/ifahad/agentos/gateway/internal/store"
)

// bodyLimitServer builds a gateway whose body cap is small enough to test
// against without allocating megabytes.
func bodyLimitServer(t *testing.T, limit int64) http.Handler {
	t.Helper()
	st := store.NewMemory()
	if err := st.EnsureOrg(t.Context(), store.DefaultOrgID, "default", 0); err != nil {
		t.Fatalf("ensure org: %v", err)
	}
	srv := New(st, &provider.Router{}, "admin", WithMaxBodyBytes(limit))
	return srv.Handler()
}

// An oversized body must be refused rather than read into memory. The status
// matters less than the refusal — what must not happen is a 2xx implying the
// gateway acted on a truncated document.
func TestBodyLimitRejectsOversizedRequest(t *testing.T) {
	h := bodyLimitServer(t, 1024)

	huge := `{"model":"ollama/x","messages":[{"role":"user","content":"` +
		strings.Repeat("A", 4096) + `"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(huge))
	req.Header.Set("Authorization", "Bearer nope")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code >= 200 && rec.Code < 300 {
		t.Fatalf("oversized body was accepted with %d; it must be refused", rec.Code)
	}
}

// A body under the cap must pass the limiter untouched and be handled normally
// — here that means reaching auth and being rejected for the bad key, proving
// the limiter did not interfere.
func TestBodyLimitAllowsNormalRequest(t *testing.T) {
	h := bodyLimitServer(t, 1<<20)

	body := `{"model":"ollama/x","messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer nope")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("normal body: got %d, want 401 (reached auth)", rec.Code)
	}
}

// A cap of 0 disables the limiter, for deployments behind a proxy that already
// enforces one.
func TestBodyLimitDisabled(t *testing.T) {
	h := bodyLimitServer(t, 0)

	body := `{"model":"ollama/x","messages":[{"role":"user","content":"` +
		strings.Repeat("A", 8192) + `"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer nope")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("with the cap disabled a large body should still reach auth: got %d", rec.Code)
	}
}

// The cap must apply to admin routes too, not just the proxy paths — a new
// endpoint should inherit it by being registered, not by remembering to opt in.
func TestBodyLimitAppliesToAdminRoutes(t *testing.T) {
	h := bodyLimitServer(t, 512)

	huge := `{"name":"` + strings.Repeat("k", 4096) + `","monthly_budget_usd":1}`
	req := httptest.NewRequest(http.MethodPost, "/admin/keys", strings.NewReader(huge))
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code >= 200 && rec.Code < 300 {
		t.Fatalf("oversized admin body was accepted with %d; it must be refused", rec.Code)
	}
}
