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

func main() {
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
