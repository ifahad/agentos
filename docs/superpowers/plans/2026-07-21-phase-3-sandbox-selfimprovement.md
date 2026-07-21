# AgentOS Phase 3 — Sandbox, Self-Improvement, Connectors, Helm

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Agents that can safely run generated code (Rust sandbox), measurably improve themselves under human control (eval-gated prompt proposals), reach REST legacy systems (OpenAPI → MCP connector), and deploy to Kubernetes (Helm) with real telemetry (OTel collector profile).

**Architecture:** Additive; Phase 1/2 contracts stay valid. Five parallel tasks against frozen contracts; task 6 integrates. The self-improvement loop is **never free-running**: propose → auto-evaluate → human approve, every time.

**Tech Stack:** Rust (tokio + axum) for `sandbox/`; Go + mcp-go for `connectors/rest/` and a dependency-free demo CRM; Python additions in `runtime/`; Helm 3; otel-collector-contrib.

## Global Constraints

- Existing tests stay green; new code tested to the same standard.
- All model calls (including reflection/eval-judge) go through the gateway.
- Prompt changes NEVER activate without an explicit human approval call.
- Sandbox defense-in-depth: rlimits + cleared env + fresh workdir in-process, plus hardened container (read_only, cap_drop ALL, no-new-privileges, tmpfs /tmp, mem/cpu limits). Documented residual risk: in-container network egress (mitigation on the roadmap: egress-less sidecar topology).

## Frozen contracts

### sandbox/ (Rust) — HTTP :8070

- `POST /execute` `{"language":"python","code":"…","timeout_s":10,"stdin":""}` →
  `{"exit_code":0,"stdout":"…","stderr":"…","duration_ms":N,"timed_out":false,"truncated":false}`.
  `language`: only `"python"` in Phase 3 (400 `{"error":"unsupported language"}` otherwise). `timeout_s` default 10, capped at `AGENTOS_SANDBOX_MAX_TIMEOUT_S` (default 30). stdout/stderr each capped at `AGENTOS_SANDBOX_MAX_OUTPUT_BYTES` (default 65536, sets `truncated`). Wall-clock timeout → SIGKILL to the process group, `timed_out:true`, exit_code -1.
- Isolation per execution: fresh temp workdir (removed after), env cleared to `PATH=/usr/local/bin:/usr/bin:/bin` only, `pre_exec` setrlimit: RLIMIT_CPU=timeout_s, RLIMIT_AS=512 MiB, RLIMIT_NPROC=64, RLIMIT_FSIZE=8 MiB; new process group.
- `GET /healthz` → `ok`. Container: image with python3 + non-root user; compose hardening per Global Constraints.

### runtime — self-improvement (all endpoints additive)

- Eval suites: YAML at `runtime/evals/<suite>.yaml`:
  `cases: [{name, input, expect_substring, expect_tool?}]` (substring case-insensitive; expect_tool = tool name that must appear in steps). Ship `default.yaml` with 4 cases against the seeded ERP + vendor-policy doc.
- Postgres tables (checkpoint DB, created lazily): `eval_runs(id, suite, score, passed, failed, cases jsonb, prompt_source, created_at)`, `prompt_proposals(id, prompt_text, rationale, baseline_score, candidate_score, status, created_at)` — status ∈ `passed_evals|failed_evals|approved|denied`, `active_prompt(id=1, prompt_text, proposal_id, updated_at)`.
- `POST /evals/run` `{"suite":"default","prompt_override":?}` → `{"run_id","suite","score","passed","failed","cases":[{name,passed,output_snippet}]}` (runs each case through the agent; prompt_override builds a temp agent).
- `GET /evals/runs?limit=20` → recent runs (no cases detail).
- `POST /improve` `{}` → 1) baseline = latest eval run (auto-runs suite if none), 2) reflection prompt to the model via gateway with failed cases + current prompt → new system prompt + rationale, 3) auto-eval candidate via prompt_override, 4) store proposal, status `passed_evals` iff candidate_score >= baseline_score. Returns the proposal JSON. 502 if the model call fails.
- `GET /proposals?limit=20`; `POST /proposals/{id}/approve` `{"approve":bool}` → approved: write `active_prompt`, rebuild the live agent with it; denied: status only. 404 unknown; 409 if status not `passed_evals`/`failed_evals` (already decided). Approving a `failed_evals` proposal is allowed (human override) — flag `"warning":"candidate scored below baseline"`.
- `GET /prompts/active` → `{"source":"default"|"proposal","prompt":"…","proposal_id":?}`. Agent uses the active prompt at build; activation hot-swaps the graph.
- Sandbox tool: env `AGENTOS_SANDBOX_URL` (empty=off) registers tool `run_python(code: str)` → POST {url}/execute (timeout_s 10), returns stdout/stderr/exit_code text.

