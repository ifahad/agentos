# AgentOS Phase 8 — Autonomous Operations (heartbeat, skills, OpenClaw interop)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give AgentOS governed, always-on autonomy — a scheduler/heartbeat that fires agent runs on triggers (interval/cron/webhook) toward stored objectives — plus portable `SKILL.md` skills, and first-class **OpenClaw interoperability** (OpenClaw drives its model calls through the AgentOS gateway for governance). Every autonomous run is governed (gateway budgets/rate-limits/audit), bounded (max cycles), and respects the existing HITL approval gate.

**Architecture:** Additive; Phases 1–7 contracts stay valid VERBATIM. The autonomy engine lives in the Python runtime (it already owns the agent, checkpointer, and gateway client); the gateway is unchanged (autonomous runs are just governed model traffic). Three tasks: runtime autonomy engine, console autonomy UI, interop/skills/docs. Task 4 integrates.

**Tech Stack:** Python (runtime — asyncio scheduler, `croniter` for cron, Postgres persistence, `SKILL.md` loader), React/TS (console), Markdown (`SKILL.md` skills + OpenClaw interop guide).

## Global Constraints

- **Backward compatibility mandatory.** Every Phase 1–7 endpoint, env var, and behavior is unchanged. Autonomy is opt-in: the scheduler only runs when `AGENTOS_AUTONOMY_ENABLED=true` AND at least one enabled objective exists.
- **Safety by construction:** every autonomous run goes through the gateway on the runtime's virtual key (budgets, rate limits, guardrails, audit all apply); each objective has a `max_cycles` cap; a global kill-switch (`AGENTOS_AUTONOMY_ENABLED=false`) stops the scheduler; runs that hit a HITL approval interrupt are recorded `needs_approval` and NOT auto-resumed (a human approves via the existing endpoint).
- Existing tests stay green; new code tested to the same standard (pytest, no network — fake model/clock). Persistence behind a small store class so tests use an in-memory fake.
- Deferred beyond Phase 8 (documented): messaging-channel gateways (Slack/WhatsApp — OpenClaw already does this and can front AgentOS), ClawHub-style public skill registry, distributed multi-node scheduler.

## Frozen contracts

### Runtime — autonomy engine (additive, opt-in)

Persistence (Postgres checkpoint DB, lazy `CREATE TABLE IF NOT EXISTS`): `agentos_objectives(id, name, goal, trigger_type, trigger_config jsonb, enabled bool, max_cycles int, created_at)` and `agentos_autonomous_runs(id, objective_id, thread_id, status, output, steps jsonb, cycles int, trigger_source, created_at)`. `status ∈ completed|needs_approval|error`. All self-improvement/eval endpoints already gate on the checkpoint DB; autonomy 503s the same way when it's unset.

- **Objective** shape: `{id, name, goal, trigger:{type, interval_s?, cron?, webhook_token?}, enabled, max_cycles, created_at}`. `type ∈ interval|cron|webhook`. `interval` needs `interval_s` (≥ 30); `cron` needs `cron` (5-field, validated with croniter); `webhook` auto-generates a `webhook_token` (returned once on create; opaque `whk-…`).
- Endpoints:
  - `POST /objectives` `{name, goal, trigger, max_cycles?, enabled?}` → the objective (webhook token included when type=webhook). Validate trigger; 400 on bad cron/interval.
  - `GET /objectives` → list (no webhook tokens).
  - `GET /objectives/{id}` → `{objective, recent_runs:[…]}` (last 10 runs); 404 unknown.
  - `PATCH /objectives/{id}` `{enabled?, goal?, max_cycles?, trigger?}` → updated objective (pause/resume via `enabled`).
  - `DELETE /objectives/{id}` → 204.
  - `POST /objectives/{id}/run` → fire once now (manual trigger, `trigger_source:"manual"`); returns the autonomous run (or 202 `needs_approval`).
  - `GET /autonomous-runs?objective_id=&limit=20` → run history (newest first).
  - `POST /webhooks/{webhook_token}` (no admin auth; token is the secret) → fire the matching `webhook` objective with the request JSON body merged into the goal context (`trigger_source:"webhook"`); 404 unknown/disabled token. Returns the run summary.
- **Scheduler:** one asyncio background task started in the FastAPI lifespan when `AGENTOS_AUTONOMY_ENABLED=true`. Every `AGENTOS_AUTONOMY_TICK_S` (default 15) it checks each enabled interval/cron objective's due time (tracked in-memory since last fire; cron via croniter `get_next`) and fires due runs sequentially (no overlap per objective). Webhook objectives fire only on their endpoint. A fired run: build/reuse the agent, invoke with the objective `goal` as input under a fresh `thread_id`, cap tool iterations at `max_cycles` (pass LangGraph `recursion_limit = 2*max_cycles+1` and record the tool-call count as `cycles`), persist the run. On a HITL interrupt → record `needs_approval` with the pending thread (resumable via the existing `/runs/{thread_id}/approve`). Errors are caught, recorded `error`, never crash the scheduler.
- **Config:** `AGENTOS_AUTONOMY_ENABLED` (default `false`), `AGENTOS_AUTONOMY_TICK_S` (default 15), `AGENTOS_AUTONOMY_MAX_CYCLES` (default 8 — objective default when unset).

