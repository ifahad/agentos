package store

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryCreateAndAuthenticate(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()

	secret, err := m.CreateKey(ctx, "runtime", 25)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	if len(secret) < 10 || secret[:5] != "agos-" {
		t.Fatalf("secret %q does not have agos- prefix", secret)
	}

	tests := []struct {
		name    string
		secret  string
		wantErr error
		wantKey string
	}{
		{name: "valid secret", secret: secret, wantKey: "runtime"},
		{name: "unknown secret", secret: "agos-nope", wantErr: ErrInvalidKey},
		{name: "empty secret", secret: "", wantErr: ErrInvalidKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := m.Authenticate(ctx, tt.secret)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Authenticate err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil {
				if key.Name != tt.wantKey {
					t.Errorf("key name = %q, want %q", key.Name, tt.wantKey)
				}
				if key.MonthlyBudgetUSD != 25 {
					t.Errorf("budget = %v, want 25", key.MonthlyBudgetUSD)
				}
			}
		})
	}
}

func TestMemoryRecordUsageUpdatesSpendAndAggregates(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	secret, err := m.CreateKey(ctx, "agent", 10)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}

	entries := []Usage{
		{KeyName: "agent", Model: "anthropic/claude-sonnet-5", InputTokens: 1000, OutputTokens: 500, CostUSD: 0.0105, LatencyMS: 120, Status: 200},
		{KeyName: "agent", Model: "anthropic/claude-sonnet-5", InputTokens: 2000, OutputTokens: 100, CostUSD: 0.0075, LatencyMS: 80, Status: 200},
	}
	for _, u := range entries {
		if err := m.RecordUsage(ctx, u); err != nil {
			t.Fatalf("RecordUsage: %v", err)
		}
	}

	key, err := m.Authenticate(ctx, secret)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if got, want := key.SpendUSD, 0.018; !almostEqual(got, want) {
		t.Errorf("spend = %v, want %v", got, want)
	}

	usage, err := m.Usage(ctx)
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	if len(usage) != 1 {
		t.Fatalf("usage rows = %d, want 1", len(usage))
	}
	got := usage[0]
	if got.Name != "agent" || got.Requests != 2 || got.InputTokens != 3000 || got.OutputTokens != 600 {
		t.Errorf("aggregate = %+v", got)
	}
	if !almostEqual(got.SpendUSD, 0.018) {
		t.Errorf("aggregate spend = %v, want 0.018", got.SpendUSD)
	}

	if audit := m.Audit(); len(audit) != 2 {
		t.Errorf("audit entries = %d, want 2", len(audit))
	}
}

func TestMemoryKeysListingHasNoSecrets(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	if _, err := m.CreateKey(ctx, "b-key", 5); err != nil {
		t.Fatal(err)
	}
	if _, err := m.CreateKey(ctx, "a-key", 1); err != nil {
		t.Fatal(err)
	}
	keys, err := m.Keys(ctx)
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("keys = %d, want 2", len(keys))
	}
	if keys[0].Name != "a-key" || keys[1].Name != "b-key" {
		t.Errorf("keys not sorted by name: %+v", keys)
	}
}

func TestMemoryEnsureKeyIdempotent(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()

	if err := m.EnsureKey(ctx, "runtime", "agos-local-dev-runtime", 25); err != nil {
		t.Fatalf("EnsureKey: %v", err)
	}
	// Accumulate spend, then re-ensure with a changed budget.
	if err := m.RecordUsage(ctx, Usage{KeyName: "runtime", CostUSD: 1.5, Status: 200}); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}
	if err := m.EnsureKey(ctx, "runtime", "agos-local-dev-runtime", 50); err != nil {
		t.Fatalf("EnsureKey second call: %v", err)
	}

	key, err := m.Authenticate(ctx, "agos-local-dev-runtime")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if key.MonthlyBudgetUSD != 50 {
		t.Errorf("budget = %v, want updated 50", key.MonthlyBudgetUSD)
	}
	if !almostEqual(key.SpendUSD, 1.5) {
		t.Errorf("spend = %v, want preserved 1.5", key.SpendUSD)
	}

	keys, err := m.Keys(ctx)
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("keys = %d, want 1 (no duplicate)", len(keys))
	}
}

func TestMemoryAuditListNewestFirstWithKinds(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()

	// Kind defaults to chat for legacy entries; RecordAudit skips aggregates.
	entries := []struct {
		auditOnly bool
		u         Usage
	}{
		{false, Usage{KeyName: "agent", Model: "m1", InputTokens: 10, Status: 200}},
		{true, Usage{KeyName: "agent", Model: "m2", Status: 200, Kind: KindGuardrailFlag}},
		{false, Usage{KeyName: "agent", Model: "m3", Status: 200, Kind: KindEmbeddings}},
	}
	for _, e := range entries {
		record := m.RecordUsage
		if e.auditOnly {
			record = m.RecordAudit
		}
		if err := record(ctx, e.u); err != nil {
			t.Fatal(err)
		}
	}

	list, err := m.AuditList(ctx, 10)
	if err != nil {
		t.Fatalf("AuditList: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("entries = %d, want 3", len(list))
	}
	// Newest first.
	if list[0].Model != "m3" || list[1].Model != "m2" || list[2].Model != "m1" {
		t.Errorf("order = %q, %q, %q, want m3, m2, m1", list[0].Model, list[1].Model, list[2].Model)
	}
	if list[0].Kind != KindEmbeddings || list[1].Kind != KindGuardrailFlag || list[2].Kind != KindChat {
		t.Errorf("kinds = %q, %q, %q", list[0].Kind, list[1].Kind, list[2].Kind)
	}
	if list[2].TS.IsZero() {
		t.Error("ts not stamped")
	}

	// Limit honored.
	if short, _ := m.AuditList(ctx, 2); len(short) != 2 || short[0].Model != "m3" {
		t.Errorf("limited list = %+v", short)
	}

	// RecordAudit did not touch aggregates: only the two RecordUsage calls count.
	usage, err := m.Usage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(usage) != 1 || usage[0].Requests != 2 {
		t.Errorf("usage = %+v, want 2 requests", usage)
	}
}

func almostEqual(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-9
}
