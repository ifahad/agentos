# agentos-runtime

AgentOS agent runtime: a LangGraph ReAct agent (data analyst over legacy
systems) that talks to the AgentOS gateway via the OpenAI-compatible API and
uses MCP streamable-http tools, exposed as a FastAPI service.

## API

- `POST /runs` `{"input": "...", "thread_id": "optional"}` ->
  `{"thread_id", "output", "steps": [{"tool", "input"}]}`
- `GET /healthz` -> `{"status": "ok"}`

## Authentication

Every route **except `GET /healthz`** requires
`Authorization: Bearer <AGENTOS_RUNTIME_AUTH_TOKEN>` (constant-time compared).
A missing/wrong token returns `401 {"detail": "invalid runtime token"}`. The
service **refuses to start** when `AGENTOS_RUNTIME_AUTH_TOKEN` is unset/empty
(fail-closed). Callers in front of the runtime (the console via nginx, smoke
scripts) must send this header on every `/runs*`, `/threads*`, `/documents*`,
`/evals*`, `/improve`, `/proposals*`, and `/prompts*` request.

Prompt proposals are **advisory persona refinements**, never a way to replace
the safety frame: an immutable safety preamble is always prepended when the
agent is built, and a proposal containing an override marker (e.g. "ignore
previous", "disregard", "system prompt", "auto-approve", "exfiltrate",
"bypass") is rejected `400` at approval. Retrieved documents are returned to
the model wrapped in `<<UNTRUSTED_DOCUMENT …>>` delimiters as reference data.

## Configuration (env)

- `AGENTOS_RUNTIME_AUTH_TOKEN` - bearer token required on every route except
  `GET /healthz` (required; the app refuses to start without it)
- `AGENTOS_GATEWAY_URL` - gateway base URL (required)
- `AGENTOS_GATEWAY_KEY` - gateway API key (required)
- `AGENTOS_MODEL` - default `anthropic/claude-sonnet-5`
- `AGENTOS_MCP_SERVERS` - comma-separated streamable-http MCP URLs (may be empty)
- `AGENTOS_CHECKPOINT_DATABASE_URL` - Postgres URL for checkpoints (empty -> in-memory)

## Develop

```sh
uv sync
uv run ruff check .
uv run pytest
uv run python -m agentos_runtime.main   # serves on 0.0.0.0:8000
```
