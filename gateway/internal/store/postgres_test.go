package store

import (
	"context"
	"errors"
	"os"
	"testing"
)

// newTestPostgres connects to AGENTOS_TEST_DATABASE_URL, skipping the test
// when it is unset, and truncates the gateway tables for isolation.
func newTestPostgres(t *testing.T) *Postgres {
	t.Helper()
	dsn := os.Getenv("AGENTOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("AGENTOS_TEST_DATABASE_URL not set; skipping postgres store tests")
	}
	ctx := context.Background()
	p, err := NewPostgres(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPostgres: %v", err)
	}
	t.Cleanup(p.Close)
	if _, err := p.pool.Exec(ctx, `TRUNCATE keys, "usage", audit_log, orgs, users`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return p
}

func TestPostgresKeyLifecycle(t *testing.T) {
	p := newTestPostgres(t)
	ctx := context.Background()

	secret, err := p.CreateKey(ctx, "runtime", 25)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	key, err := p.Authenticate(ctx, secret)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if key.Name != "runtime" || key.MonthlyBudgetUSD != 25 || key.SpendUSD != 0 {
		t.Errorf("key = %+v", key)
	}
	if _, err := p.Authenticate(ctx, "agos-bogus"); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("Authenticate bogus err = %v, want ErrInvalidKey", err)
	}

	keys, err := p.Keys(ctx)
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	if len(keys) != 1 || keys[0].Name != "runtime" {
		t.Errorf("keys = %+v", keys)
	}
}

