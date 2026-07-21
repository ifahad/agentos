# AgentOS Phase 5 — RBAC, Secrets, SOAP/Browser Connectors, CI

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enterprise governance depth (multi-tenant RBAC, secrets-manager integration), two more legacy reaches (SOAP, browser/computer-use), and continuous verification (CI with an LLM-judge eval gate).

**Architecture:** Additive; Phases 1–4 contracts stay valid VERBATIM. RBAC is layered under the existing auth — the root admin key stays a superuser and existing `agos-` virtual keys keep working (they belong to a bootstrapped `default` org). Six parallel tasks against frozen contracts; task 7 integrates.

**Tech Stack:** Go (gateway RBAC, secrets abstraction, SOAP connector via `encoding/xml`), Python + Playwright (browser connector), GitHub Actions (CI), React/TS (console RBAC pages).

## Global Constraints

- **Backward compatibility is mandatory.** Every Phase 1–4 endpoint, env var, and virtual key keeps working unchanged. RBAC/secrets are opt-in layers with safe defaults (single-tenant `default` org, env-based secrets) that reproduce current behavior exactly.
- Existing tests stay green; new code tested to the same standard (Go table-driven, pytest no-network, Rust clippy -D warnings, console vitest logic-only).
- All model calls still flow through the gateway on virtual keys.
- Deferred beyond Phase 5 (documented): SAML/OIDC SSO, HashiCorp Vault live backend (we ship the abstraction + file/age backends), mobile console.

## Frozen contracts

### Gateway — Multi-tenant RBAC (additive)

New concepts, all in the existing store (memory + Postgres, migration-safe):
- **Org**: `{id, name, monthly_budget_usd, created_at}`. A bootstrapped `default` org (id `org_default`) owns every pre-existing key. Org budget caps the sum of its keys' spend (checked in addition to per-key budget).
- **User**: `{id, org_id, email, role, created_at}` where role ∈ `owner|admin|member|viewer`. Authenticated by a **user token** `agu-<random>` (distinct prefix from `agos-` service keys).
- **Virtual key** gains `org_id` (defaults to `org_default`) and `created_by` (user id or "root").
- **Role capabilities**: `owner` (manage org, users, keys, budgets, view all), `admin` (manage users below owner, keys, view), `member` (create/list own keys, run agents, view own usage), `viewer` (read-only usage/audit). The root admin key (`AGENTOS_ADMIN_KEY`) is a global superuser above all orgs.

New admin/RBAC API (root-admin OR org-scoped by capability):
- `POST /admin/orgs` (root) `{name, monthly_budget_usd}` → org.
- `GET /admin/orgs` (root) → list with aggregate spend.
- `POST /admin/orgs/{org_id}/users` (root or org owner/admin) `{email, role}` → `{id, email, role, token}` (token shown once, `agu-…`).
- `GET /admin/orgs/{org_id}/users` (root or org owner/admin/member) → list (no tokens).
- `DELETE /admin/orgs/{org_id}/users/{user_id}` (root or org owner/admin).
- Existing `POST /admin/keys` gains optional `{org_id, role_scope}`; when called with a **user token** instead of the root admin key, the key is created in that user's org (capability-checked: member+). `GET /admin/keys` and `/admin/usage` scope to the caller's org when a user token is used; root admin sees all.
- **Auth resolution** for `/admin/*`: `Authorization: Bearer <token>` where token is the root admin key (superuser), or an `agu-…` user token (role-checked), else 401. Capability failures → 403 `{"error":{"type":"forbidden","message":…}}`.
- Org budget exceeded on a chat/embeddings call → 402 `{"error":{"type":"org_budget_exceeded"}}` (per-key 402 unchanged).
- Env: `AGENTOS_BOOTSTRAP_ORG` (default `default`), and bootstrap keys land in `org_default`. Backward-compat: with no orgs/users created, everything behaves as Phase 1–4.
- Store interface additions: `CreateOrg`, `Orgs`, `OrgSpend`, `CreateUser`, `AuthenticateUser`, `Users`, `DeleteUser`; `CreateKey` gains org/creator params (keep a back-compat wrapper for existing callers/tests).

### Gateway + shared — Secrets backend (additive)

- Provider keys (and future secrets) resolved through a `SecretSource` interface: `Get(name string) (value string, ok bool)`. Backends selected by `AGENTOS_SECRETS_BACKEND=env|file|age` (default `env` — current behavior exactly).
  - `env`: `os.Getenv` (unchanged).
  - `file`: JSON/dotenv file at `AGENTOS_SECRETS_FILE` (e.g. `{"AGENTOS_ANTHROPIC_API_KEY":"sk-…"}`); reloaded on each Get if the file mtime changed (hot rotation).
  - `age`: an age-encrypted file (`AGENTOS_SECRETS_FILE` + identity `AGENTOS_SECRETS_AGE_KEY`), decrypted in-memory at startup using `filippo.io/age`. No plaintext secret ever on disk.
- Only provider keys route through it in Phase 5 (`AGENTOS_ANTHROPIC_API_KEY`, `AGENTOS_OPENAI_API_KEY`); the abstraction is general. `GET /admin/secrets/status` (root) → `[{name, present:bool, source:"env|file|age"}]` (never values).
- Startup logs the active backend; misconfig (file missing, bad age key) → fatal with a clear message.

### connectors/soap/ (Go) — MCP :8093/mcp

