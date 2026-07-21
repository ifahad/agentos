package ratelimit

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// newTestPGLimiter connects to AGENTOS_TEST_DATABASE_URL (skipping when unset),
// truncates the bucket table, and returns a Postgres limiter whose clock the
// test controls via the returned pointer.
func newTestPGLimiter(t *testing.T) (*Postgres, *time.Time) {
	t.Helper()
	dsn := os.Getenv("AGENTOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("AGENTOS_TEST_DATABASE_URL not set; skipping postgres limiter tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	l, err := NewPostgres(ctx, pool)
	if err != nil {
		t.Fatalf("NewPostgres: %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE rate_limit_buckets`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	now := time.Unix(1_700_000_000, 0)
	l.now = func() time.Time { return now }
	return l, &now
}

func TestPGLimiterUnlimited(t *testing.T) {
	l, _ := newTestPGLimiter(t)
	for i := 0; i < 100; i++ {
		if ok, ra := l.Allow("org", 0); !ok || ra != 0 {
			t.Fatalf("rpm=0 call %d: allowed=%v ra=%v", i, ok, ra)
		}
	}
}

func TestPGLimiterBurstThenDeny(t *testing.T) {
	l, _ := newTestPGLimiter(t)
	// rpm=3: a fresh bucket starts full, so three immediate calls pass.
	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("org", 3); !ok {
			t.Fatalf("burst call %d denied, want allowed", i)
		}
	}
	ok, retryAfter := l.Allow("org", 3)
	if ok {
		t.Fatal("4th call allowed, want denied")
	}
	if retryAfter <= 0 {
		t.Fatalf("retryAfter = %v, want > 0", retryAfter)
	}
	// rpm=3 → one token every 20s; from empty that is ~20s.
	if got := RetryAfterSeconds(retryAfter); got != 20 {
		t.Errorf("RetryAfterSeconds = %d, want 20", got)
	}
}

func TestPGLimiterDeniedCallDoesNotPermanentlyDebt(t *testing.T) {
	l, now := newTestPGLimiter(t)
	// Drain rpm=2 bucket, then hammer denials.
	l.Allow("org", 2)
	l.Allow("org", 2)
	for i := 0; i < 25; i++ {
		if ok, _ := l.Allow("org", 2); ok {
			t.Fatalf("denial-storm call %d allowed unexpectedly", i)
		}
	}
	// A denied call must not consume: after exactly one refill window (30s for
	// rpm=2) a single token is available and the next call is allowed. If denials
	// had accumulated debt, this would still be rejected.
	*now = now.Add(30 * time.Second)
	l.now = func() time.Time { return *now }
	if ok, _ := l.Allow("org", 2); !ok {
		t.Fatal("after one refill window post-denial-storm: denied, want allowed (permanent debt bug)")
	}
}

func TestPGLimiterRefillOverTime(t *testing.T) {
	l, now := newTestPGLimiter(t)
	// Drain a rpm=60 bucket (one token/sec): 60 immediate calls.
	for i := 0; i < 60; i++ {
		if ok, _ := l.Allow("org", 60); !ok {
			t.Fatalf("drain call %d denied", i)
		}
	}
	if ok, _ := l.Allow("org", 60); ok {
		t.Fatal("call after drain allowed, want denied")
	}
	// Advance 1s → exactly one token refills.
	*now = now.Add(1 * time.Second)
	l.now = func() time.Time { return *now }
	if ok, _ := l.Allow("org", 60); !ok {
		t.Fatal("after 1s refill: denied, want allowed")
	}
	if ok, _ := l.Allow("org", 60); ok {
		t.Fatal("second call after single refill allowed, want denied")
	}
}

// TestPGLimiterConcurrentNeverExceedsCap runs two goroutines hammering one
// shared bucket at a frozen clock; the total allowed must never exceed the cap.
func TestPGLimiterConcurrentNeverExceedsCap(t *testing.T) {
	l, _ := newTestPGLimiter(t)
	const rpm = 20
	key := fmt.Sprintf("concurrent-%d", time.Now().UnixNano())

	var allowed atomic.Int64
	var wg sync.WaitGroup
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				if ok, _ := l.Allow(key, rpm); ok {
					allowed.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	// Clock is frozen, so no refill: at most rpm requests may be admitted.
	if got := allowed.Load(); got > rpm {
		t.Errorf("allowed = %d, want <= %d (cap exceeded under concurrency)", got, rpm)
	}
	if got := allowed.Load(); got != rpm {
		t.Errorf("allowed = %d, want exactly %d from a full frozen bucket", got, rpm)
	}
}
