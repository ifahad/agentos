package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ifahad/agentos/gateway/internal/provider"
	"github.com/ifahad/agentos/gateway/internal/store"
)

// reserveFailsStore is a Memory whose budget reservation always errors, which
// is the only way to reach the fail-open branch: it fires on a store fault, not
// on any input a request can carry.
type reserveFailsStore struct {
	*store.Memory
}

func (s reserveFailsStore) ReserveSpend(context.Context, string, float64) (string, error) {
	return "", errors.New("simulated store fault")
}

// The gateway admits the request when it cannot evaluate a budget — a store
// blip must not take traffic down. The guardrail screener has taken that stance
// since Phase 4 AND recorded it as guardrail_error, precisely so the blind spot
// is on the record. The budget path took the fail-open half without the audited
// half, so the one moment a budget was not enforced was the one moment nothing
// was written down.
func TestBudgetFailOpenIsAudited(t *testing.T) {
	fake := newFakeProvider(t)
	mem := store.NewMemory()
	st := reserveFailsStore{Memory: mem}
	router := &provider.Router{
		AnthropicBaseURL: fake.server.URL, OpenAIBaseURL: fake.server.URL,
		OllamaBaseURL: fake.server.URL, AnthropicAPIKey: "k", OpenAIAPIKey: "k",
	}
	srv := httptest.NewServer(New(st, router, testAdminKey).Handler())
	t.Cleanup(srv.Close)

	ctx := context.Background()
	secret, err := mem.CreateKey(ctx, "budgeted", 25)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}

	resp, body := doRawBytes(t, http.MethodPost, srv.URL+"/v1/chat/completions", secret,
		`{"model":"anthropic/claude-sonnet-5","messages":[{"role":"user","content":"hi"}]}`)
	// Fail OPEN: the request must still succeed.
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 — the reservation fault must not take traffic down (%s)",
			resp.StatusCode, body)
	}

	var found bool
	for _, e := range mem.Audit() {
		if e.Kind == store.KindBudgetError {
			found = true
			if e.KeyName != "budgeted" {
				t.Errorf("fail-open row names %q, want the key it could not evaluate", e.KeyName)
			}
		}
	}
	if !found {
		kinds := auditKinds(t, mem)
		t.Errorf("the budget could not be enforced and nothing was recorded; kinds: %v", kinds)
	}
}

// A healthy reservation must not write the fail-open kind, or it says nothing.
func TestHealthyBudgetWritesNoFailOpenRow(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	secret, err := mem.CreateKey(context.Background(), "budgeted", 25)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	resp, _ := doRawBytes(t, http.MethodPost, srv.URL+"/v1/chat/completions", secret,
		`{"model":"anthropic/claude-sonnet-5","messages":[{"role":"user","content":"hi"}]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if kinds := auditKinds(t, mem); hasKind(kinds, store.KindBudgetError) {
		t.Errorf("a healthy reservation recorded a fail-open: %v", kinds)
	}
}
