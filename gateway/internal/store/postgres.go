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
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
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
	secret, err := newSecret()
	if err != nil {
		return "", err
	}
	_, err = p.pool.Exec(ctx,
		`INSERT INTO keys (secret_hash, name, monthly_budget_usd) VALUES ($1, $2, $3)`,
		hashSecret(secret), name, budgetUSD)
	if err != nil {
		return "", fmt.Errorf("insert key: %w", err)
	}
	return secret, nil
}

func (p *Postgres) Authenticate(ctx context.Context, secret string) (*Key, error) {
	var k Key
	err := p.pool.QueryRow(ctx,
		`SELECT name, monthly_budget_usd, spend_usd FROM keys WHERE secret_hash = $1`,
		hashSecret(secret)).Scan(&k.Name, &k.MonthlyBudgetUSD, &k.SpendUSD)
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
	if _, err := tx.Exec(ctx,
		`INSERT INTO audit_log (key_name, model, input_tokens, output_tokens, cost_usd, latency_ms, status)
         VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		u.KeyName, u.Model, u.InputTokens, u.OutputTokens, u.CostUSD, u.LatencyMS, u.Status); err != nil {
		return fmt.Errorf("insert audit_log: %w", err)
	}
	return tx.Commit(ctx)
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
		`SELECT name, monthly_budget_usd, spend_usd FROM keys ORDER BY name, created_at`)
	if err != nil {
		return nil, fmt.Errorf("query keys: %w", err)
	}
	defer rows.Close()
	var out []KeyInfo
	for rows.Next() {
		var k KeyInfo
		if err := rows.Scan(&k.Name, &k.MonthlyBudgetUSD, &k.SpendUSD); err != nil {
			return nil, fmt.Errorf("scan key: %w", err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (p *Postgres) EnsureKey(ctx context.Context, name, secret string, budgetUSD float64) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO keys (secret_hash, name, monthly_budget_usd) VALUES ($1, $2, $3)
         ON CONFLICT (secret_hash) DO UPDATE SET
           name = EXCLUDED.name,
           monthly_budget_usd = EXCLUDED.monthly_budget_usd`,
		hashSecret(secret), name, budgetUSD)
	if err != nil {
		return fmt.Errorf("ensure key: %w", err)
	}
	return nil
}
