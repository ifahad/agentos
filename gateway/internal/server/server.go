// Package server implements the gateway HTTP API: the OpenAI-compatible
// chat-completions/embeddings proxy and the admin endpoints.
package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/ifahad/agentos/gateway/internal/guardrail"
	"github.com/ifahad/agentos/gateway/internal/oidc"
	"github.com/ifahad/agentos/gateway/internal/provider"
	"github.com/ifahad/agentos/gateway/internal/ratelimit"
	"github.com/ifahad/agentos/gateway/internal/rbac"
	"github.com/ifahad/agentos/gateway/internal/secret"
	"github.com/ifahad/agentos/gateway/internal/store"
)

// Error type strings from the frozen contract.
const (
	errInvalidKey        = "invalid_key"
	errBudgetExceeded    = "budget_exceeded"
	errOrgBudgetExceeded = "org_budget_exceeded"
	errUnsupported       = "unsupported"
	errProviderError     = "provider_error"
	errGuardrailBlocked  = "guardrail_blocked"
	errForbidden         = "forbidden"
	errNotFound          = "not_found"
	errRateLimited       = "rate_limited"
	errSSODisabled       = "sso_disabled"
	errSSOFailed         = "sso_failed"
)

// Audit listing bounds from the frozen contract.
const (
	auditDefaultLimit = 50
	auditMaxLimit     = 500
)

// Server routes gateway requests. Construct with New.
type Server struct {
	store       store.Store
	router      *provider.Router
	adminKey    string
	client      *http.Client
	guardMode   string
	guard       guardrail.Guardrail
	corsOrigins []string
	tracer      trace.Tracer
	secrets     secret.Source
	secretNames []string
	oidc        *oidc.Provider    // nil = SSO disabled
	limiter     ratelimit.Limiter // per-org buckets (in-memory or postgres)
	defaultRPM  int               // AGENTOS_RATE_LIMIT_RPM; 0 = unlimited
	scimToken   string            // AGENTOS_SCIM_TOKEN; "" disables SCIM
	scimOrg     string            // SCIM default org (AGENTOS_SCIM_DEFAULT_ORG)
	scimRole    string            // SCIM default role (AGENTOS_SCIM_DEFAULT_ROLE)
}

// DefaultSecretNames are the provider keys reported by GET /admin/secrets/status.
var DefaultSecretNames = []string{"AGENTOS_ANTHROPIC_API_KEY", "AGENTOS_OPENAI_API_KEY"}

// Option customizes a Server built by New.
type Option func(*Server)

// WithGuardrails sets the prompt-injection screening mode (off|log|block)
// and the screener implementation.
func WithGuardrails(mode string, g guardrail.Guardrail) Option {
	return func(s *Server) {
		s.guardMode = mode
		s.guard = g
	}
}

// WithCORSOrigins enables CORS for the given allowed origins ("*" allows any).
// An empty list keeps CORS disabled (no headers emitted).
func WithCORSOrigins(origins []string) Option {
	return func(s *Server) { s.corsOrigins = origins }
}

// WithTracer sets the OpenTelemetry tracer used to emit one span per proxied
// request. Without it a no-op tracer is used.
func WithTracer(t trace.Tracer) Option {
	return func(s *Server) { s.tracer = t }
}

// WithSecrets sets the secret source and the secret names surfaced by
// GET /admin/secrets/status. Without it the env backend and provider-key names
// are used, reproducing Phase 1–4 behavior.
func WithSecrets(src secret.Source, names []string) Option {
	return func(s *Server) {
		if src != nil {
			s.secrets = src
		}
		if names != nil {
			s.secretNames = names
		}
	}
}

// WithOIDC enables OpenID Connect SSO with the given (non-nil) provider.
func WithOIDC(p *oidc.Provider) Option {
	return func(s *Server) {
		if p != nil {
			s.oidc = p
		}
	}
}

// WithRateLimits sets the global default requests-per-minute applied to orgs
// whose own rate_limit_rpm is 0. A default of 0 keeps rate limiting off unless
// an org opts in, reproducing Phase 5 behavior.
func WithRateLimits(defaultRPM int) Option {
	return func(s *Server) { s.defaultRPM = defaultRPM }
}

