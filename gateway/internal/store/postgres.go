package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres is the pgx-backed Store. It creates its own schema on startup.
type Postgres struct {
	pool *pgxpool.Pool
}

const schema = `
CREATE TABLE IF NOT EXISTS keys (
    secret_hash        TEXT PRIMARY KEY,
    name               TEXT NOT NULL,
    monthly_budget_usd DOUBLE PRECISION NOT NULL,
    spend_usd          DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS "usage" (
    key_name      TEXT PRIMARY KEY,
    requests      BIGINT NOT NULL DEFAULT 0,
    input_tokens  BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    spend_usd     DOUBLE PRECISION NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS audit_log (
    id            BIGSERIAL PRIMARY KEY,
    key_name      TEXT NOT NULL,
    model         TEXT NOT NULL,
    input_tokens  BIGINT NOT NULL,
    output_tokens BIGINT NOT NULL,
    cost_usd      DOUBLE PRECISION NOT NULL,
    latency_ms    BIGINT NOT NULL,
    status        INTEGER NOT NULL,
    kind          TEXT NOT NULL DEFAULT 'chat',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Phase 2 migration: pre-existing databases lack the kind column; existing
-- rows are chat audits per the frozen contract.
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'chat';

-- Phase 5 multi-tenant RBAC (additive, migration-safe).
CREATE TABLE IF NOT EXISTS orgs (
    id                 TEXT PRIMARY KEY,
    name               TEXT NOT NULL,
    monthly_budget_usd DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS users (
    id          TEXT PRIMARY KEY,
    org_id      TEXT NOT NULL,
    email       TEXT NOT NULL,
    role        TEXT NOT NULL,
    token_hash  TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS users_org_idx ON users (org_id);
-- Pre-existing keys join the bootstrapped default org, attributed to root.
ALTER TABLE keys ADD COLUMN IF NOT EXISTS org_id     TEXT NOT NULL DEFAULT 'org_default';
ALTER TABLE keys ADD COLUMN IF NOT EXISTS created_by TEXT NOT NULL DEFAULT 'root';

-- Cost admitted but not yet settled: one row per request in flight. Budget
-- admission counts these alongside recorded spend, so concurrent requests on
-- one key cannot each be approved against the same pre-spend snapshot.
--
-- A table rather than a running total on the keys row, specifically so orphans
-- heal: a replica killed mid-request never runs its release, and a lost counter
-- increment would shrink that key's budget permanently, whereas a row simply
-- ages out of the window. Rows are also reaped opportunistically on reserve.
CREATE TABLE IF NOT EXISTS spend_reservations (
    id           BIGSERIAL PRIMARY KEY,
    secret_hash  TEXT NOT NULL,
    org_id       TEXT NOT NULL,
    estimate_usd DOUBLE PRECISION NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS spend_reservations_key_idx ON spend_reservations (secret_hash);
CREATE INDEX IF NOT EXISTS spend_reservations_org_idx ON spend_reservations (org_id);
CREATE INDEX IF NOT EXISTS spend_reservations_created_idx ON spend_reservations (created_at);

-- Phase 6 per-tenant rate limits (additive, migration-safe). 0 = unlimited,
-- so pre-existing orgs keep Phase 1–5 behavior.
ALTER TABLE orgs ADD COLUMN IF NOT EXISTS rate_limit_rpm INTEGER NOT NULL DEFAULT 0;

-- Phase 7 SCIM provisioning (additive, migration-safe). Pre-existing users are
-- active; external_id is the IdP-assigned SCIM id (nullable).
ALTER TABLE users ADD COLUMN IF NOT EXISTS active      BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE users ADD COLUMN IF NOT EXISTS external_id TEXT;

-- Phase 8 tenant isolation (H4, additive & migration-safe). Usage/spend/audit
-- become keyed on the stable per-key secret_hash and carry org_id, so two orgs
-- sharing a key name no longer contaminate each other's spend/usage/audit.
ALTER TABLE "usage" ADD COLUMN IF NOT EXISTS secret_hash TEXT;
ALTER TABLE "usage" ADD COLUMN IF NOT EXISTS org_id      TEXT;
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS secret_hash TEXT;
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS org_id      TEXT;
-- Backfill the new columns from the keys table by name (best effort: rows for
-- same-named keys were already conflated pre-migration and cannot be split).
UPDATE "usage" u SET secret_hash = k.secret_hash, org_id = k.org_id
    FROM keys k WHERE u.secret_hash IS NULL AND k.name = u.key_name;
UPDATE audit_log a SET secret_hash = k.secret_hash, org_id = k.org_id
    FROM keys k WHERE a.secret_hash IS NULL AND k.name = a.key_name;
-- Orphan usage rows (no matching key) keep the name as a synthetic identity so
-- the unique index below can be created without collisions (key_name was the
-- old PK, hence unique across existing rows).
UPDATE "usage" SET secret_hash = key_name WHERE secret_hash IS NULL;
-- Re-key the usage aggregate on secret_hash: drop the old key_name primary key
-- (so same-named keys in different orgs get distinct rows) and add a unique
-- index on secret_hash for the ON CONFLICT upsert target.
ALTER TABLE "usage" DROP CONSTRAINT IF EXISTS usage_pkey;
CREATE UNIQUE INDEX IF NOT EXISTS usage_secret_hash_idx ON "usage" (secret_hash);
CREATE INDEX IF NOT EXISTS usage_org_idx ON "usage" (org_id);
CREATE INDEX IF NOT EXISTS audit_log_org_idx ON audit_log (org_id);
-- Retention pruning is DELETE ... WHERE created_at < $1, which sequential-scans
-- without this. audit_log is the one table that grows without bound (retention
-- is off by default, deliberately), so it is also the one where a scan per
-- prune tick gets steadily worse. AuditList orders by id DESC and rides the
-- primary key, so it needs nothing here.
CREATE INDEX IF NOT EXISTS audit_log_created_at_idx ON audit_log (created_at);
CREATE INDEX IF NOT EXISTS keys_org_idx ON keys (org_id);
`

