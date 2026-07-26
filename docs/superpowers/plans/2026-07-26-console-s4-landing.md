# Landing S4 — Real-Preview Hero Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the landing page's governance-chain hero from a fixed CSS-timeline decoration into a **data-synced conveyor** driven by an in-file seeded simulation — synthetic requests advance Auth→RBAC→Budget→Rate→Audit, occasionally held/denied, ticking live readouts and prepending an audit-tail feed — while `landing/index.html` stays a single, self-contained, air-gapped file.

**Architecture:** A deterministic seeded PRNG + a small event state machine, added to the landing's trailing `<script>` IIFE, drive the EXISTING `.chain`/`.rail`/`.node .core`/`.stage` DOM via JS classes (replacing the decorative `@keyframes litcore`/`sweep` loop). The same event stream ticks the readings strip, a prepend-on-tick audit feed, a rolling inline-SVG throughput sparkline, and (lightly) rotates the two viz panels. `prefers-reduced-motion` freezes to a representative lit still. No new files, no external requests, no build step (the landing is a standalone static file, NOT part of the vite app).

**Tech Stack:** Vanilla HTML/CSS/JS in one file. Node available only for a determinism/self-containment smoke check.

## Global Constraints

From `docs/superpowers/specs/2026-07-26-agentos-console-track-a-design.md` §6 + the landing's own invariants.

- **Air-gap / self-contained (hard):** `landing/index.html` must remain a single file with ZERO external requests — no CDN, no remote fonts/scripts/images, no `fetch`/XHR/WebSocket. The seeded sim is entirely in-file. (Fonts are already inlined as data URIs on lines 17 & 24 — do NOT touch those blobs.)
- **Deterministic:** the sim uses a fixed seed so the demo sequence is reproducible (no `Math.random()`), which also keeps it Artifact-publishable and testable.
- **Reduced-motion:** under `prefers-reduced-motion: reduce`, freeze to a representative lit still (the existing rule at lines 181–183 already freezes the cores; extend it so the JS loop also does not run / paints one static frame).
- **Chroma is the governance signal set:** teal `--live` = in flight, plus `--hold` amber / `--deny` red for held/denied — reuse the page's existing tokens; introduce no new hue.
- **Preserve** the existing theme toggle (localStorage `agentos-theme`), the `.reveal` IntersectionObserver, `copyCmd`, and all copy/sections. This is additive to the `<script>`, plus targeted CSS for the JS-driven stage states.

## Nature of this work

The landing is a standalone file, so there is no vitest integration. Verification is: (1) **node smoke** — extract the sim's pure core and assert same-seed→same-sequence (determinism) and that a run produces the expected outcome mix; (2) **self-containment grep** — no `http://`/`https://`/`//cdn`/`fetch(`/`XMLHttpRequest` outside the inlined `data:` font URIs; (3) **syntax** — `node --check` on the extracted script; (4) **visual review** — the human partner (the controller will publish the updated page as an Artifact). Candidate code below is concrete; refine motion feel within the constraints.

## File Structure
- Modify: `landing/index.html` — CSS for JS-driven `.stage.active`/`.stage.hold`/`.stage.deny` + the readings/feed/sparkline markup; the trailing `<script>` gains the sim.
- No other files.

---

### Task 1: Seeded sim engine + chain conveyor

**Files:** Modify `landing/index.html` (CSS around lines 154–189; markup lines 310–326; script lines 513–540).

Replace the decorative infinite chain animation with a JS-driven conveyor.

- [ ] **Step 1: Make the chain JS-driven, not CSS-timeline.**
  - In the markup (lines 318–322), REMOVE the static `data-lit` attribute from the 5 `.stage` divs (the JS will add state classes).
  - In the CSS: remove/neutralize `@keyframes litcore` + the per-stage `animation-delay` rules (174–178) and the `.stage[data-lit] .node .core { animation: litcore … }` (172). Keep `.rail`/`.fill`. Add JS-driven state classes:
    ```css
    .stage .node .core { transition: opacity .18s ease, transform .18s ease; }
    .stage.active .node { border-color: var(--live); }
    .stage.active .node .core { opacity: 1; transform: scale(1); background: var(--live); }
    .stage.hold  .node { border-color: var(--hold); }
    .stage.hold  .node .core { opacity: 1; transform: scale(1); background: var(--hold); }
    .stage.deny  .node { border-color: var(--deny); }
    .stage.deny  .node .core { opacity: 1; transform: scale(1); background: var(--deny); }
    ```
  - Extend the reduced-motion block (181–183) so JS-lit cores still show a representative still and no transition runs.

