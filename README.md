# AgentOS

An open-source, self-hostable agentic operating layer: any LLM provider in,
any legacy system out, with governed autonomous agents in between.

> Phase 1 (core loop) is under construction. See
> [`docs/superpowers/specs/2026-07-21-agentos-design.md`](docs/superpowers/specs/2026-07-21-agentos-design.md)
> for the architecture and
> [`docs/superpowers/plans/2026-07-21-phase-1-core-loop.md`](docs/superpowers/plans/2026-07-21-phase-1-core-loop.md)
> for the build plan.

## Layout

| Directory | Language | Purpose |
|---|---|---|
| `gateway/` | Go | LLM gateway: virtual keys, budgets, routing, audit |
| `runtime/` | Python | LangGraph agent runtime, MCP client, checkpointing |
| `connectors/` | Go | Legacy systems wrapped as MCP servers |
| `console/` | TypeScript | Admin UI (Phase 2) |
| `sandbox/` | Rust | Isolated tool execution (Phase 3) |
| `deploy/` | — | Docker Compose, later Helm |

License: Apache-2.0