// WithRateLimiter injects a specific limiter — a deterministic-clock in-memory
// limiter (tests) or the Postgres distributed limiter (multi-replica). Without
// it New installs a wall-clock in-memory limiter, reproducing Phase 6 behavior.
func WithRateLimiter(l ratelimit.Limiter) Option {
	return func(s *Server) {
		if l != nil {
			s.limiter = l
		}
	}
}

// WithSCIM enables SCIM 2.0 provisioning. token guards every /scim/v2/* route
// (empty leaves SCIM disabled → those routes 404). defaultOrg and defaultRole
// are where new users land; empty values fall back to the org_default org and
// the member role.
func WithSCIM(token, defaultOrg, defaultRole string) Option {
	return func(s *Server) {
		s.scimToken = token
		if defaultOrg != "" {
			s.scimOrg = defaultOrg
		}
		if defaultRole != "" {
			s.scimRole = defaultRole
		}
	}
}

// New builds a Server. adminKey guards the /admin endpoints.
func New(st store.Store, router *provider.Router, adminKey string, opts ...Option) *Server {
	s := &Server{
		store:       st,
		router:      router,
		adminKey:    adminKey,
		client:      &http.Client{Timeout: 5 * time.Minute},
		guardMode:   guardrail.ModeOff,
		tracer:      noop.NewTracerProvider().Tracer("gateway"),
		secrets:     secret.NewEnv(),
		secretNames: DefaultSecretNames,
		limiter:     ratelimit.New(),
		scimOrg:     store.DefaultOrgID,
		scimRole:    rbac.RoleMember,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Handler returns the gateway's HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("POST /v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("POST /v1/embeddings", s.handleEmbeddings)
	mux.HandleFunc("POST /admin/keys", s.adminAuth(s.handleCreateKey))
	mux.HandleFunc("GET /admin/keys", s.adminAuth(s.handleListKeys))
	mux.HandleFunc("GET /admin/usage", s.adminAuth(s.handleUsage))
	mux.HandleFunc("GET /admin/audit", s.adminAuth(s.handleAudit))
	mux.HandleFunc("GET /admin/whoami", s.adminAuth(s.handleWhoami))
	mux.HandleFunc("POST /admin/orgs", s.adminAuth(s.handleCreateOrg))
	mux.HandleFunc("GET /admin/orgs", s.adminAuth(s.handleListOrgs))
	mux.HandleFunc("PATCH /admin/orgs/{org_id}", s.adminAuth(s.handleUpdateOrg))
	mux.HandleFunc("POST /admin/orgs/{org_id}/users", s.adminAuth(s.handleCreateUser))
	mux.HandleFunc("GET /admin/orgs/{org_id}/users", s.adminAuth(s.handleListUsers))
	mux.HandleFunc("DELETE /admin/orgs/{org_id}/users/{user_id}", s.adminAuth(s.handleDeleteUser))
	mux.HandleFunc("GET /admin/secrets/status", s.adminAuth(s.handleSecretsStatus))
	mux.HandleFunc("POST /admin/secrets/reload", s.adminAuth(s.handleSecretsReload))
	mux.HandleFunc("GET /auth/oidc/status", s.handleOIDCStatus)
	mux.HandleFunc("GET /auth/oidc/login", s.handleOIDCLogin)
	mux.HandleFunc("GET /auth/oidc/callback", s.handleOIDCCallback)
	// SCIM 2.0 provisioning routes are registered only when enabled; when
	// disabled these paths fall through to the mux's 404 (frozen contract).
	if s.scimEnabled() {
		s.registerSCIMRoutes(mux)
	}
	if len(s.corsOrigins) > 0 {
		return s.withCORS(mux)
	}
	return mux
}

// withCORS emits CORS headers for allowed origins and answers OPTIONS
// preflights. Only installed when AGENTOS_CORS_ORIGINS is non-empty.
func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && s.originAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originAllowed(origin string) bool {
	for _, allowed := range s.corsOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, "ok")
}

// bearerToken extracts the token from an Authorization: Bearer header.
func bearerToken(r *http.Request) (string, bool) {
	auth := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
	if token == "" {
		return "", false
	}
	return token, true
}

