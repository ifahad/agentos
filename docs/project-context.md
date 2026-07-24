# AgentOS — full project context

A single-document briefing for an AI model or engineer with **zero prior context**.
Everything here was verified against the repository on 2026-07-24 (62 commits).

---

## 1. What this is

**AgentOS** is an open-source, self-hostable **agentic operating layer**:
any LLM provider in, any legacy system out, with governed autonomous agents in
between. It is not a chatbot and not a framework — it is the operational plumbing
an enterprise needs to run autonomous agents in production:

- **Governed model access** — every LLM call flows through a Go gateway with
  virtual API keys, budget caps, rate limits, prompt-injection guardrails, usage
  accounting, and an audit trail.
- **Deep agents** — a Python runtime on LangGraph with Postgres checkpointing:
  durable, resumable, thread-scoped agent runs.
- **Legacy systems as tools** — old systems (SQL, REST, SOAP, SSH, browser) are
  wrapped as MCP servers by Go connectors, so any agent can use them safely.
- **Eval-gated self-improvement** — the agent proposes better prompts; they ship
  only when evals pass **and** a human approves.

Design goals, in the owner's words: *hookable to any native/old/legacy system,
able to learn/act/decide/improve itself, with loop and harness engineering, fully
open source.* Languages chosen deliberately: **Go** (gateway, connectors),
**Rust** (sandbox), **Python** (agent runtime), **TypeScript** (console).

**Repo:** `ifahad/agentos` — **PRIVATE** (explicit owner decision; license is
Apache-2.0 with intent to open later). Local path `/home/iofahd/code/agentos`,
branch `main`.

---

## 2. Architecture

```
                         ┌──────────────────┐
   browser ──────────────►  console (TS)    │  :3000  React 19 SPA, nginx
                         │  same-origin      │         proxies /api/*
                         └────────┬─────────┘
                                  │
              /api/gateway/*      │      /api/runtime/*
                    ┌─────────────┴─────────────┐
                    ▼                           ▼
        ┌────────────────────┐        ┌────────────────────┐
        │  gateway (Go)      │◄───────│  runtime (Python)  │
        │  :8080             │  all   │  :8000             │
        │  OpenAI-compatible │  model │  LangGraph agent   │
        │  + admin + SCIM    │  calls │  + MCP client      │
        └─────────┬──────────┘        └─────────┬──────────┘
                  │                             │ MCP (streamable-http)
                  ▼                             ▼
        Anthropic / OpenAI /            ┌──────────────────┐
        Ollama / (registry)             │ connectors (Go)  │
                                        │ sql :8090        │
                                        │ rest :8091       │
                                        │ soap :8093       │
                                        │ ssh :8092        │
                                        │ browser :8094    │
                                        └────────┬─────────┘
                                                 ▼
                                        legacy systems
                    ┌────────────────────┐
                    │  sandbox (Rust)    │  :8070, internal network only
                    │  run_python tool   │  no egress, no host port
                    └────────────────────┘
                    ┌────────────────────┐
                    │  postgres 16       │  :5432 — app data, checkpoints,
                    │  + pgvector        │  eval runs, proposals, pgvector KB
                    └────────────────────┘
```

**The invariant:** the runtime has **no provider credentials**. It talks only to
the gateway using a virtual key. Every token the platform spends is therefore
attributable, budgetable, and auditable — including the eval judge and the
guardrail classifier, which route through the gateway like everything else.

---

## 3. Services

| Service | Lang | Port (default / local override) | Purpose |
|---|---|---|---|
| `gateway` | Go 1.25 | 8080 | OpenAI-compatible proxy, virtual keys, budgets, rate limits, guardrails, audit, RBAC, OIDC SSO, SCIM, secrets |
| `runtime` | Python 3.12 | 8000 / **18000** | LangGraph agent, MCP client, checkpointing, HITL, RAG, evals, self-improvement |
| `sql-connector` | Go | 8090 | Legacy SQL as read-only MCP tools |
| `rest-connector` | Go | 8091 | OpenAPI/REST as MCP tools |
| `ssh-connector` | Go | 8092 | Legacy shells as MCP tools (opt-in) |
| `soap-connector` | Go | 8093 | WSDL/SOAP as MCP tools (opt-in) |
| `browser-connector` | Go | 8094 | Playwright browsing/computer-use (opt-in) |
| `sandbox` | Rust | 8070 | Isolated `run_python`; **no host port**, internal network only |
| `console` | TS/React 19 | 3000 / **13000** | Admin UI |
| `postgres` | — | 5432 / **15432** | pgvector-enabled database |
| `demo-crm` | Go | 8095 / **18095** | Stdlib demo REST service for the REST connector |

