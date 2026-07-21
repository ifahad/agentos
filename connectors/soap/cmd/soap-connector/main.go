// Command soap-connector serves the agentos-soap MCP server: it turns a WSDL
// 1.1 document into MCP tools (list_operations plus one tool per allowed SOAP
// operation) that build SOAP 1.1 envelopes and POST them to the service
// endpoint, over streamable HTTP at /mcp.
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

	"github.com/ifahad/agentos/connectors/soap/internal/tools"
	"github.com/ifahad/agentos/connectors/soap/internal/wsdl"
)

const (
	serverName    = "agentos-soap"
	serverVersion = "0.1.0"
	listenAddr    = ":8093"
	endpointPath  = "/mcp"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("soap-connector: %v", err)
	}
}

func run() error {
	wsdlURL := os.Getenv("AGENTOS_SOAP_WSDL_URL")
	if wsdlURL == "" {
		return errors.New("AGENTOS_SOAP_WSDL_URL is required (http(s):// URL or file path of a WSDL 1.1 document)")
	}

	def, err := wsdl.Load(wsdlURL)
	if err != nil {
		return err
	}

	endpoint := os.Getenv("AGENTOS_SOAP_ENDPOINT")
	if endpoint == "" {
		endpoint = def.Endpoint
	}
	if endpoint == "" {
		return errors.New("AGENTOS_SOAP_ENDPOINT is unset and the WSDL declares no soap:address location")
	}

	timeout := time.Duration(tools.DefaultTimeoutSeconds) * time.Second
	if v := os.Getenv("AGENTOS_SOAP_TIMEOUT_S"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return fmt.Errorf("AGENTOS_SOAP_TIMEOUT_S must be a positive integer, got %q", v)
		}
		timeout = time.Duration(n) * time.Second
	}

	maxBodyBytes := tools.DefaultMaxBodyBytes
	if v := os.Getenv("AGENTOS_SOAP_MAX_BODY_BYTES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return fmt.Errorf("AGENTOS_SOAP_MAX_BODY_BYTES must be a positive integer, got %q", v)
		}
		maxBodyBytes = n
	}

	cfg := tools.Config{
		Endpoint:     endpoint,
		Timeout:      timeout,
		MaxBodyBytes: maxBodyBytes,
	}
	if raw := os.Getenv("AGENTOS_SOAP_ALLOW_OPERATIONS"); raw != "" {
		for _, name := range strings.Split(raw, ",") {
			if name = strings.TrimSpace(name); name != "" {
				cfg.AllowOperations = append(cfg.AllowOperations, name)
			}
		}
	}
	if raw := os.Getenv("AGENTOS_SOAP_AUTH_HEADER"); raw != "" {
		name, value, ok := strings.Cut(raw, ":")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return fmt.Errorf(`AGENTOS_SOAP_AUTH_HEADER must look like "Name: value", got %q`, raw)
		}
		cfg.AuthHeaderName = name
		cfg.AuthHeaderValue = strings.TrimSpace(value)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mcpServer := server.NewMCPServer(serverName, serverVersion,
		server.WithToolCapabilities(false),
	)
	t := tools.New(def, cfg)
	t.Register(mcpServer)

	httpServer := server.NewStreamableHTTPServer(mcpServer,
		server.WithEndpointPath(endpointPath),
	)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("soap-connector: serving MCP (%s) on %s%s, endpoint %s, SOAP %s, %d operations",
			serverName, listenAddr, endpointPath, endpoint, def.SOAPVersion, len(t.Operations()))
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

	log.Print("soap-connector: shutting down")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
