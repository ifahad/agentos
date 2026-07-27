# W1 — GitHub Doc Overhaul Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace AgentOS's phase-by-phase README and internally-written briefing with a navigable, concept-first documentation set (README + eight `docs/` files + `SECURITY.md` + `CHANGELOG.md`) organized around one canonical capability taxonomy, correcting every known factual inaccuracy.

**Architecture:** Prose only — no code, no behavior change. Each task produces one or a small group of related Markdown files, verified against source with grep/read commands, then committed. Fact-carrying content (ports, endpoints, env vars, tool catalog, fail-open matrix) is supplied verbatim in this plan because that is where documentation errors originate; connective prose is the implementer's to write. Build order runs leaf-first (glossary, architecture, api, config) so the README, which links everything, is written last against files that already exist.

**Tech Stack:** Markdown (GitHub-flavored). No build step, no linter, no generator. Verification is grep + manual read-through.

## Global Constraints

Copied from `docs/superpowers/specs/2026-07-27-platform-docs-and-console-docs-tab.md` §4 and §6. Every task's requirements implicitly include this section.

- **Private/local:** everything commits to the current branch and merges to `main` locally. **Never push to origin.** The repo is private.
- **No behavior change:** this work-stream touches only `*.md` files plus **comment lines in `deploy/.env.example`** (Task 1 corrects one comment, adds three commented-out example vars, and retitles one block comment — no uncommented value in that file may change). It must not modify Go, Python, Rust, TypeScript, YAML manifests, or the `Makefile`.
- **Canonical taxonomy (spec §5), verbatim and in this order.** Every doc's section names and ordering derive from it:
  `Overview → Concepts & Glossary → Architecture → Gateway (governed model access) → Runtime (agents) → Sandbox → Connectors → Console (operator surface) → Quickstart → Configuration → Deploy → Security`
- **No claim from `docs/project-context.md` §8 ("Known open backlog") or §13 ("Honest gaps") may be copied into `SECURITY.md` or any new `docs/` file without independent re-verification against current source.** Both sections are known-stale; Task 2 corrects them.
- **Phase framing is removed** from the README and from every doc this plan writes or rewrites. The phase-completion history lives only in `CHANGELOG.md`. (`docs/superpowers/plans/*` and `docs/superpowers/specs/*` are historical records — leave them alone.)
- **No fabricated capabilities.** If a fact is not in this plan and not verifiable by reading source, do not write it. Every port, env var, endpoint, flag, and `make` target cited in any doc must exist in the repo.
- **Audience is external.** Dev-host specifics ("this host has no paid provider keys"), "the owner's words", and internal process narration do not appear in the new docs. `docs/project-context.md` remains the internal briefing and keeps that material.
- **Relative links only** between in-repo docs (e.g. `docs/architecture.md`, `../README.md`). No absolute GitHub URLs to this repo — it is private and the links would 404.

---

## Reference: verified facts

Every fact below was checked against source at plan time. Tasks cite this section by name. **Do not re-derive these; do not contradict them.**

### R1. Services and ports

| Service | Language | Listen port | Compose host port | Port env var | Compose default | Helm template |
|---|---|---|---|---|---|---|
| `postgres` | — | 5432 | 5432 | `AGENTOS_PG_PORT` | on | `postgres.yaml` (`enabled: true`) |
| `gateway` | Go | 8080 | 8080 | `AGENTOS_GATEWAY_PORT` | on | `gateway.yaml` (`enabled: true`) |
| `runtime` | Python | 8000 | 8000 | `AGENTOS_RUNTIME_PORT` | on | `runtime.yaml` (`enabled: true`) |
| `sandbox` | Rust | 8070 | **none — no `ports:` mapping** | `AGENTOS_SANDBOX_PORT` | on (internal-only network) | `sandbox.yaml` (`enabled: true`) + `sandbox-networkpolicy.yaml` |
| `sql-connector` | Go | 8090 | 8090 | `AGENTOS_CONNECTOR_PORT` | on | `sql-connector.yaml` (`enabled: true`) |
| `rest-connector` | Go | 8091 | 8091 | `AGENTOS_REST_PORT` | on | `rest-connector.yaml` (**`enabled: false`**) |
| `ssh-connector` | Go | 8092 (hardcoded `listenAddr`) | **no Compose service** | — | **absent** | **none** |
| `soap-connector` | Go | 8093 | 8093 | `AGENTOS_SOAP_PORT` | `--profile connectors` / `soap` | **none** |
| `browser-connector` | Python | 8094 | 8094 | `AGENTOS_BROWSER_PORT` | `--profile connectors` / `browser` | **none** |
| `demo-crm` | — | 8095 | 8095 | `AGENTOS_CRM_PORT` | on | `demo-crm.yaml` (`enabled: false`) |
| `console` | TS + nginx | 8080 (in container) | **3000** | `AGENTOS_CONSOLE_PORT` | on | `console.yaml` (`enabled: true`) |

**Trap:** `AGENTOS_SSH_PORT` is the **remote SSH target port** (default `22`), *not* the connector's listen port. The SSH connector's own listen address is the hardcoded `:8092`. Do not describe `AGENTOS_SSH_PORT` as "the connector port" anywhere.

### R2. Connector deployment tiers (spec §5; a documented gotcha, not a footnote)

- **SQL + REST** — wired into `AGENTOS_MCP_SERVERS` by default in Compose as a **hardcoded literal** (`deploy/compose.yaml`: `AGENTOS_MCP_SERVERS: http://sql-connector:8090/mcp,http://rest-connector:8091/mcp`) with **no `${}` substitution**, so it cannot be overridden from `.env`. Both have Helm templates; `restConnector.enabled` defaults to **false**.
- **SOAP + browser** — start only under `docker compose --profile connectors` (or `--profile soap` / `--profile browser`), and starting them does **not** make their tools reachable: their URLs must be appended to `AGENTOS_MCP_SERVERS` by hand-editing `deploy/compose.yaml`. No Helm templates.
- **SSH** — no Compose service, no Helm template. Run it standalone and add its URL to `AGENTOS_MCP_SERVERS` yourself.
- **Helm** composes the MCP list from enabled services and supports `runtime.extraMcpServers` (values.yaml:76, "Extra MCP server URLs appended to the composed `AGENTOS_MCP_SERVERS`").

### R3. Gateway request pipeline (spec §5; the load-bearing correction)

`POST /v1/chat/completions`:

`Auth (agos- virtual key) → Rate limit → Budget hold → Guardrail (only when AGENTOS_GUARDRAILS_MODE != off) → Upstream provider or council → Audit`

- `POST /v1/embeddings` runs the same chain **minus the guardrail**.
- Audit records outcomes as one of seven kinds (`store.go:54-70`): `chat`, `embeddings`, `guardrail_flag`, `guardrail_block`, `guardrail_error`, `rate_limited`, `secret_reload`. Among denials, **only rate-limit rejections and guardrail events are audited** — a `401` auth failure and a `402` budget exhaustion write no audit entry (verified: `server.go:493-556` `handleChatCompletions`, `server.go:618-650` `handleEmbeddings`).
- **RBAC is never evaluated on `/v1/*`.** It gates `/admin/*` only, via `agu-` user tokens and the root admin key. `/scim/v2/*` uses a separate static shared-secret bearer check (`scimAuth`, `scim.go:19-28`), not a role. `/auth/oidc/*` carries no auth wrapper at all — it is the public login/callback flow (`server.go:274-276`). Org scoping on the proxy path comes from the virtual key's own org, not a role check.
- The env var is **`AGENTOS_GUARDRAILS_MODE`** (plural `GUARDRAILS`), values `off|log|block|model`. Related: `AGENTOS_GUARDRAILS_MODEL`, `AGENTOS_GUARDRAILS_KEY`, `AGENTOS_GUARDRAILS_TIMEOUT_S`, `AGENTOS_GUARDRAILS_MAX_TOKENS`.

### R4. Fail-open / fail-closed matrix, stated per failure mode

| Control | On a *verdict* | On a *backend/classifier error* |
|---|---|---|
| Budget (per-key and per-org) | **Fail closed** — HTTP 402 `budget_exceeded` / `org_budget_exceeded` | **Fail open**, and — unlike the guardrail row below — that admission is **not** audited (`rbac.go:86-90` returns `func(){}, true` with no `recordAudit`); it reads as an ordinary success |
| Rate limit | **Fail closed** — HTTP 429 `rate_limited` + `Retry-After` | **Fail open** (Postgres backend only; the `memory` backend has no such path) |
| Guardrail | **Fail closed** — request blocked, audited | **Fail open** with a `guardrail_error` audit entry |
| OIDC `email_verified` | Fail closed | — |
| SSH host-key verification | Fail closed (see R5) | — |
| Runtime auth token | Fail closed — runtime refuses to start without `AGENTOS_RUNTIME_AUTH_TOKEN` | — |
| Council write gating | Fail closed — an unclassified tool is treated as write-class | — |

Never write the bare phrase "budget enforcement fails open". Exhaustion is enforced; only a store error admits.

### R5. SSH host-key handling — code is fail-closed, three docs say otherwise

`connectors/ssh/cmd/ssh-connector/main.go:181-186` returns an error and **refuses to start** when `AGENTOS_SSH_KNOWN_HOSTS` is unset, unless `AGENTOS_SSH_INSECURE_HOST_KEY=true` is set explicitly (an "explicit, loudly-warned dev opt-out").

Stale text to correct:
- `connectors/ssh/README.md:17` — table row claiming "**Unset → host key checking is disabled** (`InsecureIgnoreHostKey`) and a startup WARNING is logged."
- `connectors/ssh/README.md:46-51` — the whole "## Host key checking" section: "If unset, the connector accepts any host key ... and logs a startup WARNING — acceptable for lab demos".
- `deploy/.env.example:65` — `# AGENTOS_SSH_KNOWN_HOSTS=     # unset = insecure host-key (warns at startup)`.

(`deploy/.env.example:160-161` already states the correct fail-closed behavior — the file contradicts itself.)

### R6. `docs/project-context.md` staleness — verified item by item

**§8 "Known open backlog" (line 344)** claims these are open. Verified **shipped**:

