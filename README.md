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

## Security

A pre-autonomy security audit and its hardening are recorded in
[`docs/security/2026-07-21-security-assessment.md`](docs/security/2026-07-21-security-assessment.md).
Hardened in that pass: the **runtime API now requires authentication**
(`AGENTOS_RUNTIME_AUTH_TOKEN`, fail-closed; the console injects it server-side
via nginx), the **SSH connector** denies exec-capable binaries and requires
`known_hosts`, **REST/SOAP** strip their auth header across redirects and refuse
private-IP SSRF, **OIDC** requires `email_verified` and matches on stable `sub`,
**tenant usage/spend/audit** are isolated by key hash + org (no same-name
cross-org leak), **secret rotation** reaches live provider keys, and an
**immutable safety preamble** frames every agent run (retrieved/tool content is
treated as untrusted data). Set `AGENTOS_RUNTIME_AUTH_TOKEN` and a real
`AGENTOS_ADMIN_KEY` for any non-local deployment.

## Multi-tenancy & secrets

- **RBAC** — orgs, users, and roles (`owner`/`admin`/`member`/`viewer`) with
  `agu-…` user tokens. The admin API scopes keys, usage, and audit to the
  caller's org; org budgets cap aggregate spend (402 `org_budget_exceeded`).
  The root admin key stays a global superuser. Fully backward compatible —
  existing keys live in a bootstrapped `default` org (unlimited).
- **Secrets backend** — provider keys resolve through `AGENTOS_SECRETS_BACKEND`:
  `env` (default), `file` (hot-reloaded JSON), `age` (age-encrypted file
  decrypted in memory), or `vault` (HashiCorp Vault KV v2 over its HTTP API).
  `GET /admin/secrets/status` reports presence and source, never values.
- **OIDC SSO** — set `AGENTOS_OIDC_ISSUER` (+ client id/secret/redirect) and
  the console shows "Sign in with SSO". The gateway runs a real OIDC
  authorization-code flow (JWKS/RS256 ID-token verification, HMAC-signed
  state), then mints an `agu-…` user token for the identity. Off by default.
- **whoami** — `GET /admin/whoami` returns the caller's identity (root, or
  user id/org/email/role); the console uses it to drive role-aware UI without
  manual entry.
- **Per-tenant rate limits** — each org has a `rate_limit_rpm` (token bucket,
  0 = unlimited); over-limit chat/embeddings calls get 429 `rate_limited` with
  a `Retry-After` header. Set per org (`PATCH /admin/orgs/{id}`) or globally
  (`AGENTOS_RATE_LIMIT_RPM`). `AGENTOS_RATELIMIT_BACKEND=postgres` shares the
  bucket across gateway replicas via atomic SQL (default `memory` per-instance).
- **SCIM 2.0 provisioning** — set `AGENTOS_SCIM_TOKEN` and an IdP (Okta/Entra)
  can create, list, deactivate, and delete users at `/scim/v2/Users`.
  Deactivating a user immediately invalidates their `agu-…` tokens — the
  deprovisioning path. Off by default (routes 404 when the token is unset).
- **Secret rotation** — `POST /admin/secrets/reload` (root) re-fetches the
  active backend (file re-read, Vault re-GET, age re-decrypt) with no restart;
  `AGENTOS_SECRETS_REFRESH_S` polls on an interval. Rotated keys take effect
  on the next request.

## Connectors

SQL and REST connectors start with the stack. SOAP (`connectors/soap/`) and
browser/computer-use (`connectors/browser/`, Playwright) are opt-in:

```bash
docker compose --profile connectors up -d   # + soap + browser
```

## CI

GitHub Actions runs the full test matrix (Go, Python, Rust, console, Helm) on
every push, plus an **eval gate** (`evals.yml`) that scores the runtime eval
suite against a deterministic mock model and fails under 0.8 — label a PR
`run-evals` to run it, or wire real provider secrets to grade real models.

## Operators — native autonomy

**Operators** are standing objectives the runtime pursues on its own, driving the
single governed agent (parallel to the council, which drives many models). Each
operator has a goal and a **trigger**:

- **interval** — fire every N seconds (floor 30s);
- **cron** — a 5-field schedule (validated with `croniter`);
- **webhook** — fired by an inbound `POST /operators/webhooks/{token}`, where the
  opaque `whk-` token *is* the credential (the one route exempt from the runtime
  bearer; an unknown token 404s). The token is returned once at creation and
  redacted on every read.

Every operator run goes through the gateway on the runtime's key, so budgets,
rate limits, guardrails, and the audit trail all apply; it is bounded by
`max_cycles`; and a run that hits a human-approval interrupt is recorded
`needs_approval` and is **not** auto-resumed — autonomy never approves its own
writes. The scheduler is **opt-in** (`AGENTOS_AUTONOMY_ENABLED`, off by default):
the runtime serves the operators API but never fires on its own unless enabled.

