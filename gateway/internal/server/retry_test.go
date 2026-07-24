package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ifahad/agentos/gateway/internal/provider"
	"github.com/ifahad/agentos/gateway/internal/store"
)

func TestRetryable(t *testing.T) {
	tests := []struct {
		name   string
		status int
		err    error
		want   bool
	}{
		{"transport error", 0, errors.New("connection refused"), true},
		{"429", http.StatusTooManyRequests, nil, true},
		{"500", http.StatusInternalServerError, nil, true},
		{"502", http.StatusBadGateway, nil, true},
		{"503", http.StatusServiceUnavailable, nil, true},
		{"400 never", http.StatusBadRequest, nil, false},
		{"401 never", http.StatusUnauthorized, nil, false},
		{"403 never", http.StatusForbidden, nil, false},
		{"404 never", http.StatusNotFound, nil, false},
		{"200", http.StatusOK, nil, false},
	}
	for _, tt := range tests {
		if got := retryable(tt.status, tt.err); got != tt.want {
			t.Errorf("%s: retryable(%d, %v) = %v, want %v", tt.name, tt.status, tt.err, got, tt.want)
		}
	}
}

func TestParseRetryAfter(t *testing.T) {
	if d, ok := parseRetryAfter("2"); !ok || d != 2*time.Second {
		t.Errorf("seconds form: %v %v", d, ok)
	}
	if _, ok := parseRetryAfter(""); ok {
		t.Error("empty must not parse")
	}
	if _, ok := parseRetryAfter("garbage"); ok {
		t.Error("garbage must not parse")
	}
	if d, ok := parseRetryAfter("99999"); !ok || d != maxBackoff {
		t.Errorf("absurd Retry-After must clamp to maxBackoff, got %v", d)
	}
}

func TestBackoffDelay(t *testing.T) {
	fixed := func() float64 { return 0.5 } // deterministic jitter
	d0 := backoffDelay(0, "", fixed)
	d1 := backoffDelay(1, "", fixed)
	if d1 <= d0 {
		t.Errorf("backoff must grow: %v then %v", d0, d1)
	}
	if got := backoffDelay(99, "", fixed); got > maxBackoff {
		t.Errorf("delay = %v, want <= %v", got, maxBackoff)
	}
	// Retry-After wins over computed backoff.
	if got := backoffDelay(0, "3", fixed); got != 3*time.Second {
		t.Errorf("Retry-After must win, got %v", got)
	}
}

// doChat builds a gateway whose ollama route points at upstreamURL, seeds a
// budgeted virtual key, and issues one chat completion with it. The retry loop
// runs inside proxy, so this exercises it end to end.
func doChat(t *testing.T, upstreamURL, body string) *httptest.ResponseRecorder {
	t.Helper()
	mem := store.NewMemory()
	if err := mem.EnsureOrg(context.Background(), store.DefaultOrgID, "default", 0); err != nil {
		t.Fatalf("ensure org: %v", err)
	}
	secret := createKey(t, mem, "retry-key", 100)
	router := &provider.Router{OllamaBaseURL: upstreamURL}
	srv := New(mem, router, testAdminKey)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func TestProxyRetriesThenSucceeds(t *testing.T) {
	var attempts int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":3,"completion_tokens":5}}`)
	}))
	defer upstream.Close()

	rec := doChat(t, upstream.URL, `{"model":"ollama/test","messages":[{"role":"user","content":"hi"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Errorf("attempts = %d, want 2", got)
	}
}

func TestProxyDoesNotRetryClientErrors(t *testing.T) {
	var attempts int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":"bad model"}`)
	}))
	defer upstream.Close()

	rec := doChat(t, upstream.URL, `{"model":"ollama/test","messages":[{"role":"user","content":"hi"}]}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Errorf("attempts = %d, want 1 (4xx must not retry)", got)
	}
}

func TestProxyStopsAtMaxAttempts(t *testing.T) {
	var attempts int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	rec := doChat(t, upstream.URL, `{"model":"ollama/test","messages":[{"role":"user","content":"hi"}]}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := atomic.LoadInt32(&attempts); got != int32(provider.DefaultMaxAttempts) {
		t.Errorf("attempts = %d, want %d (DefaultMaxAttempts)", got, provider.DefaultMaxAttempts)
	}
}
