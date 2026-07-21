# AgentOS Phase 2 — Console, Governance & Context Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the core loop operable and enterprise-credible: a TypeScript admin console, streaming, prompt-injection guardrails, human-in-the-loop tool approvals, a LlamaIndex context engine over pgvector, an optional deep-agent profile, and opt-in OpenTelemetry.

**Architecture:** Additive to Phase 1 — no breaking changes to existing endpoints. Three parallel tasks (gateway v2, runtime v2, console) against frozen contracts; task 4 integrates (pgvector image, compose service, smoke v2).

**Tech Stack:** adds `pgvector/pgvector:pg16`, LlamaIndex (`llama-index-core`, `llama-index-vector-stores-postgres`), `deepagents`, Vite + React + TypeScript served by nginx (reverse-proxying APIs, no CORS).

## Global Constraints

- Phase 1 contracts stay valid verbatim; every Phase 2 feature is opt-in via env or new endpoints.
- Existing tests keep passing; new features get tests at the same standard (Go table-driven; pytest no-network; console: vitest for lib logic only).
- All model traffic (including embeddings) still flows through the gateway.
- Deferred out of Phase 2: additional connectors (REST/SSH/browser), Helm. Noted, not silent.

## Frozen contracts (additions)

### Gateway v2

- **Streaming:** `"stream": true` now supported — SSE passthrough of provider chunks (`text/event-stream`, raw `data:` lines forwarded verbatim, `[DONE]` terminated). Gateway injects `"stream_options": {"include_usage": true}` for openai/ollama and records usage from the final usage chunk; if the provider sends no usage, record zeros (audit still written). Budget check unchanged (pre-flight).
- **Embeddings:** `POST /v1/embeddings` — same auth/routing/prefix-stripping as chat completions (`ollama/bge-m3` → `$AGENTOS_OLLAMA_BASE_URL/v1/embeddings`, etc.). Usage recorded (prompt tokens; cost from price table, default 0).
- **Guardrails:** env `AGENTOS_GUARDRAILS_MODE=off|log|block` (default `off`). Screens the **latest user message** of chat requests with heuristic patterns (case-insensitive: "ignore previous/above/all instructions", "disregard your instructions", "you are now DAN", "reveal/print your system prompt", "###\s*system", base64 blobs > 200 chars). `log` → audit entry `guardrail_flag`; `block` → 400 `{"error":{"type":"guardrail_blocked","message":"…"}}` + audit. Implemented behind a `Guardrail` interface (screen(request) → verdict) so model-based screeners can be added later.
- **Audit listing:** `GET /admin/audit?limit=N` (default 50, max 500, newest first) → `[{ts, key_name, model, input_tokens, output_tokens, cost_usd, latency_ms, status, kind}]` where kind ∈ `chat|embeddings|guardrail_flag|guardrail_block`.
- **CORS:** env `AGENTOS_CORS_ORIGINS` (comma-separated) → standard CORS headers + OPTIONS preflight on /v1/* and /admin/*. Empty (default) = no CORS headers (console uses nginx proxy instead).
- **OTel:** env `AGENTOS_OTEL_ENDPOINT` (OTLP/HTTP base URL) — when set, emit a span per proxied request (attrs: model, key name, tokens, cost, status). When unset: zero overhead, no new required deps at runtime.

### Runtime v2

- **Streaming:** `POST /runs/stream` (same body as /runs) → SSE events, each `data: {json}`:
  `{"event":"step","tool":…,"input":…}` per tool call as it happens,
  `{"event":"pending_approval","pending":[…]}` if interrupted (stream ends),
  `{"event":"done","thread_id":…,"output":…,"steps":[…]}` terminal.
- **HITL:** env `AGENTOS_APPROVAL_TOOLS` (comma-separated tool names, e.g. `query`; empty = off). Agent graph compiled with `interrupt_before=["tools"]` when enabled. Run loop: on interrupt, if none of the pending tool calls are in the approval list → auto-resume; else `POST /runs` returns HTTP 202 `{"status":"pending_approval","thread_id":…,"pending":[{"tool":…,"input":…}]}`.
  `POST /runs/{thread_id}/approve` body `{"approve": true|false}`: true → resume execution to completion (or next interrupt), returns the normal RunResponse (or another 202); false → inject a ToolMessage "Denied by human reviewer." for each pending call, resume, return RunResponse. 404 for unknown/non-pending thread.
  Non-interrupted /runs responses gain `"status":"completed"` (additive).
- **Threads:** `GET /threads/{thread_id}` → `{"thread_id":…,"messages":[{"role":"user|assistant|tool","content":…,"tool_calls":[…]?}]}` from the checkpointer; 404 if unknown.
- **Context engine (LlamaIndex + pgvector):**
  `POST /documents` `{"name":…,"text":…}` → chunk (LlamaIndex SentenceSplitter 512/64), embed via gateway `POST /v1/embeddings` (model env `AGENTOS_EMBED_MODEL`, default `ollama/bge-m3`), store in pgvector through `llama-index-vector-stores-postgres` (table `agentos_documents`, connection `AGENTOS_CHECKPOINT_DATABASE_URL` — reuse the agentos DB). Response `{"name":…,"chunks":N}`.
  `GET /documents` → `[{"name":…,"chunks":N}]`.
  Agent gains tool `search_knowledge(query: str)` → top-5 chunks with source names — only registered when the context engine is enabled (env `AGENTOS_CONTEXT_ENGINE=on|off`, default on when checkpoint DB set).
- **Deep-agent profile:** env `AGENTOS_AGENT_PROFILE=react|deep` (default `react`). `deep` builds via the `deepagents` package (`create_deep_agent`) with the same model/tools/checkpointer/system prompt; react remains default and fully supported.
- **OTel:** env `AGENTOS_OTEL_ENDPOINT` — when set, instrument runs with spans (run, model call, tool call). Unset = no-op.

### Console (TypeScript)

- `console/`: Vite + React + TS SPA, dark-first admin UI. Served by nginx (Dockerfile) on port 3000; nginx proxies `/api/gateway/` → `http://gateway:8080/` and `/api/runtime/` → `http://runtime:8000/` (strip prefix), so the SPA calls same-origin only.
- Admin key: entered in a settings modal, kept in localStorage, sent as Bearer on /api/gateway/admin/*.
- Pages: **Overview** (usage totals per key, from /admin/usage), **Keys** (list, create — show secret once), **Audit** (table from /admin/audit, kind badges), **Playground** (input → POST /api/runtime/runs; render steps timeline; on 202 show pending tool calls with Approve/Deny wired to /approve; optional streaming via /runs/stream), **Documents** (list, add name+text).
- No UI component library needed; hand-rolled minimal styles. Vitest only for pure logic (api client, formatting). No e2e in Phase 2.

### Compose v2

- `postgres` image → `pgvector/pgvector:pg16` (same env/volume/initdb).
- New service `console` (build ../console) port `${AGENTOS_CONSOLE_PORT:-3000}:80`, depends_on gateway+runtime.
- Runtime env adds AGENTOS_APPROVAL_TOOLS (default empty), AGENTOS_EMBED_MODEL, AGENTOS_AGENT_PROFILE.
- `scripts/smoke2.sh`: Phase 1 smoke + streaming run + guardrail block check (mode=block via env override compose run) is skipped if mode off + document ingest → agent answers from ingested doc via search_knowledge + HITL approve flow (set AGENTOS_APPROVAL_TOOLS=query for the check via `docker compose exec`-free env restart or a dedicated compose override file `deploy/compose.hitl.yaml`).

---

### Task 1: Gateway v2 (Go) — streaming, embeddings, guardrails, audit list, CORS, OTel
**Files:** modify `gateway/internal/server/`, `gateway/internal/provider/`, `gateway/internal/store/` (audit listing + kind column), add `gateway/internal/guardrail/`. Tests for: SSE passthrough (httptest fake streaming provider), usage-from-stream-chunk, embeddings routing, guardrail patterns table, audit limit/order, CORS preflight.

### Task 2: Runtime v2 (Python) — streaming, HITL, threads, context engine, deep profile, OTel
**Files:** modify `runtime/src/agentos_runtime/{agent.py,api.py,config.py}`, add `context.py`, `hitl.py`. Tests with fake models/embedders (no network): interrupt→202→approve/deny paths, auto-resume for non-approval tools, /threads shape, SSE event sequence, document chunk counts with a fake embedding function, profile selection.

### Task 3: Console (TypeScript)
**Files:** `console/` full Vite app + nginx Dockerfile per contract. `npm run build` and `npm test` must pass.

### Task 4: Integration (orchestrator)
**Files:** `deploy/compose.yaml` (pgvector image, console service, new env), `deploy/compose.hitl.yaml`, `scripts/smoke2.sh`, README + spec updates, Makefile targets (`smoke2`, `test` incl. console).

## Self-review notes
- Additional connectors and Helm explicitly deferred (documented above and in README roadmap).
- pgvector image swap is compatible with the existing volume only if re-initialized; smoke docs say `make down` (drops volume) before first Phase 2 `make up`.
- Env names cross-checked against Phase 1 plan; no collisions; all new vars listed in `.env.example` in Task 4.
