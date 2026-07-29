# AgentOS

**An open-source, self-hostable agentic operating layer: any LLM provider in,
any legacy system out, with governed autonomous agents in between.**

Agents are useful exactly to the degree you can let them touch real systems.
AgentOS is the layer that makes that defensible: **nothing runs unauthorized,
unattributed, or unrecorded.**

```
                    ┌──────────────┐
   operator ───────►│   console    │  same-origin /api/* only; nginx injects
                    │  (TS, :3000) │  the runtime token server-side
                    └──────┬───────┘
                           │
  any OpenAI client ──┐    │
                      ▼    ▼
                 ┌─────────────────┐        ┌──────────────────────────┐
                 │  gateway (Go)   │───────►│ providers: Anthropic /   │
                 │     :8080       │        │ OpenAI / Ollama / …      │
                 │ keys · budgets  │        └──────────────────────────┘
                 │ limits · guard  │
                 │ RBAC · audit    │
                 └────────▲────────┘
                          │  every model call — the runtime holds no keys
                 ┌────────┴────────┐
                 │ runtime (Python)│
                 │     :8000       │
                 │ agents · skills │
                 │ operators       │
                 │ council         │
                 └───┬─────────┬───┘
                     │ MCP     │ HTTP
          ┌──────────▼──┐   ┌──▼──────────────┐
          │ connectors  │   │ sandbox (Rust)  │
          │ sql   :8090 │   │     :8070       │
          │ rest  :8091 │   │ no egress,      │
          │ ssh   :8092 │   │ no host port    │
          │ soap  :8093 │   └─────────────────┘
          │ browser:8094│
          └──────┬──────┘
                 ▼
        your legacy systems
```

That sentence is a claim about actions the platform *executes*: each one is
authorized against a credential, attributed to a caller and an org, and written
to the audit log. Coverage on the **refusal** side is deliberately partial and
documented as such — see [`SECURITY.md`](SECURITY.md).

The project's public landing page is [`landing/index.html`](landing/index.html)
— one self-contained file, so you can open it straight from disk. Compose also
serves it at `http://localhost:8081` as the `landing` service, which calls no
API and depends on nothing else in the stack. The
operator console runs at `http://localhost:3000` once the stack is up and is
documented in [`docs/console.md`](docs/console.md). The same handbook also
rides along in the running console itself, at `/docs` — reachable from the
sidebar or ⌘K — so an operator never has to leave the product to learn what
it does.

## What it is, and who it's for

AgentOS is for teams who want autonomous agents against systems of record and
have to answer, afterwards: who authorized this, what did it cost, and what did
it touch. Four things come in one box:

- **Governed model access.** Every model call — from a human in the console, an
  OpenAI-compatible client, an agent, or the eval judge — crosses a Go gateway
  that authenticates a virtual key, enforces per-key and per-org budgets and
  rate limits, screens for prompt injection, and writes an audit record.
- **Durable agents.** A Python/LangGraph runtime with Postgres checkpointing:
  resumable threads, reviewed in-repo skills, standing operators, an eval-gated
  self-improvement loop, and a council of model-bound agents that reports its
  dissent instead of averaging it away.
- **Legacy systems as tools.** SQL, REST, SSH, SOAP, and browser connectors
  expose old systems as MCP tools, with each safety constraint enforced *inside
  the connector* — read-only SQL transactions, GET-only REST, an SSH command
  allowlist, a browser domain allowlist — rather than asked of the model.
- **Untrusted code, contained.** A Rust sandbox executes agent-written Python
  in an isolated process group with no network egress and no published host
  port.

One rule holds all of it together, stated once: **the runtime never holds a
provider credential.** Every model call it makes goes back through the gateway
on a virtual key, so an agent's spend is budgeted and audited exactly like a
human's.

## Concepts & Glossary

The documentation assumes a small, specific vocabulary: planes (gateway,
runtime, sandbox, connector, console), credentials (virtual key, user token,
SCIM token, webhook token), governance primitives (budget hold, rate limit,
guardrail, audit log, RBAC, HITL), and agent primitives (thread, profile,
skill, operator, council, proposal, eval gate). Each term is defined once, in
one place, and every other page links back to it rather than redefining it.

Read [`docs/concepts.md`](docs/concepts.md) first if you are new — it is short,
and the rest of the docs are much faster afterwards.

## Quickstart

Prereqs: Docker + Compose, and either a provider API key or a local Ollama.

