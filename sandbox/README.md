# sandbox/ — Isolated Python execution

Rust service (axum + tokio) that executes untrusted, agent-generated Python
code in an isolated child process, so agents can run code without endangering
the host. Listens on **:8070**.

## API

### `POST /execute`

Request:

```json
{
  "language": "python",
  "code": "print('hi')",
  "timeout_s": 10,
  "stdin": ""
}
```

- `language` — only `"python"`. Anything else →
  `400 {"error": "unsupported language"}`.
- `timeout_s` — optional, default `10`, clamped to
  `[1, AGENTOS_SANDBOX_MAX_TIMEOUT_S]`.
- `stdin` — optional; written to the child's stdin, which is then closed (EOF).

Response (`200`):

```json
{
  "exit_code": 0,
  "stdout": "hi\n",
  "stderr": "",
  "duration_ms": 42,
  "timed_out": false,
  "truncated": false
}
```

- `stdout` / `stderr` are each capped at `AGENTOS_SANDBOX_MAX_OUTPUT_BYTES`;
  if either stream produced more, the excess is discarded and
  `truncated: true`. Streams are drained concurrently, so a chatty child never
  deadlocks on a full pipe.
- On wall-clock timeout the whole **process group** gets SIGKILL and the
  response has `timed_out: true`, `exit_code: -1`.

### `GET /healthz`

Returns `ok` (200).

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `AGENTOS_SANDBOX_MAX_TIMEOUT_S` | `30` | Upper bound for `timeout_s` |
| `AGENTOS_SANDBOX_MAX_OUTPUT_BYTES` | `65536` | Per-stream stdout/stderr cap |

Port is fixed at 8070.

## Isolation layers

Per execution, in-process:

1. **Fresh temp workdir** — code is written to `main.py` in a `tempfile`
   directory that is removed after the run.
2. **Cleared environment** — the child sees exactly
   `PATH=/usr/local/bin:/usr/bin:/bin` and nothing else. (CPython's PEP 538
   locale coercion may add `LC_CTYPE` to `os.environ` after startup; the
   exec-time environment is PATH-only.)
3. **New process group** — `setpgid(0, 0)` in `pre_exec`, so timeout
   enforcement `killpg`s the child *and* anything it spawned.
4. **rlimits** (applied in `pre_exec`, soft = hard):
   - `RLIMIT_CPU` = `timeout_s` (CPU-bound runaways die even if wall clock
     enforcement were bypassed)
   - `RLIMIT_AS` = 512 MiB
   - `RLIMIT_NPROC` = 64
   - `RLIMIT_FSIZE` = 8 MiB
5. **Wall-clock timeout** — SIGKILL to the process group at `timeout_s`.

Expected container hardening (applied by the orchestrator compose/Helm):
`read_only` rootfs, `cap_drop: [ALL]`,
`no-new-privileges`, tmpfs `/tmp`, memory/CPU limits, non-root user (the image
already runs as `sandbox`, uid 10001).

### Residual risk

Code running inside the container can still make **outbound network requests**
(in-container network egress). Mitigation is on the roadmap: an egress-less
sidecar topology where the sandbox container has no network route except the
API listener.

## Development

Requires a Rust toolchain and `python3` on PATH (integration tests execute
real Python).

```sh
cargo fmt --check
cargo clippy --all-targets -- -D warnings
cargo test
cargo run   # serves on :8070
```

## Docker

```sh
docker build -t agentos-sandbox .
docker run --rm -p 8070:8070 \
  --read-only --cap-drop ALL --security-opt no-new-privileges \
  --tmpfs /tmp --memory 768m --cpus 1 \
  agentos-sandbox
```

(`--tmpfs /tmp` is required with `--read-only`: per-run workdirs live under
`/tmp`.)
