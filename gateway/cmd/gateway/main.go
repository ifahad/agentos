// Command gateway runs the AgentOS LLM gateway: an OpenAI-compatible proxy
// with virtual keys, budget caps, usage accounting, and an audit log.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/ifahad/agentos/gateway/internal/guardrail"
	"github.com/ifahad/agentos/gateway/internal/provider"
	"github.com/ifahad/agentos/gateway/internal/server"
	"github.com/ifahad/agentos/gateway/internal/store"
	"github.com/ifahad/agentos/gateway/internal/telemetry"
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

	var opts []server.Option

	guardMode := os.Getenv("AGENTOS_GUARDRAILS_MODE")
	if guardMode == "" {
		guardMode = guardrail.ModeOff
	}
	if !guardrail.ValidMode(guardMode) {
		log.Fatalf("AGENTOS_GUARDRAILS_MODE must be off, log, block, or model (got %q)", guardMode)
	}
	switch guardMode {
	case guardrail.ModeOff:
		// guardrails disabled
	case guardrail.ModeModel:
		opts = append(opts, server.WithGuardrails(guardMode, buildModelGuardrail(ctx, st, router)))
		log.Printf("guardrails enabled (mode=%s)", guardMode)
	default:
		opts = append(opts, server.WithGuardrails(guardMode, guardrail.NewHeuristicScreen()))
		log.Printf("guardrails enabled (mode=%s)", guardMode)
	}

	if raw := os.Getenv("AGENTOS_CORS_ORIGINS"); raw != "" {
		var origins []string
		for _, o := range strings.Split(raw, ",") {
			if o = strings.TrimSpace(o); o != "" {
				origins = append(origins, o)
			}
		}
		if len(origins) > 0 {
			opts = append(opts, server.WithCORSOrigins(origins))
			log.Printf("CORS enabled for %d origin(s)", len(origins))
		}
	}

	if endpoint := os.Getenv("AGENTOS_OTEL_ENDPOINT"); endpoint != "" {
		tracer, shutdown, err := telemetry.Setup(ctx, endpoint)
		if err != nil {
			log.Fatalf("AGENTOS_OTEL_ENDPOINT: %v", err)
		}
		defer func() {
			if err := shutdown(ctx); err != nil {
				log.Printf("otel shutdown: %v", err)
			}
		}()
		opts = append(opts, server.WithTracer(tracer))
		log.Printf("otel tracing enabled (endpoint=%s)", endpoint)
	}

	srv := server.New(st, router, adminKey, opts...)
	log.Println("gateway listening on :8080")
	if err := http.ListenAndServe(":8080", srv.Handler()); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

// buildModelGuardrail wires the AGENTOS_GUARDRAILS_MODE=model screener:
// heuristic first, then a classifier model called straight through the
// provider layer. When the classifier model has no usable provider key the
// gateway warns and screens heuristic-only. AGENTOS_GUARDRAILS_KEY, when it
// names a known virtual key, attributes classifier spend in /admin/usage.
func buildModelGuardrail(ctx context.Context, st store.Store, router *provider.Router) guardrail.Guardrail {
	heuristic := guardrail.NewHeuristicScreen()
	model := os.Getenv("AGENTOS_GUARDRAILS_MODEL")
	if model == "" {
		model = guardrail.DefaultModel
	}

	route, err := router.Route(model)
	if err != nil || (route.Provider != "ollama" && route.APIKey == "") {
		log.Printf("WARNING: guardrail classifier model %q is not configured (no provider key); screening is heuristic-only", model)
		return heuristic
	}

	classifier := &guardrail.ProviderClassifier{Router: router}
	if secret := os.Getenv("AGENTOS_GUARDRAILS_KEY"); secret != "" {
		key, err := st.Authenticate(ctx, secret)
		if err != nil {
			log.Printf("WARNING: AGENTOS_GUARDRAILS_KEY is not a known virtual key; classifier spend will not appear in /admin/usage")
		} else {
			guardModel := model
			classifier.OnUsage = func(strippedModel string, inputTokens, outputTokens int64) {
				if err := st.RecordUsage(context.Background(), store.Usage{
					KeyName:      key.Name,
					Model:        guardModel,
					InputTokens:  inputTokens,
					OutputTokens: outputTokens,
					CostUSD:      provider.Cost(strippedModel, inputTokens, outputTokens),
					Status:       http.StatusOK,
					Kind:         store.KindChat,
				}); err != nil {
					log.Printf("record guardrail classifier usage: %v", err)
				}
			}
		}
	} else {
		log.Printf("WARNING: AGENTOS_GUARDRAILS_KEY is empty; classifier spend will not appear in /admin/usage")
	}

	log.Printf("guardrail classifier model %q wired via provider layer", model)
	return guardrail.NewModelScreen(heuristic, classifier, model)
}