**Before anything else, two variables are mandatory:** the gateway refuses to
start without `AGENTOS_ADMIN_KEY`, and the runtime refuses to start without
`AGENTOS_RUNTIME_AUTH_TOKEN` — the shared bearer every runtime route requires,
except `GET /healthz` and `/operators/webhooks/{token}`, where the `whk-` token
in the path is itself the credential. `deploy/.env.example` ships placeholder
values for both so a local bring-up works; change them for anything that is not
your laptop.

### Path 1 — Compose with a provider key

```bash
cd deploy
cp .env.example .env        # set AGENTOS_ANTHROPIC_API_KEY (or AGENTOS_OPENAI_API_KEY)
cd ..
make up                     # builds and starts the default services
make smoke                  # end-to-end: the agent answers from the seeded legacy ERP database
```

Open the console at `http://localhost:3000` and sign in with your
`AGENTOS_ADMIN_KEY`. Ask the agent something yourself:

```bash
curl -X POST http://localhost:8000/runs \
  -H "Authorization: Bearer $AGENTOS_RUNTIME_AUTH_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"input": "Which customers in Riyadh have unpaid invoices, and for how much?"}'
```

Then watch the governance side of the same request:

```bash
curl http://localhost:8080/admin/usage -H "Authorization: Bearer $AGENTOS_ADMIN_KEY"
```

The demo ships a seeded "legacy ERP" Postgres database (customers, orders,
invoices) the agent can reach *only* through the SQL connector's read-only MCP
tools — the same path a real system of record would take. Fixtures are
described in [`docs/operations.md`](docs/operations.md).

### Path 2 — Local Ollama, $0, no third-party calls

A bare `make up` is **not** the $0 path: the shipped defaults point
`AGENTOS_MODEL` and `AGENTOS_JUDGE_MODEL` at Anthropic, and even the
already-local `AGENTOS_EMBED_MODEL` default cannot resolve without an Ollama
base URL. Set all four in `deploy/.env` before starting:

```dotenv
AGENTOS_MODEL=ollama/qwen3.6:latest
AGENTOS_EMBED_MODEL=ollama/bge-m3
AGENTOS_JUDGE_MODEL=ollama/qwen3.6:latest
AGENTOS_OLLAMA_BASE_URL=http://host.docker.internal:11434
```

With those set, `make up` and every `make smoke*` target run without a single
third-party call. `make smoke9` runs a real five-model council against the
seeded ERP database at $0.

**An honest note about the local models.** The five local council members run
the `react` profile, not `deep`: on `deep`, local models exhaust the recursion
limit inside deepagents' own graph and answer nothing, while on `react` they
call tools reliably. Of the five, four (`qwen3.5`, `qwen3.6`, `gemma4`,
`gemma4:31b`) call tools and answer consistently; `gemma3` is flaky
(intermittent 502s), which the council tolerates by design — quorum and failure
isolation mean one or two bad members never deny the verdict.

Every environment variable is catalogued in
[`docs/configuration.md`](docs/configuration.md).

## Architecture

Four planes, wired together by the credential invariant: the **gateway** is the
only component that ever holds a provider key; the **runtime** runs agents and
borrows model access from the gateway on a virtual key; the **sandbox**
executes untrusted code with no egress; and **connectors** front one legacy
system each as MCP tools. Postgres is the only stateful store — gateway state,
agent checkpoints, and pgvector embeddings all live there.

