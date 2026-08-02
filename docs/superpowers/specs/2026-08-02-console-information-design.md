# Console information design — Cycle 3

- **Date:** 2026-08-02
- **Cycle:** 3 of 3 in the platform-wide UI/UX overhaul
- **Predecessors:** [Cycle 1](2026-07-28-design-language-and-landing-design.md) (design language + landing, shipped), [Cycle 2](2026-08-01-console-visual-system-design.md) (console visual system, shipped)
- **Backlog this consumes:** [`cycle-3-backlog.md`](cycle-3-backlog.md)

## 1. Why

Cycle 2 changed how the console looks. It deliberately changed nothing about
what a page says, leads with, or orders — that boundary is what kept rendering
sufficient as its verification.

This cycle spends that deferred budget. The central problem, measured rather
than asserted:

**Five of thirteen pages open on a form for making a new thing, not on what
already exists.** `Keys` leads with "Create key" and puts "Existing keys"
below it. The same inversion holds on `Documents` (Add document → Ingested
documents), `Operators` (New operator → Operators), `Orgs` (Create org →
Organizations) and `Users` (Invite user → Members).

For a platform whose premise is that nothing runs unauthorized, unattributed or
unrecorded, that is backwards. An operator's constant activity is reading state;
creating is occasional.

**Seven pages render nothing but a notice when no admin key is configured.**
`Overview`, `Keys`, `Audit`, `Orgs`, `Users`, `Secrets` and `Provisioning` wrap
their entire body in `{adminKey && (…)}` — see `pages/Keys.tsx:125`. Someone
evaluating the platform sees a blank page instead of what the page is for.

## 2. Starting position

Measured against the tree at Cycle 2's merge (`f40d4a1`):

- **`EmptyState` already has the right shape.** `ui/Skeleton.tsx` exports it with
  `title` / `description` / `action`. There are **14 call sites**; only **4** pass
  a `description`. The primitive is not the problem — the convention is.
- **`PanelHead` takes `title` and `actions`.** A primary action needs no new
  component; it goes in `actions`.
- **`PageHead` already carries a static `subtitle`** — descriptive prose about
  what the page is ("Virtual gateway keys with monthly budgets…"). That is not a
  state reading and this cycle does not repurpose it.
- **The admin-key notice is not the blocker.** `NeedsKey` renders *alongside*
  content (`{!adminKey && <NeedsKey …/>}`), not as an early return. What
  suppresses the scaffold is a separate `{adminKey && (…)}` wrapper around the
  panels. The backlog described this as the notice replacing the page; that was
  imprecise, and the fix is to the data gate, not to the notice.
- **No component testing exists.** `vite.config.ts` sets `environment: "node"`
  and `include: ["src/**/*.test.ts"]`. There is no jsdom and `.tsx` files are not
  collected. This constrains §5.

## 3. Decisions

| # | Decision | Rationale |
|---|---|---|
| 1 | **Reading first; creation behind a disclosure.** | An operator comes to read. Matches the reference products (Langfuse, LangGraph). |
| 2 | **Every page always renders its scaffold.** The empty slot carries the explanation. | Four pages already do this; nine are made to match rather than a new pattern invented. |
| 3 | **Empty states use all three `EmptyState` props.** | "No objectives yet" states absence and stops. An empty state should say what appears there and offer the thing that fills it. |
| 4 | **Primary actions disable until required input is present.** | Resolves the backlog's split: `Playground`, `Multiverse` and `Operators` already do this; `Documents` and `Improve` do not. Majority wins, and it stops a request that can only fail. |
| 5 | **No new dependencies, including test-only ones.** | Held for three cycles. Adding jsdom to test one boolean is a poor trade; §5 gets verification another way. |

## 4. The patterns

Two new pieces. Everything else is a convention over what exists.

### 4.1 `PanelHead` gains a `summary` prop

One line under the panel title stating what it contains and whether anything
needs attention: `12 keys · 2 inactive`, `4 operators · 1 held`,
`38 documents · 1.2 MB`.

Derived from data the page already fetches. **No new API calls.** Where a count
of "things needing attention" is zero, the clause is omitted rather than
rendered as `· 0 inactive` — a quiet panel should read quiet.

