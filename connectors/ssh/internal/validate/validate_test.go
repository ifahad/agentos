package validate

import "testing"

func TestValidateCommand(t *testing.T) {
	allow := []string{"ls", "cat", "grep", "df", "uptime"}

	tests := []struct {
		name       string
		cmd        string
		allowlist  []string
		wantOK     bool
		wantReason string
	}{
		{"allowed simple command", "ls", allow, true, ""},
		{"allowed command with args", "ls -la /var/log", allow, true, ""},
		{"absolute path basename match", "/bin/ls -l", allow, true, ""},
		{"allowed with surrounding whitespace", "  df -h  ", allow, true, ""},
		{"cat with file arg", "cat /etc/hostname", allow, true, ""},

		{"semicolon chaining", "ls; rm -rf /", allow, false, "command chaining not allowed"},
		{"pipe chaining", "ls | grep x", allow, false, "command chaining not allowed"},
		{"or chaining", "ls || reboot", allow, false, "command chaining not allowed"},
		{"and chaining", "ls && reboot", allow, false, "command chaining not allowed"},
		{"command substitution", "ls $(whoami)", allow, false, "command chaining not allowed"},
		{"backtick substitution", "ls `whoami`", allow, false, "command chaining not allowed"},
		{"background ampersand", "ls & reboot", allow, false, "command chaining not allowed"},
		{"newline separator", "ls\nreboot", allow, false, "command chaining not allowed"},
		{"metachar in argument", "grep foo file;", allow, false, "command chaining not allowed"},

		{"empty command", "", allow, false, "empty command"},
		{"whitespace only", "  \t ", allow, false, "empty command"},

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