| Claimed open | Shipped at |
|---|---|
| budget TOCTOU | `gateway/internal/store/` atomic `ReserveSpend` (covered by `reservation_test.go:30` "closes the race") |
| `http.MaxBytesReader` | `gateway/internal/server/server.go:301` |
| SQL `statement_timeout` | `connectors/sql/internal/tools/tools.go:208` (`SET LOCAL statement_timeout`) |
| runtime non-root image | `runtime/Dockerfile:29` — `USER 10002:10002` |
| HTTP server timeouts + graceful shutdown | `gateway/cmd/gateway/main.go:298` (`ReadHeaderTimeout`), `:339` (`httpServer.Shutdown`) — note `WriteTimeout` is **deliberately unset**, see the comment at `:301` |
| audit retention | `gateway/internal/store/store.go:207-211` (`PruneAudit`) + `AGENTOS_AUDIT_RETENTION_DAYS` |
| N+1 in `handleListOrgs` | `gateway/internal/server/rbac.go:224` — "One grouped query for every org's spend rather than one query per org" |

Two items in §8 were **not** verified shipped by this plan and must be re-checked by the implementer against source before being kept or dropped: **"browser IP backstop"** and **"duplicate per-request org lookups / unbuffered proxy responses"**. `connectors/browser/` is Python (`src/agentos_browser/`), so look in `allowlist.py` and `browser.py`.

**§13 "Honest gaps" (line 491)** — verified stale:

| Claim | Falsified by |
|---|---|
| "No provider retry/fallback in the gateway" | `gateway/internal/server/retry.go` (`retryable`, `parseRetryAfter`, `backoffDelay`, `retryAfterHeader`) |
| "No explicit per-run cycle cap ... relies on LangGraph's default `recursion_limit` of 25" | `runtime/src/agentos_runtime/config.py:70` — `autonomy_max_cycles: int = 8` (`AGENTOS_AUTONOMY_MAX_CYCLES`) |
| "No conversation summarization/trimming" | `runtime/src/agentos_runtime/agent.py:117` — `pre_model_hook = build_trim_hook(settings.max_context_tokens)` (`AGENTOS_MAX_CONTEXT_TOKENS`) |
| "No always-on autonomy — ... designed (Multiverse) but not built" | Operators + council shipped; `make smoke8`, `make smoke9` |
| "The security backlog in §8 is open" | Mostly shipped — see the table above |

Still true in §13 and to be **kept**: HITL is off by default (`AGENTOS_APPROVAL_TOOLS` empty); the deep profile has no HITL (deepagents exposes no `interrupt_before`); budget enforcement is untested end-to-end against paid models.

**§10 (line 399)** is titled "In flight — the 'Multiverse' feature (designed, not implemented)" and says "**no code is written**". Multiverse shipped. Note: the file contains **zero occurrences of "Operators"** — the stale autonomy claim is §13's, not §10's.

**§9 "Delivery history" (line 378)** stops at Phase 8 security hardening and needs the Multiverse + Operators entries.

### R7. Endpoint inventory (verbatim from source)

**Gateway** (`gateway/internal/server/server.go:257-276`, `scim.go:312-320`):

```
GET    /healthz                                    — no auth
POST   /v1/chat/completions                        — agos- virtual key
POST   /v1/embeddings                              — agos- virtual key
POST   /admin/keys                                 — adminAuth
GET    /admin/keys                                 — adminAuth
GET    /admin/usage                                — adminAuth
GET    /admin/audit                                — adminAuth
GET    /admin/whoami                               — adminAuth
POST   /admin/orgs                                 — adminAuth
GET    /admin/orgs                                 — adminAuth
PATCH  /admin/orgs/{org_id}                        — adminAuth
POST   /admin/orgs/{org_id}/users                  — adminAuth
GET    /admin/orgs/{org_id}/users                  — adminAuth
DELETE /admin/orgs/{org_id}/users/{user_id}        — adminAuth
GET    /admin/secrets/status                       — adminAuth
POST   /admin/secrets/reload                       — adminAuth (root)
GET    /admin/providers                            — adminAuth
GET    /auth/oidc/status                           — public
GET    /auth/oidc/login                            — public
GET    /auth/oidc/callback                         — public
POST   /scim/v2/Users                              — scimAuth (AGENTOS_SCIM_TOKEN)
GET    /scim/v2/Users                              — scimAuth
GET    /scim/v2/Users/{id}                         — scimAuth
PATCH  /scim/v2/Users/{id}                         — scimAuth
PUT    /scim/v2/Users/{id}                         — scimAuth
DELETE /scim/v2/Users/{id}                         — scimAuth
GET    /scim/v2/ServiceProviderConfig              — scimAuth
GET    /scim/v2/ResourceTypes                      — scimAuth
GET    /scim/v2/Schemas                            — scimAuth
```

SCIM routes 404 when `AGENTOS_SCIM_TOKEN` is unset.

**Runtime** (`runtime/src/agentos_runtime/api.py` + routers; all require the `AGENTOS_RUNTIME_AUTH_TOKEN` bearer **except** the webhook route):

```
GET    /healthz
POST   /runs
POST   /runs/stream
POST   /runs/{thread_id}/approve
GET    /threads/{thread_id}
GET    /documents
POST   /documents
POST   /evals/run
GET    /evals/runs
POST   /improve
GET    /proposals
POST   /proposals/{proposal_id}/approve
GET    /prompts/active
GET    /skills
POST   /council/objectives            (router prefix "/council")
GET    /council/objectives
GET    /council/objectives/{objective_id}
POST   /council/objectives/{objective_id}/run
POST   /council/objectives/{objective_id}/cancel
POST   /council/objectives/run
GET    /council/members
POST   /council/pause
POST   /council/resume
GET    /operators                     (router prefix "/operators")
POST   /operators
GET    /operators/{operator_id}
PATCH  /operators/{operator_id}
DELETE /operators/{operator_id}
POST   /operators/{operator_id}/run
GET    /operators/{operator_id}/runs
POST   /operators/webhooks/{token}    — EXEMPT from the bearer; the whk- token IS the credential; unknown token 404s
```

Verify the exact `/operators` and `/council` collection paths against `runtime/src/agentos_runtime/operators/api.py` and `council/api.py` before publishing — the router prefixes are confirmed, the bare-collection method list is inferred.

**Sandbox** (`sandbox/src/lib.rs:1-6`, frozen contract):

```
POST /execute   {"language","code","timeout_s","stdin"}
                → {"exit_code","stdout","stderr","duration_ms","timed_out","truncated"}
GET  /healthz   → ok
```

`language` accepts only `"python"`.

### R8. Tool catalog (what an agent can actually call)

| Tool | Service | Port | Arguments | Safety constraint |
|---|---|---|---|---|
| `query` | sql-connector | 8090 | SQL string | Read-only: statement validator + `READ ONLY` transaction + read-only DB role + row cap (`AGENTOS_CONNECTOR_MAX_ROWS`, default 200) + `statement_timeout` |
| `list_tables` | sql-connector | 8090 | — | Read-only metadata |
| `describe_table` | sql-connector | 8090 | table name | Read-only metadata |
| `list_operations` | rest-connector | 8091 | — | Lists OpenAPI operations exposed as tools |
| *(per-operation, generated)* | rest-connector | 8091 | from the OpenAPI spec | GET-only unless `AGENTOS_REST_ALLOW_MUTATIONS`; SSRF-screened; auth header stripped across redirects; body cap `AGENTOS_REST_MAX_BODY_BYTES` |
| `list_operations` | soap-connector | 8093 | — | Lists WSDL 1.1 operations |
| *(per-operation, generated)* | soap-connector | 8093 | from the WSDL | Operation allowlist (`AGENTOS_SOAP_ALLOW_OPERATIONS`); SSRF-screened; not XXE-vulnerable |
| `run_command` | ssh-connector | 8092 | `command` | Basename allowlist + exec-capable-binary deny-list + chaining-character rejection; returns `{exit_code, stdout, stderr, truncated, timed_out}` |
| `list_allowed` | ssh-connector | 8092 | — | Returns the configured allowlist |
| `navigate` | browser-connector | 8094 | `url` | Domain allowlist (`AGENTOS_BROWSER_ALLOW_DOMAINS`); returns `{final_url, title, status}` |
| `get_text` | browser-connector | 8094 | — | Visible page text, capped at `AGENTOS_BROWSER_MAX_TEXT` |
| `find_links` | browser-connector | 8094 | `query` (optional substring filter) | Returns `[{text, href}]` |
| `click` | browser-connector | 8094 | `text` | Clicks the first visible match; read/navigate only — no form submission surface |
| `run_python` | sandbox (via runtime) | 8070 | code, optional `stdin`, `timeout_s` | Per-run temp workdir, cleared env (`PATH` only), new process group, rlimits (CPU/AS/NPROC/FSIZE), SIGKILL on timeout, read-only rootfs, all caps dropped, **no egress** |
| `use_skill` | runtime | 8000 | skill name | Loads only from the image-baked `runtime/skills/` (`AGENTOS_SKILLS_DIR`); never fetched at runtime; each load records a sha256 **for provenance** |

**Never write "sha256-pinned"** for skills. The digest is recorded and logged; it is not compared against a pinned expected value.

### R9. Env vars present in source but absent from `deploy/.env.example` (51)

`docs/configuration.md` must cover all of these. Excluded from this list are the four test-only/placeholder artifacts of the grep (`AGENTOS_MISSING`, `AGENTOS_TEST_DATABASE_URL`, `AGENTOS_TEST_SECRET_ENV`, `AGENTOS_TEST_SECRET_MISSING`) and three prefix fragments (`AGENTOS_OIDC_`, `AGENTOS_SANDBOX_`, `AGENTOS_VAULT_`) — do not document those.

