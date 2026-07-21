// Command sql-connector serves the agentos-sql MCP server: read-only SQL
// tools (list_tables, describe_table, query) over streamable HTTP at /mcp.
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

func main() {
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
	tools.New(pool, maxRows).Register(mcpServer)

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
