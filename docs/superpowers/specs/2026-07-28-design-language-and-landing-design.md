# Design Language + Landing Page Rebuild

- **Date:** 2026-07-28
- **Status:** Approved (design)
- **Repo:** `ifahad/agentos` (private on GitHub; `origin/main` current)
- **Cycle:** 1 of 3 in a platform-wide UI/UX overhaul
- **Relates to:** [[2026-07-27-platform-docs-and-console-docs-tab]] (shipped — established the illustration-vs-instrument rule this spec extends)

## 1. Problem

The platform is well built and does not look it. The owner's words: *"washed and not beautiful. No nice interaction and visualization and animation. just one color across the board. it doesn't feel alive and dynamic."* The benchmark named: Langfuse, LangGraph, nousresearch.com.

Diagnosis, from reviewing every console page and the landing page:

- **The founding rule is the cause.** `styles.css:7-12` says *"chroma is reserved for machine state… Do not add a brand color; it would make the signals lie."* It is a good rule that was applied too widely: with no brand register at all, `--accent` resolves to ink and the entire product is graphite on graphite.
- **It is not "one colour" that is wrong.** nousresearch.com is a single hue across an entire site and reads as vivid, because the hue is *saturated and committed*. AgentOS's single colour is a desaturated grey with nothing to push against.
- **The scale is timid.** The largest type on the landing page is ~40px; on the console, 24px. Every reference site opens at 90–120px.
- **Nothing moves with intent.** The landing page has a chain simulation and a sparkline; the console has page transitions. Neither reads as alive.
- **The console renders data without ranking it** — four stat tiles of equal weight, a "Spend by key" chart that is eight bars of `$0.00` on a local-Ollama deployment, `acme-app` listed three times. This is information design, not styling, and is deferred to cycle 3.

## 2. Goals

1. Establish a **design language** with a committed accent, a real type scale, a motion vocabulary, and a signature visual — without breaking the machine-state colour semantics the governance instruments depend on.
2. **Rebuild the landing page** on that language so it showcases what the platform actually does.
3. Prove the language on one page before thirteen console pages commit to it.

## 3. Non-goals

- No console changes in this cycle (cycles 2 and 3).
- No new runtime dependency, no build step for the landing page, no framework.
- No external requests: fonts stay self-hosted, no CDN, no analytics, no embedded video.
- No light theme. The console is dark-only by design and the landing page follows it.
- No fabricated live metrics (see §5.4).

## 4. Scope and decomposition

Platform-wide overhaul, cut into three cycles, each its own spec → plan → build:

| Cycle | Deliverable | Status |
|---|---|---|
| **1** | **Design language + landing page rebuild** | **this spec** |
| 2 | Console visual system — tokens, primitives, icons, motion applied to all 13 pages | later |
| 3 | Console information design — hierarchy, zero/empty states, what each page leads with | later |

## 5. The design language

### 5.1 Palette

Two registers that must never blur into each other.

**Structure register** — surfaces, type, borders, and the brand accent:

| Token | Value | Use |
|---|---|---|
| `--bg` | `#07080B` | page ground |
| `--raised` | `#0E1015` | panels |
| `--raised2` | `#151821` | nested surfaces |
| `--line` | `rgba(255,255,255,.06)` | hairlines |
| `--line2` | `rgba(255,255,255,.12)` | emphasised hairlines, dashed rules |
| `--ink` | `#F2F4F7` | primary text |
| `--dim` | `#98A2AE` | secondary text |
| `--faint` | `#646C77` | tertiary, mono labels |
| `--v` | `#7A5AF8` | **the accent** — CTAs, marker fills, the gate |
| `--v2` | `#A78BFA` | accent text, eyebrows, active labels |
| `--v3` | `#C7B6FF` | accent display type, brightest strands |

**State register** — unchanged from the console, and reserved:

| Token | Value | Meaning |
|---|---|---|
| `--live` | `#5ad1c4` | in flight right now |
| `--ok` | `#6cc48f` | completed / allowed |
| `--hold` | `#e3a851` | held, awaiting a human |
| `--deny` | `#e2685f` | denied / failed / over budget |

**The rule that replaces the old one.** Chroma is no longer reserved outright; it is *partitioned*. Violet means "this is interface" — headings, controls, links, structure. The four state hues mean "this is a machine outcome" and appear **only** where a real outcome is being depicted. A violet button and a red chain node can sit on the same screen without ambiguity because they speak different languages. Violet is deliberately chosen for maximum separation from all four state hues; no accent from the teal/green/amber/red families may be introduced.