### connectors/rest/ (Go) — MCP :8091/mcp

- Env: `AGENTOS_REST_SPEC_URL` (OpenAPI 3 JSON, http(s):// or file path, required), `AGENTOS_REST_BASE_URL` (optional server override), `AGENTOS_REST_ALLOW_MUTATIONS` (default false → GET operations only), `AGENTOS_REST_AUTH_HEADER` (optional raw header `Name: value` attached to upstream calls), `AGENTOS_REST_MAX_BODY_BYTES` (default 65536).
- Server name `agentos-rest`, StreamableHTTP `/mcp` :8091. Tools: `list_operations()` → `[{operation_id, method, path, summary}]`; one tool per spec operation (name = operationId, snake_cased), string-typed args from path+query parameters; executes the request, returns `{"status":N,"body":<json-or-text, truncated>}`. No request-body support in Phase 3 (mutations gated off by default anyway; documented).
- Parse the spec with plain encoding/json structs (paths → methods → operationId/parameters/summary); no heavyweight OpenAPI lib.

### deploy/demo-crm/ (Go, stdlib only) — :8095

Tiny "legacy CRM" REST API for the demo: `GET /openapi.json` (hand-written spec), `GET /customers?city=` , `GET /customers/{id}`, `GET /tickets?status=` (open|closed). Static in-memory data consistent with the ERP world (same customer names; tickets referencing them, e.g. Red Sea Logistics has 2 open tickets). Dockerfile.

### deploy/helm/agentos/

Helm 3 chart: templates for gateway/runtime/sql-connector/rest-connector/sandbox/console (Deployment+Service each, per-service `enabled` flag), Postgres StatefulSet (or `externalDatabaseUrl`), Secret refs for provider keys (`existingSecret`), console Ingress (optional), values.yaml with image repo/tag per service. Must pass `helm lint` and `helm template` (rendered manifests spot-asserted by a small shell test `deploy/helm/test-render.sh`).

### deploy/compose.otel.yaml (orchestrator)

Overlay adding `otel-collector` (contrib image, config `deploy/otel-collector.yaml`: OTLP HTTP receiver :4318, debug exporter; optional forward via env) and setting `AGENTOS_OTEL_ENDPOINT=http://otel-collector:4318` on gateway + runtime. Langfuse/LangSmith forwarding documented in README (set their OTLP endpoint + auth header in the collector config).

---

### Task 1: Rust sandbox — `sandbox/` (axum + tokio; integration tests exec real python3; `cargo fmt --check`, `cargo clippy -- -D warnings`, `cargo test` green; Dockerfile rust build → slim runtime with python3, non-root)
### Task 2: Runtime self-improvement + sandbox tool — `runtime/` (evals module, improve module, prompt store; tests with fake model: eval scoring incl. expect_tool, improve flow creating passed/failed proposals, approve hot-swap, 409/404 paths, run_python tool registration; all Phase 1/2 tests stay green)
### Task 3: REST connector + demo CRM — `connectors/rest/`, `deploy/demo-crm/` (spec-parse table tests, GET-only gating test, auth header pass-through test, demo CRM handler tests)
### Task 4: Console — Improve page (eval runs, Run evals / Propose improvement buttons, proposals with prompt preview + rationale, baseline vs candidate scores, Approve/Deny incl. below-baseline warning) + nav; vitest for new pure logic; build green
### Task 5: Helm chart — `deploy/helm/agentos/` + `test-render.sh` (helm binary: install locally if absent)
### Task 6: Integration (orchestrator) — compose: sandbox + rest-connector + demo-crm services, runtime env (AGENTOS_SANDBOX_URL, AGENTOS_MCP_SERVERS += rest-connector), otel overlay + collector config, `scripts/smoke3.sh`, README/docs/.env.example updates.

## Self-review notes
- SOAP/SSH/browser connectors and the egress-less sandbox topology remain deferred (roadmap).
- The reflection step consumes budget through the runtime's virtual key — visible in /admin/usage, which is the point.
- REST connector Phase 3 limitation (no request bodies) is explicit in its README.
