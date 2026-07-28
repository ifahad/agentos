# API and Tool Reference

This page is the endpoint and tool reference for AgentOS. It answers the
question "what can an agent actually do": every HTTP route exposed by the
gateway, the runtime, and the sandbox, plus the full catalog of tools an agent
can call through the connectors and the runtime itself.

See [`docs/concepts.md`](concepts.md) for terminology and
[`docs/architecture.md`](architecture.md) for how these services fit
together.

## Gateway API

The gateway is the OpenAI-compatible edge: it fronts model traffic, admin
operations, SSO, and SCIM provisioning.

Three distinct auth mechanisms guard different parts of this surface — they
are not one RBAC-gated group:

- **`adminAuth`** guards `/admin/*` only. It accepts the root admin key
  (superuser) or an `agu-` user token; a user token is then role-checked
  per-action against the caller's role (`owner` / `admin` / `member` /
  `viewer`). `POST /admin/secrets/reload` is **root-only** — a role-checked
  `agu-` token, however privileged its role, is rejected.
- **`scimAuth`** guards `/scim/v2/*` only. It is a static shared-secret bearer
  compare against `AGENTOS_SCIM_TOKEN`, with no role evaluation at all.
  These routes are **not registered** when `AGENTOS_SCIM_TOKEN` is unset, so
  they return `404`, not `401`, on a gateway that hasn't enabled SCIM.
- **`/auth/oidc/*`** has no auth wrapper. It is the public login/callback
  flow a browser follows to establish a session.

| Method | Path | Purpose | Auth |
|---|---|---|---|
| `GET` | `/healthz` | Liveness probe | none |
| `POST` | `/v1/chat/completions` | OpenAI-compatible chat completion, routed to the configured provider | `agos-` virtual key |
| `POST` | `/v1/embeddings` | OpenAI-compatible embeddings | `agos-` virtual key |
| `POST` | `/admin/keys` | Create a virtual key | `adminAuth` (`agu-` token or root admin key, role-checked) |
| `GET` | `/admin/keys` | List virtual keys | `adminAuth` (`agu-` token or root admin key, role-checked) |
| `GET` | `/admin/usage` | Usage/spend report | `adminAuth` (`agu-` token or root admin key, role-checked) |
| `GET` | `/admin/audit` | Audit log | `adminAuth` (`agu-` token or root admin key, role-checked) |
| `GET` | `/admin/whoami` | Resolve the caller's identity and role | `adminAuth` (`agu-` token or root admin key, role-checked) |
| `POST` | `/admin/orgs` | Create an org | `adminAuth` (`agu-` token or root admin key, role-checked) |
| `GET` | `/admin/orgs` | List orgs | `adminAuth` (`agu-` token or root admin key, role-checked) |
| `PATCH` | `/admin/orgs/{org_id}` | Update an org | `adminAuth` (`agu-` token or root admin key, role-checked) |
| `POST` | `/admin/orgs/{org_id}/users` | Create a user in an org | `adminAuth` (`agu-` token or root admin key, role-checked) |
| `GET` | `/admin/orgs/{org_id}/users` | List users in an org | `adminAuth` (`agu-` token or root admin key, role-checked) |
| `DELETE` | `/admin/orgs/{org_id}/users/{user_id}` | Remove a user from an org | `adminAuth` (`agu-` token or root admin key, role-checked) |
| `GET` | `/admin/secrets/status` | Secrets-backend status | `adminAuth` (`agu-` token or root admin key, role-checked) |
| `GET` | `/admin/providers` | List configured model providers | `adminAuth` (`agu-` token or root admin key, role-checked) |
| `POST` | `/admin/secrets/reload` | Force a secret reload | `adminAuth` (**root only**) |
| `GET` | `/auth/oidc/status` | Whether OIDC SSO is configured | none (public) |
| `GET` | `/auth/oidc/login` | Begin the OIDC login redirect | none (public) |
| `GET` | `/auth/oidc/callback` | OIDC callback that completes login | none (public) |
| `POST` | `/scim/v2/Users` | Create a SCIM user | `scimAuth` (`AGENTOS_SCIM_TOKEN`) |
| `GET` | `/scim/v2/Users` | List SCIM users | `scimAuth` (`AGENTOS_SCIM_TOKEN`) |
| `GET` | `/scim/v2/Users/{id}` | Get a SCIM user | `scimAuth` (`AGENTOS_SCIM_TOKEN`) |
| `PATCH` | `/scim/v2/Users/{id}` | Patch a SCIM user | `scimAuth` (`AGENTOS_SCIM_TOKEN`) |
| `PUT` | `/scim/v2/Users/{id}` | Replace a SCIM user | `scimAuth` (`AGENTOS_SCIM_TOKEN`) |
| `DELETE` | `/scim/v2/Users/{id}` | Delete a SCIM user | `scimAuth` (`AGENTOS_SCIM_TOKEN`) |
| `GET` | `/scim/v2/ServiceProviderConfig` | SCIM service-provider config | `scimAuth` (`AGENTOS_SCIM_TOKEN`) |
| `GET` | `/scim/v2/ResourceTypes` | SCIM resource types | `scimAuth` (`AGENTOS_SCIM_TOKEN`) |
| `GET` | `/scim/v2/Schemas` | SCIM schemas | `scimAuth` (`AGENTOS_SCIM_TOKEN`) |

**SCIM routes 404 when `AGENTOS_SCIM_TOKEN` is unset** — they are not
registered on the mux at all, so a gateway without SCIM configured reports
"not found," not "unauthorized," for the whole `/scim/v2/*` tree.

