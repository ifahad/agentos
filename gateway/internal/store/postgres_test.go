package store

import (
	"context"
	"errors"
	"os"
	"testing"
)

// newTestPostgres connects to AGENTOS_TEST_DATABASE_URL, skipping the test
// when it is unset, and truncates the gateway tables for isolation.
func newTestPostgres(t *testing.T) *Postgres {
	t.Helper()
	dsn := os.Getenv("AGENTOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("AGENTOS_TEST_DATABASE_URL not set; skipping postgres store tests")
	}
	ctx := context.Background()
	p, err := NewPostgres(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPostgres: %v", err)
	}
	t.Cleanup(p.Close)
	if _, err := p.pool.Exec(ctx, `TRUNCATE keys, "usage", audit_log`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return p
}

func TestPostgresKeyLifecycle(t *testing.T) {
	p := newTestPostgres(t)
	ctx := context.Background()

	secret, err := p.CreateKey(ctx, "runtime", 25)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	key, err := p.Authenticate(ctx, secret)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if key.Name != "runtime" || key.MonthlyBudgetUSD != 25 || key.SpendUSD != 0 {
		t.Errorf("key = %+v", key)
	}
	if _, err := p.Authenticate(ctx, "agos-bogus"); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("Authenticate bogus err = %v, want ErrInvalidKey", err)
	}

	keys, err := p.Keys(ctx)
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	if len(keys) != 1 || keys[0].Name != "runtime" {
		t.Errorf("keys = %+v", keys)
	}
}

func TestPostgresRecordUsage(t *testing.T) {
	p := newTestPostgres(t)
	ctx := context.Background()

	secret, err := p.CreateKey(ctx, "agent", 10)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	entries := []Usage{
		{KeyName: "agent", Model: "openai/gpt-4o-mini", InputTokens: 1000, OutputTokens: 200, CostUSD: 0.00027, LatencyMS: 90, Status: 200},
		{KeyName: "agent", Model: "openai/gpt-4o-mini", InputTokens: 500, OutputTokens: 100, CostUSD: 0.000135, LatencyMS: 70, Status: 200},
	}
	for _, u := range entries {
		if err := p.RecordUsage(ctx, u); err != nil {
			t.Fatalf("RecordUsage: %v", err)
		}
	}

	key, err := p.Authenticate(ctx, secret)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if !almostEqual(key.SpendUSD, 0.000405) {
		t.Errorf("spend = %v, want 0.000405", key.SpendUSD)
	}

	usage, err := p.Usage(ctx)
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	if len(usage) != 1 {
		t.Fatalf("usage rows = %d, want 1", len(usage))
	}
	u := usage[0]
	if u.Name != "agent" || u.Requests != 2 || u.InputTokens != 1500 || u.OutputTokens != 300 {
		t.Errorf("aggregate = %+v", u)
	}

	var auditCount int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM audit_log`).Scan(&auditCount); err != nil {
		t.Fatalf("count audit_log: %v", err)
	}
	if auditCount != 2 {
		t.Errorf("audit_log rows = %d, want 2", auditCount)
	}
}

func TestPostgresEnsureKeyIdempotent(t *testing.T) {
	p := newTestPostgres(t)
	ctx := context.Background()

	if err := p.EnsureKey(ctx, "runtime", "agos-local-dev-runtime", 25); err != nil {
		t.Fatalf("EnsureKey: %v", err)
	}
	if err := p.RecordUsage(ctx, Usage{KeyName: "runtime", Model: "m", CostUSD: 2, Status: 200}); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}
	// Restart: same secret, updated budget — must not error or duplicate.
	if err := p.EnsureKey(ctx, "runtime", "agos-local-dev-runtime", 50); err != nil {
		t.Fatalf("EnsureKey second call: %v", err)
	}

	key, err := p.Authenticate(ctx, "agos-local-dev-runtime")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if key.MonthlyBudgetUSD != 50 {
		t.Errorf("budget = %v, want 50", key.MonthlyBudgetUSD)
	}
	if !almostEqual(key.SpendUSD, 2) {
		t.Errorf("spend = %v, want preserved 2", key.SpendUSD)
	}

	keys, err := p.Keys(ctx)
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("keys = %d, want 1 (no duplicate)", len(keys))
	}
}
