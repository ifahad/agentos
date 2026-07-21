// Package provider maps model-prefix routes to upstream LLM providers and
// prices token usage.
package provider

import (
	"errors"
	"strings"

	"github.com/ifahad/agentos/gateway/internal/secret"
)

// ErrUnknownProvider is returned when the model carries no known prefix.
var ErrUnknownProvider = errors.New("unknown model provider prefix")

// Default upstream base URLs, overridable for tests and local setups.
const (
	DefaultAnthropicBaseURL = "https://api.anthropic.com"
	DefaultOpenAIBaseURL    = "https://api.openai.com"
	DefaultOllamaBaseURL    = "http://localhost:11434"
)

// Secret names the Router resolves provider keys under when a live
// secret.Source is injected (H5 rotation). They match server.DefaultSecretNames.
const (
	AnthropicKeyName = "AGENTOS_ANTHROPIC_API_KEY"
	OpenAIKeyName    = "AGENTOS_OPENAI_API_KEY"
)

// Route is a resolved upstream target for one request.
type Route struct {
	Provider string // "anthropic", "openai", "ollama"
	URL      string // full chat-completions URL
	APIKey   string // empty means no Authorization header
	Model    string // model with the provider prefix stripped
}

// Router resolves provider-prefixed model names. Zero-value base URLs fall
// back to the defaults above.
type Router struct {
	AnthropicBaseURL string
	OpenAIBaseURL    string
	OllamaBaseURL    string
	// AnthropicAPIKey/OpenAIAPIKey are the static fallback credentials used when
	// no live Secrets source is injected (tests, static config).
	AnthropicAPIKey string
	OpenAIAPIKey    string
	// Secrets, when non-nil, is the live secret source. Route resolves provider
	// keys from it on every call so a rotation (POST /admin/secrets/reload or the
	// refresh loop) takes effect upstream without a restart (H5). When it lacks a
	// value the static field above is used as a fallback.
	Secrets secret.Source
}

// anthropicKey resolves the Anthropic credential: the live Secrets source wins
// when it has a value, else the static AnthropicAPIKey field.
func (r *Router) anthropicKey() string {
	if r.Secrets != nil {
		if v, ok := r.Secrets.Get(AnthropicKeyName); ok && v != "" {
			return v
		}
	}
	return r.AnthropicAPIKey
}

// openaiKey resolves the OpenAI credential (see anthropicKey).
func (r *Router) openaiKey() string {
	if r.Secrets != nil {
		if v, ok := r.Secrets.Get(OpenAIKeyName); ok && v != "" {
			return v
		}
	}
	return r.OpenAIAPIKey
}

// Route resolves a prefixed model like "anthropic/claude-sonnet-5" to an
// upstream chat-completions URL, credential, and stripped model name.
func (r *Router) Route(model string) (*Route, error) {
	return r.route(model, "/v1/chat/completions")
}

// RouteEmbeddings resolves a prefixed model like "ollama/bge-m3" to an
// upstream embeddings URL with the same prefix rules as Route.
func (r *Router) RouteEmbeddings(model string) (*Route, error) {
	return r.route(model, "/v1/embeddings")
}

func (r *Router) route(model, path string) (*Route, error) {
	switch {
	case strings.HasPrefix(model, "anthropic/"):
		return &Route{
			Provider: "anthropic",
			URL:      orDefault(r.AnthropicBaseURL, DefaultAnthropicBaseURL) + path,
			APIKey:   r.anthropicKey(),
			Model:    strings.TrimPrefix(model, "anthropic/"),
		}, nil
	case strings.HasPrefix(model, "openai/"):
		return &Route{
			Provider: "openai",
			URL:      orDefault(r.OpenAIBaseURL, DefaultOpenAIBaseURL) + path,
			APIKey:   r.openaiKey(),
			Model:    strings.TrimPrefix(model, "openai/"),
		}, nil
	case strings.HasPrefix(model, "ollama/"):
		return &Route{
			Provider: "ollama",
			URL:      orDefault(r.OllamaBaseURL, DefaultOllamaBaseURL) + path,
			Model:    strings.TrimPrefix(model, "ollama/"),
		}, nil
	default:
		return nil, ErrUnknownProvider
	}
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return strings.TrimSuffix(v, "/")
}

// price is USD per one million tokens.
type price struct {
	In  float64
	Out float64
}

// prices is the static Phase 1 price table; unknown models are free (local).
var prices = map[string]price{
	"claude-sonnet-5":  {In: 3, Out: 15},
	"claude-haiku-4-5": {In: 1, Out: 5},
	"gpt-4o-mini":      {In: 0.15, Out: 0.60},
}

// Cost prices a request from the stripped model name and token counts.
// Models missing from the table cost 0 (local models are free).
func Cost(model string, inputTokens, outputTokens int64) float64 {
	p := prices[model]
	return (float64(inputTokens)*p.In + float64(outputTokens)*p.Out) / 1_000_000
}
