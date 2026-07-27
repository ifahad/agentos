# Console

The console (`console/`) is the AgentOS operator surface: a single-page
TypeScript app for managing keys, orgs, users, secrets, and provisioning, and
for running, observing, and approving agent work. It has no privileges of its
own — every action it takes is a call to the gateway or the runtime, subject
to the same auth and role checks either API enforces for any other caller.

See [`docs/api.md`](api.md) for the endpoints referenced below and
[`docs/concepts.md`](concepts.md) for terminology (roles, virtual keys,
threads).

## Signing in

The console authenticates as one of two things:

- **The root admin key** — a superuser credential above every org. Entered in
  the Settings modal (sidebar), stored in the browser's `localStorage`, and
  sent as `Authorization: Bearer <key>` on `/admin/*` calls.
- **A user token** (`agu-…`) — scoped to one org and one role
  (`owner` / `admin` / `member` / `viewer`), issued from the Users page or via
  SSO.

When the gateway has `AGENTOS_OIDC_ISSUER` configured, a **"Sign in with
SSO"** button appears in Settings, redirecting to the identity provider and
returning with a user token issued for the signed-in identity.

Either way, the console never asks you to declare your own role or org. It
calls `GET /admin/whoami` with whatever token it holds, and the gateway's
answer — role, org, email — drives the UI: which pages are visible, whose
data loads, what actions are offered. There is nothing to configure by hand,
and nothing the console shows can grant you more than the gateway itself
would grant that token.

## Pages

| Page | Path | What it's for | Backed by | Visible to |
|---|---|---|---|---|
| Overview | `/` | At-a-glance usage, spend, and recent audit events for the org's keys | `GET /admin/usage`, `GET /admin/keys`, `GET /admin/audit` | every role |
| Keys | `/keys` | List virtual keys and their budgets; create a key (secret shown once, never again) | `GET /admin/keys`, `POST /admin/keys` | every role |
| Audit | `/audit` | Recent gateway events (chat, embeddings, guardrail flags/blocks) | `GET /admin/audit` | every role |
| Playground | `/playground` | Run an agent turn, watch tool calls stream in as a timeline, approve or deny paused tool calls | `POST /runs`, `POST /runs/stream`, `POST /runs/{thread_id}/approve` | every role |
| Documents | `/documents` | List and ingest documents into the context engine (embedded, chunked, stored for retrieval) | `GET /documents`, `POST /documents` | every role |
| Improve | `/improve` | Eval-gated self-improvement: run the eval suite, review a proposed system prompt, approve or deny it | `GET /prompts/active`, `GET /evals/runs`, `POST /evals/run`, `POST /improve`, `GET /proposals`, `POST /proposals/{proposal_id}/approve` | every role |
| Multiverse | `/multiverse` | Council objectives fanned out across member profiles; review member proposals and approve/deny writes they held back | `GET /council/members`, `GET /council/objectives`, `GET /council/objectives/{objective_id}`, `POST /council/objectives`, `POST /council/objectives/{objective_id}/cancel`, `POST /council/pause`, `POST /council/resume`, `GET /council/proposals`, `POST /council/proposals/{proposal_id}/approve` | every role |
| Operators | `/operators` | Standing autonomous operators: create, enable/disable, run on demand, inspect recent runs | `GET /operators`, `POST /operators`, `GET /operators/{operator_id}`, `PATCH /operators/{operator_id}`, `DELETE /operators/{operator_id}`, `POST /operators/{operator_id}/run` | every role |
| Orgs | `/orgs` | Create orgs, adjust per-org rate limits | `GET /admin/orgs`, `POST /admin/orgs`, `PATCH /admin/orgs/{org_id}` | `org.view` |
| Users | `/users` | Invite and remove users within an org | `GET /admin/orgs`, `GET /admin/orgs/{org_id}/users`, `POST /admin/orgs/{org_id}/users`, `DELETE /admin/orgs/{org_id}/users/{user_id}` | `user.view` |
| Secrets | `/secrets` | Show which provider/connector secrets are configured (status only, never values); trigger a reload | `GET /admin/secrets/status`, `POST /admin/secrets/reload` | `secret.view` |
| Provisioning | `/provisioning` | SCIM status for org/user sync from an identity provider | `GET /admin/orgs`, `GET /admin/orgs/{org_id}/users` | `provisioning.view` |

Ungated pages are visible to every role, including `viewer`. The four gated
pages each require the named permission, checked against the caller's role by
the console's own `can()` helper (`console/src/lib/rbac.ts`), which mirrors
the gateway's role table. Only `owner`/`admin`/root callers typically hold
these; the root admin key holds every permission.

The Improve page requires the runtime to have a checkpoint database
configured (`AGENTOS_CHECKPOINT_DATABASE_URL`) — without it, self-improvement,
council, and operator state don't persist, and the page shows a disabled
empty state instead of data.

## Roles and visibility

A page can be missing from the sidebar and the ⌘K command palette for one
reason: its route in `console/src/App.tsx` carries a `visible` predicate, and
that predicate returned `false` for your role. This governs four routes only
— Orgs, Users, Secrets, Provisioning — everything else renders for every
role.

This is a UI affordance, not the security boundary. Hiding a page only avoids
showing a caller a button that would answer `403`; it grants nothing and
revokes nothing. The gateway re-checks the same permission on every
`/admin/*` call regardless of what the console rendered, using the token's
real role. A modified or scripted client that calls a gated endpoint directly
gets exactly the same accept/reject decision a role-appropriate console
session would have gotten.

## How the console talks to the platform

The browser makes same-origin requests only, to two path prefixes:

- `/api/gateway/*` — proxied by nginx to the gateway service.
- `/api/runtime/*` — proxied by nginx to the runtime service.

There is no third-party traffic and no CDN dependency; everything the console
loads is either the SPA's own bundle or one of these two same-origin proxies.

The gateway calls carry whatever token the console holds (root admin key or
user token) exactly as entered — nginx passes them through unmodified. The
runtime is different: every runtime route requires its own bearer token
(`AGENTOS_RUNTIME_AUTH_TOKEN`), and the browser never holds it. nginx injects
`Authorization: Bearer ${AGENTOS_RUNTIME_AUTH_TOKEN}` on the `/api/runtime/`
location server-side, substituting the value from the container's own
environment at startup. The token exists only in the console container's
environment and in the proxied request nginx constructs — it never appears
in a response body, a script, or anything the browser's JavaScript can read.

Data on most pages is live, not static: each resource polls its endpoint on
its own cadence (a few seconds by default) through a shared registry that
pauses every poll while the browser tab is hidden and resumes on return. A
status glyph in the topbar reflects the aggregate connection state — synced,
reconnecting, or offline — so a stale screen never looks indistinguishable
from a live one.

Every route is also reachable through the **⌘K / Ctrl-K command palette**,
which lists the same routes the sidebar does, filtered by the same
`visible` predicates.