func (s *Server) handleCreateKey(w http.ResponseWriter, r *http.Request, c *caller) {
	var req struct {
		Name             string  `json:"name"`
		MonthlyBudgetUSD float64 `json:"monthly_budget_usd"`
		OrgID            string  `json:"org_id"`
		RoleScope        string  `json:"role_scope"` // reserved; accepted, currently a no-op
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errUnsupported, "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, errUnsupported, `"name" is required`)
		return
	}

	orgID, createdBy := store.DefaultOrgID, store.RootCreator
	if c.root {
		if req.OrgID != "" {
			if _, err := s.store.Org(r.Context(), req.OrgID); err != nil {
				writeError(w, http.StatusNotFound, errNotFound, fmt.Sprintf("unknown org %q", req.OrgID))
				return
			}
			orgID = req.OrgID
		}
	} else {
		if !c.can(rbac.ActCreateKey) {
			writeForbidden(w, "role lacks create_key capability")
			return
		}
		if req.OrgID != "" && req.OrgID != c.user.OrgID {
			writeForbidden(w, "cannot create keys outside your org")
			return
		}
		orgID, createdBy = c.user.OrgID, c.user.ID
	}

	secret, err := s.store.CreateKeyIn(r.Context(), req.Name, req.MonthlyBudgetUSD, orgID, createdBy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errProviderError, "failed to create key")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"key":                secret,
		"name":               req.Name,
		"monthly_budget_usd": req.MonthlyBudgetUSD,
		"org_id":             orgID,
	})
}

func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request, c *caller) {
	if !c.root && !c.can(rbac.ActListKeys) {
		writeForbidden(w, "role lacks list_keys capability")
		return
	}
	keys, err := s.store.Keys(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, errProviderError, "failed to list keys")
		return
	}
	keys = s.scopeKeys(c, keys)
	if keys == nil {
		keys = []store.KeyInfo{}
	}
	writeJSON(w, http.StatusOK, keys)
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request, c *caller) {
	if !c.root && !c.can(rbac.ActViewUsage) {
		writeForbidden(w, "role lacks view_usage capability")
		return
	}
	usage, err := s.store.Usage(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, errProviderError, "failed to load usage")
		return
	}
	if !c.root {
		names, err := s.orgKeyNames(r.Context(), c.user.OrgID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, errProviderError, "failed to scope usage")
			return
		}
		usage = filterUsage(usage, names)
	}
	if usage == nil {
		usage = []store.KeyUsage{}
	}
	writeJSON(w, http.StatusOK, usage)
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request, c *caller) {
	if !c.root && !c.can(rbac.ActViewAudit) {
		writeForbidden(w, "role lacks view_audit capability")
		return
	}
	limit := auditDefaultLimit
	if q := r.URL.Query().Get("limit"); q != "" {
		n, err := strconv.Atoi(q)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, errUnsupported, `"limit" must be a positive integer`)
			return
		}
		limit = min(n, auditMaxLimit)
	}
	entries, err := s.store.AuditList(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errProviderError, "failed to list audit log")
		return
	}
	if !c.root {
		names, err := s.orgKeyNames(r.Context(), c.user.OrgID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, errProviderError, "failed to scope audit")
			return
		}
		entries = filterAudit(entries, names)
	}
	if entries == nil {
		entries = []store.AuditEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	secret, ok := bearerToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, errInvalidKey, "missing bearer token")
		return
	}
	key, err := s.store.Authenticate(r.Context(), secret)
	if err != nil {
		writeError(w, http.StatusUnauthorized, errInvalidKey, "unknown virtual key")
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, errUnsupported, "invalid JSON body")
		return
	}
	model, _ := body["model"].(string)
	if model == "" {
		writeError(w, http.StatusBadRequest, errUnsupported, `"model" is required`)
		return
	}

	if s.rateLimited(w, r, key, model) {
		return
	}

	if key.SpendUSD >= key.MonthlyBudgetUSD {
		writeError(w, http.StatusPaymentRequired, errBudgetExceeded,
			fmt.Sprintf("monthly budget of $%.2f exhausted for key %q", key.MonthlyBudgetUSD, key.Name))
		return
	}

	if s.orgBudgetExceeded(r.Context(), key.OrgID) {
		writeError(w, http.StatusPaymentRequired, errOrgBudgetExceeded,
			fmt.Sprintf("org %q monthly budget exhausted", key.OrgID))
		return
	}

	if s.guard != nil && s.guardMode != guardrail.ModeOff {
		switch v := s.guard.Screen(latestUserMessage(body)); {
		case v.Errored:
			// Model classifier failed: fail open so the safety layer cannot
			// take down traffic, but leave a visible audit trail.
			s.recordAudit(r, store.Usage{
				KeyName: key.Name, Model: model, Status: http.StatusOK,
				Kind: store.KindGuardrailError,
			})
		case v.Flagged && s.guardMode == guardrail.ModeLog:
			// log mode: audit the flag, forward the request unchanged.
			s.recordAudit(r, store.Usage{
				KeyName: key.Name, Model: model, Status: http.StatusOK,
				Kind: store.KindGuardrailFlag,
			})
		case v.Flagged:
			// block and model modes reject flagged prompts.
			s.recordAudit(r, store.Usage{
				KeyName: key.Name, Model: model, Status: http.StatusBadRequest,
				Kind: store.KindGuardrailBlock,
			})
			writeError(w, http.StatusBadRequest, errGuardrailBlocked,
				fmt.Sprintf("prompt flagged by guardrail rule %q", v.Reason))
			return
		}
	}

	route, err := s.router.Route(model)
	if err != nil {
		writeError(w, http.StatusBadRequest, errUnsupported,
			fmt.Sprintf("unknown provider prefix in model %q", model))
		return
	}

	streaming, _ := body["stream"].(bool)
	if streaming && (route.Provider == "openai" || route.Provider == "ollama") {
		// Ask for the final usage chunk so tokens can be accounted.
		opts, _ := body["stream_options"].(map[string]any)
		if opts == nil {
			opts = map[string]any{}
		}
		opts["include_usage"] = true
		body["stream_options"] = opts
	}

	s.proxy(w, r, proxyRequest{
		span:      "gateway.chat_completions",
		key:       key,
		model:     model,
		route:     route,
		body:      body,
		kind:      store.KindChat,
		streaming: streaming,
	})
}

