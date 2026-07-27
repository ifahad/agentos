# Deployment

How to run AgentOS: Docker Compose for a laptop or single host, the optional
overlays that layer governance and observability on top of it, the connector
deployment tiers (which connectors are reachable to an agent out of the box),
the Helm chart for Kubernetes, and the CI/eval-gate pipeline.

See [`docs/configuration.md`](configuration.md) for every environment
variable, and [`docs/operations.md`](operations.md) for the verification
(`make smoke*`) targets, demo fixtures, and CI mocks referenced below.

## Compose (laptop)

Prereqs: Docker + Compose, one provider API key (or a local Ollama).

```bash
cd deploy
cp .env.example .env        # set AGENTOS_ANTHROPIC_API_KEY (or OpenAI/Ollama)
cd ..
make up                     # docker compose -f deploy/compose.yaml --env-file deploy/.env up -d --build
```

`make up` builds and starts the services that are on by default: `postgres`,
`gateway`, `sql-connector`, `rest-connector`, `runtime`, `sandbox`,
`demo-crm`, and `console`. `sql-connector` and `rest-connector` are wired into
the runtime's tool list out of the box — see [Connector tiers](#connector-tiers)
below for exactly what that means and what it does not. `soap-connector` and
`browser-connector` are opt-in and do not start with `make up`.

The console is reachable at **http://localhost:3000** (the container listens
on 8080 internally; Compose publishes it on host port 3000, overridable with
`AGENTOS_CONSOLE_PORT`). Sign in with the admin key from `.env`
(`AGENTOS_ADMIN_KEY`, default `admin-local-dev` for local dev) or, once
configured, OIDC SSO.

Other base targets:

```bash
make down     # docker compose -f deploy/compose.yaml down -v
make logs     # docker compose -f deploy/compose.yaml logs -f
```

