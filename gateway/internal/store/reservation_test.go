package store

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// budgetedKey creates a key with a budget and returns its secret hash.
func budgetedKey(t *testing.T, m *Memory, name string, budgetUSD float64) string {
	t.Helper()
	secret, err := m.CreateKey(context.Background(), name, budgetUSD)
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	k, err := m.Authenticate(context.Background(), secret)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	return k.SecretHash
}

// TestReserveSpendClosesTheRace is the regression test for the budget TOCTOU.
//
// Before reservations, every concurrent request read the same pre-spend
// snapshot, all passed the check, and all spent — so a council fanning out to N
// members could overrun an exhausted budget N times over. Admission must now be
// serial: with room for exactly 4 holds, exactly 4 of 50 racing callers win.
func TestReserveSpendClosesTheRace(t *testing.T) {
	m := NewMemory()
	hash := budgetedKey(t, m, "council", 0.20) // room for 4 x 0.05

	const racers = 50
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		admitted int
	)
	wg.Add(racers)
	for range racers {
		go func() {
			defer wg.Done()
			if _, err := m.ReserveSpend(context.Background(), hash, 0.05); err == nil {
				mu.Lock()
				admitted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if admitted != 4 {
		t.Fatalf("admitted %d concurrent requests against a $0.20 budget at $0.05 each; want exactly 4", admitted)
	}
}

func TestReserveSpendRejectsWhenBudgetExhausted(t *testing.T) {
	m := NewMemory()
	hash := budgetedKey(t, m, "small", 0.05)

	if _, err := m.ReserveSpend(context.Background(), hash, 0.05); err != nil {
		t.Fatalf("first reservation should fit exactly: %v", err)
	}
	_, err := m.ReserveSpend(context.Background(), hash, 0.05)
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("second reservation: got %v, want ErrBudgetExceeded", err)
	}
}

func TestReserveSpendCountsRecordedSpend(t *testing.T) {
	m := NewMemory()
	hash := budgetedKey(t, m, "spent", 1.0)

	// Burn most of the budget through the normal accounting path.
	if err := m.RecordUsage(context.Background(), Usage{
		KeyName: "spent", SecretHash: hash, CostUSD: 0.98,
	}); err != nil {
		t.Fatalf("record usage: %v", err)
	}
	_, err := m.ReserveSpend(context.Background(), hash, 0.05)
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("reservation over recorded spend: got %v, want ErrBudgetExceeded", err)
	}
}

func TestReleaseSpendReturnsHeadroom(t *testing.T) {
	m := NewMemory()
	hash := budgetedKey(t, m, "cycle", 0.05)

	id, err := m.ReserveSpend(context.Background(), hash, 0.05)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if _, err := m.ReserveSpend(context.Background(), hash, 0.05); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("expected the budget to be held, got %v", err)
	}
	if err := m.ReleaseSpend(context.Background(), id); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, err := m.ReserveSpend(context.Background(), hash, 0.05); err != nil {
		t.Fatalf("after release the hold should be gone: %v", err)
	}
}

// A double release must not invent headroom that was never there.
func TestReleaseSpendIsIdempotent(t *testing.T) {
	m := NewMemory()
	hash := budgetedKey(t, m, "dup", 0.10)

	id, err := m.ReserveSpend(context.Background(), hash, 0.05)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	other, err := m.ReserveSpend(context.Background(), hash, 0.05)
	if err != nil {
		t.Fatalf("second reserve: %v", err)
	}
	for range 5 {
		if err := m.ReleaseSpend(context.Background(), id); err != nil {
			t.Fatalf("release: %v", err)
		}
	}
	// `other` is still held, so only one slot may be reclaimed.
	if _, err := m.ReserveSpend(context.Background(), hash, 0.05); err != nil {
		t.Fatalf("the released slot should be reusable: %v", err)
	}
	if _, err := m.ReserveSpend(context.Background(), hash, 0.05); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("repeated releases must not create headroom: got %v", err)
	}
	_ = other
}

func TestReserveSpendEnforcesOrgBudget(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	org, err := m.CreateOrg(ctx, "capped", 0.10)
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	// Two keys, each individually unlimited, sharing one capped org.
	var hashes []string
	for _, name := range []string{"a", "b"} {
		secret, err := m.CreateKeyIn(ctx, name, 0, org.ID, RootCreator)
		if err != nil {
			t.Fatalf("create key: %v", err)
		}
		k, err := m.Authenticate(ctx, secret)
		if err != nil {
			t.Fatalf("authenticate: %v", err)
		}
		hashes = append(hashes, k.SecretHash)
	}

	if _, err := m.ReserveSpend(ctx, hashes[0], 0.06); err != nil {
		t.Fatalf("first key should fit under the org cap: %v", err)
	}
	// The org has $0.04 left, so a $0.06 hold on the *other* key must fail even
	// though that key has no budget of its own.
	_, err = m.ReserveSpend(ctx, hashes[1], 0.06)
	if !errors.Is(err, ErrOrgBudgetExceeded) {
		t.Fatalf("sibling key over org cap: got %v, want ErrOrgBudgetExceeded", err)
	}
}

// A budget of 0 means unlimited everywhere else in the gateway; reservations
// must not quietly start blocking those keys.
func TestReserveSpendTreatsZeroBudgetAsUnlimited(t *testing.T) {
	m := NewMemory()
	hash := budgetedKey(t, m, "unlimited", 0)
	for i := range 100 {
		if _, err := m.ReserveSpend(context.Background(), hash, 1000); err != nil {
			t.Fatalf("reservation %d on an unlimited key: %v", i, err)
		}
	}
}

func TestReserveSpendRejectsUnknownKey(t *testing.T) {
	m := NewMemory()
	if _, err := m.ReserveSpend(context.Background(), "nope", 0.05); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("got %v, want ErrInvalidKey", err)
	}
}
