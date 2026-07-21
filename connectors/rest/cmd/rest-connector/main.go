// Command rest-connector serves the agentos-rest MCP server: it turns an
// OpenAPI 3 JSON spec into MCP tools (list_operations plus one tool per
// operation) proxied to the upstream API, over streamable HTTP at /mcp.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/ifahad/agentos/connectors/rest/internal/spec"
	"github.com/ifahad/agentos/connectors/rest/internal/tools"
)

const (
	serverName    = "agentos-rest"
	serverVersion = "0.1.0"
	listenAddr    = ":8091"
	endpointPath  = "/mcp"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("rest-connector: %v", err)
	}
}

func run() error {
	specURL := os.Getenv("AGENTOS_REST_SPEC_URL")
	if specURL == "" {
		return errors.New("AGENTOS_REST_SPEC_URL is required (http(s):// URL or file path of an OpenAPI 3 JSON spec)")
	}

	doc, err := spec.Load(specURL)
	if err != nil {
		return err
	}

	baseURL := os.Getenv("AGENTOS_REST_BASE_URL")
	if baseURL == "" {
		if len(doc.Servers) == 0 {
			return errors.New("AGENTOS_REST_BASE_URL is unset and the spec declares no servers")
		}
		baseURL = doc.Servers[0]
	}

	allowMutations := false
	if v := os.Getenv("AGENTOS_REST_ALLOW_MUTATIONS"); v != "" {
		allowMutations, err = strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("AGENTOS_REST_ALLOW_MUTATIONS must be a boolean, got %q", v)
		}
	}

	maxBodyBytes := tools.DefaultMaxBodyBytes
	if v := os.Getenv("AGENTOS_REST_MAX_BODY_BYTES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return fmt.Errorf("AGENTOS_REST_MAX_BODY_BYTES must be a positive integer, got %q", v)
		}
		maxBodyBytes = n
	}

	cfg := tools.Config{
		BaseURL:        baseURL,
		AllowMutations: allowMutations,
		MaxBodyBytes:   maxBodyBytes,
	}
	if raw := os.Getenv("AGENTOS_REST_AUTH_HEADER"); raw != "" {
		name, value, ok := strings.Cut(raw, ":")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return fmt.Errorf(`AGENTOS_REST_AUTH_HEADER must look like "Name: value", got %q`, raw)
		}
		cfg.AuthHeaderName = name
		cfg.AuthHeaderValue = strings.TrimSpace(value)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mcpServer := server.NewMCPServer(serverName, serverVersion,
		server.WithToolCapabilities(false),
	)
	t := tools.New(doc, cfg)
	t.Register(mcpServer)

	httpServer := server.NewStreamableHTTPServer(mcpServer,
		server.WithEndpointPath(endpointPath),
	)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("rest-connector: serving MCP (%s) on %s%s, base URL %s, %d operations (mutations allowed: %t)",
			serverName, listenAddr, endpointPath, baseURL, len(t.Operations()), allowMutations)
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

	log.Print("rest-connector: shutting down")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
