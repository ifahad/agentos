# AgentOS Phase 1 — Core Loop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A runnable governed agent loop: LangGraph agent → Go gateway → LLM provider, acting on a legacy Postgres DB through a Go MCP connector, all under `docker compose up`.

**Architecture:** Three independently testable services (gateway, runtime, sql-connector) sharing one Postgres instance, glued by docker compose. Contracts below are frozen; tasks 1–3 are parallelizable, task 4 integrates.

**Tech Stack:** Go 1.24 (`net/http`, `pgx`, `mark3labs/mcp-go`), Python 3.12 (`langgraph`, `langchain-openai`, `langchain-mcp-adapters`, `langgraph-checkpoint-postgres`, FastAPI, uv), Postgres 16, Docker Compose.

## Global Constraints

- License Apache-2.0; module paths `github.com/ifahad/agentos/...`.
- Go services: stdlib HTTP, no web frameworks. Table-driven tests.
- Python: uv-managed project, `ruff` clean, pytest.
- All service config via env vars, prefixed `AGENTOS_`.
- No secrets committed; `.env.example` documents everything.
- Services must not commit to git (orchestrator commits).

## Frozen contracts

### Network layout (compose service names)

| Service | Port | URL inside compose |
|---|---|---|
| postgres | 5432 | `postgres:5432`, superuser `agentos`/`agentos` |
| gateway | 8080 | `http://gateway:8080` |
| runtime | 8000 | `http://runtime:8000` |
| sql-connector | 8090 | `http://sql-connector:8090/mcp` (MCP streamable HTTP) |

Databases in the single Postgres instance: `agentos` (gateway + checkpoints),
`legacy_erp` (demo target, seeded read-only user `erp_reader`/`erp_reader`).

### Gateway HTTP API

- `POST /v1/chat/completions` — OpenAI request/response passthrough.
  Auth: `Authorization: Bearer agos-<key>`. Model field uses provider prefix
  (`anthropic/claude-sonnet-5`, `openai/gpt-4o-mini`, `ollama/llama3.1`);
  gateway strips the prefix before forwarding. Streaming NOT in Phase 1
  (reject `"stream": true` with 400 `unsupported`).
- Errors: 401 `invalid_key`, 402 `budget_exceeded`, 400 `unsupported`,
  502 `provider_error` — body `{"error":{"type":"<type>","message":"…"}}`.
- `POST /admin/keys` `{"name":"…","monthly_budget_usd":25.0}` →
  `{"key":"agos-…","name":"…","monthly_budget_usd":25.0}` (key shown once).
- `GET /admin/keys` → list without secrets (name, budget, spend_usd).
- `GET /admin/usage` → per-key totals `{requests, input_tokens, output_tokens, spend_usd}`.
- Admin auth: `Authorization: Bearer $AGENTOS_ADMIN_KEY`.
- `GET /healthz` → 200 `ok` (no auth).
- Env: `AGENTOS_ADMIN_KEY`, `AGENTOS_DATABASE_URL` (empty → in-memory store),
  `AGENTOS_ANTHROPIC_API_KEY`, `AGENTOS_OPENAI_API_KEY`, `AGENTOS_OLLAMA_BASE_URL`.

### SQL connector MCP tools

Server name `agentos-sql`, streamable HTTP at `/mcp`, port 8090, env
`AGENTOS_CONNECTOR_DATABASE_URL`, `AGENTOS_CONNECTOR_MAX_ROWS` (default 200).

- `list_tables()` → JSON `[{"schema":"public","name":"customers"}]`
- `describe_table(table string)` → JSON columns `[{"name","type","nullable"}]`
- `query(sql string)` → JSON `{"columns":[…],"rows":[[…]],"row_count":N,"truncated":bool}`
  Read-only enforcement: reject unless first keyword (comments stripped) is
  SELECT or WITH; single statement only; executed inside a
  `BEGIN TRANSACTION READ ONLY`; rows capped at max rows.

### Runtime HTTP API

- `POST /runs` `{"input":"…","thread_id":"optional"}` →
  `{"thread_id":"…","output":"…","steps":[{"tool":"query","input":{…}}]}`
- `GET /healthz` → 200.
- Env: `AGENTOS_GATEWAY_URL`, `AGENTOS_GATEWAY_KEY`, `AGENTOS_MODEL`
  (default `anthropic/claude-sonnet-5`), `AGENTOS_MCP_SERVERS`
  (comma-separated URLs), `AGENTOS_CHECKPOINT_DATABASE_URL` (empty → in-memory
  checkpointer).

---

### Task 1: Gateway service (Go) — `gateway/`