The diagram at the top of this page is the whole system; the request flow
through it — the fixed order of auth, rate limit, budget hold, guardrail,
upstream, audit — is spelled out step by step, including which failures are
recorded and which are not, in
[`docs/architecture.md`](docs/architecture.md#request-flow).

## Capability tour

### Gateway — governed model access

An OpenAI-compatible edge (`:8080`) that fronts all model traffic plus the
admin plane. Virtual keys carry their own org, budget, and rate limit; budget
holds are reserved before the upstream call so concurrent requests cannot
overspend a cap; the guardrail screens prompts in `log`, `block`, or `model`
mode; and the audit log records what ran. Authorization is deliberately *not*
uniform: RBAC gates `/admin/*` only, SCIM routes use a static shared-secret
bearer, and the OIDC login flow is public by necessity.

→ [Request flow](docs/architecture.md#request-flow) ·
[Gateway API](docs/api.md#gateway-api)

### Runtime — agents that survive a restart

The Python service (`:8000`) that runs the agent loop over durable, resumable
threads. It hosts reviewed in-repo skills (`SKILL.md` files loaded only from
the image-baked directory, each recording a sha256 for provenance), standing
**operators** fired by interval, cron, or webhook, an eval-gated
self-improvement loop where no proposal activates without a human, and the
**council**: N model-bound members answering one objective in parallel with a
judge that reports dissent explicitly. Autonomy is off by default. The council's
five frontier members ship disabled, because their endpoints, model ids, and
prices in `deploy/providers.json` are **unverified placeholders** by that file's
own admission — and since those prices drive budget enforcement, an operator
must confirm each one before enabling it.

→ [Runtime API](docs/api.md#runtime-api)

### Sandbox — untrusted code, contained

The Rust service backing the agent's `run_python` tool: a per-run temp workdir,
a cleared environment, a fresh process group killed on timeout, CPU/memory/file
rlimits, a read-only rootfs with all capabilities dropped, an internal-only
network with no outbound route, and no published host port — only the runtime
can reach it. Helm ships a matching NetworkPolicy.

→ [`sandbox/README.md`](sandbox/README.md) for the isolation layers and the
residual risk they do not cover

### Connectors — legacy systems as tools

Five connectors, one legacy system each, every safety constraint enforced in
code: `sql` (read-only transactions, row caps, statement timeout), `rest`
(GET-only unless explicitly opened, SSRF-screened), `ssh` (basename allowlist,
exec-capable-binary deny-list, chaining rejection), `soap` (WSDL 1.1, operation
allowlist), and `browser` (domain allowlist, no form-submission surface).

**They are not equally deployed, and this trips people up.** SQL and REST are
wired into `AGENTOS_MCP_SERVERS` by default and start with the stack. SOAP and
browser start under `docker compose --profile connectors`, but starting those
containers does **not** expose their tools to the agent: a tool is callable only
if its URL is in `AGENTOS_MCP_SERVERS`, which in `deploy/compose.yaml` is a
hardcoded literal with no `${}` substitution, so it must be hand-edited there.
SSH has no Compose service at all — run it standalone and wire it up yourself.

→ [Tool catalog](docs/api.md#tool-catalog) ·
[Connector tiers](docs/deployment.md#connector-tiers)

## Console

The console (`:3000`) is the operator surface: thirteen pages covering usage and
spend, key creation, the audit trail, a streaming playground with
human-in-the-loop approvals, knowledge-base documents, eval-gated improvement
proposals, council objectives, operators, and tenancy administration. Its
gateway calls hold no privileges of their own — every `/admin/*` action is
re-checked server-side against the caller's real role, so hiding a page is an
affordance and never the security boundary. It never asks you to declare your
role: it calls `GET /admin/whoami` and lets the gateway's answer drive the UI.
The runtime side is different: the runtime has no role model, and nginx injects
its bearer token on `/api/runtime/` unconditionally, requiring nothing from the
browser. The token never reaches the browser, but the console's server-side
proxy carries full runtime authority — reaching the console's port is
equivalent to holding the runtime token.

→ [`docs/console.md`](docs/console.md)

## Deploy

Docker Compose (`deploy/compose.yaml`) runs the whole platform on a laptop or a
single host, with optional overlays layered on top for human-in-the-loop
governance, OpenTelemetry, a bundled Langfuse, mock OIDC/Vault, and the
OpenClaw interop worker. For Kubernetes, `deploy/helm/agentos/` is a Helm 3
chart with per-service toggles, a bundled or external Postgres, a hardened
sandbox pod with its NetworkPolicy, and an optional console ingress. CI runs
the Go/Python/Rust/console/Helm matrix on every push plus an eval gate that
fails the build below the score threshold.

→ [`docs/deployment.md`](docs/deployment.md) ·
[`docs/operations.md`](docs/operations.md) ·
[`deploy/helm/agentos/README.md`](deploy/helm/agentos/README.md)

## Security

The platform's posture is stated as what it enforces, where it deliberately
degrades rather than fails, and what it does not cover. The gateway is the only
egress to model providers; the sandbox has no egress at all; connectors enforce
their constraints in code. Some paths fail closed (the runtime without its auth
token, an unclassified tool treated as a write, SSH host-key verification) and
some fail open on purpose (a guardrail classifier outage, which leaves its own
audit entry) — the full matrix, the residual risks, and the record of a
pre-autonomy security assessment and its hardening are in one place.

→ [`SECURITY.md`](SECURITY.md)

## Repository layout

| Directory | Language | Purpose |
|---|---|---|
| `gateway/` | Go | LLM gateway: virtual keys, budgets, rate limits, guardrails, RBAC, audit |
| `runtime/` | Python | LangGraph agent runtime, skills, operators, council, MCP client |
| `sandbox/` | Rust | Isolated, egress-less execution of untrusted Python |
| `connectors/sql/` | Go | Legacy Postgres as read-only MCP tools |
| `connectors/rest/` | Go | OpenAPI services as MCP tools |
| `connectors/ssh/` | Go | Legacy hosts as allowlisted MCP commands |
| `connectors/soap/` | Go | WSDL 1.1 services as MCP tools |
| `connectors/browser/` | Python | Allowlisted browsing as MCP tools (Playwright) |
| `console/` | TypeScript | Operator console |
| `landing/` | HTML/CSS | Public landing page |
| `deploy/` | — | Docker Compose, Helm chart, CI mocks, demo fixtures |
| `docs/` | — | Documentation |
| `scripts/` | Bash | End-to-end smoke tests |

## Development

```bash
make test        # test-go + test-python + test-console (no network, no Docker needed)
make test-rust   # the Rust sandbox suite
make fmt         # gofmt + ruff format
```

**Two traps worth knowing before you trust a green run:**

1. `make test` **excludes** `make test-rust`. The sandbox suite is not part of
   the aggregate target and has to be run separately.
2. `make test-go` covers **only `gateway/` and `connectors/sql/`**. The REST,
   SOAP, SSH, and browser connector suites are not wired into any `make`
   target — run them from their own directories with each language's own test
   command.

The `make smoke*` targets are live end-to-end checks against a running stack;
what each one proves, the ordering dependency between them, the demo fixtures,
and the CI test doubles are all in
[`docs/operations.md`](docs/operations.md).

## Roadmap

Shipped work is recorded in [`CHANGELOG.md`](CHANGELOG.md). Next:

- SAML SSO
- Cloud-KMS secret backends
- Redis limiter option
- Secret rotation webhooks
- SCIM Groups
- Restore the CI and eval-gate status badges once the repository is public
  (they were removed because they cannot resolve against a private repo)

## Documentation

- [`docs/concepts.md`](docs/concepts.md) — the vocabulary everything else assumes
- [`docs/architecture.md`](docs/architecture.md) — the four planes, ports, and request flow
- [`docs/api.md`](docs/api.md) — gateway, runtime, and sandbox endpoints; the tool catalog
- [`docs/console.md`](docs/console.md) — the operator surface, page by page
- [`docs/configuration.md`](docs/configuration.md) — every environment variable
- [`docs/deployment.md`](docs/deployment.md) — Compose, overlays, connector tiers, Helm, CI
- [`docs/operations.md`](docs/operations.md) — verification targets, fixtures, CI mocks
- [`docs/interop/openclaw.md`](docs/interop/openclaw.md) — running an OpenClaw worker under governance
- [`SECURITY.md`](SECURITY.md) — enforcement, failure modes, residual risk, reporting
- [`CHANGELOG.md`](CHANGELOG.md) — what shipped, in order

### Taxonomy map

Both this README and the in-console Docs tab organize around the same twelve
sections. This README orders a few of them (notably Quickstart) earlier for
onboarding; the table below gives the canonical taxonomy order and maps each
name to where it actually lives, so the console surface can mirror it exactly:

| # | Section | Where it lives |
|---|---|---|
| 1 | Overview | README — hero + [What it is, and who it's for](#what-it-is-and-who-its-for) |
| 2 | Concepts & Glossary | README — [Concepts & Glossary](#concepts--glossary); [`docs/concepts.md`](docs/concepts.md) |
| 3 | Architecture | README — [Architecture](#architecture); [`docs/architecture.md`](docs/architecture.md) |
| 4 | Gateway | README — [Capability tour → Gateway](#gateway--governed-model-access) |
| 5 | Runtime | README — [Capability tour → Runtime](#runtime--agents-that-survive-a-restart) |
| 6 | Sandbox | README — [Capability tour → Sandbox](#sandbox--untrusted-code-contained); [`sandbox/README.md`](sandbox/README.md) |
| 7 | Connectors | README — [Capability tour → Connectors](#connectors--legacy-systems-as-tools); `connectors/*/README.md` |
| 8 | Console | README — [Console](#console); [`docs/console.md`](docs/console.md) |
| 9 | Quickstart | README — [Quickstart](#quickstart) |
| 10 | Configuration | [`docs/configuration.md`](docs/configuration.md) |
| 11 | Deploy | README — [Deploy](#deploy); [`docs/deployment.md`](docs/deployment.md), [`docs/operations.md`](docs/operations.md) |
| 12 | Security | README — [Security](#security); [`SECURITY.md`](SECURITY.md) |

License: [Apache-2.0](LICENSE)
