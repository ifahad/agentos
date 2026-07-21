// Package provider maps model-prefix routes to upstream LLM providers and
// prices token usage.
package provider

import (
	"errors"
	"strings"
)

// ErrUnknownProvider is returned when the model carries no known prefix.
var ErrUnknownProvider = errors.New("unknown model provider prefix")

// Default upstream base URLs, overridable for tests and local setups.
const (
	DefaultAnthropicBaseURL = "https://api.anthropic.com"
	DefaultOpenAIBaseURL    = "https://api.openai.com"
	DefaultOllamaBaseURL    = "http://localhost:11434"
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
	AnthropicAPIKey  string
	OpenAIAPIKey     string
}

// Route resolves a prefixed model like "anthropic/claude-sonnet-5" to an
// upstream URL, credential, and stripped model name.
func (r *Router) Route(model string) (*Route, error) {
	switch {
	case strings.HasPrefix(model, "anthropic/"):
		return &Route{
			Provider: "anthropic",
			URL:      orDefault(r.AnthropicBaseURL, DefaultAnthropicBaseURL) + "/v1/chat/completions",
			APIKey:   r.AnthropicAPIKey,
			Model:    strings.TrimPrefix(model, "anthropic/"),
		}, nil
	case strings.HasPrefix(model, "openai/"):
		return &Route{
			Provider: "openai",
			URL:      orDefault(r.OpenAIBaseURL, DefaultOpenAIBaseURL) + "/v1/chat/completions",
			APIKey:   r.OpenAIAPIKey,
			Model:    strings.TrimPrefix(model, "openai/"),
		}, nil
	case strings.HasPrefix(model, "ollama/"):
		return &Route{
			Provider: "ollama",
			URL:      orDefault(r.OllamaBaseURL, DefaultOllamaBaseURL) + "/v1/chat/completions",
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
