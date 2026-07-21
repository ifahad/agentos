package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
)

func callReq(args map[string]any) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Arguments = args
	return req
}

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(res.Content))
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	return tc.Text
}

// --- Pure-logic tests (no database required) ---

func TestSplitTableRef(t *testing.T) {
	tests := []struct {
		in      string
		schema  string
		name    string
		wantErr bool
	}{
		{"customers", "public", "customers", false},
		{"sales.orders", "sales", "orders", false},
		{"  customers  ", "public", "customers", false},
		{"a.b.c", "", "", true},
		{"", "", "", true},
		{".", "", "", true},
		{"public.", "", "", true},
		{".customers", "", "", true},
	}
	for _, tt := range tests {
		schema, name, err := splitTableRef(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("splitTableRef(%q) = %q,%q, want error", tt.in, schema, name)
			}
			continue
		}
		if err != nil {
			t.Errorf("splitTableRef(%q) error: %v", tt.in, err)
			continue
		}
		if schema != tt.schema || name != tt.name {
			t.Errorf("splitTableRef(%q) = %q,%q, want %q,%q", tt.in, schema, name, tt.schema, tt.name)
		}
	}
}

func TestJSONSafe(t *testing.T) {
	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name string
		in   any
		want any
	}{
		{"nil", nil, nil},
		{"time RFC3339", ts, "2024-01-02T03:04:05Z"},
		{"bytes to string", []byte("abc"), "abc"},
		{"int passthrough", int64(42), int64(42)},
		{"string passthrough", "x", "x"},
		{"bool passthrough", true, true},
		{"float passthrough", 1.5, 1.5},
		{"unmarshalable falls back to string", make(chan int), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jsonSafe(tt.in)
			if tt.name == "unmarshalable falls back to string" {
				if _, ok := got.(string); !ok {
					t.Fatalf("jsonSafe(chan) = %T, want string fallback", got)
				}
				return
			}
			if got != tt.want {
				t.Fatalf("jsonSafe(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestQueryRejectsInvalidSQLWithoutDB proves the validator runs before any
// database access: the pool is nil, so touching it would panic.
func TestQueryRejectsInvalidSQLWithoutDB(t *testing.T) {
	tl := New(nil, 10)
	tests := []struct {
		name string
		sql  string
	}{
		{"write statement", "DROP TABLE customers"},
		{"multi statement", "SELECT 1; DELETE FROM t"},
		{"cte write", "WITH d AS (DELETE FROM t RETURNING *) SELECT * FROM d"},
		{"empty", "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tl.Query(context.Background(), callReq(map[string]any{"sql": tt.sql}))
			if err != nil {
				t.Fatalf("handler error: %v", err)
			}
			if !res.IsError {
				t.Fatalf("expected tool error for %q, got success", tt.sql)
			}
			if !strings.Contains(resultText(t, res), "rejected") {
				t.Fatalf("expected rejection message, got %q", resultText(t, res))
			}
		})
	}
}

func TestQueryRequiresSQLArgument(t *testing.T) {
	tl := New(nil, 10)
	res, err := tl.Query(context.Background(), callReq(map[string]any{}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected tool error for missing sql argument")
	}
}

func TestDescribeTableRejectsBadRefWithoutDB(t *testing.T) {
	tl := New(nil, 10)
	for _, bad := range []string{"a.b.c", "", "public."} {
		res, err := tl.DescribeTable(context.Background(), callReq(map[string]any{"table": bad}))
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if !res.IsError {
			t.Fatalf("expected tool error for table ref %q", bad)
		}
	}
}

// --- Database-backed tests (require AGENTOS_TEST_DATABASE_URL) ---

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("AGENTOS_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("AGENTOS_TEST_DATABASE_URL not set; skipping database-backed test")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func setupFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS agentos_sql_connector_test (
			id integer NOT NULL,
			note text,
			created_at timestamptz NOT NULL DEFAULT now()
		)`)
	if err != nil {
		t.Fatalf("create fixture table: %v", err)
	}
	_, err = pool.Exec(ctx, `CREATE SEQUENCE IF NOT EXISTS agentos_sql_connector_test_seq`)
	if err != nil {
		t.Fatalf("create fixture sequence: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DROP TABLE IF EXISTS agentos_sql_connector_test`)
		_, _ = pool.Exec(ctx, `DROP SEQUENCE IF EXISTS agentos_sql_connector_test_seq`)
	})
}

func TestListTablesDB(t *testing.T) {
	pool := testPool(t)
	setupFixture(t, pool)
	tl := New(pool, 10)

	res, err := tl.ListTables(context.Background(), callReq(nil))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool error: %s", resultText(t, res))
	}
	var tables []tableRef
	if err := json.Unmarshal([]byte(resultText(t, res)), &tables); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	found := false
	for _, tr := range tables {
		if tr.Schema == "information_schema" || tr.Schema == "pg_catalog" {
			t.Fatalf("system schema %q leaked into list_tables", tr.Schema)
		}
		if tr.Schema == "public" && tr.Name == "agentos_sql_connector_test" {
			found = true
		}
	}
	if !found {
		t.Fatal("fixture table not present in list_tables output")
	}
}

