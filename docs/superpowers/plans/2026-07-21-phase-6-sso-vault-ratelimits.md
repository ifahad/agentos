# AgentOS Phase 6 — SSO, Vault, Rate Limits, whoami

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enterprise identity and multi-tenant safety: OIDC single sign-on, a live HashiCorp Vault secrets backend, per-tenant rate limits, and a `whoami` endpoint that cleans up the console's token-scoped UX.

**Architecture:** Additive; Phases 1–5 contracts stay valid VERBATIM. Every feature is opt-in with a safe default that reproduces current behavior. Three tasks: one gateway agent (owns all of `gateway/` to avoid go.mod contention), one console agent, one CI/test-doubles agent. Task 4 integrates.

**Tech Stack:** Go (gateway — `coreos/go-oidc` + `golang.org/x/oauth2` for OIDC, stdlib token-bucket, Vault via its KV v2 HTTP API with `net/http`), React/TS (console), Python stdlib (mock OIDC + mock Vault for smoke).

## Global Constraints

- **Backward compatibility is mandatory.** Every Phase 1–5 endpoint, env var, key, and test keeps working unchanged. SSO/Vault/rate-limits are opt-in; with none configured the gateway behaves exactly as Phase 5.
- Existing tests stay green; new code tested to the same standard (Go table-driven, no network — OIDC/Vault tested against httptest mocks). `go test -race` clean.
- All model calls still flow through the gateway on virtual keys; rate limits and org budgets both apply.
- Deferred beyond Phase 6 (documented): SAML (OIDC only here), SCIM user provisioning, distributed rate-limit store (in-memory per instance for now), automatic secret rotation webhooks.

## Frozen contracts

### Gateway — OIDC SSO (additive, opt-in)

