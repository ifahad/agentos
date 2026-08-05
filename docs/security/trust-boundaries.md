# Trust boundaries

Living document. Where the platform expects a caller to prove something, and
where it deliberately does not.

## The console is an administrative surface

**Anything that can open a TCP connection to the console port has full runtime
authority.** This is by design, and it is the single most important sentence on
this page.

The console serves the SPA and proxies two API paths. They are not equally
protected, and the difference is easy to miss:

| Path | Who authenticates | What a bare `curl` gets |
|---|---|---|
| `/api/gateway/` | the gateway, per request | `401` |
| `/api/runtime/` | **nobody** — nginx injects the bearer | `200`, and an agent runs |

```console
$ curl -X POST http://<console>/api/runtime/runs \
    -H 'Content-Type: application/json' -d '{"input":"..."}'
{"thread_id":"...","output":"...","status":"completed"}
```

That reaches agent runs, `run_python` in the sandbox, every connected MCP tool,
operator creation, proposal approval, and spend against the runtime key.

### Why it is built this way

The runtime demands a bearer on every route (`AGENTOS_RUNTIME_AUTH_TOKEN`). If
the browser held that token, every console user would hold full runtime
authority in a place scripts can read it. Instead nginx injects it server-side,
so the token never leaves the cluster — and the cost is that the proxy has no
caller to authenticate. The console's sign-in is client-side: it decides what
the SPA renders, not what the proxy forwards.

The trade is deliberate. **We keep it and defend the port**, rather than
authenticate the proxy — that is the "trusted surface" model.

### What that obliges you to do

- **Do not publish the console port to an untrusted network.** Reaching it is
  equivalent to holding the runtime token.
- **Put authentication in front of any ingress**: an SSO annotation on the
  ingress controller, mTLS, or an IP allowlist. The Helm chart refuses to render
  an Ingress until you assert this (`ingress.frontedByAuth`).
- **Bind deliberately in compose.** `AGENTOS_CONSOLE_BIND` defaults to
  `0.0.0.0`, which publishes the console on every interface of the host. Set it
  to `127.0.0.1` and reach the console over an SSH tunnel unless the host's
  network is itself the trust boundary.
- **Rotate `AGENTOS_RUNTIME_AUTH_TOKEN` as a credential**, because it is one.
  The Helm chart has no default for it, and both the runtime and console pods
  carry a checksum annotation so rotation actually rolls them.

## Surfaces that do authenticate

- **Gateway** (`:8080`) — every request carries an `agos-` virtual key or the
  admin key; RBAC, budgets, and rate limits are enforced per request, and every
  call is audited.
- **Runtime** (`:8000`) — bearer on every route except `GET /healthz` and
  `/operators/webhooks/{token}`, where the opaque `whk-` token in the path *is*
  the credential and an unknown one 404s.
- **Sandbox** (`:8070`) — no host port at all, and on an internal-only network
  reachable solely by the runtime.

## Known gaps

Recorded rather than implied:

- **The budget-reservation fail-open writes no audit row.** On a store error the
  gateway admits the request (`internal/server/rbac.go`), unlike the guardrail
  fail-open, which has a dedicated `guardrail_error` audit kind precisely so the
  blind spot is recorded.
- **RBAC denials are not audited.** There is no audit kind for a 403.
- **The sandbox NetworkPolicy depends on the CNI.** If yours does not enforce
  NetworkPolicy, the object renders and does nothing, while the manifest still
  claims the sandbox is egress-less.
