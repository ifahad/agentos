# AgentOS

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

## Roadmap

1. **Core loop** (this phase): gateway + runtime + SQL connector + compose demo.
2. Console UI, OpenTelemetry → Langfuse/LangSmith, deepagents planner graphs,
   LlamaIndex context engine, guardrails, streaming, more connectors,
   human-in-the-loop approvals.
3. Rust sandbox, eval-gated self-improvement loop, Helm charts.

License: [Apache-2.0](LICENSE)
