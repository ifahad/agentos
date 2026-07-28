# Design Language + Landing Page Rebuild — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild `landing/index.html` on a new design language — dark ground, a committed violet accent, a 118px display scale, and a signature "gate field" visual — while correcting twenty false or overstated claims the current page makes about the platform.

**Architecture:** One hand-authored, self-contained HTML file. No build step, no dependencies, no external requests; fonts stay base64-inlined exactly as they are today. Work proceeds top-down: the truth fixes land first on the existing page (independently valuable), then the token layer, then section by section. Every animation is CSS keyframes or a fixed script — never `Math.random`, never SMIL.

**Tech Stack:** Hand-written HTML/CSS/ES5 JavaScript. Archivo + IBM Plex Mono, already base64-embedded in the file. Playwright (`/home/iofahd/code/agentos/connectors/browser/.venv/bin/python`) for render verification.

## Global Constraints

From `docs/superpowers/specs/2026-07-28-design-language-and-landing-design.md` and the verified recon at `docs/superpowers/research/2026-07-28-landing-rebuild-factsheet.md`.

- **The repo is PRIVATE but now pushed.** Commit locally; push only when asked.
- **One file changes: `landing/index.html`.** Nothing else in the repo may be modified.
- **No build step, no dependency, no framework, no external request.** No CDN, no analytics, no remote font, no `<script src>`, no `<link>` to any host, no `fetch`.
- **Fonts stay base64-inlined** in the existing `@font-face` blocks. **The embedded mono family is named `"Plex Mono"`, NOT `"IBM Plex Mono"`.** The mock uses the wrong name; using it fails silently to a fallback face with no error.
- **No `Math.random` anywhere.** Every sequence is a fixed script or a pure function of an index.
- **BOTH THEMES SHIP.** *(Corrected 2026-07-28 after Task 2 — the spec's original "no light theme" non-goal was written on a false premise. The page already has a working light theme and a toggle; the owner chose to keep them.)* The `:root[data-theme="light"]` block, the `@media (prefers-color-scheme: light)` block and the `localStorage` toggle all stay functional. Every render check runs **twice — once per theme.** See R1b for the light palette.
- **Two colour registers, never blurred:** violet (`--v` / `--v2` / `--v3`) is interface — headings, CTAs, links, structure. `--live` / `--ok` / `--hold` / `--deny` are machine state and appear **only** where a real outcome is depicted.
- **Every governance visual carries a visible `ILLUSTRATION · SCRIPTED SEQUENCE` marker on the visual itself**, not only in surrounding prose.
- **No fabricated governance metric may appear unmarked.** The page holds no live connection to a gateway.
- **The chain is `auth → rate → budget → guardrail → upstream → audit`**, matching `CHAIN_STAGES` in `console/src/lib/chain.ts`. A denial may be depicted only at **rate**, **budget** or **guardrail**. An upstream failure is amber, never red. Nothing may depict a denial at `auth` or `audit`.
- **Reduced motion:** every looping animation ships an authored still. A global cap alone is insufficient — it sets `animation-iteration-count: 1`, freezing a loop at an arbitrary frame.

---

## Reference: verified facts

Checked against source by four parallel surveys plus three adversarial re-checks. **Do not re-derive these.**

### R1. Palette — with the contrast corrections applied

| Token | Value | Note |
|---|---|---|
| `--bg` | `#07080B` | page ground |
| `--raised` | `#0E1015` | panels |
| `--line` | `rgba(255,255,255,.06)` | hairlines |
| `--line2` | `rgba(255,255,255,.12)` | dashed rules, emphasised hairlines |
| `--ink` | `#F2F4F7` | primary text — 18.3:1 on `--bg` |
| `--dim` | `#98A2AE` | secondary text — 7.74:1 on `--bg`, passes AA |
| `--faint` | **`#78828E`** | **CHANGED from the spec's `#646C77`, which is 3.77:1 and fails AA for the 10px eyebrows and 11.5px captions it is used for.** `#78828E` clears 4.5:1 in the same hue family. |
| `--v` | `#7A5AF8` | **fills only — never text.** As a button background with `#FFFFFF` text it passes by 0.02, so it must not be lightened on hover. |
| `--v2` | `#A78BFA` | accent text, eyebrows, active labels |
| `--v3` | `#C7B6FF` | accent display type, brightest strands |
| `--live` | `#5ad1c4` | in flight |
| `--ok` | `#6cc48f` | allowed |
| `--hold` | `#e3a851` | awaiting a human |
| `--deny` | `#e2685f` | denied / failed |

### R1b. Light palette — already tuned, keep these values

`--bg #eceae4` · `--raised #f6f5f1` · `--raised-2 #ffffff` · `--border rgba(0,0,0,.10)` · `--border-strong rgba(0,0,0,.18)` · `--ink #1a1d1f` · `--dim #565d63` · `--faint #8a9198`, with light-adapted state hues `--live #12897c` · `--ok #2f8f57` · `--hold #a8721c` · `--deny #bb4038`, plus `--shadow` and `color-scheme: light`.

**The violet needs light-ground variants, computed not guessed.** `--v #7A5AF8` is tuned against near-black; on `#eceae4` it must be darkened enough to pass 4.5:1 as text and keep white-on-violet legible as a button fill. Record the arithmetic in the task report.

**The light palette is duplicated verbatim in two blocks** (`:root[data-theme="light"]` and the `prefers-color-scheme` media query). Editing one and not the other is the trap — they must stay in sync, or better, be collapsed so there is one source of truth.

**`--live` and `--ok` are ΔE 0.052–0.074 apart under all three common colour-vision deficiencies** while meaning "in flight" versus "allowed". Anywhere both can appear, the difference must also be carried by a **non-colour channel** — a distinct shape, a label, or a fill-versus-outline treatment. Do not rely on hue alone.

### R2. The current page — what exists

`landing/index.html` is **868 lines** (not 865). It opens with large base64 `@font-face` blocks; skip past them. One `<script>` block spans **543–866** containing five IIFEs. `:root` is at **27–46**.

Its reduced-motion block already resets `scroll-behavior` at **line 86** — **carry that rule over**; the mock omits it, and the global cap does not cover `scroll-behavior`.

The page makes **zero external requests** today. Keep it that way.

### R3. The false and overstated claims — must not survive

Ordered by severity. Verbatim page text → replacement.

**Safety-bearing, highest priority:**

1. **Line 438** — `An unattended agent reads on its own but turns every write into a human-approved proposal — fail-closed, so an unknown tool is a write.`
   **FALSE as a general claim.** The fail-closed classifier has exactly one production call site: `runtime/src/agentos_runtime/council/fanout.py:248-251`, and `council/gating.py:11-15` scopes it to `profile: react` members. The operator path never imports it.
   → `An unattended council member reads on its own but turns every write into a human-approved proposal — fail-closed, so an unknown tool is a write.`

2. **Line 473** — `…Every run is governed, bounded by a cycle cap, and a write pauses for a human. The scheduler is opt-in and off by default.`
   **The pause clause is FALSE by default.** `runtime/src/agentos_runtime/agent.py:113` — `interrupt_before = ["tools"] if settings.approval_tool_names else None`, and `AGENTOS_APPROVAL_TOOLS` ships empty (`config.py:43`, `compose.yaml:104`). No write reaches a decision point. On `AGENTOS_AGENT_PROFILE=deep` there is no HITL at any setting.
   → `Standing objectives the runtime pursues on its own: fire on an interval, a cron schedule, or an inbound webhook. Every run is governed and bounded by a cycle cap, and any tool you put on the approval list pauses for a human. The scheduler is opt-in and off by default.`
   **Keep no wording implying writes are classified or held automatically.**

**Honesty of the simulated panel:**

3. **Line 331** — the `Live` badge sits over a seeded `mulberry32(0x5EED)` simulation (the page's own comment at 693–694 says so).
   → change the status text to `Simulated`, or the eyebrow to `Governance chain — illustrative`. **One of them must ship.**

4. **Line 605** — `auditRows += 1;` is unconditional, so a 402 budget denial increments an "audit rows" tile for an entry the gateway never writes (`gateway/internal/server/rbac.go:74-96`).
   → `if (!(r.outcome === 'deny' && r.stop === 2)) auditRows += 1;`

5. **Line 623** — the feed row prints a token count and a dollar cost for **every** settled request, including denials the gateway records as zero. The page already knows better: line 606 only adds cost when `outcome === 'ok'`.
   → emit `model · org · tokens · $cost` only when `r.outcome === 'ok'`; for `deny`/`hold` emit `0 tok · $0.00` or drop both fields.

6. **Readings strip (359–362)** — fabricated numbers under the labels `requests governed`, `spend reserved`, `p95 latency`, `audit rows`.
   → mark the strip simulated in the same pass as #3, **or** drop the `spend reserved` and `p95 latency` tiles, which name real platform quantities with different real values. Fixing #4/#5 without this leaves the panel internally inconsistent.

**Capability overstatements:**

7. **Line 6 (meta description)** — `…and a complete audit trail.` → `…and an audit trail of everything served.` (Only seven kinds are audited; 401, 400 and 402 write nothing.)

8. **Line 384** — `…a complete per-key audit log — every model call, tool call, and verdict, attributable.` The audit schema has **no tool field** (`gateway/internal/store/store.go:130-140`); verdicts live in the runtime's council tables.
   → `OpenTelemetry traces, a bundled Langfuse profile, and a per-key audit log of every served model call — plus tool steps and council verdicts recorded in the runtime, all attributable.`

9. **Line 391** — `…a CI gate blocks a regression before it ships.` The eval workflow is opt-in per PR (`.github/workflows/evals.yml:58-61`, label `run-evals`), gates on an absolute threshold rather than a baseline, and runs against a mock model.
   → `Eval suites with an LLM judge measure agents against real cases, and an opt-in CI gate fails a build that scores below your threshold.`
   **Do not source replacement copy from `README.md:272` — it makes the same overstatement.**

10. **Line 398** — `…non-root images, dropped capabilities, and a hardened security context throughout.` `connectors/browser/Dockerfile` has no `USER` directive; in Compose only the sandbox sets `cap_drop`; `templates/postgres.yaml` has no `securityContext`.
    → `Docker Compose for a laptop, a Helm chart for a cluster — non-root images and, in Helm, a hardened security context on every service; the sandbox drops all capabilities on a read-only rootfs in both.`

11. **Lines 325 and 523** — the "runs air-gapped / at zero cost" pair. Shipped defaults are `anthropic/claude-sonnet-5` (`deploy/compose.yaml:100`) and Ollama is not a compose service. `README.md:141-146` says so in the repo's own words.
    → scope both to the configured case: air-gapped and $0 **when pointed at local Ollama**, which requires `AGENTOS_MODEL`, `AGENTOS_EMBED_MODEL`, `AGENTOS_JUDGE_MODEL` and `AGENTOS_OLLAMA_BASE_URL` overrides.

12. **Line 448** — `each on a different frontier model`. All five frontier council members ship `enabled: false` (`runtime/council.yaml:23-48`); the five that run are local.
    → scope to what ships enabled, or say "configurable to a different frontier model each".

**Two findings the adversarial re-check says NOT to change** — leave these verbatim:

- **Line 341** `Served calls and gate events` — rate-limit and guardrail events *are* gate events and *are* audited. A five-word sublabel asserts no universal quantifier.
- **Line 343** the chain caption — it is an **ordering** claim, and the code satisfies it (`server.go:500,517,524,530` all precede `:601`).

The remaining lower-severity items (#13–#22 in the fact sheet: kill-switch clause, council verdict pill, viz member model id, held tool name, CTA note, band claim, final CTA lede, copy-to-clipboard command, hero lead) are listed in full at `docs/superpowers/research/2026-07-28-landing-rebuild-factsheet.md:392-475`. **Read that file for their verbatim text and replacements.**

### R4. Mock code — what to port and what will silently break

The approved mock is at `docs/superpowers/research/2026-07-28-landing-mock.html`. Its JS is ES5-compatible (`var`/`function`), has no imports, and needs no build. **Port the logic; do not paste the file.**

**Traps, each of which fails silently:**

1. **The mono family name.** Mock says `"IBM Plex Mono"`; the target's embedded face is `"Plex Mono"`. Rename on port.
2. **Class collisions.** `.rail`, `.dot`, `.chain`, `.band`, `.wrap`, `.sub` all exist in **both** files with different meanings. Namespace every ported class or rename the target's.
3. **`.cdraw` base rule sets `stroke-dashoffset:1`.** Its reduced-motion override (`animation:none;stroke-dashoffset:0`) is load-bearing: without it **every council connector is invisible** under reduced motion, because the global cap does not reset `stroke-dashoffset`.
4. **`html{scroll-behavior:smooth}` is not reset in the mock.** Carry over the existing rule at `landing/index.html:86`.
5. **The mock has no viewport meta and no width breakpoints.** Both must be added — see Task 6.
6. **Drop the mock's `@font-face` block entirely.** The target already embeds Archivo; the mock's block is a `@@FONTS@@` placeholder whose substituting script does not exist on disk.

**The mock's own bugs — fix on port, do not reproduce:**

- Nav link `platform` targets `#cap`, which is the conveyor's caption div, not the capabilities section (`id="cap2"`).
- The hero meta says `CLEARED 96.4% · DENIED 3.6%` but the field draws **9 denials out of 128 strands = 7.0%**. Make the number match the drawing, or drop the number.
- `SEED 3573860127` implies a PRNG that does not exist in the mock. Drop it or make it real.
- `--raised2`, `--vdim`, `--live` are declared and never referenced; `H` is declared and never used.

**Gate-field constants** (mock lines 242–306): `W=1440, H=1000, GX=980, MY=520, N=128, SPREAD=430`. Stopped strands are `i % 29 === 7 || i % 29 === 19`; packet-carrying strands are `i % 17 === 8`. Strand fade-in begins at gradient stop `0.46` so nothing crosses the headline.

**Conveyor** (309–338): eight-frame `SCRIPT`, 2200 ms per frame, paused on `visibilitychange`, and under reduced motion it draws `SCRIPT[2]` — a rate-stage denial — and never starts its interval. That still is more informative than frame 0; keep that choice.

### R5. Verification tooling

Playwright with Chromium is available and working at `/home/iofahd/code/agentos/connectors/browser/.venv/bin/python`. Render the file over `file://`. **The page must be judged by rendering it, not by reading the source** — the docs work in this repo found a 38% dead band in a viewBox that ten arithmetic-only reviews had missed.

---

## File Structure

**Modified — one file only:**

| File | Responsibility |
|---|---|
| `landing/index.html` | The entire landing page: inlined fonts, tokens, all sections, all scripts. Stays a single self-contained artifact. |

Section order in the finished file: `<head>` + fonts + tokens → nav → hero (gate field) → governance chain → capability grid → council → quickstart → footer → one `<script>` block.

---

## Task 1: Correct every false claim on the existing page

No visual change. This lands first because it is worth shipping on its own, and because a rebuild that carries a false claim forward is worse than no rebuild.

**Files:**
- Modify: `landing/index.html`

**Interfaces:**
- Consumes: reference fact **R3**, and `docs/superpowers/research/2026-07-28-landing-rebuild-factsheet.md:274-495` for the items R3 summarises.
- Produces: a page whose every claim is true. Later tasks restyle this copy; they must not reintroduce a corrected claim.

- [ ] **Step 1: Read the full claim list**

Read `docs/superpowers/research/2026-07-28-landing-rebuild-factsheet.md`, section `## FALSE OR OVERSTATED CLAIMS`. It has 22 entries with verbatim text, evidence and exact replacements. R3 above covers the twelve most severe; the rest are in that file.

- [ ] **Step 2: Fix the two safety-bearing claims first**

Apply R3 items 1 and 2 — the "Safe by default" card (line 438) and the Operators lede (line 473). These are the two that could lead an operator to believe an unattended agent will hold writes for approval when it will not.

- [ ] **Step 3: Fix the simulated-panel honesty group together**

R3 items 3, 4, 5 and 6 are one coherent change and must land in the same pass — fixing the counter without labelling the panel leaves it internally inconsistent. Concretely:

```js
// line ~605
if (!(r.outcome === 'deny' && r.stop === 2)) auditRows += 1;
```

```js
// line ~623 — only a served request has tokens and a cost
text.textContent = r.outcome === 'ok'
  ? r.model + ' · ' + r.org + ' · ' + r.tokens + ' tok · ' + fmtMoney(r.cost)
  : r.model + ' · ' + r.org + ' · 0 tok · $0.00';
```

and change the `Live` badge text to `Simulated`.

- [ ] **Step 4: Fix the capability overstatements**

Apply R3 items 7–12, using the replacement wording verbatim.

- [ ] **Step 5: Apply the remaining lower-severity corrections**

Items #13–#22 from the fact sheet.

- [ ] **Step 6: Leave the two contested items alone**

Line 341 (`Served calls and gate events`) and line 343 (the chain caption) were flagged by one survey and cleared by the adversarial re-check. **Do not change them.** Note in your report that you deliberately left them.

- [ ] **Step 7: Verify no false claim survives**

Run:
```bash
cd /home/iofahd/code/agentos
grep -n 'complete audit trail\|every write into a human-approved proposal\|a write pauses for a human\|tool call, and verdict\|blocks a regression\|security context throughout' landing/index.html || echo "OK: none"
grep -n '>Live<' landing/index.html || echo "OK: no Live badge"
```
Expected: `OK` on both.

- [ ] **Step 8: Confirm the page still renders and animates**

Run:
```bash
cd /home/iofahd/code/agentos
connectors/browser/.venv/bin/python - <<'PY'
from playwright.sync_api import sync_playwright
with sync_playwright() as p:
    b=p.chromium.launch(); pg=b.new_context(viewport={"width":1440,"height":900}).new_page()
    errs=[]; pg.on("pageerror", lambda e: errs.append(str(e)))
    pg.goto("file:///home/iofahd/code/agentos/landing/index.html", wait_until="load")
    pg.wait_for_timeout(4000)
    print("js errors:", errs or "none")
    print("stages:", pg.eval_on_selector_all(".chain .stage .lbl","els=>els.map(e=>e.textContent.trim())"))
    b.close()
PY
```
Expected: no JS errors; six stages `Auth, Rate, Budget, Guardrail, Upstream, Audit`.

- [ ] **Step 9: Commit**

```bash
git add landing/index.html
git commit -m "fix(landing): correct every false and overstated claim on the page"
```

---

## Task 2: The token layer and type scale

The visual rebuild starts here. Everything downstream inherits these.

**Files:**
- Modify: `landing/index.html` (`:root` at 27–46, and the base type rules)

**Interfaces:**
- Consumes: **R1** (palette with the contrast corrections).
- Produces: the CSS custom properties and base type rules every later task uses. Class names introduced here must not collide with the mock's — see **R4** trap 2.

- [ ] **Step 1: Replace `:root`**

Replace the existing custom properties with **R1**'s, keeping any existing token the page still needs. Use `--faint: #78828E`, **not** the spec's `#646C77` — that value fails AA at the sizes it is used for.

Keep `--live`, `--ok`, `--hold`, `--deny` at exactly their current values; they are shared with the console and must not drift.

- [ ] **Step 2: Set the type scale**

```css
h1{font-size:118px;line-height:.9;letter-spacing:-.045em;font-weight:700}
h2{font-size:56px;line-height:1.02;letter-spacing:-.035em;font-weight:700}
h3{font-size:21px;letter-spacing:-.015em;font-weight:700}
body{font-size:16.5px;line-height:1.62}
.eyebrow{font-family:var(--mono);font-size:10px;font-weight:500;letter-spacing:.22em;text-transform:uppercase;color:var(--v2)}
```

**The mono family is `"Plex Mono"`.** Confirm against the `@font-face` block at the top of the file before writing any `font-family` declaration.

- [ ] **Step 3: Verify contrast holds**

Run the render script from Task 1 Step 8 and screenshot the page. Confirm the 10px eyebrows and small mono captions are legible against `--bg`. If any text still looks thin, raise its token rather than its opacity — opacity compounds unpredictably over the gradients used later.

- [ ] **Step 4: Commit**

```bash
git add landing/index.html
git commit -m "feat(landing): adopt the violet accent and the new type scale"
```

---

## Task 3: Nav and the hero gate field

The signature visual. This is the task that decides whether the page is impressive.

**Files:**
- Modify: `landing/index.html`

**Interfaces:**
- Consumes: **R1** tokens; **R4** gate-field constants and traps.
- Produces: `#hero` with the field SVG, and the nav. The conveyor in Task 4 reuses the stage names established here.

- [ ] **Step 1: Port the gate field**

Port the field IIFE from the mock (lines 242–306), using the constants in **R4**: `GX=980, MY=520, N=128, SPREAD=430`, stopped strands at `i % 29 === 7 || i % 29 === 19`, packets at `i % 17 === 8`, gradient fade-in at stop `0.46`.

Rules the port must preserve:
- Stopped strands terminate **only** at rate, budget or guardrail, in `--deny`, as short stubs near the gate.
- The six stage ticks are labelled and sit on opaque plates so the converging fan cannot strike through them.
- Packets are a `0.035` dash on a `pathLength=1` path, offset per strand — the house technique, not SMIL.
- No `Math.random`.

- [ ] **Step 2: Fix the mock's three bugs while porting**

Per **R4**: make the corner metadata's denial percentage match the drawing (9/128 = 7.0%) or drop the number; drop `SEED 3573860127`, which implies a PRNG that does not exist; and point the `platform` nav link at the capabilities section rather than the caption div.

- [ ] **Step 3: Add the illustration marker**

The corner metadata block must carry a visible `ILLUSTRATION · SCRIPTED SEQUENCE` line. This is non-negotiable per the spec: no fabricated governance metric appears unmarked.

- [ ] **Step 4: Write the hero copy**

Headline, sub-paragraph, two CTAs with keyboard badges, and the five-cell stat rail: `6` gate stages · `5` legacy systems as MCP tools · `0` provider keys in the runtime · `$0` five-model council local · `7` audit kinds.

**The `$0` cell must not restate the corrected claim R3 item 11.** Scope it — "$0 on local models" — rather than implying the shipped default is free.

- [ ] **Step 5: Add the reduced-motion still**

```css
@media (prefers-reduced-motion: reduce){
  .pk{display:none}
  html{scroll-behavior:auto}
}
```

The `scroll-behavior` reset is required — the global cap does not cover it, and the existing page already handles it at line 86.

- [ ] **Step 6: Render and look at it**

Capture at 1440 wide and open the PNG. Confirm: no strand crosses the headline, no stage label is struck through, the packets are visible on the hot strands, and the denial stubs read as a minority rather than a sea of red.

- [ ] **Step 7: Commit**

```bash
git add landing/index.html
git commit -m "feat(landing): rebuild the nav and hero on the gate field"
```

---

## Task 4: The governance chain section

**Files:**
- Modify: `landing/index.html`

**Interfaces:**
- Consumes: the conveyor logic described in **R4**.
- Produces: `#gov` with the six-stage conveyor.

- [ ] **Step 1: Port the conveyor**

Eight-frame fixed `SCRIPT`, 2200 ms per frame, `setInterval` paused on `visibilitychange`. Stages `auth → rate → budget → guardrail → upstream → audit`. Denials only at rate, budget, guardrail; the upstream failure is `--hold` amber.

- [ ] **Step 2: Namespace the classes**

`.chain`, `.dot`, `.rail`, `.sub` and `.band` collide with the existing page (**R4** trap 2). Prefix the ported ones — e.g. `.gc-chain`, `.gc-dot` — or rename the target's. Whichever you choose, grep afterwards to prove no selector is shared between two meanings.

- [ ] **Step 3: Carry the panel's honesty markers**

The panel header carries `illustration · scripted sequence`. Task 1 already changed the old `Live` badge; do not reintroduce it.

- [ ] **Step 4: Add the reduced-motion still**

The interval must **never start** under reduced motion, and the drawn frame is `SCRIPT[2]` — the rate-stage denial — because it teaches more than a clean pass.

- [ ] **Step 5: Verify the non-colour channel**

Per **R1**, `--live` and `--ok` are near-indistinguishable under common colour-vision deficiencies. If both appear in this section, the difference must also be carried by shape or label. If only `--deny` and `--hold` appear alongside violet, note that in your report and move on.

- [ ] **Step 6: Render and confirm the script**

Screenshot the section three times, four seconds apart, and confirm you catch different frames — a clean pass, a denial with later stages dark, and the amber upstream failure.

- [ ] **Step 7: Commit**

```bash
git add landing/index.html
git commit -m "feat(landing): rebuild the governance chain section"
```

---

## Task 5: Capability grid, council, and operators

**Files:**
- Modify: `landing/index.html`

**Interfaces:**
- Consumes: **R1** tokens; the corrected card copy from Task 1.
- Produces: `#cap2` (six cards), `#council`, and `#operators`.

- [ ] **Step 1: Build the capability grid**

**CORRECTED 2026-07-28 during execution — this step originally contradicted itself.** It named six card titles taken from the mock *and* instructed that the Observability, Evaluation and Deployment cards be preserved and re-verified — but those three are not among the six named. Ruling: **keep the page's existing six cards** (Engine / Observability / Evaluation / Deployment / Sandbox / Fleet) with Task 1's corrected copy, and add the numbered index. The mock's six topics already appear distributed across the page, its card bodies were never fact-checked against source the way the kept cards were, and "autonomy you can halt" would duplicate the Operators section three steps later.

Six cards, 3×2, hairline-separated, each with a numbered index and mono tags.

**Use the corrected copy from Task 1 verbatim.** The observability, evaluation and deployment cards were all overstated; their corrections are in R3 items 8, 9 and 10. Do not rewrite verified copy to hit a sentence count.

- [ ] **Step 2: Build the council fan-out**

Objective → five model-bound members → judge → verdict + dissent, drawn with `stroke-dashoffset` on reveal.

**Include the `.cdraw` reduced-motion override** (`animation:none;stroke-dashoffset:0`). Without it every connector is invisible under reduced motion — the base rule sets `stroke-dashoffset:1` and the global cap does not reset it. This is **R4** trap 3 and it fails silently.

- [ ] **Step 3: Keep the council copy honest**

All five frontier members ship `enabled: false`. Do not imply the shipped configuration runs five frontier models — see R3 item 12.

- [ ] **Step 4: Rebuild the operators section**

The spec's §6 requires the operators content to carry over. Its lede was corrected in Task 1 (R3 item 2) — **reuse that corrected copy verbatim** and do not re-derive it. Restyle it on the new tokens; keep the trigger list (interval, cron, webhook) and the "opt-in and off by default" clause, both of which are true.

- [ ] **Step 5: Render all three sections and look at them**

Confirm the grid's hairlines align, no card body overflows, and the council connectors actually draw. Then render again with `reduced_motion="reduce"` and confirm the connectors are **visible**, not blank.

- [ ] **Step 6: Commit**

```bash
git add landing/index.html
git commit -m "feat(landing): rebuild the capability grid, council and operators sections"
```

---

## Task 6: OSS credits, sovereignty band, quickstart, footer, and responsive behaviour

**Files:**
- Modify: `landing/index.html`

**Interfaces:**
- Consumes: everything above.
- Produces: the finished page.

- [ ] **Step 1: Rebuild the open-source credits and the sovereignty band**

Both are required to carry over by the spec's §6 and both are currently on the page (`landing/index.html`, the "Built on open source" and "Sovereign by design" sections). Restyle them on the new tokens.

The sovereignty band's air-gapped / zero-cost claims were corrected in Task 1 (R3 item 11) — **reuse that corrected copy verbatim.** Do not restate "runs air-gapped" or "$0" unscoped; both are true only when the stack is pointed at local Ollama, which the shipped defaults do not do.

- [ ] **Step 2: Build the quickstart block**

A terminal block with the real commands. Verify each against the `Makefile` before writing it — `make up` and `make smoke` exist; do not invent a target.

- [ ] **Step 3: Build the footer**

Brand, one-line positioning, four link columns.

- [ ] **Step 4: Add the viewport meta and the breakpoints**

The mock has **neither** (**R4** trap 5). Add:

```html
<meta name="viewport" content="width=device-width, initial-scale=1" />
```

and breakpoints at 1100, 900 and 620 px. At minimum: the hero headline steps down from 118px, the capability grid collapses from three columns to two then one, the gate field's right-hand fan is allowed to crop rather than squash, and the chain's stage labels drop before its nodes do — matching the console's own responsive order.

- [ ] **Step 5: Render at four widths**

1440, 1100, 900 and 620. Open every capture. Confirm no horizontal scrollbar at any width and no text overlapping a visual.

- [ ] **Step 6: Commit**

```bash
git add landing/index.html
git commit -m "feat(landing): add the quickstart, footer and responsive behaviour"
```

---

## Task 7: Whole-page verification

**Files:**
- Modify: `landing/index.html` (whatever the checks flag)

- [ ] **Step 1: Assert the invariants mechanically**

```bash
cd /home/iofahd/code/agentos
echo "--- no Math.random ---"; grep -n 'Math.random' landing/index.html && echo FAIL || echo OK
echo "--- no SMIL ---"; grep -nE '<animate|animateTransform|animateMotion' landing/index.html && echo FAIL || echo OK
echo "--- no light theme ---"; grep -n 'prefers-color-scheme' landing/index.html && echo FAIL || echo OK
echo "--- no external host ---"; grep -nE 'https?://' landing/index.html | grep -v 'www.w3.org' && echo FAIL || echo OK
echo "--- no build-step token ---"; grep -n '@@FONTS@@' landing/index.html && echo FAIL || echo OK
echo "--- mono family name ---"; grep -c '"IBM Plex Mono"' landing/index.html
echo "--- scroll-behavior reset ---"; grep -n 'scroll-behavior' landing/index.html
```
Expected: `OK` on every check; `"IBM Plex Mono"` count is **0**; `scroll-behavior` appears both as the base rule and inside a reduced-motion block.

- [ ] **Step 2: Assert no corrected claim regressed**

```bash
grep -nE 'complete audit trail|every write into a human-approved proposal|a write pauses for a human|tool call, and verdict|blocks a regression|security context throughout|>Live<|RBAC' landing/index.html || echo "OK: none"
```
Expected: `OK: none`. **`RBAC` must not appear in the chain** — it was removed in commit `0697a2b` and must not return.

- [ ] **Step 3: Verify zero network requests**

Render with Playwright and record every request. The only permitted URL is the `file://` page itself. Any http(s) request is a failure.

- [ ] **Step 4: Verify the reduced-motion stills**

Render with `reduced_motion="reduce"`. Confirm: packets hidden, conveyor showing a denial frame with no interval created, **council connectors visible**, and in-page nav links not smooth-scrolling.

- [ ] **Step 5: Look at the whole page**

Capture full-page at 1440 and 620 and read it top to bottom as a stranger. Fix anything that reads as broken, unfinished or overstated.

- [ ] **Step 6: Commit any fixes**

```bash
git add landing/index.html
git commit -m "fix(landing): address findings from the whole-page verification"
```

If Steps 1–5 found nothing, skip the commit and say so.

---

## Completion

When Task 7 passes: merge to `main` locally. Ask before pushing.
