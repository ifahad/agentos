package sshclient

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	testUser     = "tester"
	testPassword = "sekret"
)

// startTestServer runs an in-process SSH server on a random loopback port.
// It accepts testUser/testPassword, handles "exec" requests by running the
// command through /bin/sh -c with stdout/stderr piped back, and reports the
// real exit status — enough to exercise the client path end to end.
func startTestServer(t *testing.T) (addr string, hostKey ssh.PublicKey) {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("host key signer: %v", err)
	}

	cfg := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if conn.User() == testUser && string(password) == testPassword {
				return nil, nil
			}
			return nil, fmt.Errorf("authentication rejected for %q", conn.User())
		},
	}
	cfg.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			netConn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleTestConn(netConn, cfg)
		}
	}()

	return ln.Addr().String(), signer.PublicKey()
}

func handleTestConn(netConn net.Conn, cfg *ssh.ServerConfig) {
	sconn, chans, reqs, err := ssh.NewServerConn(netConn, cfg)
	if err != nil {
		_ = netConn.Close()
		return
	}
	defer sconn.Close()
	go ssh.DiscardRequests(reqs)

	for newCh := range chans {
		if newCh.ChannelType() != "session" {
			_ = newCh.Reject(ssh.UnknownChannelType, "only session channels are supported")
			continue
		}
		ch, chReqs, err := newCh.Accept()
		if err != nil {
			continue
		}
		go handleTestSession(ch, chReqs)
	}
}

func handleTestSession(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()
	for req := range reqs {
		if req.Type != "exec" {
			if req.WantReply {
				_ = req.Reply(false, nil)
			}
			continue
		}
		var payload struct{ Command string }
		if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
			_ = req.Reply(false, nil)
			continue
		}
		_ = req.Reply(true, nil)

		cmd := exec.Command("/bin/sh", "-c", payload.Command)
		cmd.Stdout = ch
		cmd.Stderr = ch.Stderr()
		status := uint32(0)
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				status = uint32(exitErr.ExitCode())
			} else {
				status = 127
			}
		}
		_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
		return
	}
}