```
AGENTOS_BOOTSTRAP_KEYS            AGENTOS_BROWSER_MAX_TEXT         AGENTOS_BROWSER_TIMEOUT_S
AGENTOS_CHECKPOINT_DATABASE_URL   AGENTOS_CHECKPOINT_DB            AGENTOS_CONNECTOR_DATABASE_URL
AGENTOS_CONNECTOR_MAX_ROWS        AGENTOS_CONNECTOR_PORT           AGENTOS_CONTEXT_ENGINE
AGENTOS_CORS_ORIGINS              AGENTOS_COUNCIL_CONFIG           AGENTOS_COUNCIL_HEARTBEAT_S
AGENTOS_COUNCIL_MAX_SPEND_USD     AGENTOS_COUNCIL_RUNTIME_URL      AGENTOS_DATABASE_URL
AGENTOS_GATEWAY_KEY               AGENTOS_GATEWAY_PORT             AGENTOS_GATEWAY_URL
AGENTOS_MCP_SERVERS               AGENTOS_MOCK_OIDC_ISSUER         AGENTOS_MOCK_OIDC_PORT
AGENTOS_MOCK_VAULT_PORT           AGENTOS_MOCK_VAULT_TOKEN         AGENTOS_OPENCLAW_KEY
AGENTOS_OTEL_ENDPOINT             AGENTOS_PG_PORT                  AGENTOS_REST_ALLOW_MUTATIONS
AGENTOS_REST_AUTH_HEADER          AGENTOS_REST_BASE_URL            AGENTOS_REST_MAX_BODY_BYTES
AGENTOS_REST_SPEC_URL             AGENTOS_RUNTIME_PORT             AGENTOS_SANDBOX_MAX_OUTPUT_BYTES
AGENTOS_SANDBOX_MAX_TIMEOUT_S     AGENTOS_SANDBOX_PORT             AGENTOS_SANDBOX_URL
AGENTOS_SOAP_ALLOW_OPERATIONS     AGENTOS_SOAP_AUTH_HEADER         AGENTOS_SOAP_ENDPOINT
AGENTOS_SOAP_MAX_BODY_BYTES       AGENTOS_SOAP_TIMEOUT_S           AGENTOS_SSH_MAX_OUTPUT_BYTES
AGENTOS_SSH_PORT                  AGENTOS_SSH_TIMEOUT_S
```

The `AGENTOS_MOCK_*` vars belong in `docs/operations.md` (CI mocks), not the main config reference.

### R10. `make` targets (the full set — cite nothing else)

`test` (= `test-go` + `test-python` + `test-console`), `test-go`, `test-python`, `test-console`, `test-rust`, `up`, `down`, `logs`, `fmt`, `smoke`, `smoke2`, `smoke3`, `smoke4`, `smoke5`, `smoke6`, `smoke7`, `smoke8`, `smoke9`.

**Two documented traps:**
1. `make test` **excludes `make test-rust`** — the Rust sandbox suite must be run separately.
2. `make test-go` covers **only `gateway/` and `connectors/sql/`**. The REST, SOAP, SSH, and browser connector suites are not in any `make` target and must be run in their own directories.

### R11. Smoke-target purposes (from each script's header comment)

| Target | Proves |
|---|---|
| `make smoke` | Core loop end-to-end: the agent answers a question from the seeded legacy ERP database through the SQL connector |
| `make smoke2` | Context engine + SSE streaming; then, on the governance overlay, HITL approval + a guardrail block |
| `make smoke3` | Sandbox, REST connector, eval-gated self-improvement, OTel overlay, Helm chart rendering |
| `make smoke4` | Model-based guardrail, LLM-judge evals, egress-less sandbox, SSH connector allowlist (Langfuse config-parse only unless `SMOKE4_LANGFUSE=1`) |
| `make smoke5` | Multi-tenant RBAC, secrets-backend status, SOAP + browser connectors, CI workflow validity |
| `make smoke6` | `whoami`, per-tenant rate limits, OIDC SSO against the mock provider, Vault backend against mock Vault (brings its own mocks up) |
| `make smoke7` | SCIM provisioning, distributed (Postgres) rate-limit store, secret reload |
| `make smoke8` | Operators end-to-end: create, fire manually and by webhook, runs go through the governed agent, skills surface |
| `make smoke9` | A real five-model council answers an objective against the seeded ERP at $0 on local Ollama; asserts distinct members on distinct models, tool evidence, cycle cap trips, kill switch halts, per-key spend attribution, and `council/multiverse` answers through the gateway |

Ordering dependency to state: several smokes assume prior state on a fresh volume (`smoke3`/`smoke4` need the `smoke2` vendor-policy document ingested). All require `make up`.

### R12. Console pages (12) and their gating

Ungated (every role): **Overview, Keys, Audit, Playground, Documents, Improve, Multiverse, Operators**.
Gated by `can(role, …)` in `console/src/App.tsx`: **Orgs** (`org.view`), **Users** (`user.view`), **Secrets** (`secret.view`), **Provisioning** (`provisioning.view`).

Roles: `owner` / `admin` / `member` / `viewer`, plus the root admin key as a global superuser.

Architecture facts: Vite + React + TypeScript SPA, hand-rolled CSS (no component library), nginx-served; browser talks only to same-origin `/api/gateway/*` and `/api/runtime/*`; **the runtime auth token is injected server-side by nginx and never reaches the browser**; sign-in is admin key or OIDC SSO with `whoami`-driven role-aware UI.

### R13. Duplicate-content ownership (spec §6)

| Topic | Canonical file | Reduced to a pointer |
|---|---|---|
| Architecture | `docs/architecture.md` | `docs/project-context.md` (internal briefing; keeps dev-host material, links out) |
| Observability / Langfuse | `docs/deployment.md` | `deploy/LANGFUSE_DOCS_SNIPPET.md`, README observability section |
| OpenClaw interop | `docs/interop/openclaw.md` | `deploy/openclaw/README.md` (overlay command only) |

Each reduced file gets a one-line header: `> Canonical source: [docs/<file>.md](../docs/<file>.md)` (adjust the relative path per file location).

---

## File Structure

**Created:**

| File | Responsibility |
|---|---|
| `CHANGELOG.md` | The phase-completion history, moved out of the README |
| `SECURITY.md` | Living security posture + the R4 fail-open matrix + how to report |
| `docs/concepts.md` | Glossary — the vocabulary every other doc assumes |
| `docs/architecture.md` | Planes, services/ports, request and data flow, persistence, the credential invariant |
| `docs/api.md` | Gateway + runtime endpoint reference **and** the R8 tool catalog |
| `docs/configuration.md` | Every `AGENTOS_*` var, grouped by subsystem |
| `docs/console.md` | Page-by-page operator-surface reference |
| `docs/deployment.md` | Compose → Helm → CI/eval-gate, overlays, connector tiers |
| `docs/operations.md` | Smoke targets, demo fixtures, CI mocks, egress verification |

**Modified:**

| File | Change |
|---|---|
| `README.md` | Full concept-first rewrite (Task 11) |
| `docs/project-context.md` | §8/§9/§10/§13 corrections + internal-briefing marker (Task 2) |
| `connectors/ssh/README.md` | Host-key section + table row → fail-closed (Task 1) |
| `deploy/.env.example` | Line 65 comment only (Task 1) |
| `sandbox/README.md` | Drop phase labels (Task 1) |
| `connectors/rest/README.md` | Drop phase labels (Task 1) |
| `console/README.md` | Rewrite: 12 pages, no phase title (Task 8) |
| `deploy/LANGFUSE_DOCS_SNIPPET.md` | Canonical-source header (Task 9) |
| `deploy/openclaw/README.md` | Canonical-source header (Task 9) |

---

## Task 1: Correct the fail-closed / phase-label inaccuracies in existing files

The smallest, most self-contained correctness fix, and it removes contradictions the later docs would otherwise inherit. No new files.

**Files:**
- Modify: `connectors/ssh/README.md:17` and `connectors/ssh/README.md:46-51`
- Modify: `deploy/.env.example:65`
- Modify: `sandbox/README.md:1`, `sandbox/README.md:22`, `sandbox/README.md:82`
- Modify: `connectors/rest/README.md:21`

**Interfaces:**
- Consumes: reference facts **R5** (SSH fail-closed) and the Global Constraint "phase framing is removed".
- Produces: a repo in which no file claims SSH host-key checking is warn-and-continue. Tasks 7, 10, and 11 rely on this.

- [ ] **Step 1: Read the current text and confirm the line numbers still match**

Run:
```bash
sed -n '14,21p;44,52p' connectors/ssh/README.md
sed -n '60,66p' deploy/.env.example
sed -n '175,190p' connectors/ssh/cmd/ssh-connector/main.go
```
Expected: the README claims `InsecureIgnoreHostKey` on unset; `main.go` returns an error refusing to start. If the line numbers have drifted, locate the text and proceed — the text, not the line number, is the target.

- [ ] **Step 2: Fix the `connectors/ssh/README.md` table row**

Replace the `AGENTOS_SSH_KNOWN_HOSTS` row's description with:

```
Path to a `known_hosts` file → strict host key checking. **Required**: with this unset the connector **refuses to start**, unless `AGENTOS_SSH_INSECURE_HOST_KEY=true` is set explicitly.
```

- [ ] **Step 3: Rewrite the `## Host key checking` section**

Replace the section body with:

```markdown
## Host key checking

Set `AGENTOS_SSH_KNOWN_HOSTS` to a `known_hosts` file for strict verification
(`knownhosts.New`). This is **required**: with it unset the connector refuses to
start and exits with an error naming the MITM risk.

The only way to run without host-key verification is the explicit dev opt-out
`AGENTOS_SSH_INSECURE_HOST_KEY=true`, which accepts any host key. Never set it
outside a lab.
```

- [ ] **Step 4: Fix the `deploy/.env.example` comment**

Replace the line

```
# AGENTOS_SSH_KNOWN_HOSTS=     # unset = insecure host-key (warns at startup)
```

with

```
# AGENTOS_SSH_KNOWN_HOSTS=     # REQUIRED for the SSH connector: unset = refuses to start
```

Change **only** this comment. Do not reformat, reorder, or otherwise touch `deploy/.env.example` — it is consumed by Compose.

- [ ] **Step 5: Add the missing SSH connector vars to `deploy/.env.example`**

In the same `Phase 4: SSH connector` block, add the three documented-but-absent vars as commented examples (see **R9**, and note the **R1** trap):

