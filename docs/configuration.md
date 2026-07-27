# Configuration reference

Every `AGENTOS_*` environment variable read by AgentOS, grouped by subsystem.
Copy `deploy/.env.example` to `.env` next to `deploy/compose.yaml` and set
what you use — everything else falls back to the default shown here.
Defaults below are read from the Go/Python/Rust source, not from `.env.example`
comments; where the two disagree, source wins.

A dash (`—`) in the Default column means the variable is required for that
feature and has no built-in fallback.

CI-only mock-service variables (`AGENTOS_MOCK_OIDC_ISSUER`,
`AGENTOS_MOCK_OIDC_PORT`, `AGENTOS_MOCK_VAULT_PORT`, `AGENTOS_MOCK_VAULT_TOKEN`)
are documented in [`operations.md`](operations.md), not here — they configure
the mock OIDC/Vault servers the smoke tests bring up, not a deployed service.

## Core

| Variable | Default | Purpose |
|---|---|---|
| `AGENTOS_ADMIN_KEY` | — | Root key for the gateway admin API (`/admin/*`). Required; the gateway refuses to start without it. |
| `AGENTOS_RUNTIME_KEY` | `agos-local-dev-runtime` | The virtual key the runtime authenticates to the gateway with. Compose bootstraps it into the default org with a $25 budget via `AGENTOS_BOOTSTRAP_KEYS`. |
| `AGENTOS_RUNTIME_AUTH_TOKEN` | — | Shared bearer token every runtime API caller must present, on every route except `GET /healthz`. Required; the runtime refuses to start without it. The console's nginx injects it as a Bearer token when proxying `/api/runtime/*` — it never reaches the browser. |
| `AGENTOS_DATABASE_URL` | *(unset)* | Postgres connection string for the gateway's store. Unset falls back to an in-memory store (state lost on restart). |
| `AGENTOS_BOOTSTRAP_ORG` | `default` | Name of the org that owns every pre-existing/bootstrapped key, created with an unlimited (0) budget at startup. |
| `AGENTOS_BOOTSTRAP_KEYS` | *(unset)* | One or more `name:secret:budget_usd` entries (comma-separated) minted into the bootstrap org at startup. Compose sets one entry for the runtime key. |

## Models & providers

| Variable | Default | Purpose |
|---|---|---|
| `AGENTOS_MODEL` | `anthropic/claude-sonnet-5` | Model the agent uses, as `<provider>/<model>` (e.g. `openai/gpt-4o-mini`, `ollama/llama3.1`). |
| `AGENTOS_ANTHROPIC_API_KEY` | *(unset)* | Anthropic provider credential. |
| `AGENTOS_OPENAI_API_KEY` | *(unset)* | OpenAI provider credential. |
| `AGENTOS_OLLAMA_BASE_URL` | *(unset)* | Base URL of a local Ollama instance (e.g. `http://host.docker.internal:11434`). |
| `AGENTOS_EMBED_MODEL` | `ollama/bge-m3` | Embedding model for the context engine, routed via the gateway. |
| `AGENTOS_JUDGE_MODEL` | `anthropic/claude-haiku-4-5` | Model used to judge eval cases that carry a `judge` block, routed via the gateway. |
| `AGENTOS_PROVIDERS_FILE` | `providers.json` (relative to the gateway's working directory; Compose mounts it at `/app/providers.json`) | Path to the operator-configured provider registry (see "Frontier providers" below). A missing file is not fatal — the gateway falls back to built-in providers only, and logs each per-entry validation error rather than refusing to start. |
| `AGENTOS_MOONSHOT_API_KEY` | *(unset)* | Credential for the `moonshot` entry in the provider registry. |
| `AGENTOS_ZHIPU_API_KEY` | *(unset)* | Credential for the `zhipu` entry in the provider registry. |
| `AGENTOS_ALIBABA_API_KEY` | *(unset)* | Credential for the `alibaba` entry in the provider registry. |
| `AGENTOS_DEEPSEEK_API_KEY` | *(unset)* | Credential for the `deepseek` entry in the provider registry. |
| `AGENTOS_MINIMAX_API_KEY` | *(unset)* | Credential for the `minimax` entry in the provider registry. |

**Frontier providers.** The five keys above correspond to entries in
`deploy/providers.json`, all shipped with `"enabled": false`. That file's own
top-level comment states its base URLs, model ids, and per-1M-token prices
are **unverified placeholders**, not vendor-confirmed figures — because those
prices drive budget enforcement, confirm each one against the vendor's current
documentation before setting the matching key and flipping `enabled` to
`true`. Leaving a key empty keeps that provider unroutable regardless of the
`enabled` flag.

## Governance

Controls enforced on the gateway's `/v1/*` request path (auth → rate limit →
budget hold → guardrail → provider/council → audit).

