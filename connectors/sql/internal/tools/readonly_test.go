package tools

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		wantErr string // substring of the expected error; "" means accepted
	}{
		{"plain select", "SELECT * FROM customers", ""},
		{"with cte select", "WITH top AS (SELECT * FROM orders) SELECT * FROM top", ""},
		{"lowercase select", "select id from customers", ""},
		{"lowercase with", "with t as (select 1) select * from t", ""},
		{"leading whitespace", "   \n\t SELECT 1", ""},
		{"leading line comment", "-- cmt\nSELECT 1", ""},
		{"leading block comment", "/* x */ SELECT 1", ""},
		{"trailing semicolon only", "SELECT 1;", ""},
		{"trailing semicolon and whitespace", "SELECT 1; \n\t", ""},
		{"forbidden word inside string literal", "SELECT 'DROP TABLE x' AS note", ""},
		{"forbidden word inside quoted identifier", `SELECT "drop" FROM t`, ""},
		{"identifier containing forbidden prefix", "SELECT * FROM updates", ""},
		{"escaped quote in literal", "SELECT 'it''s fine' AS s", ""},

		{"second statement after semicolon", "SELECT 1; DROP TABLE x", "multiple statements"},
		{"cte delete", "WITH d AS (DELETE FROM t RETURNING *) SELECT * FROM d", "DELETE"},
		{"cte insert", "WITH i AS (INSERT INTO t VALUES (1) RETURNING *) SELECT * FROM i", "INSERT"},
		{"cte update", "with u as (update t set a=1 returning *) select * from u", "UPDATE"},
		{"empty", "", "empty"},
		{"whitespace only", "   \n\t ", "empty"},
		{"comment only", "-- just a comment", "empty"},
		{"explain not allowed", "EXPLAIN SELECT 1", "only SELECT or WITH"},
		{"delete statement", "DELETE FROM t", "only SELECT or WITH"},
		{"insert statement", "INSERT INTO t VALUES (1)", "only SELECT or WITH"},
		{"copy statement", "COPY t TO '/tmp/x'", "only SELECT or WITH"},
		{"vacuum statement", "VACUUM t", "only SELECT or WITH"},
		{"leading parenthesis", "(SELECT 1)", "only SELECT or WITH"},
		{"select for update smuggle via semicolon", "SELECT 1;DROP TABLE x", "multiple statements"},
		{"comment does not hide second statement", "SELECT 1; -- x\nDROP TABLE x", "multiple statements"},
		{"block comment injection prefix", "/* harmless */ DROP TABLE x", "only SELECT or WITH"},
		{"unterminated block comment", "/* SELECT 1", "unterminated block comment"},
		{"nested comment attempt cannot hide code", "/* /* */ SELECT 1", "unterminated block comment"},
		{"nested comment fully closed", "/* /* inner */ outer */ SELECT 1", ""},
		{"unterminated string", "SELECT 'oops", "unterminated string literal"},
		{"escape string cannot smuggle statement", `SELECT E'\''; DROP TABLE x`, "multiple statements"},
		{"escape string leaving dangling quote", `SELECT E'\'' ; DROP TABLE x; SELECT '`, "unterminated string literal"},
		{"forbidden word outside literal", "SELECT * FROM t WHERE a = 1 GRANT", "GRANT"},
		{"create in tail", "SELECT 1 CREATE TABLE x (id int)", "CREATE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.sql)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate(%q) = %v, want accepted", tt.sql, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate(%q) accepted, want error containing %q", tt.sql, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate(%q) = %v, want error containing %q", tt.sql, err, tt.wantErr)
			}
		})
	}
}

func TestStripLiteralsAndComments(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"line comment", "SELECT 1 -- tail", "SELECT 1  "},
		{"block comment", "SELECT /* c */ 1", "SELECT   1"},
		{"nested block comment", "SELECT /* a /* b */ c */ 1", "SELECT   1"},
		{"string literal emptied", "SELECT 'DROP' FROM t", "SELECT '' FROM t"},
		{"escaped quote literal", "SELECT 'a''b'", "SELECT ''"},
		{"escape string with backslash quote", `SELECT E'\''`, "SELECT E''"},
		{"quoted identifier emptied", `SELECT "weird name" FROM t`, `SELECT "" FROM t`},
		{"comment marker inside string kept as literal", "SELECT '--not a comment'", "SELECT ''"},
		{"semicolon inside string is not a separator", "SELECT 'a;b'", "SELECT ''"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stripLiteralsAndComments(tt.in)
			if err != nil {
				t.Fatalf("stripLiteralsAndComments(%q) error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("stripLiteralsAndComments(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