### 5.2 Type

Both faces are already in the repo and must not be re-fetched from a CDN — the console serves them from `console/src/fonts/`, and the landing page carries them base64-inlined in its own `@font-face` blocks (see §6).

| Role | Face | Size | Tracking |
|---|---|---|---|
| Hero display | Archivo 700 | 118px / 0.9 line-height | −0.045em |
| Section display | Archivo 700 | 56px / 1.02 | −0.035em |
| Card heading | Archivo 700 | 21px | −0.015em |
| Body | Archivo 400 | 16.5px / 1.62 | — |
| Small body | Archivo 400 | 13.5px / 1.6 | — |
| Eyebrow | IBM Plex Mono 500 | 10px, uppercase | 0.22em |
| Machine values | IBM Plex Mono 400 | 10–13px | 0.04–0.12em |

The scale jump is the point: today's landing page tops out around 40px. Mono is reserved for machine-produced or machine-shaped text — ports, endpoints, env vars, counts, nav labels, button labels — never for prose.

### 5.3 Motion

- Easing is `cubic-bezier(.2,.8,.2,1)` throughout, matching the console's `--ease`.
- **Scroll reveal:** 0.7s fade + 14px rise, staggered 60ms within a group, fired once by `IntersectionObserver` at 0.15 threshold.
- **Hero packet loop:** 5.5s linear, `stroke-dasharray` on a `pathLength=1` route — the house technique from `console/src/charts`. Never SMIL, never a JS animation loop.
- **Chain conveyor:** 2200ms per frame, eight-frame fixed script, `setInterval`, paused on `document.hidden`.
- **Hover:** 0.18s, translate 1px and deepen the shadow. Nothing else moves on hover.
- **No `Math.random` anywhere.** Every sequence is a fixed script or a pure function of an index, so two people looking at the same frame see the same thing.
- **Reduced motion:** every looping animation ships an explicit `@media (prefers-reduced-motion: reduce)` still. The packet layer is hidden outright; the conveyor never starts its interval and renders a denial frame, which is the frame that teaches the most. A global cap alone is not sufficient — it sets `animation-iteration-count: 1`, which freezes a loop at an arbitrary point rather than a chosen one.

### 5.4 Illustration vs instrument

The console's governance chain is evidence-driven by contract: it lights only what recorded audit rows prove. The landing page's chain is a scripted illustration. Both are legitimate; conflating them is not.

**Rules:**

1. Any landing-page visual depicting governance carries a visible `ILLUSTRATION · SCRIPTED SEQUENCE` marker on the visual itself, not merely in surrounding prose, so the distinction survives a screenshot.
2. **No fabricated metric may appear without that marker.** The hero's `CLEARED 96.4% · DENIED 3.6%` readout is illustrative and marked. The landing page must not hold a live connection to a gateway.
3. The landing chain **may** light cleared stages in violet, where the console deliberately leaves them graphite. This is an intentional divergence: the console's chain is an instrument that should stay quiet when nothing needs attention; the landing page's job is to explain the mechanism. Recorded here so it is a decision rather than a drift.
4. Stage names and order must match `CHAIN_STAGES` in `console/src/lib/chain.ts` — `auth → rate → budget → guardrail → upstream → audit`. The landing page previously drew `Auth, RBAC, Budget, Rate, Audit` under the caption "Every call recorded", which was false in three ways; that was corrected in commit `0697a2b` and must not regress.
5. A denial may only be depicted at a stage where the gateway can actually deny: **rate (429), budget (402), guardrail (400)**. An upstream failure is amber, not red. Nothing may depict a denial at `auth` or `audit`.

### 5.5 The signature visual — the gate field

The one image the platform should be recognisable by, and the reason it is not decoration:

- Requests enter as strands from the left, spread over a sine-eased vertical distribution so density peaks at the centre.
- Every strand narrows to a single point — the gate — rendered as six horizontal ticks labelled with the six stage names, with a vertical bar and a radial halo.
- Cleared strands fan out to the right toward providers and tools.
- Roughly 7% of strands **stop**, in `--deny`, as short stubs terminating at the stage that stopped them, with a 2px dot at the terminus. This is the honest part: not every request survives, and the picture says so.
- A minority of strands carry a travelling packet — a `0.035` dash on a `pathLength=1` path, offset per strand so arrivals are staggered without randomness.
- Geometry and verdicts are pure functions of the strand index. `i % 29` selects the stopped strands; `i % 17` selects the packet-carrying ones.

