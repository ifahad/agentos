# AgentOS Phase 7 — SCIM Provisioning, Distributed Rate Limits, Secret Rotation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enterprise-scale identity and operations: SCIM 2.0 user provisioning (so an IdP can create/deactivate users automatically), a distributed rate-limit store so multi-replica gateways share buckets, and secret rotation/reload without a restart.

**Architecture:** Additive; Phases 1–6 contracts stay valid VERBATIM. Every feature is opt-in with a safe default reproducing current behavior. One gateway agent (owns all of `gateway/`), one console agent. Task 3 integrates.

**Tech Stack:** Go (gateway — SCIM via stdlib `net/http`+`encoding/json`, a Postgres-backed token bucket with atomic `UPDATE … RETURNING`, secret reload), React/TS (console).

## Global Constraints

- **Backward compatibility mandatory.** Every Phase 1–6 endpoint, env var, key, and test keeps working unchanged. SCIM/distributed-limiter/rotation are opt-in; unconfigured ⇒ identical to Phase 6.
- Existing tests stay green; new code tested to the same standard (Go table-driven, no network; Postgres-backed pieces skip without `AGENTOS_TEST_DATABASE_URL`). `go test -race` clean.
- Deferred beyond Phase 7 (documented): SAML (OIDC is the supported SSO), cloud-provider KMS backends (Vault backend covers the external-KMS story; a KMS adapter interface is noted), Redis limiter backend (Postgres backend ships; the interface admits Redis later).

## Frozen contracts

### Gateway — SCIM 2.0 user provisioning (additive, opt-in)

Enabled only when `AGENTOS_SCIM_TOKEN` is set. All `/scim/v2/*` routes require `Authorization: Bearer <AGENTOS_SCIM_TOKEN>` (else 401 SCIM error). Disabled (token unset) → every `/scim/v2/*` route 404. Env: `AGENTOS_SCIM_TOKEN`, `AGENTOS_SCIM_DEFAULT_ORG` (default `org_default`), `AGENTOS_SCIM_DEFAULT_ROLE` (default `member`).

Users store gains `active bool` (default true) and `external_id string` (SCIM id from the IdP, nullable). Deactivated users (`active=false`) fail `AuthenticateUser` (their `agu-` tokens stop working) but are retained (reactivatable). Migration-safe postgres `ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true`, `external_id TEXT`.

SCIM error shape: `{"schemas":["urn:ietf:params:scim:api:messages:2.0:Error"],"status":"<code>","detail":"…"}`. User resource shape (subset): `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"id":<user_id>,"externalId":?,"userName":<email>,"name":{"formatted":?},"emails":[{"value":<email>,"primary":true}],"active":<bool>,"meta":{"resourceType":"User","location":".../Users/<id>"}}`.

Endpoints:
- `POST /scim/v2/Users` — body has `userName` (email, required), optional `externalId`, `name.formatted`, `active`. Upsert by email in the default org (reuse existing user by email; else create with default role). Set active/external_id. → 201 with the User resource. Duplicate active user → 409 SCIM error.
- `GET /scim/v2/Users/{id}` → 200 User resource; unknown → 404 SCIM error.
- `GET /scim/v2/Users?filter=userName eq "x@y"` → SCIM ListResponse `{"schemas":["urn:ietf:params:scim:api:messages:2.0:ListResponse"],"totalResults":N,"Resources":[…],"startIndex":1,"itemsPerPage":N}`. Support the `userName eq "…"` filter and no-filter (list all in default org). Ignore paging params beyond returning the fields.
- `PATCH /scim/v2/Users/{id}` — RFC 7644 patch with operations setting `active` (the common deactivation op: `{"Operations":[{"op":"replace","path":"active","value":false}]}` and the pathless `{"op":"replace","value":{"active":false}}` form). Also accept `PUT /scim/v2/Users/{id}` (full replace of active/name/externalId). → 200 User resource.
- `DELETE /scim/v2/Users/{id}` → 204 (hard-delete the user via the existing DeleteUser).
- Discovery stubs (Bearer-protected): `GET /scim/v2/ServiceProviderConfig`, `GET /scim/v2/ResourceTypes`, `GET /scim/v2/Schemas` — minimal valid SCIM docs advertising the User resource and `filter` support (patch supported, no bulk, no sort). Enough for Okta/Entra to introspect.
- Store additions: `SetUserActive(ctx, id, active bool) error`, `UserByExternalID(ctx, orgID, extID) (*User, error)`; `CreateUser` variant or field-setter to persist `external_id`. `AuthenticateUser` must reject inactive users (`ErrUserInactive` or treat as not-found → the existing 401 path).