Local port overrides live in `deploy/.env` (gitignored) because 5432, 8000,
3000, and 8095 are occupied on the dev host.

Opt-in connectors start with `docker compose --profile connectors up -d`.
Overlays: `compose.hitl.yaml`, `compose.otel.yaml`, `compose.langfuse.yaml`,
`compose.auth-mocks.yaml`, `openclaw/compose.openclaw.yaml`.

---

## 4. The gateway (Go) — the governance chokepoint

`gateway/internal/` packages: `server`, `provider`, `store`, `rbac`, `secret`,
`guardrail`, `ratelimit`, `oidc`, `scim`, `telemetry`.

**Request path for `POST /v1/chat/completions`** — order matters and is enforced:

```
authenticate key  →  guardrail screen  →  rate limit  →  key budget
   →  org budget  →  route by model prefix  →  forward  →  record usage + audit
```

**Model routing** is by prefix: `anthropic/claude-sonnet-5`, `openai/gpt-4o-mini`,
`ollama/qwen3.6:latest`. Unknown prefixes return `ErrUnknownProvider`.

**Endpoints**

| Group | Routes |
|---|---|
| OpenAI-compatible | `POST /v1/chat/completions` (SSE streaming supported), `POST /v1/embeddings` |
| Admin | `POST/GET /admin/keys`, `GET /admin/usage`, `GET /admin/audit`, `GET /admin/whoami`, `POST/GET /admin/orgs`, `PATCH /admin/orgs/{id}`, `POST/GET/DELETE /admin/orgs/{id}/users[/{uid}]`, `GET /admin/secrets/status`, `POST /admin/secrets/reload` |
| SSO | `GET /auth/oidc/{status,login,callback}` |
| SCIM 2.0 | `GET/POST/PUT/PATCH/DELETE /scim/v2/Users[/{id}]`, `GET /scim/v2/{ServiceProviderConfig,Schemas,ResourceTypes}` |
| Health | `GET /healthz` |

**Identity and tokens**
- `agos-…` — **virtual API keys** (machine callers; carry budget + org)
- `agu-…` — **user tokens** (humans; carry role + org; minted by OIDC callback)
- `AGENTOS_ADMIN_KEY` — root superuser, global scope
- SCIM bearer token — IdP provisioning only

**RBAC**: orgs × users × roles `owner | admin | member | viewer`. The admin API
scopes keys, usage, and audit to the caller's org. Backward compatible: pre-RBAC
keys live in a bootstrapped `default` org with budget 0 = unlimited.

**Budgets**: per-key monthly cap → `402 budget_exceeded`; per-org aggregate cap →
`402 org_budget_exceeded`. Prices come from a static table; **unknown models cost
$0**, so local Ollama models never trip a budget (this matters — see §11).

**Rate limits**: per-org token bucket, `rate_limit_rpm` (0 = unlimited) →
`429 rate_limited` + `Retry-After`. Backend `memory` (per-instance) or
`postgres` (atomic SQL, shared across replicas).

**Guardrails**: `AGENTOS_GUARDRAILS_MODE = off | log | block | model`. `model`
runs the fast heuristic first, then an LLM classifier through the gateway.
Classifier outages **fail open** with a `guardrail_error` audit entry, so the
safety layer can never take down traffic.

**Secrets**: `AGENTOS_SECRETS_BACKEND = env | file | age | vault`. Provider keys
resolve through a live `secret.Source` on **every** request, so rotation
(`POST /admin/secrets/reload` or the refresh poll) reaches upstream with no
restart. `GET /admin/secrets/status` reports presence and source, never values.

---

## 5. The runtime (Python) — the agent loop

`runtime/src/agentos_runtime/`: `api.py`, `agent.py`, `hitl.py`, `context.py`,
`evals.py`, `improve.py`, `prompts.py`, `store.py`, `sandbox.py`, `messages.py`,
`otel.py`, `config.py`.

**Endpoints** (all behind an app-wide auth dependency except `GET /healthz`):