Strands must fade in only after the text column ends (gradient stop at 0.46) so they never cross the headline, and stage labels sit on opaque plates so the converging fan cannot strike through them.

## 6. The landing page

Single self-contained `landing/index.html`. No build step, no dependencies, zero external requests.

**Fonts stay base64-inlined in the file**, which is how the page already works — `@font-face` blocks at the top of `landing/index.html` carry Archivo and Plex Mono as `data:font/woff2` URIs. This is what makes the page a genuinely single artifact with no asset directory and no build, and it must be preserved: do not switch to relative font paths, and do not fetch from a CDN.

Sections in order:

1. **Nav** — fixed, blurred, dashed bottom rule. Mono labels. Two CTAs with keyboard-shortcut badges (the Langfuse device, and honest here because the console really does have a ⌘K palette).
2. **Hero** — the gate field, 118px display headline with a violet marker on the payoff line, sub-paragraph, two CTAs, corner machine-metadata block with its illustration marker, and a five-cell stat rail on a dashed rule: `6` gate stages · `5` legacy systems as MCP tools · `0` provider keys in the runtime · `$0` five-model council local · `7` audit kinds.
3. **Governance chain** — "Governance isn't a setting. It's the architecture." The six-stage conveyor running its eight-frame script, panel-framed, captioned with the current frame's outcome, marked as an illustration.
4. **Capability grid** — six cards, 3×2, hairline-separated: governed model access · deep agents · legacy systems as tools · sandboxed execution · eval-gated self-improvement · autonomy you can halt. Each with a numbered index, two-sentence body, and mono tags.
5. **Multiverse council** — "Many models. One verdict. Honest dissent." Objective → five model-bound members → judge → verdict + dissent, drawn with `stroke-dashoffset` on reveal.
6. **Quickstart** — a terminal block with the real commands (`make up`, `make smoke`) and the three health URLs.
7. **Footer** — brand, one-line positioning, four link columns.

**Content carried over from the current page** (it is accurate and hard-won, and must not be lost): the sovereign/self-hosted positioning, the open-source credits, the operators and council sections, and the honest note about local model behaviour. **Dropped:** the RBAC stage (corrected), and any claim the corrected chain contradicts.

## 7. Constraints

- **Private repo, local-first.** Commit locally; push only when asked.
- **Air-gapped page:** no CDN, no analytics, no external font, no remote image, no embedded third-party frame. Verified by loading the built page with request interception and asserting zero non-same-origin requests.
- **No new dependency and no build step** for the landing page. It stays a single hand-authored HTML file.
- **Determinism:** no `Math.random`.
- **Accessibility:** decorative SVG is `aria-hidden`; the conveyor is `aria-live="off"` because announcing every frame would be noise; every interactive element remains keyboard-reachable; text contrast on `--dim` against `--bg` must meet WCAG AA at its rendered size.
- **The corrected chain stays corrected.** Any regression to `Auth, RBAC, Budget, Rate, Audit` is a defect.

## 8. Verification

No unit tests — the landing page is a static file with no test harness today, and this spec does not add one.

- Render the page headlessly at 1440, 1024 and 720 and inspect the captures: no horizontal overflow, no text overlapping a visual, no strand crossing the headline, no label struck through.
- Assert zero external network requests.
- Assert no `Math.random`, no SMIL, no `prefers-color-scheme: light` rule.
- Assert the six stage labels and their order match `CHAIN_STAGES`, and that no denial is depicted at `auth` or `audit`.
- Load with `prefers-reduced-motion: reduce` emulated and confirm a deliberate still: packets hidden, conveyor showing a denial frame, no interval created.
- Confirm every `make` target and URL quoted in the quickstart exists.

## 9. Risks

- **The state palette losing its meaning.** Mitigated by §5.1's two-register rule and by §5.4's constraint that state hues appear only on real outcomes. The riskiest moment is cycle 2, when violet reaches the console; this spec fixes the rule before that happens.
- **A beautiful lie.** A governance platform whose front page invents governance numbers is worse than a plain one. Mitigated by §5.4's marker requirement, which is non-negotiable.
- **Motion that annoys.** Two looping animations on one page is the ceiling; both pause when the tab is hidden and both have authored reduced-motion stills.
- **Cycle 2 scope.** Thirteen console pages is a much larger surface than one landing page, and the language will need adjustment under real data density. That is the reason for proving it here first.
