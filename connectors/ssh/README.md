# agentos-ssh connector

MCP server (`agentos-ssh`, StreamableHTTP `/mcp` on `:8092`) that exposes one
legacy SSH host as tools: `run_command` (allowlisted single commands) and
`list_allowed`. One SSH connection is dialed lazily and reused (mutex-guarded)
with a fresh session per call.

## Environment

| Variable | Default | Meaning |
| --- | --- | --- |
| `AGENTOS_SSH_HOST` | — (required) | Target host. Fatal if unset. |
| `AGENTOS_SSH_PORT` | `22` | Target port. |
| `AGENTOS_SSH_USER` | — (required) | SSH user. Fatal if unset. |
| `AGENTOS_SSH_PASSWORD` | — | Password auth. Exactly one of this or `AGENTOS_SSH_PRIVATE_KEY` must be set; fatal otherwise. |
| `AGENTOS_SSH_PRIVATE_KEY` | — | PEM-encoded private key (the key material itself, not a path). |
| `AGENTOS_SSH_KNOWN_HOSTS` | — | Path to a `known_hosts` file → strict host key checking. **Unset → host key checking is disabled** (`InsecureIgnoreHostKey`) and a startup WARNING is logged. |
| `AGENTOS_SSH_ALLOW_COMMANDS` | — (deny all) | Comma-separated allowlist of command basenames, e.g. `ls,cat,grep,df,uptime,systemctl`. Empty = every command denied (startup warning). |
| `AGENTOS_SSH_TIMEOUT_S` | `15` | Per-command timeout in seconds; on expiry the session is closed and the result carries `timed_out: true`, `exit_code: -1`. |
| `AGENTOS_SSH_MAX_OUTPUT_BYTES` | `65536` | Per-stream stdout/stderr cap; overflow is discarded and flagged `truncated: true`. |

## Tools

- `run_command(command)` → runs one command, returns JSON
  `{"exit_code": N, "stdout": "...", "stderr": "...", "truncated": bool, "timed_out": bool}`.
  A remote non-zero exit is a normal result, not a tool error.
- `list_allowed()` → the configured allowlist as a JSON array.

## Allowlist model

`run_command` validates before anything touches the wire — a rejected command
is **never** executed:

1. Shell chaining/substitution metacharacters (`;`, `&&`, `||`, `|`,
   backtick, `$(` — plus `&` and newlines, which chain just as well) are
   rejected with `command chaining not allowed`. The allowlist governs a
   single command only.
2. The first whitespace token's basename (`/bin/ls` → `ls`) must be on the
   `AGENTOS_SSH_ALLOW_COMMANDS` list, else `command not allowed: <base>`.
   An empty allowlist denies everything.

This is a lexical gate, not a sandbox: an allowlisted binary can still do
whatever its arguments permit on the remote host. Keep the allowlist to
read-only diagnostics and pair it with a restricted SSH account.

## Host key checking

Set `AGENTOS_SSH_KNOWN_HOSTS` to a `known_hosts` file for strict checking
(`knownhosts.New`). If unset, the connector accepts any host key
(`ssh.InsecureIgnoreHostKey`) and logs a startup WARNING — acceptable for lab
demos, not for anything that matters.