```
# AGENTOS_SSH_PORT=22          # remote SSH target port (NOT the connector's listen port, which is 8092)
# AGENTOS_SSH_TIMEOUT_S=15     # per-command timeout
# AGENTOS_SSH_MAX_OUTPUT_BYTES=65536
```

Also retitle that block's comment from `# --- Phase 4: SSH connector (opt-in; no live target in the demo) ---` to `# --- SSH connector (opt-in; no live target in the demo) ---`.

- [ ] **Step 6: Drop phase labels from `sandbox/README.md`**

- Line 1 title: `# sandbox/ — Isolated Python execution (Phase 3)` → `# sandbox/ — Isolated Python execution`
- Line 22: `only "python" in Phase 3` → `only "python"`
- Line 82: `(the Phase 3 global constraints)` → delete the parenthetical, keeping the sentence grammatical.

- [ ] **Step 7: Drop the phase label from `connectors/rest/README.md`**

Line 21 heading `## Phase 3 limitation` → `## Limitation`. Read the section body and, if it opens by referring to "Phase 3", rewrite that clause to describe the limitation directly.

- [ ] **Step 8: Verify no phase references remain in the touched files**

Run:
```bash
grep -rn -i 'phase' connectors/ssh/README.md connectors/rest/README.md sandbox/README.md deploy/.env.example
```
Expected: no matches in the three READMEs. `deploy/.env.example` may still contain other `Phase N` block comments — those are out of scope for this task and are addressed in Task 7 only if that task rewrites the file (it does not; leave them).

- [ ] **Step 9: Verify the SSH claim now matches the code**

Run:
```bash
grep -n -i 'insecure\|refuses to start\|known_hosts' connectors/ssh/README.md deploy/.env.example
```
Expected: every hit describes fail-closed behavior or the explicit `AGENTOS_SSH_INSECURE_HOST_KEY=true` opt-out. No hit claims a warning-and-continue default.

- [ ] **Step 10: Commit**

```bash
git add connectors/ssh/README.md deploy/.env.example sandbox/README.md connectors/rest/README.md
git commit -m "docs: correct SSH host-key behavior to fail-closed; drop phase labels"
```

---

## Task 2: Correct `docs/project-context.md` and mark it an internal briefing

Removes the stale-source contamination risk before any new doc is written from this file.

**Files:**
- Modify: `docs/project-context.md` — §8 (line ~344), §9 (line ~378), §10 (line ~399), §13 (line ~491), and a new header note near line 1

**Interfaces:**
- Consumes: reference fact **R6** (the verified item-by-item staleness table).
- Produces: a `project-context.md` whose §8/§13 can be read without re-verification, and a stated canonical pointer to `docs/architecture.md` (created in Task 5). The link will be dead until Task 5 lands — that is expected and is resolved by Task 12's link check.

- [ ] **Step 1: Add the internal-briefing marker**

Immediately under the `# AgentOS — full project context` title, insert:

```markdown
> **Internal briefing.** This file is the working context for people building
> AgentOS: dev-host quirks, delivery history, and hard-won gotchas. It is not
> the public documentation. For architecture see
> [`architecture.md`](architecture.md); for the security posture see
> [`../SECURITY.md`](../SECURITY.md).
```

- [ ] **Step 2: Re-verify the two unconfirmed §8 backlog items**

Run:
```bash
grep -rn -i 'private\|ip\|allowlist' connectors/browser/src/agentos_browser/allowlist.py | head -20
grep -rn 'org' gateway/internal/server/rbac.go | grep -i 'lookup\|cache' | head -10
```
Decide from what you read whether "browser IP backstop" and "duplicate per-request org lookups / unbuffered proxy responses" are still open. Keep only what the source confirms is open.

- [ ] **Step 3: Rewrite the §8 "Known open backlog" paragraph**

Replace the paragraph beginning `**Known open backlog** (tracked, not yet done): budget TOCTOU, …` with a paragraph that states what actually shipped and lists only what Step 2 confirmed is still open. Use this shape, substituting your Step 2 findings for the bracketed clause:

```markdown
**Backlog status.** Most of the items this section once tracked have shipped and
were verified in source: atomic budget reservation (`ReserveSpend`, race-covered
by test), `http.MaxBytesReader` request-body caps, SQL `statement_timeout`,
HTTP server read-header timeout and graceful shutdown (`WriteTimeout` is
deliberately unset — see the comment in `gateway/cmd/gateway/main.go`), a
non-root runtime image (`USER 10002:10002`), opt-in audit retention with
`PruneAudit`, and the `handleListOrgs` N+1 (now one grouped spend query).
Still open: [your Step 2 findings, or "nothing from the original list"].
```

Do not write "tracked, not yet done" about anything you have not just re-verified.

- [ ] **Step 4: Extend §9 "Delivery history"**

After the numbered list (which ends at `7. Scale & provisioning …`) and the existing Phase 8 paragraph, append:

```markdown
9. Multiverse — config-driven provider registry, upstream retry/fallback, a
   council of model-bound agents with judge synthesis and dissent reporting, a
   governed autonomous loop, and `council/multiverse` as an OpenAI-compatible
   model. Smoke-tested live: `make smoke9`.
10. Operators — governed single-agent autonomy on interval / cron / webhook
   triggers toward stored objectives, bounded and audited, with in-repo
   `SKILL.md` skills pulled on demand. Smoke-tested live: `make smoke8`.
```

Renumber or reposition only if the existing list numbering makes that necessary; do not renumber phases 1–7.

- [ ] **Step 5: Rewrite §10**

Change the heading

```
## 10. In flight — the "Multiverse" feature (designed, not implemented)
```

to

```
## 10. The Multiverse council
```

Delete the line `Approved design and a 16-task implementation plan exist; **no code is written**.` and replace it with a statement that the feature shipped and is smoke-tested by `make smoke9`, keeping the spec/plan links as historical design records. Then reconcile the rest of the section with reality:

- The "three prerequisites that do not exist today" paragraph (provider registry, real pricing, multi-agent runtime) — all three shipped; rewrite as delivered capabilities or delete.
- The "closes two harness gaps: no per-run cycle cap and no provider retry/fallback" clause — both shipped; rewrite accordingly.
- The "**New console surface it defines** (Task 14, the likely UI/UX target)" paragraph — the Multiverse page shipped; restate in the present tense or delete.

Leave the concept description (N agents, one judge, verdict plus dissent) intact — it is accurate.

- [ ] **Step 6: Rewrite §13 "Honest gaps"**

Delete the five stale bullets identified in **R6** (no provider retry/fallback; no per-run cycle cap; no conversation trimming; no always-on autonomy; "the security backlog in §8 is open"). Keep and leave unchanged the three still-true bullets: HITL off by default, deep profile has no HITL, budget enforcement untested against paid models.

Add whatever Step 2 confirmed is genuinely still open. If §13 would otherwise be nearly empty, that is the correct outcome — do not invent gaps to fill it.

- [ ] **Step 7: Verify no stale claim survives**

Run:
```bash
grep -n -i 'not implemented\|no code is written\|designed (Multiverse)\|No provider retry\|No explicit per-run cycle cap\|No conversation summarization\|No always-on autonomy\|not yet done' docs/project-context.md
```
Expected: **no matches.**

- [ ] **Step 8: Commit**

```bash
git add docs/project-context.md
git commit -m "docs: correct stale backlog, gaps, and Multiverse status in project-context"
```

---

## Task 3: Create `CHANGELOG.md`

Must land before the README rewrite, which deletes the phase list and links here instead.

**Files:**
- Create: `CHANGELOG.md`
- Read: `README.md:299-328` (the current Roadmap list — the source content)

**Interfaces:**
- Consumes: `README.md`'s existing numbered roadmap entries 1–10 (all struck through / complete) and entry 11 (forward-looking).
- Produces: `CHANGELOG.md` with one `##` heading per shipped phase. Task 11's README links to it; Task 11 keeps **only** entry 11's content as its Roadmap.

- [ ] **Step 1: Read the source list**

Run: `sed -n '299,330p' README.md`

- [ ] **Step 2: Write `CHANGELOG.md`**

Structure — one section per shipped phase, newest first, each keeping the existing wording (which is accurate) with the strikethrough and ✅ removed:

```markdown
# Changelog

All notable delivery milestones. Each phase was smoke-tested live end-to-end
before it was considered done; the verification targets are documented in
[`docs/operations.md`](docs/operations.md).

## Operators — governed single-agent autonomy

Interval / cron / webhook triggers toward stored objectives, bounded and
audited, with in-repo `SKILL.md` skills the agent pulls on demand.
Verified by `make smoke8`.

## Multiverse — a council of models

Config-driven provider registry, upstream retry/fallback, a council of
model-bound agents with judge synthesis and dissent reporting, a governed
autonomous loop, and `council/multiverse` as an OpenAI-compatible model.
Verified by `make smoke9`.

## Hardening & efficiency

Atomic budget reservation (closing a TOCTOU race), request-body caps, server
timeouts and graceful drain, non-root images with Kubernetes `securityContext`,
SQL `statement_timeout`, opt-in audit retention, runtime context trimming.

## Scale & provisioning

SCIM 2.0 user provisioning, distributed (Postgres) rate-limit store, secret
rotation and reload. Verified by `make smoke7`.

## Enterprise identity

OIDC SSO, Vault secrets backend, per-tenant rate limits, the `whoami` endpoint.
Verified by `make smoke6`.

## Enterprise reach & governance

Multi-tenant RBAC, secrets backends (env / file / age), SOAP and browser
connectors, CI with an LLM-judge eval gate. Verified by `make smoke5`.

## Reach & hardening

SSH connector, egress-less sandbox, model-based guardrail, LLM-judge evals,
bundled Langfuse profile. Verified by `make smoke4`.

## Autonomy, safely

Rust sandbox, eval-gated self-improvement loop, REST/OpenAPI connector and demo
CRM, Helm chart, OTel collector profile. Verified by `make smoke3`.

## Operability & governance

Console UI, SSE streaming, guardrails, human-in-the-loop approvals,
LlamaIndex/pgvector context engine, deepagents profile, opt-in OpenTelemetry.
Verified by `make smoke2`.

## Core loop

Gateway, runtime, SQL connector, and the Compose demo. Verified by `make smoke`.
```

