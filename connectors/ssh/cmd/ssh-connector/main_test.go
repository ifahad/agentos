package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// writeKnownHosts creates a syntactically valid known_hosts file and returns
// its path.
func writeKnownHosts(t *testing.T) string {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("ssh public key: %v", err)
	}
	line := knownhosts.Line([]string{"example.com:22"}, sshPub)
	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte(line+"\n"), 0o600); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}
	return path
}

func TestResolveHostKeyCallback(t *testing.T) {
	knownHosts := writeKnownHosts(t)

	t.Run("unset known_hosts and no insecure opt-out refuses to start", func(t *testing.T) {
		cb, mode, err := resolveHostKeyCallback("", "")
		if err == nil {
			t.Fatalf("expected error when known_hosts unset and no opt-out, got nil (mode=%d, cb=%v)", mode, cb != nil)
		}
		if cb != nil {
			t.Fatalf("expected nil callback on error, got non-nil")
		}
	})

	t.Run("insecure opt-out true is allowed with insecure mode", func(t *testing.T) {
		cb, mode, err := resolveHostKeyCallback("", "true")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cb == nil {
			t.Fatalf("expected non-nil callback for insecure opt-out")
		}
		if mode != hostKeyInsecure {
			t.Fatalf("expected hostKeyInsecure mode, got %d", mode)
		}
	})

	t.Run("insecure opt-out is case-insensitive and whitespace-tolerant", func(t *testing.T) {
		cb, mode, err := resolveHostKeyCallback("", "  TRUE  ")
		if err != nil || cb == nil || mode != hostKeyInsecure {
			t.Fatalf("expected insecure allow for %q, got (cb=%v, mode=%d, err=%v)", "  TRUE  ", cb != nil, mode, err)
		}
	})

	t.Run("insecure opt-out other than true still refuses", func(t *testing.T) {
		for _, v := range []string{"false", "0", "yes", "1"} {
			cb, _, err := resolveHostKeyCallback("", v)
			if err == nil {
				t.Fatalf("expected refusal for opt-out=%q, got nil error", v)
			}
			if cb != nil {
				t.Fatalf("expected nil callback for opt-out=%q", v)
			}
		}
	})

	t.Run("known_hosts set uses strict verification", func(t *testing.T) {
		cb, mode, err := resolveHostKeyCallback(knownHosts, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cb == nil {
			t.Fatalf("expected non-nil strict callback")
		}
		if mode != hostKeyStrict {
			t.Fatalf("expected hostKeyStrict mode, got %d", mode)
		}
	})

	t.Run("known_hosts set takes precedence over insecure opt-out", func(t *testing.T) {
		cb, mode, err := resolveHostKeyCallback(knownHosts, "true")
		if err != nil || cb == nil || mode != hostKeyStrict {
			t.Fatalf("expected strict when known_hosts set, got (cb=%v, mode=%d, err=%v)", cb != nil, mode, err)
		}
	})

	t.Run("bad known_hosts path errors", func(t *testing.T) {
		cb, _, err := resolveHostKeyCallback(filepath.Join(t.TempDir(), "does-not-exist"), "")
		if err == nil {
			t.Fatalf("expected error for missing known_hosts file")
		}
		if cb != nil {
			t.Fatalf("expected nil callback on load error")
		}
	})
}
