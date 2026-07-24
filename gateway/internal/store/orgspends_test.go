package store

import (
	"context"
	"testing"
)

// OrgSpends must agree with OrgSpend per org — the batch query exists to save
// round trips, not to report different numbers.
func TestOrgSpendsMatchesPerOrgSpend(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()

	orgA, err := m.CreateOrg(ctx, "a", 0)
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	orgB, err := m.CreateOrg(ctx, "b", 0)
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	for _, tc := range []struct {
		org  string
		name string
		cost float64
	}{
		{orgA.ID, "a1", 1.25},
		{orgA.ID, "a2", 0.75},
		{orgB.ID, "b1", 3.00},
	} {
		secret, err := m.CreateKeyIn(ctx, tc.name, 0, tc.org, RootCreator)
		if err != nil {
			t.Fatalf("create key: %v", err)
		}
		k, err := m.Authenticate(ctx, secret)
		if err != nil {
			t.Fatalf("authenticate: %v", err)
		}
		if err := m.RecordUsage(ctx, Usage{
			KeyName: tc.name, SecretHash: k.SecretHash, OrgID: tc.org, CostUSD: tc.cost,
		}); err != nil {
			t.Fatalf("record usage: %v", err)
		}
	}

	batch, err := m.OrgSpends(ctx)
	if err != nil {
		t.Fatalf("org spends: %v", err)
	}
	for _, id := range []string{orgA.ID, orgB.ID} {
		want, err := m.OrgSpend(ctx, id)
		if err != nil {
			t.Fatalf("org spend: %v", err)
		}
		if got := batch[id]; got != want {
			t.Fatalf("org %s: batch %v, per-org %v", id, got, want)
		}
	}
	if batch[orgA.ID] != 2.0 {
		t.Fatalf("org a total = %v, want 2.0", batch[orgA.ID])
	}
}

// An org with no keys is simply absent, and reads as zero.
func TestOrgSpendsOmitsOrgsWithoutKeys(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	org, err := m.CreateOrg(ctx, "empty", 0)
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	spends, err := m.OrgSpends(ctx)
	if err != nil {
		t.Fatalf("org spends: %v", err)
	}
	if v, present := spends[org.ID]; present || v != 0 {
		t.Fatalf("empty org present=%v value=%v; want absent and zero-valued", present, v)
	}
}
