# Changelog

All notable delivery milestones. Each phase was smoke-tested live end-to-end
before it was considered done; the verification targets are documented in
[`docs/operations.md`](docs/operations.md).

## SCIM Groups — IdP-driven role assignment

Six `/scim/v2/Groups` routes speaking both Entra's and Okta's PATCH dialects,
with groups stored as first-class org-scoped objects. `AGENTOS_SCIM_GROUP_ROLES`
maps group displayNames to roles; unset (the default) means groups grant
nothing and SCIM still cannot change any role. A user in several mapped groups
takes the strongest; leaving their last mapped group demotes them back to the
default, because an identity provider that can grant but never revoke is worse
than one that does neither. Only users carrying a SCIM `externalId` are
reconciled, so a hand-created owner is never demoted by group membership.

## Operators — governed single-agent autonomy

Interval / cron / webhook triggers toward stored objectives, bounded and
audited, with in-repo `SKILL.md` skills the agent pulls on demand.
Verified by `make smoke8`.

## Multiverse — a council of models

Config-driven provider registry, upstream retry/fallback, a council of
model-bound agents with judge synthesis and dissent reporting, a governed
autonomous loop, and `council/multiverse` as an OpenAI-compatible model.
Verified by `make smoke9`.

## Hardening & efficiency

Atomic budget reservation (closing a TOCTOU race), request-body caps, server
timeouts and graceful drain, non-root images with Kubernetes `securityContext`,
SQL `statement_timeout`, opt-in audit retention, runtime context trimming.

## Scale & provisioning

SCIM 2.0 user provisioning, distributed (Postgres) rate-limit store, secret
rotation and reload. Verified by `make smoke7`.

## Enterprise identity

OIDC SSO, Vault secrets backend, per-tenant rate limits, the `whoami` endpoint.
Verified by `make smoke6`.

## Enterprise reach & governance

Multi-tenant RBAC, secrets backends (env / file / age), SOAP and browser
connectors, CI with an LLM-judge eval gate. Verified by `make smoke5`.

## Reach & hardening

SSH connector, egress-less sandbox, model-based guardrail, LLM-judge evals,
bundled Langfuse profile. Verified by `make smoke4`.

## Autonomy, safely

Rust sandbox, eval-gated self-improvement loop, REST/OpenAPI connector and demo
CRM, Helm chart, OTel collector profile. Verified by `make smoke3`.

## Operability & governance

Console UI, SSE streaming, guardrails, human-in-the-loop approvals,
LlamaIndex/pgvector context engine, deepagents profile, opt-in OpenTelemetry.
Verified by `make smoke2`.

## Core loop

Gateway, runtime, SQL connector, and the Compose demo. Verified by `make smoke`.
