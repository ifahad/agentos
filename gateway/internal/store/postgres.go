package store

import (
	"context"
	"errors"
	"fmt"

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
	err := p.pool.QueryRow(ctx,
		`SELECT name, monthly_budget_usd, spend_usd, org_id FROM keys WHERE secret_hash = $1`,
		hashSecret(secret)).Scan(&k.Name, &k.MonthlyBudgetUSD, &k.SpendUSD, &k.OrgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidKey
	}
	if err != nil {
		return nil, fmt.Errorf("authenticate: %w", err)
	}
	return &k, nil
}

func (p *Postgres) RecordUsage(ctx context.Context, u Usage) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE keys SET spend_usd = spend_usd + $1 WHERE name = $2`,
		u.CostUSD, u.KeyName); err != nil {
		return fmt.Errorf("update spend: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO "usage" (key_name, requests, input_tokens, output_tokens, spend_usd)
         VALUES ($1, 1, $2, $3, $4)
         ON CONFLICT (key_name) DO UPDATE SET
           requests = "usage".requests + 1,
           input_tokens = "usage".input_tokens + EXCLUDED.input_tokens,
           output_tokens = "usage".output_tokens + EXCLUDED.output_tokens,
           spend_usd = "usage".spend_usd + EXCLUDED.spend_usd`,
		u.KeyName, u.InputTokens, u.OutputTokens, u.CostUSD); err != nil {
		return fmt.Errorf("upsert usage: %w", err)
	}
	if _, err := tx.Exec(ctx, insertAuditSQL,
		u.KeyName, u.Model, u.InputTokens, u.OutputTokens, u.CostUSD, u.LatencyMS, u.Status, kindOrChat(u.Kind)); err != nil {
		return fmt.Errorf("insert audit_log: %w", err)
	}
	return tx.Commit(ctx)
}

const insertAuditSQL = `INSERT INTO audit_log (key_name, model, input_tokens, output_tokens, cost_usd, latency_ms, status, kind)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

func (p *Postgres) RecordAudit(ctx context.Context, u Usage) error {
	if _, err := p.pool.Exec(ctx, insertAuditSQL,
		u.KeyName, u.Model, u.InputTokens, u.OutputTokens, u.CostUSD, u.LatencyMS, u.Status, kindOrChat(u.Kind)); err != nil {
		return fmt.Errorf("insert audit_log: %w", err)
	}
	return nil
}

func (p *Postgres) AuditList(ctx context.Context, limit int) ([]AuditEntry, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT created_at, key_name, model, input_tokens, output_tokens, cost_usd, latency_ms, status, kind
         FROM audit_log ORDER BY id DESC LIMIT $1`, limit)
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

func (p *Postgres) Usage(ctx context.Context) ([]KeyUsage, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT k.name,
                COALESCE(u.requests, 0),
                COALESCE(u.input_tokens, 0),
                COALESCE(u.output_tokens, 0),
                COALESCE(u.spend_usd, 0)
         FROM (SELECT DISTINCT name FROM keys) k
         LEFT JOIN "usage" u ON u.key_name = k.name
         ORDER BY k.name`)
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
		`SELECT id, name, monthly_budget_usd, created_at FROM orgs WHERE id = $1`, id).
		Scan(&o.ID, &o.Name, &o.MonthlyBudgetUSD, &o.CreatedAt)
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
		`SELECT id, name, monthly_budget_usd, created_at FROM orgs ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query orgs: %w", err)
	}
	defer rows.Close()
	var out []Org
	for rows.Next() {
		var o Org
		if err := rows.Scan(&o.ID, &o.Name, &o.MonthlyBudgetUSD, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan org: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
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

func (p *Postgres) CreateUser(ctx context.Context, orgID, email, role string) (*User, string, error) {
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
	u := &User{ID: id, OrgID: orgID, Email: email, Role: role}
	err = p.pool.QueryRow(ctx,
		`INSERT INTO users (id, org_id, email, role, token_hash) VALUES ($1, $2, $3, $4, $5) RETURNING created_at`,
		id, orgID, email, role, hashSecret(token)).Scan(&u.CreatedAt)
	if err != nil {
		return nil, "", fmt.Errorf("insert user: %w", err)
	}
	return u, token, nil
}

func (p *Postgres) AuthenticateUser(ctx context.Context, token string) (*User, error) {
	var u User
	err := p.pool.QueryRow(ctx,
		`SELECT id, org_id, email, role, created_at FROM users WHERE token_hash = $1`,
		hashSecret(token)).Scan(&u.ID, &u.OrgID, &u.Email, &u.Role, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidToken
	}
	if err != nil {
		return nil, fmt.Errorf("authenticate user: %w", err)
	}
	return &u, nil
}

func (p *Postgres) Users(ctx context.Context, orgID string) ([]User, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT id, org_id, email, role, created_at FROM users WHERE org_id = $1 ORDER BY id`, orgID)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.OrgID, &u.Email, &u.Role, &u.CreatedAt); err != nil {
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
