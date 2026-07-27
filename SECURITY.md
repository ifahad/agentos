# Security

AgentOS is a governed platform for running agents against real systems. This
document states what the platform actually enforces, where it deliberately
degrades instead of failing, and what it does not cover. Controls described
here are present in source; anything not described here should be assumed
absent.

## The invariant

**The runtime never holds provider credentials.** Every model call the runtime
makes goes to the gateway with a gateway-issued virtual key
(`runtime/src/agentos_runtime/agent.py`, `build_chat_model` — the only model
wiring in the runtime is a client pointed at `gateway_url + "/v1"` and
authenticated with `gateway_key`). Provider secrets are resolved inside the
gateway from a `secret.Source` and are never handed downstream.

**Every action that runs is authorized, attributed to a caller, and recorded.**
A served request writes both a usage record and an audit entry, together:
`gateway/internal/server/server.go` (`s.record` → `store.RecordUsage`),
`gateway/internal/store/postgres.go` (`RecordUsage` — one transaction inserts
the `usage` row and the `audit_log` row, then commits),
`gateway/internal/store/memory.go` (`RecordUsage`, the in-memory backend does
the equivalent).

**Coverage on the refusal side is partial, and you should plan around that.**
The audit log records exactly seven kinds (`gateway/internal/store/store.go`):
`chat`, `embeddings`, `guardrail_flag`, `guardrail_block`, `guardrail_error`,
`rate_limited`, `secret_reload`. Among refusals, **only rate-limit rejections
and guardrail events are audited**. A `401` auth failure, a `400` malformed
request, and a `402` budget exhaustion write nothing
(`gateway/internal/server/server.go`, `handleChatCompletions`). If you need a
complete record of rejected traffic, take it from your ingress logs, not from
the audit table.

**Authorization is not uniform across the gateway's surfaces.** Role-based
access control gates `/admin/*` only, via the root admin key or an `agu-` user
token (`gateway/internal/server/rbac.go`, `adminAuth`, with role evaluation in
`rbac.Can`). `/scim/v2/*` uses a separate static shared-secret bearer compare
with no role evaluation (`gateway/internal/server/scim.go`, `scimAuth`).
`/auth/oidc/*` carries no auth wrapper at all — it is the public login and
callback flow (`gateway/internal/server/server.go`, the `/auth/oidc/` route
registrations). RBAC is never evaluated on `/v1/*`; org scoping there comes
from the virtual key's own org.

**Connectors are not uniformly opt-in.** The SQL and REST connectors are wired
into the runtime's tool list and start by default in Compose
(`deploy/compose.yaml`, `AGENTOS_MCP_SERVERS`). Only the SOAP, browser, and SSH
connectors are opt-in — SOAP and browser behind Compose profiles
(`deploy/compose.yaml`, `profiles:` on `soap-connector` and
`browser-connector`), SSH with no Compose service at all. Do not assume an
integration is off because it is not configured; assume it is on unless it is
one of those three.

## Trust boundaries

- **The gateway is the only egress to model providers.** Nothing else in the
  platform is configured with a provider key, and the runtime's sole model
  client points at the gateway.