// NewPostgres connects to databaseURL and ensures the schema exists.
func NewPostgres(ctx context.Context, databaseURL string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

// Close releases the connection pool.
func (p *Postgres) Close() {
	p.pool.Close()
}

// Pool exposes the underlying pgx pool so sibling packages (e.g. the Postgres
// rate-limit backend) can share this store's connection to AGENTOS_DATABASE_URL.
func (p *Postgres) Pool() *pgxpool.Pool {
	return p.pool
}

func (p *Postgres) CreateKey(ctx context.Context, name string, budgetUSD float64) (string, error) {
	return p.CreateKeyIn(ctx, name, budgetUSD, DefaultOrgID, RootCreator)
}

func (p *Postgres) CreateKeyIn(ctx context.Context, name string, budgetUSD float64, orgID, createdBy string) (string, error) {
	secret, err := newSecret()
	if err != nil {
		return "", err
	}
	if orgID == "" {
		orgID = DefaultOrgID
	}
	if createdBy == "" {
		createdBy = RootCreator
	}
	_, err = p.pool.Exec(ctx,
		`INSERT INTO keys (secret_hash, name, monthly_budget_usd, org_id, created_by) VALUES ($1, $2, $3, $4, $5)`,
		hashSecret(secret), name, budgetUSD, orgID, createdBy)
	if err != nil {
		return "", fmt.Errorf("insert key: %w", err)
	}
	return secret, nil
}

func (p *Postgres) Authenticate(ctx context.Context, secret string) (*Key, error) {
	var k Key
	hash := hashSecret(secret)
	err := p.pool.QueryRow(ctx,
		`SELECT name, monthly_budget_usd, spend_usd, org_id FROM keys WHERE secret_hash = $1`,
		hash).Scan(&k.Name, &k.MonthlyBudgetUSD, &k.SpendUSD, &k.OrgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidKey
	}
	if err != nil {
		return nil, fmt.Errorf("authenticate: %w", err)
	}
	k.SecretHash = hash
	return &k, nil
}

// pgQuerier is satisfied by both *pgxpool.Pool and pgx.Tx so identity
// resolution can run inside or outside a transaction.
type pgQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// resolveIdentity fills the stable secret_hash and org_id for a usage/audit row
// from the owning key when the caller supplied only a key name (legacy path).
// An unresolved name falls back to the name itself as a synthetic identity and
// an empty org (root-only visibility). It never errors.
func resolveIdentity(ctx context.Context, q pgQuerier, u Usage) (secretHash, orgID string) {
	secretHash, orgID = u.SecretHash, u.OrgID
	if secretHash == "" || orgID == "" {
		var h, o string
		if err := q.QueryRow(ctx,
			`SELECT secret_hash, org_id FROM keys WHERE name = $1 ORDER BY created_at LIMIT 1`,
			u.KeyName).Scan(&h, &o); err == nil {
			if secretHash == "" {
				secretHash = h
			}
			if orgID == "" {
				orgID = o
			}
		}
	}
	if secretHash == "" {
		secretHash = u.KeyName
	}
	return secretHash, orgID
}

// ReserveSpend admits a request only if the key and its org both have room,
// counting cost already in flight.
//
// Serialisation comes from row locks, not from application logic: the org row
// is locked first and the key row second, always in that order, so concurrent
// reservations queue instead of racing and no pair of callers can deadlock by
// grabbing the two locks in opposite orders. Both budget reads then happen
// inside that lock, which is what closes the check-then-act window.
func (p *Postgres) ReserveSpend(ctx context.Context, secretHash string, estimateUSD float64) (string, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin reserve: %w", err)
	}
	defer tx.Rollback(ctx)

	var orgID string
	if err := tx.QueryRow(ctx,
		`SELECT org_id FROM keys WHERE secret_hash = $1`, secretHash).Scan(&orgID); err != nil {
		return "", ErrInvalidKey
	}

	// Lock the org row first, then the key row, and always in that order, so
	// concurrent reservations serialise instead of racing and no two callers
	// can deadlock by taking the locks in opposite orders. A missing org row is
	// not an error: pre-Phase-5 keys predate orgs and are unconstrained at the
	// org level.
	var orgBudget float64
	hasOrg := tx.QueryRow(ctx,
		`SELECT monthly_budget_usd FROM orgs WHERE id = $1 FOR UPDATE`, orgID).Scan(&orgBudget) == nil

	var spend, keyBudget float64
	if err := tx.QueryRow(ctx,
		`SELECT spend_usd, monthly_budget_usd FROM keys WHERE secret_hash = $1 FOR UPDATE`,
		secretHash).Scan(&spend, &keyBudget); err != nil {
		return "", ErrInvalidKey
	}

	// Clear orphans from processes that died mid-request before counting.
	if _, err := tx.Exec(ctx,
		`DELETE FROM spend_reservations WHERE created_at < now() - $1::interval`,
		ttlInterval()); err != nil {
		return "", fmt.Errorf("reap reservations: %w", err)
	}

	// A budget of 0 means unlimited, matching the rest of the gateway.
	if keyBudget > 0 {
		var reserved float64
		if err := tx.QueryRow(ctx,
			`SELECT COALESCE(SUM(estimate_usd), 0) FROM spend_reservations WHERE secret_hash = $1`,
			secretHash).Scan(&reserved); err != nil {
			return "", fmt.Errorf("key reserved: %w", err)
		}
		if spend+reserved+estimateUSD > keyBudget {
			return "", ErrBudgetExceeded
		}
	}

	if hasOrg && orgBudget > 0 {
		var committed float64
		if err := tx.QueryRow(ctx,
			`SELECT COALESCE((SELECT SUM(spend_usd) FROM keys WHERE org_id = $1), 0)
			      + COALESCE((SELECT SUM(estimate_usd) FROM spend_reservations WHERE org_id = $1), 0)`,
			orgID).Scan(&committed); err != nil {
			return "", fmt.Errorf("org committed: %w", err)
		}
		if committed+estimateUSD > orgBudget {
			return "", ErrOrgBudgetExceeded
		}
	}

	var id int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO spend_reservations (secret_hash, org_id, estimate_usd)
		 VALUES ($1, $2, $3) RETURNING id`,
		secretHash, orgID, estimateUSD).Scan(&id); err != nil {
		return "", fmt.Errorf("reserve: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit reserve: %w", err)
	}
	return strconv.FormatInt(id, 10), nil
}

// ReleaseSpend drops a reservation by handle. Releasing an unknown or already
// released handle is a no-op, so a double release cannot manufacture headroom.
func (p *Postgres) ReleaseSpend(ctx context.Context, reservationID string) error {
	id, err := strconv.ParseInt(reservationID, 10, 64)
	if err != nil {
		return nil
	}
	if _, err := p.pool.Exec(ctx, `DELETE FROM spend_reservations WHERE id = $1`, id); err != nil {
		return fmt.Errorf("release: %w", err)
	}
	return nil
}

// ttlInterval renders ReservationTTL as a Postgres interval literal, so the
// expiry window has exactly one definition shared by both store backends.
func ttlInterval() string {
	return strconv.FormatInt(int64(ReservationTTL.Seconds()), 10) + " seconds"
}

func (p *Postgres) RecordUsage(ctx context.Context, u Usage) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	secretHash, orgID := resolveIdentity(ctx, tx, u)

	// Attribute spend by the stable secret_hash when the caller provided one
	// (the server hot path) so same-named keys in different orgs stay isolated
	// (H4); legacy name-only callers keep updating by name.
	if u.SecretHash != "" {
		if _, err := tx.Exec(ctx,
			`UPDATE keys SET spend_usd = spend_usd + $1 WHERE secret_hash = $2`,
			u.CostUSD, u.SecretHash); err != nil {
			return fmt.Errorf("update spend: %w", err)
		}
	} else {
		if _, err := tx.Exec(ctx,
			`UPDATE keys SET spend_usd = spend_usd + $1 WHERE name = $2`,
			u.CostUSD, u.KeyName); err != nil {
			return fmt.Errorf("update spend: %w", err)
		}
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO "usage" (secret_hash, key_name, org_id, requests, input_tokens, output_tokens, spend_usd)
         VALUES ($1, $2, $3, 1, $4, $5, $6)
         ON CONFLICT (secret_hash) DO UPDATE SET
           requests = "usage".requests + 1,
           input_tokens = "usage".input_tokens + EXCLUDED.input_tokens,
           output_tokens = "usage".output_tokens + EXCLUDED.output_tokens,
           spend_usd = "usage".spend_usd + EXCLUDED.spend_usd,
           key_name = EXCLUDED.key_name,
           org_id = COALESCE(NULLIF(EXCLUDED.org_id, ''), "usage".org_id)`,
		secretHash, u.KeyName, orgID, u.InputTokens, u.OutputTokens, u.CostUSD); err != nil {
		return fmt.Errorf("upsert usage: %w", err)
	}
	if _, err := tx.Exec(ctx, insertAuditSQL,
		secretHash, orgID, u.KeyName, u.Model, u.InputTokens, u.OutputTokens, u.CostUSD, u.LatencyMS, u.Status, kindOrChat(u.Kind)); err != nil {
		return fmt.Errorf("insert audit_log: %w", err)
	}
	return tx.Commit(ctx)
}