Enabled only when `AGENTOS_OIDC_ISSUER` is set. Env:
- `AGENTOS_OIDC_ISSUER` (issuer URL; discovery at `{issuer}/.well-known/openid-configuration`), `AGENTOS_OIDC_CLIENT_ID`, `AGENTOS_OIDC_CLIENT_SECRET`, `AGENTOS_OIDC_REDIRECT_URL` (this gateway's callback, e.g. `http://localhost:8080/auth/oidc/callback`), `AGENTOS_OIDC_POST_LOGIN_URL` (console URL to return to, e.g. `http://localhost:3000/`), `AGENTOS_OIDC_DEFAULT_ORG` (default `org_default`), `AGENTOS_OIDC_DEFAULT_ROLE` (default `member`).
- Misconfig at startup (issuer set but discovery fails, or client id/secret missing) → fatal with a clear message.

Endpoints:
- `GET /auth/oidc/status` → `{"enabled":bool,"issuer":string?}` (no auth; console uses it to show/hide the SSO button).
- `GET /auth/oidc/login` → 302 to the provider's authorization endpoint with `response_type=code`, `scope=openid email profile`, `client_id`, `redirect_uri`, and a signed `state` (HMAC over a nonce+expiry using the admin key as the secret; 10-min expiry). Disabled → 404 `{"error":{"type":"sso_disabled"}}`.
- `GET /auth/oidc/callback?code=&state=` → validate state (HMAC+expiry), exchange the code at the token endpoint, verify the ID token (signature via JWKS, issuer, audience, expiry), extract `email` (fallback `sub`). Upsert a user: if a user with that email exists in the default org, reuse it; else create one (default org + default role). Mint an `agu-…` user token for that user. Then **302 redirect** to `{AGENTOS_OIDC_POST_LOGIN_URL}#token=<agu-…>&email=<email>&role=<role>` so the console SPA reads the fragment. On any validation failure → 401 `{"error":{"type":"sso_failed","message":…}}`.
- Reuse the existing RBAC user store (a returning SSO user keeps their role/keys). SSO-created users are normal users.

### Gateway — whoami (additive)

- `GET /admin/whoami` (any authenticated caller): root admin key → `{"root":true}`; `agu-…` user token → `{"root":false,"user_id":…,"org_id":…,"email":…,"role":…}`. Unauthenticated/invalid → 401 `{"error":{"type":"invalid_key"}}`.

### Gateway — per-tenant rate limits (additive)

- Token-bucket per org, requests-per-minute. `Org` gains `rate_limit_rpm int` (0 = unlimited, the default → current behavior). Settable at creation (`POST /admin/orgs` accepts optional `rate_limit_rpm`) and via `PATCH /admin/orgs/{org_id}` `{"monthly_budget_usd"?,"rate_limit_rpm"?}` (root or org owner/admin). Global default `AGENTOS_RATE_LIMIT_RPM` (default 0) applies to orgs whose own limit is 0.
- Enforced on `/v1/chat/completions` and `/v1/embeddings` for virtual-key callers: consume one token from the caller key's org bucket; empty bucket → 429 `{"error":{"type":"rate_limited"}}` with a `Retry-After` header (seconds until a token refills). The root admin key is exempt. Buckets are in-memory keyed by org id (documented: per-instance; distributed store is future work). The check runs after auth/guardrails and before the budget check; audit the rejection with a new kind `rate_limited`.
- Store: add `rate_limit_rpm` to Org (memory + postgres migration-safe `ADD COLUMN IF NOT EXISTS ... DEFAULT 0`); `UpdateOrg(ctx, id, budget *float64, rpm *int)` (nil = unchanged).

### Gateway — Vault secrets backend (additive)

- New `AGENTOS_SECRETS_BACKEND=vault`. Env: `AGENTOS_VAULT_ADDR` (e.g. `http://vault:8200`), `AGENTOS_VAULT_TOKEN`, `AGENTOS_VAULT_KV_PATH` (KV v2 data path, e.g. `secret/data/agentos`). At startup, GET `{addr}/v1/{kv_path}` with `X-Vault-Token`, parse the KV v2 shape `{"data":{"data":{NAME:VALUE,…}}}` into the secret map. `Get(name)` serves from that map (non-empty = present). Misconfig (unreachable, 403, missing path) → fatal with a clear message. Add a `vault` case to `secret.FromEnv`. Tested against an httptest mock returning the KV v2 JSON (success, 403, malformed).

### Console — SSO, whoami identity, rate limits

- On load, if a token is set, call `GET /api/gateway/admin/whoami` → drive the UI from the real identity (role from whoami, not manual entry). Remove the manual org-id field from Settings for user-token callers (root still selects org from the orgs list); the Users page uses `whoami.org_id`.
- SSO: call `GET /api/gateway/auth/oidc/status`; if enabled, show a "Sign in with SSO" button → `window.location = "/api/gateway/auth/oidc/login"`. On return, read `#token=…&email=…&role=…` from the URL fragment, store the token, clear the fragment, call whoami. Keep the manual-token and admin-key paths.
- Orgs page: show and edit `rate_limit_rpm` (PATCH). Rate-limit 429s from any call → a friendly "rate limited, retry shortly" inline notice (read `Retry-After`).
- vitest for new pure logic (fragment parser, whoami→capability wiring, retry-after formatting). Keep existing tests green.

### CI / test doubles

- `deploy/ci/mock-oidc.py` — stdlib-only minimal OIDC provider: `/.well-known/openid-configuration`, `/authorize` (auto-approves, 302 back to redirect_uri with a code), `/token` (returns an ID token — an RS256 JWT signed with a generated key), `/jwks` (the public key). Deterministic; a fixed test email. Used by smoke6 to prove the full login flow against the real gateway.
- `deploy/ci/mock-vault.py` — stdlib-only KV v2 endpoint returning a canned `{"data":{"data":{…}}}` for the configured token/path; 403 on wrong token. Used by smoke6 to prove the vault backend.
- `.github/workflows/ci.yml` unchanged in structure (already covers gateway/console); confirm it still parses. No new required jobs.

---

### Task 1: Gateway — OIDC SSO + whoami + per-tenant rate limits + Vault backend (owns all of gateway/). Sub-parts, each TDD with httptest mocks; all Phase 1–5 tests green, `-race` clean; back-compat: no OIDC/Vault/RPM configured ⇒ identical to Phase 5.
### Task 2: Console — whoami-driven identity, SSO login flow, rate-limit display/edit; vitest for pure logic; build green; keep 107 tests green.
### Task 3: CI/test-doubles — mock-oidc.py + mock-vault.py (stdlib, self-tested), confirm workflows parse.
### Task 4: Integration (orchestrator) — compose: OIDC/Vault/RPM env passthrough on gateway (defaults preserve current behavior), `scripts/smoke6.sh` (whoami root+user; rate-limit 429 via a tiny per-org rpm; SSO full flow against mock-oidc; vault backend via mock-vault; back-compat unaffected), README/roadmap/.env.example, Makefile `smoke6`.

## Self-review notes
- SAML, SCIM, distributed rate-limit store, rotation webhooks explicitly deferred (roadmap).
- Rate limit runs before the budget check; both can reject — documented order.
- SSO mints normal `agu-` tokens through the existing RBAC store — no parallel identity system.
- All new env vars added to `.env.example` in Task 4; no collisions with Phases 1–5.
- The state HMAC and ID-token verification are real (JWKS/RS256); the mock provider exercises the real verification path, not a bypass.