- [ ] **Step 2: Add the seeded sim to the `<script>` IIFE.** Insert (keeping theme toggle / IntersectionObserver / copyCmd intact):

```js
// --- Governance-chain live preview: a deterministic in-file simulation.
// No Math.random / no network — the demo sequence is reproducible.
(function () {
  var panel = document.querySelector('.chain-panel');
  if (!panel) return;
  var stages = Array.prototype.slice.call(panel.querySelectorAll('.stage')); // Auth,RBAC,Budget,Rate,Audit
  if (stages.length !== 5) return;
  var reduce = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  function mulberry32(a) {
    return function () {
      a |= 0; a = (a + 0x6D2B79F5) | 0;
      var t = Math.imul(a ^ (a >>> 15), 1 | a);
      t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  }
  var rand = mulberry32(0x5EED);
  var MODELS = ['qwen3.8-max', 'deepseek-v4', 'kimi-k3', 'llama-4-70b'];
  var ORGS = ['acme', 'globex', 'initech', 'umbrella'];

  // Decide a request's fate: index of the stage it stops at (0..4) and outcome.
  // Most clear all 5 (ok); some are held (amber) or denied (red) mid-chain.
  function nextRequest() {
    var roll = rand();
    var stop, outcome;
    if (roll < 0.8) { stop = 4; outcome = 'ok'; }
    else if (roll < 0.92) { stop = 2 + Math.floor(rand() * 2); outcome = 'hold'; } // Budget/Rate
    else { stop = 1 + Math.floor(rand() * 3); outcome = 'deny'; }                   // RBAC/Budget/Rate
    return {
      stop: stop, outcome: outcome,
      model: MODELS[Math.floor(rand() * MODELS.length)],
      org: ORGS[Math.floor(rand() * ORGS.length)],
      tokens: 200 + Math.floor(rand() * 1800),
      cost: Math.round((0.4 + rand() * 6) * 100) / 100,
      latency: 120 + Math.floor(rand() * 900),
    };
  }

  function clearStages() { stages.forEach(function (s) { s.classList.remove('active', 'hold', 'deny'); }); }

  var req = null, at = -1;
  function tick() {
    if (!req) { req = nextRequest(); at = -1; clearStages(); }
    at += 1;
    if (at <= req.stop) {
      var stage = stages[at];
      var terminal = at === req.stop && req.outcome !== 'ok';
      stage.classList.add(terminal ? req.outcome : 'active');
      if (at === req.stop) {
        // request settled — emit it, then start a fresh one next tick
        onSettled(req);
        req = null;
      }
    }
  }

  if (reduce) {
    // one representative still: a cleared request lighting all five, no loop.
    stages.forEach(function (s) { s.classList.add('active'); });
    onSettled({ stop: 4, outcome: 'ok', model: MODELS[0], org: ORGS[0], tokens: 1024, cost: 2.4, latency: 380 });
    return;
  }
  setInterval(tick, 520);
  tick();

  // onSettled is defined in Task 2 (readouts + feed). For Task 1, stub it:
  window.__agentosOnSettled = window.__agentosOnSettled || function () {};
  function onSettled(r) { window.__agentosOnSettled(r); }
})();
```

- [ ] **Step 3: Verify** — extract the `<script>` and run node checks:
  - `node --check` on the script (syntax).
  - Determinism: `mulberry32(0x5EED)` produces the SAME first-N sequence on two runs; a 500-request run yields ~80% ok / ~12% hold / ~8% deny (±a few %). Write a tiny `/tmp/...smoke.mjs` that inlines `mulberry32` + `nextRequest` and asserts both.
  - Self-containment: `grep -nE "https?://|//cdn|fetch\(|XMLHttpRequest|new WebSocket" landing/index.html` returns only the inlined `data:` font URIs (no network).

