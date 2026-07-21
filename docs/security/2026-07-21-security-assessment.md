# AgentOS Security & Quality Assessment — 2026-07-21

Pre-Phase-8 audit of the shipped Phase 1–7 platform (48 commits, 8 services,
Go/Python/Rust/TS). Four independent read-only audits (gateway, connectors,
runtime, infra/efficiency) plus an OpenClaw security review. Findings are
deduplicated and ranked by real severity **including exploit chains**. The two
CRITICALs were re-verified by hand against source.

## Bottom line

The **gateway core is solid** — no criticals, strong crypto (`crypto/rand`,
SHA-256-hashed tokens, fully-verified OIDC JWTs, parameterized SQL, no routing
SSRF, RBAC cross-org mutations blocked, SCIM can't escalate). The **Rust sandbox
and its egress-less isolation are genuinely sound**. But three issues are
**must-fix before autonomy**, and they *chain*:

> **The scary chain:** the runtime API has **no authentication** and its port is
> published; the **SSH connector's allowlist is bypassable to arbitrary RCE**.
> Anyone who can reach the runtime port can drive an agent that calls the SSH
> connector → **unauthenticated remote code execution on connected legacy hosts.**
> Phase 8 (autonomous heartbeat + unauthenticated webhooks + unsigned SKILL.md
> skills) would *amplify* every one of these. **Harden first, then autonomy.**

## CRITICAL (verified by hand)

**C1 — Runtime API has zero authentication.** `runtime/src/agentos_runtime/api.py`
(+ improve/prompts/evals routers): every route's only `Depends(...)` are resource
getters — no bearer/API-key/middleware. `deploy/compose.yaml` publishes the port
(`:8000`→host 18000). Unauthenticated callers can drive `POST /runs`, **approve
HITL tool calls** (`/runs/{tid}/approve`), **approve prompt hot-swaps**
(`/proposals/{id}/approve`), ingest poisoned docs (`/documents`), and trigger
`/improve`. This converts every "human approval" control into "anyone approves."

**C2 — SSH connector allowlist → arbitrary RCE via argument injection.**
`connectors/ssh/internal/validate/validate.go:32-47` checks only (a) a metachar set
`;|&\`\n\r` + `$(`, and (b) the **first token's basename** against the allowlist.
Arguments are never inspected and the command runs through the remote login shell.
So allowlisted "LOLBins" break out with no blocked character:
`find / -maxdepth 0 -exec id {} +` (uses `+`, not the blocked `;`),
`awk 'BEGIN{system("id")}'`, `tar cf /dev/null --to-command=id x`,
`git -c core.pager=id log`. Also `>`/`<` are **not** blocked → arbitrary file
write (`find / > ~/.ssh/authorized_keys`). Basename allowlisting is not a security
boundary for shells.

**C3 — Approved prompt proposal hot-swaps the live system prompt with no
validation.** `prompts.py:68-72` applies arbitrary LLM-authored `prompt_text`
verbatim as the agent's system prompt, persisted across restarts. Nothing bounds
it; combined with C1 the "approval" is anyone. A proposal saying "ignore
read-only, auto-approve, exfiltrate results" becomes the permanent persona.

## HIGH

- **H1 — SSH defaults to no host-key verification** (`InsecureIgnoreHostKey`,
  ssh-connector `main.go`) → MITM captures the SSH password/session. Should fail
  closed when `known_hosts` is unset.
- **H2 — REST/SOAP leak the custom auth header on redirects → SSRF pivot.** Both
  use a default `http.Client` (follows 10 redirects, no `CheckRedirect`). Go only
  strips `Authorization`/`Cookie` on cross-host redirects — a custom header
  (`X-Api-Key`, the common case) is forwarded. A model-controlled path to an
  open-redirect on the upstream reaches `169.254.169.254`/internal hosts *with the
  secret* and returns the body to the model.
- **H3 — OIDC account takeover.** `oidc/oidc.go` never checks `email_verified`, and
  `server/oidc.go` `upsertSSOUser` matches an existing user **by email** and adopts
  their **stored role**. An IdP that emits an arbitrary/unverified email lets an
  attacker log in as `owner@victim` and inherit owner/admin. Match on stable `sub`;
  require `email_verified`.
- **H4 — Multi-tenant leakage & spend miscount via key *name*.** Usage/spend/audit
  are keyed on the user-chosen key **name**, not `org_id` (`store/postgres.go:148`
  `UPDATE keys SET spend_usd… WHERE name=$2`; `rbac.go:89-123` filters by name).
  Two orgs with a key named `runtime` cross-contaminate spend and expose each
  other's usage/audit. Scope by `org_id`/`secret_hash`.
- **H5 — Secret rotation is a silent no-op for provider keys.** Provider keys are
  read once into `provider.Router` at startup (`main.go:76-83`).
  `ReloadSecrets`/`StartSecretsRefresh` update only `s.secrets` (what
  `/admin/secrets/status` reports), **never the router**. A rotated/compromised
  Anthropic/OpenAI key keeps being used. Route provider reads through the live
  `secret.Source`.
- **H6 — Stored prompt injection via `/documents` → `search_knowledge`.**
  `context.py:113-135` returns retrieved chunks (attacker-controlled text *and*
  name) into the model context with no trust boundary; unauthenticated ingest (C1)
  makes this a remote hijack. Label retrieved content as untrusted, delimited data.
- **H7 — HITL off by default + tool output trusted.** `AGENTOS_APPROVAL_TOOLS`
  defaults empty → `run_python` and all MCP tools execute with no human gate, so an
  injected instruction (H6) can invoke code execution unattended.

## MEDIUM

- Budget caps bypassable under concurrency (TOCTOU: pre-flight read vs
  post-response write; no atomic check-and-debit). `server.go:408`.
- Unbounded request bodies (no `http.MaxBytesReader`) → memory-exhaustion DoS.
- Non-constant-time compare of the admin key and SCIM token (`==`/`!=`; use
  `crypto/subtle`). Virtual/user tokens are safe (hashed lookup).
- SQL connector has no `statement_timeout` → `pg_sleep`/heavy scans pin a
  connection (DoS). Add per-query timeout.
- SQL read-only tx still allows `pg_read_file`/`lo_export` **if the DB role is
  over-privileged** (defense-in-depth; `erp_reader` is correctly SELECT-only).
- Browser allowlist is name-only → DNS-rebinding / no private-IP backstop. Resolve
  and reject link-local/loopback/RFC-1918; pin the IP.
- SOAP `xml_body` escape hatch injects raw unescaped XML into the envelope.
- Runtime leaks internal errors incl. the checkpoint DSN via `HTTPException(detail=str(exc))`.
- Guessable/client-chosen `thread_id` + no ownership check → cross-run approve/deny
  hijack (with C1).
- OTel exports raw tool inputs (may contain `run_python` code / SQL args).
- Default credentials are the shipping defaults (`admin-local-dev`, pg `agentos`,
  helm `values.yaml`) — "override for prod" is a comment, not enforced.
- No `securityContext`; **runtime image runs as root** (no `USER` in
  `runtime/Dockerfile`), writable rootfs, full caps. Only the sandbox is hardened.

## LOW

No HTTP server timeouts (Slowloris) + no SIGTERM handling (deferred
`pg.Close()`/OTel flush never run on redeploy); host ports bind `0.0.0.0`
(DB + admin exposed to LAN); provider non-2xx collapsed to 502 (loses 429/401
semantics); rate limiter fails open on DB error (documented); negative `rate_limit_rpm`
accepted; OIDC state not bound to a browser cookie / no `nonce` (login-CSRF, limited
impact); inverted zero-budget semantics (key 0 = blocked, org 0 = unlimited).

## Efficiency (scale risks)

- **N+1** in `GET /admin/orgs` (`OrgSpend` per org) — fold into one `GROUP BY`.
- **Missing index** on `keys(org_id)` — every org-scoped path seq-scans.
- **~4 duplicate DB round-trips per proxied request** (two `Org()` lookups +
  budget + rpm) on the hot path — fetch org once, reuse/cache.
- **Non-streaming provider responses fully buffered** (`io.ReadAll`) — memory blowup
  on large bodies (SSE path is fine).
- Whole-table loads then filter-in-Go for org listings; SCIM `UserByID` is O(n);
  unbounded `audit_log` growth (no retention/index); per-op psycopg connections +
  per-embed `httpx.Client` in the runtime; in-memory rate-limit buckets never evict.

## Confirmed-correct controls (the platform got these right)

`crypto/rand` for all secrets; tokens stored SHA-256-hashed; OIDC ID token fully
verified (JWKS/iss/aud/exp); **no SQL injection** (fully parameterized);
**no SSRF via model routing** (provider URLs are fixed operator config); **SCIM
cannot escalate** org/role; **RBAC blocks cross-org mutations**; **SQL read-only
genuinely holds** (lexical validator + real `READ ONLY` tx + SELECT-only grant —
triple defense); **SOAP is not XXE-exploitable** (Go `encoding/xml` is safe by
default); **browser allowlist resists** userinfo@/suffix/`file:`/`data:` tricks;
**sandbox hardening is sound** (read-only, cap-drop ALL, no-new-priv, internal
egress-less network + Helm NetworkPolicy, uid 10001); HITL deny truly blocks the
tool; agent built once (no per-request rebuild); audits survive client disconnect;
CI is safe (`contents: read`, no `pull_request_target`, mocks never in prod).

## OpenClaw: security problems & secure operation

OpenClaw is a high-privilege, always-on agent (host shell/file/browser/Docker, a
WebSocket control plane on :18789, and the ClawHub skill marketplace). Its trust
model is the opposite of what this platform needs:

- **Snyk: ~36% of ClawHub skills contain prompt injection.** **341+ skills** caught
  stealing API keys / installing malware (The Hacker News, Feb 2026); **>1,184
  malicious skills**, ~**1 in 12 packages** malicious as the registry passed ~13,700.
- Skills run **unsandboxed** with the agent's full privileges.
- Risk classes: excessive privilege, supply-chain poisoning, prompt injection,
  autonomous misoperation, credential leakage, token-billing explosion.

**Operate OpenClaw safely *under AgentOS* ("secure shell").** OpenClaw's own
recommended defenses (sandbox isolation, credential protection, cost capping,
confirmation gating, supply-chain screening, privilege restriction) map 1:1 onto
AgentOS controls — but only **after** C1/C2/C3 are fixed, or you'd be exposing the
very RCE surface OpenClaw would call:

| OpenClaw risk | AgentOS control | Wiring |
|---|---|---|
| Unsandboxed exec | egress-less Rust sandbox / gated SSH | route exec through sandbox or an argv-safe SSH, never host shell |
| Cred leak + token blowup | governed gateway (keys, budgets, rate limits, audit) | point OpenClaw's OpenAI-compatible provider at `gateway/v1` with an org-scoped key |
| Poisoned skills (36% injected) | trusted-dir-only SKILL.md + injection screen + no auto-fetch | never install from ClawHub at runtime; scan + human-review each skill |
| Autonomous misoperation | max_cycles + kill-switch + HITL | gate high-risk tools; bound cycles; default-off |
| Excessive privilege / :18789 | non-root, read-only rootfs, isolated net | don't expose the control plane; egress allow-list |

**Literal "secure shell":** OpenClaw's shell tool must never touch the host — give
it the sandbox (code) or an argv-based, allowlisted SSH jail (commands), gated by
HITL confirmation.

## Recommendation — pivot Phase 8

**Do not build autonomous operations on this base.** An unauthenticated webhook
firing an autonomous agent that follows unsigned SKILL.md instructions and can hit
an RCE-vulnerable SSH connector is the worst-case chain — and Phase 8 as planned
builds exactly that. Instead:

**Phase 8 = Security Hardening (do first):**
1. **Runtime auth** on every mutating route (shared secret / gateway-injected
   identity / mTLS); never expose it unauthenticated. *(C1)*
2. **SSH connector**: drop basename allowlisting for a vetted, argv-based,
   no-exec-taking command policy; block `>`/`<`; require `known_hosts`
   (fail-closed). *(C2, H1)*
3. **Bound the prompt hot-swap**: authenticated human approval + an immutable
   safety preamble proposals can't override + real adversarial safety evals in the
   gate. *(C3, H6)*
4. **Redirect hardening** on REST/SOAP (`CheckRedirect` re-strips auth header, deny
   private IPs). *(H2)*
5. **OIDC**: require `email_verified`, match on `sub`. *(H3)*
6. **Tenant isolation**: scope usage/spend/audit by `org_id`/`secret_hash`, not
   name. *(H4)*
7. **Provider-key rotation**: read keys from the live `secret.Source`. *(H5)*
8. Mediums as capacity allows: atomic budget debit, `MaxBytesReader`,
   `crypto/subtle`, SQL `statement_timeout`, browser IP backstop, runtime `USER`
   + securityContext, gate default creds.

**Then Phase 8b = Governed Autonomy** on the hardened base, with security baked in:
constant-time high-entropy webhook tokens + rate limits + untrusted-body handling;
**SKILL.md from a trusted read-only dir only, injection-screened, no auto-fetch,
treated as advisory not authoritative**; HITL default-on for autonomous runs;
per-objective budgets. Position AgentOS as the **governed control plane for
OpenClaw fleets** rather than re-implementing ClawHub.

Sources (OpenClaw): The Hacker News (341 malicious skills), Snyk (36% injection),
Unit42/Palo Alto, HKCERT, Giskard, Nebius/InsiderLLM hardening guides.
