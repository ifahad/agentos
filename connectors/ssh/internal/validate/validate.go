// Package validate implements the pure command validation used by the
// agentos-ssh run_command tool: a single command whose basename must be on
// the configured allowlist, with shell-dangerous characters, exec-capable
// "LOLBin" binaries, and known argument-injection flags rejected outright.
//
// SECURITY NOTE — this is defense-in-depth, not a hard boundary. Commands are
// executed through the remote login shell, and basename allowlisting through a
// shell is NOT a perfect security boundary: any allowlisted binary that can be
// coerced into running other programs (via -exec flags, embedded scripts,
// pager/editor escapes, etc.) can break out. The correct fix is a server-side
// forced command (an authorized_keys `command="..."` wrapper or a restricted
// shell on the target host) that never invokes a general shell at all. The
// layered checks here — a shell-metacharacter screen, a deny-list of
// exec-capable binaries applied even to allowlisted names, and an
// argument-flag screen — substantially reduce the practical attack surface but
// do not replace that server-side control.
package validate

import (
	"path/filepath"
	"strings"
)

// blockedChars are single characters that are rejected anywhere in the command
// because a remote shell gives them special meaning. This covers:
//   - command chaining/piping/substitution: ';' '|' (also '||') '&' (also
//     '&&') backtick, and newline/carriage-return separators;
//   - redirection: '>' '<' (arbitrary file read/write);
//   - globbing/pathname expansion: '*' '?' '[' ']';
//   - brace expansion and command grouping: '{' '}' '(' ')';
//   - tilde (home) expansion and history expansion: '~' '!';
//   - the escape character '\', which defeats naive lexical screening.
const blockedChars = ";|&`\n\r><*?~!{}()[]\\"

// deniedBinaries are basenames that are REJECTED even when present on the
// allowlist: each can execute arbitrary code or spawn other programs (directly
// via -exec/script flags, or via editor/pager/shell escapes), which makes any
// single-command allowlist meaningless once one of them is reachable.
var deniedBinaries = map[string]struct{}{
	"find": {}, "awk": {}, "gawk": {}, "mawk": {}, "xargs": {}, "env": {},
	"tar": {}, "git": {}, "sed": {}, "perl": {}, "python": {}, "python3": {},
	"ruby": {}, "node": {}, "sh": {}, "bash": {}, "zsh": {}, "ksh": {},
	"csh": {}, "tcsh": {}, "vi": {}, "vim": {}, "nano": {}, "emacs": {},
	"less": {}, "more": {}, "man": {}, "nice": {}, "timeout": {}, "watch": {},
	"nohup": {}, "setsid": {}, "make": {}, "ssh": {}, "scp": {}, "sftp": {},
	"rsync": {}, "socat": {}, "nc": {}, "ncat": {}, "netcat": {}, "dd": {},
	"tee": {}, "eval": {}, "exec": {}, "source": {}, "gdb": {}, "strace": {},
	"ltrace": {}, "lua": {}, "php": {}, "expect": {}, "ed": {}, "ex": {},
	"pico": {}, "mail": {}, "mutt": {}, "crontab": {}, "at": {}, "screen": {},
	"tmux": {},
}

// disallowedFlags are standalone (whitespace-separated) argument tokens that
// turn an otherwise-benign command into an arbitrary-code-execution primitive.
// They are rejected defensively even though the deny-list already removes the
// binaries that best-known-use them.
var disallowedFlags = map[string]struct{}{
	"-exec":                  {},
	"-execdir":               {},
	"--to-command":           {},
	"--use-compress-program": {},
	"-e":                     {},
}

// ValidateCommand reports whether cmd is a single, allowlisted command safe to
// run through a remote shell.
//
// It rejects, in order:
//  1. empty (or whitespace-only) commands, reason "empty command";
//  2. any shell-dangerous character — chaining, substitution, redirection,
//     globbing, brace/grouping, tilde/history expansion, or backslash escape
//     (see blockedChars, plus explicit "$(" detection) — reason
//     "disallowed shell character";
//  3. a first-token basename on the built-in deny-list of exec-capable
//     binaries, reason "command not permitted: <base> can execute arbitrary
//     code" (applied BEFORE the allowlist, so allowlisting one of them has no
//     effect);
//  4. a standalone argument token that is a known arbitrary-exec flag
//     (-exec, -execdir, --to-command, --use-compress-program, -e), reason
//     "disallowed command flag";
//  5. commands whose first whitespace-separated token's basename
//     (filepath.Base, so "/bin/ls" matches "ls") is not on the allowlist,
//     reason "command not allowed: <base>". An empty allowlist denies
//     everything.
//
// On success it returns (true, ""). The check is purely lexical.
func ValidateCommand(cmd string, allowlist []string) (ok bool, reason string) {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return false, "empty command"
	}
	if strings.ContainsAny(cmd, blockedChars) || strings.Contains(cmd, "$(") {
		return false, "disallowed shell character"
	}

	fields := strings.Fields(trimmed)
	base := filepath.Base(fields[0])

	// Deny-list is checked before the allowlist: exec-capable binaries are
	// rejected even if an operator mistakenly allowlists them.
	if _, denied := deniedBinaries[base]; denied {
		return false, "command not permitted: " + base + " can execute arbitrary code"
	}

	// Defensive: reject known argument-injection flags on any surviving
	// command, regardless of the allowlist.
	for _, tok := range fields[1:] {
		if _, bad := disallowedFlags[tok]; bad {
			return false, "disallowed command flag"
		}
	}

	for _, allowed := range allowlist {
		if base == allowed {
			return true, ""
		}
	}
	return false, "command not allowed: " + base
}