```
POST /runs                      run to completion (202 when HITL pauses)
POST /runs/stream               SSE: step | pending_approval | done | error
POST /runs/{thread_id}/approve  approve/deny a paused tool batch
GET  /threads/{thread_id}       thread state
POST/GET /documents             knowledge base (pgvector)
POST /evals/run, GET /evals/runs
POST /improve, GET /proposals, POST /proposals/{id}/approve
GET  /prompts/active
GET  /healthz
```

**Two agent profiles**, selected by `AGENTOS_AGENT_PROFILE`:
- `react` (default) — LangGraph `create_react_agent`; supports HITL via
  `interrupt_before=["tools"]` when `AGENTOS_APPROVAL_TOOLS` is non-empty
- `deep` — `deepagents.create_deep_agent`; **no `interrupt_before` pass-through**,
  so tool-approval HITL is react-only (documented deviation at `agent.py:90`)

**Durability**: `AsyncPostgresSaver` keyed by `thread_id` (falls back to
`InMemorySaver` with no checkpoint DB). Runs survive restarts and are resumable.

**Safety**: `SAFETY_PREAMBLE` in `agent.py:30` is **immutable** and always
prepended to the effective system prompt, so a hot-swapped prompt proposal can
never drop the safety frame. It instructs the model to treat retrieved documents
and tool output as **untrusted data, never instructions**.

**Context engine** (`context.py`): LlamaIndex `SentenceSplitter` chunking +
pgvector retrieval exposed as a `search_knowledge` tool; embeddings route through
the gateway (`ollama/bge-m3` by default).

**Self-improvement loop**: run eval suite → reflect on failures → propose a new
system prompt → auto-evaluate candidate vs baseline → **human approves** →
hot-swap. Proposals never self-activate. Eval cases may carry an optional
`judge: {criteria, threshold}` block graded by an LLM judge; a judge outage fails
that case, never the suite.

---

## 6. Connectors, sandbox, observability

**Connectors** are Go MCP servers (streamable-HTTP). Hardening highlights:
- **SQL** — read-only enforced, real `READ ONLY` transaction, row caps
- **REST/SOAP** — `safehttp` strips the auth header across redirects and refuses
  private/loopback/link-local targets (SSRF)
- **SSH** — command allowlist **plus** a deny-list of exec-capable binaries
  (`find`, `awk`, `xargs`, `tar`, `git`, `perl`, `python`, `bash`, …), blocked
  chaining/redirection characters, `known_hosts` required
- **Browser** — Playwright with a domain allowlist

**Sandbox (Rust)**: per-run tempdir, `env_clear`, `setpgid`, `setrlimit`
(CPU/AS/NPROC/FSIZE), process-group SIGKILL — inside a container that is
read-only, all caps dropped, on an `internal: true` network with **no host port
and no outbound route**. Helm ships a matching NetworkPolicy.

**Observability**: OpenTelemetry spans from gateway and runtime → collector →
optional bundled Langfuse. Overlays only; off by default.

**Kubernetes**: `deploy/helm/agentos` — per-service toggles, bundled or external
Postgres, hardened sandbox pod, optional console ingress.

**CI**: GitHub Actions 5-job matrix (Go, Python, Rust, console, Helm) plus an
**eval gate** scoring the runtime suite against a deterministic mock model,
failing under 0.8.

---

## 7. The console (TypeScript) — read this section for UI work

**Stack, deliberately minimal:** React **19**, TypeScript 5.8, Vite 6, Vitest 3.
Runtime dependencies are only `react`, `react-dom` and `framer-motion` — no
router, no state library, no component library, no CSS framework. Routing is
hand-rolled in `App.tsx` over a `Route[]` array; styling is hand-written CSS
with tokens in `console/src/styles.css` plus co-located per-page sheets.

**Design language — "Instrument".** The one rule that governs every visual
decision: **chroma is reserved for machine state.** There is no brand accent. If
something is colored it is `--live` (in flight), `--ok` (allowed), `--hold`
(awaiting a human) or `--deny` (refused); navigation, buttons, selection, links
and charts at rest are graphite and ink only. A quiet screen is a healthy one.
Adding a decorative accent would make the signals lie — don't.

