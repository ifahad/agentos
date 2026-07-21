// Package validate implements the pure command validation used by the
// agentos-ssh run_command tool: a single command whose basename must be on
// the configured allowlist, with shell chaining metacharacters rejected
// outright (the allowlist governs a single command only).
package validate

import (
	"path/filepath"
	"strings"
)

// chainingChars are single characters that let one command chain into or
// feed another (';' separators, '|' pipes — which also covers '||' — '&'
// backgrounding/'&&' chaining, backtick substitution, and newlines/carriage
// returns, which the remote shell treats as command separators).
const chainingChars = ";|&`\n\r"

// ValidateCommand reports whether cmd is a single, allowlisted command.
//
// It rejects, in order:
//  1. empty (or whitespace-only) commands;
//  2. any chaining/substitution metacharacter (';', '&&', '||', '|',
//     backtick, '$(', plus '&' and newlines, which chain just as well)
//     with reason "command chaining not allowed";
//  3. commands whose first whitespace-separated token's basename
//     (filepath.Base, so "/bin/ls" matches "ls") is not on the allowlist,
//     with reason "command not allowed: <base>". An empty allowlist
//     therefore denies everything.
//
// On success it returns (true, ""). The check is purely lexical: arguments
// are not inspected beyond the metacharacter scan.
func ValidateCommand(cmd string, allowlist []string) (ok bool, reason string) {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return false, "empty command"
	}
	if strings.ContainsAny(cmd, chainingChars) || strings.Contains(cmd, "$(") {
		return false, "command chaining not allowed"
	}
	base := filepath.Base(strings.Fields(trimmed)[0])
	for _, allowed := range allowlist {
		if base == allowed {
			return true, ""
		}
	}
	return false, "command not allowed: " + base
}
