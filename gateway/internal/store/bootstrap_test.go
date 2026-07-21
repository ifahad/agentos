package store

import (
	"context"
	"reflect"
	"testing"
)

func TestParseBootstrapKeys(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []BootstrapKey
		wantErr bool
	}{
		{
			name: "valid entry",
			raw:  "runtime:agos-local-dev-runtime:25",
			want: []BootstrapKey{{Name: "runtime", Secret: "agos-local-dev-runtime", BudgetUSD: 25}},
		},
		{
			name: "multiple entries with spaces",
			raw:  "runtime:agos-a:25, demo:agos-b:1.5",
			want: []BootstrapKey{
				{Name: "runtime", Secret: "agos-a", BudgetUSD: 25},
				{Name: "demo", Secret: "agos-b", BudgetUSD: 1.5},
			},
		},
		{name: "empty var is a no-op", raw: "", want: nil},
		{name: "whitespace only is a no-op", raw: "   ", want: nil},
		{name: "bad budget", raw: "runtime:agos-a:lots", wantErr: true},
		{name: "missing field", raw: "runtime:agos-a", wantErr: true},
		{name: "too many fields", raw: "runtime:agos:a:25", wantErr: true},
		{name: "empty name", raw: ":agos-a:25", wantErr: true},
		{name: "empty secret", raw: "runtime::25", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBootstrapKeys(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestApplyBootstrapKeysIdempotent(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	keys := []BootstrapKey{{Name: "runtime", Secret: "agos-local-dev-runtime", BudgetUSD: 25}}

	if err := ApplyBootstrapKeys(ctx, m, keys); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if err := m.RecordUsage(ctx, Usage{KeyName: "runtime", CostUSD: 0.5, Status: 200}); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}
	// Simulate a restart.
	if err := ApplyBootstrapKeys(ctx, m, keys); err != nil {
		t.Fatalf("second apply: %v", err)
	}

	key, err := m.Authenticate(ctx, "agos-local-dev-runtime")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if !almostEqual(key.SpendUSD, 0.5) {
		t.Errorf("spend = %v, want preserved 0.5", key.SpendUSD)
	}
	listed, err := m.Keys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Errorf("keys = %d, want 1", len(listed))
	}
}
