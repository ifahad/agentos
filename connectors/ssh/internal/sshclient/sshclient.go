// Package sshclient wraps golang.org/x/crypto/ssh with the execution model
// the agentos-ssh connector needs: one lazily-dialed, mutex-guarded
// connection reused across calls, a fresh session per command, a per-command
// timeout, and byte-capped stdout/stderr capture.
package sshclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// Defaults applied when the corresponding env vars are unset.
const (
	DefaultTimeout        = 15 * time.Second
	DefaultMaxOutputBytes = 65536
)

// Config holds everything needed to dial and run commands.
type Config struct {
	Addr            string // host:port
	User            string
	Auth            []ssh.AuthMethod
	HostKeyCallback ssh.HostKeyCallback
	Timeout         time.Duration // per-command run timeout (also the dial timeout)
	MaxOutputBytes  int           // per-stream stdout/stderr cap
}

// Result is the outcome of one command run, shaped for the run_command tool.
type Result struct {
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Truncated bool   `json:"truncated"`
	TimedOut  bool   `json:"timed_out"`
}

// Client runs commands over a single reused SSH connection.
type Client struct {
	cfg Config

	mu   sync.Mutex
	conn *ssh.Client
}

// New returns a Client for cfg. Zero Timeout / MaxOutputBytes fall back to
// the defaults. The connection is dialed lazily on the first Run.
func New(cfg Config) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.MaxOutputBytes <= 0 {
		cfg.MaxOutputBytes = DefaultMaxOutputBytes
	}
	return &Client{cfg: cfg}
}

// Close tears down the underlying connection, if any.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}

// session returns a fresh session on the shared connection, dialing lazily
// and redialing once if the cached connection turns out to be dead.
func (c *Client) session() (*ssh.Session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		sess, err := c.conn.NewSession()
		if err == nil {
			return sess, nil
		}
		// Connection went stale (server restart, network drop): redial.
		_ = c.conn.Close()
		c.conn = nil
	}

	conn, err := ssh.Dial("tcp", c.cfg.Addr, &ssh.ClientConfig{
		User:            c.cfg.User,
		Auth:            c.cfg.Auth,
		HostKeyCallback: c.cfg.HostKeyCallback,
		Timeout:         c.cfg.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", c.cfg.Addr, err)
	}
	sess, err := conn.NewSession()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("new session: %w", err)
	}
	c.conn = conn
	return sess, nil
}

// Run executes command in a fresh session on the shared connection.
//
// The command is raced against the configured timeout (and ctx): on timeout
// the session is closed to abandon the remote command and the result carries
// timed_out=true with exit code -1. Stdout and stderr are each capped at
// MaxOutputBytes (truncated=true when either overflowed). A remote non-zero
// exit is not an error: its status lands in ExitCode via *ssh.ExitError.
func (c *Client) Run(ctx context.Context, command string) (Result, error) {
	sess, err := c.session()
	if err != nil {
		return Result{}, err
	}
	defer sess.Close()

	stdout := newCappedBuffer(c.cfg.MaxOutputBytes)
	stderr := newCappedBuffer(c.cfg.MaxOutputBytes)
	sess.Stdout = stdout
	sess.Stderr = stderr

	runCtx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- sess.Run(command) }()

	var res Result
	select {
	case <-runCtx.Done():
		// Closing the session closes the channel, which unblocks Run.
		_ = sess.Close()
		<-done
		res.TimedOut = true
		res.ExitCode = -1
	case runErr := <-done:
		var exitErr *ssh.ExitError
		switch {
		case runErr == nil:
			res.ExitCode = 0
		case errors.As(runErr, &exitErr):
			res.ExitCode = exitErr.ExitStatus()
		default:
			return Result{}, fmt.Errorf("run command: %w", runErr)
		}
	}

	res.Stdout = stdout.String()
	res.Stderr = stderr.String()
	res.Truncated = stdout.Truncated() || stderr.Truncated()
	return res, nil
}

// cappedBuffer buffers writes up to limit bytes, then silently discards the
// rest while flagging truncation. Write never returns an error so the SSH
// stream copy keeps draining the remote output.
type cappedBuffer struct {
	mu        sync.Mutex
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func newCappedBuffer(limit int) *cappedBuffer {
	return &cappedBuffer{limit: limit}
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if remain := b.limit - b.buf.Len(); remain < len(p) {
		if remain > 0 {
			b.buf.Write(p[:remain])
		}
		if len(p) > 0 {
			b.truncated = true
		}
	} else {
		b.buf.Write(p)
	}
	return len(p), nil
}

func (b *cappedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *cappedBuffer) Truncated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.truncated
}