- **The sandbox has no egress at all, and no published host port.** It sits
  alone on an `internal: true` Docker network (`deploy/compose.yaml`, the
  `sandbox` service and the `sandbox-net` network), so code executed inside it
  has no route to the internet or to the other services. There is no `ports:`
  mapping — Docker forbids publishing a host port from an internal network — so
  the runtime, which is attached to both networks (`deploy/compose.yaml`, the
  `runtime` service's `networks:`), is the only way in.
- **Connectors are the only path to legacy systems.** Agents do not open
  sockets; they call tools, and each connector enforces its own constraints
  before the call leaves it (read-only SQL, GET-only REST by default, SSH
  allowlist, SOAP operation allowlist, browser domain allowlist).
- **The console's browser bundle never sees the runtime auth token.** nginx
  injects it server-side when proxying `/api/runtime/`
  (`console/nginx.conf.template`, substituted at container start by
  `console/docker-entrypoint.sh`), so the credential stays out of anything
  the browser can read.
- **The console's runtime proxy is itself an unauthenticated path to the
  runtime.** That injection is unconditional: nginx attaches the bearer to
  every `/api/runtime/` request and requires no credential from the browser
  (`console/nginx.conf.template`, the `/api/runtime/` location), and the
  runtime authenticates one shared bearer with no role concept at all
  (`runtime/src/agentos_runtime/api.py`, `require_auth`). Confidentiality and
  access control are separate properties here: the token never leaks to the
  browser, and reaching the console's port is equivalent to holding it. Anyone
  who can reach that port can drive the full runtime API — create or delete
  operators, ingest documents, approve a council write-class proposal, or
  hot-swap the live system prompt via `POST /proposals/{id}/approve`. The
  gateway's role checks (`/admin/*`) do not apply to this path.
- **Two runtime paths require no bearer at all.** `GET /healthz`, so
  orchestrator probes need no credential, and any method under
  `/operators/webhooks/` — an inbound trigger path into an autonomous operator,
  where the opaque `whk-` token in the URL is the only credential and an
  unknown token 404s (`runtime/src/agentos_runtime/api.py`, `OPEN_PATHS` and
  `OPEN_PREFIXES`). Treat a webhook URL as a secret; it is one.

## Fail-open vs fail-closed

A backend outage must not take traffic down; a policy verdict must not be
bypassable. The two are different failures and are handled differently.

| Control | On a *verdict* | On a *backend/classifier error* |
|---|---|---|
| Budget (per-key and per-org) | **Fail closed** — HTTP 402 `budget_exceeded` / `org_budget_exceeded` | **Fail open**, and — unlike the guardrail row below — that admission is **not** audited (`gateway/internal/server/rbac.go`, `admitSpend`, returns `func(){}, true` with no `recordAudit`); it reads as an ordinary success |
| Rate limit | **Fail closed** — HTTP 429 `rate_limited` + `Retry-After` | **Fail open** (Postgres backend only; the `memory` backend has no such path) |
| Guardrail | **Fail closed** — request blocked, audited | **Fail open** with a `guardrail_error` audit entry |
| OIDC `email_verified` | Fail closed | — |
| SSH host-key verification | Fail closed | — |
| Runtime auth token | Fail closed — runtime refuses to start without `AGENTOS_RUNTIME_AUTH_TOKEN` | — |
| Council write gating | Fail closed — an unclassified tool is treated as write-class — **for `profile: react` members only** (see below) | — |

Read the budget row precisely: exhaustion **is** enforced and returns a 402
(`gateway/internal/server/rbac.go`, `admitSpend`). Only a store error admits, and that
admission leaves no trace — which is the one blind spot in this table that is
not self-reporting. The guardrail's equivalent blind spot *is* audited, under
kind `guardrail_error` (`gateway/internal/server/server.go`, `KindGuardrailError`).

Verified locations for the other rows: rate limit 429 with `Retry-After` and a
`rate_limited` audit entry in `gateway/internal/server/server.go` (`rateLimited`),
with the fail-open branch in `gateway/internal/ratelimit/postgres.go`; OIDC
`email_verified` required in `gateway/internal/oidc/oidc.go`; SSH host-key
verification refusing to start when `AGENTOS_SSH_KNOWN_HOSTS` is unset, in
`connectors/ssh/cmd/ssh-connector/main.go` (`resolveHostKeyCallback` — an
explicit, loudly-warned dev opt-out, `AGENTOS_SSH_INSECURE_HOST_KEY=true`, is
the only way past it); runtime auth token required in
`runtime/src/agentos_runtime/config.py` (`require_runtime_auth_token`); council
write gating in `runtime/src/agentos_runtime/council/gating.py`
(`READ_SAFE_TOOLS`, `write_class_calls`), where anything outside a small
read-safe set is held as a proposal rather than executed.

The council write-gating row carries the same deepagents caveat as HITL, and
for the same reason: deepagents compiles its own graph with no
`interrupt_before` pass-through, so in-graph write gating applies to
`profile: react` members only (`runtime/src/agentos_runtime/council/gating.py`,
module docstring). A `profile: deep` member is constrained solely by the
hand-written read-only `tools:` allowlist on its entry in
`runtime/council.yaml` — that list *is* its action surface, and a test asserts
the shipped file holds to it. All five deep members ship `enabled: false`;
read the row before enabling one.

## Hardening summary

### Fixed findings

A four-audit security review is recorded at
[docs/security/2026-07-21-security-assessment.md](docs/security/2026-07-21-security-assessment.md).
It found a real unauthenticated-RCE chain and other issues. All were fixed and
verified before any autonomy work proceeded:

| Finding | Fix |
|---|---|
| Runtime had **no auth on any route** + a published port | `AGENTOS_RUNTIME_AUTH_TOKEN`, fail-closed at startup; console injects it via nginx |
| SSH allowlist bypassable to RCE via LOLBins (`find -exec`, `awk system()`) | Deny-list of exec-capable binaries, blocked chaining chars, `known_hosts` required |
| Prompt hot-swap unvalidated | Immutable `SAFETY_PREAMBLE` always prepended |
| REST/SOAP leaked the auth header across redirects → SSRF to metadata | `safehttp` `CheckRedirect` + private-IP refusal |
| OIDC accepted unverified emails; role adoption takeover | `email_verified` required; match on stable `sub` |
| Cross-org usage/spend/audit leak via key **name** scoping | Keyed by `secret_hash` + `org_id`, with migration |
| Secret rotation was a silent no-op for provider keys | Router reads the live `secret.Source` per call |

Verified in source: the immutable preamble in
`runtime/src/agentos_runtime/agent.py` (`SAFETY_PREAMBLE`,
`build_system_prompt`); redirect screening and private/loopback/link-local
refusal in `connectors/rest/internal/safehttp/safehttp.go` and
`connectors/soap/internal/safehttp/safehttp.go`; the SSH exec-capable deny-list
and shell-character rejection in `connectors/ssh/internal/validate/`;
`secret.Source` injected into provider routing in
`gateway/internal/provider/provider.go`; secret reloads audited in
`gateway/internal/server/secrets.go`.

Also in place: request bodies capped with `http.MaxBytesReader`
(`gateway/internal/server/server.go`) and opt-in audit retention via
`PruneAudit` (`gateway/internal/store/store.go`) — never called unless
retention is explicitly configured, because an audit trail is evidence.

### Sandbox isolation

Code the agent writes runs in a Rust sandbox, not a host shell. Per execution
(`sandbox/src/executor.rs`, `execute_python`):

- fresh temporary working directory, removed when the run ends
- cleared environment — the child sees only a restricted `PATH` (`CHILD_PATH`)
- new process group, SIGKILL'd as a group on timeout so grandchildren die with
  the child (`kill_process_group`)
- rlimits on CPU time, address space, process count, and file size

And per container (`sandbox/Dockerfile`, `deploy/compose.yaml`, the `sandbox`
service): read-only root filesystem, `tmpfs` for `/tmp`, all capabilities
dropped, `no-new-privileges`, memory/pid/CPU caps, a non-root user
(`sandbox/Dockerfile`), and the internal-only network described above.

### Container users

The two images reach non-root by different means, and both are worth checking
if you re-base them: `runtime/Dockerfile` sets `USER 10002:10002`
explicitly; `gateway/Dockerfile` inherits non-root from a
`gcr.io/distroless/static-debian12:nonroot` base and carries no `USER` line of
its own.

### Skills supply chain

Skills live in-repo, are baked into the image (`runtime/Dockerfile`), and are
never fetched at runtime. The load path is a local directory —
`AGENTOS_SKILLS_DIR` when set, otherwise the in-repo `runtime/skills/`
(`runtime/src/agentos_runtime/api.py`) — and there is no network fetch
anywhere in it. Each load computes and records a sha256 **for provenance** and
logs it (`runtime/src/agentos_runtime/operators/skills.py`, `load_skills`).
That digest is recorded, not enforced — nothing compares it against a pinned
expected value.
The integrity guarantee here comes from the skills being in the image, not from
the hash.

## Residual risk

Stated plainly, so nothing above reads as more than it is:

- **A model can act badly within its permissions.** Every control here bounds
  *what* an agent may reach. None of them make its judgment sound. If a tool is
  allowed, a sufficiently confused agent will use it wrongly.
- **Prompt injection via content the agent reads.** Retrieved documents, web
  pages, and API responses are untrusted input that reaches the model. The
  safety preamble and untrusted-content delimiters raise the cost of an attack;
  they do not close the class.
- **The browser connector has no IP-level backstop.** Its allowlist
  (`connectors/browser/src/agentos_browser/allowlist.py`, `host_allowed`)
  matches on hostname only and never resolves the host to an address, so unlike
  the REST and SOAP connectors it has no private/loopback/link-local refusal. A
  hostname that resolves into your internal network is reachable if it is on the
  allowlist. Keep that allowlist narrow and hold it to names you control.
- **Budget enforcement is untested end-to-end against paid models.** The
  admission race is covered by a test
  (`gateway/internal/store/reservation_test.go`,
  `TestReserveSpendClosesTheRace`); the full path against a
  real metered provider is not.
- **The console's port is the runtime's authorization boundary.** Its nginx
  proxy attaches the runtime bearer unconditionally and the runtime has no
  roles, so network reachability of `:3000` is the only thing standing between
  a caller and the full runtime API — there is no second check behind it. See
  the trust-boundaries bullet above; bound that port at the network layer,
  because nothing in the application layer bounds it.
- **The deep agent profile has no human-in-the-loop.** deepagents compiles its
  own graph and exposes no `interrupt_before` pass-through, so tool approvals
  apply to the react profile only
  (`runtime/src/agentos_runtime/agent.py`, `build_agent`). The council's
  in-graph write gating is limited the same way; a `profile: deep` council
  member is bounded only by its `tools:` allowlist.
- **HITL is off by default.** `AGENTOS_APPROVAL_TOOLS` is empty
  (`runtime/src/agentos_runtime/config.py`, `approval_tools`); tools run
  unattended unless you opt in.

## Reporting a vulnerability

Open a private security advisory on the repository, or contact the maintainer
directly. Please do not open a public issue for a suspected vulnerability.

Useful things to include: the affected component, the version or commit you
tested, and a minimal reproduction.

## Assessment record

[docs/security/2026-07-21-security-assessment.md](docs/security/2026-07-21-security-assessment.md)
records a point-in-time audit dated 2026-07-21: four independent read-only
audits (gateway, connectors, runtime, infrastructure) plus a review of the
OpenClaw interop design, with findings deduplicated and ranked by severity
including exploit chains. It reflects the platform as it stood on that date and
is not maintained as a live document. This file is the current statement of
posture; the assessment is the historical record of how it got here.

Related: [docs/architecture.md](docs/architecture.md) for the trust boundaries
in context, [docs/configuration.md](docs/configuration.md) for the settings
named above, and [docs/operations.md](docs/operations.md) for audit and usage
operations.