`make down` removes the Postgres volume (`-v`), so a fresh `make up` reseeds
the demo data described in [`docs/operations.md`](operations.md#demo-fixtures).

## Overlays

Each overlay is an additional `-f` file layered on top of `deploy/compose.yaml`.
Overlays compose freely (e.g. HITL + OTel), except where noted.

### HITL (human-in-the-loop governance)

```bash
docker compose -f deploy/compose.yaml -f deploy/compose.hitl.yaml up -d
```

Sets `AGENTOS_APPROVAL_TOOLS=query` on the runtime (the `query` tool now
requires human approval before it runs) and `AGENTOS_GUARDRAILS_MODE=block`
on the gateway (guardrail hits are blocked rather than merely logged).

### OTel (observability)

```bash
docker compose -f deploy/compose.yaml -f deploy/compose.otel.yaml up -d
```

Adds an OpenTelemetry collector (`otel/opentelemetry-collector-contrib`) and
points `AGENTOS_OTEL_ENDPOINT` on both gateway and runtime at it, so every
request, model call, and tool call emits a span. By default the collector logs
spans to its own debug exporter; edit `deploy/otel-collector.yaml` (commented
example inside) to forward elsewhere, or use the Langfuse overlay below.

### Langfuse (bundled observability UI)

```bash
docker compose -f deploy/compose.yaml -f deploy/compose.otel.yaml \
  -f deploy/compose.langfuse.yaml up -d
```

This is layered on top of the OTel overlay (gateway/runtime must already be
emitting spans for Langfuse to have anything to show), and is the canonical
place this repo documents Langfuse setup — `deploy/LANGFUSE_DOCS_SNIPPET.md`
and the observability material formerly in the README both point here.

It brings up Langfuse OSS (`${LANGFUSE_IMAGE:-langfuse/langfuse:2}`) with its
own dedicated Postgres, separate from the AgentOS database, and swaps in
`otel-collector.langfuse.yaml`, which exports traces to both the debug log and
Langfuse. First run:

1. Open **http://localhost:3001** (override with `AGENTOS_LANGFUSE_PORT`),
   sign up, and create an organization and project.
2. In the project settings, copy the API key pair (`pk-lf-...` / `sk-lf-...`).
3. Base64-encode `"pk:sk"` into `.env` as `LANGFUSE_OTLP_BASIC_AUTH` — see
   `deploy/langfuse.env.example` for this and the other `LANGFUSE_*` variables
   (`AGENTOS_LANGFUSE_PORT`, `LANGFUSE_NEXTAUTH_SECRET`, `LANGFUSE_SALT`,
   `LANGFUSE_OTLP_ENDPOINT`):

   ```bash
   echo "LANGFUSE_OTLP_BASIC_AUTH=$(echo -n 'pk-lf-xxx:sk-lf-xxx' | base64 -w0)" >> .env
   ```

4. Restart the collector so it picks up the credentials:

   ```bash
   docker compose -f deploy/compose.yaml -f deploy/compose.otel.yaml \
     -f deploy/compose.langfuse.yaml up -d --force-recreate otel-collector
   ```

**Langfuse v3 caveat:** native OTLP ingestion (`/api/public/otel`) landed in
Langfuse **v3**. The bundled profile pins the lightweight, Postgres-only
`langfuse:2` image, whose collector export path is therefore best-effort — the
collector's `debug` exporter always shows your traces regardless of whether
Langfuse ingests them. To actually ingest spans into the Langfuse UI, set
`LANGFUSE_IMAGE=langfuse/langfuse:3` in `.env` and add the v3 dependencies
(ClickHouse/Redis/MinIO), or point `LANGFUSE_OTLP_ENDPOINT` at a Langfuse
Cloud project. The image tag is env-overridable — no file edits needed.

### auth-mocks (mock OIDC + mock Vault)

```bash
docker compose -f deploy/compose.yaml -f deploy/compose.auth-mocks.yaml up -d
```

Brings up a mock OIDC provider and a mock Vault so SSO and the Vault secrets
backend can be exercised without real infrastructure. See
[`docs/operations.md`](operations.md#ci-mocks) for what each mock does and
which verification target uses it.

### openclaw (governed OpenClaw worker)

```bash
docker compose -f deploy/compose.yaml -f deploy/openclaw/compose.openclaw.yaml up -d openclaw
```

Runs an OpenClaw worker as a budgeted, audited, contained client of the
AgentOS gateway. [`docs/interop/openclaw.md`](interop/openclaw.md) is
canonical for this overlay — read it before using the command above.

## Connector tiers

Not every connector is reachable to an agent just because its container is
running. A tool is only callable if its MCP server's URL is present in
`AGENTOS_MCP_SERVERS` — this is the gotcha this section exists to document.

- **SQL + REST** — wired into `AGENTOS_MCP_SERVERS` by default. In
  `deploy/compose.yaml` this is a **hardcoded literal**
  (`AGENTOS_MCP_SERVERS: http://sql-connector:8090/mcp,http://rest-connector:8091/mcp`)
  with **no `${}` substitution**, so it cannot be overridden from `.env`. Both
  connectors have Helm templates; Helm's `sqlConnector.enabled` defaults to
  `true`, `restConnector.enabled` defaults to **`false`**.
- **SOAP + browser** — start only under `docker compose --profile connectors`
  (or the narrower `--profile soap` / `--profile browser`). Starting them does
  **not** make their tools reachable to the agent: their URLs must be appended
  to `AGENTOS_MCP_SERVERS` by hand-editing `deploy/compose.yaml`. Neither has
  a Helm template.
- **SSH** — no Compose service and no Helm template at all. Run
  `connectors/ssh/` standalone and add its URL to `AGENTOS_MCP_SERVERS`
  yourself.
- **Helm** composes the MCP list from whichever connectors are enabled and
  additionally supports `runtime.extraMcpServers` — extra MCP server URLs
  appended to the composed `AGENTOS_MCP_SERVERS` (`deploy/helm/agentos/values.yaml`).

See [`docs/api.md`](api.md#tool-catalog) for the full list of tools each
connector exposes once it is reachable.

## Kubernetes (Helm)

`deploy/helm/agentos/` is a Helm 3 chart. Full reference:
[`deploy/helm/agentos/README.md`](../deploy/helm/agentos/README.md). This
section summarizes what matters when deciding what to enable.

Chart templates exist only for: `console`, `demo-crm`, `gateway`, `ingress`,
`postgres`, `rest-connector`, `runtime`, `sandbox` (+
`sandbox-networkpolicy`), `secret`, `sql-connector`. There is **no** SOAP,
browser, or SSH template — those connectors are Compose/standalone-only (see
[Connector tiers](#connector-tiers)).

Per-service `enabled` toggles and their defaults, from
`deploy/helm/agentos/values.yaml`:

| Service | `enabled` default |
|---|---|
| `postgres` | `true` |
| `gateway` | `true` |
| `runtime` | `true` |
| `sqlConnector` | `true` |
| `restConnector` | **`false`** |
| `sandbox` | `true` |
| `console` | `true` |
| `demoCrm` | **`false`** |
| `ingress` | `false` |

Other toggles worth knowing before installing:

- **Database** — `postgres.enabled=true` (default) runs a bundled
  single-replica pgvector StatefulSet with a PVC, seeded with the same demo
  `legacy_erp` database as Compose. With `postgres.enabled=false` you must set
  `externalDatabaseUrl` (gateway store + runtime checkpoints) and, if
  `sqlConnector.enabled`, `sqlConnector.databaseUrl`.
- **Sandbox hardening** — the sandbox pod runs read-only root filesystem,
  `allowPrivilegeEscalation: false`, all capabilities dropped, non-root, and
  RuntimeDefault seccomp, on top of resource limits. `sandbox.networkPolicy.enabled`
  (default `true`) adds a NetworkPolicy restricting the sandbox pod to ingress
  from the runtime pod only and egress to kube-dns (port 53) only — the
  cluster equivalent of the Compose egress-less topology. Requires a CNI that
  enforces NetworkPolicy.
- **Console ingress** — `ingress.enabled` (default `false`) exposes only the
  console; the gateway/runtime APIs are reachable through the console's own
  nginx proxy paths, not directly.
- **Extra MCP servers** — `runtime.extraMcpServers` (values.yaml) is a list of
  additional MCP server URLs appended to the composed `AGENTOS_MCP_SERVERS` —
  the mechanism for wiring in a connector Helm has no template for (SOAP,
  browser, SSH) once you've deployed it yourself and exposed it to the
  cluster.

Verify a chart change renders correctly before installing:

```bash
deploy/helm/test-render.sh
```

This runs `helm lint --strict` plus `helm template` under several value
combinations (defaults, external database, rest-connector + demo-crm enabled,
ingress enabled, NetworkPolicy on/off), asserting specific strings in the
rendered manifests.

## CI and the eval gate

`.github/workflows/ci.yml` runs five independent jobs on every push and pull
request: `go` (gateway + connectors), `python` (runtime), `rust` (sandbox),
`console`, and `helm` (chart render checks via `deploy/helm/test-render.sh`).
Each job is isolated to its own language/service, so a failure in one does not
block the others from reporting.

`.github/workflows/evals.yml` is a separate, opt-in eval gate. It boots the
full Compose stack against a deterministic, offline mock model
(`deploy/ci/mock-model.py` — see
[`docs/operations.md`](operations.md#ci-mocks)) so the runtime eval suite
(`runtime/evals/default.yaml`) runs with no real provider and no network, then
fails the job if the suite's score drops below **0.8**. It triggers on manual
`workflow_dispatch` or on a pull request labelled **`run-evals`** — evals are
heavier than the unit-test CI, so they run only when asked for. The workflow
also documents how to point it at real provider models instead of the mock,
by exporting real provider secrets and dropping the mock-wiring steps.