- [ ] **Step 4: Commit** — `git add landing/index.html && git commit -m "feat(landing): seeded governance-chain conveyor sim"`

---

### Task 2: Live readouts + audit-tail feed + throughput sparkline

**Files:** Modify `landing/index.html` (readings markup ~line 328; add a feed + sparkline near the chain panel; extend the sim's `onSettled`).

- [ ] **Step 1: Readings strip → live metrics.** Give the four `.readings` tiles ids and drive them from settled requests: `requests governed` (count++), `spend reserved` (`$` sum of ok costs), `p95 latency` (rolling p95 over the last ~40 latencies), `audit rows` (count++). Keep the tile labels; only the numbers tick. (Keep the sovereignty copy elsewhere on the page — this strip becomes the live instrument.)

- [ ] **Step 2: Audit-tail feed.** Add a compact list under the chain caption (or beside the readings): on each settled request prepend a row — `mono` `model · org · N tok · $cost · VERDICT` where VERDICT is a colored chip (`--ok`/`--hold`/`--deny`), cap at ~6 rows, and animate the row-in (respect reduced-motion). Reuse the page's existing type/spacing tokens; monochrome except the verdict chip.

- [ ] **Step 3: Throughput sparkline.** A small inline-SVG polyline (no library) over a rolling window of per-few-seconds request counts, redrawn on tick. Single accent stroke (ink), faint baseline; ~24 points.

- [ ] **Step 4: Wire `onSettled`.** Replace the Task 1 stub: define `window.__agentosOnSettled = function (r) { …update readings, prepend feed row, push sparkline… }` BEFORE the sim IIFE runs (or have the sim call a real `onSettled` defined in the same scope). Ensure the reduced-motion path (which calls `onSettled` once) renders a sensible static readout + one feed row + a flat/representative sparkline.

- [ ] **Step 5: Verify** — `node --check`; self-containment grep clean; determinism unaffected; reduced-motion renders a representative still (no interval). Commit `feat(landing): live readouts, audit-tail feed, throughput sparkline`.

---

### Task 3: Viz-panel motion + final verification

**Files:** Modify `landing/index.html` (viz panels ~415 & ~440; final checks).

- [ ] **Step 1: Council panel (~426) — quorum rotation.** On a slow interval (seeded), cycle which member rows read "answered" vs the occasional "dissent", landing on "quorum met" — a light, believable loop reusing the existing rows/pills. Reduced-motion → the existing static still.
- [ ] **Step 2: Operators panel (~451) — run-history prepend.** On a slow interval, prepend a synthetic run row (name · completed/needs-approval pill) capped at the existing count. Reduced-motion → static still.
- [ ] **Step 3: Final verification:**
  - `node --check` on the extracted script; the page opens with no console errors (manual).
  - Self-containment: `grep -nE "https?://|fetch\(|XMLHttpRequest|WebSocket|src=\"http" landing/index.html` → only inlined `data:` font URIs.
  - Reduced-motion: with the media query on, no `setInterval` animation runs; the chain/readings/feed/panels show a representative lit still.
  - The theme toggle, `.reveal` observer, and `copyCmd` still work.
- [ ] **Step 4: Commit** — `feat(landing): animate council & operators viz panels`

---

## Self-Review

- **Spec coverage (§6 Option A):** seeded in-file sim → Task 1; chain becomes a data-synced conveyor (per-stage `data-lit`/state as the request arrives, hold/deny flashes) → Task 1; live readouts + audit-tail feed + throughput sparkline → Task 2; the two viz panels animated → Task 3; deterministic + air-gapped + reduced-motion-safe throughout. Options B (live gateway fetch) and C (recorded trace) are explicitly NOT built (Option A baseline only), keeping the file self-contained.
- **Placeholder scan:** Task 1 ships concrete sim code; Tasks 2–3 specify exactly which elements update and how, with the reduced-motion + determinism + self-containment gates named. No TBD.
- **Consistency:** `onSettled`/`__agentosOnSettled` is the single seam between the sim (Task 1) and the readouts/feed (Task 2); Task 3 reuses the same `rand`/interval discipline.
- **Honest verification:** no vitest (standalone file); gated on node syntax + a determinism/outcome-mix smoke, a self-containment grep, reduced-motion, and a human visual review via a published Artifact.
