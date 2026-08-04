// Command gateway runs the AgentOS LLM gateway: an OpenAI-compatible proxy
// with virtual keys, budget caps, usage accounting, and an audit log.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ifahad/agentos/gateway/internal/guardrail"
	"github.com/ifahad/agentos/gateway/internal/oidc"
	"github.com/ifahad/agentos/gateway/internal/provider"
	"github.com/ifahad/agentos/gateway/internal/ratelimit"
	"github.com/ifahad/agentos/gateway/internal/secret"
	"github.com/ifahad/agentos/gateway/internal/server"
	"github.com/ifahad/agentos/gateway/internal/store"
	"github.com/ifahad/agentos/gateway/internal/telemetry"
)

// healthCheckArg is the argument a container probe passes to make the binary
// check itself. The gateway image is gcr.io/distroless/static — no shell and no
// curl — so the binary is the only thing in it that can make an HTTP request,
// and compose runs it against itself rather than the image growing a probe tool
// it would otherwise never use.
const healthCheckArg = "-healthcheck"

// healthCheckURL is where the probe looks. main serves on :8080 unconditionally.
const healthCheckURL = "http://127.0.0.1:8080/healthz"

// runHealthCheck probes the given URL and returns the exit code a container
// healthcheck expects: 0 healthy, anything else not. It reads no configuration,
// so a gateway that failed to configure itself still reports unhealthy rather
// than leaving the probe unable to run at all.
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
	// Before any configuration is read: the probe must work regardless of it.
	if len(os.Args) > 1 && os.Args[1] == healthCheckArg {
		os.Exit(runHealthCheck(healthCheckURL))
	}

	adminKey := os.Getenv("AGENTOS_ADMIN_KEY")
	if adminKey == "" {
		log.Fatal("AGENTOS_ADMIN_KEY must be set")
	}

	ctx := context.Background()

	var st store.Store
	var pgStore *store.Postgres // non-nil only with the postgres store; used by the distributed limiter
	if dsn := os.Getenv("AGENTOS_DATABASE_URL"); dsn != "" {
		pg, err := store.NewPostgres(ctx, dsn)
		if err != nil {
			log.Fatalf("postgres store: %v", err)
		}
		defer pg.Close()
		st = pg
		pgStore = pg
		log.Println("using postgres store")
	} else {
		st = store.NewMemory()
		log.Println("AGENTOS_DATABASE_URL empty; using in-memory store")
	}

	// Bootstrap the default org that owns every pre-existing key. Its budget is
	// 0 (unlimited), so Phase 1–4 keys behave exactly as before.
	bootstrapOrg := os.Getenv("AGENTOS_BOOTSTRAP_ORG")
	if bootstrapOrg == "" {
		bootstrapOrg = "default"
	}
	if err := st.EnsureOrg(ctx, store.DefaultOrgID, bootstrapOrg, 0); err != nil {
		log.Fatalf("bootstrap org: %v", err)
	}

	bootstrap, err := store.ParseBootstrapKeys(os.Getenv("AGENTOS_BOOTSTRAP_KEYS"))
	if err != nil {
		log.Fatalf("AGENTOS_BOOTSTRAP_KEYS: %v", err)
	}
	if err := store.ApplyBootstrapKeys(ctx, st, bootstrap); err != nil {
		log.Fatalf("bootstrap keys: %v", err)
	}
	if len(bootstrap) > 0 {
		log.Printf("bootstrapped %d virtual key(s) into org %q", len(bootstrap), store.DefaultOrgID)
	}

	// Resolve provider keys through the secret source (default env backend
	// reproduces current behavior exactly). Misconfig is fatal.
	secrets, err := secret.FromEnv()
	if err != nil {
		log.Fatalf("secrets backend: %v", err)
	}
	log.Printf("secrets backend: %s", secrets.Backend())
	anthropicKey, _ := secrets.Get("AGENTOS_ANTHROPIC_API_KEY")
	openaiKey, _ := secrets.Get("AGENTOS_OPENAI_API_KEY")

	// Operator-configured OpenAI-compatible providers. A missing file is fine
	// (built-ins only); per-entry validation errors are logged and skipped so
	// one bad entry never blocks startup.
	providersPath := os.Getenv("AGENTOS_PROVIDERS_FILE")
	if providersPath == "" {
		providersPath = "providers.json"
	}
	registry, regErrs := provider.LoadRegistry(providersPath)
	for _, err := range regErrs {
		log.Printf("provider registry: %v", err)
	}
	if names := registry.Names(); len(names) > 0 {
		log.Printf("provider registry: %d provider(s) loaded from %s: %v",
			len(names), providersPath, names)
	}

	router := &provider.Router{
		AnthropicAPIKey: anthropicKey,
		OpenAIAPIKey:    openaiKey,
		OllamaBaseURL:   os.Getenv("AGENTOS_OLLAMA_BASE_URL"),
		// Resolve provider keys from the live secret source on every Route call
		// so POST /admin/secrets/reload and the refresh loop rotate the upstream
		// key without a restart (H5). The static fields above remain a fallback.
		Secrets:  secrets,
		Registry: registry,
	}

	opts := []server.Option{
		server.WithSecrets(secrets, server.DefaultSecretNames),
		server.WithProviders(registry),
	}

	// The council/* model is served by the runtime's council API. Enabled only
	// when the runtime URL is set; the runtime auth token is the same one the
	// console injects, so the gateway authenticates to the runtime the same way.
	if runtimeURL := os.Getenv("AGENTOS_COUNCIL_RUNTIME_URL"); runtimeURL != "" {
		opts = append(opts, server.WithCouncil(runtimeURL, os.Getenv("AGENTOS_RUNTIME_AUTH_TOKEN")))
		log.Printf("council model enabled via runtime %s", runtimeURL)
	}

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

	// Per-tenant rate limits (Phase 6). Default 0 = unlimited unless an org
	// opts in, so behavior is unchanged when the env is unset.
	if raw := os.Getenv("AGENTOS_RATE_LIMIT_RPM"); raw != "" {
		rpm, err := strconv.Atoi(raw)
		if err != nil || rpm < 0 {
			log.Fatalf("AGENTOS_RATE_LIMIT_RPM must be a non-negative integer (got %q)", raw)
		}
		opts = append(opts, server.WithRateLimits(rpm))
		if rpm > 0 {
			log.Printf("default rate limit: %d requests/min per org", rpm)
		}
	}

	// Audit retention. Off by default and deliberately so: the audit log is
	// evidence of what every key did, and a gateway that silently discards it
	// out of the box would be a worse failure than an oversized table. Set a
	// retention window only when you have decided how long you must keep it.
	if raw := os.Getenv("AGENTOS_AUDIT_RETENTION_DAYS"); raw != "" {
		days, err := strconv.Atoi(raw)
		if err != nil || days < 0 {
			log.Fatalf("AGENTOS_AUDIT_RETENTION_DAYS must be a non-negative integer (got %q)", raw)
		}
		if days > 0 {
			startAuditRetention(ctx, st, time.Duration(days)*24*time.Hour)
			log.Printf("audit retention: pruning entries older than %d day(s)", days)
		}
	}

	// Request body cap. Without one, every JSON decode reads until the client
	// stops sending, so a single request can drive memory to whatever an
	// attacker is willing to upload.
	if raw := os.Getenv("AGENTOS_MAX_BODY_BYTES"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			log.Fatalf("AGENTOS_MAX_BODY_BYTES must be a non-negative integer (got %q)", raw)
		}
		opts = append(opts, server.WithMaxBodyBytes(n))
		log.Printf("max request body: %d bytes", n)
	}

	// Budget hold per in-flight request. Budgets cannot be enforced exactly —
	// a call's real cost is unknown until the provider answers — so admission
	// holds this much and settles afterwards. Raise it for expensive models.
	if raw := os.Getenv("AGENTOS_BUDGET_RESERVE_USD"); raw != "" {
		reserve, err := strconv.ParseFloat(raw, 64)
		if err != nil || reserve < 0 {
			log.Fatalf("AGENTOS_BUDGET_RESERVE_USD must be a non-negative number (got %q)", raw)
		}
		opts = append(opts, server.WithBudgetReserve(reserve))
		log.Printf("budget reserve: $%.4f held per in-flight request", reserve)
	}

	// Rate-limit backend selection (Phase 7). Default "memory" is the Phase 6
	// in-process limiter (unchanged). "postgres" shares one bucket per org across
	// gateway replicas via atomic SQL, and requires the Postgres store.
	rlBackend := os.Getenv("AGENTOS_RATELIMIT_BACKEND")
	if rlBackend == "" {
		rlBackend = "memory"
	}
	switch rlBackend {
	case "memory":
		// default in-process limiter; nothing to wire
	case "postgres":
		if pgStore == nil {
			log.Fatal("AGENTOS_RATELIMIT_BACKEND=postgres requires the Postgres store (set AGENTOS_DATABASE_URL)")
		}
		pgLimiter, err := ratelimit.NewPostgres(ctx, pgStore.Pool())
		if err != nil {
			log.Fatalf("postgres rate-limit backend: %v", err)
		}
		opts = append(opts, server.WithRateLimiter(pgLimiter))
		log.Println("rate-limit backend: postgres (distributed)")
	default:
		log.Fatalf("AGENTOS_RATELIMIT_BACKEND must be memory or postgres (got %q)", rlBackend)
	}

	// SCIM 2.0 provisioning (Phase 7). Enabled only when AGENTOS_SCIM_TOKEN is
	// set; unset leaves every /scim/v2/* route 404 (unchanged behavior).
	if scimToken := os.Getenv("AGENTOS_SCIM_TOKEN"); scimToken != "" {
		scimOrg := os.Getenv("AGENTOS_SCIM_DEFAULT_ORG")
		if scimOrg == "" {
			scimOrg = store.DefaultOrgID
		}
		scimRole := os.Getenv("AGENTOS_SCIM_DEFAULT_ROLE")
		if scimRole == "" {
			scimRole = "member"
		}
		// Ensure the SCIM landing org exists so provisioning never 500s on a
		// missing org. The default org is already bootstrapped above.
		if scimOrg != store.DefaultOrgID {
			if err := st.EnsureOrg(ctx, scimOrg, scimOrg, 0); err != nil {
				log.Fatalf("ensure SCIM default org: %v", err)
			}
		}
		opts = append(opts, server.WithSCIM(scimToken, scimOrg, scimRole))
		log.Printf("SCIM provisioning enabled (default org=%s, role=%s)", scimOrg, scimRole)
	}

	// OpenID Connect SSO (Phase 6). Enabled only when AGENTOS_OIDC_ISSUER is
	// set; misconfiguration (discovery failure, missing client id/secret) is
	// fatal at startup.
	oidcProvider, oidcEnabled, err := oidc.FromEnv(ctx, adminKey)
	if err != nil {
		log.Fatalf("OIDC: %v", err)
	}
	if oidcEnabled {
		opts = append(opts, server.WithOIDC(oidcProvider))
		log.Printf("OIDC SSO enabled (issuer=%s)", oidcProvider.Issuer())
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

	// Optional background secret refresh (Phase 7). 0 (default) = off. When > 0
	// and the backend is Reloadable, a goroutine re-fetches on the interval so a
	// rotated key takes effect without a restart.
	if raw := os.Getenv("AGENTOS_SECRETS_REFRESH_S"); raw != "" {
		secs, err := strconv.Atoi(raw)
		if err != nil || secs < 0 {
			log.Fatalf("AGENTOS_SECRETS_REFRESH_S must be a non-negative integer (got %q)", raw)
		}
		if secs > 0 {
			srv.StartSecretsRefresh(ctx, time.Duration(secs)*time.Second)
			log.Printf("secrets refresh enabled: every %ds", secs)
		}
	}

	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: srv.Handler(),
		// Slow-loris defence: a client that opens a connection and dribbles
		// headers holds a goroutine forever without this.
		ReadHeaderTimeout: 10 * time.Second,
		// Bodies are prompts and documents, not uploads, so a minute is ample.
		ReadTimeout: 60 * time.Second,
		// WriteTimeout is deliberately UNSET. It bounds the time from the end of
		// the request headers to the end of the response, which for a streamed
		// completion is the whole generation — setting it would sever long SSE
		// responses mid-token. Runaway upstreams are bounded instead by the
		// proxy client's own 5-minute timeout.
		IdleTimeout: 120 * time.Second,
	}

	// Serve until the process is asked to stop, then drain.
	serveErr := make(chan error, 1)
	go func() {
		log.Println("gateway listening on :8080")
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serveErr:
		if err != nil {
			log.Fatalf("listen: %v", err)
		}
		return
	case <-stopCtx.Done():
	}

	// Graceful shutdown matters here beyond tidiness: in-flight requests hold
	// budget reservations that are only released when they finish. Killing them
	// mid-flight would strand those holds until they expire, shrinking the
	// affected keys' budgets in the meantime.
	log.Println("gateway shutting down; draining in-flight requests")
	drainCtx, cancelDrain := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelDrain()
	if err := httpServer.Shutdown(drainCtx); err != nil {
		log.Printf("shutdown: %v", err)
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
	if s := os.Getenv("AGENTOS_GUARDRAILS_MAX_TOKENS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			classifier.MaxTokens = n
		} else {
			log.Printf("WARNING: invalid AGENTOS_GUARDRAILS_MAX_TOKENS %q; using default", s)
		}
	}
	if secret := os.Getenv("AGENTOS_GUARDRAILS_KEY"); secret != "" {
		key, err := st.Authenticate(ctx, secret)
		if err != nil {
			log.Printf("WARNING: AGENTOS_GUARDRAILS_KEY is not a known virtual key; classifier spend will not appear in /admin/usage")
		} else {
			guardModel := model
			classifier.OnUsage = func(_ string, inputTokens, outputTokens int64) {
				if err := st.RecordUsage(context.Background(), store.Usage{
					SecretHash:   key.SecretHash,
					OrgID:        key.OrgID,
					KeyName:      key.Name,
					Model:        guardModel,
					InputTokens:  inputTokens,
					OutputTokens: outputTokens,
					// route is the classifier's own route, resolved above; it
					// carries the correct price for this exact model.
					CostUSD: route.Cost(inputTokens, outputTokens),
					Status:  http.StatusOK,
					Kind:    store.KindChat,
				}); err != nil {
					log.Printf("record guardrail classifier usage: %v", err)
				}
			}
		}
	} else {
		log.Printf("WARNING: AGENTOS_GUARDRAILS_KEY is empty; classifier spend will not appear in /admin/usage")
	}

	timeout := time.Duration(0)
	if s := os.Getenv("AGENTOS_GUARDRAILS_TIMEOUT_S"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			timeout = time.Duration(n) * time.Second
		} else {
			log.Printf("WARNING: invalid AGENTOS_GUARDRAILS_TIMEOUT_S %q; using default", s)
		}
	}

	log.Printf("guardrail classifier model %q wired via provider layer", model)
	return guardrail.NewModelScreenWithTimeout(heuristic, classifier, model, timeout)
}

// startAuditRetention prunes audit rows older than retention, once at start-up
// and daily thereafter.
//
// Pruning runs in the background rather than on the request path so a large
// first sweep on a long-neglected table cannot add latency to live traffic. A
// failed sweep is logged and retried on the next tick: falling behind on
// retention is a housekeeping problem, never a reason to stop serving.
func startAuditRetention(ctx context.Context, st store.Store, retention time.Duration) {
	prune := func() {
		cutoff := time.Now().Add(-retention)
		removed, err := st.PruneAudit(ctx, cutoff)
		if err != nil {
			log.Printf("audit retention: prune failed: %v", err)
			return
		}
		if removed > 0 {
			log.Printf("audit retention: pruned %d entr(ies) older than %s", removed, cutoff.Format(time.RFC3339))
		}
	}
	go func() {
		prune()
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				prune()
			}
		}
	}()
}
