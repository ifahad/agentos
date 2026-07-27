# Operations

Verification targets, demo fixtures, CI test doubles, and the egress-isolation
proof for the sandbox. This is where `make test`/`make smoke*` and the
supporting fixtures get their first real documentation — several other pages
link here rather than repeat this material.

## Verification targets

All of the following require the stack to be up (`make up`). Run against a
fresh Postgres volume the first time (`make down && make up` clears it); a few
targets depend on state a prior target ingested, noted below.

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

**Ordering dependency:** several targets assume prior state on a fresh
volume. `smoke3` and `smoke4` need the vendor-policy document `smoke2`
ingests — run `smoke2` once first on a fresh volume before either. The other
targets do not depend on one another's state.

## Test targets

| Target | Runs |
|---|---|
| `make test` | `test-go` + `test-python` + `test-console` |
| `make test-go` | `go vet` + `go test` for `gateway/` and `connectors/sql/` only |
| `make test-python` | `ruff check` + `pytest` for `runtime/` |
| `make test-console` | `npm test -- --run` + `npm run build` for `console/` |
| `make test-rust` | `cargo fmt --check` + `cargo clippy` + `cargo test` for `sandbox/` |
| `make fmt` | Formats Go (`gofmt`), Python (`ruff format`) |

**Three traps, worth knowing before you trust a green run:**

1. `make test` **excludes `make test-rust`** — the Rust sandbox suite is not
   part of the `test` aggregate target and must be run separately:
   `make test-rust`.
2. `make test-go` covers **only `gateway/` and `connectors/sql/`**. The REST,
   SOAP, SSH, and browser connector suites are not wired into any `make`
   target; run them from their own directories (`connectors/rest/`,
   `connectors/soap/`, `connectors/ssh/`, `connectors/browser/`) with each
   language's own test command. CI does cover the three Go ones — its `go` job
   runs `connectors/rest`, `connectors/ssh`, and `connectors/soap` alongside
   the gateway and `connectors/sql`.
3. **`connectors/browser/tests/` runs in no `make` target and no CI job.** The
   browser connector is Python, so the `go` job never sees it, and the `python`
   job runs only against `runtime/` (`.github/workflows/ci.yml`). A green CI
   run says nothing about it — run it by hand from `connectors/browser/` when
   you change that connector.

## Demo fixtures

Two fixtures back the demo connectors so the verification targets above have
something real to exercise.

- **`deploy/initdb/01-legacy-erp.sql`** — seeds a `legacy_erp` Postgres
  database (run once, via the `postgres` image's entrypoint) behind a
  read-only `erp_reader` role. It creates `customers`, `orders`, and
  `invoices` tables with sample rows — the "legacy ERP" the SQL connector
  exposes to agents as read-only MCP tools.
- **`deploy/demo-crm/`** — a small Go HTTP service with a hand-written
  OpenAPI 3 document served at `/openapi.json`. It is the REST connector's
  default target: `rest-connector` points `AGENTOS_REST_SPEC_URL` at
  `http://demo-crm:8095/openapi.json` in `deploy/compose.yaml`, and the
  connector turns its operations into MCP tools.

## CI mocks

`deploy/ci/` holds three stdlib-only Python test doubles, each replacing a
real external dependency so a code path can be exercised offline with nothing
but `python3`:

| Script | Stands in for | Used by |
|---|---|---|
| `deploy/ci/mock-model.py` | An OpenAI-compatible LLM provider (`/v1/chat/completions`, `/v1/embeddings`) with deterministic, canned responses | `.github/workflows/evals.yml` — lets the eval gate score the runtime suite with no real provider, no network, and the same score every run |
| `deploy/ci/mock-oidc.py` | A minimal OIDC identity provider (discovery, `/authorize`, `/token`, `/jwks`, real RS256-signed ID tokens) | `make smoke6`, via `deploy/compose.auth-mocks.yaml` — exercises the gateway's OIDC SSO flow end-to-end |
| `deploy/ci/mock-vault.py` | A HashiCorp Vault KV v2 endpoint (`GET /v1/<path>` with `X-Vault-Token`) | `make smoke6`, via `deploy/compose.auth-mocks.yaml` — exercises `AGENTOS_SECRETS_BACKEND=vault` |

`deploy/compose.auth-mocks.yaml` brings up `mock-oidc` and `mock-vault` as
Compose services:

```bash
docker compose -f deploy/compose.yaml -f deploy/compose.auth-mocks.yaml up -d
```

The four `AGENTOS_MOCK_OIDC_ISSUER`, `AGENTOS_MOCK_OIDC_PORT`,
`AGENTOS_MOCK_VAULT_TOKEN`, and `AGENTOS_MOCK_VAULT_PORT` variables configure
these mocks (not a deployed service, which is why they live here rather than
in [`docs/configuration.md`](configuration.md)):

| Variable | Compose sets it to | Purpose |
|---|---|---|
| `AGENTOS_MOCK_OIDC_ISSUER` | `http://mock-oidc:9000` | Public base URL the gateway uses to reach the mock OIDC server; echoed verbatim as the discovery document's `issuer` (the script's own fallback if unset is `http://localhost:<port>`) |
| `AGENTOS_MOCK_OIDC_PORT` | `9000` (via `${AGENTOS_MOCK_OIDC_PORT:-9000}` host port mapping) | Host port the mock OIDC server binds to |
| `AGENTOS_MOCK_VAULT_TOKEN` | `test-token` | Token the mock Vault requires in `X-Vault-Token`; set the gateway's `AGENTOS_VAULT_TOKEN` to the same value |
| `AGENTOS_MOCK_VAULT_PORT` | `8200` (via `${AGENTOS_MOCK_VAULT_PORT:-8200}` host port mapping) | Host port the mock Vault server binds to |

## Egress verification

[`deploy/SANDBOX_EGRESS_VERIFY.md`](../deploy/SANDBOX_EGRESS_VERIFY.md) is a
recorded verification run proving that a container on the internal
`sandbox-net` Docker network can neither resolve nor reach the public
internet — only the runtime, also attached to `sandbox-net`, has a route to
the sandbox.

## Helm render check

```bash
deploy/helm/test-render.sh
```

Runs `helm lint --strict` plus `helm template` under several value
combinations (defaults, external database, rest-connector + demo-crm enabled,
ingress enabled, sandbox NetworkPolicy on/off) and asserts specific strings in
the rendered manifests — see [`docs/deployment.md`](deployment.md#kubernetes-helm)
for what those toggles mean.