## Runtime API

Every runtime route requires the `AGENTOS_RUNTIME_AUTH_TOKEN` bearer, with two
deliberate exceptions — `GET /healthz`, so orchestrator probes need no
credential, and the operator webhook below
(`runtime/src/agentos_runtime/api.py`, `OPEN_PATHS` and `OPEN_PREFIXES`):

**`POST /operators/webhooks/{token}` is exempt from the bearer.** The opaque
`whk-` token in the path *is* the credential — an external system firing a
webhook does not hold the runtime's bearer token, so it authenticates by
knowing this per-operator secret instead. An unknown or wrong token returns
`404`, not `401`, so the endpoint does not confirm the existence of a
non-matching operator.

| Method | Path | Purpose | Auth |
|---|---|---|---|
| `GET` | `/healthz` | Liveness probe | none (only open route besides the webhook) |
| `POST` | `/runs` | Run the agent to completion (or to a pending approval) on a thread | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `POST` | `/runs/stream` | Same as `/runs`, streamed as Server-Sent Events | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `POST` | `/runs/{thread_id}/approve` | Approve or deny a pending tool call and resume the run | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `GET` | `/threads/{thread_id}` | Fetch a thread's message history | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `GET` | `/documents` | List ingested context-engine documents | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `POST` | `/documents` | Ingest a document into the context engine | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `POST` | `/evals/run` | Run the eval suite against a prompt candidate | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `GET` | `/evals/runs` | List past eval runs | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `POST` | `/improve` | Propose a self-improvement prompt candidate | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `GET` | `/proposals` | List self-improvement prompt proposals | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `POST` | `/proposals/{proposal_id}/approve` | Approve or deny a prompt proposal (hot-swaps the live agent on approval) | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `GET` | `/prompts/active` | The currently active system prompt and its source | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `POST` | `/council/objectives` | Queue an objective for the council | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `GET` | `/council/objectives` | List council objectives | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `GET` | `/council/objectives/{objective_id}` | An objective plus its cycles | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `POST` | `/council/objectives/{objective_id}/cancel` | Request cancellation of a queued objective | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `POST` | `/council/objectives/{objective_id}/run` | Run a queued objective synchronously, one cycle by default | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `POST` | `/council/objectives/run` | Create and run an objective to a verdict synchronously | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `POST` | `/council/pause` | Pause the council | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `POST` | `/council/resume` | Resume the council | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `GET` | `/council/members` | List configured council members (never returns a credential) | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `GET` | `/council/proposals` | List council proposals for held write-class tool calls | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `POST` | `/council/proposals/{proposal_id}/approve` | Approve a held write-class council action (records the decision; does not execute the tool) | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `GET` | `/operators` | List operators | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `POST` | `/operators` | Create an operator | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `GET` | `/operators/skills` | List loaded SKILL.md skills (name, description, provenance sha256) | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `GET` | `/operators/{operator_id}` | Get an operator plus its recent runs | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `PATCH` | `/operators/{operator_id}` | Update an operator | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `DELETE` | `/operators/{operator_id}` | Delete an operator | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `POST` | `/operators/{operator_id}/run` | Fire an operator manually | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `GET` | `/operators/{operator_id}/runs` | List an operator's recent runs | `AGENTOS_RUNTIME_AUTH_TOKEN` bearer |
| `POST` | `/operators/webhooks/{token}` | Fire an operator via its webhook trigger | **exempt — the `whk-` token in the path is the credential; unknown token 404s** |

Two notes on routes corrected against this file's own reference material
during verification, both confirmed by reading the router source directly:

- `GET /operators/skills` is a real route (declared before `/{operator_id}`
  so the static path wins over the parameter) that had been missed in the
  prior inventory this page was drafted from.
- `GET /council/proposals` and `POST /council/proposals/{proposal_id}/approve`
  are real routes under the `/council` router prefix, distinct from the
  top-level `/proposals` and `/proposals/{proposal_id}/approve` pair.
  The top-level pair (in the table above) is the self-improvement prompt
  proposal flow; the `/council/*` pair is approval of held write-class tool
  calls raised during a council cycle. They are different resources that
  happen to share a name.

## Sandbox API

The sandbox executes untrusted code with per-run isolation. This is a
**frozen contract** — the request and response shapes do not change without
a version bump elsewhere in this documentation set.

```
POST /execute
  request:  {"language", "code", "timeout_s", "stdin"}
  response: {"exit_code", "stdout", "stderr", "duration_ms", "timed_out", "truncated"}

GET /healthz
  response: ok
```

`language` accepts only `"python"`.

The sandbox has **no published host port** under Compose. It is not reachable
from outside the Compose network — the runtime reaches it over the internal
network only, via `AGENTOS_SANDBOX_URL`.

## Tool catalog

This is what an agent can actually call, beyond the model itself. A tool is
reachable by the agent only if its server's URL is present in
`AGENTOS_MCP_SERVERS` — a connector that is running but not listed there is
invisible to the agent. See the connector-tiers section of
[`docs/deployment.md`](deployment.md#connector-tiers) for which connectors a
given deployment tier enables by default.

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
| `use_skill` | runtime | 8000 | skill name | Loads only from the image-baked `runtime/skills/` (`AGENTOS_SKILLS_DIR`); never fetched at runtime; each load records a sha256 for provenance |

The `use_skill` sha256 is recorded and logged for provenance — it is not
compared against a pinned expected value, so it does not by itself prevent a
changed skill file from loading.
