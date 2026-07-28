# AgentOS landing-page rebuild — authoritative fact sheet

Assembled from four surveys (inventory, claim audit, mock code assessment, contrast/CVD analysis) plus an
adversarial re-check of the claim audit. Every file:line, string and numeric value below is preserved verbatim
from the source surveys or re-verified against disk.

**Output-path note.** The orchestrating prompt passed the literal token `undefined` as this document's
destination and as the surveys' input paths. Resolved targets:

| Role | Real absolute path |
|---|---|
| This fact sheet | `/tmp/claude-1000/-home-iofahd-code/ac2f7c75-c6cb-4ccb-8376-09a3dd58f318/scratchpad/landing-rebuild-factsheet.md` |
| Page being rebuilt | `/home/iofahd/code/agentos/landing/index.html` |
| Sibling readme (not read) | `/home/iofahd/code/agentos/landing/README.md` (683 bytes) |
| Design mock, source | `/tmp/claude-1000/-home-iofahd-code/ac2f7c75-c6cb-4ccb-8376-09a3dd58f318/scratchpad/mock/page.src.html` (383 lines, Jul 28 15:05) |
| Design mock, built | `/tmp/claude-1000/-home-iofahd-code/ac2f7c75-c6cb-4ccb-8376-09a3dd58f318/scratchpad/mock/agentos-landing-mock.html` (132,368 bytes) |
| Palette spec | `/home/iofahd/code/agentos/docs/superpowers/specs/2026-07-28-design-language-and-landing-design.md` |

`landing/index.html` is **868 lines** (re-verified with `wc -l`), **112,004 bytes**. Any instruction that says
865 lines is wrong.

---

## Current page inventory

### 0. Structural landmarks

- `<head>` lines 1–9. `<body>` opens line 10.
- Exactly one `<style>`: lines 11–299.
- Lines 12–24 are the two `@font-face` base64 blocks — `"Archivo"` at 12–17, `"Plex Mono"` at 19–24. The two
  payload lines are **46,637** and **19,906** characters (lines 17 and 24).
- Exactly one `<script>`: lines 543–866, containing five IIFEs.

### 1. Sections in document order

| Lines | Element | Eyebrow | Heading | Purpose |
|---|---|---|---|---|
| 301–315 | `<div class="bar">` | none | brand `AgentOS` (303) | Sticky top bar: brand + pulse dot, 4 nav anchors, theme toggle `#tt`, solid "Deploy" button scrolling to `#start`. |
| 318–364 | `<header class="hero">` | `Sovereign agentic platform` (319, in `.kicker`) | `The governed operating layer for AI agents.` (320) | Headline, lead, 2 CTAs + note; contains chain panel and readings strip. |
| 328–356 | `.chain-panel` (in hero) | `Governance chain` (330) + status `Live` (331) | — | Signature visual: 6-stage rail (Auth/Rate/Budget/Guardrail/Upstream/Audit), caption, live audit feed `#audit-feed`, throughput sparkline `#spark-line`. |
| 358–363 | `.readings` (in hero) | none | — | 4 counters: `#rd-count`, `#rd-spend`, `#rd-p95`, `#rd-audit`. |
| 366–416 | `<section id="platform">` | `The platform` (368) | `Everything an agent needs to run in production.` (369) | 6-card capability grid: Engine, Observability, Evaluation, Deployment, Sandbox, Fleet. |
| 418–441 | `<section id="governance">` | `Why AgentOS` (420) | `Governance isn't a setting. It's the architecture.` (421) | 3 numbered cards (01 Attributable, 02 Bounded, 03 Safe by default). |
| 443–489 | `<section id="multiverse">` | — | — | Wrapper for two alternating feature blocks. |
| 444–467 | `.feature` (multiverse) | `Multiverse council` (446) | `Many models. One verdict. Honest dissent.` (447) | Copy + 3 bullets, paired with `#council-viz` panel (3 member rows + verdict). |
| 469–488 | `.feature.rev` (operators) | `Operators` (471) | `Governed autonomy — on a schedule, or a webhook.` (472) | Copy + 3 bullets, paired with `#operators-viz` run-history panel (3 rows). |
| 491–502 | `<section id="oss">` | `Built on open source` (493) | `The best agent frameworks, under governance.` (494) | 3 `.oss` cards: deepagents, langgraph, langchain. |
| 505–517 | `<div class="band">` | `Sovereign by design` (508) | `Your agents. Your keys. Your infrastructure.` (509) | Full-bleed band, 3 `.claim` items. |
| 520–528 | `<section class="final" id="start">` | `Get started` (521) | `Stand up a governed agent platform in one command.` (522) | Centered final CTA; primary button label `docker compose up -d`, `onclick="copyCmd(this)"`. |
| 531–541 | `<footer>` | none | — | Mono tagline + 4 repeat nav links. |

Wrapper nesting: outer `.wrap` at **317** spans hero → `#oss` (closes **503**); the band sits *outside* it; a
second `.wrap` at **519–529** holds the final CTA.

### 2. CSS custom properties (`:root`, lines 27–46)

```
--bg: #121517;            --raised: #191d20;        --raised-2: #20252a;
--border: rgba(255,255,255,0.08);   --border-strong: rgba(255,255,255,0.14);
--ink: #e6e9eb;           --dim: #98a1a8;           --faint: #626b71;
--live: #5ad1c4;          --ok: #6cc48f;            --hold: #e3a851;   --deny: #e2685f;
--mono: "Plex Mono", ui-monospace, "Cascadia Mono", Menlo, Consolas, monospace;
--sans: "Archivo", ui-sans-serif, system-ui, "Segoe UI", Helvetica, Arial, sans-serif;
--eyebrow: 11px;          --track: 0.18em;
--shadow: 0 1px 0 rgba(255,255,255,0.03) inset, 0 20px 50px -30px rgba(0,0,0,0.8);
color-scheme: dark;
```

**A light theme EXISTS, twice over.** `:root[data-theme="light"]` (48–63) and, duplicated verbatim,
`@media (prefers-color-scheme: light) { :root:not([data-theme="dark"]) { … } }` (65–82). Both override:

```
--bg: #eceae4;  --raised: #f6f5f1;  --raised-2: #ffffff;
--border: rgba(0,0,0,0.10);  --border-strong: rgba(0,0,0,0.18);
--ink: #1a1d1f;  --dim: #565d63;  --faint: #8a9198;
--live: #12897c; --ok: #2f8f57;   --hold: #a8721c;  --deny: #bb4038;
--shadow: 0 1px 0 rgba(255,255,255,0.6) inset, 0 20px 50px -34px rgba(0,0,0,0.25);
color-scheme: light;
```

Light does **not** redeclare `--mono`, `--sans`, `--eyebrow`, `--track`.

**Broken rule, index.html:135** — invalid CSS (an at-rule inside a selector list), so `.btn-solid` never gets
white text in light mode:

```
:root[data-theme="light"] .btn-solid, @media (prefers-color-scheme: light){ :root:not([data-theme="dark"]) .btn-solid{ color: #fff; } }
```

### 3. Script blocks (one `<script>`, 543–866, five IIFEs)

- **543–567 — theme + reveal + copy.** Reads `localStorage.getItem('agentos-theme')`, sets `data-theme`;
  toggle flips against `matchMedia('(prefers-color-scheme: dark)')`. `IntersectionObserver` (threshold
  `0.12`) adds `.in` to every `.reveal`, then unobserves. `window.copyCmd` writes
  `'git clone && cd agentos/deploy && docker compose up -d'` via `navigator.clipboard` and swaps the button
  label to `'copied ✓'` for 1400 ms.
- **576–691 — readouts + audit feed + sparkline.** Defines `window.__agentosOnSettled(r)`. Running
  `requestCount`, `auditRows`, `spend` (only when `r.outcome === 'ok'`, line 606), p95 over a 40-sample
  rolling `latencies` window (`Math.floor(0.95 * (sorted.length - 1))`). Prepends `.feed-row` elements capped
  at `FEED_CAP = 6`. Sparkline buckets settled requests into `BUCKET_MS = 2500` real-time windows, keeps
  `SPARK_POINTS = 24`, normalizes to max, writes a `points` string into `#spark-line` on a 240×40 viewBox with
  `PAD = 3`. Owns no timer; `Date.now()` only buckets.
- **695–764 — governance-chain simulation.** Drives the 6 `.stage` nodes on `setInterval(tick, 520)` (line
  762). Fate decided by a seeded `mulberry32(0x5EED)` (line 710): `roll < 0.8` → clears all six (`ok`),
  `< 0.92` → halts at Upstream (`hold`), else denies at stage `1 + Math.floor(rand()*3)` (line 723 →
  Rate/Budget/Guardrail). Synthesizes `model` from `['qwen3.8-max','deepseek-v4','kimi-k3','llama-4-70b']`,
  `org` from `['acme','globex','initech','umbrella']`, `tokens: 200 + …*1800` (728),
  `cost: Math.round((0.4 + rand()*6)*100)/100` (729), `latency: 120 + …*900` (730), and hands each settled
  request to `window.__agentosOnSettled`.
- **773–804 — council quorum rotation.** Seeded `mulberry32(0xC0DE1)`, `setInterval(tick, 3200)`; each tick
  either sets no dissenter (`roll < 0.6`) or exactly one, toggling `.pill` between `ok`/`answered` and
  `hold`/`dissent` inside `#council-viz` (`setPill`, lines 791–795).
- **811–865 — operators run-history prepend.** Seeded `mulberry32(0xA0BA5)`, `setInterval(tick, 4000)`;
  prepends a `.member` row from a fixed 6-entry `RUNS` roster (line 837 contains `fs.write held`),
  `rand() < 0.3` marks it `needs approval`, trims back to original row count `CAP`.

**`grep -n "Math\.random" index.html`** returns **only comment lines — 573, 694, 770, 809** — each asserting
"no Math.random". **Zero executable use of `Math.random`.** All four PRNGs are seeded `mulberry32`; the page
is deterministic. `Math.imul`, `Math.floor`, `Math.round` are used inside the PRNGs and formatters.

### 4. Layout classes and keyframes

Declaration lines: `.wrap` (98), `.bar` (114) / `.bar-in` (120), `.brand` (121), `.nav` (124),
`.theme-toggle` (136), `.hero` (141), `.cta-row` (150), `.chain-panel` (154), `.chain-head` (158),
`.chain-body` (161), `.chain` (162, `repeat(6, 1fr)`), `.rail` (165) / `.rail .fill` (166), `.stage` (168),
`.node` (169) / `.core` (170), `.chain-live` (191, row-flex ≥621px), `.audit-feed` (194/197), `.spark-wrap`
(195, 180px fixed), `.feed-row` (198), `.readings` (208, 4→2 col), `.reading` (209), `section` (215),
`.sec-head` (216), `.grid` (222, 3→2→1 col), `.card` (225) / `.legend` (230), `.feature` (240, `1fr 1fr`) and
`.feature.rev` (241, RTL flip), `.viz` (248) / `.viz-head` (252) / `.viz-body` (253), `.member` (254) / `.mid`
(255), `.verdict` (261) / `.vh` (262), `.stack` (266, 3→1 col), `.oss` (268), `.band` (274) / `.band .grid2`
(276, 3→1 col), `.claim` (278), `.final` (283), `.foot` (290) / `.foot .links` (292), `.reveal` (296).
JS-applied state classes absent from markup: `.in`, `.active`, `.hold`, `.deny`, `.new`.
`.feed-row .mono` at **199** is `font-size: 11.5px; color: var(--dim)`.

Breakpoints: `max-width: 720px` (nav hidden, stack, band grid2), `620px` (chain subs, readings, grid),
`820px` (feature), `900px` (grid), `min-width: 621px` (chain-live row).

`@keyframes` — exactly three: **`pulse`** (123), **`sweep`** (167), **`feedIn`** (201).

### 5. Reduced-motion handling (existing page)

index.html:86
```css
@media (prefers-reduced-motion: reduce) { html { scroll-behavior: auto; } }
```

index.html:179–185
```css
@media (prefers-reduced-motion: reduce) {
  .rail .fill { animation: none; right: 0; }
  .stage .node .core { transition: none !important; }
  .stage.active .node .core, .stage.hold .node .core, .stage.deny .node .core { opacity: 1; transform: scale(1); }
  .brand .dot, .chain-head .status .d { animation: none; }
  .feed-row.new { animation: none; }
}
```

index.html:298
```css
@media (prefers-reduced-motion: reduce){ .reveal { opacity: 1; transform: none; transition: none; } }
```

JS honours it at four sites, all via
`window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches`:
- **585** — drops the `new` class on feed rows.
- **700** — chain sim lights all stages once and emits one representative settled request
  `{stop:5, outcome:'ok', model:MODELS[0], org:ORGS[0], tokens:1024, cost:2.4, latency:380}` (lines 756–761),
  then `return`s before `setInterval`.
- **778** — council rotation `return`s, leaving the authored single dissent.
- **818** — operators prepend `return`s, leaving the three static rows.

`.reveal` opacity is fixed by CSS but the `IntersectionObserver` at 556–559 still runs unconditionally —
harmless.

### 6. Network requests / external hosts

**None. The page is fully self-contained.**

- `grep -n "https://"` → **NO MATCHES**.
- `grep -n "http://"` → exactly **one hit, index.html:8**, an SVG namespace inside a `data:` URI favicon, not a
  fetch:
  `<link rel="icon" href="data:image/svg+xml,%3Csvg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 16 16%22%3E%3Ccircle cx=%228%22 cy=%228%22 r=%223%22 fill=%22%235ad1c4%22/%3E%3C/svg%3E">`
  (`%235ad1c4` = `--live`).
- `grep -nE "fetch\(|XMLHttpRequest|WebSocket|EventSource|<script src|<img "` → no hits. Both fonts inlined as
  `data:font/woff2;base64`; every icon is inline `<svg>`. Only external-ish API is
  `navigator.clipboard.writeText` (563), which is local.

---

## Copy worth keeping (verbatim)

