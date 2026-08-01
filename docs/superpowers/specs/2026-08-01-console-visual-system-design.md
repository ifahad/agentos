# Console visual system — Cycle 2

- **Date:** 2026-08-01
- **Cycle:** 2 of 3 in the platform-wide UI/UX overhaul
- **Predecessor:** [`2026-07-28-design-language-and-landing-design.md`](2026-07-28-design-language-and-landing-design.md) (Cycle 1 — design language + landing page, shipped)
- **Successor:** Cycle 3 — console information design (not yet specced)

## 1. Why

Cycle 1 established a design language and proved it on one page. The console —
thirteen pages, 3,138 lines of CSS — still looks the way it did before that work
started: graphite, monochrome, no accent. Moving between the landing page and the
console currently reads as moving between two products.

Cycle 2 propagates the language into the console. It changes how the console
looks. It does not change what any page says, leads with, or orders.

## 2. Starting position

Measured, not assumed:

- **The console is already ~90% token-driven.** Only **31** hardcoded colour
  literals exist across all page and component CSS combined, against 300+
  `var(--token)` references. A token-block change therefore restyles most of the
  surface for free.
- **Primitives already exist.** `console/src/ui/` ships Button, Card/Panel, Stat,
  Table, Badge, Field, Tabs, Modal, Toast, Skeleton/EmptyState, icons and a motion
  preset module — all built during Track A. 18 of 29 page/component files already
  import from `../ui`.
- **Icons are done.** `ui/icons.tsx` (468 lines) provides `Icon`, `BrandMark` and
  `StateIcon`, used by 8 files.
- **Motion is mostly adopted already.** `ui/motion.ts` is re-exported from the
  `../ui` barrel, which eight files already consume: App, CommandPalette,
  LiveList, Docs, Improve, Keys, Overview, Playground. Three files hand-roll
  their own variants and sit outside the consolidated system — Sidebar, Chain,
  GovernanceChainVisual — so the work is two gaps, not eleven files. (Earlier
  grepping missed barrel imports and undercounted existing adoption.)
- **The console has no light theme.** Zero occurrences of `data-theme` or
  `prefers-color-scheme` in any console CSS. The landing ships both.

Consequence: of the four deliverables the Cycle 1 spec named for Cycle 2
("tokens, primitives, icons, motion"), icons are complete and motion is
consolidation rather than authoring. The real work is the token layer, the
primitive re-skin, and propagation.

## 3. Decisions

| # | Decision | Rationale |
|---|---|---|
| 1 | **Restrained partition.** Violet takes interface chrome; it is banned from any surface depicting machine state. | Keeps "a coloured thing on an instrument surface means machine state" true where an operator actually reads it. |
| 2 | **Both themes ship.** Light theme, `prefers-color-scheme` default, `localStorage` toggle. | Matches the landing exactly; cost is bounded by the 31-literal measurement. |
| 3 | **Adopt the near-black ground** `#07080B`. | Continuity from landing to console, and more contrast headroom for both violet and the four state hues. |
| 4 | **Visual only.** Structural problems are logged to a Cycle 3 backlog, never fixed here. | Keeps rendering sufficient as verification, and keeps Cycle 3 a real piece of work rather than leftovers. |
| 5 | **The partition is enforced by a test**, not a convention. | The existing dead `--accent: var(--text)` alias is what an unenforced convention decays into. |

### 3.1 Explicitly superseded

`console/src/styles.css:6-12` currently reads, in the file header:

> THE RULE THAT DEFINES THIS DESIGN: chroma is reserved for machine state. There
> is no decorative accent. … Do not add a brand color; it would make the signals
> lie.

Cycle 1 §5.1 replaced this rule with the partition. This spec applies that
replacement to the console. **The header must be rewritten as part of the work** —
leaving it would make the file contradict its own contents.

The old rule's concern is not dismissed; it is answered by decision 5. Chroma
still means machine state *on the surfaces where machine state is depicted*, and
a test now guarantees it.

## 4. The token layer

### 4.1 Contract

`--v` measures **4.43:1** against the new `#07080B` ground — below the 4.5:1 floor
for normal text. It passes as a *fill* (white label on it is 4.52:1).

> **`--v` is fill-only. It may never appear as a `color:` value.**
> `--v2` is the accent text token.

This is a contract, not a guideline; §6 enforces it.

### 4.2 Dark ground

Adopted verbatim from Cycle 1 §5.1.

