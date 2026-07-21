// Package server implements the gateway HTTP API: the OpenAI-compatible
// chat-completions proxy and the admin endpoints.
package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ifahad/agentos/gateway/internal/provider"
	"github.com/ifahad/agentos/gateway/internal/store"
)

// Error type strings from the frozen contract.
const (
	errInvalidKey     = "invalid_key"
	errBudgetExceeded = "budget_exceeded"
	errUnsupported    = "unsupported"
	errProviderError  = "provider_error"
)

// Server routes gateway requests. Construct with New.
type Server struct {
	store    store.Store
	router   *provider.Router
	adminKey string
	client   *http.Client
}

// New builds a Server. adminKey guards the /admin endpoints.
func New(st store.Store, router *provider.Router, adminKey string) *Server {
	return &Server{
		store:    st,
		router:   router,
		adminKey: adminKey,
		client:   &http.Client{Timeout: 5 * time.Minute},
	}
}

// Handler returns the gateway's HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("POST /v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("POST /admin/keys", s.adminOnly(s.handleCreateKey))
	mux.HandleFunc("GET /admin/keys", s.adminOnly(s.handleListKeys))
	mux.HandleFunc("GET /admin/usage", s.adminOnly(s.handleUsage))
	return mux
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

func (s *Server) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok || s.adminKey == "" || token != s.adminKey {
			writeError(w, http.StatusUnauthorized, errInvalidKey, "invalid admin key")
			return
		}
		next(w, r)
	}
}

func (s *Server) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name             string  `json:"name"`
		MonthlyBudgetUSD float64 `json:"monthly_budget_usd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errUnsupported, "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, errUnsupported, `"name" is required`)
		return
	}
	secret, err := s.store.CreateKey(r.Context(), req.Name, req.MonthlyBudgetUSD)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errProviderError, "failed to create key")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"key":                secret,
		"name":               req.Name,
		"monthly_budget_usd": req.MonthlyBudgetUSD,
	})
}

func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.store.Keys(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, errProviderError, "failed to list keys")
		return
	}
	if keys == nil {
		keys = []store.KeyInfo{}
	}
	writeJSON(w, http.StatusOK, keys)
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	usage, err := s.store.Usage(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, errProviderError, "failed to load usage")
		return
	}
	if usage == nil {
		usage = []store.KeyUsage{}
	}
	writeJSON(w, http.StatusOK, usage)
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
	if streaming, _ := body["stream"].(bool); streaming {
		writeError(w, http.StatusBadRequest, errUnsupported, "streaming is not supported")
		return
	}
	model, _ := body["model"].(string)
	if model == "" {
		writeError(w, http.StatusBadRequest, errUnsupported, `"model" is required`)
		return
	}

	if key.SpendUSD >= key.MonthlyBudgetUSD {
		writeError(w, http.StatusPaymentRequired, errBudgetExceeded,
			fmt.Sprintf("monthly budget of $%.2f exhausted for key %q", key.MonthlyBudgetUSD, key.Name))
		return
	}

	route, err := s.router.Route(model)
	if err != nil {
		writeError(w, http.StatusBadRequest, errUnsupported,
			fmt.Sprintf("unknown provider prefix in model %q", model))
		return
	}

	body["model"] = route.Model
	payload, err := json.Marshal(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, errUnsupported, "failed to encode request body")
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, route.URL, strings.NewReader(string(payload)))
	if err != nil {
		writeError(w, http.StatusBadGateway, errProviderError, "failed to build provider request")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if route.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+route.APIKey)
	}

	start := time.Now()
	resp, err := s.client.Do(req)
	latencyMS := time.Since(start).Milliseconds()
	if err != nil {
		s.record(r, store.Usage{
			KeyName: key.Name, Model: model, LatencyMS: latencyMS, Status: http.StatusBadGateway,
		})
		writeError(w, http.StatusBadGateway, errProviderError,
			fmt.Sprintf("provider %s unreachable: %v", route.Provider, err))
		return
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	latencyMS = time.Since(start).Milliseconds()
	if err != nil {
		s.record(r, store.Usage{
			KeyName: key.Name, Model: model, LatencyMS: latencyMS, Status: http.StatusBadGateway,
		})
		writeError(w, http.StatusBadGateway, errProviderError,
			fmt.Sprintf("failed to read provider %s response: %v", route.Provider, err))
		return
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		s.record(r, store.Usage{
			KeyName: key.Name, Model: model, LatencyMS: latencyMS, Status: resp.StatusCode,
		})
		writeError(w, http.StatusBadGateway, errProviderError,
			fmt.Sprintf("provider %s returned status %d: %s", route.Provider, resp.StatusCode, string(respBody)))
		return
	}

	inputTokens, outputTokens := parseUsage(respBody)
	cost := provider.Cost(route.Model, inputTokens, outputTokens)
	s.record(r, store.Usage{
		KeyName:      key.Name,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CostUSD:      cost,
		LatencyMS:    latencyMS,
		Status:       resp.StatusCode,
	})

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

// record persists one audit entry; failures are logged, not fatal to the
// response already produced by the provider.
func (s *Server) record(r *http.Request, u store.Usage) {
	if err := s.store.RecordUsage(r.Context(), u); err != nil {
		log.Printf("record usage for key %q: %v", u.KeyName, err)
	}
}

// parseUsage extracts token counts from a provider response's usage object,
// preferring prompt_tokens/completion_tokens with a fallback to
// input_tokens/output_tokens.
func parseUsage(body []byte) (inputTokens, outputTokens int64) {
	var parsed struct {
		Usage struct {
			PromptTokens     *int64 `json:"prompt_tokens"`
			CompletionTokens *int64 `json:"completion_tokens"`
			InputTokens      *int64 `json:"input_tokens"`
			OutputTokens     *int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, 0
	}
	u := parsed.Usage
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
