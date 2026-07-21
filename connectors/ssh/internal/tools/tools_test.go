package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/ifahad/agentos/connectors/ssh/internal/sshclient"
)

type fakeRunner struct {
	calls  []string
	result sshclient.Result
	err    error
}

func (f *fakeRunner) Run(_ context.Context, command string) (sshclient.Result, error) {
	f.calls = append(f.calls, command)
	return f.result, f.err
}

func callReq(args map[string]any) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Arguments = args
	return req
}

// resultText returns the text payload of res along with whether res is an
// error result.
func resultText(t *testing.T, res *mcp.CallToolResult) (text string, isErr bool) {
	t.Helper()
	if len(res.Content) != 1 {
		t.Fatalf("result has %d content items, want 1", len(res.Content))
	}
	tc, ok := mcp.AsTextContent(res.Content[0])
	if !ok {
		t.Fatalf("result content is %T, want text", res.Content[0])
	}
	return tc.Text, res.IsError
}

func TestRunCommand(t *testing.T) {
	allow := []string{"ls", "uptime"}

	tests := []struct {
		name      string
		args      map[string]any
		runResult sshclient.Result
		runErr    error
		wantText  string // substring of the expected result text
		wantErr   bool
		wantRuns  int
	}{
		{
			name:      "allowed command executes",
			args:      map[string]any{"command": "ls -la"},
			runResult: sshclient.Result{ExitCode: 0, Stdout: "total 0\n"},
			wantText:  `"exit_code":0`,
			wantRuns:  1,
		},
		{
			name:      "non-zero exit is a normal result",
			args:      map[string]any{"command": "ls /nope"},
			runResult: sshclient.Result{ExitCode: 2, Stderr: "no such file\n"},
			wantText:  `"exit_code":2`,
			wantRuns:  1,
		},
		{
			name:     "chaining rejected before execution",
			args:     map[string]any{"command": "ls; rm -rf /"},
			wantText: "rejected: disallowed shell character",
			wantErr:  true,
			wantRuns: 0,
		},
		{
			name:     "exec-capable binary rejected before execution",
			args:     map[string]any{"command": "find / -name x"},
			wantText: "rejected: command not permitted: find can execute arbitrary code",
			wantErr:  true,
			wantRuns: 0,
		},
		{
			name:     "disallowed command rejected before execution",
			args:     map[string]any{"command": "reboot"},
			wantText: "rejected: command not allowed: reboot",
			wantErr:  true,
			wantRuns: 0,
		},
		{
			name:     "missing command argument",
			args:     map[string]any{},
			wantText: "command",
			wantErr:  true,
			wantRuns: 0,
		},
		{
			name:     "runner error surfaces as error result",
			args:     map[string]any{"command": "uptime"},
			runErr:   errors.New("dial 10.0.0.1:22: connection refused"),
			wantText: "run command: dial",
			wantErr:  true,
			wantRuns: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{result: tt.runResult, err: tt.runErr}
			tl := New(runner, allow)

			res, err := tl.RunCommand(context.Background(), callReq(tt.args))
			if err != nil {
				t.Fatalf("RunCommand returned transport error: %v", err)
			}
			text, isErr := resultText(t, res)
			if isErr != tt.wantErr {
				t.Fatalf("RunCommand IsError = %t, want %t (text %q)", isErr, tt.wantErr, text)
			}
			if !strings.Contains(text, tt.wantText) {
				t.Fatalf("RunCommand text = %q, want substring %q", text, tt.wantText)
			}
			if len(runner.calls) != tt.wantRuns {
				t.Fatalf("runner ran %d times (%v), want %d", len(runner.calls), runner.calls, tt.wantRuns)
			}
		})
	}
}

func TestRunCommandResultShape(t *testing.T) {
	runner := &fakeRunner{result: sshclient.Result{
		ExitCode: -1, Stdout: "partial", Stderr: "killed", Truncated: true, TimedOut: true,
	}}
	tl := New(runner, []string{"sleep"})

	res, err := tl.RunCommand(context.Background(), callReq(map[string]any{"command": "sleep 99"}))
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	text, isErr := resultText(t, res)
	if isErr {
		t.Fatalf("RunCommand unexpectedly errored: %q", text)
	}
	want := `{"exit_code":-1,"stdout":"partial","stderr":"killed","truncated":true,"timed_out":true}`
	if text != want {
		t.Fatalf("RunCommand text = %q, want %q", text, want)
	}
}

func TestListAllowed(t *testing.T) {
	tests := []struct {
		name  string
		allow []string
		want  string
	}{
		{"configured allowlist", []string{"ls", "cat", "df"}, `["ls","cat","df"]`},
		{"empty allowlist serializes as empty array", nil, `[]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := New(&fakeRunner{}, tt.allow)
			res, err := tl.ListAllowed(context.Background(), callReq(nil))
			if err != nil {
				t.Fatalf("ListAllowed: %v", err)
			}
			text, isErr := resultText(t, res)
			if isErr || text != tt.want {
				t.Fatalf("ListAllowed = (%q, err=%t), want (%q, false)", text, isErr, tt.want)
			}
		})
	}
}