func (s *Server) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	secret, ok := bearerToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, errInvalidKey, "missing bearer token")
		return
	}
	key, err := s.store.Authenticate(r.Context(), secret)
	if err != nil {
		writeError(w, http.StatusUnauthorized, errInvalidKey, "unknown virtual key")
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, errUnsupported, "invalid JSON body")
		return
	}
	model, _ := body["model"].(string)
	if model == "" {
		writeError(w, http.StatusBadRequest, errUnsupported, `"model" is required`)
		return
	}

	if s.rateLimited(w, r, key, model) {
		return
	}

	if key.SpendUSD >= key.MonthlyBudgetUSD {
		writeError(w, http.StatusPaymentRequired, errBudgetExceeded,
			fmt.Sprintf("monthly budget of $%.2f exhausted for key %q", key.MonthlyBudgetUSD, key.Name))
		return
	}

	if s.orgBudgetExceeded(r.Context(), key.OrgID) {
		writeError(w, http.StatusPaymentRequired, errOrgBudgetExceeded,
			fmt.Sprintf("org %q monthly budget exhausted", key.OrgID))
		return
	}

	route, err := s.router.RouteEmbeddings(model)
	if err != nil {
		writeError(w, http.StatusBadRequest, errUnsupported,
			fmt.Sprintf("unknown provider prefix in model %q", model))
		return
	}

	s.proxy(w, r, proxyRequest{
		span:  "gateway.embeddings",
		key:   key,
		model: model,
		route: route,
		body:  body,
		kind:  store.KindEmbeddings,
	})
}

// proxyRequest carries one authenticated, routed request through the shared
// forwarding pipeline.
type proxyRequest struct {
	span      string
	key       *store.Key
	model     string // original provider-prefixed model, for auditing
	route     *provider.Route
	body      map[string]any
	kind      string
	streaming bool
}