Type is self-hosted, never a CDN, because a font request to a third party would
leak console usage off-box: **Archivo** for interface text, **IBM Plex Mono**
for every machine-produced value (ids, counts, money, models, timestamps).
Icons are hand-drawn schematic marks in `src/ui/icons.tsx` on a 16px grid — no
icon library. The signature element is the **governance chain** in the topbar,
which renders how far the most recent request actually got through
auth → rbac → budget → rate → audit, derived from recorded audit statuses in
`src/lib/chain.ts` rather than from a timer.

```
console/src/
  App.tsx                 nav + hand-rolled routing + identity/whoami wiring
  main.tsx                mount
  styles.css              tokens + shell + shared element styling
  fonts/                  self-hosted Archivo + IBM Plex Mono (OFL, woff2)
  ui/
    icons.tsx             the AgentOS icon set (hand-drawn SVG)
    Button/Card/Stat/Table/Badge/Modal/Toast/Skeleton + motion presets
  charts/                 hand-rolled SVG sparkline / usage / spend + transforms
  components/
    Chain.tsx             the governance chain (signature element)
    common.tsx            shared primitives (errorMessage, etc.)
    Sidebar.tsx           nav shell with icons and the live dot
    SettingsModal.tsx     admin key / identity settings
  lib/                    pure logic, each with a .test.ts sibling
    api.ts                typed same-origin client + buildRequest()
    types.ts, format.ts, rbac.ts, sse.ts, sso.ts, improve.ts, provisioning.ts
  pages/
    Overview.tsx  Keys.tsx  Audit.tsx  Playground.tsx  Documents.tsx
    Improve.tsx   Orgs.tsx  Users.tsx  Secrets.tsx     Provisioning.tsx
```

**Navigation** (`App.tsx`), with capability-gated visibility:

| Path | Label | Visible when |
|---|---|---|
| `/` | Overview | always |
| `/keys` | Keys | always |
| `/audit` | Audit | always |
| `/playground` | Playground | always |
| `/documents` | Documents | always |
| `/improve` | Improve | always |
| `/orgs` | Orgs | `org.view` |
| `/users` | Users | `user.view` |
| `/secrets` | Secrets | `secret.view` |
| `/provisioning` | Provisioning | `provisioning.view` |

**Networking model — important.** The SPA never talks to a service directly and
never needs CORS. All calls go same-origin through `/api/gateway/...` and
`/api/runtime/...`:
- **dev**: Vite proxy rewrites the prefixes (`vite.config.ts`)
- **prod**: nginx `proxy_pass` to `gateway:8080` / `runtime:8000`, and — this is
  a security control — **nginx injects the runtime bearer token server-side** on
  `/api/runtime/` only, so the browser never holds it.

**Testing convention**: request construction lives in pure `buildRequest`-style
functions in `lib/`, unit-tested with **no DOM and no network** (137 vitest
tests). Page components stay thin. Preserve this split in any redesign.

**Capabilities** (`lib/rbac.ts`): `org.create`, `org.view`, `user.invite`,
`user.remove`, `user.view`, `key.create`, `key.view`, `usage.view`,
`secret.view`, `provisioning.view`, `agent.run`. Root is superuser; `viewer` is
read-only.

**Identity flow**: `GET /admin/whoami` drives role-aware UI. SSO sends the
browser to `/api/gateway/auth/oidc/login`; the gateway 302s back with the token
in the URL **fragment**, parsed by `lib/sso.ts`.

**Access from another machine**: the console is reachable from a laptop to the
dev host at `http://<host>:13000` (LAN or Tailscale, or `ssh -L 13000`).

---

## 8. Security posture

A four-audit pre-autonomy security review is recorded at
`docs/security/2026-07-21-security-assessment.md`. It found a real
**unauthenticated-RCE chain** and other issues; **all were fixed and verified
live** before any autonomy work proceeded:

| Finding | Fix |
|---|---|
| Runtime had **no auth on any route** + a published port | `AGENTOS_RUNTIME_AUTH_TOKEN`, fail-closed at startup; console injects it via nginx |
| SSH allowlist bypassable to RCE via LOLBins (`find -exec`, `awk system()`) | Deny-list of exec-capable binaries, blocked chaining chars, `known_hosts` required |
| Prompt hot-swap unvalidated | Immutable `SAFETY_PREAMBLE` always prepended |
| REST/SOAP leaked the auth header across redirects → SSRF to metadata | `safehttp` `CheckRedirect` + private-IP refusal |
| OIDC accepted unverified emails; role adoption takeover | `email_verified` required; match on stable `sub` |
| Cross-org usage/spend/audit leak via key **name** scoping | Keyed by `secret_hash` + `org_id`, with migration |
| Secret rotation was a silent no-op for provider keys | Router reads the live `secret.Source` per call |

