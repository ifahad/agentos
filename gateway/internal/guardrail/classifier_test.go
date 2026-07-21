package guardrail

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ifahad/agentos/gateway/internal/provider"
)

func TestParseVerdict(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		wantInjection bool
		wantReason    string
		wantErr       bool
	}{
		{
			name:          "strict JSON true",
			content:       `{"injection": true, "reason": "instruction override"}`,
			wantInjection: true,
			wantReason:    "instruction override",
		},
		{
			name:       "strict JSON false",
			content:    `{"injection": false, "reason": "benign question"}`,
			wantReason: "benign question",
		},
		{
			name:          "fenced JSON",
			content:       "```json\n{\"injection\": true, \"reason\": \"exfiltration\"}\n```",
			wantInjection: true,
			wantReason:    "exfiltration",
		},
		{
			name:       "JSON wrapped in prose",
			content:    "Sure! Here is my verdict: {\"injection\": false, \"reason\": \"clean\"} — hope that helps.",
			wantReason: "clean",
		},
		{name: "no JSON at all", content: "I cannot make that determination.", wantErr: true},
		{name: "empty content", content: "", wantErr: true},
		{name: "malformed JSON", content: `{"injection": tru`, wantErr: true},
		{name: "missing injection field", content: `{"reason": "no verdict"}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			injection, reason, err := parseVerdict(tt.content)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if injection != tt.wantInjection || reason != tt.wantReason {
				t.Errorf("verdict = %v %q, want %v %q", injection, reason, tt.wantInjection, tt.wantReason)
			}
		})
	}
}

// classifierReply wraps content in an OpenAI-style chat completion body.
func classifierReply(content string) string {
	raw, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": content}}},
		"usage":   map[string]any{"prompt_tokens": 42, "completion_tokens": 7},
	})
	return string(raw)
}

func TestProviderClassifierRoutesAndParses(t *testing.T) {
	var lastPath, lastAuth string
	var lastBody map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		lastAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		lastBody = nil
		_ = json.Unmarshal(raw, &lastBody)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, classifierReply(`{"injection": true, "reason": "prompt injection"}`))
	}))
	t.Cleanup(upstream.Close)

	var usageModel string
	var usageIn, usageOut int64
	c := &ProviderClassifier{
		Router: &provider.Router{AnthropicBaseURL: upstream.URL, AnthropicAPIKey: "anthropic-key"},
		OnUsage: func(model string, in, out int64) {
			usageModel, usageIn, usageOut = model, in, out
		},
	}
	injection, reason, err := c.Classify(t.Context(), "anthropic/claude-haiku-4-5", "sneaky message")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if !injection || reason != "prompt injection" {
		t.Errorf("verdict = %v %q", injection, reason)
	}

	// Routed through the provider layer with the prefix stripped.
	if lastPath != "/v1/chat/completions" {
		t.Errorf("path = %q, want /v1/chat/completions", lastPath)
	}
	if lastAuth != "Bearer anthropic-key" {
		t.Errorf("auth = %q, want provider key bearer", lastAuth)
	}
	if lastBody["model"] != "claude-haiku-4-5" {
		t.Errorf("model = %v, want stripped claude-haiku-4-5", lastBody["model"])
	}

	// System prompt demands strict JSON; the user message is passed verbatim.
	msgs, _ := lastBody["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("messages = %v, want system+user", lastBody["messages"])
	}
	sys := msgs[0].(map[string]any)
	if sys["role"] != "system" || !strings.Contains(sys["content"].(string), `{"injection": true|false`) {
		t.Errorf("system message = %v", sys)
	}
	usr := msgs[1].(map[string]any)
	if usr["role"] != "user" || usr["content"] != "sneaky message" {
		t.Errorf("user message = %v", usr)
	}

	// Token usage surfaced for spend attribution.
	if usageModel != "claude-haiku-4-5" || usageIn != 42 || usageOut != 7 {
		t.Errorf("OnUsage(%q, %d, %d), want (claude-haiku-4-5, 42, 7)", usageModel, usageIn, usageOut)
	}
}

func TestProviderClassifierErrors(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantPart string
	}{
		{name: "provider 500", status: http.StatusInternalServerError, body: `{"error":"boom"}`, wantPart: "status 500"},
		{name: "no choices", status: http.StatusOK, body: `{"choices":[]}`, wantPart: "no choices"},
		{name: "not JSON", status: http.StatusOK, body: `<html>gateway timeout</html>`, wantPart: "decode"},
		{name: "unparseable verdict", status: http.StatusOK, body: classifierReply("I refuse to answer in JSON."), wantPart: "no JSON object"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				io.WriteString(w, tt.body)
			}))
			t.Cleanup(upstream.Close)

			c := &ProviderClassifier{Router: &provider.Router{OpenAIBaseURL: upstream.URL, OpenAIAPIKey: "k"}}
			if _, _, err := c.Classify(t.Context(), "openai/gpt-4o-mini", "msg"); err == nil {
				t.Fatal("Classify err = nil, want error")
			} else if !strings.Contains(err.Error(), tt.wantPart) {
				t.Errorf("err = %v, want containing %q", err, tt.wantPart)
			}
		})
	}
}

func TestProviderClassifierUnroutableModel(t *testing.T) {
	c := &ProviderClassifier{Router: &provider.Router{}}
	if _, _, err := c.Classify(context.Background(), "mistral/large", "msg"); err == nil {
		t.Fatal("Classify err = nil, want unknown-provider error")
	}
}

func TestProviderClassifierUnreachableProvider(t *testing.T) {
	// A closed server: the classifier must surface a transport error so
	// ModelScreen can fail open.
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	upstream.Close()

	c := &ProviderClassifier{Router: &provider.Router{OllamaBaseURL: upstream.URL}}
	if _, _, err := c.Classify(context.Background(), "ollama/llama3.1", "msg"); err == nil {
		t.Fatal("Classify err = nil, want unreachable error")
	}
}