| Token | Value | On ground | Role |
|---|---|---|---|
| `--bg` | `#07080B` | — | page ground (was `#121517`) |
| `--raised` | `#0E1015` | — | panels (was `#191D20`) |
| `--raised-2` | `#151821` | — | nested surfaces (was `#20252A`) |
| `--v` | `#7A5AF8` | 4.43:1 | **fill only** — buttons, active bars, focus rings |
| `--v2` | `#A78BFA` | 7.36:1 | accent text, eyebrows, active nav labels |
| `--v3` | `#C7B6FF` | 11.03:1 | display type |

State hues (`--live #5ad1c4`, `--ok #6cc48f`, `--hold #e3a851`, `--deny #e2685f`)
are unchanged.

### 4.3 Light ground

Computed, hue-locked to **252.2°** at saturation **0.637** — the HSV decomposition
of `--v #7A5AF8`. The lightness ladder **inverts**: on a dark ground emphasis gets
brighter, on a light ground it gets darker.

| Token | Value | White on it | On `--bg` | On `--raised` |
|---|---|---|---|---|
| `--v` | `#6D50DE` | 5.45:1 | 4.53:1 | 5.00:1 |
| `--v2` | `#5F46C2` | 6.68:1 | 5.56:1 | 6.13:1 |
| `--v3` | `#503BA3` | 8.41:1 | 6.99:1 | 7.71:1 |

**Derivation of `--v` light.** Holding hue and saturation, value was walked down
from 0.970 until the colour cleared 4.5:1 both as a fill under a white label and
as text on `--bg #eceae4`. The first passing value is 0.869 → `#6D50DE`. Anything
darker is unnecessary and drains the hue; the source `#7A5AF8` itself fails at
3.76:1 on the light ground and cannot be reused. `--v2` at value 0.760 and `--v3`
at 0.640 continue the ladder downward.

Surfaces and light-adapted state hues come from Cycle 1 §5.6, already tuned:
`--bg #eceae4`, `--raised #f6f5f1`, `--raised-2 #ffffff`,
`--border rgba(0,0,0,.10)`, `--border-strong rgba(0,0,0,.18)`, `--text #1a1d1f`,
`--text-dim #565d63`, `--text-faint #8a9198`, `--live #12897c`, `--ok #2f8f57`,
`--hold #a8721c`, `--deny #bb4038`.

### 4.4 Mechanics

- `:root` holds the dark values.
- `:root[data-theme="light"]` overrides them.
- `@media (prefers-color-scheme: light)` sets the default for a user who has not
  chosen.
- A toggle persists the choice in `localStorage`.

Same construction as `landing/index.html`, so the two surfaces behave identically.

**Naming.** Console names (`--text`, `--text-dim`, `--text-faint`, `--border`,
`--border-strong`, `--raised-2`) stay. The landing's names (`--ink`, `--dim`,
`--faint`, `--line`, `--line2`, `--raised2`) are added as aliases pointing at
them, so a rule copied from either file works in the other. Renaming 300+
existing references is churn with no visual payoff.

## 5. The partition

**Violet is permitted on:** sidebar active state, focus rings, primary buttons,
links, tab underlines and active pills, headings, section eyebrows, text
selection, table sort indicators.

**Violet is banned from:** `components/Chain.css`, `pages/Audit.css`,
`charts/charts.css`, and every state-badge rule. These depict machine outcomes
and keep graphite-plus-signal-hue exactly as they are today.

The landing page's chain lights cleared stages in violet; the console's does not.
That divergence is deliberate and was already recorded in Cycle 1 §5.4 rule 3 —
the console's chain is an instrument that stays quiet when nothing needs
attention, the landing's explains a mechanism. Cycle 2 does not revisit it.

## 6. Guard test

A vitest alongside the existing suite, reading the CSS files as text:

1. **Surface ban.** Fails if `--v`, `--v2` or `--v3` appears anywhere in
   `components/Chain.css`, `pages/Audit.css` or `charts/charts.css`; and, in
   `styles.css`, fails if a violet token appears inside any rule whose selector
   contains `.badge`.
2. **Fill-only contract.** Fails if `--v` appears as the value of a `color:`
   declaration in any console CSS file.

Two assertions. Both are the kind of rule that otherwise decays into a comment
nobody reads.

## 7. Primitives

Twelve components in `console/src/ui/`.

