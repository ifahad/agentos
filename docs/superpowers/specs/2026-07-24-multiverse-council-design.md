# Multiverse: a governed council of model-bound deep agents

Status: approved design (2026-07-24). Successor to the parked Phase 8 autonomy
plan (`docs/superpowers/plans/2026-07-21-phase-8-autonomous-operations.md`),
which this spec absorbs.

## Summary

**Multiverse** runs one objective through *N* independent deep agents, each bound
to a different frontier model, then synthesizes their answers into a single
verdict with an explicit dissent report. The council is the autonomous unit: a
heartbeat pulls objectives from a queue, runs a fan-out/judge cycle, records the
verdict, and decides whether to continue — under cycle caps, a spend ceiling, a
kill switch, and human approval for every write.

Target members (all OpenAI-compatible):

| Member | Model | Vendor |
|---|---|---|
| alpha | Kimi K3 | Moonshot AI |
| beta | GLM-5.2 | Zhipu AI / Z.ai |
| gamma | Qwen 3.8 Max | Alibaba |
| delta | DeepSeek-V4 Pro | DeepSeek |
| epsilon | MiniMax M3 | MiniMax |

"OpenAI-compatible" applies in **both directions**: the gateway *consumes* each
vendor's OpenAI-compatible endpoint, and the council itself is *exposed* as a
single OpenAI-compatible model (`council/multiverse`).

## Why this needs the work below first

Three facts about the current tree make Multiverse impossible today:

1. **The gateway cannot route to any of these vendors.** `provider.go:87-112` is
   a hardcoded switch over exactly `anthropic/`, `openai/`, `ollama/`; every
   other prefix returns `ErrUnknownProvider`.
2. **Budgets are inert for these models.** The price table
   (`provider.go:128-132`) has three entries and `Cost()` returns 0 for anything
   else. Five autonomous agents on paid APIs with `Cost()==0` means org budgets,
   key budgets, and `/admin/usage` all report zero — the control that stops a
   runaway loop would not fire.
3. **The runtime builds exactly one agent.** The lifespan in `api.py` constructs
   a single graph from a single `AGENTOS_MODEL` (`config.py:36`).