Confirmed sound in the audit: `crypto/rand`, hashed tokens, verified JWTs, no
SQL injection, SQL read-only holds, SOAP not XXE-vulnerable, sandbox isolation.

**Known open backlog** (tracked, not yet done): budget TOCTOU, `http.MaxBytesReader`,
SQL `statement_timeout`, browser IP backstop, runtime non-root image + K8s
`securityContext`, HTTP server timeouts + graceful shutdown, audit retention,
plus efficiency items (an N+1 in `handleListOrgs`, duplicate per-request org
lookups, unbuffered proxy responses).

**OpenClaw interop** (`docs/interop/openclaw.md`): a recipe for running the
third-party OpenClaw autonomous agent as a **governed, jailed worker** — model
traffic through the gateway, execution jailed to the sandbox or hardened SSH,
skills screened from a read-only vetted directory (never ClawHub auto-fetch),
non-root/read-only/unexposed deployment. It cites the documented ClawHub
supply-chain problem (~36% of skills carrying prompt injection; 341+ malicious
skills found). Honest limits: the WebSocket control plane and messaging channels
are **not** bridged.

---

## 9. Delivery history

Phases 1–7 shipped and were each **smoke-tested live end-to-end**:

1. Core loop — gateway + runtime + SQL connector + compose demo
2. Operability — console, SSE streaming, guardrails, HITL, LlamaIndex/pgvector, deepagents profile, OTel
3. Autonomy safely — Rust sandbox, eval-gated self-improvement, REST connector + demo CRM, Helm
4. Reach & hardening — SSH connector, egress-less sandbox, model guardrail, LLM-judge evals, Langfuse
5. Enterprise governance — multi-tenant RBAC, secrets backends, SOAP + browser connectors, CI + eval gate
6. Enterprise identity — OIDC SSO, Vault backend, per-tenant rate limits, `whoami`
7. Scale & provisioning — SCIM 2.0, distributed rate-limit store, secret rotation/reload

Phase 8 was **redirected from autonomy to security hardening** at the owner's
explicit demand for a full assessment before further implementation. That
hardening shipped (see §8).

Test totals: ~148 Go gateway test functions (+31 SOAP, +17 browser, +49 SSH),
76+ pytest, 137 vitest, 11 Rust.

---

## 10. In flight — the "Multiverse" feature (designed, not implemented)

Approved design and a 16-task implementation plan exist; **no code is written**.

- Spec: `docs/superpowers/specs/2026-07-24-multiverse-council-design.md`
- Plan: `docs/superpowers/plans/2026-07-24-multiverse-council.md`

**Concept:** run one objective through N independent deep agents, each bound to a
different frontier model (Kimi K3 / Moonshot, GLM-5.2 / Zhipu, Qwen 3.8 Max /
Alibaba, DeepSeek-V4 Pro, MiniMax M3), then have a judge synthesize **one verdict
plus an explicit dissent report**. The council is the autonomous unit: a
heartbeat pulls objectives from a queue and runs cycles under cycle caps, a spend
ceiling, and a kill switch, where **reads run freely and every write becomes a
human-approved proposal**.

It requires three prerequisites that do not exist today: a **config-driven
provider registry** (routing is a hardcoded 3-prefix switch), **real pricing**
(unknown models cost $0, so budgets are inert for new vendors), and a
**multi-agent runtime** (one agent is built from one `AGENTOS_MODEL`). It also
closes two harness gaps: no per-run cycle cap and no provider retry/fallback.

It is additionally exposed as `council/multiverse`, an OpenAI-compatible model,
so any client — including the governed OpenClaw worker — gets the whole council
behind one model name, with two independent recursion guards.

**New console surface it defines** (Task 14, the likely UI/UX target): a
**Multiverse** page with a member grid (model · enabled · latency · error rate ·
spend · agreement rate), an objective timeline with per-cycle agreement, a
verdict view showing dissent beside the synthesized answer, a global pause/resume
kill switch, and a proposals strip with approve controls. Its API contract lives
in a new `console/src/lib/council.ts`, independent of visual direction.

---

## 11. Environment quirks and hard-won gotchas

