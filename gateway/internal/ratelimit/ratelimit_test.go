package ratelimit

import (
	"sync"
	"testing"
	"time"
)

func TestUnlimitedRPMAlwaysAllows(t *testing.T) {
	l := New()
	for i := 0; i < 1000; i++ {
		if ok, ra := l.Allow("org", 0); !ok || ra != 0 {
			t.Fatalf("rpm=0 call %d: allowed=%v retryAfter=%v, want allowed with 0", i, ok, ra)
		}
	}
}

func TestBurstThenReject(t *testing.T) {
	now := time.Unix(0, 0)
	l := NewWithClock(func() time.Time { return now })

	// rpm=3: a fresh bucket starts full, so three immediate calls pass.
	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("org", 3); !ok {
			t.Fatalf("burst call %d rejected, want allowed", i)
		}
	}
	// The fourth (clock frozen, no refill) is rejected with a positive wait.
	ok, retryAfter := l.Allow("org", 3)
	if ok {
		t.Fatal("4th call allowed, want rejected")
	}
	if retryAfter <= 0 {
		t.Fatalf("retryAfter = %v, want > 0", retryAfter)
	}
	// rpm=3 → one token every 20s; from empty that is 20s.
	if got := RetryAfterSeconds(retryAfter); got != 20 {
		t.Errorf("RetryAfterSeconds = %d, want 20", got)
	}
}

func TestRefillOverTime(t *testing.T) {
	now := time.Unix(0, 0)
	l := NewWithClock(func() time.Time { return now })

	// Drain a rpm=60 bucket (one token/sec) fully: 60 immediate calls.
	for i := 0; i < 60; i++ {
		if ok, _ := l.Allow("org", 60); !ok {
			t.Fatalf("drain call %d rejected", i)
		}
	}
	if ok, _ := l.Allow("org", 60); ok {
		t.Fatal("call after drain allowed, want rejected")
	}
	// Advance 1s → exactly one token refills.
	now = now.Add(1 * time.Second)
	if ok, _ := l.Allow("org", 60); !ok {
		t.Fatal("after 1s refill: rejected, want allowed")
	}
	// Bucket empty again immediately.
	if ok, _ := l.Allow("org", 60); ok {
		t.Fatal("second call after single refill allowed, want rejected")
	}
}

func TestPerKeyIsolation(t *testing.T) {
	now := time.Unix(0, 0)
	l := NewWithClock(func() time.Time { return now })

	// Drain org A (rpm=1: one token).
	if ok, _ := l.Allow("A", 1); !ok {
		t.Fatal("A first call rejected")
	}
	if ok, _ := l.Allow("A", 1); ok {
		t.Fatal("A second call allowed, want rejected")
	}
	// Org B has its own full bucket.
	if ok, _ := l.Allow("B", 1); !ok {
		t.Fatal("B first call rejected, buckets not isolated")
	}
}

func TestRPMChangeResizesBurst(t *testing.T) {
	now := time.Unix(0, 0)
	l := NewWithClock(func() time.Time { return now })

	// Start at rpm=1, consume the single token.
	if ok, _ := l.Allow("org", 1); !ok {
		t.Fatal("rpm=1 first call rejected")
	}
	if ok, _ := l.Allow("org", 1); ok {
		t.Fatal("rpm=1 second call allowed, want rejected")
	}
	// Raise to rpm=100 without advancing the clock. The ceiling grows but the
	// tokens do not jump; the bucket is still near-empty, so this is rejected.
	if ok, _ := l.Allow("org", 100); ok {
		t.Fatal("immediately after raising rpm, call allowed; tokens should not jump")
	}
	// After a full second at rpm=100, ~1.67 tokens accrue → allowed.
	now = now.Add(1 * time.Second)
	if ok, _ := l.Allow("org", 100); !ok {
		t.Fatal("after refill at rpm=100, call rejected")
	}
}

func TestConcurrentAllowIsRaceFree(t *testing.T) {
	l := New()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				l.Allow("shared", 10)
			}
		}()
	}
	wg.Wait()
}

func TestRetryAfterSecondsClamps(t *testing.T) {
	if got := RetryAfterSeconds(0); got != 1 {
		t.Errorf("RetryAfterSeconds(0) = %d, want 1", got)
	}
	if got := RetryAfterSeconds(1500 * time.Millisecond); got != 2 {
		t.Errorf("RetryAfterSeconds(1.5s) = %d, want 2", got)
	}
}