Legacy SOAP services as MCP tools. Env: `AGENTOS_SOAP_WSDL_URL` (http(s) or file, required), `AGENTOS_SOAP_ENDPOINT` (override the WSDL's soap:address), `AGENTOS_SOAP_ALLOW_OPERATIONS` (comma allowlist; empty = all read-ish operations exposed), `AGENTOS_SOAP_AUTH_HEADER` (raw header on upstream calls), `AGENTOS_SOAP_TIMEOUT_S` (default 20), `AGENTOS_SOAP_MAX_BODY_BYTES` (default 131072).
Parse WSDL with `encoding/xml` (portType operations, binding SOAPAction, message/part → input element names; SOAP 1.1 first, note 1.2). Server `agentos-soap`, StreamableHTTP `/mcp` :8093. Tools: `list_operations()` → `[{name, soap_action, doc}]`; one tool per allowed operation with **string args** for each top-level input part (flat; nested types passed as raw XML string via an `xml_body` arg escape hatch when the input isn't simple). Tool builds the SOAP envelope, POSTs with the SOAPAction header, returns `{"status":N, "body":<parsed-or-raw, truncated>}`. Tests: WSDL parse (a bundled sample WSDL — e.g. a calculator/temperature service fixture), envelope construction, operation allowlist, auth header, against an httptest SOAP echo server.

### connectors/browser/ (Python) — MCP :8094

Browser/computer-use as MCP tools (Playwright, chromium headless). Env: `AGENTOS_BROWSER_ALLOW_DOMAINS` (comma allowlist of hostnames; empty = deny all navigation, log warning), `AGENTOS_BROWSER_TIMEOUT_S` (default 20), `AGENTOS_BROWSER_MAX_TEXT` (default 8000). MCP (streamable HTTP, mirror the Python runtime's server style using `mcp`/FastMCP or the same stack the runtime uses) `/mcp` :8094. Tools (all domain-allowlist enforced, one shared headless context, hardened: no downloads, JS enabled, block requests to non-allowlisted hosts):
- `navigate(url)` → `{final_url, title, status}`.
- `get_text()` → visible text of the current page (capped).
- `find_links(query?)` → `[{text, href}]` filtered by optional substring.
- `click(text)` → clicks the first visible element whose text matches; returns new `{final_url, title}`.
Tests: domain-allowlist validation (pure), and a Playwright integration test against a local `file://` or a tiny in-process HTTP fixture, `@pytest.mark.skipif` when chromium isn't installed. Dockerfile uses the official Playwright python image.

### CI — GitHub Actions (`.github/workflows/`)

- `ci.yml`: matrix jobs — Go (gateway + all connectors: `go vet` + `go test`), Python runtime (`ruff` + `pytest`), Rust sandbox (`cargo fmt --check` + `clippy -D warnings` + `test`), console (`npm ci` + `npm test` + `npm run build`), Helm (`helm lint` + `test-render.sh`). Triggers on push + PR. Go version 1.25, Python 3.12, Node 22, Rust stable.
- `evals.yml`: an **LLM-judge eval gate** that runs the runtime eval suite against a spun-up compose stack using a **mock model** (a tiny stub OpenAI-compatible server, `deploy/ci/mock-model.py`, returning canned tool-calls/answers so evals are deterministic and offline — NO real provider needed). Manual + PR-label triggered (`run-evals`). Publishes the eval score as a job summary; fails if score < threshold (0.8). Documented that with real keys it runs against real models.
- A status badge line added to README.

### Console — RBAC pages

- **Orgs** (root admin only): list orgs + aggregate spend; create org.
- **Users** (per selected org): list users/roles; invite user (email+role) → show `agu-…` token once; remove user.
- **Secrets** (root): the `/admin/secrets/status` table (name, present, source) — never values.
- Settings modal gains a mode: authenticate as root admin key OR as a user token; the UI hides actions the role can't perform (capability map mirrored from the contract). Existing pages scope to the caller automatically via the API.
- vitest for the pure capability map + any new formatting.

---

### Task 1: Gateway RBAC — orgs/users/roles/scoped-keys, additive store + API, capability checks; back-compat wrapper for CreateKey; all Phase 1–4 gateway tests green
### Task 2: Secrets backend — SecretSource (env|file|age), provider-key resolution through it, /admin/secrets/status, fatal-on-misconfig; env default reproduces current behavior; tests for all three backends (age with a generated test identity)
### Task 3: SOAP connector — connectors/soap/, WSDL parse + envelope + allowlist, sample WSDL fixture, httptest SOAP echo tests
### Task 4: Browser connector — connectors/browser/ (Python + Playwright), domain-allowlist, 4 tools, skipif-chromium integration test, Playwright Dockerfile
### Task 5: CI — .github/workflows/ci.yml (test matrix) + evals.yml (mock-model eval gate) + deploy/ci/mock-model.py; README badges
### Task 6: Console — Orgs/Users/Secrets pages + role-aware settings + capability map + vitest; build green
### Task 7: Integration (orchestrator) — compose: soap-connector (opt-in) + browser-connector (opt-in) services + secrets/RBAC env, `scripts/smoke5.sh` (org+user create → scoped key → org budget 402; secrets status; SOAP op via echo fixture; browser navigate against a local fixture if chromium present; CI workflows lint via `act`/yamllint or a parse check), README/roadmap/.env.example, Makefile `smoke5`, run the mock-model eval gate locally once.

## Self-review notes
- RBAC additive: `CreateKey` keeps a back-compat signature wrapper; `org_default` bootstrapped so existing keys/tests are untouched.
- Browser + SOAP connectors are opt-in (not in the default compose up path); smoke5 treats them like SSH (unit-covered; live check when the dependency is present).
- Secrets `env` default = current behavior byte-for-byte; age/file are opt-in.
- SAML/OIDC SSO and a live Vault backend explicitly deferred (roadmap).
- All new env vars added to `.env.example` in Task 7; no collisions with Phases 1–4.
