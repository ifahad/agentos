# Concepts & Glossary

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
  after, so concurrent calls cannot overspend a cap. Exhaustion is enforced
  with HTTP 402; only a store error admits the request, and — unlike the
  guardrail's classifier-outage path — that admission is **not** audited.
- **Rate limit** — a per-org token bucket (`rate_limit_rpm`, 0 = unlimited).
  Over-limit calls return 429 with `Retry-After` and write a `rate_limited`
  audit entry.
- **Guardrail** — prompt-injection screening at the gateway
  (`AGENTOS_GUARDRAILS_MODE` = `off` | `log` | `block` | `model`). A classifier
  outage fails open with a `guardrail_error` audit entry.
- **Audit log** — the gateway's record of outcomes, as one of seven kinds:
  `chat`, `embeddings`, `guardrail_flag`, `guardrail_block`,
  `guardrail_error`, `rate_limited`, `secret_reload`. Among denials, only
  rate-limit and guardrail events are recorded; auth failures and budget
  exhaustion are not.
- **RBAC** — role checks on the **admin plane**, `/admin/*` only. `/scim/v2/*`
  is gated separately by a static shared-secret bearer token, not a role;
  `/auth/oidc/*` is unauthenticated (the public login/callback flow). **Not**
  evaluated on `/v1/*`, where scoping comes from the virtual key's own org.
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
