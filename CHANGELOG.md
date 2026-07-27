# Changelog

All notable delivery milestones. Each phase was smoke-tested live end-to-end
before it was considered done; the verification targets are documented in
[`docs/operations.md`](docs/operations.md).

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
