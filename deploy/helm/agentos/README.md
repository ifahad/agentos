# agentos Helm chart

Deploys the AgentOS stack to Kubernetes: model **gateway**,
agent **runtime**, **sql-connector** (MCP over the seeded legacy ERP),
optional **rest-connector** (OpenAPI -> MCP) with an optional **demo-crm**
backend, the Rust code **sandbox**, the admin **console**, and a bundled
pgvector **Postgres** (or an external database).

The env contracts match `deploy/compose.yaml` exactly (`AGENTOS_*` names);
inter-service URLs use chart-fullname-based service DNS
(e.g. `AGENTOS_GATEWAY_URL=http://<release>-gateway:8080`).

## Quick start

Images are not published; build and push them to a registry your cluster can
pull from (repos default to `agentos/<service>`, tag defaults to the chart
`appVersion`):

```sh
# from the repo root, for each of gateway runtime connectors/sql \
# connectors/rest sandbox console deploy/demo-crm
docker build -t <registry>/agentos/gateway:0.3.0 gateway/
```

Create the provider-keys secret (either key alone is fine — both are
optional):

```sh
kubectl create secret generic agentos-provider-keys \
  --from-literal=AGENTOS_ANTHROPIC_API_KEY=sk-ant-... \
  --from-literal=AGENTOS_OPENAI_API_KEY=sk-...
```

Install:

```sh
helm install agentos deploy/helm/agentos \
  --set providerKeys.existingSecret=agentos-provider-keys \
  --set adminKey=$(openssl rand -hex 16) \
  --set runtimeKey=agos-$(openssl rand -hex 16)
```

Then open the console: `kubectl port-forward svc/agentos-console 3000:80`.

## Architecture notes

- **Secrets** — `adminKey`, `runtimeKey`, the composed
  `AGENTOS_BOOTSTRAP_KEYS` (`runtime:<runtimeKey>:<runtimeBudget>`), the
  database URLs and the Postgres password live in a chart-managed Secret
  (`<fullname>-secrets`). Provider API keys are **never** stored by the chart;
  they come from `providerKeys.existingSecret` via optional `secretKeyRef`s.
- **Postgres** — `postgres.enabled=true` (default) runs a single-replica
  pgvector StatefulSet with a PVC and seeds the demo `legacy_erp` database
  from this chart's `files/initdb/` (a copy of `deploy/initdb/`) via a
  ConfigMap mounted at `/docker-entrypoint-initdb.d`. With
  `postgres.enabled=false` you must set `externalDatabaseUrl` (gateway store +
  runtime checkpoints) and, if `sqlConnector.enabled`,
  `sqlConnector.databaseUrl` (read-only ERP connection).
- **MCP wiring** — the runtime's `AGENTOS_MCP_SERVERS` is composed from the
  enabled connectors (`sql-connector`, `rest-connector`) plus
  `runtime.extraMcpServers`. `AGENTOS_SANDBOX_URL` is set iff
  `sandbox.enabled`.
- **Sandbox hardening** — read-only root filesystem, `allowPrivilegeEscalation:
  false`, all capabilities dropped, `runAsNonRoot`, RuntimeDefault seccomp,
  emptyDir `/tmp`, and resource limits set by default — on top of the
  in-process rlimit/env-clearing isolation. Residual risk (as in compose):
  in-container network egress; restrict with a NetworkPolicy if your CNI
  supports it.
- **rest-connector** — disabled by default. When enabled it needs
  `restConnector.specUrl`; if `demoCrm.enabled=true` and no spec URL is set,
  it defaults to the demo CRM's `/openapi.json`.
- **Console** — the image's nginx config proxies `/api/gateway/` and
  `/api/runtime/` to the compose hostnames, so the chart replaces it with a
  ConfigMap pointing at the fullname-based services. The ConfigMap is mounted
  over `/etc/nginx/templates/default.conf.template`, the image entrypoint's
  **input** — not over `/etc/nginx/conf.d/default.conf`, its output. The
  entrypoint renders one into the other under `set -eu`, so mounting the output
  path read-only makes that write fail and the container exits before nginx
  starts. (It did exactly that until 2026-08-04.)
- **Console exposure** — `/api/runtime/` is proxied with the runtime bearer
  injected server-side, so the proxy asks the caller for nothing:
  `POST /api/runtime/runs` against the console port runs an agent, with no
  credential. `/api/gateway/` authenticates every request itself, so the two
  paths are not equally protected. The console's sign-in is client-side and
  does not gate the proxy. Treat the console Service as an administrative
  surface, and put authentication in front of `ingress.enabled=true` — the
  chart prints this after install.
- **`runtimeAuthToken` is required** and has no default: a shipped one is a
  published credential for the whole runtime API. Generate it, e.g.
  `--set runtimeAuthToken=$(openssl rand -hex 32)`. It must not contain a `$`
  (the console entrypoint substitutes it into nginx config, where `$` starts a
  variable reference; the entrypoint rejects it up front).

## Values

| Key | Default | Description |
| --- | --- | --- |
| `adminKey` | `admin-local-dev` | Gateway admin key (override in real deployments) |
| `runtimeKey` | `agos-local-dev-runtime` | Runtime's virtual gateway key |
| `runtimeBudget` | `25` | Budget attached to the runtime key in `AGENTOS_BOOTSTRAP_KEYS` |
| `providerKeys.existingSecret` | `""` | Existing Secret with `AGENTOS_ANTHROPIC_API_KEY` / `AGENTOS_OPENAI_API_KEY` |
| `externalDatabaseUrl` | `""` | Used when `postgres.enabled=false` |
| `otel.endpoint` | `""` | Sets `AGENTOS_OTEL_ENDPOINT` on gateway + runtime |
| `postgres.enabled` | `true` | Bundled pgvector StatefulSet |
| `postgres.persistence.size` | `8Gi` | PVC size (set `persistence.enabled=false` for emptyDir) |
| `<svc>.enabled` | see values | Per-service toggle (`gateway`, `runtime`, `sqlConnector`, `restConnector`, `sandbox`, `console`, `demoCrm`) |
| `<svc>.image.repository/tag/pullPolicy` | `agentos/<svc>` / appVersion / `IfNotPresent` | Image settings per service |
| `<svc>.resources` / `<svc>.extraEnv` | `{}` / `[]` | Standard per-service knobs |
| `runtime.model` | `anthropic/claude-sonnet-5` | `AGENTOS_MODEL` |
| `runtime.approvalTools` | `""` | `AGENTOS_APPROVAL_TOOLS` (HITL) |
| `restConnector.specUrl` | `""` | OpenAPI 3 JSON URL (required when enabled, unless demoCrm) |
| `restConnector.allowMutations` | `false` | GET-only unless true |
| `ingress.enabled` | `false` | Console Ingress (`host`, `className`, `tls`, `annotations`) |

## Verification

`deploy/helm/test-render.sh` runs `helm lint` plus `helm template` under
several value combinations and asserts the key invariants of the rendered
manifests. CI-friendly; requires only the `helm` binary.