// proxy forwards the request upstream, relays the response (buffered or SSE
// passthrough), and records usage plus a span.
func (s *Server) proxy(w http.ResponseWriter, r *http.Request, p proxyRequest) {
	ctx, span := s.tracer.Start(r.Context(), p.span)
	defer span.End()

	p.body["model"] = p.route.Model
	payload, err := json.Marshal(p.body)
	if err != nil {
		writeError(w, http.StatusBadRequest, errUnsupported, "failed to encode request body")
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.route.URL, bytes.NewReader(payload))
	if err != nil {
		writeError(w, http.StatusBadGateway, errProviderError, "failed to build provider request")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if p.route.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.route.APIKey)
	}

	start := time.Now()
	resp, err := s.client.Do(req)
	latencyMS := time.Since(start).Milliseconds()
	if err != nil {
		s.record(r, span, store.Usage{
			KeyName: p.key.Name, Model: p.model, LatencyMS: latencyMS,
			Status: http.StatusBadGateway, Kind: p.kind,
		})
		writeError(w, http.StatusBadGateway, errProviderError,
			fmt.Sprintf("provider %s unreachable: %v", p.route.Provider, err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		respBody, _ := io.ReadAll(resp.Body)
		latencyMS = time.Since(start).Milliseconds()
		s.record(r, span, store.Usage{
			KeyName: p.key.Name, Model: p.model, LatencyMS: latencyMS,
			Status: resp.StatusCode, Kind: p.kind,
		})
		writeError(w, http.StatusBadGateway, errProviderError,
			fmt.Sprintf("provider %s returned status %d: %s", p.route.Provider, resp.StatusCode, string(respBody)))
		return
	}

	var inputTokens, outputTokens int64
	if p.streaming {
		if ct := resp.Header.Get("Content-Type"); ct != "" {
			w.Header().Set("Content-Type", ct)
		} else {
			w.Header().Set("Content-Type", "text/event-stream")
		}
		w.WriteHeader(resp.StatusCode)
		inputTokens, outputTokens = streamProviderResponse(w, resp.Body)
		latencyMS = time.Since(start).Milliseconds()
	} else {
		respBody, err := io.ReadAll(resp.Body)
		latencyMS = time.Since(start).Milliseconds()
		if err != nil {
			s.record(r, span, store.Usage{
				KeyName: p.key.Name, Model: p.model, LatencyMS: latencyMS,
				Status: http.StatusBadGateway, Kind: p.kind,
			})
			writeError(w, http.StatusBadGateway, errProviderError,
				fmt.Sprintf("failed to read provider %s response: %v", p.route.Provider, err))
			return
		}
		inputTokens, outputTokens = parseUsage(respBody)
		if ct := resp.Header.Get("Content-Type"); ct != "" {
			w.Header().Set("Content-Type", ct)
		} else {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
	}

	s.record(r, span, store.Usage{
		KeyName:      p.key.Name,
		Model:        p.model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CostUSD:      provider.Cost(p.route.Model, inputTokens, outputTokens),
		LatencyMS:    latencyMS,
		Status:       resp.StatusCode,
		Kind:         p.kind,
	})
}

// streamProviderResponse forwards provider SSE bytes verbatim, flushing per
// line, while watching data: lines for the final usage object. Returns zeros
// when the provider never sends usage.
func streamProviderResponse(w http.ResponseWriter, body io.Reader) (inputTokens, outputTokens int64) {
	flusher, _ := w.(http.Flusher)
	reader := bufio.NewReader(body)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if _, werr := w.Write(line); werr != nil {
				// Client went away; keep draining so usage still gets parsed.
				flusher = nil
			}
			if flusher != nil {
				flusher.Flush()
			}
			if in, out, ok := parseSSEUsageLine(line); ok {
				inputTokens, outputTokens = in, out
			}
		}
		if err != nil {
			return inputTokens, outputTokens
		}
	}
}

// parseSSEUsageLine reports token counts when line is an SSE data: line whose
// JSON payload carries a usage object.
func parseSSEUsageLine(line []byte) (inputTokens, outputTokens int64, ok bool) {
	trimmed := bytes.TrimSpace(line)
	if !bytes.HasPrefix(trimmed, []byte("data:")) {
		return 0, 0, false
	}
	payload := bytes.TrimSpace(trimmed[len("data:"):])
	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
		return 0, 0, false
	}
	var parsed struct {
		Usage *usageObject `json:"usage"`
	}
	if err := json.Unmarshal(payload, &parsed); err != nil || parsed.Usage == nil {
		return 0, 0, false
	}
	in, out := parsed.Usage.tokens()
	return in, out, true
}

// latestUserMessage returns the text of the last "user" message in an
// OpenAI-style messages array; string and multimodal-part contents supported.
func latestUserMessage(body map[string]any) string {
	msgs, _ := body["messages"].([]any)
	for i := len(msgs) - 1; i >= 0; i-- {
		m, ok := msgs[i].(map[string]any)
		if !ok || m["role"] != "user" {
			continue
		}
		switch content := m["content"].(type) {
		case string:
			return content
		case []any:
			var b strings.Builder
			for _, part := range content {
				if p, ok := part.(map[string]any); ok {
					if text, ok := p["text"].(string); ok {
						b.WriteString(text)
						b.WriteString("\n")
					}
				}
			}
			return b.String()
		}
		return ""
	}
	return ""
}

