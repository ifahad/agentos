package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// DefaultMaxRows is the row cap applied when AGENTOS_CONNECTOR_MAX_ROWS is unset.
const DefaultMaxRows = 200

// Tools holds the shared dependencies of the agentos-sql MCP tool handlers.
type Tools struct {
	pool    *pgxpool.Pool
	maxRows int
}

// New returns a Tools bound to pool. maxRows <= 0 falls back to DefaultMaxRows.
func New(pool *pgxpool.Pool, maxRows int) *Tools {
	if maxRows <= 0 {
		maxRows = DefaultMaxRows
	}
	return &Tools{pool: pool, maxRows: maxRows}
}

// Register adds the list_tables, describe_table and query tools to s.
func (t *Tools) Register(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("list_tables",
		mcp.WithDescription("List all user tables in the connected Postgres database. Returns a JSON array of {schema, name} objects."),
	), t.ListTables)

	s.AddTool(mcp.NewTool("describe_table",
		mcp.WithDescription("Describe the columns of a table. Returns a JSON array of {name, type, nullable} objects."),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description(`Table name, either "schema.table" or a bare table name (schema defaults to "public").`),
		),
	), t.DescribeTable)

	s.AddTool(mcp.NewTool("query",
		mcp.WithDescription("Run a read-only SQL query (single SELECT or WITH statement). Returns JSON {columns, rows, row_count, truncated}. Executed inside a READ ONLY transaction with a row cap."),
		mcp.WithString("sql",
			mcp.Required(),
			mcp.Description("A single SELECT or WITH statement. Writes, DDL and multiple statements are rejected."),
		),
	), t.Query)
}

type tableRef struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
}

// ListTables implements the list_tables tool.
func (t *Tools) ListTables(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	rows, err := t.pool.Query(ctx, `
		SELECT table_schema, table_name
		FROM information_schema.tables
		WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY table_schema, table_name`)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list tables: %v", err)), nil
	}
	defer rows.Close()

	tables := make([]tableRef, 0)
	for rows.Next() {
		var tr tableRef
		if err := rows.Scan(&tr.Schema, &tr.Name); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("list tables: %v", err)), nil
		}
		tables = append(tables, tr)
	}
	if err := rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list tables: %v", err)), nil
	}
	return jsonResult(tables)
}

type columnInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

// splitTableRef parses "schema.table" or a bare "table" (schema defaults to
// public). The parts are only ever used as parameterized query arguments,
// never interpolated into SQL.
func splitTableRef(table string) (schema, name string, err error) {
	parts := strings.Split(strings.TrimSpace(table), ".")
	switch len(parts) {
	case 1:
		schema, name = "public", parts[0]
	case 2:
		schema, name = parts[0], parts[1]
	default:
		return "", "", fmt.Errorf("invalid table reference %q: expected \"table\" or \"schema.table\"", table)
	}
	if schema == "" || name == "" {
		return "", "", fmt.Errorf("invalid table reference %q: empty schema or table name", table)
	}
	return schema, name, nil
}

// DescribeTable implements the describe_table tool. The identifier is
// validated against information_schema via a parameterized query; unvalidated
// input is never interpolated into SQL.
func (t *Tools) DescribeTable(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	table, err := req.RequireString("table")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	schema, name, err := splitTableRef(table)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	rows, err := t.pool.Query(ctx, `
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position`, schema, name)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("describe table: %v", err)), nil
	}
	defer rows.Close()

	cols := make([]columnInfo, 0)
	for rows.Next() {
		var ci columnInfo
		var nullable string
		if err := rows.Scan(&ci.Name, &ci.Type, &nullable); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("describe table: %v", err)), nil
		}
		ci.Nullable = strings.EqualFold(nullable, "YES")
		cols = append(cols, ci)
	}
	if err := rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("describe table: %v", err)), nil
	}
	if len(cols) == 0 {
		return mcp.NewToolResultError(fmt.Sprintf("table %q not found (looked up %s.%s in information_schema)", table, schema, name)), nil
	}
	return jsonResult(cols)
}

type queryResult struct {
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
	RowCount  int      `json:"row_count"`
	Truncated bool     `json:"truncated"`
}

// Query implements the query tool. Read-only enforcement is layered:
//  1. Validate rejects anything that is not a single SELECT/WITH statement
//     free of write/DDL keywords (defense-in-depth, lexical).
//  2. The statement runs inside a READ ONLY transaction
//     (pgx.TxOptions{AccessMode: pgx.ReadOnly}), which is the actual
//     guarantee: Postgres refuses writes in such a transaction even if the
//     lexical check were bypassed.
//  3. Scanning stops after maxRows rows; truncated=true signals the cap.
func (t *Tools) Query(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sql, err := req.RequireString("sql")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := Validate(sql); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rejected: %v", err)), nil
	}

	tx, err := t.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("begin read-only transaction: %v", err)), nil
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, sql)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query: %v", err)), nil
	}
	defer rows.Close()

	res := queryResult{
		Columns: make([]string, 0),
		Rows:    make([][]any, 0),
	}
	for _, fd := range rows.FieldDescriptions() {
		res.Columns = append(res.Columns, fd.Name)
	}

	for rows.Next() {
		if len(res.Rows) >= t.maxRows {
			res.Truncated = true
			break
		}
		vals, err := rows.Values()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query: %v", err)), nil
		}
		row := make([]any, len(vals))
		for i, v := range vals {
			row[i] = jsonSafe(v)
		}
		res.Rows = append(res.Rows, row)
	}
	if !res.Truncated {
		if err := rows.Err(); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query: %v", err)), nil
		}
	}
	res.RowCount = len(res.Rows)
	return jsonResult(res)
}

// jsonSafe converts a pgx row value into something that serializes cleanly to
// JSON: time.Time becomes RFC 3339, []byte becomes a string, values pgx
// already represents as JSON-marshalable Go types (ints, floats, bools,
// strings, pgtype.Numeric, ...) pass through, and anything unmarshalable
// falls back to its fmt.Sprint form.
func jsonSafe(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case time.Time:
		return t.Format(time.RFC3339)
	case []byte:
		return string(t)
	default:
		if _, err := json.Marshal(v); err != nil {
			return fmt.Sprint(v)
		}
		return v
	}
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	out, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("encode result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}
