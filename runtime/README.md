# agentos-runtime

AgentOS agent runtime: a LangGraph ReAct agent (data analyst over legacy
systems) that talks to the AgentOS gateway via the OpenAI-compatible API and
uses MCP streamable-http tools, exposed as a FastAPI service.

## API

- `POST /runs` `{"input": "...", "thread_id": "optional"}` ->
  `{"thread_id", "output", "steps": [{"tool", "input"}]}`
- `GET /healthz` -> `{"status": "ok"}`

## Configuration (env)

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