### 4.2 A `Disclosure` primitive

Wraps a create form. Collapsed by default, with its trigger rendered in
`PanelHead`'s `actions`.

- Holds one boolean.
- Animates with the existing `ui/motion` presets and honours `useReducedMotion()`.
- Returns focus to the trigger on close.
- `aria-expanded` on the trigger, `aria-controls` pointing at the panel body.

### 4.3 `EmptyState` convention

No code change to the primitive. The convention, applied at all 14 call sites:

| Prop | Says |
|---|---|
| `title` | what is absent — "No keys yet" |
| `description` | what will appear here, and why it matters |
| `action` | the control that fills it |

`Keys`, `Orgs` and `Users` have **no empty state at all** today and need one
written.

## 5. Verification

Cycle 2's method does not transfer. A screenshot cannot tell you whether a page
leads with the right thing, and decision 5 rules out component tests. So:

**5.1 Pure functions carry the logic.** Each summary line is a pure function in
`lib/`, unit-tested in the existing vitest setup: `keysSummary(keys)`,
`operatorsSummary(ops)`, and so on. Tests must cover zero, one, many, and the
attention case — a singular/plural bug in a line every operator reads is exactly
the kind of thing that ships unnoticed.

The `Disclosure` boolean gets no test. The string beside it does.

**5.2 The render harness asserts page order.** `console/scripts/render.py`
already drives 13 routes × 2 themes and already fails on an unmeasured route.
It gains one assertion: **each route's first panel heading matches an expected
value**, from a table in the harness. This checks "what does this page lead
with" mechanically, and fails loudly if a page is ever reinverted.

The harness must keep its existing non-vacuity property: prove the new assertion
can fail by inverting one page and watching it report, before trusting a pass.

**5.3 Both gates stay green.** `npm run build` exits 0; the suite stays at ≥312
passing.

## 6. Carried-over defects

Two items from the backlog sit in files this cycle already opens, and are folded
in as a first task:

1. **`ui/Field.tsx` renders `<input {...rest} />` with no `type`.** The console's
   form styling is keyed on `input[type="text"], input[type="number"], …`, so a
   bare `<Input>` matches none of it and gets user-agent styling. Since Cycle 2
   bound `color-scheme` to the theme, those inputs take UA dark styling on the
   dark theme and their placeholder measures **2.43:1**. Scope is 3 inputs on 2
   routes. Fixing `Field.tsx` to emit an explicit `type` makes the console's own
   styling apply; **re-measure the focus ring and field contrast afterwards.**
2. **`pages/Users.tsx:106`** — the panel subtitle reads "…issues a one-time agu-
   token…". `agu-` is a token prefix, not a word; the sentence lost a fragment.

## 7. Non-goals

- **No new dependencies**, including test-only ones. If component testing is
  wanted later, that is its own decision.
- **No new data or API calls.** Every summary derives from data already fetched.
- **No visual-language changes.** Tokens, colour, spacing and motion belong to
  Cycle 2. Anything that looks wrong goes to a backlog, not this diff.
- **The motion duration drift and the non-text contrast harness stay deferred.**
  Both remain in `cycle-3-backlog.md` for a tooling effort of their own.
- **`.nav-note-hold`'s missing hue stays deferred** — it needs a non-text
  treatment, which is visual-language work.
- **No Track B.**

## 8. Page archetypes

| Archetype | Pages | Work |
|---|---|---|
| **List + create** | Keys, Documents, Operators, Orgs, Users | Full treatment: inversion, summary, disclosure, empty states. Keys/Orgs/Users need empty states written from scratch. |
| **List + create (partial)** | Multiverse | Already *leads* with a reading (`Council`), so it is not inverted at the page level — but it still places `New objective` above `Objectives`. That inner pair is inverted; the page's lead-in is left alone. |
| **Reading** | Overview, Audit | Already lead correctly. Summary lines; existing empty states upgraded to the three-part convention. |
| **Workbench** | Playground, Improve, Secrets | No list to invert. Scaffold fix; Improve also takes decision 4. |
| **Reference** | Docs, Provisioning | Static. Scaffold fix only. |

The six List pages carry most of the work; seven pages are light.