**Skills** (`SKILL.md`) are reviewed, in-repo instruction sheets the agent pulls
on demand via a `use_skill` tool. They load **only** from the image-baked
`runtime/skills/` directory — never fetched at runtime, never from a registry —
and each records a sha256 for provenance (see `runtime/skills/README.md`). This
is the direct lesson of the OpenClaw supply chain: a skill is code, so it is
reviewed in-repo, not downloaded. `make smoke8` exercises the whole surface live.

## Multiverse council

A **council** of model-bound agents answers one objective in parallel, and a
judge synthesizes their answers into a single verdict **with an explicit dissent
report** — disagreement is recorded, not averaged away.

- **Members** are configured in `runtime/council.yaml`: each is an agent bound
  to one model, with its own profile, persona, tool subset, and checkpoint
  thread. The five frontier members (Kimi K3, GLM-5.2, Qwen 3.8 Max,
  DeepSeek-V4 Pro, MiniMax M3) ship **disabled** — their vendor endpoints, model
  ids, and pricing in `deploy/providers.json` are **unverified placeholders** an
  operator must confirm before enabling. Five local Ollama models ship enabled,
  so a real five-way council runs at **$0**.
- **Provider registry** (`deploy/providers.json`): any OpenAI-compatible vendor
  is reachable by config, with real per-1M-token pricing that makes budgets and
  `/admin/usage` accurate for non-builtin models. Base URLs are SSRF-screened
  (https-only except loopback; no private/link-local hosts). `GET /admin/providers`
  reports what is configured without ever leaking a key.
- **Governance defaults are conservative.** The autonomous heartbeat is **off**
  (`AGENTOS_COUNCIL_HEARTBEAT_S=0`); the action surface is **read freely, propose
  writes** (write-class tool calls become human-approved proposals, fail-closed —
  an unclassified tool is a write); each objective is bounded by a cycle cap and
  a spend ceiling; and a **kill switch** (`POST /council/pause`) halts the loop
  within one cycle.
- **`council/multiverse`** is exposed as an OpenAI-compatible model: any OpenAI
  client gets the whole council behind one model name, with dissent in an
  `x_agentos_council` extension. **Two independent recursion guards** stop the
  council calling itself: the gateway refuses a council model on any request
  carrying the depth marker, and `council.yaml` rejects members on a `council/`
  model.
- **Try it:** `make smoke9` runs a real five-model council against the seeded
  legacy ERP database.

**Honest note on the local models** (from `scripts/smoke9.sh`): the local
council runs the **react** profile, not deep. On the deep profile the local
models exhaust the recursion limit inside deepagents' own graph and answer
nothing; on react they call tools reliably. Of the five local models, four
(qwen3.5, qwen3.6, gemma4, gemma4:31b) call tools and answer consistently;
gemma3 is flaky (intermittent 502s) but the council tolerates it by design —
quorum and failure isolation mean one or two bad members never deny the verdict.
The frontier five remain unverified and disabled.

## Roadmap

1. ~~**Core loop**: gateway + runtime + SQL connector + compose demo.~~ ✅
2. ~~**Operability & governance**: console UI, streaming, guardrails,
   human-in-the-loop approvals, LlamaIndex/pgvector context engine,
   deepagents profile, opt-in OpenTelemetry.~~ ✅
3. ~~**Autonomy, safely**: Rust sandbox, eval-gated self-improvement loop,
   REST/OpenAPI connector + demo CRM, Helm chart, OTel collector profile.~~ ✅
4. ~~**Reach & hardening**: SSH connector, egress-less sandbox, model-based
   guardrail, LLM-judge evals, bundled Langfuse profile.~~ ✅
5. ~~**Enterprise reach & governance**: multi-tenant RBAC, secrets backend
   (env/file/age), SOAP + browser connectors, CI with an LLM-judge eval
   gate.~~ ✅
6. ~~**Enterprise identity**: OIDC SSO, Vault secrets backend, per-tenant rate
   limits, `whoami` endpoint.~~ ✅
7. ~~**Scale & provisioning**: SCIM 2.0 user provisioning, distributed
   (Postgres) rate-limit store, secret rotation/reload.~~ ✅
8. ~~**Hardening & efficiency**: atomic budget reservation (TOCTOU),
   request-body caps + server timeouts + graceful drain, non-root images with
   K8s `securityContext`, SQL `statement_timeout`, opt-in audit retention,
   runtime context trimming.~~ ✅
9. ~~**Multiverse**: config-driven provider registry, upstream retry/fallback,
   a council of model-bound deep agents with judge synthesis and dissent
   reporting, governed autonomous loop, and `council/multiverse` as an
   OpenAI-compatible model.~~ ✅
10. ~~**Operators**: governed single-agent autonomy — interval/cron/webhook
   triggers toward stored objectives, bounded and audited, with in-repo
   checksummed `SKILL.md` skills the agent pulls on demand.~~ ✅
11. Next: SAML SSO, cloud-KMS secret backends, Redis limiter option, secret
   rotation webhooks, SCIM Groups.

License: [Apache-2.0](LICENSE)