These caused real, time-consuming failures. Respect them.

1. **Compose only injects env vars declared in a service's `environment:` block.**
   `X=y docker compose up` does nothing unless `X` is listed there. This silently
   broke guardrail wiring for a whole debugging session.
2. **`UID` is a readonly bash builtin** — never assign to it in scripts.
3. **Single-quoted `curl -d '{…$VAR…}'` does not expand shell variables.** Use
   double quotes with escaped `\"`. Smoke tests need unique per-run identifiers
   (`$$` suffix) or SCIM/RBAC creates return 409 on rerun.
4. **`grep -q` on a live `docker compose logs` pipe under `pipefail` SIGPIPEs**
   and yields false negatives — capture logs to a variable first.
5. **Rate-limit tests must fire the burst concurrently.** Sequential curls let the
   token bucket refill during slow (5–20s) Ollama calls, so the limit never trips.
6. **Reasoning models make bad fast classifiers.** `qwen3.6` spends its token
   budget thinking and returns an empty verdict, which fails open. Use a
   non-reasoning classifier, and tune `AGENTOS_GUARDRAILS_{MODEL,TIMEOUT_S,MAX_TOKENS}`.
7. **Budgets can't be exercised with local Ollama** — those models are $0 in the
   price table, so spend never exceeds any cap. Unit-test budgets instead.
8. **`present: true` must mean non-empty** — compose passes `${VAR:-}` as
   set-but-empty and `os.LookupEnv` reports it present.
9. **`zsh` eats `?` in unquoted URLs** and mangles JSON in `echo`. Quote URLs; use
   heredocs or files.
10. **Sandbox builder must stay `rust:1-bookworm`** (glibc match; `rust:1-slim`
    produced a `GLIBC_2.39 not found` crash).

**Dev host specifics:** Go 1.24.5 lives at `~/.local/go/bin` and is **not on
PATH** (`export PATH=$HOME/.local/go/bin:$PATH`). There are **no Anthropic or
OpenAI API keys** on this machine — local **Ollama** at `:11434` is the working
provider (`qwen3.6` does tool calling). Available local models: `qwen3.6`,
`qwen3.5`, `gemma4:31b`, `gemma4`, `gemma3`, `bge-m3` (embeddings). Two
`deepseek-v4-*:cloud` models are pulled but **subscription-gated**.

---

## 12. Conventions

- **TDD**: write the failing test, watch it fail, implement minimally, watch it
  pass, commit. Every task ends with a commit.
- **Tests**: `cd gateway && go test ./...` · `cd runtime && uv run pytest` ·
  `cd console && npm test` · `make test` for Go+Python · `make fmt` for
  gofmt+ruff.
- **Live verification**: `scripts/smoke*.sh` (one per phase) exercise the real
  stack end-to-end. Claims of completion are expected to be backed by an actual
  run, not inference.
- **Docs**: designs in `docs/superpowers/specs/YYYY-MM-DD-<topic>-design.md`,
  plans in `docs/superpowers/plans/`.
- **Style**: match surrounding code — comment density, naming, idiom. Go is
  stdlib-first (the SOAP connector uses `encoding/xml`, not a library; the
  gateway has no YAML dependency). Python is typed and docstringed. The console
  keeps logic in pure, unit-tested `lib/` functions.
- **Security defaults are fail-closed**: unknown tool ⇒ write-class; unresolvable
  host ⇒ disallowed; missing runtime token ⇒ refuse to start; provider with no
  key ⇒ disabled, not attempted.

---

## 13. Honest gaps

Stated plainly so nothing here reads as more finished than it is:

- **No provider retry/fallback** in the gateway — one upstream 502 ends a run.
- **No explicit per-run cycle cap** in the runtime — it relies on LangGraph's
  default `recursion_limit` of 25.
- **No conversation summarization/trimming** — context management is RAG +
  checkpointer persistence only.
- **HITL is off by default** (`AGENTOS_APPROVAL_TOOLS` empty); tools run
  unattended unless opted in.
- **No always-on autonomy** — runs are request-driven. The heartbeat/objectives
  loop is designed (Multiverse) but not built.
- **Deep profile has no HITL** — deepagents exposes no `interrupt_before`.
- **Budget enforcement is untested end-to-end against paid models**, because this
  host has no paid provider keys.
- The security backlog in §8 is open.