// record persists one accounted audit entry and annotates the span. It uses a
// non-cancelable context so audits survive client disconnects mid-stream.
func (s *Server) record(r *http.Request, span trace.Span, u store.Usage) {
	setSpanAttributes(span, u)
	if err := s.store.RecordUsage(context.WithoutCancel(r.Context()), u); err != nil {
		log.Printf("record usage for key %q: %v", u.KeyName, err)
	}
}

// recordAudit persists an audit-only entry (guardrail events): no spend or
// usage aggregates are touched.
func (s *Server) recordAudit(r *http.Request, u store.Usage) {
	if err := s.store.RecordAudit(context.WithoutCancel(r.Context()), u); err != nil {
		log.Printf("record audit for key %q: %v", u.KeyName, err)
	}
}

// effectiveRPM returns the rate limit for a key's org: the org's own
// rate_limit_rpm when positive, else the global default. 0 means unlimited.
func (s *Server) effectiveRPM(ctx context.Context, orgID string) int {
	if org, err := s.store.Org(ctx, orgID); err == nil && org.RateLimitRPM > 0 {
		return org.RateLimitRPM
	}
	return s.defaultRPM
}

// rateLimited enforces the per-org token bucket for a virtual-key caller. It
// runs after authentication and before the budget checks. When the org's bucket
// is empty it writes a 429 with a Retry-After header, audits the rejection with
// kind rate_limited, and returns true. An unlimited org (effective rpm 0) is
// always allowed. The root admin key never reaches here (it is not a virtual
// key), so it is inherently exempt.
func (s *Server) rateLimited(w http.ResponseWriter, r *http.Request, key *store.Key, model string) bool {
	rpm := s.effectiveRPM(r.Context(), key.OrgID)
	if rpm <= 0 {
		return false
	}
	allowed, retryAfter := s.limiter.Allow(key.OrgID, rpm)
	if allowed {
		return false
	}
	s.recordAudit(r, store.Usage{
		KeyName: key.Name, Model: model, Status: http.StatusTooManyRequests,
		Kind: store.KindRateLimited,
	})
	w.Header().Set("Retry-After", strconv.Itoa(ratelimit.RetryAfterSeconds(retryAfter)))
	writeError(w, http.StatusTooManyRequests, errRateLimited,
		fmt.Sprintf("org %q exceeded its rate limit of %d requests/min", key.OrgID, rpm))
	return true
}

func setSpanAttributes(span trace.Span, u store.Usage) {
	span.SetAttributes(
		attribute.String("agentos.model", u.Model),
		attribute.String("agentos.key_name", u.KeyName),
		attribute.Int64("agentos.input_tokens", u.InputTokens),
		attribute.Int64("agentos.output_tokens", u.OutputTokens),
		attribute.Float64("agentos.cost_usd", u.CostUSD),
		attribute.Int("agentos.status", u.Status),
	)
}

// usageObject tolerates both OpenAI-style prompt/completion and Anthropic
// input/output token field names.
type usageObject struct {
	PromptTokens     *int64 `json:"prompt_tokens"`
	CompletionTokens *int64 `json:"completion_tokens"`
	InputTokens      *int64 `json:"input_tokens"`
	OutputTokens     *int64 `json:"output_tokens"`
}

func (u *usageObject) tokens() (inputTokens, outputTokens int64) {
	if u.PromptTokens != nil {
		inputTokens = *u.PromptTokens
	} else if u.InputTokens != nil {
		inputTokens = *u.InputTokens
	}
	if u.CompletionTokens != nil {
		outputTokens = *u.CompletionTokens
	} else if u.OutputTokens != nil {
		outputTokens = *u.OutputTokens
	}
	return inputTokens, outputTokens
}

// parseUsage extracts token counts from a provider response's usage object.
func parseUsage(body []byte) (inputTokens, outputTokens int64) {
	var parsed struct {
		Usage usageObject `json:"usage"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, 0
	}
	return parsed.Usage.tokens()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

// writeError emits the contract error shape:
// {"error":{"type":"<type>","message":"..."}}
func writeError(w http.ResponseWriter, status int, typ, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"type": typ, "message": msg},
	})
}