| Variable | Default | Purpose |
|---|---|---|
| `AGENTOS_GUARDRAILS_MODE` | `off` | Prompt-injection guardrail mode: `off` \| `log` \| `block` \| `model`. Plural `GUARDRAILS` — not a typo. |
| `AGENTOS_GUARDRAILS_MODEL` | `anthropic/claude-haiku-4-5` | Classifier model used when the mode is `model`. |
| `AGENTOS_GUARDRAILS_KEY` | *(unset)* | Virtual key the classifier's own spend is attributed to in `/admin/usage`. If it doesn't name a known key, classifier spend is untracked (a warning is logged; screening still works). |
| `AGENTOS_GUARDRAILS_TIMEOUT_S` | `20` | Screening budget in seconds, bounding both classifier attempts combined. |
| `AGENTOS_GUARDRAILS_MAX_TOKENS` | `512` | Token cap on the classifier's completion; raise it for reasoning-model classifiers. |
| `AGENTOS_APPROVAL_TOOLS` | *(empty = off)* | Comma-separated tool names that require human-in-the-loop approval before execution. |
| `AGENTOS_RATE_LIMIT_RPM` | `0` (unlimited) | Global default requests-per-minute applied to orgs that haven't set their own limit. |
| `AGENTOS_RATELIMIT_BACKEND` | `memory` | Rate-limit store: `memory` (per-instance) \| `postgres` (shared across gateway replicas; requires `AGENTOS_DATABASE_URL`). |
| `AGENTOS_BUDGET_RESERVE_USD` | `0.05` | Amount held against a key's budget while a request is in flight, settled once the provider answers. Values ≤ 0 are ignored (the hold that closes the admission race can't be disabled this way). |
| `AGENTOS_MAX_BODY_BYTES` | `10485760` (10 MiB) | Cap on a single request body. A value ≤ 0 removes the cap. |
| `AGENTOS_AUDIT_RETENTION_DAYS` | *(unset; off)* | Days of audit history to retain; older entries are pruned in the background. Off by default — deciding to discard audit history is a decision an operator must make explicitly. |
| `AGENTOS_CORS_ORIGINS` | *(unset; no CORS)* | Comma-separated allowed origins for the gateway's `/v1/*` API. The console doesn't need this — its nginx proxies same-origin. |

## Identity & secrets

