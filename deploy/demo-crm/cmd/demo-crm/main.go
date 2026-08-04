// Command demo-crm serves the tiny "legacy CRM" REST API used by the AgentOS
// Phase 3 demo: static customers and tickets (consistent with the legacy ERP
// seed) plus a hand-written OpenAPI spec at /openapi.json for the
// rest-connector to consume. Stdlib only.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ifahad/agentos/deploy/demo-crm/internal/crm"
)

const listenAddr = ":8095"

// healthCheckArg makes the binary probe itself. The image is
// gcr.io/distroless/static — no shell, no curl — so the binary is the only
// thing in it that can make an HTTP request.
const healthCheckArg = "-healthcheck"

const healthCheckURL = "http://127.0.0.1:8095/healthz"

// runHealthCheck returns the exit code a container healthcheck expects:
// 0 healthy, anything else not.
func runHealthCheck(url string) int {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("healthcheck: %v", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("healthcheck: %s -> %d", url, resp.StatusCode)
		return 1
	}
	return 0
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == healthCheckArg {
		os.Exit(runHealthCheck(healthCheckURL))
	}
	if err := run(); err != nil {
		log.Fatalf("demo-crm: %v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpServer := &http.Server{
		Addr:              listenAddr,
		Handler:           crm.NewHandler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("demo-crm: serving on %s", listenAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

	log.Print("demo-crm: shutting down")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
