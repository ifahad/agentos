package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/ifahad/agentos/gateway/internal/guardrail"
	"github.com/ifahad/agentos/gateway/internal/store"
)

// stubGuard returns a fixed verdict, standing in for a ModelScreen whose
// classifier already decided.
type stubGuard struct{ v guardrail.Verdict }

func (s stubGuard) Screen(string) guardrail.Verdict { return s.v }

func TestGuardrailsModelModeBlocksFlaggedPrompt(t *testing.T) {
	fake, mem, srv := newTestGateway(t, WithGuardrails(guardrail.ModeModel,
		stubGuard{guardrail.Verdict{Flagged: true, Reason: "model says injection"}}))
	secret := createKey(t, mem, "agent", 100)

	resp, body := doJSON(t, http.MethodPost, srv.URL+"/v1/chat/completions", secret, cleanBody)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %v", resp.StatusCode, body)
	}
	if got := errorType(t, body); got != "guardrail_blocked" {
		t.Errorf("error type = %q, want guardrail_blocked", got)
	}
	if fake.lastBody != nil {
		t.Error("blocked request must not be forwarded to the provider")
	}
	audit := mem.Audit()
	if len(audit) != 1 || audit[0].Kind != store.KindGuardrailBlock || audit[0].Status != http.StatusBadRequest {
		t.Errorf("audit = %+v, want one guardrail_block entry with status 400", audit)
	}
}

func TestGuardrailsModelModeCleanVerdictForwards(t *testing.T) {
	fake, mem, srv := newTestGateway(t, WithGuardrails(guardrail.ModeModel, stubGuard{}))
	secret := createKey(t, mem, "agent", 100)

	resp, body := doJSON(t, http.MethodPost, srv.URL+"/v1/chat/completions", secret, cleanBody)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %v", resp.StatusCode, body)
	}
	if fake.lastBody == nil {
		t.Fatal("clean request was not forwarded to the provider")
	}
	audit := mem.Audit()
	if len(audit) != 1 || audit[0].Kind != store.KindChat {
		t.Errorf("audit = %+v, want single chat entry", audit)
	}
}

func TestGuardrailsModelModeClassifierErrorFailsOpen(t *testing.T) {
	fake, mem, srv := newTestGateway(t, WithGuardrails(guardrail.ModeModel,
		stubGuard{guardrail.Verdict{Errored: true, Reason: "classifier: provider unreachable"}}))
	secret := createKey(t, mem, "agent", 100)

	resp, body := doJSON(t, http.MethodPost, srv.URL+"/v1/chat/completions", secret, cleanBody)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (fail open): %v", resp.StatusCode, body)
	}
	if fake.lastBody == nil {
		t.Fatal("fail-open request was not forwarded to the provider")
	}

	audit := mem.Audit()
	if len(audit) != 2 {
		t.Fatalf("audit entries = %d, want 2 (guardrail_error + chat)", len(audit))
	}
	if audit[0].Kind != store.KindGuardrailError || audit[0].Status != http.StatusOK ||
		audit[0].KeyName != "agent" || audit[0].Model != "openai/gpt-4o-mini" {
		t.Errorf("error entry = %+v, want guardrail_error with status 200", audit[0])
	}
	if audit[1].Kind != store.KindChat {
		t.Errorf("chat entry = %+v", audit[1])
	}

	// The guardrail_error entry is audit-only: it must not count as a request.
	usage, err := mem.Usage(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(usage) != 1 || usage[0].Requests != 1 {
		t.Errorf("usage = %+v, want exactly 1 accounted request", usage)
	}
}

// TestGuardrailsModelModeEndToEndWithModelScreen exercises the real
// ModelScreen inside the server: the heuristic short-circuit blocks without
// consulting the classifier.
func TestGuardrailsModelModeEndToEndWithModelScreen(t *testing.T) {
	fake, mem, srv := newTestGateway(t, WithGuardrails(guardrail.ModeModel,
		guardrail.NewModelScreen(guardrail.NewHeuristicScreen(), countingClassifier{t}, "anthropic/claude-haiku-4-5")))
	secret := createKey(t, mem, "agent", 100)

	resp, body := doJSON(t, http.MethodPost, srv.URL+"/v1/chat/completions", secret, injectionBody)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %v", resp.StatusCode, body)
	}
	if got := errorType(t, body); got != "guardrail_blocked" {
		t.Errorf("error type = %q, want guardrail_blocked", got)
	}
	if fake.lastBody != nil {
		t.Error("blocked request must not be forwarded")
	}
	if audit := mem.Audit(); len(audit) != 1 || audit[0].Kind != store.KindGuardrailBlock {
		t.Errorf("audit = %+v, want one guardrail_block entry", audit)
	}
}

// countingClassifier fails the test if the model is ever consulted.
type countingClassifier struct{ t *testing.T }

func (c countingClassifier) Classify(_ context.Context, _, _ string) (bool, string, error) {
	c.t.Error("classifier must not be called for heuristic-obvious injections")
	return false, "", nil
}
