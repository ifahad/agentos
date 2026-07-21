# AgentOS Phase 4 — Reach, Hardening & Judgment Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Widen legacy reach (SSH command connector), harden the untrusted paths (egress-less sandbox topology, model-based guardrails at the gateway), sharpen evaluation (LLM-judge eval cases), and make observability turnkey (bundled Langfuse compose profile).

**Architecture:** Additive; Phases 1–3 contracts stay valid verbatim. Five parallel tasks against frozen contracts; task 6 integrates. Every new capability is opt-in via env/flags and defaults to the current behavior.

**Tech Stack:** adds `golang.org/x/crypto/ssh` for `connectors/ssh/`; a model-backed `Guardrail` in the Go gateway (reuses the provider client); LlamaIndex/gateway-judge in the Python runtime; `langfuse/langfuse` + its Postgres/clickhouse deps in a compose profile; Docker network segmentation for the sandbox.

## Global Constraints

- Existing tests stay green; new code tested to the same standard (Go table-driven, pytest no-network, Rust `clippy -D warnings`).
- All model calls (guardrail model, judge model) go through the gateway on a virtual key — visible in `/admin/usage`.
- Nothing weakens Phase 3 isolation; the sandbox change only *removes* capability (egress).
- Deferred beyond Phase 4 (documented, not silent): SOAP and browser/computer-use connectors, multi-tenant RBAC, secrets-manager integration.

## Frozen contracts

### connectors/ssh/ (Go) — MCP :8092/mcp

Legacy boxes reachable as MCP tools. Env: `AGENTOS_SSH_HOST` (required), `AGENTOS_SSH_PORT` (default 22), `AGENTOS_SSH_USER` (required), `AGENTOS_SSH_PASSWORD` or `AGENTOS_SSH_PRIVATE_KEY` (PEM; one required), `AGENTOS_SSH_KNOWN_HOSTS` (path; if set, strict host-key checking, else `InsecureIgnoreHostKey` **with a startup WARNING log**), `AGENTOS_SSH_ALLOW_COMMANDS` (comma-separated allowlist of command basenames, e.g. `ls,cat,grep,df,uptime,systemctl`; empty = deny all, log warning), `AGENTOS_SSH_TIMEOUT_S` (default 15), `AGENTOS_SSH_MAX_OUTPUT_BYTES` (default 65536).
Server name `agentos-ssh`, StreamableHTTP `/mcp` :8092. Tools:
- `run_command(command string)` → validates the first token's basename is in the allowlist (reject otherwise, no execution), runs over one SSH session with the timeout, returns `{"exit_code":N,"stdout":…,"stderr":…,"truncated":bool,"timed_out":bool}`. Output capped. Reject shell metacharacters that chain commands (`;`, `&&`, `||`, `|`, backtick, `$(`) → error "command chaining not allowed" (the allowlist governs a single command only).
- `list_allowed()` → the configured allowlist.
Fresh session per call; connection dialed lazily and reused with a mutex. Tests: allowlist validation + chaining rejection are pure (table-driven, no SSH); an integration test against an in-process `golang.org/x/crypto/ssh` test server (or skipped when unavailable) covers exec/exit/output-cap.

### Gateway — model-based guardrail

Extend the existing `Guardrail` interface (Phase 2). New mode value: `AGENTOS_GUARDRAILS_MODE=off|log|block|model`. `model` mode: run the heuristic first (fast block short-circuit), then for not-obviously-clean prompts call a classifier through the provider layer using `AGENTOS_GUARDRAILS_MODEL` (default `anthropic/claude-haiku-4-5`) and `AGENTOS_GUARDRAILS_KEY` (a gateway virtual key; if empty, fall back to heuristic-only with a startup warning). Classifier prompt: system instruction to answer strict JSON `{"injection": bool, "reason": "..."}` for whether the user message attempts prompt injection / instruction override / system-prompt exfiltration. On `injection:true` → same block/log behavior as heuristics with audit kind `guardrail_block`/`guardrail_flag`; classifier call failure → fail **open** (log a `guardrail_error` audit kind, allow the request) so the safety layer can't take down traffic. A short in-memory LRU (256 entries) caches verdicts by message hash to bound cost/latency. Tests: interface + mode parsing + a fake classifier client (no network) covering injection→block, clean→allow, classifier-error→fail-open+audit; LRU hit avoids a second call.