Cross-check every `make smoke*` claim against **R11** before writing it.

- [ ] **Step 3: Verify every cited target exists**

Run:
```bash
for t in smoke smoke2 smoke3 smoke4 smoke5 smoke6 smoke7 smoke8 smoke9; do grep -q "^$t:" Makefile && echo "OK $t" || echo "MISSING $t"; done
```
Expected: nine `OK` lines.

- [ ] **Step 4: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: add CHANGELOG with the phase delivery history"
```

---

## Task 4: Create `docs/concepts.md`

The vocabulary every later doc assumes. Written first so the others can link terms instead of re-explaining them.

**Files:**
- Create: `docs/concepts.md`

**Interfaces:**
- Consumes: **R1** (services), **R3** (pipeline), **R8** (skills/provenance wording), **R12** (roles).
- Produces: anchor targets other docs link to. Use these exact `##`/`###` headings so anchors are stable: `Planes`, `Identity & tokens`, `Governance`, `Agents`, `Integration`. Term entries are `**Term** — definition` list items (not headings), so link to the section anchor, not the term.

- [ ] **Step 1: Write the file**

Required structure and content:

```markdown
# Concepts & glossary

The vocabulary the rest of the documentation assumes. Terms are grouped by the
part of the system they belong to.

## Planes

- **Gateway** — the Go service (`:8080`) every model call flows through. It
  issues and authenticates virtual keys, holds budget, applies rate limits and
  guardrails, routes to providers, and writes the audit log. It is the only
  path from the platform to a model provider.
- **Runtime** — the Python service (`:8000`) that runs agents: the agent loop,
  durable threads, skills, operators, and the council. It holds **no** provider
  credentials and reaches models only through the gateway.
- **Sandbox** — the Rust service (`:8070`) that executes untrusted Python with
  per-run isolation and no network egress. It has no published host port; only
  the runtime can reach it.
- **Connector** — a Go or Python service that exposes one legacy system as MCP
  tools, with the safety constraint enforced inside the connector rather than
  asked of the model (read-only SQL, GET-only REST, an SSH command allowlist, a
  browser domain allowlist).
- **Console** — the operator surface (`:3000`): keys, usage, audit, runs,
  documents, proposals, council, operators, and tenancy administration.

## Identity & tokens

- **Virtual key (`agos-…`)** — a gateway-issued credential for model access.
  Carries its own org, budget, and rate limit. The only credential accepted on
  `/v1/*`.
- **User token (`agu-…`)** — an identity credential for the admin plane, bound
  to a user, org, and role. Minted by OIDC sign-in or created directly.
- **Root admin key** (`AGENTOS_ADMIN_KEY`) — global superuser for the admin API.
- **SCIM token** (`AGENTOS_SCIM_TOKEN`) — gates `/scim/v2/*`; when unset those
  routes 404.
- **Webhook token (`whk-…`)** — the credential *is* the URL path segment for
  `POST /operators/webhooks/{token}`; the one runtime route exempt from the
  bearer. Returned once at creation, redacted on every read.
- **Org** — the tenancy boundary. Keys, usage, spend, and audit are scoped to it.
- **Role** — `owner` / `admin` / `member` / `viewer`. Evaluated on the admin
  plane only (see **Governance**).

## Governance

- **Budget hold** — a reservation taken before an upstream call and settled
  after, so concurrent calls cannot overspend a cap. Exhaustion returns HTTP
  402; only a store error admits the request (and that is audited).
- **Rate limit** — a per-org token bucket (`rate_limit_rpm`, 0 = unlimited).
  Over-limit calls return 429 with `Retry-After`.
- **Guardrail** — prompt-injection screening at the gateway
  (`AGENTOS_GUARDRAILS_MODE` = `off` | `log` | `block` | `model`). A classifier
  outage fails open with a `guardrail_error` audit entry.
- **Audit log** — the record of every gateway outcome, including denials.
- **RBAC** — role checks on the **admin plane** (`/admin/*`, `/auth/oidc/*`,
  `/scim/v2/*`). **Not** evaluated on `/v1/*`, where scoping comes from the
  virtual key's own org.
- **Secret backend** — where provider keys resolve from:
  `env` | `file` | `age` | `vault`.
- **HITL (human-in-the-loop)** — configured tools pause a run for approval
  (`AGENTOS_APPROVAL_TOOLS`; off by default).

## Agents

- **Run / thread** — one agent invocation and its durable, resumable
  conversation state (Postgres checkpointing).
- **Profile** — `react` (LangGraph ReAct) or `deep` (deepagents).
- **Skill** — a reviewed `SKILL.md` instruction sheet the agent pulls on demand
  via `use_skill`. Loaded only from the image-baked skills directory, never
  fetched at runtime; each load records a sha256 for provenance.
- **Operator** — a standing objective the runtime pursues on its own, fired by
  an interval, a cron schedule, or a webhook. Off by default.
- **Council (Multiverse)** — N model-bound members answer one objective in
  parallel; a judge synthesizes one verdict plus an explicit dissent report.
- **Proposal** — a change (a write-class tool call, or a new system prompt) that
  only a human can activate.
- **Eval / eval-gate** — a scored suite, and the CI check that blocks a change
  scoring below the threshold.

## Integration

- **MCP (Model Context Protocol)** — the streamable-HTTP protocol connectors
  speak; the runtime discovers tools from the servers listed in
  `AGENTOS_MCP_SERVERS`.
- **Provider registry** (`deploy/providers.json`) — config-driven vendor
  routing with real per-1M-token pricing, so budgets and usage stay accurate
  for non-builtin models. Base URLs are SSRF-screened.
```

Fill in the elided definitions (`…`) with one or two sentences each, sourced from **R1** and **R3**.

- [ ] **Step 2: Verify the token prefixes and env var names against source**

Run:
```bash
grep -rn '"agos-\|"agu-\|"whk-' gateway/internal runtime/src --include='*.go' --include='*.py' | head -10
grep -n 'AGENTOS_GUARDRAILS_MODE\|AGENTOS_APPROVAL_TOOLS\|AGENTOS_SCIM_TOKEN\|AGENTOS_ADMIN_KEY' deploy/.env.example
```
Expected: all three prefixes and all four env var names appear. Correct the doc to whatever source says if anything differs.

- [ ] **Step 3: Commit**

```bash
git add docs/concepts.md
git commit -m "docs: add concepts and glossary"
```

---

## Task 5: Create `docs/architecture.md`

**Files:**
- Create: `docs/architecture.md`
- Read: `docs/project-context.md` §2 (line ~36), §3 (line ~82), §6 (line ~208) for structure — **de-internalized**, not copied

**Interfaces:**
- Consumes: **R1** (the services/ports table, verbatim), **R2** (connector tiers), **R3** (the pipeline).
- Produces: the canonical architecture reference (**R13**). Stable `##` headings for inbound links: `The four planes`, `Services and ports`, `Request flow`, `Data and persistence`, `The credential invariant`.

- [ ] **Step 1: Write the ASCII architecture diagram**

Use this — it is correct, unlike the README's current one (which shows a single unnamed connector and omits the sandbox and console):

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

- [ ] **Step 2: Write the four-planes section**

One paragraph per plane (gateway / runtime / sandbox+connectors are separable — write four: Gateway, Runtime, Sandbox, Connectors), each stating what it owns and what it deliberately does not. The runtime paragraph must state that it holds no provider credentials.

- [ ] **Step 3: Insert the services-and-ports table**

Reproduce the **R1** table. Keep the "Compose default" and "Helm template" columns — the deployment asymmetry is the single most confusing thing about the platform. Add the `AGENTOS_SSH_PORT` trap as a note under the table.

- [ ] **Step 4: Write the request-flow section**

Two flows:

1. **A model call** — reproduce the **R3** chain as an ordered list, with one clause per stage saying what it checks and what it returns on denial (402 / 429 / blocked). State that `/v1/embeddings` skips the guardrail, that audit records every outcome including denials, and — explicitly — that **RBAC is not in this path**.
2. **An agent run** — client → runtime `/runs` → agent loop → (model call via the gateway, looping back into flow 1) → MCP tool call to a connector or `run_python` to the sandbox → durable checkpoint → response. Note where HITL interrupts.

- [ ] **Step 5: Write persistence and the credential invariant**

Persistence: Postgres holds keys, orgs, users, usage, audit, agent checkpoints, and pgvector document embeddings; the sandbox is stateless; connectors hold no state.

The credential invariant, stated once, plainly: the runtime never holds provider credentials; every action is authorized, attributed, and recorded; all external integrations are opt-in and off by default.

- [ ] **Step 6: Verify the diagram's ports against source**

Run:
```bash
grep -n 'listenAddr *=' connectors/*/cmd/*/main.go
grep -n 'LISTEN_PORT' connectors/browser/src/agentos_browser/config.py
grep -n 'BIND_ADDR' sandbox/src/main.rs
```
Expected: 8090 (sql), 8091 (rest), 8092 (ssh), 8093 (soap), 8094 (browser), 8070 (sandbox). Fix the diagram if anything differs.

- [ ] **Step 7: Commit**

```bash
git add docs/architecture.md
git commit -m "docs: add public architecture reference"
```

---

## Task 6: Create `docs/api.md`

**Files:**
- Create: `docs/api.md`

**Interfaces:**
- Consumes: **R7** (endpoint inventory, verbatim) and **R8** (tool catalog, verbatim).
- Produces: the reference `docs/console.md` (Task 8) links per page, and the endpoint anchors the README's capability tour targets. Headings: `Gateway API`, `Runtime API`, `Sandbox API`, `Tool catalog`.

- [ ] **Step 1: Write the gateway section**

A table with columns `Method · Path · Purpose · Auth`, one row per line of **R7**'s gateway block. Auth values: `none`, `agos- virtual key`, `adminAuth (agu- token or root admin key, role-checked)`, `adminAuth (root only)` for `/admin/secrets/reload`, `scimAuth (AGENTOS_SCIM_TOKEN)`. Add the note that SCIM routes 404 when the token is unset.

- [ ] **Step 2: Write the runtime section**

Same table shape, one row per line of **R7**'s runtime block. Auth is `AGENTOS_RUNTIME_AUTH_TOKEN` bearer for every route **except** `POST /operators/webhooks/{token}` — call that exception out in bold, with the reason (the `whk-` token is itself the credential; an unknown token 404s).

- [ ] **Step 3: Verify the inferred runtime collection routes**

**R7** flags that the bare `/operators` and `/council` collection methods were inferred from router prefixes. Confirm before publishing:

Run:
```bash
grep -n '@router\.' runtime/src/agentos_runtime/operators/api.py
grep -n '@router\.' runtime/src/agentos_runtime/council/api.py
```
Expected: each decorator's path, prefixed with `/operators` or `/council` respectively. Correct the table to match exactly what you read.

- [ ] **Step 4: Write the sandbox section**

Reproduce the **R7** sandbox contract, including the request and response field names, and note that it is a frozen contract and that `language` accepts only `"python"`. State that the sandbox has no published host port under Compose — the runtime reaches it over the internal network.

- [ ] **Step 5: Write the tool catalog**

Reproduce the **R8** table in full. This section answers "what can an agent actually do", which nothing in the repo currently documents. Add a lead paragraph stating that a tool is reachable only if its server's URL is in `AGENTOS_MCP_SERVERS` — and link the connector-tiers section of `docs/deployment.md` (Task 9).

- [ ] **Step 6: Verify the tool names**

Run:
```bash
grep -rn '"query"\|"list_tables"\|"describe_table"\|"list_operations"\|"run_command"\|"list_allowed"' connectors/*/internal/tools/tools.go
grep -n 'async def' connectors/browser/src/agentos_browser/server.py
grep -rn 'run_python\|use_skill' runtime/src/agentos_runtime --include='*.py' | grep -v test | head -5
```
Expected: sql exposes `query`, `list_tables`, `describe_table`; rest and soap expose `list_operations`; ssh exposes `run_command`, `list_allowed`; browser exposes `navigate`, `get_text`, `find_links`, `click`; the runtime surfaces `run_python` and `use_skill`.

- [ ] **Step 7: Commit**

```bash
git add docs/api.md
git commit -m "docs: add API and tool-catalog reference"
```

---

## Task 7: Create `docs/configuration.md`

**Files:**
- Create: `docs/configuration.md`
- Read: `deploy/.env.example`, `deploy/compose.yaml`, `deploy/helm/agentos/values.yaml`, `gateway/cmd/gateway/main.go`, `runtime/src/agentos_runtime/config.py`, `sandbox/src/lib.rs`, `connectors/*/README.md`

**Interfaces:**
- Consumes: **R9** (the 51 vars missing from `.env.example`), **R2** (connector tiers), **R1** (the `AGENTOS_SSH_PORT` trap), **R5** (SSH fail-closed).
- Produces: the config reference the README quickstart and `docs/deployment.md` link to. Group headings: `Core`, `Models & providers`, `Governance`, `Identity & secrets`, `Runtime & agents`, `Council`, `Operators & skills`, `Sandbox`, `Connectors`, `Console & networking`, `Observability`, `Ports`.

- [ ] **Step 1: Build the complete variable list**

Run:
```bash
grep -rhoE 'AGENTOS_[A-Z0-9_]+' --include='*.go' --include='*.py' --include='*.rs' --include='*.yaml' --include='*.yml' --include='*.ts' --include='*.tsx' --include='*.conf' --include='*.sh' --include='*.md' --include='*.example' gateway runtime sandbox connectors console deploy scripts | sort -u > /tmp/agentos-vars.txt
wc -l /tmp/agentos-vars.txt
```
Expected: ~120 lines. This is the authoritative set the doc must cover, minus the exclusions named in **R9** (four test-only vars, three prefix fragments).

- [ ] **Step 2: Write the grouped reference**

One table per group, columns `Variable · Default · Purpose`. Rules:

- **Defaults must come from source**, not from `.env.example` comments. Where a default is set in Go/Python/Rust, read it there.
- **No phase tags** in group names (the `.env.example` groupings are phase-labeled; regroup by subsystem).
- The `AGENTOS_MOCK_*` vars go in `docs/operations.md` (Task 9), not here — note that in a one-line pointer.
- `AGENTOS_SSH_PORT` — the row must say **remote SSH target port, default 22; not the connector's listen port (`:8092`)**.
- `AGENTOS_SSH_KNOWN_HOSTS` — required; unset means the connector refuses to start unless `AGENTOS_SSH_INSECURE_HOST_KEY=true`.
- `AGENTOS_GUARDRAILS_MODE` — note the plural `GUARDRAILS`, a name that reads like a typo and is not.

- [ ] **Step 3: Write the "MCP wiring" gotcha section**

A dedicated `## Connector wiring (read this before enabling a connector)` section reproducing **R2** in full, including that `AGENTOS_MCP_SERVERS` is a hardcoded literal in `deploy/compose.yaml` with no `${}` substitution, that `--profile connectors` starts SOAP and browser **without** making their tools reachable, that SSH has no Compose or Helm service, and that Helm's `restConnector.enabled` defaults to `false` while `runtime.extraMcpServers` is the supported extension point.

- [ ] **Step 4: Verify coverage — every var is documented**

Run:
```bash
missing=0
while read -r v; do
  case "$v" in AGENTOS_MISSING|AGENTOS_TEST_*|AGENTOS_OIDC_|AGENTOS_SANDBOX_|AGENTOS_VAULT_|AGENTOS_MOCK_*) continue;; esac
  grep -q "$v" docs/configuration.md || { echo "UNDOCUMENTED: $v"; missing=$((missing+1)); }
done < /tmp/agentos-vars.txt
echo "undocumented: $missing"
```
Expected: `undocumented: 0`. Any `AGENTOS_MOCK_*` reported here is fine only once Task 9 documents it — re-run this check in Task 12.

- [ ] **Step 5: Verify no invented variables**

Run:
```bash
grep -ohE 'AGENTOS_[A-Z0-9_]+' docs/configuration.md | sort -u | while read -r v; do
  grep -qx "$v" /tmp/agentos-vars.txt || echo "NOT IN SOURCE: $v"
done
```
Expected: no output.

- [ ] **Step 6: Commit**

```bash
git add docs/configuration.md
git commit -m "docs: add full configuration reference"
```

---

## Task 8: Create `docs/console.md` and rewrite `console/README.md`

Same subject, one reviewer, one commit.

**Files:**
- Create: `docs/console.md`
- Modify: `console/README.md` (full rewrite)
- Read: `console/src/App.tsx:62-99` (the `ROUTES` array), `console/src/pages/*.tsx`

**Interfaces:**
- Consumes: **R12** (the 12 pages and their gating), **R7** (the endpoints each page calls).
- Produces: the operator-surface reference the README's Console section links to. Headings: `Signing in`, `Pages`, `Roles and visibility`, `How the console talks to the platform`.

- [ ] **Step 1: Confirm the page list and gating**

Run:
```bash
sed -n '62,99p' console/src/App.tsx
ls -1 console/src/pages/*.tsx | xargs -n1 basename
```
Expected: 12 routes; `visible:` predicates on exactly Orgs, Users, Secrets, Provisioning.

- [ ] **Step 2: Find the backing endpoint for each page**

Run:
```bash
grep -rn "api/gateway\|api/runtime\|/admin/\|/runs\|/documents\|/council\|/operators\|/proposals\|/evals" console/src/pages/*.tsx | grep -oE '"/[a-z/{}_-]+"' | sort | uniq -c | sort -rn | head -30
```
Use the result to fill the "backed by" column. Do not guess.

- [ ] **Step 3: Write `docs/console.md`**

Required content:

- **Signing in** — two paths: the root admin key, or OIDC SSO ("Sign in with SSO" appears when `AGENTOS_OIDC_ISSUER` is configured). `GET /admin/whoami` drives the role-aware UI, so what you see reflects your role without manual configuration.
- **Pages** — a table: `Page · Path · What it's for · Backed by · Visible to`. All 12 rows. Ungated pages are "every role"; the four gated pages name their permission (`org.view`, `user.view`, `secret.view`, `provisioning.view`).
- **Roles and visibility** — why a page may be missing: the sidebar and the ⌘K palette render only routes whose `visible` predicate passes for your role. This is a UI affordance; the gateway enforces the same permissions server-side regardless.
- **How the console talks to the platform** — the browser makes **same-origin** requests to `/api/gateway/*` and `/api/runtime/*` only; nginx proxies them and **injects `AGENTOS_RUNTIME_AUTH_TOKEN` server-side, so it never reaches the browser**. No third-party requests, no CDN. Mention the freshness indicators (live dot / poll cadence) and the ⌘K command palette.

- [ ] **Step 4: Rewrite `console/README.md`**

Replace the whole file. It currently documents 5 of 12 pages and is titled "Phase 2".

```markdown
# console/ — the AgentOS operator surface

TypeScript admin console for AgentOS. Vite + React + TypeScript SPA,
hand-rolled CSS (no component library), served by nginx in production.

For what each page does and who can see it, see
[`../docs/console.md`](../docs/console.md).

## Developing

```bash
cd console
npm install
npm run dev      # Vite dev server
npm test -- --run
npm run build
```

`make test-console` runs the vitest suite and the production build.

## Architecture notes

- Routing is a single `ROUTES` array in `src/App.tsx` driving both the sidebar
  and the ⌘K command palette — add a route there and both pick it up.
- The browser talks only to same-origin `/api/gateway/*` and `/api/runtime/*`;
  nginx injects the runtime auth token server-side.
- Design system: graphite surfaces, colour reserved for machine state
  (in-flight / ok / awaiting-human / denied). Motion presets live in
  `src/ui/motion.ts` and reduced motion is honoured.
```

Keep the fenced blocks exactly as shown, and add nothing that is not verifiable in `console/`.

- [ ] **Step 5: Verify**

Run:
```bash
grep -n -i 'phase' console/README.md docs/console.md
grep -c '|' docs/console.md
```
Expected: no phase matches; the pages table has at least 13 pipe-bearing lines (header + separator + 12 rows).

- [ ] **Step 6: Commit**

```bash
git add docs/console.md console/README.md
git commit -m "docs: document the console operator surface"
```

---

## Task 9: Create `docs/deployment.md` and `docs/operations.md`; set canonical ownership

**Files:**
- Create: `docs/deployment.md`, `docs/operations.md`
- Modify: `deploy/LANGFUSE_DOCS_SNIPPET.md`, `deploy/openclaw/README.md` (canonical-source headers only)
- Read: `deploy/compose.yaml`, `deploy/helm/agentos/values.yaml`, `deploy/helm/agentos/README.md`, `deploy/ci/`, `deploy/SANDBOX_EGRESS_VERIFY.md`, `.github/workflows/`

**Interfaces:**
- Consumes: **R1**, **R2**, **R10**, **R11**, **R13**.
- Produces: the deploy and verification references the README links. `docs/deployment.md` becomes the canonical home for observability/Langfuse.

- [ ] **Step 1: Write `docs/deployment.md`**

Sections:

1. **Compose (laptop)** — `make up` / `make down` / `make logs`, what starts by default (**R1** "Compose default" column), the `.env` step, the console at `:3000`.
2. **Overlays** — one subsection each, with the exact command:
   - HITL: `docker compose -f deploy/compose.yaml -f deploy/compose.hitl.yaml up -d`
   - OTel: `docker compose -f deploy/compose.yaml -f deploy/compose.otel.yaml up -d`
   - Langfuse: `docker compose -f deploy/compose.yaml -f deploy/compose.otel.yaml -f deploy/compose.langfuse.yaml up -d` — **this is now the canonical Langfuse documentation**; carry over the substance of the README's current "Bundled Langfuse" section (port 3001, the `base64("pk:sk")` → `LANGFUSE_OTLP_BASIC_AUTH` step, `deploy/langfuse.env.example`, the Langfuse v3 caveat).
   - auth-mocks: `deploy/compose.auth-mocks.yaml` (point at `docs/operations.md` for detail).
   - openclaw: the overlay command, linking `docs/interop/openclaw.md` as canonical.
3. **Connector tiers** — reproduce **R2**. This is the section `docs/api.md` links to.
4. **Kubernetes (Helm)** — `deploy/helm/agentos/`, the per-service toggles, which services ship enabled vs disabled (**R1** "Helm template" column — call out `restConnector.enabled: false` and that SOAP/browser/SSH have no templates at all), bundled pgvector Postgres or `externalDatabaseUrl`, the hardened sandbox pod + NetworkPolicy, optional console ingress, `runtime.extraMcpServers`, and `deploy/helm/test-render.sh`.
5. **CI and the eval gate** — the workflow matrix and the eval gate that scores the runtime suite against a deterministic mock model and fails under 0.8, run by labelling a PR `run-evals`. Verify the threshold and label before writing:

```bash
grep -rn 'run-evals\|0\.8\|threshold' .github/workflows/ | head -10
```

- [ ] **Step 2: Write `docs/operations.md`**

Sections:

1. **Verification targets** — the **R11** table in full, plus the ordering dependency (several smokes assume prior state on a fresh volume; `smoke3`/`smoke4` need `smoke2`'s vendor-policy document ingested) and that all require `make up`.
2. **Test targets** — **R10**, including both traps: `make test` excludes `make test-rust`, and `make test-go` covers only `gateway/` and `connectors/sql/` (the REST, SOAP, SSH, and browser suites run from their own directories).
3. **Demo fixtures** — `deploy/initdb/01-legacy-erp.sql` (the seeded legacy ERP Postgres behind the SQL connector) and `deploy/demo-crm/` (the REST connector's OpenAPI target). Confirm what each contains before describing it:

```bash
head -30 deploy/initdb/01-legacy-erp.sql
ls -1 deploy/demo-crm/
```
4. **CI mocks** — `deploy/ci/mock-model.py`, `mock-oidc.py`, `mock-vault.py`, and `deploy/compose.auth-mocks.yaml`; what each stands in for and which smoke/CI job uses it. Document the `AGENTOS_MOCK_*` vars here (**R9**).
5. **Egress verification** — link `deploy/SANDBOX_EGRESS_VERIFY.md` and state in one sentence what it proves (a container on `sandbox-net` cannot resolve or reach the public internet).
6. **Helm render check** — `deploy/helm/test-render.sh`.

- [ ] **Step 3: Add the canonical-source headers**

At the very top of `deploy/LANGFUSE_DOCS_SNIPPET.md`:

```markdown
> **Canonical source:** [`docs/deployment.md`](../docs/deployment.md) — observability and Langfuse setup. This file is a snippet kept for reference.
```

At the very top of `deploy/openclaw/README.md`:

```markdown
> **Canonical source:** [`docs/interop/openclaw.md`](../../docs/interop/openclaw.md). This file covers only the overlay command.
```

Change nothing else in either file.

- [ ] **Step 4: Verify every command and path cited exists**

Run:
```bash
for f in deploy/compose.yaml deploy/compose.hitl.yaml deploy/compose.otel.yaml deploy/compose.langfuse.yaml deploy/compose.auth-mocks.yaml deploy/langfuse.env.example deploy/helm/test-render.sh deploy/SANDBOX_EGRESS_VERIFY.md deploy/initdb/01-legacy-erp.sql deploy/ci/mock-model.py deploy/ci/mock-oidc.py deploy/ci/mock-vault.py; do
  test -e "$f" && echo "OK $f" || echo "MISSING $f"
done
```
Expected: all `OK`. If any path is `MISSING`, find the real one and correct the doc.

- [ ] **Step 5: Commit**

```bash
git add docs/deployment.md docs/operations.md deploy/LANGFUSE_DOCS_SNIPPET.md deploy/openclaw/README.md
git commit -m "docs: add deployment and operations references; set canonical ownership"
```

---

## Task 10: Create `SECURITY.md`

**Files:**
- Create: `SECURITY.md` (repo root — GitHub surfaces this location specially)
- Read: `docs/security/2026-07-21-security-assessment.md`, `docs/project-context.md` §8 (as corrected in Task 2)

**Interfaces:**
- Consumes: **R4** (the fail-open matrix, verbatim), **R5**, **R6**, **R8** (skills provenance wording), and the Global Constraint forbidding uncritical reuse of `project-context.md` §8/§13.
- Produces: the posture doc the README's Security section links to.

- [ ] **Step 1: Write the file**

Required sections, in order:

1. **The invariant** — the runtime never holds provider credentials; every action is authorized, attributed, and recorded; all external integrations are opt-in and off by default.
2. **Trust boundaries** — the gateway is the only egress to model providers; the sandbox has no egress at all and no published host port; connectors are the only path to legacy systems; the console's browser bundle never sees the runtime auth token (nginx injects it server-side).
3. **Fail-open vs fail-closed** — reproduce the **R4** table verbatim, per failure mode. Add the sentence: *A backend outage must not take traffic down; a policy verdict must not be bypassable. The two are different failures and are handled differently.*
4. **Hardening summary** — carry over the fixed-findings table from `project-context.md` §8 (the seven-row table is accurate and describes shipped fixes), plus the sandbox isolation layers (per-run temp workdir, cleared env, process group SIGKILL, CPU/AS/NPROC/FSIZE rlimits, read-only rootfs, all caps dropped, non-root, internal-only network) and the supply-chain stance on skills (in-repo, image-baked, never fetched; each load records a sha256 for provenance — **not** pinned).
5. **Residual risk** — stated plainly: a model acting badly within its permissions; prompt injection via content it reads; budget enforcement is untested end-to-end against paid models; the deep profile has no HITL (deepagents exposes no `interrupt_before`); HITL is off by default.
6. **Reporting a vulnerability** — how to report privately. Do not invent an email address or a security.txt URL. Write: *Open a private security advisory on the repository, or contact the maintainer directly. Please do not open a public issue for a suspected vulnerability.*
7. **Assessment record** — link `docs/security/2026-07-21-security-assessment.md`, dated, described as a point-in-time audit.

- [ ] **Step 2: Verify nothing stale leaked in from §8/§13**

Run:
```bash
grep -n -i 'not yet done\|open backlog\|TOCTOU\|no provider retry\|no per-run cycle cap\|no always-on autonomy\|sha256-pinned\|fails open' SECURITY.md
```
Expected: the only surviving `fails open` hits are the R4-table rows scoped to backend/classifier errors. No hit claims an already-shipped control is missing. No `sha256-pinned` anywhere.

- [ ] **Step 3: Verify the claims that carry weight**

Run:
```bash
grep -n 'USER 10002' runtime/Dockerfile
grep -n 'read_only\|cap_drop' deploy/compose.yaml | head -6
grep -n 'internal: true' deploy/compose.yaml
grep -rn 'RUNTIME_AUTH_TOKEN' console/nginx.conf console/*.conf 2>/dev/null | head -3
```
Expected: each control you claimed is visible in source. Remove or soften any claim you cannot show.

- [ ] **Step 4: Commit**

```bash
git add SECURITY.md
git commit -m "docs: add living SECURITY.md with the fail-open/fail-closed matrix"
```

---

## Task 11: Rewrite `README.md`

Written last, against docs that already exist.

**Files:**
- Modify: `README.md` (full rewrite, 331 lines → concept-first)

**Interfaces:**
- Consumes: every file created in Tasks 3–10, plus **R1**, **R2**, **R10**.
- Produces: the entry point. Every `docs/` file must be reachable from it.

- [ ] **Step 1: Write the new README against this outline**

Follow spec §6 exactly. Twelve sections, in this order:

1. **Hero** — the one-line pitch, the sentence *nothing runs unauthorized, unattributed, or unrecorded*, the corrected ASCII diagram from `docs/architecture.md` (Task 5, Step 1), and links to the landing page and the in-console Docs tab. **Delete both shields.io CI badges** (lines 3–4) — they cannot resolve while the repo is private and unpushed. Note re-adding them under Roadmap.
2. **What it is / who it's for** — 3–4 value props; the credential invariant stated once.
3. **Concepts & glossary** — three sentences, link `docs/concepts.md`.
4. **Quickstart** — **two** paths:
   - *Compose with a provider key*: `cd deploy && cp .env.example .env`, set `AGENTOS_ANTHROPIC_API_KEY`, `make up`, `make smoke`, console at `http://localhost:3000`.
   - *Local Ollama, $0, no third-party calls*: spell out the required overrides — `AGENTOS_MODEL`, `AGENTOS_EMBED_MODEL`, `AGENTOS_JUDGE_MODEL`, `AGENTOS_OLLAMA_BASE_URL` — because the shipped defaults point at Anthropic and a bare `make up` is **not** the $0 path.
   - State the `AGENTOS_RUNTIME_AUTH_TOKEN` gotcha: the runtime refuses to start without it.
5. **Architecture** — the diagram plus three sentences; link `docs/architecture.md`.
6. **Capability tour** — follows the **§5 taxonomy**, *not* the landing grid (whose legends are Engine / Observability / Evaluation / Deployment / Sandbox / Fleet and do not map onto it). One short group per plane, each ending in an exact link: Gateway → `docs/architecture.md#request-flow` and `docs/api.md#gateway-api`; Runtime → `docs/api.md#runtime-api`; Sandbox → `sandbox/README.md`; Connectors → `docs/api.md#tool-catalog` and `docs/deployment.md` (connector tiers). Verify each anchor exists (Step 3).
7. **Console** — four sentences on the operator surface; link `docs/console.md`.
8. **Deploy** — Compose → Helm → CI/eval-gate in three sentences; link `docs/deployment.md`, `docs/operations.md`, `deploy/helm/agentos/README.md`.
9. **Security** — short posture paragraph; link `SECURITY.md`.
10. **Repository layout** — **rewritten** (the current table is wrong: it lists one connector, calls the console "Phase 2", the sandbox "Phase 3", and says "Helm later"):

```markdown
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
```

11. **Development / testing / contributing** — `make test`, `make test-rust`, `make fmt`, and **both R10 traps stated explicitly**: `make test` excludes `make test-rust`, and `make test-go` covers only `gateway/` and `connectors/sql/`. Link `docs/operations.md`.
12. **Roadmap + docs index + license** — forward-looking only (keep the current entry 11's content: SAML SSO, cloud-KMS secret backends, Redis limiter option, secret rotation webhooks, SCIM Groups; add "restore CI badges once the repo is public"). Then a docs index linking exactly these, in taxonomy order: `docs/concepts.md`, `docs/architecture.md`, `docs/api.md`, `docs/console.md`, `docs/configuration.md`, `docs/deployment.md`, `docs/operations.md`, `docs/interop/openclaw.md`, `SECURITY.md`, `CHANGELOG.md`. Then `License: [Apache-2.0](LICENSE)`.

**Delete** from the README: both badges, the numbered phase roadmap (now `CHANGELOG.md`), and the deep per-feature sections (Governance overlay, Sandboxed code execution, Self-improvement, Kubernetes, Observability, Bundled Langfuse, Hardening, Security detail, Multi-tenancy & secrets, Connectors, CI, Operators, Multiverse) — their content now lives in the `docs/` files. Carry any fact that has no home into the right `docs/` file rather than dropping it.

- [ ] **Step 2: Fix the `--profile connectors` claim**

The current README says `docker compose --profile connectors up -d   # + soap + browser` implying the agent gains those tools. Per **R2** it does not. If the capability tour mentions the connectors profile at all, it must say the services start but their URLs must also be added to `AGENTOS_MCP_SERVERS`, and link the connector-tiers section of `docs/deployment.md`.

- [ ] **Step 3: Verify every link resolves**

Run:
```bash
grep -oE '\]\(([^)#]+)(#[^)]*)?\)' README.md | sed -E 's/^\]\(//; s/\)$//; s/#.*//' | sort -u | while read -r p; do
  [ -z "$p" ] && continue
  case "$p" in http*) echo "EXTERNAL $p"; continue;; esac
  test -e "$p" && echo "OK $p" || echo "BROKEN $p"
