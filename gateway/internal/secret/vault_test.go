package secret

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const testVaultToken = "root-token"

// mockVault serves a KV v2 endpoint at /v1/{path}, returning 403 on a wrong
// token and the given body otherwise.
func mockVault(t *testing.T, path, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != testVaultToken {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errors":["permission denied"]}`))
			return
		}
		if r.URL.Path != "/v1/"+path {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[]}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestVaultSourceSuccess(t *testing.T) {
	const path = "secret/data/agentos"
	body := `{"data":{"data":{"AGENTOS_ANTHROPIC_API_KEY":"sk-vault-anthropic","AGENTOS_OPENAI_API_KEY":"sk-vault-openai"},"metadata":{"version":1}}}`
	srv := mockVault(t, path, body)

	s, err := NewVault(srv.URL, testVaultToken, path)
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	if s.Backend() != BackendVault {
		t.Errorf("backend = %q, want vault", s.Backend())
	}
	if v, ok := s.Get("AGENTOS_ANTHROPIC_API_KEY"); !ok || v != "sk-vault-anthropic" {
		t.Errorf("Get = %q, %v; want sk-vault-anthropic, true", v, ok)
	}
	if v, ok := s.Get("AGENTOS_OPENAI_API_KEY"); !ok || v != "sk-vault-openai" {
		t.Errorf("Get = %q, %v; want sk-vault-openai, true", v, ok)
	}
	if _, ok := s.Get("AGENTOS_MISSING"); ok {
		t.Error("Get(missing) = true, want false")
	}
}

func TestVaultSourceForbidden(t *testing.T) {
	const path = "secret/data/agentos"
	srv := mockVault(t, path, `{"data":{"data":{"K":"v"}}}`)
	if _, err := NewVault(srv.URL, "wrong-token", path); err == nil {
		t.Fatal("NewVault with wrong token: want error (403)")
	}
}

func TestVaultSourceMalformedJSON(t *testing.T) {
	const path = "secret/data/agentos"
	srv := mockVault(t, path, `{"data":{"data":`) // truncated
	if _, err := NewVault(srv.URL, testVaultToken, path); err == nil {
		t.Fatal("NewVault with malformed JSON: want error")
	}
}

func TestVaultSourceMissingDataObject(t *testing.T) {
	const path = "secret/data/agentos"
	srv := mockVault(t, path, `{"data":{"metadata":{"version":1}}}`)
	if _, err := NewVault(srv.URL, testVaultToken, path); err == nil {
		t.Fatal("NewVault with missing data.data: want error")
	}
}

func TestVaultSourceMisconfig(t *testing.T) {
	if _, err := NewVault("", testVaultToken, "secret/data/x"); err == nil {
		t.Error("NewVault empty addr: want error")
	}
	if _, err := NewVault("http://vault:8200", "", "secret/data/x"); err == nil {
		t.Error("NewVault empty token: want error")
	}
	if _, err := NewVault("http://vault:8200", testVaultToken, ""); err == nil {
		t.Error("NewVault empty path: want error")
	}
	// Unreachable server (nothing listening) is fatal.
	if _, err := NewVault("http://127.0.0.1:0", testVaultToken, "secret/data/x"); err == nil {
		t.Error("NewVault unreachable: want error")
	}
}

func TestFromEnvVaultBackend(t *testing.T) {
	const path = "secret/data/agentos"
	srv := mockVault(t, path, `{"data":{"data":{"AGENTOS_OPENAI_API_KEY":"sk-from-env"}}}`)
	t.Setenv("AGENTOS_SECRETS_BACKEND", "vault")
	t.Setenv("AGENTOS_VAULT_ADDR", srv.URL)
	t.Setenv("AGENTOS_VAULT_TOKEN", testVaultToken)
	t.Setenv("AGENTOS_VAULT_KV_PATH", path)

	s, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv vault: %v", err)
	}
	if s.Backend() != BackendVault {
		t.Errorf("backend = %q, want vault", s.Backend())
	}
	if v, ok := s.Get("AGENTOS_OPENAI_API_KEY"); !ok || v != "sk-from-env" {
		t.Errorf("Get = %q, %v; want sk-from-env, true", v, ok)
	}
}