| Primitive | Change |
|---|---|
| `Button` | `.btn.primary` becomes a violet fill with a white label — today it is a white button with `color:#0a0b0d` (one of the 31 literals). `danger` stays `--deny`. `ghost` gains a violet hover border. |
| `Tabs` | active pill and underline → violet |
| `Field` | focus ring → violet (currently ink) |
| `Table` | sort indicator → violet; header and hover repoint via tokens |
| `Modal` | focus ring and primary action inherit from `Field`/`Button` |
| `Card`/`Panel` | ground and border repoint only — already token-driven |
| `Skeleton`/`EmptyState` | ground repoint; the action button inherits `Button` |
| `Toast` | unchanged — success and error are signal hues |
| `Stat` | unchanged — readings stay ink, sparkline stays signal |
| `Badge` | **unchanged.** All eleven variants (`chat`, `embeddings`, `guardrail_flag`, `guardrail_block`, `pass`, `fail`, `passed_evals`, `failed_evals`, `approved`, `denied`, `inactive`) are machine states. |
| `StateIcon` | unchanged, by rule |
| `Icon` | unchanged — inherits `currentColor`, so it follows its context |

That `Badge` needs no change is the partition validating itself: the primitive
whose entire job is depicting machine state has no violet in it.

## 8. Motion

**No new animation is authored.** The work is routing three files that
hand-roll framer-motion variants (Sidebar, Chain, GovernanceChainVisual) through
the existing `ui/motion` presets, joining the eight files that already use them
via the `../ui` barrel. Entrance, stagger and page transition become one system.

Reduced motion is already handled — globally in `styles.css` for CSS keyframes
and transitions, and per-component via `useReducedMotion()`. That stays.

**No addition either.** This spec originally proposed making the sidebar active
indicator a sliding `layoutId` bar. Reading `Sidebar.css:44` shows it already is
one — Track A built it, moved by framer-motion under `layoutId="nav-pill"`. Only
its colour changes, and that belongs to the primitive re-skin (§7), not here.

## 9. Rollout

Subagent-driven development, as Cycle 1: one task per unit, each with a brief, an
implementer, a scoped reviewer and a fix loop; then a whole-branch review.

| Task | Unit |
|---|---|
| 0 | Three carried-over follow-ups in files this cycle opens anyway (§11) |
| 1 | Token layer — both grounds, aliases, toggle, `prefers-color-scheme` |
| 2 | Guard test + rewritten `styles.css` header |
| 3 | Primitive re-skin (§7) |
| 4 | The 43 hardcoded literals (31 in page/component CSS + 12 in `styles.css`) |
| 5 | Motion consolidation (§8) |
| 6 | Page sweep — all 13 pages, both themes |
| 7 | Whole-branch review |

## 10. Verification

**Rendering, twice.** Every page captured in both themes through the Playwright
environment at `connectors/browser/.venv/bin/python`, plus a contrast audit over
every text node measured against its **composited** background — `getComputedStyle`
reports `color` ignoring `opacity`, so opacity must be composited in before a
ratio is computed. This is the Cycle 1 method, and it is what caught every real
defect in that branch; arithmetic alone caught none of them.

Also required:

- The existing console test suite (24 files, 305 tests) stays green throughout.
- The guard test (§6) joins it.
- No new dependencies. The console's air-gapped posture is unchanged.
- No `Math.random` in any animation — every sequence stays a fixed script or a
  pure function of an index.

## 11. Carried-over follow-ups

Two open items live in files this cycle opens anyway, and are folded in as task 0:

1. `console/src/components/Chain.tsx:93,96` — the in-flight pulse compares
   `entries.length` against the unfiltered feed while `:105` derives progress from
   the filtered `state.cleared`. Mixed sources: an admin-plane `secret_reload` row
   grows the raw count and fires the pulse, announcing a request the chain itself
   refuses to place.
2. `console/src/components/Chain.css:56` — comment reads `(cleared / 5)` for a
   six-stage chain.

**Withdrawn.** `console/src/lib/types.ts:26` was previously listed here as a
defect for declaring 4 of the gateway's 7 audit kinds. It is not one.
`lib/chain.ts:100-104` documents the choice: the chain keeps its own wider
`GatewayAuditKind` **specifically so** the shared type — and with it every
`Badge variant={kind}` call site — does not have to widen. That is a recorded
trade-off with a written rationale. Do not "fix" it.

The fourth known follow-up — the `require_auth` docstrings at
`runtime/src/agentos_runtime/api.py:68`, `api.py:256` and `config.py:13`, which
claim one exempt route where there are two — is unrelated to the console and
stays out of this cycle.

## 12. Non-goals

- **No information design.** Nothing changes about what a page leads with, how
  its content is ordered, or what an empty state says. That is Cycle 3.
- **No primitive migration.** Page-local CSS is not moved onto primitives and not
  deleted, even where it duplicates one. That changes structure, which would break
  rendering as sufficient verification. It is a natural Cycle 3 companion.
- **No new pages, routes or features.**
- **No Helm or deploy changes.**
- **No Track B.**

Anything structural noticed during the sweep is written to
`docs/superpowers/specs/cycle-3-backlog.md` instead of being fixed.
