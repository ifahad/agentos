// Command ssh-connector serves the agentos-ssh MCP server: allowlisted
// command execution on one legacy SSH host (run_command, list_allowed) over
// streamable HTTP at /mcp.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/ifahad/agentos/connectors/ssh/internal/sshclient"
	"github.com/ifahad/agentos/connectors/ssh/internal/tools"
)

const (
	serverName    = "agentos-ssh"
	serverVersion = "0.1.0"
	listenAddr    = ":8092"
	endpointPath  = "/mcp"
)

// healthCheckArg makes the binary probe itself. The image is
// gcr.io/distroless/static — no shell, no curl — so the binary is the only
// thing in it that can open a socket.
const healthCheckArg = "-healthcheck"

const healthCheckAddr = "127.0.0.1:8092"

// runHealthCheck reports whether the connector is accepting connections on
// addr: 0 healthy, anything else not.
//
// This is a liveness check and deliberately no more. The MCP endpoint speaks
// streamable HTTP, where a GET is a server-initiated event stream that stays
// open — probing it would hang until the timeout and report a healthy
// connector as failed. A successful dial proves the process is up and
// listening; it does not prove a tool call would succeed.
//
// This connector has no compose service (there is no SSH target in the demo,
// and it fails closed without a known_hosts file), but it ships the same probe
// as its siblings so deploying it needs no extra wiring.
func runHealthCheck(addr string) int {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		log.Printf("healthcheck: %v", err)
		return 1
	}
	_ = conn.Close()
	return 0
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == healthCheckArg {
		os.Exit(runHealthCheck(healthCheckAddr))
	}
	if err := run(); err != nil {
		log.Fatalf("ssh-connector: %v", err)
	}
}