func TestDescribeTableDB(t *testing.T) {
	pool := testPool(t)
	setupFixture(t, pool)
	tl := New(pool, 10)
	ctx := context.Background()

	for _, ref := range []string{"agentos_sql_connector_test", "public.agentos_sql_connector_test"} {
		res, err := tl.DescribeTable(ctx, callReq(map[string]any{"table": ref}))
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if res.IsError {
			t.Fatalf("tool error for %q: %s", ref, resultText(t, res))
		}
		var cols []columnInfo
		if err := json.Unmarshal([]byte(resultText(t, res)), &cols); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		want := []columnInfo{
			{Name: "id", Type: "integer", Nullable: false},
			{Name: "note", Type: "text", Nullable: true},
			{Name: "created_at", Type: "timestamp with time zone", Nullable: false},
		}
		if len(cols) != len(want) {
			t.Fatalf("describe %q: got %d columns, want %d: %+v", ref, len(cols), len(want), cols)
		}
		for i := range want {
			if cols[i] != want[i] {
				t.Errorf("describe %q column %d = %+v, want %+v", ref, i, cols[i], want[i])
			}
		}
	}

	res, err := tl.DescribeTable(ctx, callReq(map[string]any{"table": "no_such_table_xyz"}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !res.IsError || !strings.Contains(resultText(t, res), "not found") {
		t.Fatalf("expected not-found tool error, got %q", resultText(t, res))
	}
}

func TestQueryDB(t *testing.T) {
	pool := testPool(t)
	setupFixture(t, pool)
	ctx := context.Background()

	t.Run("basic select", func(t *testing.T) {
		tl := New(pool, 10)
		res, err := tl.Query(ctx, callReq(map[string]any{
			"sql": "SELECT g AS n, 'x' || g::text AS label FROM generate_series(1, 3) g ORDER BY g",
		}))
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if res.IsError {
			t.Fatalf("tool error: %s", resultText(t, res))
		}
		var qr queryResult
		if err := json.Unmarshal([]byte(resultText(t, res)), &qr); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if fmt.Sprint(qr.Columns) != "[n label]" {
			t.Errorf("columns = %v", qr.Columns)
		}
		if qr.RowCount != 3 || len(qr.Rows) != 3 || qr.Truncated {
			t.Errorf("got row_count=%d rows=%d truncated=%v, want 3/3/false", qr.RowCount, len(qr.Rows), qr.Truncated)
		}
		if qr.Rows[0][1] != "x1" {
			t.Errorf("rows[0][1] = %v, want x1", qr.Rows[0][1])
		}
	})

	t.Run("row cap sets truncated", func(t *testing.T) {
		tl := New(pool, 5)
		res, err := tl.Query(ctx, callReq(map[string]any{
			"sql": "SELECT g FROM generate_series(1, 100) g",
		}))
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if res.IsError {
			t.Fatalf("tool error: %s", resultText(t, res))
		}
		var qr queryResult
		if err := json.Unmarshal([]byte(resultText(t, res)), &qr); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if qr.RowCount != 5 || len(qr.Rows) != 5 || !qr.Truncated {
			t.Errorf("got row_count=%d rows=%d truncated=%v, want 5/5/true", qr.RowCount, len(qr.Rows), qr.Truncated)
		}
	})

	t.Run("timestamps and bytea serialize as strings", func(t *testing.T) {
		tl := New(pool, 10)
		res, err := tl.Query(ctx, callReq(map[string]any{
			"sql": "SELECT TIMESTAMPTZ '2024-01-02T03:04:05Z' AS ts, 'abc'::bytea AS b",
		}))
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if res.IsError {
			t.Fatalf("tool error: %s", resultText(t, res))
		}
		var qr queryResult
		if err := json.Unmarshal([]byte(resultText(t, res)), &qr); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		tsStr, ok := qr.Rows[0][0].(string)
		if !ok {
			t.Fatalf("ts serialized as %T, want string", qr.Rows[0][0])
		}
		parsed, err := time.Parse(time.RFC3339, tsStr)
		if err != nil {
			t.Fatalf("ts %q is not RFC3339: %v", tsStr, err)
		}
		if !parsed.Equal(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)) {
			t.Errorf("ts = %v", parsed)
		}
		if qr.Rows[0][1] != "abc" {
			t.Errorf("bytea = %v, want abc", qr.Rows[0][1])
		}
	})

	// setval() writes sequence state but passes the lexical validator (it is
	// a SELECT). The READ ONLY transaction must block it — this is the
	// defense-in-depth layer the validator documentation points at.
	t.Run("read-only transaction blocks writes that pass the validator", func(t *testing.T) {
		tl := New(pool, 10)
		res, err := tl.Query(ctx, callReq(map[string]any{
			"sql": "SELECT setval('agentos_sql_connector_test_seq', 42)",
		}))
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if !res.IsError {
			t.Fatal("expected read-only transaction to reject setval()")
		}
		if !strings.Contains(resultText(t, res), "read-only") {
			t.Errorf("error should mention read-only transaction, got %q", resultText(t, res))
		}
	})
}
