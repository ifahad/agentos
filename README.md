# AgentOS

[![ci](https://img.shields.io/github/actions/workflow/status/ifahad/agentos/ci.yml?branch=main&label=ci)](https://github.com/ifahad/agentos/actions/workflows/ci.yml)
[![evals](https://img.shields.io/github/actions/workflow/status/ifahad/agentos/evals.yml?branch=main&label=evals)](https://github.com/ifahad/agentos/actions/workflows/evals.yml)

**An open-source, self-hostable agentic operating layer: any LLM provider in,
any legacy system out, with governed autonomous agents in between.**

AgentOS combines the pieces enterprises actually need to run autonomous agents
in production — and nothing else:

- **Governed model access** — every LLM call flows through a Go gateway with
  virtual API keys, per-key budget caps, usage accounting, and an audit trail
  (LLMOps: centralized access, cost management, governance).
- **Deep agents** — a Python runtime built on LangGraph with Postgres
  checkpointing: durable, resumable, thread-scoped agent runs.
- **Legacy systems as tools** — old systems are wrapped as MCP servers by Go
  connectors, making any database (later: SOAP/REST, SSH, files, screens)
  available to any agent, safely (read-only enforcement, row caps).
- **Self-improvement, eval-gated** (Phase 3) — episodic memory + eval harness;
  proposed prompt/policy changes ship only when evals pass and a human
  approves.

```
 agent runtime (Python/LangGraph) ──► gateway (Go) ──► Anthropic / OpenAI / local
        │
        └─ MCP ──► connectors (Go) ──► your legacy systems
```

## Quickstart

Prereqs: Docker + Compose, one provider API key (or a local Ollama).

```bash
cd deploy
cp .env.example .env        # set AGENTOS_ANTHROPIC_API_KEY (or OpenAI/Ollama)
cd ..
make up                     # builds and starts postgres, gateway, connector, runtime
make smoke                  # end-to-end: agent answers a question from the legacy DB
```

Ask the agent something yourself:

```bash
curl -X POST http://localhost:8000/runs \
  -H 'Content-Type: application/json' \
  -d '{"input": "Which customers in Riyadh have unpaid invoices, and for how much?"}'
```

Watch the governance side:

```bash
curl http://localhost:8080/admin/usage -H "Authorization: Bearer admin-local-dev"
```

The demo ships a seeded "legacy ERP" Postgres database (customers, orders,
invoices) that the agent can only reach through the SQL connector's read-only
MCP tools — the same path any real legacy system would take.

## Repository layout

| Directory | Language | Purpose |
|---|---|---|
| `gateway/` | Go | LLM gateway: virtual keys, budgets, routing, audit |
| `runtime/` | Python | LangGraph agent runtime, MCP client, checkpointing |
| `connectors/sql/` | Go | Legacy SQL databases as read-only MCP tools |
| `console/` | TypeScript | Admin UI (Phase 2) |
| `sandbox/` | Rust | Isolated untrusted-tool execution (Phase 3) |
| `deploy/` | — | Docker Compose (Helm later) |

## Development

```bash
make test        # Go + Python unit tests (no network, no docker needed)
make fmt         # gofmt + ruff format
```

Design spec: [`docs/superpowers/specs/2026-07-21-agentos-design.md`](docs/superpowers/specs/2026-07-21-agentos-design.md)
· Phase 1 plan: [`docs/superpowers/plans/2026-07-21-phase-1-core-loop.md`](docs/superpowers/plans/2026-07-21-phase-1-core-loop.md)

## Console

`make up` also starts the admin console at http://localhost:3000
(`AGENTOS_CONSOLE_PORT`): usage per key, key creation, the audit trail with
guardrail badges, a playground with streaming and human-in-the-loop
approvals, and knowledge-base document management.

## Governance overlay

```bash
docker compose -f deploy/compose.yaml -f deploy/compose.hitl.yaml up -d
```

flips on human approval for the `query` tool (runs return
`202 pending_approval` until approved in the console or via
`POST /runs/{thread_id}/approve`) and prompt-injection blocking at the
gateway. `make smoke2` exercises all of it end-to-end.

## Sandboxed code execution

Agents get a `run_python` tool backed by the Rust sandbox service: per-run
temp workdir, cleared environment, process-group kill, CPU/memory/file
rlimits — inside a container running read-only with all capabilities
dropped. See `sandbox/README.md` for the isolation layers and residual risk.

## Self-improvement (eval-gated, human-approved)

```
POST /evals/run      run the eval suite (runtime/evals/default.yaml)
POST /improve        reflect on failures -> propose a new system prompt,
                     auto-evaluated against the baseline
POST /proposals/{id}/approve   the ONLY way a proposal activates
```

Proposals never activate themselves — the console's **Improve** page shows
eval history, baseline-vs-candidate scores, and approve/deny controls
(with an explicit warning if a human overrides a below-baseline candidate).

## Kubernetes

`deploy/helm/agentos/` — Helm 3 chart: all seven services with per-service
toggles, bundled pgvector Postgres or an external database URL, hardened
sandbox pod, optional console ingress. `deploy/helm/test-render.sh` verifies
rendering.

## Observability

```bash
docker compose -f deploy/compose.yaml -f deploy/compose.otel.yaml up -d
```

adds an OpenTelemetry collector; gateway and runtime emit spans per request,
model call, and tool call. Forward to Langfuse/LangSmith by editing
`deploy/otel-collector.yaml` (commented example inside).

### Bundled Langfuse

```bash
docker compose -f deploy/compose.yaml -f deploy/compose.otel.yaml \
  -f deploy/compose.langfuse.yaml up -d
```

brings up Langfuse OSS with its own dedicated Postgres and points the
collector at it. Open http://localhost:3001, create a project, and paste its
OTLP key pair into `.env` (`base64("pk:sk")` → `LANGFUSE_OTLP_BASIC_AUTH`);
see `deploy/langfuse.env.example`. Native OTLP-into-UI needs Langfuse v3
(`LANGFUSE_IMAGE=langfuse/langfuse:3` plus its ClickHouse/Redis/MinIO deps);
the collector's debug exporter shows traces either way.

## Hardening

- **Egress-less sandbox** — the code-execution sandbox runs on an
  internal-only Docker network with no outbound route; only the runtime can
  reach it. Helm ships a matching NetworkPolicy (DNS-only egress, runtime-only
  ingress).
- **Model-based guardrail** — `AGENTOS_GUARDRAILS_MODE=model` runs the fast
  heuristic first, then an LLM classifier (through the gateway, spend audited)
  for subtler injections. Classifier outages fail **open** with a
  `guardrail_error` audit entry, so the safety layer can't take down traffic.
  Use a fast, non-reasoning classifier — the default `claude-haiku-4-5` is
  ideal; reasoning models spend their token budget thinking and may return no
  verdict (which fails open). Tune with `AGENTOS_GUARDRAILS_MODEL`,
  `AGENTOS_GUARDRAILS_TIMEOUT_S`, and `AGENTOS_GUARDRAILS_MAX_TOKENS`.
- **SSH connector** — legacy boxes as MCP tools with a strict command
  allowlist and command-chaining rejection (`connectors/ssh/`, opt-in).

## Roadmap

1. ~~**Core loop**: gateway + runtime + SQL connector + compose demo.~~ ✅
2. ~~**Operability & governance**: console UI, streaming, guardrails,
   human-in-the-loop approvals, LlamaIndex/pgvector context engine,
   deepagents profile, opt-in OpenTelemetry.~~ ✅
3. ~~**Autonomy, safely**: Rust sandbox, eval-gated self-improvement loop,
   REST/OpenAPI connector + demo CRM, Helm chart, OTel collector profile.~~ ✅
4. ~~**Reach & hardening**: SSH connector, egress-less sandbox, model-based
   guardrail, LLM-judge evals, bundled Langfuse profile.~~ ✅
5. Next: SOAP/browser connectors, multi-tenant RBAC, secrets-manager
   integration, LLM-judge in CI.

License: [Apache-2.0](LICENSE)