func run() error {
	host := os.Getenv("AGENTOS_SSH_HOST")
	if host == "" {
		return errors.New("AGENTOS_SSH_HOST is required")
	}

	port := 22
	if v := os.Getenv("AGENTOS_SSH_PORT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > 65535 {
			return fmt.Errorf("AGENTOS_SSH_PORT must be a port number, got %q", v)
		}
		port = n
	}

	user := os.Getenv("AGENTOS_SSH_USER")
	if user == "" {
		return errors.New("AGENTOS_SSH_USER is required")
	}

	auth, err := authMethod(os.Getenv("AGENTOS_SSH_PASSWORD"), os.Getenv("AGENTOS_SSH_PRIVATE_KEY"))
	if err != nil {
		return err
	}

	hostKeyCallback, hostKeyMode, err := resolveHostKeyCallback(
		os.Getenv("AGENTOS_SSH_KNOWN_HOSTS"),
		os.Getenv("AGENTOS_SSH_INSECURE_HOST_KEY"),
	)
	if err != nil {
		return err
	}
	if hostKeyMode == hostKeyInsecure {
		log.Print("ssh-connector: WARNING: host-key verification is DISABLED via AGENTOS_SSH_INSECURE_HOST_KEY=true. " +
			"The connection is exposed to man-in-the-middle attacks and the SSH password/session can be captured. " +
			"This is for local development ONLY; set AGENTOS_SSH_KNOWN_HOSTS to a known_hosts file for production.")
	}

	allowlist := parseAllowlist(os.Getenv("AGENTOS_SSH_ALLOW_COMMANDS"))
	if len(allowlist) == 0 {
		log.Print("ssh-connector: WARNING: AGENTOS_SSH_ALLOW_COMMANDS is empty, every command is denied; set a comma-separated list of command basenames (e.g. ls,cat,grep,df,uptime)")
	}

	timeout := sshclient.DefaultTimeout
	if v := os.Getenv("AGENTOS_SSH_TIMEOUT_S"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return fmt.Errorf("AGENTOS_SSH_TIMEOUT_S must be a positive integer, got %q", v)
		}
		timeout = time.Duration(n) * time.Second
	}

	maxOutputBytes := sshclient.DefaultMaxOutputBytes
	if v := os.Getenv("AGENTOS_SSH_MAX_OUTPUT_BYTES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return fmt.Errorf("AGENTOS_SSH_MAX_OUTPUT_BYTES must be a positive integer, got %q", v)
		}
		maxOutputBytes = n
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	client := sshclient.New(sshclient.Config{
		Addr:            addr,
		User:            user,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         timeout,
		MaxOutputBytes:  maxOutputBytes,
	})
	defer client.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mcpServer := server.NewMCPServer(serverName, serverVersion,
		server.WithToolCapabilities(false),
	)
	tools.New(client, allowlist).Register(mcpServer)

	httpServer := server.NewStreamableHTTPServer(mcpServer,
		server.WithEndpointPath(endpointPath),
	)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("ssh-connector: serving MCP (%s) on %s%s, target %s@%s, %d allowed commands, timeout %s",
			serverName, listenAddr, endpointPath, user, addr, len(allowlist), timeout)
		if err := httpServer.Start(listenAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Print("ssh-connector: shutting down")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

// hostKeyMode records how host-key verification was resolved, so callers can
// emit the appropriate warning.
type hostKeyMode int

const (
	// hostKeyStrict verifies against a known_hosts file (fail-closed).
	hostKeyStrict hostKeyMode = iota
	// hostKeyInsecure disables host-key verification (explicit dev opt-out).
	hostKeyInsecure
)

// resolveHostKeyCallback decides the SSH host-key policy fail-closed (H1):
//
//   - known_hosts path set  → strict knownhosts verification;
//   - path unset, insecure opt-out ("true") → InsecureIgnoreHostKey (dev only);
//   - path unset, no opt-out → error (refuse to start).
//
// Both env values unset is the default and returns an error: the connector
// will not run with host-key verification silently disabled.
func resolveHostKeyCallback(knownHostsPath, insecureOptOut string) (ssh.HostKeyCallback, hostKeyMode, error) {
	if knownHostsPath != "" {
		cb, err := knownhosts.New(knownHostsPath)
		if err != nil {
			return nil, hostKeyStrict, fmt.Errorf("AGENTOS_SSH_KNOWN_HOSTS: load %q: %w", knownHostsPath, err)
		}
		return cb, hostKeyStrict, nil
	}
	if strings.EqualFold(strings.TrimSpace(insecureOptOut), "true") {
		return ssh.InsecureIgnoreHostKey(), hostKeyInsecure, nil //nolint:gosec // explicit, loudly-warned dev opt-out
	}
	return nil, hostKeyStrict, errors.New(
		"AGENTOS_SSH_KNOWN_HOSTS is unset: refusing to start with host-key verification disabled (MITM risk). " +
			"Set AGENTOS_SSH_KNOWN_HOSTS to a known_hosts file for strict verification, " +
			"or set AGENTOS_SSH_INSECURE_HOST_KEY=true to explicitly disable host-key checking (INSECURE, dev only).")
}

// authMethod builds the SSH auth from exactly one of password / PEM private
// key. Setting neither or both is a configuration error.
func authMethod(password, privateKeyPEM string) ([]ssh.AuthMethod, error) {
	switch {
	case password != "" && privateKeyPEM != "":
		return nil, errors.New("exactly one of AGENTOS_SSH_PASSWORD or AGENTOS_SSH_PRIVATE_KEY must be set, got both")
	case password != "":
		return []ssh.AuthMethod{ssh.Password(password)}, nil
	case privateKeyPEM != "":
		signer, err := ssh.ParsePrivateKey([]byte(privateKeyPEM))
		if err != nil {
			return nil, fmt.Errorf("AGENTOS_SSH_PRIVATE_KEY: parse PEM: %w", err)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	default:
		return nil, errors.New("exactly one of AGENTOS_SSH_PASSWORD or AGENTOS_SSH_PRIVATE_KEY must be set")
	}
}

// parseAllowlist splits the comma-separated allowlist, trimming whitespace
// and dropping empty entries.
func parseAllowlist(raw string) []string {
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
