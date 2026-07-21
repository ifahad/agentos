// Package ratelimit provides per-tenant (per-org) request rate limiting
// (Phase 6). The default in-memory token bucket keeps buckets in process; a
// Postgres-backed bucket (Phase 7) shares one bucket across gateway replicas
// via an atomic SQL upsert. Both satisfy the Limiter interface and are safe for
// concurrent use; the in-memory limiter accepts an injectable clock so its
// refill behavior can be unit-tested deterministically.
package ratelimit

import (
	"math"
	"sync"
	"time"
)

// Limiter enforces a per-key (per-org) request rate. Allow attempts to consume
// one request against the key's bucket sized for rpm. An rpm <= 0 means
// unlimited: Allow permits and returns a zero Retry-After. Otherwise a denied
// call returns allowed=false and a positive retryAfter. Implementations: the
// in-memory Memory bucket (default) and the Postgres distributed bucket.
type Limiter interface {
	Allow(key string, rpm int) (allowed bool, retryAfter time.Duration)
}

// bucket is one org's token bucket.
type bucket struct {
	tokens float64   // current whole/fractional tokens available
	rpm    int       // the requests-per-minute the bucket is sized for
	last   time.Time // last refill timestamp
}

// Memory holds an in-process token bucket per key (org id). A key's effective
// rpm is supplied on each call, so an org's limit can change at runtime.
type Memory struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	now     func() time.Time
}

// New returns an in-memory Limiter driven by the real wall clock.
func New() *Memory { return NewWithClock(time.Now) }

// NewWithClock returns an in-memory Limiter whose refill math uses the supplied
// clock. Tests inject a controllable now to make token refill deterministic.
func NewWithClock(now func() time.Time) *Memory {
	if now == nil {
		now = time.Now
	}
	return &Memory{buckets: make(map[string]*bucket), now: now}
}

// Allow attempts to consume one token from key's bucket sized for rpm.
//
// An rpm <= 0 means unlimited: Allow always permits and returns a zero
// Retry-After. Otherwise the bucket refills at rpm/60 tokens per second with a
// burst capacity of rpm. When a token is available it is consumed and allowed
// is true; when the bucket is empty allowed is false and retryAfter is the time
// until the next whole token refills (always > 0).
func (l *Memory) Allow(key string, rpm int) (allowed bool, retryAfter time.Duration) {
	if rpm <= 0 {
		return true, 0
	}
	now := l.now()
	perSecond := float64(rpm) / 60.0
	capacity := float64(rpm)

	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		// A fresh bucket starts full so the first burst up to rpm is allowed.
		b = &bucket{tokens: capacity, rpm: rpm, last: now}
		l.buckets[key] = b
	}

	// Refill based on elapsed time since the last observation.
	if elapsed := now.Sub(b.last); elapsed > 0 {
		b.tokens += elapsed.Seconds() * perSecond
		b.last = now
	}
	// If the org's rpm changed, resize the burst ceiling.
	if b.rpm != rpm {
		b.rpm = rpm
	}
	if b.tokens > capacity {
		b.tokens = capacity
	}

	if b.tokens >= 1 {
		b.tokens -= 1
		return true, 0
	}
	// Time for (1 - tokens) more tokens to accrue at perSecond.
	deficit := 1 - b.tokens
	seconds := deficit / perSecond
	return false, time.Duration(seconds * float64(time.Second))
}

// RetryAfterSeconds converts a Retry-After duration to whole seconds, rounding
// up and clamping to a minimum of 1 (the HTTP header is an integer count).
func RetryAfterSeconds(d time.Duration) int {
	if d <= 0 {
		return 1
	}
	return int(math.Ceil(d.Seconds()))
}
