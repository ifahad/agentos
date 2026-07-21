package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres is a distributed token-bucket Limiter backed by a single row per org
// in rate_limit_buckets. Every gateway replica shares the same bucket, so the
// per-org limit holds across a horizontally-scaled deployment. Each Allow is one
// atomic refill-and-consume upsert, so concurrent replicas stay correct without
// application-level locking.
type Postgres struct {
	pool *pgxpool.Pool
	// now supplies the wall clock; tests inject a controllable value so refill
	// over time is deterministic. It is threaded into SQL as a parameter rather
	// than using now() so the injected clock governs refill.
	now func() time.Time
}

const rateLimitSchema = `
CREATE TABLE IF NOT EXISTS rate_limit_buckets (
    org_id     TEXT PRIMARY KEY,
    tokens     DOUBLE PRECISION NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);`

// NewPostgres ensures the rate_limit_buckets table exists and returns a
// distributed Limiter using pool (typically the same pool as the store).
func NewPostgres(ctx context.Context, pool *pgxpool.Pool) (*Postgres, error) {
	if pool == nil {
		return nil, fmt.Errorf("ratelimit postgres backend requires a database pool")
	}
	if _, err := pool.Exec(ctx, rateLimitSchema); err != nil {
		return nil, fmt.Errorf("create rate_limit_buckets: %w", err)
	}
	return &Postgres{pool: pool, now: time.Now}, nil
}

// allowSQL refills lazily from the elapsed time since updated_at (capped at the
// rpm burst ceiling) and consumes one token, all in one atomic upsert. A brand
// new bucket starts full (rpm) less the one token this call consumes. RETURNING
// yields the post-consume token count: >= 0 means the call is allowed; < 0 means
// the bucket was short and the call is denied (the debited token is restored by
// the caller so a denied call never leaves the bucket in permanent debt).
const allowSQL = `
INSERT INTO rate_limit_buckets AS b (org_id, tokens, updated_at)
VALUES ($1, $2::float8 - 1, $3::timestamptz)
ON CONFLICT (org_id) DO UPDATE SET
    tokens = LEAST($2::float8, b.tokens + extract(epoch FROM ($3::timestamptz - b.updated_at)) * $2 / 60.0) - 1,
    updated_at = $3::timestamptz
RETURNING b.tokens`

// restoreSQL adds the debited token back after a denied call so the bucket is
// not permanently debited (a denied call must not consume). It is a relative
// increment, so it composes correctly under concurrency from multiple replicas.
const restoreSQL = `UPDATE rate_limit_buckets SET tokens = tokens + 1 WHERE org_id = $1`

// Allow refills and consumes one token from the org's shared bucket. An rpm <= 0
// is unlimited. On a database error it fails open (allows) so a transient DB
// blip cannot take down proxy traffic — matching the never-erroring in-memory
// limiter's availability posture.
func (p *Postgres) Allow(key string, rpm int) (allowed bool, retryAfter time.Duration) {
	if rpm <= 0 {
		return true, 0
	}
	ctx := context.Background()
	now := p.now()
	var tokens float64
	if err := p.pool.QueryRow(ctx, allowSQL, key, float64(rpm), now).Scan(&tokens); err != nil {
		return true, 0 // fail open
	}
	if tokens >= 0 {
		return true, 0
	}
	// Denied: restore the debited token (no consume on denial) and report the
	// wait until the bucket recovers one whole token. tokens is in [-1, 0).
	if _, err := p.pool.Exec(ctx, restoreSQL, key); err != nil {
		return false, time.Second // still deny; retry shortly
	}
	perSecond := float64(rpm) / 60.0
	seconds := -tokens / perSecond
	return false, time.Duration(seconds * float64(time.Second))
}