const insertAuditSQL = `INSERT INTO audit_log (secret_hash, org_id, key_name, model, input_tokens, output_tokens, cost_usd, latency_ms, status, kind)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

// PruneAudit drops audit rows recorded before the cutoff.
func (p *Postgres) PruneAudit(ctx context.Context, before time.Time) (int64, error) {
	tag, err := p.pool.Exec(ctx, `DELETE FROM audit_log WHERE created_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("prune audit: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (p *Postgres) RecordAudit(ctx context.Context, u Usage) error {
	secretHash, orgID := resolveIdentity(ctx, p.pool, u)
	if _, err := p.pool.Exec(ctx, insertAuditSQL,
		secretHash, orgID, u.KeyName, u.Model, u.InputTokens, u.OutputTokens, u.CostUSD, u.LatencyMS, u.Status, kindOrChat(u.Kind)); err != nil {
		return fmt.Errorf("insert audit_log: %w", err)
	}
	return nil
}

func (p *Postgres) AuditList(ctx context.Context, orgID string, limit int) ([]AuditEntry, error) {
	// An empty orgID ($1 = '') returns all rows (root); a non-empty orgID scopes
	// to that org, pushing the WHERE org_id filter into the query (H4).
	rows, err := p.pool.Query(ctx,
		`SELECT created_at, key_name, model, input_tokens, output_tokens, cost_usd, latency_ms, status, kind
         FROM audit_log
         WHERE ($1 = '' OR org_id = $1)
         ORDER BY id DESC LIMIT $2`, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("query audit_log: %w", err)
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.TS, &e.KeyName, &e.Model, &e.InputTokens, &e.OutputTokens,
			&e.CostUSD, &e.LatencyMS, &e.Status, &e.Kind); err != nil {
			return nil, fmt.Errorf("scan audit_log: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (p *Postgres) Usage(ctx context.Context, orgID string) ([]KeyUsage, error) {
	// One row per key (by secret_hash), so same-named keys across orgs are
	// distinct. An empty orgID ($1 = '') returns all keys (root); otherwise the
	// org filter is pushed down (H4). The name is still shown but is never the
	// isolation key.
	rows, err := p.pool.Query(ctx,
		`SELECT k.name,
                COALESCE(u.requests, 0),
                COALESCE(u.input_tokens, 0),
                COALESCE(u.output_tokens, 0),
                COALESCE(u.spend_usd, 0)
         FROM keys k
         LEFT JOIN "usage" u ON u.secret_hash = k.secret_hash
         WHERE ($1 = '' OR k.org_id = $1)
         ORDER BY k.name, k.created_at`, orgID)
	if err != nil {
		return nil, fmt.Errorf("query usage: %w", err)
	}
	defer rows.Close()
	var out []KeyUsage
	for rows.Next() {
		var u KeyUsage
		if err := rows.Scan(&u.Name, &u.Requests, &u.InputTokens, &u.OutputTokens, &u.SpendUSD); err != nil {
			return nil, fmt.Errorf("scan usage: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (p *Postgres) Keys(ctx context.Context) ([]KeyInfo, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT name, monthly_budget_usd, spend_usd, org_id FROM keys ORDER BY name, created_at`)
	if err != nil {
		return nil, fmt.Errorf("query keys: %w", err)
	}
	defer rows.Close()
	var out []KeyInfo
	for rows.Next() {
		var k KeyInfo
		if err := rows.Scan(&k.Name, &k.MonthlyBudgetUSD, &k.SpendUSD, &k.OrgID); err != nil {
			return nil, fmt.Errorf("scan key: %w", err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (p *Postgres) EnsureKey(ctx context.Context, name, secret string, budgetUSD float64) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO keys (secret_hash, name, monthly_budget_usd, org_id, created_by) VALUES ($1, $2, $3, $4, $5)
         ON CONFLICT (secret_hash) DO UPDATE SET
           name = EXCLUDED.name,
           monthly_budget_usd = EXCLUDED.monthly_budget_usd`,
		hashSecret(secret), name, budgetUSD, DefaultOrgID, RootCreator)
	if err != nil {
		return fmt.Errorf("ensure key: %w", err)
	}
	return nil
}

func (p *Postgres) CreateOrg(ctx context.Context, name string, monthlyBudgetUSD float64) (*Org, error) {
	id, err := newID("org_")
	if err != nil {
		return nil, err
	}
	org := &Org{ID: id, Name: name, MonthlyBudgetUSD: monthlyBudgetUSD}
	err = p.pool.QueryRow(ctx,
		`INSERT INTO orgs (id, name, monthly_budget_usd) VALUES ($1, $2, $3) RETURNING created_at`,
		id, name, monthlyBudgetUSD).Scan(&org.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert org: %w", err)
	}
	return org, nil
}

func (p *Postgres) EnsureOrg(ctx context.Context, id, name string, monthlyBudgetUSD float64) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO orgs (id, name, monthly_budget_usd) VALUES ($1, $2, $3)
         ON CONFLICT (id) DO UPDATE SET
           name = EXCLUDED.name,
           monthly_budget_usd = EXCLUDED.monthly_budget_usd`,
		id, name, monthlyBudgetUSD)
	if err != nil {
		return fmt.Errorf("ensure org: %w", err)
	}
	return nil
}

func (p *Postgres) Org(ctx context.Context, id string) (*Org, error) {
	var o Org
	err := p.pool.QueryRow(ctx,
		`SELECT id, name, monthly_budget_usd, rate_limit_rpm, created_at FROM orgs WHERE id = $1`, id).
		Scan(&o.ID, &o.Name, &o.MonthlyBudgetUSD, &o.RateLimitRPM, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrgNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query org: %w", err)
	}
	return &o, nil
}

func (p *Postgres) Orgs(ctx context.Context) ([]Org, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT id, name, monthly_budget_usd, rate_limit_rpm, created_at FROM orgs ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query orgs: %w", err)
	}
	defer rows.Close()
	var out []Org
	for rows.Next() {
		var o Org
		if err := rows.Scan(&o.ID, &o.Name, &o.MonthlyBudgetUSD, &o.RateLimitRPM, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan org: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// UpdateOrg patches the monthly budget and/or rate limit; nil fields are left
// unchanged. It uses COALESCE so a single statement handles any combination.
func (p *Postgres) UpdateOrg(ctx context.Context, id string, monthlyBudgetUSD *float64, rateLimitRPM *int) (*Org, error) {
	var o Org
	err := p.pool.QueryRow(ctx,
		`UPDATE orgs SET
                 monthly_budget_usd = COALESCE($2, monthly_budget_usd),
                 rate_limit_rpm     = COALESCE($3, rate_limit_rpm)
             WHERE id = $1
             RETURNING id, name, monthly_budget_usd, rate_limit_rpm, created_at`,
		id, monthlyBudgetUSD, rateLimitRPM).
		Scan(&o.ID, &o.Name, &o.MonthlyBudgetUSD, &o.RateLimitRPM, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrgNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update org: %w", err)
	}
	return &o, nil
}

func (p *Postgres) OrgSpend(ctx context.Context, orgID string) (float64, error) {
	var total float64
	err := p.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(spend_usd), 0) FROM keys WHERE org_id = $1`, orgID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("org spend: %w", err)
	}
	return total, nil
}

// OrgSpends aggregates every org's key spend in a single grouped query.
func (p *Postgres) OrgSpends(ctx context.Context) (map[string]float64, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT org_id, COALESCE(SUM(spend_usd), 0) FROM keys GROUP BY org_id`)
	if err != nil {
		return nil, fmt.Errorf("org spends: %w", err)
	}
	defer rows.Close()
	out := make(map[string]float64)
	for rows.Next() {
		var orgID string
		var spend float64
		if err := rows.Scan(&orgID, &spend); err != nil {
			return nil, fmt.Errorf("scan org spend: %w", err)
		}
		out[orgID] = spend
	}
	return out, rows.Err()
}

func (p *Postgres) CreateUser(ctx context.Context, orgID, email, role string) (*User, string, error) {
	return p.CreateUserWithExternalID(ctx, orgID, email, role, "")
}

func (p *Postgres) CreateUserWithExternalID(ctx context.Context, orgID, email, role, externalID string) (*User, string, error) {
	if err := validateRole(role); err != nil {
		return nil, "", err
	}
	if _, err := p.Org(ctx, orgID); err != nil {
		return nil, "", err
	}
	id, err := newID("usr_")
	if err != nil {
		return nil, "", err
	}
	token, err := newUserToken()
	if err != nil {
		return nil, "", err
	}
	u := &User{ID: id, OrgID: orgID, Email: email, Role: role, Active: true, ExternalID: externalID}
	// NULLIF stores an empty external id as SQL NULL; COALESCE reads it back as "".
	err = p.pool.QueryRow(ctx,
		`INSERT INTO users (id, org_id, email, role, token_hash, active, external_id)
         VALUES ($1, $2, $3, $4, $5, true, NULLIF($6, '')) RETURNING created_at`,
		id, orgID, email, role, hashSecret(token), externalID).Scan(&u.CreatedAt)
	if err != nil {
		return nil, "", fmt.Errorf("insert user: %w", err)
	}
	return u, token, nil
}

const selectUserCols = `id, org_id, email, role, active, COALESCE(external_id, ''), created_at`

func scanUser(row pgx.Row, u *User) error {
	return row.Scan(&u.ID, &u.OrgID, &u.Email, &u.Role, &u.Active, &u.ExternalID, &u.CreatedAt)
}

func (p *Postgres) AuthenticateUser(ctx context.Context, token string) (*User, error) {
	var u User
	err := scanUser(p.pool.QueryRow(ctx,
		`SELECT `+selectUserCols+` FROM users WHERE token_hash = $1`, hashSecret(token)), &u)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidToken
	}
	if err != nil {
		return nil, fmt.Errorf("authenticate user: %w", err)
	}
	if !u.Active {
		return nil, ErrUserInactive
	}
	return &u, nil
}

func (p *Postgres) UserByEmail(ctx context.Context, orgID, email string) (*User, error) {
	var u User
	err := scanUser(p.pool.QueryRow(ctx,
		`SELECT `+selectUserCols+` FROM users WHERE org_id = $1 AND email = $2 ORDER BY created_at LIMIT 1`,
		orgID, email), &u)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("user by email: %w", err)
	}
	return &u, nil
}

func (p *Postgres) UserByExternalID(ctx context.Context, orgID, externalID string) (*User, error) {
	if externalID == "" {
		return nil, ErrUserNotFound
	}
	var u User
	err := scanUser(p.pool.QueryRow(ctx,
		`SELECT `+selectUserCols+` FROM users WHERE org_id = $1 AND external_id = $2 ORDER BY created_at LIMIT 1`,
		orgID, externalID), &u)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("user by external id: %w", err)
	}
	return &u, nil
}

func (p *Postgres) SetUserActive(ctx context.Context, userID string, active bool) error {
	tag, err := p.pool.Exec(ctx, `UPDATE users SET active = $2 WHERE id = $1`, userID, active)
	if err != nil {
		return fmt.Errorf("set user active: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (p *Postgres) SetUserExternalID(ctx context.Context, userID, externalID string) error {
	tag, err := p.pool.Exec(ctx, `UPDATE users SET external_id = NULLIF($2, '') WHERE id = $1`, userID, externalID)
	if err != nil {
		return fmt.Errorf("set user external id: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (p *Postgres) IssueUserToken(ctx context.Context, userID string) (string, error) {
	token, err := newUserToken()
	if err != nil {
		return "", err
	}
	tag, err := p.pool.Exec(ctx,
		`UPDATE users SET token_hash = $1 WHERE id = $2`, hashSecret(token), userID)
	if err != nil {
		return "", fmt.Errorf("issue user token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return "", ErrUserNotFound
	}
	return token, nil
}

func (p *Postgres) Users(ctx context.Context, orgID string) ([]User, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT `+selectUserCols+` FROM users WHERE org_id = $1 ORDER BY id`, orgID)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := scanUser(rows, &u); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (p *Postgres) DeleteUser(ctx context.Context, orgID, userID string) error {
	tag, err := p.pool.Exec(ctx, `DELETE FROM users WHERE id = $1 AND org_id = $2`, userID, orgID)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}