| Variable | Default | Purpose |
|---|---|---|
| `AGENTOS_SECRETS_BACKEND` | `env` | Secrets backend: `env` \| `file` \| `age` \| `vault`. |
| `AGENTOS_SECRETS_FILE` | *(unset)* | Path to the secrets file (`file`/`age` backends). |
| `AGENTOS_SECRETS_AGE_KEY` | *(unset)* | `age` identity used to decrypt the secrets file (`age` backend). |
| `AGENTOS_SECRETS_REFRESH_S` | `0` (off) | Background secret re-fetch interval in seconds, for `file`/`age`/`vault` backends only. `0` disables the refresh loop; a rotated key otherwise takes effect only via `POST /admin/secrets/reload` or a restart. |
| `AGENTOS_VAULT_ADDR` | *(unset)* | Vault server address (`vault` backend), e.g. `http://vault:8200`. |
| `AGENTOS_VAULT_TOKEN` | *(unset)* | Vault auth token (`vault` backend). |
| `AGENTOS_VAULT_KV_PATH` | *(unset)* | KV v2 mount path Vault secrets are read from. |
| `AGENTOS_OIDC_ISSUER` | *(unset; SSO disabled)* | OIDC issuer URL. Setting this is what turns SSO on; its discovery document is expected at `{issuer}/.well-known/openid-configuration`. Misconfiguration here is fatal at gateway startup. |
| `AGENTOS_OIDC_CLIENT_ID` | — | OIDC client id. Required once `AGENTOS_OIDC_ISSUER` is set. |
| `AGENTOS_OIDC_CLIENT_SECRET` | — | OIDC client secret. Required once `AGENTOS_OIDC_ISSUER` is set. |
| `AGENTOS_OIDC_REDIRECT_URL` | — | Callback URL registered with the IdP, e.g. `http://localhost:8080/auth/oidc/callback`. |
| `AGENTOS_OIDC_POST_LOGIN_URL` | — | Where the browser lands after a successful login, e.g. `http://localhost:3000/`. |
| `AGENTOS_OIDC_DEFAULT_ORG` | `org_default` | Org a new SSO user is provisioned into on first login. |
| `AGENTOS_OIDC_DEFAULT_ROLE` | `member` | Role a new SSO user is provisioned with on first login. |
| `AGENTOS_SCIM_TOKEN` | *(unset)* | Static bearer secret that guards every `/scim/v2/*` route. Unset leaves all SCIM routes 404 — setting it is what enables provisioning; the IdP sends it as its Bearer token. |
| `AGENTOS_SCIM_DEFAULT_ORG` | `org_default` | Org a user provisioned via SCIM lands in by default. |
| `AGENTOS_SCIM_DEFAULT_ROLE` | `member` | Role a user provisioned via SCIM is given by default. |

## Runtime & agents

| Variable | Default | Purpose |
|---|---|---|
| `AGENTOS_GATEWAY_URL` | — | Base URL of the gateway the runtime calls for every model request, e.g. `http://gateway:8080`. Required. |
| `AGENTOS_GATEWAY_KEY` | — | Virtual key (`agos-...`) the runtime authenticates to the gateway with. Required. |
| `AGENTOS_AGENT_PROFILE` | `react` | Agent architecture: `react` (default single-agent loop) or `deep` (deepagents planner). |
| `AGENTOS_MCP_SERVERS` | `""` (empty) | Comma-separated streamable-HTTP MCP server URLs the agent's tools are loaded from. **Read this before enabling any connector** — see "Connector wiring" below; it is not simply settable from `.env` in the shipped Compose file. |
| `AGENTOS_APPROVAL_TOOLS` | `""` (empty = HITL off) | Same variable as in Governance, consumed by the runtime side of the approval flow. |
| `AGENTOS_MAX_CONTEXT_TOKENS` | `0` (off) | Caps the message history resent to the model each turn on the `react` profile; oldest messages are dropped first, newest plus the system prompt kept. `0` disables trimming. The `deep` profile ignores this — it summarizes context on its own. |
| `AGENTOS_CONTEXT_ENGINE` | `""` (empty) | `on` \| `off` \| empty. Empty means "on if a checkpoint database is configured, off otherwise." |
| `AGENTOS_CHECKPOINT_DATABASE_URL` | *(unset; in-memory checkpoints)* | Postgres URL used for LangGraph checkpoints (conversation state, operator runs, council runs). Unset means checkpoints don't survive a restart, and Operators/self-improvement stay disabled (see their sections below). |
| `AGENTOS_CHECKPOINT_DB` | — | Not a real variable: this is the name one console page's empty-state copy uses for `AGENTOS_CHECKPOINT_DATABASE_URL`. Setting `AGENTOS_CHECKPOINT_DB` does nothing — set `AGENTOS_CHECKPOINT_DATABASE_URL` instead. |
| `AGENTOS_SANDBOX_URL` | `""` (empty = `run_python` tool disabled) | Sandbox service base URL, e.g. `http://sandbox:8070`. See the Sandbox group below. |
| `AGENTOS_OTEL_ENDPOINT` | *(unset; tracing disabled)* | OTLP/HTTP endpoint the runtime exports traces to. See Observability. |
| `AGENTOS_SKILLS_DIR` | `""` (empty = the image-baked `runtime/skills/`) | Directory `SKILL.md` files are loaded from. See Operators & skills. |

## Council

The Multiverse council: a governed multi-agent objective runner, served by the
runtime and proxied through the gateway as the `council/*` model.

