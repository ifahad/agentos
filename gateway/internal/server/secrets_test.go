package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ifahad/agentos/gateway/internal/provider"
	"github.com/ifahad/agentos/gateway/internal/rbac"
	"github.com/ifahad/agentos/gateway/internal/store"
)

// fakeReloadableSecrets is a secret.Source that also implements
// secret.Reloadable, counting Reload calls.
type fakeReloadableSecrets struct {
	present map[string]bool
	calls   atomic.Int64
}

func (f *fakeReloadableSecrets) Get(name string) (string, bool) {
	if f.present[name] {
		return "REDACTED", true
	}
	return "", false
}

func (f *fakeReloadableSecrets) Backend() string { return "file" }

func (f *fakeReloadableSecrets) Reload() (int, error) {
	f.calls.Add(1)
	return 1, nil
}

func TestSecretsReloadRequiresRoot(t *testing.T) {
	fake := &fakeReloadableSecrets{present: map[string]bool{"AGENTOS_ANTHROPIC_API_KEY": true}}
	_, mem, srv := newTestGateway(t, WithSecrets(fake, DefaultSecretNames))
	ctx := context.Background()
	org, err := mem.CreateOrg(ctx, "acme", 0)
	if err != nil {
		t.Fatal(err)
	}
	_, memberTok, err := mem.CreateUser(ctx, org.ID, "m@acme.test", rbac.RoleMember)
	if err != nil {
		t.Fatal(err)
	}

	// Unauthenticated → 401.
	if resp, _ := doJSON(t, http.MethodPost, srv.URL+"/admin/secrets/reload", "", ""); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauth reload = %d, want 401", resp.StatusCode)
	}
	// A user token → 403 forbidden.
	resp, body := doJSON(t, http.MethodPost, srv.URL+"/admin/secrets/reload", memberTok, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("user reload = %d, want 403", resp.StatusCode)
	}
	if got := errorType(t, body); got != errForbidden {
		t.Errorf("error type = %q, want forbidden", got)
	}
	// Neither attempt should have reloaded.
	if n := fake.calls.Load(); n != 0 {
		t.Errorf("Reload called %d times before any root call, want 0", n)
	}
}

func TestSecretsReloadRootReloadsStatusAndAudits(t *testing.T) {
	fake := &fakeReloadableSecrets{present: map[string]bool{"AGENTOS_ANTHROPIC_API_KEY": true, "AGENTOS_OPENAI_API_KEY": false}}
	_, mem, srv := newTestGateway(t, WithSecrets(fake, DefaultSecretNames))

	// Root reload twice → 200 with the status array; the fake counts calls.
	for i := 1; i <= 2; i++ {
		resp, raw := doRawBytes(t, http.MethodPost, srv.URL+"/admin/secrets/reload", testAdminKey, "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("reload %d status = %d, want 200", i, resp.StatusCode)
		}
		var status []secretStatus
		if err := json.Unmarshal(raw, &status); err != nil {
			t.Fatalf("unmarshal status: %v (%s)", err, raw)
		}
		if len(status) != 2 {
			t.Errorf("status rows = %d, want 2", len(status))
		}
		if int(fake.calls.Load()) != i {
			t.Errorf("Reload calls = %d, want %d", fake.calls.Load(), i)
		}
	}

	// The reloads are audited with the secret_reload kind and no spend.
	var reloadAudits int
	for _, u := range mem.Audit() {
		if u.Kind == store.KindSecretReload {
			reloadAudits++
			if u.CostUSD != 0 {
				t.Errorf("secret_reload audit recorded spend %v", u.CostUSD)
			}
		}
	}
	if reloadAudits != 2 {
		t.Errorf("secret_reload audits = %d, want 2", reloadAudits)
	}
}

func TestSecretsReloadEnvBackendIsNoOp(t *testing.T) {
	// The default env backend is not Reloadable; reload is a 200 no-op.
	_, _, srv := newTestGateway(t)
	resp, raw := doRawBytes(t, http.MethodPost, srv.URL+"/admin/secrets/reload", testAdminKey, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("env reload status = %d, want 200", resp.StatusCode)
	}
	var status []secretStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, raw)
	}
	if len(status) != len(DefaultSecretNames) {
		t.Errorf("status rows = %d, want %d", len(status), len(DefaultSecretNames))
	}
}

// reloadableSecrets is a settable secret.Source whose Reload swaps the returned
// values, standing in for a rotated file/age/vault secret.
type reloadableSecrets struct {
	mu     sync.Mutex
	values map[string]string
	next   map[string]string
}

func (s *reloadableSecrets) Get(name string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.values[name]
	return v, ok
}
func (s *reloadableSecrets) Backend() string { return "reloadable" }
func (s *reloadableSecrets) Reload() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values = s.next
	return len(s.next), nil
}

// TestSecretsReloadRotatesRouterKey proves H5 through the admin endpoint: the
// router resolves provider keys from the live source, so POST /admin/secrets/
// reload changes the key Route() returns upstream.
func TestSecretsReloadRotatesRouterKey(t *testing.T) {
	src := &reloadableSecrets{
		values: map[string]string{provider.AnthropicKeyName: "sk-ant-old"},
		next:   map[string]string{provider.AnthropicKeyName: "sk-ant-new"},
	}
	router := &provider.Router{Secrets: src}
	mem := store.NewMemory()
	srv := New(mem, router, testAdminKey, WithSecrets(src, DefaultSecretNames))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	// Before reload the router serves the old key.
	route, err := router.Route("anthropic/claude-sonnet-5")
	if err != nil {
		t.Fatal(err)
	}
	if route.APIKey != "sk-ant-old" {
		t.Fatalf("pre-reload key = %q, want sk-ant-old", route.APIKey)
	}

	resp, _ := doRawBytes(t, http.MethodPost, ts.URL+"/admin/secrets/reload", testAdminKey, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reload status = %d, want 200", resp.StatusCode)
	}

	// After reload the very next Route call serves the rotated key.
	route, _ = router.Route("anthropic/claude-sonnet-5")
	if route.APIKey != "sk-ant-new" {
		t.Errorf("post-reload key = %q, want sk-ant-new", route.APIKey)
	}
}

// TestSecureCompareConstantTime checks the constant-time comparison guarding the
// admin key and SCIM token (M1): equal strings match, everything else rejects.
func TestSecureCompareConstantTime(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"admin-secret", "admin-secret", true},
		{"admin-secret", "admin-wrong", false},
		{"admin-secret", "admin-secre", false}, // different length
		{"", "admin-secret", false},            // empty configured secret fails closed
		{"admin-secret", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		if got := secureCompare(tc.a, tc.b); got != tc.want {
			t.Errorf("secureCompare(%q,%q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
