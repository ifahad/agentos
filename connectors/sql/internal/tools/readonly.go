// Package tools implements the agentos-sql MCP tools and the read-only SQL
// validator used by the query tool.
package tools

import (
	"fmt"
	"strings"
)

// forbiddenKeywords are statement keywords that must never appear anywhere in
// a submitted statement, even inside CTEs (e.g. WITH d AS (DELETE ...) SELECT).
// Matching is done on word tokens outside string literals, quoted identifiers,
// and comments, so identifiers like "updates" are not false positives.
var forbiddenKeywords = map[string]struct{}{
	"DELETE":   {},
	"UPDATE":   {},
	"INSERT":   {},
	"DROP":     {},
	"ALTER":    {},
	"TRUNCATE": {},
	"GRANT":    {},
	"CREATE":   {},
	"COPY":     {},
}

// Validate reports whether sql is a single read-only statement.
//
// It strips line comments (--) and block comments (/* */, with Postgres-style
// nesting), rejects anything containing more than one statement (a ';'
// followed by non-whitespace), requires the first keyword to be SELECT or
// WITH, and scans the whole statement for forbidden write/DDL keywords as
// word tokens outside string literals.
//
// This validator is defense-in-depth only. The real read-only guarantee is
// that every query runs inside a BEGIN ... READ ONLY transaction
// (pgx.TxOptions{AccessMode: pgx.ReadOnly}), so even a statement that slips
// past this check cannot write. A consequence of the lexical scan is that
// some legitimate queries are rejected (e.g. a SELECT mentioning DROP inside
// a dollar-quoted string, which this scanner does not special-case); that
// trade-off is intentional — false rejections are safe, false acceptances
// are still caught by the transaction access mode.
func Validate(sql string) error {
	cleaned, err := stripLiteralsAndComments(sql)
	if err != nil {
		return err
	}

	trimmed := strings.TrimSpace(cleaned)
	if trimmed == "" {
		return fmt.Errorf("empty statement")
	}

	// Single statement only: a ';' followed by any non-whitespace is rejected.
	// String literals and comments were already removed, so a ';' here is a
	// real statement separator.
	if i := strings.IndexByte(cleaned, ';'); i >= 0 {
		if strings.TrimSpace(cleaned[i+1:]) != "" {
			return fmt.Errorf("multiple statements are not allowed")
		}
	}

	first := leadingWord(trimmed)
	switch strings.ToUpper(first) {
	case "SELECT", "WITH":
		// allowed
	default:
		return fmt.Errorf("only SELECT or WITH statements are allowed (got %q)", first)
	}

	for _, w := range wordTokens(cleaned) {
		if _, bad := forbiddenKeywords[strings.ToUpper(w)]; bad {
			return fmt.Errorf("forbidden keyword %s in statement", strings.ToUpper(w))
		}
	}
	return nil
}

// stripLiteralsAndComments returns sql with comments replaced by a single
// space and the contents of string literals / quoted identifiers removed
// (the surrounding quotes are kept as empty ” / "" placeholders).
//
// Handled lexical forms:
//   - line comments: -- to end of line
//   - block comments: /* ... */ with Postgres-style nesting; an unterminated
//     comment (including a nesting attempt like "/* /* */ DROP ...") is an
//     error rather than silently un-commented text
//   - standard string literals: '...' with ” as the escaped quote
//   - E'...' escape strings: backslash escapes the next character
//   - quoted identifiers: "..." with "" as the escaped quote
//
// Dollar-quoted strings ($$...$$) are deliberately not treated as literals:
// their content is scanned as ordinary tokens, which can only cause a safe
// false rejection, never let content hide from the keyword scan.
func stripLiteralsAndComments(sql string) (string, error) {
	var b strings.Builder
	n := len(sql)
	for i := 0; i < n; {
		c := sql[i]

		// Line comment.
		if c == '-' && i+1 < n && sql[i+1] == '-' {
			for i < n && sql[i] != '\n' {
				i++
			}
			b.WriteByte(' ')
			continue
		}

		// Block comment, nested per Postgres rules.
		if c == '/' && i+1 < n && sql[i+1] == '*' {
			depth := 1
			i += 2
			for i < n && depth > 0 {
				switch {
				case sql[i] == '/' && i+1 < n && sql[i+1] == '*':
					depth++
					i += 2
				case sql[i] == '*' && i+1 < n && sql[i+1] == '/':
					depth--
					i += 2
				default:
					i++
				}
			}
			if depth > 0 {
				return "", fmt.Errorf("unterminated block comment")
			}
			b.WriteByte(' ')
			continue
		}

		// String literal. If introduced by a standalone E/e (Postgres escape
		// string), backslash escapes the next character; in standard literals
		// backslash is an ordinary character (standard_conforming_strings=on).
		if c == '\'' {
			backslashEscapes := isEscapeStringIntro(b.String())
			i++
			closed := false
			for i < n {
				switch {
				case backslashEscapes && sql[i] == '\\':
					i += 2
				case sql[i] == '\'':
					if i+1 < n && sql[i+1] == '\'' {
						i += 2 // '' escape, still inside the literal
						continue
					}
					closed = true
					i++
				default:
					i++
				}
				if closed {
					break
				}
			}
			if !closed {
				return "", fmt.Errorf("unterminated string literal")
			}
			b.WriteString("''")
			continue
		}

		// Quoted identifier.
		if c == '"' {
			i++
			closed := false
			for i < n {
				if sql[i] == '"' {
					if i+1 < n && sql[i+1] == '"' {
						i += 2 // "" escape, still inside the identifier
						continue
					}
					closed = true
					i++
					break
				}
				i++
			}
			if !closed {
				return "", fmt.Errorf("unterminated quoted identifier")
			}
			b.WriteString(`""`)
			continue
		}

		b.WriteByte(c)
		i++
	}
	return b.String(), nil
}

// isEscapeStringIntro reports whether the already-emitted text ends with a
// standalone E/e token, meaning the string literal that follows is a
// Postgres escape string (E'...').
func isEscapeStringIntro(emitted string) bool {
	if emitted == "" {
		return false
	}
	last := emitted[len(emitted)-1]
	if last != 'E' && last != 'e' {
		return false
	}
	return len(emitted) == 1 || !isWordByte(emitted[len(emitted)-2])
}

// leadingWord returns the run of word bytes at the start of s. The statement
// must begin with its keyword; anything else (e.g. a leading parenthesis)
// yields an empty word and is rejected by the caller.
func leadingWord(s string) string {
	i := 0
	for i < len(s) && isWordByte(s[i]) {
		i++
	}
	return s[:i]
}

// wordTokens splits s into identifier/keyword-shaped tokens.
func wordTokens(s string) []string {
	var out []string
	n := len(s)
	for i := 0; i < n; {
		if !isWordStart(s[i]) {
			i++
			continue
		}
		j := i + 1
		for j < n && isWordByte(s[j]) {
			j++
		}
		out = append(out, s[i:j])
		i = j
	}
	return out
}

func isWordStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isWordByte(c byte) bool {
	return isWordStart(c) || (c >= '0' && c <= '9')
}