| Variable | Default | Purpose |
|---|---|---|
| `AGENTOS_COUNCIL_RUNTIME_URL` | `http://runtime:8000` (Compose default) | Runtime base URL the gateway proxies `council/*` model calls to. Unset on the gateway disables the council model entirely (no `council/*` route is registered). |
| `AGENTOS_COUNCIL_CONFIG` | `""` (runtime built-in default; empty disables the council). Compose sets it to `/app/council.yaml`, an image-baked file, so the council is enabled out of the box in the shipped stack. | Path to the council's member/model configuration file. |
| `AGENTOS_COUNCIL_HEARTBEAT_S` | `0` (off) | Interval, in seconds, at which the council checks for due autonomous objectives. `0` means the council serves its API but never acts on its own. |
| `AGENTOS_COUNCIL_MAX_SPEND_USD` | `5.0` (runtime built-in default; Compose sets the same value explicitly) | Default per-objective spend ceiling. |

## Operators & skills

Single-agent, always-on autonomy (distinct from the council above).

| Variable | Default | Purpose |
|---|---|---|
| `AGENTOS_AUTONOMY_ENABLED` | `false` | Turns the operator scheduler on. The runtime always serves the `/operators` API; when this is `false` nothing fires on its own — interval/cron operators need it `true`, webhook operators fire on their endpoint regardless. |
| `AGENTOS_AUTONOMY_TICK_S` | `15` | How often, in seconds, the scheduler checks for due operators. |
| `AGENTOS_AUTONOMY_MAX_CYCLES` | `8` | Default tool-iteration cap for an operator that doesn't set its own. |
| `AGENTOS_SKILLS_DIR` | `""` (empty = the image-baked `runtime/skills/`) | In-repo `SKILL.md` directory. Skills are never fetched at runtime — only loaded from this directory — and each load records a sha256 digest for provenance (it is logged, not compared against a pinned expected value). |

## Sandbox

Egress-less Rust service the `run_python` tool executes code in. No `ports:`
mapping in Compose — it lives only on the internal `sandbox-net`, reachable
solely from the runtime.

| Variable | Default | Purpose |
|---|---|---|
| `AGENTOS_SANDBOX_URL` | `""` (empty = tool disabled) | Runtime-side: where the sandbox is reachable, e.g. `http://sandbox:8070`. |
| `AGENTOS_SANDBOX_MAX_TIMEOUT_S` | `30` | Sandbox-side: upper bound a caller's requested `timeout_s` is clamped to. |
| `AGENTOS_SANDBOX_MAX_OUTPUT_BYTES` | `65536` | Sandbox-side: per-stream cap on captured stdout/stderr. |

## Connectors

MCP tool servers. Read the "Connector wiring" section further down before
enabling any of these — starting a connector's container is not the same as
making its tools reachable by the agent.

**SQL** (read-only queries against the seeded legacy ERP):

| Variable | Default | Purpose |
|---|---|---|
| `AGENTOS_CONNECTOR_DATABASE_URL` | — | Postgres connection string for the read-only ERP role. Required. |
| `AGENTOS_CONNECTOR_MAX_ROWS` | `200` | Row cap applied to every `query` result. |
| `AGENTOS_CONNECTOR_STATEMENT_TIMEOUT_S` | `30` | Per-query `statement_timeout`; bounds how long a query may run (independent of the row cap). |

**REST** (OpenAPI 3 spec turned into MCP tools):