Everything in this section is either verified TRUE against source or is neutral framing. Preserve character
for character unless a TRAP or FALSE-CLAIMS entry says otherwise.

### Meta / chrome
- `<title>` (7): `AgentOS — the governed operating layer for AI agents`
- meta description (6), **first clause only** — see FALSE CLAIMS #1 for the tail:
  `AgentOS — the governed operating layer for AI agents.`
- footer mono (533): `AgentOS · the governed operating layer for AI agents`
- nav + footer links (both sets): `Platform`, `Governance`, `Multiverse`, `Open source`

### Hero
- kicker (319): `Sovereign agentic platform`
- headline (320): `The governed operating layer for AI agents.` — markup
  `<h1>The governed operating layer for <em>AI agents</em>.</h1>`, with `h1 em { font-style: normal; color: var(--live) }` (148)
- CTA buttons: `Get started` (323), `See how governance works` (324)

### Governance chain (panel)
- eyebrow (330): `Governance chain`
- stage labels/subs (336–341), **order verified correct against `server.go:500,517,524,530,564/583,799` and
  `docs/architecture.md:117-140`**:
  `Auth` / `Virtual key` · `Rate` / `Per-org limits` · `Budget` / `Atomic spend reserve` ·
  `Guardrail` / `Prompt screening, when on` · `Upstream` / `Provider or council` ·
  `Audit` / `Served calls and gate events`
  - `Prompt screening, when on` is explicitly praised by the audit as honestly hedged (`server.go:238`,
    `main.go:122-131`). Keep exactly.
  - `Served calls and gate events` — audit wanted it changed; adversarial re-check says **leave it alone**
    (see FALSE CLAIMS #4 for the full disagreement).
- caption (343): `Every request clears auth, limits, budget and screening before a provider is ever called — retries and autonomous runs included.` (`<b>` wraps `before a provider is ever called`). Adversarial re-check: **keep as written** (see FALSE CLAIMS #5).
- reading labels (359–362): `requests governed`, `spend reserved`, `p95 latency`, `audit rows` — labels are
  fine, **the values behind two of them are not** (FALSE CLAIMS #6, #7).

### Section ledes
- `#platform` (370): `One governed operating layer — from improving agents to shipping and scaling them, with the observability and evaluation to trust what they do.`
- `#governance` (422) — **verified TRUE** (`agent.py:55-66`, `SECURITY.md:10-16`, `docs/architecture.md:2-7`):
  `The runtime holds no provider credentials. Every model call is issued through the gateway on a virtual key, so it is impossible for an agent — or an autonomous loop — to route around the budgets, limits, and audit that make it accountable.`
- `#oss` (495): `AgentOS doesn't reinvent the agent loop — it governs it. The runtime is built on the frameworks your team already knows, and adds the operating layer around them.`
- Band (508–509): eyebrow `Sovereign by design`, h2 `Your agents. Your keys. Your infrastructure.` —
  **no lede paragraph** exists.

### Platform card copy (373–414) — heading / body / tag
1. `Self-improving agents` / `The reflection loop proposes prompt improvements, auto-evaluates candidate against baseline, and ships only what a human approves.` / `eval-gated · human-approved` — **TRUE** (`improve.py:1-9,131-145,155-166,167-173`).
2. `See exactly what agents do` / … / `otel · langfuse · audit` — body is FALSE, see FALSE CLAIMS #8.
3. `Score and gate performance` / … / `llm-judge · ci gate` — body OVERSTATED, see FALSE CLAIMS #9.
4. `Ship and scale in production` / … / `compose · helm · non-root` — body OVERSTATED, see FALSE CLAIMS #10.
5. `Run generated code safely` / `Agent-written Python executes in an egress-less Rust sandbox: read-only rootfs, every capability dropped, no route to the network.` / `egress-less · rlimited` — **TRUE** (`executor.rs:34-45,63-66`; `compose.yaml:136-152,202-207`; `helm/.../sandbox.yaml:24-36`; `SECURITY.md:60-67`).
6. `Agents for the whole company` / `Multi-tenant orgs, roles, and per-tenant budgets and rate limits. SSO and SCIM provisioning put every team on the same governed gateway.` / `rbac · sso · scim` — **TRUE** (`store/store.go:85-109`; `server/rbac.go`; `internal/rbac/rbac.go`; `server/oidc.go`; `server/scim.go`; `main.go:229-261`). The tag's amber `d hold` dot at 413 correctly signals SSO/SCIM are opt-in (`main.go:231`).

### Governance card copy (425–439) — id / heading / body
1. `Attributable` / `Every action has a name` / body OVERSTATED, see FALSE CLAIMS #11.
2. `Bounded` / `Nothing runs unbounded` / body OVERSTATED, see FALSE CLAIMS #12. First clause
   (`Atomic budget reservation stops a fan-out from overrunning`) is **TRUE** (`store/store.go:171-190`;
   `server/rbac.go:60-96`).
3. `Safe by default` / `Read freely, propose writes` / body FALSE, see FALSE CLAIMS #2. **Headings are fine.**

### Multiverse council — all three bullets verified TRUE (450–452)
- `Call the whole council behind one OpenAI-compatible model name.` (`server.go:89-93,564-581,867-953`)
- `Quorum and failure isolation — one bad vendor never denies the verdict.` (`fanout.py:5-7,161-176,255-257`; `council.yaml:15` quorum 3 of 5)
- `Two independent guards stop the council calling itself.` (`server.go:566-573`, stamped `:888`; `council/config.py:18-21,174-179`; `server.go:568` comment names exactly this pair)
- Panel header (456): `council/multiverse · quorum met` — **TRUE** (`server.go:564` prefix match accepts any suffix; `docs/api.md:97-106`).
- Panel answer (463): `Riyadh owes 4,200 SAR across two open invoices.` — keep.
- Member ids `alpha` / `beta` / `gamma` — illustrative, acceptable (shipped ids are `kimi, glm, qwen-max, deepseek, minimax`, `council.yaml:23,29,34,39,44`).

### Operators bullets (475–477)
- `A run that needs approval is recorded, never auto-resumed.` — **TRUE** (`operators/engine.py:5-7,84-90`; no resume route in `operators/api.py`). Keep.
- `interval · cron · webhook triggers toward stored objectives.` — **TRUE** (`operators/triggers.py`; `engine.py:33-36,114-135`; `operators/api.py:147-163`). Keep.
- `In-repo, checksummed skills the agent pulls on demand — never a registry.` — **KEEP AS IS**; the audit
  wanted it rewritten and the adversarial re-check overrode that (FALSE CLAIMS #16).
- Panel header (481): `operators · run history`. Rows: `nightly invoice report` / `cron 0 9 * * * · 4 cycles`
  and `pending orders` / `every 300s · answered: 4` — keep. The middle row is FALSE (#13).

### OSS cards (498–500)
- `deepagents` — `Long-running agents for complex, multi-step tasks — the deep profile with planning and self-summarization.` / `powers · the deep profile` — **TRUE** (`runtime/pyproject.toml:14` `deepagents>=0.6.12`; `agent.py:90-112`, comment at `:96-98`).
- `langgraph` — `Reliable agents with low-level control — the graph, checkpointing, and human-in-the-loop interrupts.` / `powers · the runtime` — **TRUE** (`pyproject.toml:19-20`; `agent.py:113,145-161`).
- `langchain` — `Any model provider, one interface — tools, MCP adapters, and the message model.` / `powers · tools & models` — **KEEP AS IS** (FALSE CLAIMS #17: audit called it overstated, re-check overrode).

### Band claims (512–514), each prefixed `✳`
- `Zero external services` heading — keep; body needs edit (#14).
- `Runs air-gapped` / `Local models mean a full five-model council answers at $0 with no internet at all. Bring frontier keys when you want them.` — **TRUE** (`council.yaml:14,50-76` — five enabled local members `ollama/qwen3.6:latest`, `qwen3.5:latest`, `gemma4:31b`, `gemma4:latest`, `gemma3:latest`, judge `ollama/qwen3.6:latest`; `README.md:156-157` "`make smoke9` runs a real five-model council against the seeded ERP database at $0"). Unstated prerequisite: Ollama is **not** a compose service (`compose.yaml:32`, `.env.example:16-17`); `README.md:159-165` notes `gemma3` is flaky.
- `Credentials never leave` / `Provider keys live in the gateway behind a pluggable secrets backend — env, encrypted file, or Vault — and the runtime never holds one.` — **TRUE** (`secret.go:185-202`, error text at `:200` *"must be env, file, age, or vault"*; `SECURITY.md:10-16`). Minor imprecision: the page's "encrypted file" maps to `age`; the plain **`file`** backend is omitted and is *not* encrypted. Optional tightening: `— env, file, age-encrypted file, or Vault —`.

---

## FALSE OR OVERSTATED CLAIMS (must not survive the rebuild)

Each entry: verbatim page text → verdict → evidence → exact replacement wording. Where the claim audit and the
adversarial re-check disagree, both positions are given with an explicit "trust" call.

---

### #1 — meta description tail
**Verbatim (line 6):** `AgentOS — the governed operating layer for AI agents. Self-hosted, sovereign agents under budgets, rate limits, guardrails, and a complete audit trail.`
**Verdict:** OVERSTATED — the `complete audit trail` clause is FALSE.
**Evidence:** `SECURITY.md:28-34` — *"The audit log records exactly seven kinds … Among refusals, **only rate-limit rejections and guardrail events are audited**. A `401` auth failure, a `400` malformed request, and a `402` budget exhaustion write nothing"*. Confirmed: `gateway/internal/server/server.go:500-504` (invalid key → `writeError`, no `recordAudit`); `gateway/internal/server/rbac.go:74-96` (`admitSpend` writes 402 and returns, no audit on either branch); `server.go:584-588` (unknown provider prefix 400, no audit). Rate limits off by default: `gateway/cmd/gateway/main.go:153-164` + `deploy/compose.yaml:45` `AGENTOS_RATE_LIMIT_RPM: ${...:-0}`; `server.go:1018` `if rpm <= 0 { return false }`. Guardrails off by default: `server.go:238` `guardMode: guardrail.ModeOff`; `main.go:122-131`; `compose.yaml:34` default `off`.
**Replacement:** `AgentOS — the governed operating layer for AI agents. Self-hosted, sovereign agents under budgets, rate limits, guardrails, and an audit trail of everything served.`

---

### #2 — "Safe by default" card body  ← HIGHEST PRIORITY, SAFETY-BEARING
**Verbatim (line 438):** `An unattended agent reads on its own but turns every write into a human-approved proposal — fail-closed, so an unknown tool is a write.`
**Verdict:** FALSE as a general claim about unattended agents. **Both the audit and the adversarial re-check agree; the re-check strengthens it.**
**Evidence:** The fail-closed classifier has exactly one production call site — `runtime/src/agentos_runtime/council/fanout.py:248-251` (`if store is not None and write_class_calls(pending): await hold_writes_as_proposals(...)`), inside `_drive`. A repo-wide grep for `hold_writes_as_proposals|write_class_calls|is_read_safe` returns only `fanout.py`, `gating.py` and tests — the operator path never imports it. `council/gating.py:7-9`: *"Classification is FAIL-CLOSED. Only tools on the read-safe allowlist execute unattended…"*; `READ_SAFE_TOOLS` at `:34-42` = `{query, list_tables, describe_table, search_knowledge, run_python}`; `gating.py:11-15` scopes it to `profile: react` members. `SECURITY.md:110` table row: *"Council write gating | Fail closed — an unclassified tool is treated as write-class — **for `profile: react` members only**"*; same in `console/src/pages/docs/content.ts:257`. The operator path is the inverse: `operators/engine.py:78-80` passes `deps.approval_tools`; `api.py:225` `approval_tools=settings.approval_tool_names`; `config.py:43` `approval_tools: str = ""`; `compose.yaml:104` and `.env.example:25` both empty; `hitl.py:72` `if any(call.tool in approval_tools for call in pending)` is an allowlist membership test, so an *unknown* tool auto-resumes at `hitl.py:76`.
**Partial defence (does not save it):** all five shipped-enabled council members are `profile: react` (`runtime/council.yaml:57-76`), so the classifier does cover everything that runs out of the box; the disabled frontier members (`:23-48`) are bounded by hand-written read-only `tools:` lists instead. The defect is purely the missing scope word on a page that separately sells unattended operators.
**Replacement:** `An unattended council member reads on its own but turns every write into a human-approved proposal — fail-closed, so an unknown tool is a write.`
If the card must also cover operators, add the operator truth rather than implying it: operators pause only on tools named in `AGENTOS_APPROVAL_TOOLS`, and a run that hits that interrupt is recorded `needs_approval` and never auto-resumed (`engine.py:84-90`).

---

### #3 — Operators lede, third clause  ← SAFETY-BEARING
**Verbatim (line 473):** `Standing objectives the runtime pursues on its own: fire on an interval, a cron schedule, or an inbound webhook. Every run is governed, bounded by a cycle cap, and a write pauses for a human. The scheduler is opt-in and off by default.`
**Verdict:** OVERSTATED overall; the clause `a write pauses for a human` is FALSE by default. **Both surveys agree; the re-check makes it stronger.**
**Evidence:**
- Triggers TRUE (`operators/triggers.py`; `engine.py:33-36,114-135`; `api.py:147-163`). "Every run is governed" TRUE (`engine.py:3-7`: *"going through the gateway like every other run — so budgets, rate limits, guardrails, and the audit trail all apply unchanged"*). Cycle cap TRUE (`engine.py:64`, `:76` `"recursion_limit": 2 * max_cycles + 1`). "opt-in and off by default" TRUE (`config.py:68` `autonomy_enabled: bool = False`; `compose.yaml:116-118`; `.env.example:31-35`).
- **The audit said the pause happens but auto-resumes. The re-check found worse: with the shipped default no interrupt is compiled at all.** `runtime/src/agentos_runtime/agent.py:113` — `interrupt_before = ["tools"] if settings.approval_tool_names else None`. Default empty (`config.py:43`, `compose.yaml:104`). No write ever reaches a decision point.
- The repo's own docs say the opposite of the page: `SECURITY.md:244-246` — *"**HITL is off by default.** `AGENTOS_APPROVAL_TOOLS` is empty … tools run unattended unless you opt in."* Also `docs/project-context.md:525`, `docs/concepts.md:68`.
- Extra nail the audit missed: on `AGENTOS_AGENT_PROFILE=deep` there is no HITL at any setting — `agent.py:86-88`, `SECURITY.md:238-243` (*"The deep agent profile has no human-in-the-loop … exposes no `interrupt_before` pass-through"*).
**Replacement (re-check wording — trust this one):** `Standing objectives the runtime pursues on its own: fire on an interval, a cron schedule, or an inbound webhook. Every run is governed and bounded by a cycle cap, and any tool you put on the approval list pauses for a human. The scheduler is opt-in and off by default.`
(The audit's variant named the env var explicitly: `…and any tool you list in AGENTOS_APPROVAL_TOOLS pauses for a human.` Either is accurate; the re-check's reads better in body copy.) **Do NOT keep any wording implying writes are classified or held automatically.**

---

### #4 — Audit stage sublabel  ← SURVEYS DISAGREE
**Verbatim (line 341):** `Served calls and gate events`
**Audit verdict:** OVERSTATED — budget denial is a gate event and is not recorded (`rbac.go:74-96`; `SECURITY.md:28-34`; `docs/architecture.md:135-140`: *"a `401` auth failure and a `402` budget exhaustion do not"*). Proposed replacement: `Served calls, rate-limit and guardrail events`.
**Adversarial re-check verdict:** AUDITOR OVERREACHED — `rate_limited` and `guardrail_flag/block/error` **are** gate events and **are** audited (`store/store.go:55-69`), and a five-word stage sublabel asserts no universal quantifier. **Leave it alone.**
**Which to trust:** the **re-check**. Keep `Served calls and gate events` verbatim. (If the rebuild wants belt-and-braces, the audit's replacement is harmless and also true — but it is not required, and swapping it costs a keeper-string.)

---

### #5 — Chain caption  ← SURVEYS DISAGREE
**Verbatim (line 343):** `Every request clears auth, limits, budget and screening before a provider is ever called — retries and autonomous runs included.`
**Audit verdict:** OVERSTATED — "clears … limits and screening" reads as active controls; both are no-ops in shipped defaults, so a request "clears" screening that never ran. Proposed replacement: `Every request clears auth, budget, and any limits and screening you have enabled <b>before a provider is ever called</b> — retries and autonomous runs included.`
**Adversarial re-check verdict:** AUDITOR OVERREACH — this is an **ordering** claim the code satisfies (`server.go:500,517,524,530` all precede `:601`). Do NOT rewrite.
**Component verifications (both surveys agree these three are TRUE):** "before a provider is ever called" — all four checks precede `s.proxy` (`server.go:601`) and the council hop (`:564`). "retries … included" — `server.go:690-694` (*"Retries sit AFTER the auth/guardrail/rate-limit/budget checks in the handlers, so they cannot bypass governance."*). "autonomous runs included" — `agent.py:62-66`; `operators/engine.py:1-11`; `council/fanout.py:70`.
**Which to trust:** the **re-check** — keep the caption verbatim. If the rebuild is being maximally conservative about defaults elsewhere on the page, the audit's replacement is a safe optional hedge, not a required fix.

---

### #6 — "Live" status badge over a seeded simulation
**Verbatim (line 331):** `Live` (inside `.chain-head .status`, alongside eyebrow `Governance chain` at 330)
**Verdict:** FALSE.
**Evidence:** The page's own comment, lines 693–694: `// --- Governance-chain live preview: a deterministic in-file simulation.` / `// No Math.random / no network — the demo sequence is reproducible.` Animation is `setInterval(tick, 520)` (762) over seeded `mulberry32(0x5EED)` (710). No viewer-visible label distinguishes it from real telemetry.
**Replacement:** change the status text to `Simulated`, i.e. `<span class="status">…Simulated</span>`; or move the honesty into the eyebrow: `Governance chain — illustrative`. Either is acceptable; **one of them must ship.**

---

### #7 — Audit feed rows and the "audit rows" tile count what the gateway never records
**Verbatim (lines 346, 359–362 markup; 605, 617–633, 721–723 script):** the `#audit-feed` rows and the `audit rows` tile present every simulated request — including denials — as an audit entry carrying a token count and a per-request dollar cost.
**Verdict:** CONFIRMED FALSE. The re-check explicitly could not refute it and found the defect **wider than the audit describes**.
**Evidence, sim side:** `:605` `auditRows += 1;` — unconditional, inside `updateReadings(r)`, which runs for every settled request; feeds `:613` → `:362` `<div class="n" id="rd-audit">`. `:723` `else { stop = 1 + Math.floor(rand() * 3); outcome = 'deny'; }` over `(Auth,Rate,Budget,Guardrail,Upstream,Audit)` → deny lands on stage 1/2/3 = Rate / Budget / Guardrail. `:623` `text.textContent = r.model + ' · ' + r.org + ' · ' + r.tokens + ' tok · ' + fmtMoney(r.cost);` — `prependFeedRow` runs for EVERY settled request, so deny and hold rows print a token count (`:728`, 200–2000) and a dollar cost (`:729`, $0.40–$6.40). `:606` `if (r.outcome === 'ok') spend += r.cost;` — the page already knows a denied request costs nothing, yet still prints a cost on its feed row.
**Evidence, gateway side:** 402 budget denial writes **nothing** — `rbac.go:74-96`, both `ErrBudgetExceeded` and `ErrOrgBudgetExceeded` branches call `writeError` and `return nil, false` with no `recordAudit`; corroborated `SECURITY.md:28-34` and the `AuditEntry`/kind list at `store/store.go:54-69,129-140`. The two deny kinds that ARE audited carry zero tokens and zero cost — `server.go:1024-1027` (`KindRateLimited`) and `server.go:530-556` (`KindGuardrailError/Flag/Block`), with `Usage.InputTokens/OutputTokens/CostUSD` left at zero. The `hold` (upstream failure) case likewise records `CostUSD` 0 — `server.go:780-786`. Net: of the six outcome classes the sim emits, **every non-`ok` one renders a token count and a dollar figure the gateway would record as 0**, and one (Budget) produces a counted "audit row" the gateway never writes at all (~2.7% of settled requests: 8% deny × 1/3).
**Audit's proposed fix — DO NOT SHIP:** `if (r.outcome !== 'deny' || r.stop !== 2) auditRows += 1;` only patches the counter for the budget stage and leaves every Rate (429), Guardrail (400) and Upstream (502) feed row still displaying fabricated tokens and dollars.
**Replacement (re-check — trust this; two changes, both in `landing/index.html`):**
1. `:605` → `if (!(r.outcome === 'deny' && r.stop === 2)) auditRows += 1;` (Rate=1 → `rate_limited`; Guardrail=3 → `guardrail_block`; Upstream hold and ok → `s.record`; Budget=2 → nothing).
2. `:623` → emit `model · org · tokens · $cost` **only** when `r.outcome === 'ok'`; for `deny`/`hold` emit `model · org · 0 tok · $0.00` (or drop the two fields entirely). Mirrors `server.go:1024-1027` / `:530-556` / `:780-786` and the page's own `:606` logic.

---

### #8 — Readings strip values  ← SURVEYS PARTLY DISAGREE
**Verbatim (lines 359–362 labels):** `requests governed`, `spend reserved`, `p95 latency`, `audit rows`
**Audit verdict:** FALSE — fabricated numbers presented as measurements. `cost: Math.round((0.4 + rand() * 6) * 100) / 100` (`:729`) → $0.40–$6.40 per request feeding a tile labelled `spend reserved`, against the platform's real per-request hold `DefaultReserveUSD = 0.05` (`gateway/internal/server/server.go:112`) — "8–128× smaller". `latency: 120 + Math.floor(rand() * 900)` (`:730`) → the `p95 latency` tile; no measurement anywhere in the repo produces this. `tokens: 200 + Math.floor(rand() * 1800)` (`:728`) feeds the feed rows.
**Adversarial re-check verdict:** the `spend reserved` tile is a **cumulative sum** (`spend += r.cost`, `:606`), never a per-request figure, so the "8–128× vs `DefaultReserveUSD = 0.05`" framing **overstates the mismatch**. The real problem is the unlabelled simulation (#6), not the arithmetic.
**Which to trust:** the **re-check** on the arithmetic framing — do not print "8–128×" anywhere. **Both agree** the strip must be visibly marked as simulated.
**Replacement:** add a `<span class="eyebrow">Simulated</span>` header on `.readings` (or fold it into the #6 badge change so the whole panel reads as illustrative). Dropping `spend reserved` and `p95 latency` — the two tiles that name real, differently-valued platform quantities — is the stronger option and is acceptable. Fix in the **same pass** as #7 or the panel stays internally inconsistent.

---

### #9 — Observability card body
**Verbatim (line 384):** `OpenTelemetry traces, a bundled Langfuse profile, and a complete per-key audit log — every model call, tool call, and verdict, attributable.`
**Verdict:** FALSE.
**Evidence:** OTel TRUE but opt-in (`gateway/cmd/gateway/main.go:263-275` `AGENTOS_OTEL_ENDPOINT`; `runtime/.../config.py:47`). Bundled Langfuse TRUE (`deploy/compose.langfuse.yaml`, Langfuse OSS v2 + Postgres). "**complete** per-key audit log" FALSE (`SECURITY.md:28-34`, as #1). "every **model call**" TRUE for served calls (`server.go:799-810`). "every **tool call**" FALSE — the audit schema has no tool field: `store/store.go:130-140` `AuditEntry{TS, KeyName, Model, InputTokens, OutputTokens, CostUSD, LatencyMS, Status, Kind}`; the seven kinds at `store/store.go:55-69` are `chat, embeddings, guardrail_flag, guardrail_block, guardrail_error, rate_limited, secret_reload`. Tool calls live only in LangGraph checkpoint messages, the runtime's `steps` response field (`runtime/.../api.py:300-310`), operator run rows (`operators/engine.py:82`), and — only with tracing on — OTel spans (`api.py:302` `record_tool_spans`). "every **verdict**" FALSE for the audit log — verdicts persist in the runtime's council tables (`council/loop.py:102-113` `insert_cycle`); the gateway audits the council call as `Kind: store.KindChat` with `CostUSD` omitted entirely (`server.go:947-951`, comment *"CostUSD stays 0: members were billed on their own keys"*).
**Replacement:** `OpenTelemetry traces, a bundled Langfuse profile, and a per-key audit log of every served model call — plus tool steps and council verdicts recorded in the runtime, all attributable.`

---

### #10 — Evaluation card body
**Verbatim (line 391):** `Eval suites with an LLM judge measure agents against real cases, and a CI gate blocks a regression before it ships.`
**Verdict:** OVERSTATED.
**Evidence:** LLM judge TRUE (`runtime/.../evals.py:9-15,139-159` `judge_case`; `AGENTOS_JUDGE_MODEL` at `config.py:49`). The gate claim fails on three counts: (1) opt-in, not a merge gate — `.github/workflows/evals.yml:8-9` *"Triggers: manual (workflow_dispatch) and pull requests labelled `run-evals` (evals are heavier than unit CI, so they are opt-in per PR)"*, confirmed `:58-61` `contains(github.event.pull_request.labels.*.name, 'run-evals')`; (2) gates on an **absolute threshold**, not a baseline, so it does not detect a regression as such — `:150-152` `if score < threshold: … sys.exit(1)`; (3) runs against a mock model — `:3-5` *"Boots the full compose stack against a DETERMINISTIC, OFFLINE mock model (deploy/ci/mock-model.py)"*, job name `:56` is `eval gate (mock model)`.
**Replacement:** `Eval suites with an LLM judge measure agents against real cases, and an opt-in CI gate fails a build that scores below your threshold.`
**Do not source replacement copy from the README:** `README.md:272` makes the same overstatement (*"CI runs the … matrix on every push plus an eval gate"*) and is also wrong.

---

### #11 — Deployment card body
**Verbatim (line 398):** `Docker Compose for a laptop, a Helm chart for a cluster — non-root images, dropped capabilities, and a hardened security context throughout.`
**Verdict:** OVERSTATED — `throughout` is false. (Re-check independently confirmed the browser Dockerfile.)
**Evidence:** Compose + Helm exist (`deploy/compose.yaml`, `deploy/helm/agentos/`). "non-root images … throughout" FALSE — `connectors/browser/Dockerfile` has **no `USER` directive** (its only directives are `FROM mcr.microsoft.com/playwright/python:v1.61.0-noble`, `COPY`, `ENV`, `WORKDIR`, `RUN`, `EXPOSE`, `CMD`). Every other image is non-root: gateway/sql/rest/soap/ssh/demo-crm on `gcr.io/distroless/static-debian12:nonroot`; `runtime/Dockerfile:29` `USER 10002:10002`; `sandbox/Dockerfile:17` `USER sandbox`; `console/Dockerfile:25` `USER 101:101`. "dropped capabilities … throughout" FALSE for Compose — only `sandbox` sets them (`compose.yaml:136-144`: `read_only: true`, `cap_drop: [ALL]`, `no-new-privileges:true`); `gateway`, `runtime`, `console`, `postgres` and every connector carry no `cap_drop`, `read_only` or `security_opt`. Helm is closer but incomplete: `securityContext` blocks exist in `templates/{gateway,runtime,sandbox,console,sql-connector,rest-connector,demo-crm}.yaml`; **`templates/postgres.yaml` has none**, and SOAP, browser and SSH have no Helm template at all (`docs/architecture.md:96-98`).
**Replacement:** `Docker Compose for a laptop, a Helm chart for a cluster — non-root images and, in Helm, a hardened security context on every service; the sandbox drops all capabilities on a read-only rootfs in both.`

---

### #12 — "Attributable" card body  ← SURVEYS PARTLY DISAGREE
**Verbatim (line 428):** `Virtual keys, orgs, and users mean each request is traced to who made it and what it cost — never an anonymous provider call.`
**Audit verdict:** OVERSTATED. "orgs" and "what it cost" TRUE (`store/store.go:112-127` `Usage.OrgID`, `.CostUSD`; `server.go:799-810`). "**users** … traced to **who** made it" FALSE — the audit row carries no user identity: `store/store.go:130-140` `AuditEntry{TS, KeyName, Model, InputTokens, OutputTokens, CostUSD, LatencyMS, Status, Kind}`; `Usage` (`:112-127`) has only `SecretHash`, `OrgID`, `KeyName`. A user id is captured at key *creation* (`store.CreateKeyIn(..., createdBy)`, `server.go:420`) but never propagated to a request record. Council calls record `CostUSD` implicitly zero (`server.go:947-951`), so "what it cost" is $0.00 on that row by design.
**Adversarial re-check verdict:** "partially true — 'who made it' is **transitively** supported by `keys.created_by`" (`gateway/internal/store/postgres.go:68,165`).
**Which to trust:** the **audit's core factual point stands** (no user id on the request record); the re-check only softens the severity to "partially true / transitive". Ship the replacement — a transitive join is not what "each request is traced to who made it" implies to a reader.
**Replacement:** `Virtual keys and orgs mean each request is traced to the key and tenant that made it and what it cost — never an anonymous provider call.`

---

### #13 — "Bounded" card body, kill-switch clause
**Verbatim (line 433):** `Atomic budget reservation stops a fan-out from overrunning; cycle caps and a kill switch bound every autonomous run.`
**Verdict:** OVERSTATED — `a kill switch … every` is FALSE.
**Evidence:** Atomic reservation TRUE (`store/store.go:171-190` `ReserveSpend`, *"N concurrent requests on one key all observe the same pre-spend snapshot … A council fanning out to five members is exactly that shape"*; `server/rbac.go:60-96`). Cycle caps TRUE on both autonomy paths (council `council/config.py:81-82` `max_cycles`, `max_tool_steps` → `fanout.py:238-241` `recursion_limit`; operators `engine.py:64,76`). The kill switch is **council-only**: `council/store.py:360-377` (`is_paused`/`set_paused` on `council_state`), `council/api.py:128-137` (`POST /council/pause` / `/resume`), checked at `council/loop.py:138-142,184`. **Operators have no equivalent** — `operators/api.py` (163 lines, read in full) exposes no pause/resume route; the only controls are the per-operator `enabled` flag (`operators/api.py:33,115-122`) and the process-level `AGENTOS_AUTONOMY_ENABLED` env var, which requires a restart (`config.py:68`, `api.py:229-234`).
**Replacement:** `Atomic budget reservation stops a fan-out from overrunning; every autonomous run is bounded by a cycle cap, and the council carries a kill switch.`

---

### #14 — Council lede, "frontier model"
**Verbatim (line 448):** `Route one objective to a council of model-bound agents, each on a different frontier model. A judge synthesizes their answers into a single verdict — and reports where they disagreed, instead of averaging it away.`
**Verdict:** FALSE as shipped (first sentence). Second sentence is fine.
**Evidence:** `runtime/council.yaml:3-6` — *"The five frontier members below are **DISABLED** … The enabled members are local Ollama models."* Confirmed `:23-48` (all five frontier entries `enabled: false`) vs `:57-76` (five `ollama/*` members, `enabled: true`). `deploy/providers.json:2` marks every frontier endpoint/price a *"PLACEHOLDER … NOT verified against vendor documentation"*; `:8,15,21,29,36` set `"enabled": false`. Corroborated `README.md:208-212`.
**Replacement:** `Route one objective to a council of model-bound agents, each on a different model — five local models out of the box, frontier models when you enable them. A judge synthesizes their answers into a single verdict — and reports where they disagreed, instead of averaging it away.`

---

### #15 — Council verdict pill
**Verbatim (line 462):** `unanimous · 0.67` — rendered directly beneath a member row whose pill reads `dissent` (line 460).
**Verdict:** FALSE, on two independent counts. (Re-check confirmed both.)
**Evidence:** (1) Self-contradictory — `council/judge.py:50`: *"`dissent` lists every member that materially disagreed. **Empty when unanimous.**"* A panel showing a dissenting member cannot show a unanimous verdict. (2) `unanimous` is not a status the platform emits — the only verdict statuses are `STATUS_OK = "ok"` (`judge.py:26`) and `JUDGE_UNAVAILABLE = "judge_unavailable"` (`:25`); `Verdict.status` defaults to `STATUS_OK` (`:65`). What `0.67` actually is: `agreement`, a float in [0,1] clamped at `judge.py:121`, compared against `agreement_threshold: 0.6` (`runtime/council.yaml:16`) at `council/loop.py:68` to yield `STOP_CONVERGED`.
**Replacement:** `<span class="pill ok">converged · agreement 0.67</span>`
**Related:** the rotation script's comment at 769–770 asserts *"At most one member ever dissents per tick, so the panel's 'quorum met' framing always stays true"* — true of the framing, but it does nothing about the `unanimous` string, which is hardcoded in the markup and never rewritten by `setPill` (791–795).

---

### #16 — Council viz member model id
**Verbatim (line 459):** `deepseek/deepseek-v4`
**Verdict:** FALSE — the id does not exist.
**Evidence:** `runtime/council.yaml:40` is `model: deepseek/deepseek-v4-pro`; `deploy/providers.json:30` prices `"deepseek-v4-pro"`. `moonshot/kimi-k3` (458) and `alibaba/qwen3.8-max` (460) do match `council.yaml:24` and `:35` — but all three are `enabled: false` members with placeholder pricing.
**Audit's rationale, corrected by the re-check:** the audit said routing `deepseek/deepseek-v4` "would 400". The re-check refuted the *rationale*: `provider.go:130-156` routes any model id under an **enabled** provider at $0, and under the shipped disabled default the correct id `deepseek-v4-pro` 400s identically. **The verdict stands (wrong id); do not print the "would 400" justification.**
**Replacement:** change line 459's `<div class="b">` to `deepseek/deepseek-v4-pro`.

---

### #17 — Operators viz, held tool name (two sites)
**Verbatim (line 484):** `webhook · sql.write held`  ·  **Verbatim (line 837, JS `RUNS` roster):** `webhook · fs.write held`
**Verdict:** FALSE — neither `sql.write` nor `fs.write` exists in the platform. (Re-check confirmed.)
**Evidence:** The SQL connector registers exactly three tools, all read-only — `connectors/sql/internal/tools/tools.go:51-65`: `list_tables`, `describe_table`, `query` (confirmed `connectors/sql/cmd/sql-connector/main.go:2`). There is no filesystem connector. The only occurrence of the string `sql.write` in the repo is a made-up name inside a unit-test fixture (`runtime/tests/test_council_store.py:78`).
**Replacement:** use a tool that exists and can be listed for approval — the re-check's concrete suggestion is `query held`. Acceptable alternatives: a REST connector mutation opened via `AGENTOS_REST_ALLOW_MUTATIONS`, or the generic label `write-class tool held`. **Both line 484 and line 837 must change.**

---

### #18 — CTA note under the hero buttons
**Verbatim (line 325):** `docker compose up · runs air-gapped`
**Verdict:** FALSE as written.
**Evidence:** `README.md:141-146` states the opposite in the repo's own words: *"A bare `make up` is **not** the $0 path: the shipped defaults point `AGENTOS_MODEL` and `AGENTOS_JUDGE_MODEL` at Anthropic, and even the already-local `AGENTOS_EMBED_MODEL` default cannot resolve without an Ollama base URL. Set all four in `deploy/.env` before starting."* Source: `deploy/compose.yaml:100` `AGENTOS_MODEL: ${AGENTOS_MODEL:-anthropic/claude-sonnet-5}`; `:107` `AGENTOS_JUDGE_MODEL: ${...:-anthropic/claude-haiku-4-5}`; `:32` `AGENTOS_OLLAMA_BASE_URL: ${AGENTOS_OLLAMA_BASE_URL:-}` (empty); `runtime/.../config.py:40` `model: str = "anthropic/claude-sonnet-5"`. There is **no Ollama service in `deploy/compose.yaml`** — the air-gapped path needs an Ollama the operator runs on the host.
**Replacement:** `docker compose up · air-gapped with local models`

---

### #19 — Band claim 1, second sentence
**Verbatim (line 512):** `Zero external services` / `The whole platform runs on your own hardware — gateway, runtime, sandbox, database. No third party sees your prompts or usage.`
**Verdict:** OVERSTATED — second sentence FALSE under shipped defaults.
**Evidence:** The four named components are all self-hosted (`deploy/compose.yaml`: `postgres`, `gateway`, `runtime`, `sandbox`). But `compose.yaml:100` routes the agent to `anthropic/claude-sonnet-5` out of the box; `README.md:143-146` says so explicitly.
**Replacement:** `The whole platform runs on your own hardware — gateway, runtime, sandbox, database. Point it at local models and no third party sees a prompt.`

---

### #20 — Final CTA lede
**Verbatim (line 523):** `Clone the repo, docker compose up, and open the console. Everything on this page runs locally, at zero cost, on your own machine.` (`docker compose up` is wrapped in `<span class="mono" style="color:var(--ink)">`)
**Verdict:** FALSE, on two counts. (Re-check confirmed the compose default.)
**Evidence:** (1) "at zero cost" contradicts `README.md:141-146` — the shipped default is a paid Anthropic model. (2) "Everything on this page" is false regardless of model choice. Off or disabled in shipped defaults: guardrails (`compose.yaml:34` `off`), rate limits (`:45` `0`), operators scheduler (`:118` `false`), council heartbeat (`:114` `0`), SSO (`:54` empty issuer), SCIM (`:50` empty token), OTel (`main.go:263`), Langfuse (separate overlay), audit retention (`main.go:170`), and all five frontier council members (`runtime/council.yaml:25,31,36,41,46`).
**Replacement:** `Clone the repo, <span class="mono">docker compose up</span>, and open the console. Point it at a local Ollama and the whole stack — including the five-model council — runs on your machine at $0.`

---

### #21 — Copy-to-clipboard command
**Verbatim (line 563):** `'git clone && cd agentos/deploy && docker compose up -d'` — while the button label (line 525) reads `docker compose up -d`.
**Verdict:** FALSE — not a runnable command. (Re-check: "genuinely broken but cosmetic".)
**Evidence:** `git clone` with no repository argument exits non-zero, so the `&&` chain never proceeds. The README quickstart is different again (`README.md:112-118`: `cd deploy; cp .env.example .env; cd ..; make up`) and also requires `cp .env.example .env` first (`deploy/.env.example:1`).
**Replacement:** `'git clone <repo-url> agentos && cd agentos/deploy && cp .env.example .env && docker compose up -d'`

---

### #22 — Hero lead
**Verbatim (line 321):** `Model-agnostic agents that run under budgets, rate limits, guardrails, and a complete audit trail — self-hosted, open, and yours. Nothing runs unauthorized, unattributed, or unrecorded.`
**Audit verdict:** OVERSTATED on three grounds — (a) "complete audit trail", same evidence as #1, **FALSE**; (b) "Nothing runs unauthorized, unattributed, or unrecorded" defensible only with the qualifier the README attaches to the identical sentence and the landing page drops — `README.md:46-49`: *"That sentence is a claim about actions the platform *executes* … Coverage on the **refusal** side is deliberately partial and documented as such."*; (c) "rate limits, guardrails" read as active controls, both off in shipped defaults.
**Adversarial re-check:** the audit **overreached on (b)** — `README.md:46-49`'s qualifier *explains* that sentence rather than restricting it, and "runs" already means "executes"; the proposed rewrite adds words without adding accuracy.
**Which to trust:** fix (a) — non-negotiable, both surveys agree "complete audit trail" is false. Treat (b) as **settled in the page's favour**: keep `Nothing runs unauthorized, unattributed, or unrecorded.` unchanged. (c) is worth the hedge.
**Replacement (recommended, merging both positions):** `Model-agnostic agents that run under budgets, optional rate limits and guardrails, and an audit record of every served call — self-hosted, open, and yours. Nothing runs unauthorized, unattributed, or unrecorded.`
(The audit's fuller variant ended `Nothing the platform executes runs unauthorized, unattributed, or unrecorded.` — accurate but, per the re-check, unnecessary.)

---

### #23 — Two audit findings the re-check says NOT to "fix"
Listed here so the rebuild does not regress into a false correction.

- **`In-repo, checksummed skills the agent pulls on demand — never a registry.` (line 477).** Audit called `checksummed` OVERSTATED (a sha256 is computed at `skills.py:43`, stored `:66`, logged `:88`, but never compared; `README.md:205-206` *"each recording a sha256 for provenance"*; `docs/superpowers/specs/2026-07-27-platform-docs-and-console-docs-tab.md:54` *"each loaded skill records a sha256 **for provenance** — there is no comparison against a pinned expected digest"*). **Re-check: do NOT ship the audit's rewrite — `skills.py:43,66,88` do checksum them, and there is nothing to verify against since they are image-baked. A rewrite would be a regression.** Trust the re-check: **keep line 477 verbatim.** (In-repo / never-a-registry / pulls-on-demand are all independently TRUE: `skills.py:5-8,70-89,107-123`.)
- **`Any model provider, one interface — tools, MCP adapters, and the message model.` (line 500).** Audit called it marginally OVERSTATED (the runtime's only model client is one `ChatOpenAI` pointed at the gateway, `agent.py:62-66`; provider-agnosticism comes from `gateway/internal/provider/`). **Re-check: overreach — the sentence describes LangChain the upstream project, not AgentOS's wiring.** Trust the re-check: **keep line 500 verbatim.** (The audit's suggested `One interface to the gateway — tools, MCP adapters, and the message model.` is available if the rebuild wants it, but it is not required.)

---

### Priority order for the rebuild
1. #2 (line 438) — fail-closed write gating attributed to "an unattended agent"; council/react-only, operator path is fail-open and off by default. **Safety-bearing.**
2. #3 (line 473) — "a write pauses for a human" on a path where no interrupt is even compiled by default. **Safety-bearing.**
3. #7 + #8 + #9 (lines 605/617/623, 359–362, 384) — budget denials rendered and counted as audit rows with fabricated tokens/dollars; "complete per-key audit log"; "every tool call, and verdict".
4. #18 + #20 (lines 325 / 523) — "runs air-gapped" / "zero cost" on a bare `docker compose up`, contradicted by `README.md:141-146`.
5. #14 (line 448) — "each on a different frontier model"; all five frontier members ship `enabled: false`.
6. #6 (line 331) — a "Live" badge over a seeded simulation.
7. #13 (433) kill switch "every autonomous run"; #10 (391) CI gate "blocks a regression"; #11 (398) hardening "throughout"; #12 (428) "users … who made it".
8. #16, #17, #21 (lines 459, 484, 837, 563) — a model id, two tool names, and a clipboard command that do not exist / do not run.

---

## Mock code assessment

Source: `page.src.html` (383 lines). The built file `agentos-landing-mock.html` is **byte-identical except**
that the single line `@@FONTS@@` (page.src.html:3) is replaced by 4 `@font-face` blocks (1 × `"Archivo"`,
3 × `"IBM Plex Mono"`). Diffed; no other difference. All line numbers below are `page.src.html`.

### CSS custom properties (one `:root`, lines 4–12; no second `:root`, no `[data-theme]`, no `prefers-color-scheme` anywhere)

| Property | Value | Line | Used? |
|---|---|---|---|
| `--bg` | `#07080B` | 5 | yes |
| `--raised` | `#0E1015` | 5 | yes (69, 98) |
| `--raised2` | `#151821` | 5 | **never referenced** (0 `var(--raised2)`) |
| `--line` | `rgba(255,255,255,.06)` | 6 | yes |
| `--line2` | `rgba(255,255,255,.12)` | 6 | yes |
| `--ink` | `#F2F4F7` | 7 | yes |
| `--dim` | `#98A2AE` | 7 | yes |
| `--faint` | `#646C77` | 7 | yes |
| `--v` | `#7A5AF8` | 8 | yes |
| `--v2` | `#A78BFA` | 8 | yes (7 uses) |
| `--v3` | `#C7B6FF` | 8 | yes (2 uses) |
| `--vdim` | `rgba(122,90,248,.14)` | 8 | **never referenced** |
| `--live` | `#5ad1c4` | 9 | **never referenced** |
| `--ok` | `#6cc48f` | 9 | yes (1 use, `pre .o` line 109) |
| `--hold` | `#e3a851` | 9 | yes (3 uses) |
| `--deny` | `#e2685f` | 9 | yes (3 uses) |
| `--sans` | `"Archivo",system-ui,sans-serif` | 10 | yes |
| `--mono` | `"IBM Plex Mono",ui-monospace,monospace` | 10 | yes |
| `--ease` | `cubic-bezier(.2,.8,.2,1)` | 11 | yes (11 uses) |

Values that match the existing page **exactly**: `--live #5ad1c4`, `--ok #6cc48f`, `--hold #e3a851`,
`--deny #e2685f`. Names that differ: mock `--raised2` vs existing `--raised-2`; mock `--line`/`--line2` vs
existing `--border`/`--border-strong`.

### Determinism

`grep -n "Math.random" page.src.html` → **no matches (exit 1)**. `grep -c` on the built mock → `0`. Also **no
`Date.now`, no `performance.now`, no `new Date`**. **There is no PRNG in the file at all.** The hero `.meta`
string `SEED <b>3573860127</b>` (line 136) is cosmetic text — nothing reads it.

### IIFE 1 — Hero gate field, lines 242–306

Draws into `<svg class="field" id="f" viewBox="0 0 1440 1000">` (134): a radial halo at the gate, 128
index-driven bezier strands flowing left→right, 9 cut short and terminated with a red dot (denials), 8 drawn
"hot" with a travelling packet dash, plus a 6-tick gate column with per-stage `<text>` labels and one vertical
pulsing bar.

- `var W=1440,H=1000,GX=980,MY=520,N=128,SPREAD=430;` (244). **`H` is declared and never used.**
- `STAGES=["auth","rate","budget","guardrail","upstream","audit"]` (245).
- Halo: `cx=GX, cy=MY, r=190`, `fill="url(#halo)"` (261–262).
- Per-strand: `t=i/(N-1)`, `e=Math.sin((t-.5)*Math.PI)`, `y0=MY+e*SPREAD`, `m=i%29`, `dens=1-Math.abs(t-.5)*2` (267).
- Denials: `if(m===7||m===19)` → i ∈ {7,36,65,94,123} ∪ {19,48,77,106} = **9 strands**. `si=(m===7)?1:3`, `cut=GX-58+si*12` (x=934 or 958), `yc=MY+e*44`, `st=GX-330`, stroke `#e2685f`, width `1`, opacity `.34`; end dot `r=2.1`, opacity `.95` (269–272).
- Hot: `var hot=(i%17===8)` → i ∈ {8,25,42,59,76,93,110,127} = **8 strands** (no collision with denials). Stroke width `hot?1.6:.75`; opacity `hot?.95:(.07+.22*dens)` (275, 279–280).
- Inbound path controls `GX*.52`, `GX*.87`, starting `x=-40` (276). Outbound `y1=MY+e*SPREAD*.86`, controls `GX+120` and `GX+(W-GX)*.5`, ending `x=W+40` (277–278).
- Packet: `pathLength="1"`, `strokeDasharray=".035 .965"`, `animation="pkt 5.5s linear infinite"`, `animationDelay=(-(i%5)*1.1)+"s"` (5 phases), stroke `#E9E2FF`, width `1.7`, opacity `.9` (282–287).
- Gate ticks: `y=MY-66+k*26.5`; rect `x=GX-9,w=18,h=2.5,rx=1.2`, fill `k===5?"#C7B6FF":"#A78BFA"`, opacity `.95` (291–294).
- Label plate: rect `x=GX+20, y=y-8, w=84, h=16, rx=3`, fill `#07080B`, opacity `.82` (295–297). Text `x=GX+26, y=y+4.5`, fill `#6E7681`, `font-family="IBM Plex Mono, monospace"`, `font-size="9.5"`, `letter-spacing="1.6"` (298–301).
- Gate bar: `x=GX-1, y=MY-86, w=2, h=172`, fill `#7A5AF8`, opacity `.55`, `class="gbar"` (303–305).

**Deterministic: fully.** Geometry is a pure function of loop index `i` through `Math.sin`; identical DOM on
every load. Only the CSS animation *phase* varies with wall-clock start.

### IIFE 2 — Governance conveyor, lines 309–338

Drives existing markup `#chain` (170–177, six `.st` children) and caption `#cap` (178) by setting `data-o` on
the chain and `data-r` ∈ `cleared|stopped|unlit` on each stage; CSS at 79–84 colours them. The whole script is
the 8-frame `SCRIPT` array (310–319):

```
{stop:null,       o:"pass", c:"cleared every stage"}
{stop:null,       o:"pass", c:"cleared every stage"}
{stop:"rate",     o:"deny", c:"429 — over the org's rate limit"}
{stop:null,       o:"pass", c:"cleared every stage"}
{stop:"guardrail",o:"deny", c:"400 — prompt flagged by the guardrail"}
{stop:null,       o:"pass", c:"cleared every stage"}
{stop:"upstream", o:"fail", c:"502 — governance cleared, provider failed"}
{stop:"budget",   o:"deny", c:"402 — key budget exhausted"}
```

`ST=["auth","rate","budget","guardrail","upstream","audit"]` (320). Tick `setInterval(..., 2200)` ms (334),
`i=(i+1)%SCRIPT.length`. Pauses on tab-hide: `document.addEventListener("visibilitychange", ...)` (336).
**Deterministic: yes** — comment at 308 says "fixed script, no randomness" and that is accurate.

**Cross-check against the claims audit:** the `402 — key budget exhausted` frame is the exact case the gateway
does **not** audit (`rbac.go:74-96`). If the conveyor caption or any adjacent counter is ever wired to an
"audit rows" tile, this frame must not increment it — same defect as FALSE CLAIMS #7.

### IIFE 3 — Council fan-out, lines 341–373

Draws into `<svg id="cv" viewBox="0 0 1200 300">` (203): an objective box, 5 model boxes, a judge box, and
verdict/dissent boxes, joined by 12 beziers that stroke-draw in.

- `MEM=[{y:30,n:"kimi-k3"},{y:82,n:"glm-5.2"},{y:134,n:"qwen3.8-max"},{y:186,n:"deepseek-v4"},{y:238,n:"minimax-m3"}]` (343). **`deepseek-v4` is the non-existent id from FALSE CLAIMS #16 — must become `deepseek-v4-pro`.**
- Boxes: `box(0,116,150,68,"objective","one question",true)` (363); members `box(420,m.y,190,36,m.n,null,false)` (366); `box(860,116,130,68,"judge","synthesis",true)` (369); `box(1060,88,140,44,"verdict")` and `box(1060,174,140,44,"dissent")` (371–372). `rx=5`, fill `#0E1015`, stroke `#7A5AF8` if accent else `rgba(255,255,255,.12)` (346–347). Label text `x+14`, `y+h/2+(sub?-2:4)`, size `12`; sub text `y+h/2+14`, size `10`, fill `#646C77` (349–354).
- Curves: stroke `#7A5AF8`, width `1`, opacity `.5`, `pathLength="1"`, class `cdraw` (359–361). Fan-in `curve(150,150,420,m.y+18)` delay `k*90` ms; fan-out `curve(610,m.y+18,860,150)` delay `300+k*90` ms (365, 367). The two final curves `curve(990,150,1060,110)` and `curve(990,150,1060,196)` get **no** `animationDelay` (370).

**Deterministic: yes.** No randomness, no time input.

### Keyframes and animations (two `<style>` blocks: 2–121 and 375–383)

| Keyframe | Line | Body |
|---|---|---|
| `bpulse` | 30 | `0%,100%{opacity:1}50%{opacity:.45}` |
| `pkt` | 376 | `to { stroke-dashoffset:-1; }` |
| `gbar` | 378 | `0%,100%{opacity:.42}50%{opacity:.85}` |
| `cdraw` | 380 | `from { stroke-dashoffset:1 } to { stroke-dashoffset:0 }` |

| Animation | Line | Duration | Easing | Iteration |
|---|---|---|---|---|
| `.brand i{animation:bpulse 3s var(--ease) infinite}` | 29 | 3s | `cubic-bezier(.2,.8,.2,1)` | infinite (2 elements: nav 124, footer 227) |
| `pk.style.animation="pkt 5.5s linear infinite"` | 285 | 5.5s | `linear` | infinite × 8, delays `0/-1.1/-2.2/-3.3/-4.4s` |
| `.gbar{animation:gbar 3.4s var(--ease) infinite}` | 379 | 3.4s | `cubic-bezier(.2,.8,.2,1)` | infinite (1 element) |
| `.cdraw{… animation:cdraw 1.1s var(--ease) forwards}` | 381 | 1.1s | `cubic-bezier(.2,.8,.2,1)` | 1, `forwards` (12 elements, staggered 0–660 ms) |

Transitions: `.rv` `opacity .7s / transform .7s var(--ease)` (22); `nav a.l` `color .18s` (31); `.btn`
`transform .18s, box-shadow .18s var(--ease)` (35); `.dot` `background .3s, box-shadow .3s var(--ease)` (77);
`.lbl` `color .3s var(--ease)` (78); `.card` `background .3s var(--ease)` (89). Plus the 2200 ms conveyor
`setInterval` (334), which is JS motion, not CSS.

**Keyframe-name collisions with the existing page: NONE.** Mock has `bpulse, pkt, gbar, cdraw`; existing has
`pulse` (123), `sweep` (167), `feedIn` (201). (`bpulse` ≠ `pulse`.)

### Reduced motion in the mock

page.src.html:117–120
```css
@media (prefers-reduced-motion: reduce){
  .rv{opacity:1;transform:none;transition:none}
  *,*:before,*:after{animation-duration:.01ms !important;animation-iteration-count:1 !important;transition-duration:.01ms !important}
}
```

page.src.html:382
```css
@media (prefers-reduced-motion: reduce){ .cdraw{animation:none;stroke-dashoffset:0} .pk{display:none} }
```

JS guard, 323 and 332–333:
```js
var reduced=window.matchMedia&&window.matchMedia("(prefers-reduced-motion: reduce)").matches;
...
draw(reduced?SCRIPT[2]:SCRIPT[0]);
if(reduced) return;
```

Per-animation judgement:
- **`pkt` / `.pk` — authored still, correct.** `display:none` (382) removes packets outright; without it the global cap would freeze a visible 3.5%-length dash mid-strand at opacity `.9`, reading as a rendering artifact.
- **`cdraw` — authored still, correct and load-bearing.** `animation:none;stroke-dashoffset:0` (382). Necessary because the base rule sets `stroke-dashoffset:1` (381), so the global cap alone would leave **every council connector invisible**.
- **Conveyor interval — authored still, best of the three.** Frame `SCRIPT[2]` (a `rate`-stage denial) is drawn and the timer never starts (332–333) — a *more informative* still than frame 0.
- **`bpulse` — no authored still**, merely inherits the global cap. Benign by luck: no `animation-fill-mode`, so it reverts to base `opacity` (implicitly 1).
- **`gbar` — no authored still**, benign by luck: base opacity comes from `bar.setAttribute("opacity",".55")` (305), settling between the keyframe extremes `.42`/`.85`.
- **`.rv` reveal — authored still** (118). The IntersectionObserver at 237–239 still runs and still adds `.in`; harmless.

**Gap:** `html{scroll-behavior:smooth}` (14) is **not** reset under reduced motion. The global cap covers
`animation-duration` / `animation-iteration-count` / `transition-duration` only — `scroll-behavior` is none of
those, so every in-page nav link (125–126, 129, 145–146) still smooth-scrolls. The existing page handles it at
`landing/index.html:86`. **Carry that rule over.**

### Build step

`page.src.html:3` is the literal token `@@FONTS@@`; opening the source directly gives unstyled fallback fonts.
**The substituting script DOES NOT EXIST on disk** — grepping the scratchpad for `page.src.html` or
`@@FONTS@@` across `*.py`, `*.mjs`, `*.js`, `*.sh` returns nothing. (`build_landing.py` in the scratchpad
builds the *existing* landing page, not this mock.) Since the target already embeds Archivo, the practical
move is to drop the mock's font block entirely and rename `"IBM Plex Mono"` → `"Plex Mono"`.

Beyond that there is **no** build requirement: no ES modules, no `import`, no JSX, no external
`<link>`/`<script src>`, no `fetch`. The JS is ES5-compatible `var`/`function` throughout.

### Mock's own bugs

- `<a class="l" href="#cap">platform</a>` (125) targets `#cap`, which is the **conveyor caption div**
  `<div class="cap" id="cap">` (178) inside the governance panel — not the capabilities section, which is
  `id="cap2"` (184). The "platform" nav link jumps to the wrong section.
- Hero meta claims `CLEARED 96.4% · DENIED 3.6%` (137) but the field draws 9 denials out of 128 strands =
  **7.0%**.
- `SEED 3573860127` (136) implies a PRNG that does not exist.
- `--raised2`, `--vdim`, `--live` declared and never referenced; `H` declared and never used (244).

---

## Contrast and accessibility findings

### Method and provenance

WCAG 2.x: `c = ch/255`; `lin = c/12.92 if c ≤ 0.04045 else ((c+0.055)/1.055)^2.4`;
`Y = 0.2126R + 0.7152G + 0.0722B`; `ratio = (Y_light+0.05)/(Y_dark+0.05)`. Worked example, `--dim #98A2AE`:
R 152→0.596078→(0.651078/1.055)=0.617136→^2.4=0.313989; G 162→0.635294→0.654307→0.361307;
B 174→0.682353→0.698913→0.423268; Y = 0.066754+0.258407+0.030560 = **0.355721**. `--bg #07080B` → lin
(0.002125, 0.002428, 0.003347), Y = **0.002430**. Ratio = 0.405721/0.052430 = **7.74:1**.

Palette source: `docs/superpowers/specs/2026-07-28-design-language-and-landing-design.md:55-65` (structure),
`:71-74` (state). Exact rows include `` | `--bg` | `#07080B` | page ground | `` (:55),
`` | `--faint` | `#646C77` | tertiary, mono labels | `` (:62),
`` | `--v` | `#7A5AF8` | **the accent** — CTAs, marker fills, the gate | `` (:63).
Type scale, same file `:87-89`: `` | Body | Archivo 400 | 16.5px / 1.62 | — | ``,
`` | Small body | Archivo 400 | 13.5px / 1.6 | — | ``,
`` | Eyebrow | IBM Plex Mono 500 | 10px, uppercase | 0.22em | ``.
**An 11.5px row DOES NOT EXIST in that table.** 11.5px + faint exists in shipped code:
`console/src/pages/Overview.css:99-101` (`.ov-feed-sub { font-size: 11.5px; color: var(--text-faint); }`) and
`:161-164` (`.ov-meter-nums`). On the landing page 11.5px is `--dim`, not `--faint`:
`landing/index.html:199` — `.feed-row .mono { font-size: 11.5px; color: var(--dim); }`.

**The proposed structure hexes are not yet in any stylesheet.** `#07080B` occurs exactly once in the repo, at
the spec line above. Live console tokens: `--bg: #121517;` / `--text-faint: #7e878e;`
(`console/src/styles.css:19,36`). Live landing tokens: `--bg: #121517;` / `--faint: #626b71;`
(`landing/index.html:28,35`). Only the four state hues already exist as proposed
(`console/src/styles.css:41-44`, `landing/index.html:36-39`).

Existing policy comment the new `--faint` would break, `console/src/styles.css:34-36`: *"Faint is the quietest
ink allowed to carry information: 4.9:1 on --bg, the floor for eyebrow-size text. Anything quieter is
decoration only."*

### Contrast ratios (proposed palette)

Surface luminances: `--bg` 0.002430, `--raised` 0.005181, `--raised2` 0.009225.

| Foreground | Hex | Y | vs `--bg` #07080B | vs `--raised` #0E1015 | vs `--raised2` #151821 |
|---|---|---|---|---|---|
| `--ink` | #F2F4F7 | 0.902940 | 18.18 | 17.27 | 16.09 |
| `--dim` | #98A2AE | 0.355721 | 7.74 | 7.35 | 6.85 |
| `--faint` | #646C77 | 0.147664 | **3.77** | **3.58** | **3.34** |
| `--v` | #7A5AF8 | 0.182272 | **4.43** | **4.21** | **3.92** |
| `--v2` | #A78BFA | 0.335829 | 7.36 | 6.99 | 6.51 |
| `--v3` | #C7B6FF | 0.528180 | 11.03 | 10.48 | 9.76 |
| `--live` | #5ad1c4 | 0.517601 | 10.83 | 10.29 | 9.58 |
| `--ok` | #6cc48f | 0.446512 | 9.47 | 9.00 | 8.38 |
| `--hold` | #e3a851 | 0.449302 | 9.52 | 9.05 | 8.43 |
| `--deny` | #e2685f | 0.268956 | 6.08 | 5.78 | 5.39 |

**Fails AA normal text (< 4.5:1)** on `--bg` and `--raised` alike: `--faint` #646C77 (3.77 / 3.58 / 3.34) and
`--v` #7A5AF8 **as text** (4.43 / 4.21 / 3.92). Everything else clears 4.5:1 on all three surfaces; tightest
passers are `--dim` 6.85 and `--deny` 5.39, both on `--raised2`.

**Fails AA large text (< 3:1): nothing.** Both failures clear 3:1 everywhere (worst cases `--v` 3.92,
`--faint` 3.34 on `--raised2`), so both are legal at ≥24px or ≥18.66px bold. `--faint` at 3.34 also clears the
3:1 non-text/UI-component threshold (1.4.11) — fine for hairline-adjacent icon strokes and borders.

### The specific mock uses

- **`--faint` #646C77 at 10px mono eyebrows — FAILS.** 3.77 / 3.58 / 3.34. 10px is nowhere near the
  18.66px-bold / 24px exemption, so 4.5:1 governs. Uppercase + 0.22em tracking (spec `:89`) makes real-world
  legibility worse than the number, not better.
- **`--faint` #646C77 at 11.5px captions — FAILS**, same ratios. This pairing is a shipped console pattern at
  `console/src/pages/Overview.css:99-101`, where today's `--text-faint` #7e878e gives **5.48:1** against
  #07080B — the proposed token is a **1.7:1 regression** on the quietest ink and violates the floor written at
  `console/src/styles.css:34-36`.
- **`--dim` #98A2AE at 13.5px / 16.5px body — PASSES.** 7.74 / 7.35 / 6.85. Clears AAA (7:1) on `--bg` and
  `--raised`, misses AAA only on `--raised2`.

**Minimum lightness bump for `--faint`, hue family preserved.** Base is HSL H=214.74° S=8.68% L=42.94%,
OKLCH L=0.5283 C=0.0199 H=256.34°. Raising L only:

| Target surface | Min hex | HSL L | OKLCH L | Achieved (bg / raised / raised2) |
|---|---|---|---|---|
| `--bg` only | `#707985` | 42.9% → 47.9% | 0.5283 → 0.5715 | 4.54 / 4.32 / 4.02 |
| `--raised` too | `#737C88` | 42.9% → 49.2% | → 0.5823 | 4.74 / 4.50 / 4.19 |
| all three | **`#78828E`** | 42.9% → 51.5% | → 0.5995 | 5.13 / 4.88 / **4.54** |

**Recommend `#78828E`** (or the OKLCH-locus equivalent `#79818C`, holding H=256.32° C=0.0192 exactly): a
**+8.6 percentage-point HSL lightness bump**, hue and saturation untouched, passing on every defined surface.
If faint text is contractually never placed on `--raised2`, `#737C88` (+6.3pt) is the true minimum. `#78828E`
is still darker than today's shipped `#7e878e`, so this is a conservative fix.

**Minimum bump for `--v` as text:** `#7B5BFA` reaches 4.50 on `--bg`, but `#8467FF` is needed for 4.53 on
`--raised2`. Cleaner answer: use `--v2` #A78BFA for accent text — exactly what the spec already says at `:64`
(`accent text, eyebrows, active labels`) — and reserve `--v` for fills.

### `--v` #7A5AF8 button with #FFFFFF text

**Passes — by 0.02.** Y(#7A5AF8) = 0.182272; ratio = 1.05/(0.182272+0.05) = 1.05/0.232272 = **4.521:1** ≥ 4.5.
The fill against the page is also fine as a UI component: vs `--bg` 4.43:1, vs `--raised2` 3.92:1, both over
the 3:1 required by 1.4.11.

Treat the margin as a constraint, not a result. Maximum luminance for white text at 4.5:1 is
Y = 1.05/4.5 − 0.05 = 0.183333; `--v` sits at 0.182272 — **0.00106 of luminance headroom**.

- `#7B5BF9` (one step lighter per channel) → **4.462, fails**.
- `#7D5EF8` → 4.349; `#8060F8` → 4.245 — any "lighten on hover/focus" treatment of the fill breaks AA.
- Darkening is safe and gains fast: `#7658F0` → 4.746; `#7050E8` → 5.248 (the latter drops fill-vs-`--bg` to
  3.82, still ≥3).
- `--v2` #A78BFA as a fill with white text is **2.72:1 — hard fail**; it needs dark text (#07080B on #A78BFA =
  7.36; on #C7B6FF = 11.03).

**Hover must darken, never lighten.** Disabled/`opacity` variants need re-checking, since alpha compositing
over `--bg` lowers Y.

### Colour-vision deficiency (Machado et al. 2009, severity 1.0, applied in linear RGB; ΔE = Euclidean OKLab)

**Violet vs the four state hues — safe under all three deficiencies**, one exception. Every violet↔state pair
stays at ΔE ≥ 0.144 (deuteranopia `--v3`/`--ok`); most sit 0.19–0.39. Under protanopia/deuteranopia the violets
collapse toward blue (`--v` → `#007AFD` / `#0072F5`) while the states collapse toward khaki/yellow (`--ok` →
`#C3B88C` / `#B6AF92`) — close to ideal, since the blue-yellow axis is the one both deficiencies retain. Under
tritanopia the violets go blue-grey (`--v` → `#4A81A1`, `--v2` → `#94A0B7`) and the states hold teal/red, still
separable.

**The exception: `--v3` #C7B6FF vs `--live` #5ad1c4.** Protanopia ΔE = 0.097 (`#A5C1FF` vs `#C7C6C4`,
luminance ratio 1.05); deuteranopia ΔE = 0.078 (`#A7BFFD` vs `#B5B9C5`, ratio 1.07). Both become pale, nearly
equiluminant, low-chroma washes. **The spec's partition rule at `:66` ("Violet is deliberately chosen for
maximum separation from all four state hues") holds for `--v` and `--v2` but is NOT true of `--v3` against
`--live`** — qualify the claim, or keep `--v3` ("brightest strands", `:65`) away from live-state marks.

**Worst state-hue pair: `--live` #5ad1c4 vs `--ok` #6cc48f** — closest pair even in normal vision (ΔE 0.069,
hue separation 29.6°, luminance ratio 1.14, essentially equiluminant):

| Vision | `--live` sim | `--ok` sim | ΔE | Luminance ratio |
|---|---|---|---|---|
| normal | #5AD1C4 | #6CC48F | 0.069 | 1.14 |
| protanopia | #C7C6C4 (C=0.003, effectively grey) | #C3B88C | 0.074 | 1.17 |
| deuteranopia | #B5B9C5 (C=0.018, effectively grey) | #B6AF92 | 0.068 | 1.12 |
| tritanopia | #00D5CD | #58C3B6 | **0.052** | 1.15 |

Under tritanopia they are 5.4° apart in hue at similar chroma — the worst case in the palette. Under
protan/deutan `--live` is functionally achromatic and `--ok` a dull khaki, leaving only a ~1.15 luminance
ratio, far under the 3:1 needed to distinguish adjacent UI components. These two carry the semantically
adjacent meanings "in flight right now" (`:71`) and "completed / allowed" (`:72`) — the pair users most need
to tell apart, and the pair they can least tell apart.

Secondary risks, all protan/deutan, all from the red-green collapse:
- `--ok` vs `--hold`: protanopia ΔE 0.074 (`#C3B88C` vs `#BEAA48`, ratio 1.17); deuteranopia ΔE 0.088.
- `--ok` vs `--deny`: deuteranopia ΔE 0.087 (`#B6AF92` vs `#A4975C`, hue 1.3° apart, ratio 1.33) — "allowed"
  vs "denied" reduced to a 1.33:1 lightness step. Protanopia better at ΔE 0.190 (ratio 2.05).
- `--hold` vs `--deny`: deuteranopia ΔE 0.116, ratio 1.48.
- Tritanopia keeps green/teal vs red/orange well (`--ok`/`--deny` ΔE 0.308) but pushes `--hold` #E3A851 to
  `#F69995` and `--deny` to `#F65366` — ΔE 0.138, 5.0° apart, ratio 1.56: "held" and "denied" become two pinks.

**Net:** hue alone does not carry state for any of the three common deficiencies. The four state hues are
near-equiluminant by construction — pairwise luminance ratios in normal vision are 1.01–1.78
(`ok`/`hold` = 1.01, `live`/`ok` = 1.14, `live`/`hold` = 1.14, `ok`/`deny` = 1.56, `hold`/`deny` = 1.57,
`live`/`deny` = 1.78) — so lightness cannot rescue them either. **A redundant non-colour channel (glyph, fill
pattern, or a text label) is required wherever state is conveyed**, and `--live` vs `--ok` specifically needs
either that redundancy or a hue/lightness re-separation before it ships.

### Accessibility findings in the mock

- **Decorative SVG marking is correct for the hero:** `<svg class="field" id="f" … aria-hidden="true">` (134).
- **But the council SVG is *also* `aria-hidden="true"` (203) while carrying the section's only concrete
  content.** Everything specific there — `"objective"`, `"one question"`, the five model names
  `kimi-k3 / glm-5.2 / qwen3.8-max / deepseek-v4 / minimax-m3`, `"judge"`, `"synthesis"`, `"verdict"`,
  `"dissent"` — exists solely as SVG `<text>` created at 351 and 354. A screen-reader user gets the h2 and
  lede and nothing else. This is not "read out of order" — it is **not read at all**. Same shape in the hero:
  the six `AUTH…AUDIT` labels (301) are the only place the stage names appear as hero content, and they are
  hidden. Both need a visually-hidden `<ul>` (or `role="img"` + `aria-label`) mirroring the content.
- **`aria-live` does not exist anywhere in the file.** `grep -n "aria-\|role="` returns only the two
  `aria-hidden` attributes. The conveyor's caption swap every 2200 ms (330, 334) is silent — the right
  default. Flip side: the `data-r` state changes (328) are invisible to AT. **If live-region semantics are
  added later, do NOT put them on `#cap`, which changes 27 times a minute.**
- **Keyboard reachability is fine; keyboard visibility is not.** Every interactive element is a native
  `<a href>` — 4 nav links (125–126), 2 nav buttons (128–129), 2 hero buttons (145–146), 12 footer links
  (230–232). No `tabindex`, no div-as-button. But **`grep ":focus"` returns nothing** — no `:focus` or
  `:focus-visible` rule exists, while `.btn:hover{transform:translateY(-1px)}` (36), `.btn.p:hover` (38),
  `.btn.s:hover` (40), `nav a.l:hover` (32), `.card:hover` (90) and `.fgrid .col a:hover` (115) all style
  hover. Nothing sets `outline:none`, so the UA ring survives — but keyboard users get none of the affordance
  parity mouse users get.
- **Thirteen dead links.** `href="#"` at 128, 230 (×4), 231 (×4), 232 (×4). Each announces as a link and jumps
  to the top of the document.
- **False keyboard-shortcut affordance.** `.kbd` badges render `D` (128), `R` (129, 145), `G` (146) as if
  shortcuts exist. **There is no `keydown`/`keyup` listener anywhere in the file.** A screen reader reads the
  button as "read the docs D" / "run it locally R"; a sighted keyboard user who presses `R` gets nothing.
- **No `<meta name="viewport">`** (line 1) — mobile browsers fall back to a ~980px layout viewport and scale
  the page down, so fixed 118px/16.5px type ends up sub-legible before any zoom.
- **No `lang` attribute** (there is no `<html>` element at all), **no `<main>`, no skip link.** Landmarks are
  only the implicit `nav` (123) and `footer` (225).
- **Contrast failures in the mock as authored** (against `--bg #07080B`):
  - `.meta .ill` / `pre .c` `#4d545d` → **2.61:1** (47, 107). Fails 4.5:1 and even 3:1.
  - `--faint #646C77` → **3.77:1**, used at 10–11.5px in `.cell span` (59), `.cap` (85), `.tag` (95),
    `.tbar span` (105), `.meta` (45), `.fgrid .col b` (113), `.lbl` (78). Fails 4.5:1.
  - SVG stage label `#6E7681` at `font-size 9.5` (299) → **4.36:1**. Fails.
  - Council sub-label `#646C77` on the `#0E1015` box fill (353, 347) → **3.58:1**. Fails.
  - Passing: `--dim #98A2AE` 7.74:1; `--v2 #A78BFA` 7.36:1; `--ink #F2F4F7` ~18:1.
- **Heading order is clean:** one `h1` (142), `h2` per section (161, 186, 201, 209), `h3` in cards (189–194).
  The `.eyebrow` divs above each `h2` (141, 160, 185, 200, 208) are unlabelled `<div>`s — no heading-level
  pollution.

---

## TRAPS

Numbered, blunt, exhaustive. Anything that would silently break the page or ship a wrong claim.

### Paths, provenance, and file facts

1. **The path `undefined/landing/index.html` does not exist.** The real file is
   `/home/iofahd/code/agentos/landing/index.html`. Every survey received `undefined` as an input path and had
   to resolve it. Do not let any tool call inherit the literal string `undefined`.
2. **The page is 868 lines, not 865.** Any instruction, diff, or line-anchored patch built on 865 is offset.
   Re-verified with `wc -l`.
3. **Lines 17 and 24 are base64 font payloads of 46,637 and 19,906 characters.** Do not `cat`, do not
   line-wrap, do not reformat with a prettifier. A regex over the whole file will match inside them.
4. **The mock's builder script is gone.** `page.src.html:3` is the literal token `@@FONTS@@`; grepping the
   scratchpad for `page.src.html` or `@@FONTS@@` across `*.py/*.mjs/*.js/*.sh` returns nothing.
   `build_landing.py` builds the *existing* page, not the mock. Opening `page.src.html` directly yields
   unstyled fallback fonts and a page that looks broken for reasons unrelated to the design.

### Silent CSS/JS breakage on paste

5. **Font-family name mismatch — the loudest silent failure.** Mock declares
   `--mono:"IBM Plex Mono",…` (page.src.html:10) plus three hard-coded `font-family="IBM Plex Mono, monospace"`
   SVG attributes (299, 350, 353). The existing embedded face is declared **`font-family: "Plex Mono"`**
   (`landing/index.html:40` and its `@font-face` at 19–24). Pasting the mock's `:root` makes every mono string
   fall through to `ui-monospace` **with no error**. Rename to `"Plex Mono"` in all four places.
6. **CSS variable names differ between mock and target.** `--raised2` vs `--raised-2`; `--line`/`--line2` vs
   `--border`/`--border-strong`. A stale `var(--raised-2)` in copied CSS resolves to nothing and inherits.
7. **`landing/index.html:135` is invalid CSS** — an at-rule inside a selector list — so `.btn-solid` never
   gets white text in light mode. If the rebuild copies this line forward, the bug ships again:
   `:root[data-theme="light"] .btn-solid, @media (prefers-color-scheme: light){ :root:not([data-theme="dark"]) .btn-solid{ color: #fff; } }`
8. **17 class-name collisions between mock and existing page:** `a, band, brand, btn, card, chain, dot,
   eyebrow, grid, in, lbl, mono, n, rail, sub, tag, wrap`. The damaging ones:
   - **`.rail`** — mock: hero stat strip, `position:absolute;left:44px;right:44px;bottom:54px;display:flex`
     (56). Existing: a 2px progress track inside the chain,
     `position:absolute;left:5%;right:5%;top:7px;height:2px` (`landing/index.html:165`) with `.rail .fill`
     running `sweep` (166). **Mutual destruction.**
   - **`.dot`** — mock: 15px square gate node with inset box-shadow (76–77), **unscoped**. Existing:
     `.brand .dot` is a 7px teal circle running `pulse` (`landing/index.html:122`). The mock restyles the
     brand dot's `width/height/border-radius/background`.
   - **`.chain`** — mock: `display:flex` with `.st` children (72–74). Existing: grid with `.stage` children
     (`landing/index.html:162,177-178`) plus a `@media (max-width:620px)` override (188).
   - **`.band`** — mock uses `<section class="band">` styled by
     `section.band{padding:130px 0;border-top:1px solid var(--line)}` (63). Existing `.band`
     (`landing/index.html:274`) is lower-specificity but still applies, adding an unwanted
     `background: var(--raised)` and `border-bottom`; `.band .wrap{padding:66px 28px}` (275) fights the mock's
     `.wrap` padding.
   - **`.wrap`** — `max-width:1320px;padding:0 44px` (18) vs `max-width:1120px;padding:0 28px`
     (`landing/index.html:98`). Whichever loses, every section on the other half of the page shifts.
   - **`.sub`** — mock declares it globally at 16.5px/`--dim` (53); existing scopes it as `.stage .sub` at
     12px (`landing/index.html:178`) and hides it under 620px (188).
   - `.n`, `.tag`, `.card`, `.grid`, `.btn`, `.eyebrow`, `.brand`, `.mono`, `.lbl` all have competing
     declarations at `landing/index.html:210, 235, 225, 222, 127, 100, 121, 108, 177`.
   - `.in` is not a standalone selector in the existing page (only `.reveal.in`, 297) — no rule conflict, but
     both pages use the same token for two different observers (`.rv` in the mock, `.reveal` in the target).
9. **Keyframe-name collisions: NONE.** Mock `bpulse, pkt, gbar, cdraw` vs existing `pulse` (123), `sweep`
   (167), `feedIn` (201). Do not rename defensively — renaming `cdraw` without also updating the
   reduced-motion override at 382 breaks trap #14.
10. **ID collision: `start`.** Mock `<section class="band" id="start">` (207) vs existing
    `<section class="final" id="start">` (`landing/index.html:520`). No other ID overlaps. Mock IDs:
    `cap, cap2, chain, council, cv, f, gin, gout, gov, halo, hero, start`. Existing IDs: `audit-feed,
    council-viz, governance, multiverse, operators-viz, oss, platform, rd-audit, rd-count, rd-p95, rd-spend,
    spark-line, start, tt`. The SVG gradient IDs `gin`/`gout`/`halo` (248, 253, 257) are safe today but are
    unnamespaced globals — a second SVG reusing `halo` silently repaints the first.
11. **Bare-element rules in the mock would restyle the whole existing page:**
    `*{box-sizing:border-box;margin:0;padding:0}` (13), `html` (14), `body` (15), `a{text-decoration:none}`
    (19), `nav{position:fixed;top:0;…height:62px;z-index:40}` (26–27), `h1{font-size:118px}` (49),
    `h2{font-size:56px}` (64), `pre{…}` (106), `footer{…}` (111). The existing page has `<nav class="nav">`
    (304), six `<h2>` (369, 421, 447, 472, 494, 509, 522), an `<h1>` (320) and a `<footer>` (531) — all hit.
    In particular the mock's fixed 62px `nav` would rip the existing nav out of `<header class="hero">` (318).
12. **`var io=new IntersectionObserver(...)` (page.src.html:237) is a true global.** The existing page's `io`
    is inside an IIFE (`landing/index.html:544,556`), so no collision today — but paste both at top level and
    the second `var io` silently wins.
13. **The mock has two `<style>` blocks** (2–121 and 375–383); the keyframes live in the second, *after*
    `</script>`. Merging into the target's single style block is required, and the merge **must preserve
    source order** — the reduced-motion overrides at 382 come after the base rules at 377/379/381 and depend
    on it.

### Motion and reduced-motion

14. **`.cdraw` base rule sets `stroke-dashoffset:1` (381).** If the reduced-motion override at 382
    (`.cdraw{animation:none;stroke-dashoffset:0}`) is dropped, reordered, or renamed away, the global
    `animation-duration:.01ms` cap leaves **every one of the 12 council connectors invisible** under reduced
    motion — a blank panel, no error.
15. **`.pk{display:none}` under reduced motion (382) is load-bearing.** Without it the global cap freezes a
    3.5%-length dash mid-strand at opacity `.9` on 8 strands — reads as a rendering artifact.
16. **`html{scroll-behavior:smooth}` (page.src.html:14) is NOT reset under reduced motion.** The global cap
    covers `animation-duration` / `animation-iteration-count` / `transition-duration` only. Every in-page nav
    link (125–126, 129, 145–146) still smooth-scrolls. **Carry over `landing/index.html:86`:**
    `@media (prefers-reduced-motion: reduce) { html { scroll-behavior: auto; } }`
17. **`bpulse` and `gbar` have no authored reduced-motion still** — they are benign only *by luck*
    (`bpulse` has no `animation-fill-mode` so reverts to implicit opacity 1; `gbar` settles at the
    `bar.setAttribute("opacity",".55")` from line 305, between keyframe extremes `.42`/`.85`). Change either
    base opacity and the reduced-motion result changes silently.
18. **The existing page's four JS reduced-motion guards must survive the rebuild** — lines 585, 700, 778, 818,
    all `window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches`. Losing 700
    restarts the 520 ms chain interval; losing 778/818 restarts the 3200 ms and 4000 ms intervals. Line 700's
    single representative request is
    `{stop:5, outcome:'ok', model:MODELS[0], org:ORGS[0], tokens:1024, cost:2.4, latency:380}` (756–761).

### Responsive / viewport

19. **The mock has no responsive design at all.** `grep -n "@media" page.src.html` returns **only the two
    `prefers-reduced-motion` blocks** (117, 382). Not one width breakpoint. Concretely:
    - `#hero{height:1000px}` (43) — fixed, not `min-height`/`svh`.
    - `.hwrap{position:absolute;left:44px;top:200px;width:780px}` (48) — a hard 780px column starting at
      x=44. At 375px viewport that is 824px of content clipped by `body{overflow-x:hidden}` (15).
    - `h1{font-size:118px}` (49) — fixed px, no clamp.
    - Hero SVG uses `preserveAspectRatio="xMidYMid slice"` on `viewBox="0 0 1440 1000"` (134). At 375×1000 the
      scale is `max(375/1440, 1000/1000) = 1`, cropping to viewBox x ∈ [532.5, 907.5]. **The gate column
      (`GX=980`) and all six stage labels (x = 1006…1090) fall entirely outside the crop** — on a phone the
      hero illustration loses its subject.
    - `.rail` is five `flex:1` cells with 22px mono numbers (56–57), no wrap.
    - `.grid{grid-template-columns:repeat(3,1fr)}` (88) unconditional; the existing page steps 3→2→1 at
      900/620px (`landing/index.html:223-224`).
    - `.fgrid{display:flex;gap:70px}` (112) unconditional, 4 columns.
    - `.council svg{height:300px}` with `viewBox="0 0 1200 300"` (99, 203) and default `meet` scaling: inside
      a 375px viewport the inner width is ~219px → scale ≈ 0.18 → the 12px labels render at ~2.2px.
      Illegible, not broken — which is worse, because nothing errors.
    - `nav` is `display:flex` with no `flex-wrap` and 6 children (123–130).
    - Existing breakpoints that must be preserved or replaced deliberately: `720px` (nav hidden, stack, band
      grid2), `620px` (chain subs, readings, grid), `820px` (feature), `900px` (grid), `min-width: 621px`
      (chain-live row).
20. **The mock is a fragment, not a document:** line 1 is
    `<!doctype html><meta charset="utf-8" /><title>…</title>`. No `<html>`, `<head>`, `<body>`, no `lang`, no
    `<meta name="viewport">`, no favicon, no `<meta name="description">`. The target has all of these
    (`landing/index.html:1-8`). Dropping the viewport meta alone makes mobile fall back to a ~980px layout
    viewport.

### Theme

21. **The mock is dark-only; the target is not.** The mock has **zero** light-theme support — no
    `[data-theme]`, no `prefers-color-scheme`. The existing page has a full light palette
    (`landing/index.html:48-63`), a duplicate `prefers-color-scheme: light` block (65–82), and a working
    toggle (`#tt`, script 545–553, persisted to `localStorage` key **`agentos-theme`**).
22. **The mock hard-codes theme-bound colours in JS/SVG where no CSS variable can reach them:** `#7A5AF8`,
    `#C7B6FF`, `#A78BFA`, `#E9E2FF`, `#e2685f` (270, 272, 282, 294, 347, 359), `#07080B` as the label-plate
    fill (297), `#0E1015` as box fill (347), `#F2F4F7` / `#646C77` / `#6E7681` as text fills (299, 350, 353).
    **In light mode the gate labels would be near-white-on-white and the plates would be black rectangles.**
    These must be re-read from `getComputedStyle` or re-drawn on theme change.
23. **The light theme is duplicated verbatim in two blocks** (48–63 and 65–82). Editing one and not the other
    produces a page that is correct with an explicit toggle and wrong under OS preference, or vice versa.
    Light does **not** redeclare `--mono`, `--sans`, `--eyebrow`, `--track` — do not assume it is a full
    palette.

### Determinism and simulation honesty

24. **The existing page contains zero executable `Math.random`.** `grep -n "Math\.random"` hits only comment
    lines 573, 694, 770, 809, each asserting "no Math.random". All four PRNGs are seeded `mulberry32` —
    `0x5EED` (710), `0xC0DE1` (773–804 IIFE), `0xA0BA5` (811–865 IIFE). **Introducing `Math.random` breaks a
    documented invariant and makes screenshots non-reproducible.**
25. **The mock has no PRNG at all** — no `Math.random`, no `Date.now`, no `performance.now`, no `new Date`.
    The hero string `SEED <b>3573860127</b>` (page.src.html:136) is cosmetic; nothing reads it. Do not carry a
    seed label onto a design that has no seed.
26. **`Live` (line 331) over a seeded simulation is a false claim** — see FALSE CLAIMS #6. The page's own
    comments at 693–694 say the panel is a deterministic in-file simulation.
27. **`auditRows += 1` at line 605 is unconditional** and the feed row at 623 prints tokens and dollars for
    every settled request including denials — see FALSE CLAIMS #7. **Do not ship the audit's one-line fix**
    (`if (r.outcome !== 'deny' || r.stop !== 2) auditRows += 1;`); it patches the counter for one stage and
    leaves Rate/Guardrail/Upstream rows printing fabricated tokens and cost.
28. **The mock's conveyor `SCRIPT[7]` is `{stop:"budget", o:"deny", c:"402 — key budget exhausted"}`** — the
    exact case the gateway never audits (`rbac.go:74-96`). If any counter is ever wired to this conveyor, that
    frame must not increment an "audit rows" tile.
29. **The mock's hero meta claims `CLEARED 96.4% · DENIED 3.6%` (137) but the field draws 9 of 128 = 7.0%.**
    Fix the copy or fix the strand counts; do not ship both.
30. **The mock's "platform" nav link points at the wrong element** — `href="#cap"` (125) targets the conveyor
    caption `<div class="cap" id="cap">` (178), not the capabilities section `id="cap2"` (184).

### Claim-shipping traps

31. **Do not source landing copy from `README.md` blindly.** `README.md:272` repeats the same false CI-gate
    claim the page makes (*"CI runs the … matrix on every push plus an eval gate"*) — see FALSE CLAIMS #10.
32. **`README.md:141-146` directly contradicts the page's own CTA note and final lede** — *"A bare `make up`
    is **not** the $0 path…"*. Any copy asserting air-gapped or zero-cost from a bare `docker compose up` is
    wrong (FALSE CLAIMS #18, #20).
33. **Ollama is not a compose service.** `deploy/compose.yaml` has no Ollama; `:32`
    `AGENTOS_OLLAMA_BASE_URL: ${AGENTOS_OLLAMA_BASE_URL:-}` is empty. The air-gapped path requires an Ollama
    the operator runs on the host (`.env.example:16-17`). `README.md:159-165` also notes `gemma3` is flaky.
34. **All five frontier council members ship `enabled: false`** (`runtime/council.yaml:3-6,23-48`;
    `deploy/providers.json:2,8,15,21,29,36` — every frontier endpoint/price a *"PLACEHOLDER … NOT verified
    against vendor documentation"*). Any copy saying "each on a different frontier model" is false as shipped.
35. **`deepseek/deepseek-v4` does not exist** — the real id is `deepseek/deepseek-v4-pro`
    (`runtime/council.yaml:40`; `deploy/providers.json:30`). It appears at `landing/index.html:459` **and** at
    `page.src.html:343` in `MEM`. **Do not repeat the audit's "would 400" rationale** — `provider.go:130-156`
    routes any model id under an *enabled* provider at $0, and under the shipped disabled default the correct
    id 400s identically. The id is still wrong; the reason given for it was not.
36. **`sql.write` and `fs.write` do not exist anywhere in the platform.** Two sites:
    `landing/index.html:484` (`webhook · sql.write held`) and `landing/index.html:837` (JS `RUNS` roster,
    `webhook · fs.write held`). The SQL connector registers only `list_tables`, `describe_table`, `query`
    (`connectors/sql/internal/tools/tools.go:51-65`). There is no filesystem connector. The only occurrence of
    `sql.write` in the repo is a fixture at `runtime/tests/test_council_store.py:78`. **Fix both sites.**
37. **`unanimous` is not a status the platform emits.** Only `STATUS_OK = "ok"` (`council/judge.py:26`) and
    `JUDGE_UNAVAILABLE = "judge_unavailable"` (`:25`). `0.67` is `agreement` (clamped `judge.py:121`, compared
    against `agreement_threshold: 0.6` at `council/loop.py:68`). And `judge.py:50` says dissent is *"Empty
    when unanimous"* — so the pill contradicts the row directly above it.
38. **The clipboard string is not runnable:** `'git clone && cd agentos/deploy && docker compose up -d'`
    (line 563). `git clone` with no argument exits non-zero and the `&&` chain stops. The README quickstart is
    a third thing again (`README.md:112-118`) and also needs `cp .env.example .env` (`.env.example:1`).
39. **The kill switch is council-only.** `council/store.py:360-377`, `council/api.py:128-137`,
    `council/loop.py:138-142,184`. `operators/api.py` (163 lines, read in full) has no pause/resume route —
    only the per-operator `enabled` flag (`:33,115-122`) and process-level `AGENTOS_AUTONOMY_ENABLED`
    requiring a restart (`config.py:68`; `api.py:229-234`).
40. **`connectors/browser/Dockerfile` has no `USER` directive** — verified twice. Any "non-root throughout"
    claim is false. Only the sandbox has `cap_drop`/`read_only`/`security_opt` in Compose
    (`compose.yaml:136-144`); `templates/postgres.yaml` has no `securityContext`; SOAP, browser and SSH have
    no Helm template at all (`docs/architecture.md:96-98`).
41. **HITL is off by default and, on the `deep` profile, absent entirely.** `agent.py:113`
    `interrupt_before = ["tools"] if settings.approval_tool_names else None`; `config.py:43`
    `approval_tools: str = ""`; `compose.yaml:104` and `.env.example:25` empty. `agent.py:86-88` and
    `SECURITY.md:238-243`: *"The deep agent profile has no human-in-the-loop … exposes no `interrupt_before`
    pass-through"*. `SECURITY.md:244-246`: *"**HITL is off by default.**"* Also `docs/project-context.md:525`,
    `docs/concepts.md:68`.
42. **The fail-closed classifier is council + `profile: react` only** — one production call site,
    `council/fanout.py:248-251`. `SECURITY.md:110` and `console/src/pages/docs/content.ts:257` both scope it.
    Never generalise it to "an unattended agent".
43. **Six sub-systems are OFF in shipped defaults** and must not be described as active: guardrails
    (`compose.yaml:34` `off`), rate limits (`:45` `0`), operators scheduler (`:118` `false`), council
    heartbeat (`:114` `0`), SSO (`:54` empty issuer), SCIM (`:50` empty token). Plus OTel (`main.go:263`),
    Langfuse (separate overlay `deploy/compose.langfuse.yaml`), audit retention (`main.go:170`).
44. **Do NOT "fix" two things the audit flagged** — both would be regressions:
    `In-repo, checksummed skills…` (line 477) and `Any model provider, one interface…` (line 500). See FALSE
    CLAIMS #23.
45. **Do not print the "8–128× vs `DefaultReserveUSD = 0.05`" framing for the spend tile.** The tile is a
    cumulative sum (`spend += r.cost`, line 606), not a per-request figure; the re-check says the audit's
    framing overstates the mismatch. The real fix is labelling the panel simulated.

### Self-containment

46. **The page has zero network requests and must stay that way.** `grep -n "https://"` → **NO MATCHES**.
    `grep -n "http://"` → exactly one hit, `landing/index.html:8`, and it is an SVG namespace inside a `data:`
    URI favicon. `grep -nE "fetch\(|XMLHttpRequest|WebSocket|EventSource|<script src|<img "` → no hits. Fonts
    are inlined `data:font/woff2;base64`; every icon is inline `<svg>`. The only external-ish API is
    `navigator.clipboard.writeText` (563), which is local. **Any CDN font, remote image or analytics tag
    breaks the page's own sovereignty claim.**
47. **`defs.innerHTML=` on an SVG element (page.src.html:247)** works in current browsers but should become
    `createElementNS` if the target ever gains an XML-serialization step.

### Accessibility traps

48. **`aria-hidden="true"` on the council SVG (page.src.html:203) hides the section's only content** — the
    five model names, `objective`, `one question`, `judge`, `synthesis`, `verdict`, `dissent` exist solely as
    SVG `<text>` (351, 354). Same for the hero's six `AUTH…AUDIT` labels (301). Screen-reader users get the h2
    and lede and **nothing else**. Add a visually-hidden `<ul>` or `role="img"` + `aria-label`.
49. **No `:focus` or `:focus-visible` rule exists in the mock** while six `:hover` rules do (32, 36, 38, 40,
    90, 115). Nothing sets `outline:none` so the UA ring survives, but keyboard users get no affordance
    parity. Add focus styling explicitly.
50. **Thirteen `href="#"` dead links** (128, 230 ×4, 231 ×4, 232 ×4) and **four false keyboard-shortcut
    badges** (`.kbd` `D` at 128, `R` at 129 and 145, `G` at 146) with **no `keydown`/`keyup` listener anywhere
    in the file**. Either implement the shortcuts or remove the badges.
51. **Never attach `aria-live` to `#cap`** — the conveyor caption changes every 2200 ms, i.e. **27 times a
    minute**. That is an announcement storm.
52. **`--faint #646C77` fails AA at every size the design uses it** (3.77 / 3.58 / 3.34 on
    `--bg`/`--raised`/`--raised2`; 10px eyebrows and 11.5px captions both governed by 4.5:1). Ship
    **`#78828E`** (passes 5.13 / 4.88 / 4.54, hue and saturation untouched) or `#737C88` if faint text never
    lands on `--raised2`. Note this token is still darker than today's shipped `#7e878e` and that the current
    proposal is a **1.7:1 regression** violating the written floor at `console/src/styles.css:34-36`.
53. **`--v #7A5AF8` fails AA as text** (4.43 / 4.21 / 3.92). Use `--v2 #A78BFA` for accent text — which the
    spec itself says at `:64` — and reserve `--v` for fills.
54. **The `--v` button with white text passes by 0.02 (4.521:1) with 0.00106 luminance of headroom.**
    `#7B5BF9` — one step lighter per channel — already fails at 4.462. **Hover and focus states must darken,
    never lighten.** `#7658F0` → 4.746; `#7050E8` → 5.248. `--v2` as a fill with white text is **2.72:1, hard
    fail** — it needs dark text.
55. **`--live` and `--ok` are effectively indistinguishable under all three common CVDs** (ΔE 0.052–0.074,
    luminance ratios 1.12–1.17) and they carry the adjacent meanings "in flight" and "completed / allowed".
    All four state hues are near-equiluminant (pairwise ratios 1.01–1.78), so lightness cannot rescue them.
    **A redundant non-colour channel — glyph, fill pattern, or a text label — is mandatory wherever state is
    conveyed.**
56. **The spec's claim at `:66` ("Violet is deliberately chosen for maximum separation from all four state
    hues") is not true of `--v3` #C7B6FF against `--live` #5ad1c4** (protanopia ΔE 0.097, deuteranopia 0.078).
    Qualify the claim or keep `--v3` away from live-state marks.
57. **Unused tokens invite false confidence:** the mock declares `--raised2`, `--vdim` and `--live` and never
    references any of them; `H` is declared at page.src.html:244 and never used. A contrast audit that scores
    `--live` on this page is scoring a token the design never renders.
