package provider

import (
	"errors"
	"testing"
)

func TestRoute(t *testing.T) {
	router := &Router{
		AnthropicAPIKey: "sk-ant-test",
		OpenAIAPIKey:    "sk-oai-test",
		OllamaBaseURL:   "http://ollama:11434",
	}

	tests := []struct {
		name         string
		model        string
		wantProvider string
		wantURL      string
		wantAPIKey   string
		wantModel    string
		wantErr      error
	}{
		{
			name:         "anthropic prefix",
			model:        "anthropic/claude-sonnet-5",
			wantProvider: "anthropic",
			wantURL:      "https://api.anthropic.com/v1/chat/completions",
			wantAPIKey:   "sk-ant-test",
			wantModel:    "claude-sonnet-5",
		},
		{
			name:         "openai prefix",
			model:        "openai/gpt-4o-mini",
			wantProvider: "openai",
			wantURL:      "https://api.openai.com/v1/chat/completions",
			wantAPIKey:   "sk-oai-test",
			wantModel:    "gpt-4o-mini",
		},
		{
			name:         "ollama prefix uses configured base URL and no key",
			model:        "ollama/llama3.1",
			wantProvider: "ollama",
			wantURL:      "http://ollama:11434/v1/chat/completions",
			wantAPIKey:   "",
			wantModel:    "llama3.1",
		},
		{name: "unknown prefix", model: "mistral/mistral-7b", wantErr: ErrUnknownProvider},
		{name: "no prefix", model: "gpt-4o-mini", wantErr: ErrUnknownProvider},
		{name: "empty model", model: "", wantErr: ErrUnknownProvider},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route, err := router.Route(tt.model)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Route err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if route.Provider != tt.wantProvider {
				t.Errorf("provider = %q, want %q", route.Provider, tt.wantProvider)
			}
			if route.URL != tt.wantURL {
				t.Errorf("url = %q, want %q", route.URL, tt.wantURL)
			}
			if route.APIKey != tt.wantAPIKey {
				t.Errorf("api key = %q, want %q", route.APIKey, tt.wantAPIKey)
			}
			if route.Model != tt.wantModel {
				t.Errorf("model = %q, want %q", route.Model, tt.wantModel)
			}
		})
	}
}

func TestRouteEmbeddings(t *testing.T) {
	router := &Router{
		AnthropicAPIKey: "sk-ant-test",
		OpenAIAPIKey:    "sk-oai-test",
		OllamaBaseURL:   "http://ollama:11434",
	}

	tests := []struct {
		name         string
		model        string
		wantProvider string
		wantURL      string
		wantAPIKey   string
		wantModel    string
		wantErr      error
	}{
		{
			name:         "ollama embeddings",
			model:        "ollama/bge-m3",
			wantProvider: "ollama",
			wantURL:      "http://ollama:11434/v1/embeddings",
			wantAPIKey:   "",
			wantModel:    "bge-m3",
		},
		{
			name:         "openai embeddings",
			model:        "openai/text-embedding-3-small",
			wantProvider: "openai",
			wantURL:      "https://api.openai.com/v1/embeddings",
			wantAPIKey:   "sk-oai-test",
			wantModel:    "text-embedding-3-small",
		},
		{
			name:         "anthropic embeddings",
			model:        "anthropic/some-embed",
			wantProvider: "anthropic",
			wantURL:      "https://api.anthropic.com/v1/embeddings",
			wantAPIKey:   "sk-ant-test",
			wantModel:    "some-embed",
		},
		{name: "unknown prefix", model: "mistral/embed", wantErr: ErrUnknownProvider},
		{name: "no prefix", model: "bge-m3", wantErr: ErrUnknownProvider},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route, err := router.RouteEmbeddings(tt.model)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("RouteEmbeddings err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if route.Provider != tt.wantProvider || route.URL != tt.wantURL ||
				route.APIKey != tt.wantAPIKey || route.Model != tt.wantModel {
				t.Errorf("route = %+v", route)
			}
		})
	}
}

func TestRouteDefaultOllamaBaseURL(t *testing.T) {
	router := &Router{}
	route, err := router.Route("ollama/llama3.1")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if route.URL != "http://localhost:11434/v1/chat/completions" {
		t.Errorf("url = %q", route.URL)
	}
}

func TestCost(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		input, output int64
		want          float64
	}{
		{name: "claude-sonnet-5", model: "claude-sonnet-5", input: 1_000_000, output: 1_000_000, want: 18},
		{name: "claude-haiku-4-5", model: "claude-haiku-4-5", input: 2_000_000, output: 1_000_000, want: 7},
		{name: "gpt-4o-mini", model: "gpt-4o-mini", input: 1_000_000, output: 1_000_000, want: 0.75},
		{name: "gpt-4o-mini fractional", model: "gpt-4o-mini", input: 100_000, output: 50_000, want: 0.045},
		{name: "unknown local model is free", model: "llama3.1", input: 5_000_000, output: 5_000_000, want: 0},
		{name: "zero tokens", model: "claude-sonnet-5", input: 0, output: 0, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Cost is now a method on Route; a built-in model carries no per-route
			// price, so it falls back to the static table by Model name.
			route := &Route{Model: tt.model}
			got := route.Cost(tt.input, tt.output)
			diff := got - tt.want
			if diff < 0 {
				diff = -diff
			}
			if diff > 1e-9 {
				t.Errorf("Route{Model:%q}.Cost(%d, %d) = %v, want %v", tt.model, tt.input, tt.output, got, tt.want)
			}
		})
	}
}

// mutableSource is a settable secret.Source for the rotation test: Reload flips
// the returned value, standing in for a file/age/vault secret being rotated.
type mutableSource struct{ anthropic, openai string }

func (s *mutableSource) Get(name string) (string, bool) {
	switch name {
	case AnthropicKeyName:
		return s.anthropic, s.anthropic != ""
	case OpenAIKeyName:
		return s.openai, s.openai != ""
	}
	return "", false
}
func (s *mutableSource) Backend() string { return "mutable" }

// TestRouteResolvesKeyFromLiveSource proves H5: with a secret.Source injected,
// Route reads the provider key on each call, so a rotation changes the key used
// upstream. The static field is only a fallback when the source lacks a value.
func TestRouteResolvesKeyFromLiveSource(t *testing.T) {
	src := &mutableSource{anthropic: "sk-ant-v1", openai: "sk-oai-v1"}
	router := &Router{Secrets: src, AnthropicAPIKey: "static-fallback"}

	route, err := router.Route("anthropic/claude-sonnet-5")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if route.APIKey != "sk-ant-v1" {
		t.Fatalf("api key = %q, want live source value sk-ant-v1", route.APIKey)
	}

	// Rotate the secret; the very next Route call must reflect it (no restart).
	src.anthropic = "sk-ant-v2"
	route, _ = router.Route("anthropic/claude-sonnet-5")
	if route.APIKey != "sk-ant-v2" {
		t.Errorf("after rotation api key = %q, want sk-ant-v2", route.APIKey)
	}

	// OpenAI resolves independently from the same source.
	oroute, _ := router.Route("openai/gpt-4o-mini")
	if oroute.APIKey != "sk-oai-v1" {
		t.Errorf("openai key = %q, want sk-oai-v1", oroute.APIKey)
	}

	// When the source lacks a value, fall back to the static field.
	src.anthropic = ""
	route, _ = router.Route("anthropic/claude-sonnet-5")
	if route.APIKey != "static-fallback" {
		t.Errorf("empty source api key = %q, want static-fallback", route.APIKey)
	}
}