| Variable | Default | Purpose |
|---|---|---|
| `AGENTOS_REST_SPEC_URL` | — | URL or file path of the OpenAPI 3 JSON spec. Required. |
| `AGENTOS_REST_BASE_URL` | *(unset; falls back to the spec's first declared server)* | Upstream base URL calls are proxied to. Required if the spec declares no `servers`. |
| `AGENTOS_REST_ALLOW_MUTATIONS` | `false` | Whether non-GET operations are exposed as tools. |
| `AGENTOS_REST_MAX_BODY_BYTES` | `65536` | Response-body cap per call. |
| `AGENTOS_REST_AUTH_HEADER` | *(unset)* | A single `"Name: value"` header attached to every upstream call. |

**SOAP** (WSDL 1.1 turned into MCP tools; opt-in — `docker compose --profile connectors` or `--profile soap`):

| Variable | Default | Purpose |
|---|---|---|
| `AGENTOS_SOAP_WSDL_URL` | — | URL or file path of the WSDL 1.1 document. Required. |
| `AGENTOS_SOAP_ENDPOINT` | *(unset; falls back to the WSDL's declared `soap:address`)* | SOAP service endpoint. Required if the WSDL declares none. |
| `AGENTOS_SOAP_ALLOW_OPERATIONS` | `""` (empty = none allowed) | Comma-separated operation allowlist. |
| `AGENTOS_SOAP_TIMEOUT_S` | `20` | Upstream request timeout. |
| `AGENTOS_SOAP_MAX_BODY_BYTES` | `131072` | Response-body cap per call. |
| `AGENTOS_SOAP_AUTH_HEADER` | *(unset)* | A single `"Name: value"` header attached to every upstream call. |

**SSH** (allowlisted command execution on one legacy host; opt-in, no Compose service, no Helm template — run it standalone):

| Variable | Default | Purpose |
|---|---|---|
| `AGENTOS_SSH_HOST` | — | Remote host to connect to. Required. |
| `AGENTOS_SSH_PORT` | `22` | **Remote SSH target port** — the port on the remote host the connector dials. This is *not* the connector's own listen port, which is the hardcoded `:8092`. |
| `AGENTOS_SSH_USER` | — | Remote SSH username. Required. |
| `AGENTOS_SSH_PASSWORD` | *(unset)* | Password auth. Exactly one of this or `AGENTOS_SSH_PRIVATE_KEY` must be set. |
| `AGENTOS_SSH_PRIVATE_KEY` | *(unset)* | PEM private key auth. Exactly one of this or `AGENTOS_SSH_PASSWORD` must be set. |
| `AGENTOS_SSH_ALLOW_COMMANDS` | `""` (empty = every command denied) | Comma-separated allowlist of command basenames (e.g. `ls,cat,grep,df,uptime`). |
| `AGENTOS_SSH_KNOWN_HOSTS` | — | Path to a `known_hosts` file. **Required**: with this unset the connector refuses to start (fail-closed), unless `AGENTOS_SSH_INSECURE_HOST_KEY=true` is set as an explicit opt-out. |
| `AGENTOS_SSH_INSECURE_HOST_KEY` | `false` | Set to `true` to start with host-key verification disabled (`InsecureIgnoreHostKey`) when `AGENTOS_SSH_KNOWN_HOSTS` is unset. Logs a loud startup warning. Development only — never set this in production. |
| `AGENTOS_SSH_TIMEOUT_S` | `15` | Per-command timeout. |
| `AGENTOS_SSH_MAX_OUTPUT_BYTES` | `65536` | Captured stdout/stderr cap. |

**Browser** (headless navigation with a domain allowlist; opt-in — `docker compose --profile connectors` or `--profile browser`):

| Variable | Default | Purpose |
|---|---|---|
| `AGENTOS_BROWSER_ALLOW_DOMAINS` | `""` (empty = every domain denied) | Comma-separated domain allowlist for `navigate`. |
| `AGENTOS_BROWSER_TIMEOUT_S` | `20` | Navigation timeout. |
| `AGENTOS_BROWSER_MAX_TEXT` | `8000` | Character cap on `get_text` output. |

## Connector wiring (read this before enabling a connector)

Bringing a connector's container up does not, by itself, make its tools
reachable by the agent. What actually wires a connector's MCP endpoint in is
`AGENTOS_MCP_SERVERS`, and each connector tier handles that differently:

- **SQL + REST** are wired in by default, but as a **hardcoded literal** in
  `deploy/compose.yaml`:
  `AGENTOS_MCP_SERVERS: http://sql-connector:8090/mcp,http://rest-connector:8091/mcp`.
  There is no `${...}` substitution on that line, so it **cannot be
  overridden from `.env`** in the shipped Compose file — changing it means
  editing `compose.yaml` directly (or using Helm, below). Both have Helm
  templates; `restConnector.enabled` defaults to **`false`** in
  `deploy/helm/agentos/values.yaml`.
- **SOAP + browser** start only under `docker compose --profile connectors`
  (or the narrower `--profile soap` / `--profile browser`), and starting them
  does **not** make their tools reachable — their URLs must still be appended
  to `AGENTOS_MCP_SERVERS` by hand-editing `deploy/compose.yaml`. Neither has
  a Helm template.
- **SSH** has **no Compose service and no Helm template at all**. Run its
  binary standalone (see `connectors/ssh/README.md`) and add its URL to
  `AGENTOS_MCP_SERVERS` yourself.
- **Helm** composes `AGENTOS_MCP_SERVERS` from whichever connector services
  are enabled, and additionally supports `runtime.extraMcpServers`
  (`deploy/helm/agentos/values.yaml`) — "Extra MCP server URLs appended to
  the composed `AGENTOS_MCP_SERVERS`" — which is the supported extension
  point for connectors Helm doesn't template (SOAP, browser, SSH) or any
  MCP server outside this repo.

## Console & networking

| Variable | Default | Purpose |
|---|---|---|
| `AGENTOS_CORS_ORIGINS` | *(unset; no CORS)* | See Governance — listed again here because it is the variable operators reach for when a browser client other than the console needs to call the gateway directly. |

The console itself takes no configuration of its own beyond
`AGENTOS_RUNTIME_AUTH_TOKEN` (Core) and `AGENTOS_CONSOLE_PORT` (Ports): its
nginx proxies `/api/gateway/*` and `/api/runtime/*` same-origin and injects
the runtime bearer token server-side, so the browser never holds it.

## Observability

| Variable | Default | Purpose |
|---|---|---|
| `AGENTOS_OTEL_ENDPOINT` | *(unset; tracing disabled)* | OTLP/HTTP base URL both the gateway and the runtime export traces to (e.g. `http://otel-collector:4318`, brought up by the `compose.otel.yaml` overlay). Misconfiguration is fatal at gateway startup. |

## Ports

Host-side port mappings from `deploy/compose.yaml`. Each variable controls
only the *host* side of the mapping — the container's own listen port is
fixed and unrelated to it (e.g. the console listens on `8080` inside its
container regardless of what `AGENTOS_CONSOLE_PORT` is set to on the host).

| Variable | Default | Purpose |
|---|---|---|
| `AGENTOS_PG_PORT` | `5432` | Postgres host port. |
| `AGENTOS_GATEWAY_PORT` | `8080` | Gateway host port. |
| `AGENTOS_RUNTIME_PORT` | `8000` | Runtime host port. |
| `AGENTOS_CONNECTOR_PORT` | `8090` | SQL connector host port. |
| `AGENTOS_REST_PORT` | `8091` | REST connector host port. |
| `AGENTOS_SOAP_PORT` | `8093` | SOAP connector host port (only relevant once started under its profile). |
| `AGENTOS_BROWSER_PORT` | `8094` | Browser connector host port (only relevant once started under its profile). |
| `AGENTOS_CRM_PORT` | `8095` | Demo CRM (legacy REST target) host port. |
| `AGENTOS_CONSOLE_PORT` | `3000` | Console host port (the container listens on `8080` internally; this is the one port in this table where the host side differs from the container's own listen port). |
| `AGENTOS_OTEL_PORT` | `4318` | OTel collector host port (`compose.otel.yaml` overlay). |
| `AGENTOS_LANGFUSE_PORT` | `3001` | Langfuse UI host port (`compose.langfuse.yaml` overlay). |
| `AGENTOS_SANDBOX_PORT` | `8070` | Sandbox's own listen port. No Compose host mapping exists — the sandbox is reachable only from the runtime, over the internal `sandbox-net`. |
| — | `8092` (hardcoded, not an env var) | The SSH connector's own listen port. Do not confuse this with `AGENTOS_SSH_PORT`, which is the *remote* SSH target port (see Connectors above). No Compose service and no host mapping exist for it. |

## Interop: OpenClaw overlay

| Variable | Default | Purpose |
|---|---|---|
| `AGENTOS_OPENCLAW_KEY` | — | A scoped `agos-` virtual key with its own budget and rate limit, used only by the optional governed OpenClaw worker overlay (`deploy/openclaw/`). Not read by any AgentOS service directly — see `docs/interop/openclaw.md`. |
