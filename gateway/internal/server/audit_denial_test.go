package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/ifahad/agentos/gateway/internal/rbac"
	"github.com/ifahad/agentos/gateway/internal/store"
)

// auditKinds returns every audit kind recorded so far, in order.
func auditKinds(t *testing.T, mem *store.Memory) []string {
	t.Helper()
	entries := mem.Audit()
	kinds := make([]string, 0, len(entries))
	for _, e := range entries {
		kinds = append(kinds, e.Kind)
	}
	return kinds
}

func hasKind(kinds []string, want string) bool {
	for _, k := range kinds {
		if k == want {
			return true
		}
	}
	return false
}

// A platform whose contract is "nothing runs unauthorized OR unrecorded" had
// exactly one outcome it never wrote down: the refusals. Those are the events
// an auditor asks to see.
func TestRBACDenialIsAudited(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	ctx := context.Background()
	org, err := mem.CreateOrg(ctx, "acme", 0)
	if err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}
	viewer, token, err := mem.CreateUser(ctx, org.ID, "viewer@acme.test", rbac.RoleViewer)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	resp, _ := doRawBytes(t, http.MethodPost, srv.URL+"/admin/orgs/"+org.ID+"/users", token,
		`{"email":"new@acme.test","role":"member"}`)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("a viewer creating a user = %d, want 403", resp.StatusCode)
	}

	kinds := auditKinds(t, mem)
	if !hasKind(kinds, store.KindDenied) {
		t.Fatalf("the denial left no audit row; kinds recorded: %v", kinds)
	}
	// The row must say WHO was refused, or it cannot answer the question it
	// exists to answer.
	var found bool
	for _, e := range mem.Audit() {
		if e.Kind == store.KindDenied {
			found = true
			if e.Status != http.StatusForbidden {
				t.Errorf("denial row status = %d, want 403", e.Status)
			}
			if e.KeyName != viewer.Email {
				t.Errorf("denial row actor = %q, want %q", e.KeyName, viewer.Email)
			}
		}
	}
	if !found {
		t.Error("no denial row to inspect")
	}
}

func TestCrossOrgDenialIsAudited(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	ctx := context.Background()
	a, _ := mem.CreateOrg(ctx, "a", 0)
	b, _ := mem.CreateOrg(ctx, "b", 0)
	_, token, err := mem.CreateUser(ctx, a.ID, "owner@a.test", rbac.RoleOwner)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	// An owner of one org reaching into another is the denial that matters most
	// on a multi-tenant platform, and it was unrecorded too.
	resp, _ := doRawBytes(t, http.MethodGet, srv.URL+"/admin/orgs/"+b.ID+"/users", token, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-org read = %d, want 403", resp.StatusCode)
	}
	if kinds := auditKinds(t, mem); !hasKind(kinds, store.KindDenied) {
		t.Errorf("cross-org denial left no audit row; kinds: %v", kinds)
	}
}

// An allowed request must NOT be recorded as a denial, or the kind means
// nothing.
func TestAllowedActionRecordsNoDenial(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	ctx := context.Background()
	org, _ := mem.CreateOrg(ctx, "acme", 0)
	_, token, err := mem.CreateUser(ctx, org.ID, "owner@acme.test", rbac.RoleOwner)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	resp, _ := doRawBytes(t, http.MethodGet, srv.URL+"/admin/orgs/"+org.ID+"/users", token, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("an owner listing its own org = %d, want 200", resp.StatusCode)
	}
	if kinds := auditKinds(t, mem); hasKind(kinds, store.KindDenied) {
		t.Errorf("an allowed action was recorded as a denial: %v", kinds)
	}
}

// The budget is this platform's central control, so its refusal is the denial
// most worth being able to point at — and it was the one that wrote nothing.
func TestBudgetExhaustionIsAudited(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	// A budget below the per-request admission reserve is refused on the first
	// call, which is the behaviour that stops a burst slipping past a limit.
	secret, err := mem.CreateKey(context.Background(), "broke", 0.0001)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	resp, _ := doRawBytes(t, http.MethodPost, srv.URL+"/v1/chat/completions", secret,
		`{"model":"anthropic/claude-sonnet-5","messages":[{"role":"user","content":"hi"}]}`)
	if resp.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402", resp.StatusCode)
	}
	var found bool
	for _, e := range mem.Audit() {
		if e.Kind == store.KindBudgetExceeded {
			found = true
			if e.KeyName != "broke" || e.Status != http.StatusPaymentRequired {
				t.Errorf("row = key %q status %d, want \"broke\" / 402", e.KeyName, e.Status)
			}
		}
	}
	if !found {
		t.Errorf("a request was refused for budget and nothing was recorded; kinds: %v", auditKinds(t, mem))
	}
}

func TestFundedKeyWritesNoBudgetExceededRow(t *testing.T) {
	_, mem, srv := newTestGateway(t)
	secret, err := mem.CreateKey(context.Background(), "funded", 25)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	resp, _ := doRawBytes(t, http.MethodPost, srv.URL+"/v1/chat/completions", secret,
		`{"model":"anthropic/claude-sonnet-5","messages":[{"role":"user","content":"hi"}]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if kinds := auditKinds(t, mem); hasKind(kinds, store.KindBudgetExceeded) {
		t.Errorf("a funded key recorded a budget refusal: %v", kinds)
	}
}