done
```
Expected: no `BROKEN`. The only `EXTERNAL` entries should be the landing page and, if cited, third-party docs — no `github.com/ifahad/agentos` URLs (the repo is private).

- [ ] **Step 4: Verify every anchor cited in the capability tour exists**

For each `docs/<file>.md#<anchor>` link, confirm a matching heading. Anchors are the heading text lowercased with spaces → hyphens and punctuation dropped:

```bash
grep -oE 'docs/[a-z]+\.md#[a-z0-9-]+' README.md | sort -u | while IFS='#' read -r f a; do
  slug=$(grep -oE '^#{2,3} .*' "$f" | sed -E 's/^#+ //; s/[^A-Za-z0-9 -]//g; s/ /-/g' | tr 'A-Z' 'a-z')
  echo "$slug" | grep -qx "$a" && echo "OK $f#$a" || echo "BROKEN ANCHOR $f#$a"
done
```
Expected: no `BROKEN ANCHOR`. Adjust either the link or the heading.

- [ ] **Step 5: Verify no badge, no phase framing, no invented make target**

Run:
```bash
grep -n 'shields.io\|img.shields' README.md
grep -n -iE 'phase [0-9]' README.md
grep -ohE 'make [a-z0-9-]+' README.md | sort -u | sed 's/make //' | while read -r t; do grep -q "^$t:" Makefile || echo "NO SUCH TARGET: make $t"; done
```
Expected: no output from any of the three.