### Gateway — distributed rate-limit store (additive)

- `AGENTOS_RATELIMIT_BACKEND=memory|postgres` (default `memory` — the Phase 6 in-process limiter, unchanged behavior). `Limiter` becomes an interface `Allow(key string, rpm int) (allowed bool, retryAfter time.Duration)`; the existing in-memory bucket implements it.
- `postgres` backend: a token bucket persisted in table `rate_limit_buckets(org_id PK, tokens double precision, updated_at timestamptz)`, refilled lazily on each check inside one atomic statement so concurrent gateway replicas stay correct — a single `INSERT … ON CONFLICT … DO UPDATE SET tokens = LEAST(rpm, tokens + elapsed*rpm/60) - 1 … RETURNING tokens` pattern (compute refill from `now() - updated_at`); if the resulting tokens would be `< 0`, deny and return `retryAfter`. Uses the checkpoint/agentos DB via `AGENTOS_DATABASE_URL`. Requires the Postgres store; with the memory store + `postgres` backend → fatal at startup with a clear message.
- Tests: the in-memory limiter keeps its deterministic-clock tests. The postgres limiter is tested against a real DB (skip without `AGENTOS_TEST_DATABASE_URL`): burst then deny, refill over time (inject `now` via a SQL-parameter clock or a settable time source), and two concurrent goroutines sharing one bucket never exceed the limit.

### Gateway — secret rotation / reload (additive)

- `POST /admin/secrets/reload` (root only) → forces the active secret source to re-fetch: `file` re-reads, `vault` re-GETs, `age` re-decrypts, `env` is a no-op. Returns the fresh `/admin/secrets/status` array. Audits a `secret_reload` event (new audit kind). A `Reloadable` optional interface (`Reload() error`) on `secret.Source`; sources that don't implement it are no-ops.
- `AGENTOS_SECRETS_REFRESH_S` (default 0 = off): when > 0 and the source is Reloadable, a background goroutine reloads on that interval (logs on change count, never on unchanged). Provider routing reads the current secret value on each request (already does), so a rotated key takes effect without restart.
- Tests: file source Reload picks up a rewritten file immediately (not just on mtime poll); a fake Reloadable counts calls; the endpoint requires root (403 for a user token, 401 unauth) and audits.

### Console — provisioning & ops

- **Provisioning** page (root): SCIM enabled/disabled status; if enabled, a read-only table of users with an `active` badge and `externalId` (from `GET /admin/orgs/{org}/users` extended to include active/external_id — extend that existing endpoint's response additively). Explanatory copy that provisioning is IdP-driven (no create/delete here).
- **Secrets** page: add a "Reload secrets" button → `POST /api/gateway/admin/secrets/reload`, then refresh the status table; show the rate-limit backend and secrets refresh interval if surfaced.
- Existing Users page: show the `active` badge next to each user.
- vitest for any new pure logic; keep all existing tests green; build clean.

---

### Task 1: Gateway — SCIM 2.0 provisioning + distributed (Postgres) rate-limit store + secret reload/refresh (owns all of gateway/). Each sub-part TDD; Postgres pieces skip without AGENTOS_TEST_DATABASE_URL; all Phase 1–6 tests green, `-race` clean; unconfigured ⇒ identical to Phase 6. Extend the users listing response with active/external_id (additive). New audit kind `secret_reload`.
### Task 2: Console — Provisioning page, Reload-secrets button, active badges, backend/refresh display; vitest for new pure logic; keep 127 tests green; build clean.
### Task 3: Integration (orchestrator) — compose: SCIM/ratelimit-backend/secrets-refresh env passthrough on gateway (defaults preserve behavior), `scripts/smoke7.sh` (SCIM create→get→list-filter→deactivate(agu- token stops working)→delete; distributed limiter via AGENTOS_RATELIMIT_BACKEND=postgres with a concurrent burst 429; secret reload endpoint + audit; back-compat), README/roadmap/.env.example, Makefile `smoke7`, confirm CI workflows still parse.

## Self-review notes
- SAML, cloud-KMS, Redis limiter explicitly deferred (roadmap); Postgres limiter + Vault backend + OIDC cover the sovereign path.
- Distributed limiter shares the org bucket across replicas via atomic SQL — documented as the multi-instance correctness fix for the Phase 6 per-instance note.
- Deactivation (SCIM active=false) invalidates `agu-` tokens by failing AuthenticateUser — the security-relevant deprovisioning path; smoke7 proves a deactivated user's token stops working.
- All new env vars added to `.env.example` in Task 3; no collisions with Phases 1–6.
