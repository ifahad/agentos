// Command gateway runs the AgentOS LLM gateway: an OpenAI-compatible proxy
// with virtual keys, budget caps, usage accounting, and an audit log.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/ifahad/agentos/gateway/internal/provider"
	"github.com/ifahad/agentos/gateway/internal/server"
	"github.com/ifahad/agentos/gateway/internal/store"
)

func main() {
	adminKey := os.Getenv("AGENTOS_ADMIN_KEY")
	if adminKey == "" {
		log.Fatal("AGENTOS_ADMIN_KEY must be set")
	}

	ctx := context.Background()

	var st store.Store
	if dsn := os.Getenv("AGENTOS_DATABASE_URL"); dsn != "" {
		pg, err := store.NewPostgres(ctx, dsn)
		if err != nil {
			log.Fatalf("postgres store: %v", err)
		}
		defer pg.Close()
		st = pg
		log.Println("using postgres store")
	} else {
		st = store.NewMemory()
		log.Println("AGENTOS_DATABASE_URL empty; using in-memory store")
	}

	bootstrap, err := store.ParseBootstrapKeys(os.Getenv("AGENTOS_BOOTSTRAP_KEYS"))
	if err != nil {
		log.Fatalf("AGENTOS_BOOTSTRAP_KEYS: %v", err)
	}
	if err := store.ApplyBootstrapKeys(ctx, st, bootstrap); err != nil {
		log.Fatalf("bootstrap keys: %v", err)
	}
	if len(bootstrap) > 0 {
		log.Printf("bootstrapped %d virtual key(s)", len(bootstrap))
	}

	router := &provider.Router{
		AnthropicAPIKey: os.Getenv("AGENTOS_ANTHROPIC_API_KEY"),
		OpenAIAPIKey:    os.Getenv("AGENTOS_OPENAI_API_KEY"),
		OllamaBaseURL:   os.Getenv("AGENTOS_OLLAMA_BASE_URL"),
	}

	srv := server.New(st, router, adminKey)
	log.Println("gateway listening on :8080")
	if err := http.ListenAndServe(":8080", srv.Handler()); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
