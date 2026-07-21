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

func main() {
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

	var hostKeyCallback ssh.HostKeyCallback
	if path := os.Getenv("AGENTOS_SSH_KNOWN_HOSTS"); path != "" {
		hostKeyCallback, err = knownhosts.New(path)
		if err != nil {
			return fmt.Errorf("AGENTOS_SSH_KNOWN_HOSTS: load %q: %w", path, err)
		}
	} else {
		log.Print("ssh-connector: WARNING: AGENTOS_SSH_KNOWN_HOSTS is unset, host key checking is DISABLED (InsecureIgnoreHostKey); set it to a known_hosts file for strict checking")
		hostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec // deliberate, warned about above
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