func newTestClient(t *testing.T, addr string, hostKey ssh.PublicKey, mutate func(*Config)) *Client {
	t.Helper()
	cfg := Config{
		Addr:            addr,
		User:            testUser,
		Auth:            []ssh.AuthMethod{ssh.Password(testPassword)},
		HostKeyCallback: ssh.FixedHostKey(hostKey),
		Timeout:         10 * time.Second,
		MaxOutputBytes:  DefaultMaxOutputBytes,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	c := New(cfg)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestRunIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in-process SSH server integration test in -short mode")
	}
	addr, hostKey := startTestServer(t)

	t.Run("success with stdout", func(t *testing.T) {
		c := newTestClient(t, addr, hostKey, nil)
		res, err := c.Run(context.Background(), "echo hello")
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if res.ExitCode != 0 || res.Stdout != "hello\n" || res.TimedOut || res.Truncated {
			t.Fatalf("Run = %+v, want exit 0, stdout %q", res, "hello\n")
		}
	})

	t.Run("non-zero exit code", func(t *testing.T) {
		c := newTestClient(t, addr, hostKey, nil)
		res, err := c.Run(context.Background(), "exit 3")
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if res.ExitCode != 3 || res.TimedOut {
			t.Fatalf("Run = %+v, want exit 3", res)
		}
	})

	t.Run("stderr captured separately", func(t *testing.T) {
		c := newTestClient(t, addr, hostKey, nil)
		res, err := c.Run(context.Background(), "echo out; echo err 1>&2")
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if res.Stdout != "out\n" || res.Stderr != "err\n" {
			t.Fatalf("Run = %+v, want stdout %q stderr %q", res, "out\n", "err\n")
		}
	})

	t.Run("output capped and flagged truncated", func(t *testing.T) {
		c := newTestClient(t, addr, hostKey, func(cfg *Config) { cfg.MaxOutputBytes = 16 })
		res, err := c.Run(context.Background(), "printf '%01000d' 7")
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if !res.Truncated || len(res.Stdout) != 16 || res.ExitCode != 0 {
			t.Fatalf("Run = truncated=%t len(stdout)=%d exit=%d, want truncated 16-byte stdout with exit 0",
				res.Truncated, len(res.Stdout), res.ExitCode)
		}
	})

	t.Run("timeout closes session", func(t *testing.T) {
		c := newTestClient(t, addr, hostKey, func(cfg *Config) { cfg.Timeout = 300 * time.Millisecond })
		start := time.Now()
		res, err := c.Run(context.Background(), "sleep 5")
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if !res.TimedOut || res.ExitCode != -1 {
			t.Fatalf("Run = %+v, want timed_out with exit -1", res)
		}
		if elapsed := time.Since(start); elapsed > 3*time.Second {
			t.Fatalf("Run took %v, want prompt return after the 300ms timeout", elapsed)
		}
	})

	t.Run("connection reused across calls", func(t *testing.T) {
		c := newTestClient(t, addr, hostKey, nil)
		if _, err := c.Run(context.Background(), "true"); err != nil {
			t.Fatalf("first Run: %v", err)
		}
		c.mu.Lock()
		first := c.conn
		c.mu.Unlock()
		if first == nil {
			t.Fatal("connection not cached after first Run")
		}
		if _, err := c.Run(context.Background(), "true"); err != nil {
			t.Fatalf("second Run: %v", err)
		}
		c.mu.Lock()
		second := c.conn
		c.mu.Unlock()
		if first != second {
			t.Fatal("connection was not reused across calls")
		}
	})

	t.Run("bad password fails to dial", func(t *testing.T) {
		c := newTestClient(t, addr, hostKey, func(cfg *Config) {
			cfg.Auth = []ssh.AuthMethod{ssh.Password("wrong")}
		})
		if _, err := c.Run(context.Background(), "true"); err == nil ||
			!strings.Contains(err.Error(), "dial") {
			t.Fatalf("Run with bad password = %v, want dial error", err)
		}
	})

	t.Run("host key mismatch rejected", func(t *testing.T) {
		_, otherPriv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("generate key: %v", err)
		}
		otherSigner, err := ssh.NewSignerFromKey(otherPriv)
		if err != nil {
			t.Fatalf("signer: %v", err)
		}
		c := newTestClient(t, addr, hostKey, func(cfg *Config) {
			cfg.HostKeyCallback = ssh.FixedHostKey(otherSigner.PublicKey())
		})
		if _, err := c.Run(context.Background(), "true"); err == nil {
			t.Fatal("Run with mismatched host key succeeded, want error")
		}
	})
}

func TestCappedBuffer(t *testing.T) {
	tests := []struct {
		name          string
		limit         int
		writes        []string
		want          string
		wantTruncated bool
	}{
		{"under limit", 10, []string{"abc", "def"}, "abcdef", false},
		{"exactly at limit", 6, []string{"abc", "def"}, "abcdef", false},
		{"split write over limit", 4, []string{"abc", "def"}, "abcd", true},
		{"single write over limit", 3, []string{"abcdef"}, "abc", true},
		{"writes after full are discarded", 3, []string{"abc", "x"}, "abc", true},
		{"empty write never truncates", 3, []string{"abc", ""}, "abc", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newCappedBuffer(tt.limit)
			for _, w := range tt.writes {
				n, err := b.Write([]byte(w))
				if err != nil || n != len(w) {
					t.Fatalf("Write(%q) = (%d, %v), want (%d, nil)", w, n, err, len(w))
				}
			}
			if b.String() != tt.want || b.Truncated() != tt.wantTruncated {
				t.Fatalf("buffer = (%q, truncated=%t), want (%q, %t)",
					b.String(), b.Truncated(), tt.want, tt.wantTruncated)
			}
		})
	}
}