func TestPostgresRecordUsage(t *testing.T) {
	p := newTestPostgres(t)
	ctx := context.Background()

	secret, err := p.CreateKey(ctx, "agent", 10)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	entries := []Usage{
		{KeyName: "agent", Model: "openai/gpt-4o-mini", InputTokens: 1000, OutputTokens: 200, CostUSD: 0.00027, LatencyMS: 90, Status: 200},
		{KeyName: "agent", Model: "openai/gpt-4o-mini", InputTokens: 500, OutputTokens: 100, CostUSD: 0.000135, LatencyMS: 70, Status: 200},
	}
	for _, u := range entries {
		if err := p.RecordUsage(ctx, u); err != nil {
			t.Fatalf("RecordUsage: %v", err)
		}
	}

	key, err := p.Authenticate(ctx, secret)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if !almostEqual(key.SpendUSD, 0.000405) {
		t.Errorf("spend = %v, want 0.000405", key.SpendUSD)
	}

	usage, err := p.Usage(ctx, "")
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	if len(usage) != 1 {
		t.Fatalf("usage rows = %d, want 1", len(usage))
	}
	u := usage[0]
	if u.Name != "agent" || u.Requests != 2 || u.InputTokens != 1500 || u.OutputTokens != 300 {
		t.Errorf("aggregate = %+v", u)
	}

	var auditCount int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM audit_log`).Scan(&auditCount); err != nil {
		t.Fatalf("count audit_log: %v", err)
	}
	if auditCount != 2 {
		t.Errorf("audit_log rows = %d, want 2", auditCount)
	}
}

func TestPostgresAuditListNewestFirstWithKinds(t *testing.T) {
	p := newTestPostgres(t)
	ctx := context.Background()

	if _, err := p.CreateKey(ctx, "agent", 100); err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	if err := p.RecordUsage(ctx, Usage{KeyName: "agent", Model: "m1", InputTokens: 5, Status: 200}); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}
	if err := p.RecordAudit(ctx, Usage{KeyName: "agent", Model: "m2", Status: 400, Kind: KindGuardrailBlock}); err != nil {
		t.Fatalf("RecordAudit: %v", err)
	}
	if err := p.RecordUsage(ctx, Usage{KeyName: "agent", Model: "m3", Status: 200, Kind: KindEmbeddings}); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}

	list, err := p.AuditList(ctx, "", 10)
	if err != nil {
		t.Fatalf("AuditList: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("entries = %d, want 3", len(list))
	}
	if list[0].Model != "m3" || list[1].Model != "m2" || list[2].Model != "m1" {
		t.Errorf("order = %q, %q, %q, want m3, m2, m1", list[0].Model, list[1].Model, list[2].Model)
	}
	if list[0].Kind != KindEmbeddings || list[1].Kind != KindGuardrailBlock || list[2].Kind != KindChat {
		t.Errorf("kinds = %q, %q, %q", list[0].Kind, list[1].Kind, list[2].Kind)
	}
	if list[0].TS.IsZero() {
		t.Error("ts not populated")
	}

	if short, err := p.AuditList(ctx, "", 1); err != nil || len(short) != 1 || short[0].Model != "m3" {
		t.Errorf("limited list = %+v (err %v)", short, err)
	}

	// RecordAudit must not touch spend or usage aggregates.
	usage, err := p.Usage(ctx, "")
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	if len(usage) != 1 || usage[0].Requests != 2 {
		t.Errorf("usage = %+v, want 2 requests", usage)
	}
}

// TestPostgresKindColumnMigration simulates a Phase 1 database (audit_log
// without the kind column, with existing rows) and verifies reconnecting
// adds the column and backfills existing rows as chat.
func TestPostgresKindColumnMigration(t *testing.T) {
	p := newTestPostgres(t)
	ctx := context.Background()

	if _, err := p.pool.Exec(ctx, `ALTER TABLE audit_log DROP COLUMN kind`); err != nil {
		t.Fatalf("drop kind column: %v", err)
	}
	if _, err := p.pool.Exec(ctx,
		`INSERT INTO audit_log (key_name, model, input_tokens, output_tokens, cost_usd, latency_ms, status)
         VALUES ('agent', 'openai/gpt-4o-mini', 10, 5, 0.001, 42, 200)`); err != nil {
		t.Fatalf("insert phase 1 row: %v", err)
	}

	p2, err := NewPostgres(ctx, os.Getenv("AGENTOS_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("NewPostgres over phase 1 schema: %v", err)
	}
	t.Cleanup(p2.Close)

	list, err := p2.AuditList(ctx, "", 10)
	if err != nil {
		t.Fatalf("AuditList: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("entries = %d, want 1", len(list))
	}
	if list[0].Kind != KindChat {
		t.Errorf("pre-existing row kind = %q, want %q", list[0].Kind, KindChat)
	}

	// A second reconnect must also be a no-op (ADD COLUMN IF NOT EXISTS).
	p3, err := NewPostgres(ctx, os.Getenv("AGENTOS_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("NewPostgres re-run: %v", err)
	}
	p3.Close()
}

func TestPostgresEnsureKeyIdempotent(t *testing.T) {
	p := newTestPostgres(t)
	ctx := context.Background()

	if err := p.EnsureKey(ctx, "runtime", "agos-local-dev-runtime", 25); err != nil {
		t.Fatalf("EnsureKey: %v", err)
	}
	if err := p.RecordUsage(ctx, Usage{KeyName: "runtime", Model: "m", CostUSD: 2, Status: 200}); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}
	// Restart: same secret, updated budget — must not error or duplicate.
	if err := p.EnsureKey(ctx, "runtime", "agos-local-dev-runtime", 50); err != nil {
		t.Fatalf("EnsureKey second call: %v", err)
	}

	key, err := p.Authenticate(ctx, "agos-local-dev-runtime")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if key.MonthlyBudgetUSD != 50 {
		t.Errorf("budget = %v, want 50", key.MonthlyBudgetUSD)
	}
	if !almostEqual(key.SpendUSD, 2) {
		t.Errorf("spend = %v, want preserved 2", key.SpendUSD)
	}

	keys, err := p.Keys(ctx)
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("keys = %d, want 1 (no duplicate)", len(keys))
	}
}

// TestPostgresUserActiveExternalIDMigration simulates a Phase 5/6 users table
// lacking the Phase 7 active/external_id columns, with an existing row, and
// verifies reconnecting adds the columns and backfills the row as active.
func TestPostgresUserActiveExternalIDMigration(t *testing.T) {
	p := newTestPostgres(t)
	ctx := context.Background()

	if _, err := p.pool.Exec(ctx, `ALTER TABLE users DROP COLUMN active, DROP COLUMN external_id`); err != nil {
		t.Fatalf("drop phase 7 columns: %v", err)
	}
	if err := p.EnsureOrg(ctx, DefaultOrgID, "default", 0); err != nil {
		t.Fatalf("EnsureOrg: %v", err)
	}
	if _, err := p.pool.Exec(ctx,
		`INSERT INTO users (id, org_id, email, role, token_hash) VALUES ('usr_legacy', $1, 'legacy@acme.test', 'member', 'deadbeef')`,
		DefaultOrgID); err != nil {
		t.Fatalf("insert phase 6 user: %v", err)
	}

	p2, err := NewPostgres(ctx, os.Getenv("AGENTOS_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("NewPostgres over phase 6 schema: %v", err)
	}
	t.Cleanup(p2.Close)

	users, err := p2.Users(ctx, DefaultOrgID)
	if err != nil {
		t.Fatalf("Users: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("users = %d, want 1", len(users))
	}
	if !users[0].Active {
		t.Errorf("migrated user Active = false, want true (backfilled)")
	}
	if users[0].ExternalID != "" {
		t.Errorf("migrated user ExternalID = %q, want empty", users[0].ExternalID)
	}
}

// TestPostgresUsageSecretHashMigration simulates a pre-Phase-8 usage table
// (key_name primary key, no secret_hash/org_id) with an existing aggregate row
// and verifies reconnecting adds the columns, backfills them from the keys
// table by name, re-keys the aggregate on secret_hash, and scopes by org (H4).
func TestPostgresUsageSecretHashMigration(t *testing.T) {
	p := newTestPostgres(t)
	ctx := context.Background()

	if err := p.EnsureOrg(ctx, DefaultOrgID, "default", 0); err != nil {
		t.Fatalf("EnsureOrg: %v", err)
	}
	secret, err := p.CreateKey(ctx, "legacy", 10)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	key, err := p.Authenticate(ctx, secret)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	// Revert the usage table to its pre-Phase-8 shape and add a name-keyed row.
	for _, stmt := range []string{
		`DROP INDEX IF EXISTS usage_secret_hash_idx`,
		`DROP INDEX IF EXISTS usage_org_idx`,
		`ALTER TABLE "usage" DROP COLUMN secret_hash, DROP COLUMN org_id`,
		`ALTER TABLE "usage" ADD PRIMARY KEY (key_name)`,
		`INSERT INTO "usage" (key_name, requests, input_tokens, output_tokens, spend_usd)
             VALUES ('legacy', 5, 100, 50, 1.25)`,
	} {
		if _, err := p.pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("simulate pre-phase-8 usage (%s): %v", stmt, err)
		}
	}

	// Reconnect: the schema migration backfills and re-keys the aggregate.
	p2, err := NewPostgres(ctx, os.Getenv("AGENTOS_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("NewPostgres over pre-phase-8 usage: %v", err)
	}
	t.Cleanup(p2.Close)

	// The row survives, still shows the aggregate, and is now joined by secret_hash.
	all, err := p2.Usage(ctx, "")
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	if len(all) != 1 || all[0].Name != "legacy" || all[0].Requests != 5 || all[0].SpendUSD != 1.25 {
		t.Fatalf("migrated usage = %+v, want legacy row 5 req / 1.25 spend", all)
	}
	// Backfilled org_id makes org scoping work.
	scoped, err := p2.Usage(ctx, DefaultOrgID)
	if err != nil {
		t.Fatalf("scoped Usage: %v", err)
	}
	if len(scoped) != 1 || scoped[0].Requests != 5 {
		t.Errorf("org-scoped usage = %+v, want the migrated legacy row", scoped)
	}
	if other, _ := p2.Usage(ctx, "org_absent"); len(other) != 0 {
		t.Errorf("foreign-org usage = %+v, want empty", other)
	}
	_ = key
}