### Runtime — SKILL.md skills (additive)

Portable, OpenClaw/Claude-Code-compatible skills. Load `*/SKILL.md` from `AGENTOS_SKILLS_DIR` (default `<pkg>/skills`, fallback `<cwd>/skills`) at startup. `SKILL.md` = YAML frontmatter `{name, description, when_to_use?}` + a Markdown body of instructions. Effects:
- The agent's system prompt gains an "Available skills" section (name — description, one per line) so the model knows what exists.
- A `use_skill(name: str)` tool returns the full skill body (instructions) for the agent to follow; unknown name → an error string listing available skills.
- `GET /skills` → `[{name, description, when_to_use?}]` (no bodies). Ship one example skill `skills/erp-analysis/SKILL.md` (how to analyze the legacy ERP: inspect schema, filter by status, cite tables). Skills load whether autonomy is on or not.

### Console — Autonomy UI

- **Autonomy** page (root or member+; gate on a new `autonomy.view` capability granted to owner/admin/member): list objectives (name, trigger summary, enabled toggle, last-run status); create-objective form (name, goal textarea, trigger type + interval/cron/webhook fields, max_cycles); per-objective "Run now" button; a run-history panel (status badges: completed/needs_approval/error) with the output + steps of a selected run; pause/resume via the enabled toggle (PATCH); delete. `needs_approval` runs link to the Playground's approve flow (same thread). Webhook objectives show their POST URL (token) once on creation.
- **Skills** panel (on the Autonomy page or its own): list from `GET /skills`.
- vitest for new pure logic (trigger-summary formatter, status→badge). Keep existing tests green; build clean.

### Interop — OpenClaw ↔ AgentOS

- `docs/interop/openclaw.md`: how to point OpenClaw at the AgentOS gateway as its OpenAI-compatible provider (base URL `http://<gateway>/v1`, a virtual `agos-` key, model `anthropic/…`|`openai/…`|`ollama/…`), so an OpenClaw fleet's model traffic is governed (budgets/rate-limits/guardrails/audit) and attributable per key/org. Note the `SKILL.md` format is shared, so skills are portable between AgentOS and OpenClaw. Include a minimal `openclaw.json` provider snippet and a curl proving an OpenClaw-style `/v1/chat/completions` call is governed.
- `skills/README.md`: the `SKILL.md` format spec (frontmatter fields + body), and that it's compatible with OpenClaw/Claude Code/Cursor.

---

### Task 1: Runtime autonomy engine + SKILL.md skills (owns runtime/). Objectives/runs store (Postgres + in-memory fake), scheduler (asyncio, interval/cron via croniter, webhook endpoint), max_cycles guard, HITL `needs_approval` handling, all endpoints, `use_skill` tool + `/skills` + skill loader + example skill. Config flags. All Phase 1–7 runtime tests green; scheduler off by default. Tests with fake model + injected clock: objective CRUD, trigger validation, a due interval fires a run, cron next-fire, webhook fires by token (404 unknown), manual run, max_cycles cap, needs_approval path, skill loading + use_skill + /skills.
### Task 2: Console Autonomy page + Skills panel + capability wiring; vitest for pure logic; keep tests green; build clean.
### Task 3: Interop + docs — docs/interop/openclaw.md, skills/README.md (SKILL.md spec), the example skill content reviewed; a governed-call curl snippet. (Owns docs/interop/ + skills/README.md; coordinate: Task 1 owns skills/erp-analysis/SKILL.md content.)
### Task 4: Integration (orchestrator) — compose: runtime autonomy env (AGENTOS_AUTONOMY_ENABLED default false, TICK_S, MAX_CYCLES, SKILLS_DIR), `scripts/smoke8.sh` (create interval objective → enable autonomy → a run fires and is recorded; manual run; webhook fire by token; skills listed + agent uses use_skill; OpenClaw-style governed call attributes usage; autonomy off by default = no scheduler), README/roadmap/.env.example, Makefile `smoke8`.

## Self-review notes
- Messaging channels, public skill registry, multi-node scheduler deferred (roadmap) — OpenClaw fronts channels and can run governed by AgentOS.
- Autonomy is opt-in and kill-switched; every run is governed + bounded + HITL-aware — directly addressing OpenClaw's unsandboxed-skill / no-central-governance gaps.
- `needs_approval` reuses the Phase 2 HITL `/runs/{thread_id}/approve` path — no parallel approval system.
- All new env vars in `.env.example` in Task 4; no collisions with Phases 1–7.