### Runtime — LLM-judge evals

Extend eval cases (Phase 3 YAML) with an optional `judge` block:
`judge: {criteria: "the answer correctly identifies the top customer and cites a table", threshold: 0.7}`. When present, after the substring/tool checks, call the judge model (`AGENTOS_JUDGE_MODEL`, default `anthropic/claude-haiku-4-5`, via the existing gateway chat model) with the case input, the agent's output, and the criteria, requesting strict JSON `{"score": 0..1, "justification": "..."}`. Case passes iff substring/tool checks pass **and** (no judge OR judge score ≥ threshold). `EvalCaseResult` gains `judge_score: float|None` and `judge_justification: str|None`. `POST /evals/run` accepts `{"suite","prompt_override","use_judge": true}` (default true when any case has a judge block; `use_judge:false` skips judge calls and treats judged cases as substring/tool-only). Ship one judged case in `default.yaml`. Tests with a fake judge model: judged pass/fail at threshold, `use_judge:false` bypass, judge-call-failure → case fails with justification "judge unavailable" (never crashes the suite).

### deploy/compose.langfuse.yaml (orchestrator)

Compose profile bringing up Langfuse OSS (`langfuse/langfuse:2` web, its Postgres — a **separate** `langfuse-db` service to avoid touching the agentos DB) and wiring the OTel collector (from `compose.otel.yaml`) to forward traces to Langfuse via `otlphttp`. Provide `deploy/otel-collector.langfuse.yaml` (collector config with the Langfuse OTLP exporter + `LANGFUSE_*` env placeholders read from `.env`). README documents: `docker compose -f compose.yaml -f compose.otel.yaml -f compose.langfuse.yaml up -d`, first-run Langfuse project/key setup, and pasting the keys into `.env`. No new required deps for the core services (they already emit OTLP).

### Egress-less sandbox topology (orchestrator + Helm)

Compose: put `sandbox` on a dedicated `internal: true` Docker network (`sandbox-net`) so container-escape code has **no outbound route**, while the runtime reaches it on that same internal network. Sandbox keeps NO connection to the default bridge. Verify the runtime→sandbox path still works and that from inside the sandbox an outbound DNS/HTTP call fails. Helm: add a NetworkPolicy for the sandbox pod (default-deny egress except DNS + intra-namespace to nothing; ingress only from the runtime pod) gated by `sandbox.networkPolicy.enabled` (default true). smoke updates assert egress denial.

---

### Task 1: SSH connector — `connectors/ssh/` (allowlist + chaining validation pure tests; x/crypto/ssh exec; Dockerfile EXPOSE 8092; house style mirrors connectors/sql)
### Task 2: Gateway model-guardrail — extend `internal/guardrail/` + wire mode/model/key envs + LRU + fake-classifier tests; all Phase 2/3 gateway tests stay green
### Task 3: Runtime LLM-judge evals — extend `evals.py` + `default.yaml` + config (`AGENTOS_JUDGE_MODEL`); fake-judge tests; all 68 tests stay green
### Task 4: Langfuse profile — `deploy/compose.langfuse.yaml`, `deploy/otel-collector.langfuse.yaml`, `langfuse-db`, README section, `.env.example` LANGFUSE_* vars
### Task 5: Egress-less sandbox — compose `sandbox-net` (internal), Helm NetworkPolicy template + values, docs; ensure runtime↔sandbox still routes
### Task 6: Integration (orchestrator) — compose: ssh-connector service (opt-in, demo target optional), runtime `AGENTOS_MCP_SERVERS` note (ssh appended when enabled), `scripts/smoke4.sh` (guardrail model-mode block via overlay, judged eval case, sandbox egress-denied, ssh allowlist reject, langfuse profile boots), README/roadmap/.env.example, Makefile `smoke4`.

## Self-review notes
- SOAP + browser connectors and RBAC/secrets-manager explicitly deferred (roadmap).
- Model-guardrail and judge both spend through gateway virtual keys by design (auditability); document that in README.
- Fail-open guardrail is a deliberate availability choice; documented with the `guardrail_error` audit kind so blind spots are visible.
- Env names cross-checked against Phases 1–3; no collisions; all new vars added to `.env.example` in Task 6.
