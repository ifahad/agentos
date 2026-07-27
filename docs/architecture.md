# Architecture

AgentOS is four planes wired together by one rule: **the runtime never holds a
provider credential.** Every model call — from a human in the console, an
OpenAI-compatible client, an agent, or the eval judge — crosses the gateway on
a virtual key, so it is authenticated, budgeted, rate-limited, and audited the
same way regardless of where it originated.

Terms below are defined once, in [`concepts.md`](concepts.md); this page links
to them rather than redefining them.

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

## The four planes

**Gateway.** The Go service every model call flows through, and the only part
of the platform that ever holds a provider credential. It authenticates
[virtual keys](concepts.md#identity--tokens), enforces
[budgets and rate limits](concepts.md#governance), runs the
[guardrail](concepts.md#governance), routes to a provider (or the council),
and writes the [audit log](concepts.md#governance). It also carries the admin
plane — key management, usage, audit, orgs, OIDC SSO, SCIM — but the three
route groups there are not gated the same way: [role checks
(RBAC)](concepts.md#identity--tokens) apply only to `/admin/*`; `/scim/v2/*`
is gated by a static shared-secret bearer token, not a role; `/auth/oidc/*` is
the unauthenticated public login/callback flow. It does not run agents and
does not execute code.

**Runtime.** The Python service that runs the agent loop: durable,
resumable [threads](concepts.md#agents), [skills](concepts.md#agents),
[operators](concepts.md#agents), the [council](concepts.md#agents), retrieval,
evals, and self-improvement. **It holds no provider credentials of its own** —
every model call it makes, including the eval judge and the guardrail
classifier, goes back through the gateway on a virtual key, so it is
budgeted and audited like any other caller. It does not talk to legacy
systems directly; it reaches them only through connectors and the sandbox.

**Sandbox.** The Rust service that executes untrusted Python for the
`run_python` tool, one isolated process group per run, with no network egress
and no published host port — only the runtime can reach it. It holds no
state between runs and makes no decisions about what code is safe to run; it
only isolates and kills it.

**Connectors.** Go (and, for browser, Python) services that expose one legacy
system each as [MCP](concepts.md#integration) tools: SQL, REST, SSH, SOAP,
browser. Each connector enforces its own safety constraint in code rather than
asking the model to behave — read-only SQL transactions, GET-only REST,
an SSH command allowlist, a browser domain allowlist — and none of them holds
application state; they front the legacy system and nothing else.

## Services and ports

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

The deployment column asymmetry is deliberate, not an oversight: SQL and REST
are wired into `AGENTOS_MCP_SERVERS` by default and have Helm templates (REST
ships disabled by default in Helm); SOAP and browser start only under a
Compose profile and have no Helm template at all; SSH has neither a Compose
service nor a Helm template and must be run and wired up by hand.

**A trap in the naming:** `AGENTOS_SSH_PORT` is the **remote SSH target
port** (default `22`) — the port the connector connects *to* on the machine it
manages — not the connector's own listen port, which is the hardcoded `:8092`
shown in the table above.

## Request flow

### A model call

Every call to `POST /v1/chat/completions` passes through the gateway in this
fixed order:

1. **Auth** — the request must carry a valid `agos-…` [virtual
   key](concepts.md#identity--tokens); otherwise `401`.
2. **Rate limit** — the key's org must have budget in its request-per-minute
   bucket; otherwise `429` with `Retry-After`.
3. **Budget hold** — a reservation is taken against both the key's and the
   org's monthly cap; exhaustion is enforced with `402`, not advisory. The
   only way a request is admitted past an exhausted budget is a store or
   database error — and unlike the guardrail's classifier-outage path below,
   that admission is **not** audited: it is recorded as an ordinary success,
   indistinguishable from any other admitted call.
4. **Guardrail** — run only when `AGENTOS_GUARDRAILS_MODE != off`; a flagged
   prompt is blocked (`400`) in `block`/`model` mode, or logged and forwarded
   in `log` mode. A classifier outage fails open, but — unlike the budget
   store error above — it leaves its own audit entry.
5. **Upstream provider or council** — the request is routed by model prefix to
   a provider, or, for a `council/…` model, to the runtime's council.
6. **Audit** — the gateway's audit log records outcomes as one of seven
   kinds: `chat`, `embeddings`, `guardrail_flag`, `guardrail_block`,
   `guardrail_error`, `rate_limited`, `secret_reload`. Among the denials
   above, only the rate-limit rejection and the three guardrail outcomes
   write an audit entry; a `401` auth failure and a `402` budget exhaustion
   do not.

`POST /v1/embeddings` runs the identical chain **minus the guardrail step**.

**RBAC plays no part in this path.** [Role checks](concepts.md#identity--tokens)
gate only the admin plane's `/admin/*` routes. `/scim/v2/*` is gated
separately, by a static shared-secret bearer token compared on every request,
not a role. `/auth/oidc/*` carries no auth wrapper at all — it is the public
login/callback flow. Org scoping on `/v1/*` comes entirely from the virtual
key's own org, not from a role evaluation of any kind.

### An agent run

1. A client calls `POST /runs` (or `/runs/stream`) on the runtime.
2. The agent loop starts (or resumes) a durable, checkpointed
   [thread](concepts.md#agents).
3. Each model call the agent makes re-enters the gateway on the runtime's
   virtual key and goes through the model-call flow above.
4. A tool call reaches a legacy system over MCP through a connector, or runs
   code via `run_python` in the sandbox.
5. **HITL interrupts here, if configured:** when a called tool is in
   `AGENTOS_APPROVAL_TOOLS`, the graph pauses before executing it and the run
   returns `pending_approval`; a human calls
   `POST /runs/{thread_id}/approve` to let it proceed or deny it. Off by
   default, and not available in the `deep` agent profile.
6. Progress is checkpointed durably at each step, so the run survives a
   restart.
7. The run completes and returns its result (or exits earlier on the paused
   `pending_approval` state).

## Data and persistence

Postgres is the only stateful store in the platform. It holds gateway state
(virtual keys, orgs, users, usage, the audit log), agent checkpoints for
durable and resumable threads, and pgvector document embeddings for retrieval.
The sandbox is stateless between runs. Connectors hold no application state of
their own — they front a legacy system and nothing else.

## The credential invariant

Stated once, plainly, because everything above is a consequence of it: **the
runtime never holds a provider credential.** Every action the platform takes
against a model or a legacy system is authorized and attributed to a caller —
though, as the request-flow section above spells out precisely, not every
outcome is recorded to the audit log. Beyond the SQL and REST connectors,
which are wired in and start by default, every other external integration
(SOAP, browser, SSH, an observability overlay) is opt-in and off by default.