**Files:** `gateway/go.mod`, `gateway/cmd/gateway/main.go`,
`gateway/internal/server/server.go` (+ handlers), `gateway/internal/store/store.go`
(interface + memory impl), `gateway/internal/store/postgres.go`,
`gateway/internal/provider/provider.go` (routing + price table),
`gateway/internal/*/**_test.go`, `gateway/Dockerfile`.

**Interfaces — Produces:** the Gateway HTTP API above. Store interface:

```go
type Store interface {
    CreateKey(ctx context.Context, name string, budgetUSD float64) (secret string, err error)
    Authenticate(ctx context.Context, secret string) (*Key, error) // ErrInvalidKey
    RecordUsage(ctx context.Context, u Usage) error                // updates spend
    Usage(ctx context.Context) ([]KeyUsage, error)
    Keys(ctx context.Context) ([]KeyInfo, error)
}
```

Price table (USD per 1M tokens, in/out): `claude-sonnet-5` 3/15,
`claude-haiku-4-5` 1/5, `gpt-4o-mini` 0.15/0.60, default 0/0 (local models
free). Budget check happens **before** forwarding: if `spend >= budget` → 402.

Steps (TDD, commit per green): key auth (memory store) → budget accounting →
model-prefix routing table → chat-completions proxy handler with recorded
usage from response `usage` field → admin endpoints → Postgres store (guarded
by `AGENTOS_TEST_DATABASE_URL`, skipped otherwise) → Dockerfile (multi-stage,
distroless). Run: `cd gateway && go test ./...`.

### Task 2: SQL MCP connector (Go) — `connectors/sql/`

**Files:** `connectors/sql/go.mod`, `connectors/sql/cmd/sql-connector/main.go`,
`connectors/sql/internal/tools/tools.go`, `connectors/sql/internal/tools/readonly.go`
(+ tests), `connectors/sql/Dockerfile`.

**Interfaces — Produces:** the MCP tools above via `mark3labs/mcp-go`
streamable HTTP server.

Steps: read-only SQL validator (strip `--`/`/* */` comments, single statement,
first keyword SELECT|WITH — full table-driven test incl. `; DROP`,
`WITH x AS (…) DELETE`, comment-prefixed injections) → tool handlers against
pgx with READ ONLY tx + row cap → server wiring → Dockerfile.
Run: `cd connectors/sql && go test ./...`.

### Task 3: Agent runtime (Python) — `runtime/`

**Files:** `runtime/pyproject.toml`, `runtime/src/agentos_runtime/{config.py,agent.py,api.py,main.py}`,
`runtime/tests/{test_agent.py,test_api.py}`, `runtime/Dockerfile`.

**Interfaces — Consumes:** gateway chat-completions (via `ChatOpenAI(base_url=…)`),
MCP tools (via `langchain_mcp_adapters.client.MultiServerMCPClient`,
transport `streamable_http`). **Produces:** the Runtime HTTP API above.

Agent: `langgraph.prebuilt.create_react_agent(model, tools, prompt=SYSTEM_PROMPT, checkpointer=…)`;
system prompt: careful data analyst over legacy systems, inspect schema before
querying, never fabricate data. `steps` in the response = tool calls extracted
from the final message list. Checkpointer: `AsyncPostgresSaver` when
`AGENTOS_CHECKPOINT_DATABASE_URL` set, else `InMemorySaver`; `.setup()` on boot.
Tests use a `FakeToolCallingModel`/monkeypatched agent — no network.
Run: `cd runtime && uv run pytest`.

### Task 4: Compose integration + demo + docs (orchestrator)

**Files:** `deploy/compose.yaml`, `deploy/initdb/01-legacy-erp.sql` (seed:
customers, orders, invoices w/ realistic rows; `erp_reader` read-only role),
`deploy/.env.example`, `Makefile`, `README.md`, `scripts/smoke.sh`,
`docs/` cross-links, `console/README.md`, `sandbox/README.md` (phase markers).

Smoke: `docker compose up -d --build` → create key via admin API → `POST /runs`
asking "Which customer has the highest total order value?" → assert non-empty
output mentioning seeded top customer; `GET /admin/usage` shows spend > 0.

---

## Self-review notes

- Streaming, guardrail execution, OTel, LlamaIndex, HITL, retry/fallback are
  explicitly Phase 2 (spec updated to match). `/v1/messages` native passthrough
  deferred to Phase 2 — runtime speaks OpenAI-compat only.
- Type/name consistency: env names and endpoint shapes in tasks match the
  frozen-contracts section verbatim; subagents must copy them exactly.
