package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ifahad/agentos/gateway/internal/provider"
	"github.com/ifahad/agentos/gateway/internal/store"
)

// newCouncilGateway builds a gateway with a budgeted virtual key and, when
// runtimeURL is non-empty, the council model enabled. Returns the key secret and
// the handler.
func newCouncilGateway(t *testing.T, runtimeURL, runtimeToken string) (string, http.Handler) {
	t.Helper()
	mem := store.NewMemory()
	if err := mem.EnsureOrg(context.Background(), store.DefaultOrgID, "default", 0); err != nil {
		t.Fatalf("ensure org: %v", err)
	}
	secret := createKey(t, mem, "council-key", 100)
	opts := []Option{}
	if runtimeURL != "" {
		opts = append(opts, WithCouncil(runtimeURL, runtimeToken))
	}
	srv := New(mem, &provider.Router{}, testAdminKey, opts...)
	return secret, srv.Handler()
}

func TestCouncilModelRejectsRecursion(t *testing.T) {
	// A request carrying the council depth marker must never be allowed to ask
	// for a council model: that is the council calling itself.
	secret, h := newCouncilGateway(t, "http://runtime.invalid", "tok")
	body := `{"model":"council/multiverse","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set(CouncilDepthHeader, "1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "recursion") {
		t.Errorf("error must explain the recursion guard, got %s", rec.Body.String())
	}
}

func TestCouncilModelReturnsASynthesizedCompletion(t *testing.T) {
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer runtime-tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get(CouncilDepthHeader) == "" {
			t.Error("gateway must stamp the council depth marker on the runtime call")
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"objective":{"id":"obj-1"},"verdict":{"answer":"synthesized",
		  "agreement":0.8,"dissent":[{"member":"gamma","claim":"other","basis":"b"}],
		  "cited_members":["alpha","beta"]},"spend_usd":0.02}`)
	}))
	defer runtime.Close()

	secret, h := newCouncilGateway(t, runtime.URL, "runtime-tok")
	body := `{"model":"council/multiverse","messages":[{"role":"user","content":"who owes what?"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Object  string `json:"object"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Council struct {
			Agreement float64 `json:"agreement"`
			Dissent   []struct {
				Member string `json:"member"`
			} `json:"dissent"`
			CitedMembers []string `json:"cited_members"`
		} `json:"x_agentos_council"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Object != "chat.completion" {
		t.Errorf("object = %q, want chat.completion", out.Object)
	}
	if out.Model != "council/multiverse" {
		t.Errorf("model = %q", out.Model)
	}
	if len(out.Choices) != 1 || out.Choices[0].Message.Content != "synthesized" {
		t.Fatalf("choices = %+v", out.Choices)
	}
	if out.Choices[0].Message.Role != "assistant" {
		t.Errorf("role = %q, want assistant", out.Choices[0].Message.Role)
	}
	if out.Council.Agreement != 0.8 || len(out.Council.Dissent) != 1 {
		t.Errorf("council extension = %+v", out.Council)
	}
}

func TestCouncilModelUnconfiguredIsUnsupported(t *testing.T) {
	secret, h := newCouncilGateway(t, "", "") // no WithCouncil
	body := `{"model":"council/multiverse","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
