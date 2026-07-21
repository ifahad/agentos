package secret

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"filippo.io/age"
)

func TestEnvSourcePassthrough(t *testing.T) {
	const name = "AGENTOS_TEST_SECRET_ENV"
	t.Setenv(name, "sk-env-value")
	s := NewEnv()
	if s.Backend() != BackendEnv {
		t.Errorf("backend = %q, want env", s.Backend())
	}
	if v, ok := s.Get(name); !ok || v != "sk-env-value" {
		t.Errorf("Get(%q) = %q, %v; want sk-env-value, true", name, v, ok)
	}
	if v, ok := s.Get("AGENTOS_TEST_SECRET_MISSING"); ok || v != "" {
		t.Errorf("Get(missing) = %q, %v; want \"\", false", v, ok)
	}
}

func TestFromEnvDefaultsToEnv(t *testing.T) {
	t.Setenv("AGENTOS_SECRETS_BACKEND", "")
	s, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if s.Backend() != BackendEnv {
		t.Errorf("default backend = %q, want env", s.Backend())
	}
}

func TestFromEnvUnknownBackend(t *testing.T) {
	// "vault" is a valid backend as of Phase 6; use a name that is still unknown.
	t.Setenv("AGENTOS_SECRETS_BACKEND", "consul")
	if _, err := FromEnv(); err == nil {
		t.Fatal("FromEnv with unknown backend: want error")
	}
}

func TestFileSourceReadAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.json")
	if err := os.WriteFile(path, []byte(`{"AGENTOS_ANTHROPIC_API_KEY":"sk-file-1"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := NewFile(path)
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	if s.Backend() != BackendFile {
		t.Errorf("backend = %q, want file", s.Backend())
	}
	if v, ok := s.Get("AGENTOS_ANTHROPIC_API_KEY"); !ok || v != "sk-file-1" {
		t.Errorf("Get = %q, %v; want sk-file-1, true", v, ok)
	}
	if _, ok := s.Get("AGENTOS_OPENAI_API_KEY"); ok {
		t.Error("Get(unset) = true, want false")
	}

	// Rotate the file and bump its mtime; the next Get must observe the change.
	if err := os.WriteFile(path, []byte(`{"AGENTOS_ANTHROPIC_API_KEY":"sk-file-2","AGENTOS_OPENAI_API_KEY":"sk-oai"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	if v, ok := s.Get("AGENTOS_ANTHROPIC_API_KEY"); !ok || v != "sk-file-2" {
		t.Errorf("after reload Get = %q, %v; want sk-file-2, true", v, ok)
	}
	if v, ok := s.Get("AGENTOS_OPENAI_API_KEY"); !ok || v != "sk-oai" {
		t.Errorf("after reload Get(new key) = %q, %v; want sk-oai, true", v, ok)
	}
}

func TestFileSourceMissingFileIsFatal(t *testing.T) {
	if _, err := NewFile(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("NewFile with missing file: want error")
	}
	if _, err := NewFile(""); err == nil {
		t.Fatal("NewFile with empty path: want error")
	}
}

// writeAgeFile encrypts plaintext to path under recipient and returns nothing.
func writeAgeFile(t *testing.T, path string, recipient age.Recipient, plaintext string) {
	t.Helper()
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipient)
	if err != nil {
		t.Fatalf("age.Encrypt: %v", err)
	}
	if _, err := w.Write([]byte(plaintext)); err != nil {
		t.Fatalf("write plaintext: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close age writer: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write age file: %v", err)
	}
}

func TestAgeSourceDecrypts(t *testing.T) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("GenerateX25519Identity: %v", err)
	}
	path := filepath.Join(t.TempDir(), "secrets.age")
	writeAgeFile(t, path, id.Recipient(), `{"AGENTOS_ANTHROPIC_API_KEY":"sk-age-secret"}`)

	s, err := NewAge(path, id.String())
	if err != nil {
		t.Fatalf("NewAge: %v", err)
	}
	if s.Backend() != BackendAge {
		t.Errorf("backend = %q, want age", s.Backend())
	}
	if v, ok := s.Get("AGENTOS_ANTHROPIC_API_KEY"); !ok || v != "sk-age-secret" {
		t.Errorf("Get = %q, %v; want sk-age-secret, true", v, ok)
	}
	if _, ok := s.Get("AGENTOS_OPENAI_API_KEY"); ok {
		t.Error("Get(unset) = true, want false")
	}
}

