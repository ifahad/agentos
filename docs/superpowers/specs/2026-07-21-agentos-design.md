# AgentOS — Design Spec

**Date:** 2026-07-21
**Status:** Approved (Phase 1 scope locked)

## One-liner

An open-source, self-hostable agentic operating layer: any LLM provider in, any
legacy system out, with governed autonomous agents in between that learn from
evaluated experience.

## Decisions (locked with user)

- **Phase 1 scope:** Core loop first — gateway + agent runtime + one legacy
  connector + governance/audit, wired end-to-end with a runnable demo.
- **Relationship to ai-forge:** standalone, clean start. No code dependency.
- **Repo:** `ifahad/agentos`, private for now, Apache-2.0, flips public later.
- **Deployment:** Docker Compose for dev/demo; Helm/K8s in a later phase.

## Architecture

Monorepo with five services. Every arrow is a network boundary with a stable
contract.

```
                    ┌─────────────────────────────┐
                    │  console/  (TypeScript)      │  Phase 2 — admin UI:
                    │                              │  keys, budgets, traces
                    └──────────────┬──────────────┘
                                   │
┌──────────────┐    ┌──────────────▼──────────────┐
│ LLM providers │◄───│  gateway/  (Go)             │  Phase 1 — OpenAI-compatible
│ Anthropic,    │    │                             │  proxy: virtual keys, budget
│ OpenAI, local │    └──────────────▲──────────────┘  caps, usage accounting,
└──────────────┘                    │                 audit log, guardrail hooks
                    ┌──────────────┴──────────────┐
                    │  runtime/  (Python)          │  Phase 1 — LangGraph deep
                    │                              │  agents, Postgres
                    └──────┬───────────────┬──────┘  checkpointing, MCP client
                           │ MCP (HTTP)    │
            ┌──────────────▼───┐   ┌───────▼────────┐
            │ connectors/ (Go) │   │ sandbox/ (Rust) │  Phase 3 — isolated
            │ legacy systems   │   │                 │  untrusted-tool exec
            │ as MCP servers   │   └────────────────┘
            └──────────────────┘
```

### gateway/ (Go)

Every model call in the platform flows through it.

- OpenAI-compatible `POST /v1/chat/completions`.
- Virtual API keys (`agos-…`) with per-key monthly USD budget caps; token usage
  priced from a static price table and accounted in Postgres.
- Provider routing by model prefix: `anthropic/…`, `openai/…`, `ollama/…`.
- Request audit log (who, model, tokens, cost, latency) — the AI-governance
  trail.
- Admin API (`/admin/keys`, `/admin/usage`) guarded by a root admin key.
- Pluggable guardrail hook interface (prompt-injection screening lands
  Phase 2; the interface exists Phase 1).

### runtime/ (Python)

- LangGraph agent (`create_react_agent` supervisor in Phase 1; deeper
  plan/delegate/reflect graphs with `deepagents` in Phase 2).
- Postgres checkpointing so runs survive restarts and support resuming
  threads.
- Talks to models **only** via the gateway (OpenAI-compatible client, virtual
  key). Talks to the world **only** via MCP tools.
- HTTP API: `POST /runs` to execute, thread-scoped memory via `thread_id`.
- LlamaIndex context engine (ingestion/retrieval): Phase 2.
- Human-in-the-loop interrupts: Phase 2 (checkpointer already supports it).

### connectors/ (Go)

Legacy systems wrapped as MCP servers. Phase 1: **SQL connector** — read-only
introspection and querying of any Postgres legacy database (`list_tables`,
`describe_table`, `query` with enforced read-only + row caps). Later: SOAP/REST
wrapper, SSH/terminal, file shares, browser/computer-use connectors.

### Self-improvement loop (Phase 3, designed now)

Episodic memory of runs + eval harness scoring outcomes; reflection jobs
propose prompt/policy changes that ship only when evals pass **and** a human
approves. Never free-running.

### Observability

Phase 1: structured logs, gateway audit/usage tables, checkpoint history.
Phase 2: OpenTelemetry traces exportable to Langfuse (OSS default) or
LangSmith.

## Phase plan

1. **Core loop (now):** gateway + runtime + SQL connector + Postgres +
   compose; demo agent answering questions against a seeded "legacy ERP"
   database through the full governed path.
2. Console UI (TS), OTel/Langfuse, deepagents graphs, LlamaIndex context
   engine, guardrails, more connectors, HITL.
3. Rust sandbox worker, self-improvement loop, Helm charts.

## Error handling

- Budget exceeded → HTTP 402 `{"error":{"type":"budget_exceeded"}}`.
- Unknown/invalid virtual key → 401.
- Provider failure → 502 with provider error passthrough (retry/fallback
  routing Phase 2).
- Connector rejects non-SELECT statements and caps result rows.
- Agent tool errors are captured in graph state, not fatal to the run.

## Testing

- Go: table-driven unit tests for key auth, budget accounting, routing, and
  SQL read-only enforcement (no network, memory store).
- Python: pytest over graph construction and the runs API with a fake
  model/gateway.
- End-to-end: compose-based smoke test script.
