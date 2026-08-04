// Command sql-connector serves the agentos-sql MCP server: read-only SQL
// tools (list_tables, describe_table, query) over streamable HTTP at /mcp.
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
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/server"

	"github.com/ifahad/agentos/connectors/sql/internal/tools"
)

const (
	serverName    = "agentos-sql"
	serverVersion = "0.1.0"
	listenAddr    = ":8090"
	endpointPath  = "/mcp"
)

// healthCheckArg makes the binary probe itself. The image is
// gcr.io/distroless/static — no shell, no curl — so the binary is the only
// thing in it that can open a socket.
const healthCheckArg = "-healthcheck"

const healthCheckAddr = "127.0.0.1:8090"

// runHealthCheck reports whether the connector is accepting connections on
// addr: 0 healthy, anything else not.
//
// This is a liveness check and deliberately no more. The MCP endpoint speaks
// streamable HTTP, where a GET is a server-initiated event stream that stays
// open — probing it would hang until the timeout and report a healthy
// connector as failed. A successful dial proves the process is up and
// listening; it does not prove a tool call would succeed.
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
		log.Fatalf("sql-connector: %v", err)
	}
}

func run() error {
	databaseURL := os.Getenv("AGENTOS_CONNECTOR_DATABASE_URL")
	if databaseURL == "" {
		return errors.New("AGENTOS_CONNECTOR_DATABASE_URL is required")
	}

	maxRows := tools.DefaultMaxRows
	if v := os.Getenv("AGENTOS_CONNECTOR_MAX_ROWS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return fmt.Errorf("AGENTOS_CONNECTOR_MAX_ROWS must be a positive integer, got %q", v)
		}
		maxRows = n
	}

	// Bounds how long one agent-issued query may run. The row cap limits how
	// much comes back but not how long it takes to get there, and cancelling
	// the client does not stop a Postgres backend already executing.
	stmtTimeout := tools.DefaultStatementTimeout
	if v := os.Getenv("AGENTOS_CONNECTOR_STATEMENT_TIMEOUT_S"); v != "" {
		secs, err := strconv.Atoi(v)
		if err != nil || secs < 0 {
			return fmt.Errorf("AGENTOS_CONNECTOR_STATEMENT_TIMEOUT_S must be a non-negative integer, got %q", v)
		}
		stmtTimeout = time.Duration(secs) * time.Second
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	mcpServer := server.NewMCPServer(serverName, serverVersion,
		server.WithToolCapabilities(false),
	)
	tools.New(pool, maxRows).WithStatementTimeout(stmtTimeout).Register(mcpServer)

	httpServer := server.NewStreamableHTTPServer(mcpServer,
		server.WithEndpointPath(endpointPath),
	)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("sql-connector: serving MCP (%s) on %s%s, max rows %d", serverName, listenAddr, endpointPath, maxRows)
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

	log.Print("sql-connector: shutting down")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