func TestAgeSourceMisconfig(t *testing.T) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "secrets.age")
	writeAgeFile(t, path, id.Recipient(), `{"K":"v"}`)

	// Missing file.
	if _, err := NewAge(filepath.Join(t.TempDir(), "nope.age"), id.String()); err == nil {
		t.Error("NewAge missing file: want error")
	}
	// Empty key / empty path.
	if _, err := NewAge(path, ""); err == nil {
		t.Error("NewAge empty key: want error")
	}
	if _, err := NewAge("", id.String()); err == nil {
		t.Error("NewAge empty path: want error")
	}
	// Bad key.
	if _, err := NewAge(path, "AGE-SECRET-KEY-1notarealkey"); err == nil {
		t.Error("NewAge bad key: want error")
	}
	// Wrong identity cannot decrypt.
	other, _ := age.GenerateX25519Identity()
	if _, err := NewAge(path, other.String()); err == nil {
		t.Error("NewAge wrong identity: want decrypt error")
	}
}

func TestFileSourceReloadImmediate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.json")
	if err := os.WriteFile(path, []byte(`{"AGENTOS_ANTHROPIC_API_KEY":"sk-1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := NewFile(path)
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	r, ok := s.(Reloadable)
	if !ok {
		t.Fatal("file source does not implement Reloadable")
	}

	// Rewrite the file WITHOUT bumping mtime beyond the current second; Reload
	// must re-read unconditionally and observe the change.
	if err := os.WriteFile(path, []byte(`{"AGENTOS_ANTHROPIC_API_KEY":"sk-2","AGENTOS_OPENAI_API_KEY":"sk-oai"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := r.Reload()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	// One value changed (anthropic), one added (openai) → 2.
	if changed != 2 {
		t.Errorf("changed = %d, want 2", changed)
	}
	if v, ok := s.Get("AGENTOS_ANTHROPIC_API_KEY"); !ok || v != "sk-2" {
		t.Errorf("after Reload Get = %q, %v; want sk-2", v, ok)
	}
	if v, ok := s.Get("AGENTOS_OPENAI_API_KEY"); !ok || v != "sk-oai" {
		t.Errorf("after Reload Get(new) = %q, %v; want sk-oai", v, ok)
	}

	// No change → 0.
	changed, err = r.Reload()
	if err != nil || changed != 0 {
		t.Errorf("second Reload changed = %d, err = %v; want 0, nil", changed, err)
	}
}

func TestEnvSourceNotReloadable(t *testing.T) {
	if _, ok := NewEnv().(Reloadable); ok {
		t.Error("env source should not be Reloadable")
	}
}

func TestAgeSourceReloadable(t *testing.T) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "secrets.age")
	writeAgeFile(t, path, id.Recipient(), `{"AGENTOS_ANTHROPIC_API_KEY":"sk-age-1"}`)
	s, err := NewAge(path, id.String())
	if err != nil {
		t.Fatalf("NewAge: %v", err)
	}
	r, ok := s.(Reloadable)
	if !ok {
		t.Fatal("age source does not implement Reloadable")
	}
	// Re-encrypt with a rotated value; Reload must re-decrypt and pick it up.
	writeAgeFile(t, path, id.Recipient(), `{"AGENTOS_ANTHROPIC_API_KEY":"sk-age-2"}`)
	changed, err := r.Reload()
	if err != nil || changed != 1 {
		t.Fatalf("Reload changed = %d, err = %v; want 1, nil", changed, err)
	}
	if v, _ := s.Get("AGENTOS_ANTHROPIC_API_KEY"); v != "sk-age-2" {
		t.Errorf("after Reload Get = %q, want sk-age-2", v)
	}
}