- [ ] **Step 6: Commit**

```bash
git add README.md
git commit -m "docs: rewrite README concept-first around the capability taxonomy"
```

---

## Task 12: Whole-set verification pass

The spec §6 checklist, run against the finished set. This task fixes what it finds.

**Files:**
- Modify: whichever docs the checks flag

**Interfaces:**
- Consumes: everything Tasks 1–11 produced.
- Produces: a documentation set with no broken links, no undocumented env vars, no invented targets, and taxonomy names matching spec §5 verbatim — the precondition for W2 mirroring it.

- [ ] **Step 1: Every relative link in every doc resolves**

Run:
```bash
for f in README.md SECURITY.md CHANGELOG.md docs/*.md docs/interop/*.md console/README.md connectors/*/README.md sandbox/README.md; do
  d=$(dirname "$f")
  grep -oE '\]\(([^)#]+)(#[^)]*)?\)' "$f" 2>/dev/null | sed -E 's/^\]\(//; s/\)$//; s/#.*//' | while read -r p; do
    [ -z "$p" ] && continue
    case "$p" in http*|mailto:*) continue;; esac
    test -e "$d/$p" || test -e "$p" || echo "BROKEN $f -> $p"
  done
done
```
Expected: no output. Fix every hit.

- [ ] **Step 2: Every `make` target cited anywhere exists**

