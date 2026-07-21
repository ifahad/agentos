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
			got := Cost(tt.model, tt.input, tt.output)
			diff := got - tt.want
			if diff < 0 {
				diff = -diff
			}
			if diff > 1e-9 {
				t.Errorf("Cost(%q, %d, %d) = %v, want %v", tt.model, tt.input, tt.output, got, tt.want)
			}
		})
	}
}
