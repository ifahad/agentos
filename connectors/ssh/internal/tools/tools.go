// Package tools implements the agentos-ssh MCP tools: run_command (validated
// against the command allowlist before anything touches the wire) and
// list_allowed.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/ifahad/agentos/connectors/ssh/internal/sshclient"
	"github.com/ifahad/agentos/connectors/ssh/internal/validate"
)

// Runner executes one already-validated command on the remote host. It is
// satisfied by *sshclient.Client and faked in tests.
type Runner interface {
	Run(ctx context.Context, command string) (sshclient.Result, error)
}

// Tools holds the shared dependencies of the agentos-ssh MCP tool handlers.
type Tools struct {
	runner Runner
	allow  []string
}

// New returns a Tools running commands through runner, gated by allowlist.
func New(runner Runner, allowlist []string) *Tools {
	if allowlist == nil {
		allowlist = []string{}
	}
	return &Tools{runner: runner, allow: allowlist}
}

// Register adds the run_command and list_allowed tools to s.
func (t *Tools) Register(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("run_command",
		mcp.WithDescription("Run a single command on the configured SSH host. The command's basename must be on the allowlist (see list_allowed). Rejected: shell-dangerous characters (chaining ; && || | backtick $(); redirection > <; globbing * ? [ ]; brace/group { } ( ); ~ ! and backslash), exec-capable binaries (find, awk, tar, git, sed, perl, python, bash, ... — even if allowlisted), and arbitrary-exec flags (-exec, --to-command, ...). Returns JSON {exit_code, stdout, stderr, truncated, timed_out}."),
		mcp.WithString("command",
			mcp.Required(),
			mcp.Description(`A single command with arguments, e.g. "df -h" or "/bin/ls /var/log". No pipes, chaining, or substitution.`),
		),
	), t.RunCommand)

	s.AddTool(mcp.NewTool("list_allowed",
		mcp.WithDescription("List the command basenames allowed by run_command. Returns a JSON array of strings; an empty array means every command is denied."),
	), t.ListAllowed)
}

// RunCommand implements the run_command tool. Validation always runs first:
// a rejected command returns an error result and is never executed.
func (t *Tools) RunCommand(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	command, err := req.RequireString("command")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if ok, reason := validate.ValidateCommand(command, t.allow); !ok {
		return mcp.NewToolResultError("rejected: " + reason), nil
	}
	res, err := t.runner.Run(ctx, command)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("run command: %v", err)), nil
	}
	return jsonResult(res)
}

// ListAllowed implements the list_allowed tool.
func (t *Tools) ListAllowed(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return jsonResult(t.allow)
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	out, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("encode result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}
