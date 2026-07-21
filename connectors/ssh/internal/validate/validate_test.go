package validate

import "testing"

func TestValidateCommand(t *testing.T) {
	allow := []string{"ls", "cat", "grep", "df", "uptime", "head", "tail", "wc", "stat", "hostname", "uname", "free", "ps", "who"}

	tests := []struct {
		name       string
		cmd        string
		allowlist  []string
		wantOK     bool
		wantReason string
	}{
		// --- genuinely-safe commands still allowed ---
		{"allowed simple command", "ls", allow, true, ""},
		{"allowed command with args", "ls -la /var/log", allow, true, ""},
		{"absolute path basename match", "/bin/ls -l", allow, true, ""},
		{"allowed with surrounding whitespace", "  df -h  ", allow, true, ""},
		{"cat with file arg", "cat /etc/hostname", allow, true, ""},
		{"cat file", "cat file", allow, true, ""},
		{"uptime", "uptime", allow, true, ""},
		{"df -h", "df -h", allow, true, ""},
		{"grep is not exec-capable, stays allowed", "grep foo bar", allow, true, ""},

		// --- command chaining / substitution (now "disallowed shell character") ---
		{"semicolon chaining", "ls; rm -rf /", allow, false, "disallowed shell character"},
		{"pipe chaining", "ls | grep x", allow, false, "disallowed shell character"},
		{"or chaining", "ls || reboot", allow, false, "disallowed shell character"},
		{"and chaining", "ls && reboot", allow, false, "disallowed shell character"},
		{"command substitution dollar-paren", "ls $(whoami)", allow, false, "disallowed shell character"},
		{"command substitution bare $(id)", "$(id)", allow, false, "disallowed shell character"},
		{"backtick substitution", "ls `whoami`", allow, false, "disallowed shell character"},
		{"backtick alone", "cat `id`", allow, false, "disallowed shell character"},
		{"background ampersand", "ls & reboot", allow, false, "disallowed shell character"},
		{"newline separator", "ls\nreboot", allow, false, "disallowed shell character"},
		{"metachar in argument", "grep foo file;", allow, false, "disallowed shell character"},

		// --- redirection (MEDIUM: >/< arbitrary file read/write) ---
		{"redirect stdout to passwd", "cat x > /etc/passwd", allow, false, "disallowed shell character"},
		{"redirect append", "cat x >> /root/.bashrc", allow, false, "disallowed shell character"},
		{"redirect stdin", "find / < y", allow, false, "disallowed shell character"},

		// --- globbing / expansion / grouping ---
		{"glob star", "cat *", allow, false, "disallowed shell character"},
		{"glob question", "cat file?", allow, false, "disallowed shell character"},
		{"tilde expansion", "cat ~/.ssh/id_rsa", allow, false, "disallowed shell character"},
		{"history expansion bang", "cat !x", allow, false, "disallowed shell character"},
		{"brace expansion", "cat {a,b}", allow, false, "disallowed shell character"},
		{"bracket glob", "cat file[0-9]", allow, false, "disallowed shell character"},
		{"backslash escape", "cat foo\\;bar", allow, false, "disallowed shell character"},

		// --- LOLBins from the assessment: rejected as exec-capable or metachar ---
		{"find -exec breakout (grouping chars)", "find / -maxdepth 0 -exec id {} +", allow, false, "disallowed shell character"},
		{"find plain still denied by deny-list", "find /etc -name passwd", allow, false, "command not permitted: find can execute arbitrary code"},
		{"awk system() (grouping chars)", "awk 'BEGIN{system(\"id\")}'", allow, false, "disallowed shell character"},
		{"awk plain denied by deny-list", "awk /root/x", allow, false, "command not permitted: awk can execute arbitrary code"},
		{"gawk denied", "gawk /root/x", allow, false, "command not permitted: gawk can execute arbitrary code"},
		{"tar --to-command", "tar cf /dev/null --to-command=id x", allow, false, "command not permitted: tar can execute arbitrary code"},
		{"git core.pager", "git -c core.pager=id log", allow, false, "command not permitted: git can execute arbitrary code"},
		{"xargs breakout", "xargs -a /etc/passwd id", allow, false, "command not permitted: xargs can execute arbitrary code"},
		{"env exec", "env id", allow, false, "command not permitted: env can execute arbitrary code"},
		{"perl one-liner denied by deny-list", "perl -MPOSIX x", allow, false, "command not permitted: perl can execute arbitrary code"},
		{"python denied", "python /tmp/x", allow, false, "command not permitted: python can execute arbitrary code"},
		{"bash denied", "bash script", allow, false, "command not permitted: bash can execute arbitrary code"},
		{"less pager escape denied", "less /etc/passwd", allow, false, "command not permitted: less can execute arbitrary code"},
		{"vim denied", "vim /etc/passwd", allow, false, "command not permitted: vim can execute arbitrary code"},
		{"absolute-path deny-list still denied", "/usr/bin/find /etc", allow, false, "command not permitted: find can execute arbitrary code"},
		{"deny-list beats allowlist", "find /", []string{"find", "ls"}, false, "command not permitted: find can execute arbitrary code"},

		// --- defensive flag screen (surviving/allowlisted commands) ---
		{"disallowed -exec flag on allowlisted cmd", "ls -exec id", allow, false, "disallowed command flag"},
		{"disallowed --to-command flag", "ls --to-command foo", allow, false, "disallowed command flag"},
		{"disallowed --use-compress-program", "ls --use-compress-program id", allow, false, "disallowed command flag"},
		{"disallowed -execdir flag", "ls -execdir id", allow, false, "disallowed command flag"},
		{"disallowed -e token", "cat -e file", allow, false, "disallowed command flag"},

		// --- empty ---
		{"empty command", "", allow, false, "empty command"},
		{"whitespace only", "  \t ", allow, false, "empty command"},

		// --- allowlist gate ---
		{"empty allowlist denies everything", "ls", nil, false, "command not allowed: ls"},
		{"empty non-nil allowlist denies", "uptime", []string{}, false, "command not allowed: uptime"},
		{"command not in list", "reboot now", allow, false, "command not allowed: reboot"},
		{"absolute path not in list", "/sbin/shutdown -h", allow, false, "command not allowed: shutdown"},
		{"allowlist match is exact", "lsblk", allow, false, "command not allowed: lsblk"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, reason := ValidateCommand(tt.cmd, tt.allowlist)
			if ok != tt.wantOK || reason != tt.wantReason {
				t.Fatalf("ValidateCommand(%q, %v) = (%t, %q), want (%t, %q)",
					tt.cmd, tt.allowlist, ok, reason, tt.wantOK, tt.wantReason)
			}
		})
	}
}