Run:
```bash
grep -rohE 'make [a-z0-9-]+' README.md SECURITY.md CHANGELOG.md docs/*.md console/README.md | sort -u | sed 's/make //' | while read -r t; do grep -q "^$t:" Makefile || echo "NO SUCH TARGET: make $t"; done
```
Expected: no output.

- [ ] **Step 3: Every `AGENTOS_*` var cited anywhere exists in source, and every source var is documented**

Run:
```bash
grep -rhoE 'AGENTOS_[A-Z0-9_]+' --include='*.go' --include='*.py' --include='*.rs' --include='*.yaml' --include='*.yml' --include='*.ts' --include='*.tsx' --include='*.conf' --include='*.sh' --include='*.example' gateway runtime sandbox connectors console deploy scripts | sort -u > /tmp/agentos-vars.txt
echo "--- cited but not in source ---"
grep -rohE 'AGENTOS_[A-Z0-9_]+' README.md SECURITY.md CHANGELOG.md docs/*.md console/README.md | sort -u | while read -r v; do grep -qx "$v" /tmp/agentos-vars.txt || echo "INVENTED: $v"; done
echo "--- in source but undocumented ---"
while read -r v; do
  case "$v" in AGENTOS_MISSING|AGENTOS_TEST_*|AGENTOS_OIDC_|AGENTOS_SANDBOX_|AGENTOS_VAULT_) continue;; esac
  grep -qr "$v" docs/configuration.md docs/operations.md || echo "UNDOCUMENTED: $v"
done < /tmp/agentos-vars.txt
```
Expected: no `INVENTED`, no `UNDOCUMENTED`.

- [ ] **Step 4: Taxonomy names and order match spec §5 verbatim**

The canonical order is: Overview, Concepts & Glossary, Architecture, Gateway, Runtime, Sandbox, Connectors, Console, Quickstart, Configuration, Deploy, Security.

Confirm by reading the README's section order and the `docs/` file titles that every taxonomy name appears as a README heading or a `docs/` file title, in that relative order. Fix any name that drifted (e.g. a doc titled "Setup" where the taxonomy says "Quickstart").

Record the final mapping in a short table at the end of the README's docs index so W2 can mirror it exactly.

- [ ] **Step 5: No stale-claim regressions across the whole set**

Run:
```bash
grep -rn -i 'sha256-pinned\|insecure host-key (warns\|Helm later\|Phase [0-9]' README.md SECURITY.md docs/*.md console/README.md connectors/*/README.md sandbox/README.md
```
Expected: no output. (`CHANGELOG.md` is exempt — it is the history — but check that even there nothing is described in the present tense as unbuilt.)

- [ ] **Step 6: Full read-through**

Read `README.md` end to end as someone who has never seen the platform. Then read each `docs/` file. Fix anything that assumes context the reader does not have, contradicts another file, or reads as internal narration.

- [ ] **Step 7: Confirm no non-doc file was modified**

Run:
```bash
git diff --stat main...HEAD -- . ':(exclude)*.md' ':(exclude)deploy/.env.example'
```
Expected: empty. W1 must not have touched code, manifests, or the `Makefile`.

- [ ] **Step 8: Commit any fixes**

```bash
git add -A
git commit -m "docs: fix links, anchors, and taxonomy drift found in the W1 verification pass"
```

If Steps 1–7 found nothing, skip the commit and say so.

---

## Completion

When Task 12 passes: merge to `main` locally (fast-forward), delete the working branch, and **do not push**. Then W2 (the console Docs tab) mirrors the taxonomy recorded in Task 12, Step 4.