Two known harness gaps also become mandatory rather than optional once loops
self-trigger against third-party APIs: there is **no per-run cycle cap** (only
LangGraph's default `recursion_limit`), and there is **no provider
retry/fallback** — a single upstream 502 ends a run.

## Non-goals (v1)

- The council does not invent its own objectives. Objectives are human-created.
  Self-generated goals are deliberately deferred until the loop has been
  observed running.
- No debate/critique rounds between members. Members answer independently; only
  the judge sees all answers.
- No changes to OpenClaw. It benefits automatically via `council/multiverse`.

## Architecture

```
POST /council/objectives ──► objectives queue (Postgres)
                                    │
     heartbeat (SKIP LOCKED claim) ─┘
                │
                ▼   one cycle
        ┌───────────────────────────────────────┐
        │  fan-out (concurrent, per-member cap) │
        │   alpha  → gateway → moonshot/kimi-k3 │
        │   beta   → gateway → zhipu/glm-5.2    │
        │   gamma  → gateway → alibaba/qwen…    │
        │   delta  → gateway → deepseek/…       │
        │   epsilon→ gateway → minimax/…        │
        └───────────────┬───────────────────────┘
                        ▼  quorum reached?
                    judge synthesize
                        ▼
             verdict + dissent → persist
                        ▼
        continue? (agreement · max_cycles · spend · paused)
```

Every member call traverses the existing gateway, so keys, budgets, rate limits,
guardrails, and audit apply unchanged and per member.

---

## 1. Gateway: OpenAI-compatible provider registry

Replace the switch in `provider.go` with a registry keyed by model prefix,
seeded with built-in defaults for `anthropic`, `openai`, and `ollama` (exact
current behaviour, so nothing regresses) and extended from
`AGENTOS_PROVIDERS_FILE` (default `deploy/providers.yaml`).

```yaml
providers:
  - name: moonshot                      # model string: "moonshot/kimi-k3"
    base_url: https://api.moonshot.ai
    key_name: AGENTOS_MOONSHOT_API_KEY  # resolved via secret.Source
    enabled: false                      # operator must confirm endpoint+pricing
    chat_path: /v1/chat/completions     # optional override
    max_attempts: 2                     # optional per-provider retry cap
    prices:                             # USD per 1M tokens
      kimi-k3: { in: 1.0, out: 4.0 }
```

- **Key resolution** reuses the existing live-`secret.Source`-wins-over-static
  pattern (`provider.go:56-73`), generalized to any `key_name`. Vault/age/file
  rotation therefore covers new providers with no additional code.
- **`Cost()`** consults the registry's per-model prices, falling back to the
  built-in table, then to 0. Budgets become real for the five vendors.
- **Unknown prefixes** still return `ErrUnknownProvider`.
- A provider with no resolvable key is **disabled at load** and reported by
  `GET /admin/providers` (read-only, no secret values) rather than failing at
  request time with an opaque upstream 401.

**Security — operator-supplied base URLs are attacker-adjacent config.** Each
`base_url` is validated at load: scheme must be `https` (or `http` only for
explicit loopback/`ollama`-style local hosts), and the host is checked with the
existing `safehttp.IsDisallowedHost` so a registry entry cannot point the
gateway at link-local/private metadata endpoints (the SSRF class already fixed
for REST/SOAP in the security pass). Invalid entries are rejected at load with a
logged reason; the gateway still starts on its built-in providers.

Prices are **config, not code**, precisely because published pricing and model
IDs for these vendors change and are not verifiable from this environment. The
shipped `providers.yaml` carries the five vendors with placeholder prices,
`enabled: false`, and a comment requiring the operator to confirm current
endpoint/model/pricing before enabling.

## 2. Gateway: bounded retry and fallback

- Retry on connection errors, timeouts, `429`, and `5xx`. Never on `4xx` other
  than 429. Exponential backoff with jitter, honoring `Retry-After` when present.
- `max_attempts` defaults to **2** (one retry), overridable per provider.
- Retries happen **after** the auth → guardrail → rate-limit → budget checks, so
  a retry can never bypass governance. Each attempt is recorded so retry spend
  is visible.
- Streaming requests retry only before the first byte reaches the client.
- Optional `fallback_model` per council member (§3): if the primary is
  exhausted, the member reruns once on the fallback and the verdict records
  which model actually answered.

Trade-off stated plainly: retries multiply spend, which is why the cap is low,
per-provider, and audited.

## 3. Runtime: council member registry

`AGENTOS_COUNCIL_CONFIG` (default `runtime/council.yaml`):

```yaml
judge: ollama/qwen3.6:latest
quorum: 3                 # members that must answer for a valid verdict
agreement_threshold: 0.6  # below this, objective goes to needs_review
max_cycles: 8             # per objective
max_tool_steps: 12        # per member per cycle -> LangGraph recursion_limit
member_timeout_s: 300
members:
  - id: alpha
    model: moonshot/kimi-k3
    enabled: false
    profile: deep          # react | deep
    persona: "Prioritize schema inspection before answering."
    tools: [sql, rag, sandbox]
    budget_usd_per_cycle: 0.50
    fallback_model: ""
```

- Each enabled member gets its **own compiled graph**, its **own gateway virtual
  key** (so `/admin/usage` attributes spend per member), and its **own thread
  namespace** `{objective_id}:{member_id}` (so checkpoints and history never
  collide).
- `build_agent()` (`agent.py:68`) already accepts model and prompt overrides —
  member construction is wiring, not new agent code.
- `persona` refines the system prompt; `build_system_prompt()` still prepends
  the immutable `SAFETY_PREAMBLE` (`agent.py:30-51`), so no member configuration
  can weaken the safety frame.
- `tools` selects from the tools the runtime already loaded; a member cannot
  name a tool the runtime did not load.
- Members with `enabled: false` are configuration only — never called, never
  counted toward quorum.

## 4. Council orchestration and the autonomous loop

New module `runtime/src/agentos_runtime/council.py`.

**`fanout(objective, cycle)`** — runs enabled members concurrently
(`asyncio.gather(..., return_exceptions=True)`), each with `member_timeout_s`
and `recursion_limit=max_tool_steps`. A member that errors, times out, or
exhausts its per-cycle budget is **isolated, not fatal**: its failure is
recorded and the cycle proceeds if the remaining answers meet `quorum`. Below
quorum, the cycle fails with `quorum_not_met` and the objective stops for review.

**`synthesize(answers)`** — one judge call (`AGENTOS_JUDGE_MODEL`, through the
gateway like every other call) returning strict JSON:

```json
{
  "answer": "synthesized verdict",
  "agreement": 0.6,
  "dissent": [{"member": "gamma", "claim": "...", "basis": "..."}],
  "cited_members": ["alpha", "delta", "epsilon"],
  "done": false
}
```

Unparseable judge output is retried once, then the cycle is recorded with
`judge_unavailable` and the objective moves to `needs_review` — matching the
existing eval-judge failure convention (`evals.py` docstring): a judge outage
degrades one cycle, never the whole loop.

**`cycle()`** = fanout → synthesize → persist → decide.

**Heartbeat** — a background task on `AGENTOS_COUNCIL_HEARTBEAT_S` (0 = off,
the default) claims the next pending objective with
`SELECT ... FOR UPDATE SKIP LOCKED`, so multiple runtime replicas never
double-run one objective.

**Stop conditions**, all enforced and all recorded as the objective's
`stop_reason`:

| Reason | Trigger |
|---|---|
| `converged` | `agreement >= agreement_threshold`, or judge `done: true` |
| `max_cycles` | cycle count reached `max_cycles` |
| `budget_exceeded` | objective spend reached its ceiling |
| `paused` | kill switch engaged |
| `cancelled` | operator cancelled |
| `needs_review` | below-threshold agreement, quorum failure, or judge unavailable |

Below-threshold agreement routes to `needs_review` rather than burning another
cycle.

**Action surface — read and propose.** Members read freely (SQL read-only, REST
GET, RAG search, the egress-less sandbox). Any write-class tool call is **not
executed**; it is written to `council_proposals` and requires
`POST /council/proposals/{id}/approve`, the same human gate that already governs
prompt self-improvement. Classification is by an explicit allowlist of read-safe
tools — anything unrecognized is treated as write-class (fail-closed).

### Persistence

Extends the existing `ImprovementStore` pattern (idempotent
`CREATE TABLE IF NOT EXISTS` on the checkpoint database):

- `council_objectives` — id, input, status, stop_reason, cycles_run, spend_usd,
  created_at, claimed_at, claimed_by
- `council_cycles` — id, objective_id, cycle_no, verdict JSONB, agreement,
  dissent JSONB, started_at, finished_at
- `council_member_runs` — id, cycle_id, member_id, model_used, thread_id,
  status, output, steps JSONB, tokens, cost_usd, error
- `council_proposals` — id, objective_id, member_id, tool, arguments JSONB,
  status, created_at, decided_at

## 5. Runtime API and console

```
POST /council/objectives              create {input, max_cycles?, budget_usd?}
GET  /council/objectives              list with status
GET  /council/objectives/{id}         cycles, per-member answers, verdicts, dissent
POST /council/objectives/{id}/cancel
POST /council/pause | /council/resume kill switch (global)
GET  /council/members                 registry health: enabled, model, latency,
                                      error rate, spend
POST /council/proposals/{id}/approve  approve a held write-class action
```

All routes sit behind the existing app-wide `require_auth` dependency
(`api.py`), so the runtime token is required exactly as for `/runs`.

**Console** gains a **Multiverse** page: a member grid (model · enabled ·
latency · error rate · spend · agreement rate), an objective timeline with
per-cycle agreement, and a verdict view that shows the synthesized answer beside
each dissent with its basis. Pause/resume and proposal approval are surfaced
here.

## 6. Council as an OpenAI-compatible model

The gateway exposes the council as one model:

```
POST /v1/chat/completions  {"model": "council/multiverse", ...}
```

The gateway forwards to the runtime's council endpoint, runs a **single
synchronous cycle** (not the heartbeat loop), and returns a normal
chat-completion whose content is the synthesized verdict; dissent is attached in
an `x_agentos_council` extension field. Streaming emits progress events
(`member_started`, `member_done`, `judging`) followed by the verdict, so an
OpenAI client that ignores extensions still sees a valid stream.

Any OpenAI-compatible client — **including the governed OpenClaw worker from
`docs/interop/openclaw.md`** — gets a five-model council behind one model name
with no client-side change.

**Recursion guard (required).** A council member must never be able to request a
`council/*` model: that is unbounded recursion and a spend bomb. The gateway
rejects `council/*` when the request carries the council depth marker
(an internal header set on member calls), and members' configured models are
validated at council load to reject any `council/` prefix. Both layers are
tested.

Usage accounting: member calls are billed **once**, to their own member keys
(§3). The `council/multiverse` request itself records **zero direct spend** and
stores the summed member cost as a reported `council_spend_usd` on its audit
entry, linked to the member runs. Spend is therefore visible to the caller
without being double-counted in `/admin/usage`. Budget enforcement for a council
call happens on the member keys and the council org, and an objective's ceiling
is checked against that same summed cost.

## Configuration summary

| Variable | Default | Meaning |
|---|---|---|
| `AGENTOS_PROVIDERS_FILE` | `deploy/providers.yaml` | provider registry path |
| `AGENTOS_COUNCIL_CONFIG` | `runtime/council.yaml` | member registry path |
| `AGENTOS_COUNCIL_HEARTBEAT_S` | `0` (off) | autonomous loop interval |
| `AGENTOS_COUNCIL_MAX_SPEND_USD` | `5` | default per-objective ceiling |
| `AGENTOS_<VENDOR>_API_KEY` | — | per-provider credential |

Defaults are deliberately conservative: **the heartbeat is off** and the five
frontier members are **disabled** until an operator enables them.

## Testing

**Go (gateway)**
- registry load: valid/invalid YAML, unknown prefix still `ErrUnknownProvider`,
  built-in providers unchanged (regression)
- base-URL validation rejects private/link-local/metadata hosts and non-HTTPS
  remotes
- pricing: registry price beats built-in; unknown model still 0
- key resolution: live secret source wins; missing key disables the provider
- retry: backoff on 429/5xx against a flaky test server, no retry on 400,
  `Retry-After` honored, attempts capped
- `council/*` rejected when the depth marker is present

**Python (runtime)**
- fan-out with injected fake tool-calling models (existing test pattern) —
  distinct models produce distinct member answers
- quorum: 5-member config with 3 failures → `quorum_not_met`; with 2 failures →
  valid verdict
- malformed judge output → one retry → `judge_unavailable` → `needs_review`
- `max_cycles`, budget ceiling, and pause each stop the loop with the right
  `stop_reason`
- write-class tool call becomes a proposal and is not executed; unknown tool
  classified write-class (fail-closed)
- member config naming a `council/` model is rejected at load

**Live smoke (`scripts/smoke9.sh`)** — runs a real objective against the seeded
legacy ERP database using **five local Ollama models** as council members
(`qwen3.6`, `qwen3.5`, `gemma4:31b`, `gemma4`, `gemma3` — all present on this
host, $0 cost). Asserts: five distinct members answered, per-member usage
attributed in `/admin/usage`, a verdict with dissent was persisted, the cycle
cap trips, pause halts the loop, and `council/multiverse` returns a synthesized
answer through `POST /v1/chat/completions`.

**CI** uses a deterministic mock OpenAI-compatible upstream (extending
`deploy/ci/mock-model.py`) so registry routing, retry, quorum, and fallback are
verified without network or spend.

### Verification honesty

Endpoint URLs, exact model IDs, and pricing for Kimi K3, GLM-5.2, Qwen 3.8 Max,
DeepSeek-V4 Pro, and MiniMax M3 are **not verified** in this environment (no
provider keys are configured, and `ollama/deepseek-v4-pro:cloud` returns
`requires a subscription`). Everything vendor-specific is therefore
configuration with placeholders and `enabled: false`. The shipped
`providers.yaml` states this in a header comment. The council mechanism itself
is proven live on local models.

## Implementation sequence

| Step | Scope | Independently verifiable by |
|---|---|---|
| 8a | provider registry, pricing, base-URL SSRF guard, `GET /admin/providers` | Go tests + a live call through a new prefix |
| 8b | retry/backoff/fallback, cycle caps (`max_tool_steps`) | Go tests against a flaky server; runtime recursion-limit test |
| 8c | council member registry, per-member keys/threads | Python tests; `GET /council/members` |
| 8d | fan-out + judge synthesis + persistence + `POST /council/objectives` | Python tests; live 5-local-model cycle |
| 8e | heartbeat loop, stop conditions, pause, write-class proposals | Python tests; `smoke9.sh` |
| 8f | `council/multiverse` model, recursion guard, console Multiverse page | Go + vitest; smoke9 final assertion |

Each step lands as its own commit with tests green before the next begins.
