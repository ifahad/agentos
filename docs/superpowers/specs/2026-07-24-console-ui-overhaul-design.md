# AgentOS Console UI/UX Overhaul — Design Spec

**Date:** 2026-07-24
**Status:** Approved (brainstorming complete)
**Scope:** `console/` only — no API, gateway, or runtime changes. No feature changes:
same endpoints, same RBAC logic, same data flows.

## Goal

Transform the AgentOS console from a utilitarian dark admin UI into a
Linear-grade, minimal, animated interface where motion is *telemetry*: the UI
makes agent activity visible and feels alive because the platform is alive.

**Aesthetic direction:** Linear-grade minimal — refined dark, near-monochrome,
hairline borders, one restrained accent. Dark-only (no light theme).

**Depth:** Full overhaul — new design tokens, component system, redesigned
shell/navigation, and all 10 pages restyled, plus hand-rolled SVG data
visualization.

## Design tokens

CSS custom properties in `src/styles.css` (single source of truth):

- **Surfaces:** base `#0a0b0d`, raised `#101318`, inset `#0c0e12`. Elevation
  communicated by subtle surface steps, not heavy borders or shadows.
- **Borders:** hairlines — `rgba(255,255,255,0.06)` default,
  `rgba(255,255,255,0.09)` strong.
- **Text:** primary `#d6dae1`-class near-white, dim, faint (three steps).
- **Accent:** one electric blue (`#5b8cff`), used sparingly: active nav pill,
  primary buttons, live indicators, focus rings. `--accent-dim` translucent
  variant for fills.
- **Semantic:** green/amber/red muted ~15% from current values; used only for
  status (badges, meters, guardrails, pass/fail).
- **Type:** Inter (system stack fallback). Scale: 20px/600 page titles with
  `-0.01em` tracking, 13px body, 11px uppercase tracked (+0.1em) section
  labels, 12.5px mono (tabular-nums) for all data/IDs/numbers.
- **Radius:** 6px controls, 8px cards/panels, 10px modals.
- **Motion tokens:** `--dur-fast: 120ms`, `--dur-med: 200ms`,
  `--ease: cubic-bezier(0.2, 0.8, 0.2, 1)`. All non-essential animation
  disabled under `prefers-reduced-motion`.

## Shell & navigation

- Sidebar slims to ~200px. Nav items get a **sliding active-pill** animated
  between items via framer-motion `layoutId` — the signature shell animation.
- Brand mark gains a small live dot that pulses whenever any agent run is
  in-flight (polled from the same activity source as Overview; cheap, shared).
- Page transitions on route change: 160ms fade + 6px rise.
- Settings modal: backdrop fade + panel scale/rise entrance.
- Sidebar footer (settings entry, role indicator) restyled to tokens.

## Component system (`src/ui/`)

New primitives replacing ad-hoc markup, all token-driven:

- `Card`, `Panel`, `Stat` (label + count-up value + optional sparkline)
- `Table` — staggered row entrance on mount/data load, hover lift
- `Badge` — refined pill, same semantic mapping as today
- `Button` — primary / ghost / danger / small; 90ms press-scale micro-interaction
- `Input`, `Select`, `Textarea` — animated focus ring
- `Tabs`, `Modal`
- `Toast` — **new**: transient feedback for actions (key created, secret saved,
  proposal approved, errors); bottom-right, auto-dismiss, slide+fade
- `Skeleton` — shimmer placeholders replacing "loading…" text
- `EmptyState` — centered, dim, with optional action
- Motion presets in `src/ui/motion.ts` (durations, easings, stagger configs,
  shared variants) so all animation is consistent.

## "Agents working" — dynamic core

- **Overview:** stats count up on load; live request sparkline; budget meters
  animate to fill; **activity feed** of recent runs (polled, existing usage/
  audit endpoints) — new entries slide in, in-flight runs show a pulsing
  status dot.
- **Playground:** run timeline becomes a **step stream** — events cascade in
  with staggered entrances; tool calls expand/collapse with layout animation;
  pending steps show a thinking-shimmer; outputs reveal softly. While a run is
  live, an accent pulse accompanies the stream. SSE rendering logic unchanged
  (`lib/sse.ts`) — presentation only.
- **Audit:** staggered row entrance; guardrail block/flag events get a brief
  emphasis flash on arrival.

## Data visualization

Hand-rolled SVG components (no chart library): `Sparkline`,
`UsageChart` (line/area over time), `SpendBreakdown` (by model/key). Thin
strokes, soft area fills, minimal axes, accessible (aria labels, non-color
differentiation where categorical). Follow the `dataviz` skill's palette and
mark rules at implementation time.

## Pages

All 10 pages restyled onto the new primitives: Overview, Keys, Audit,
Playground, Documents, Improve, Orgs, Users, Secrets, Provisioning.
Notable per-page work beyond restyle:

- **Improve:** proposals become cards with animated approve/deny outcomes
  (badge transition + collapse).
- **Documents:** drag-state upload treatment (border pulse + progress).
- **Keys/Secrets:** creation success surfaces via toast; revealed secrets get
  the refined reveal panel (existing pattern, restyled).

## Tech approach

- **Hybrid motion (approved):** framer-motion for layout/entrance/page
  transitions (nav pill, step stream, modals, toasts); CSS transitions/
  keyframes for hovers, shimmers, pulses, count-ups. **One new dependency
  (`framer-motion`), nothing else.**
- File layout: `src/ui/` (primitives + motion presets), `src/charts/` (SVG
  viz), pages keep their data/logic hooks and swap markup to primitives.
- Existing tests (`lib/*.test.ts`) must pass unchanged. Add unit tests for new
  pure utilities: chart data transforms, count-up hook, activity-feed reducer.

## Acceptance criteria

- `make test` green (Go + Python untouched, vitest suites pass).
- `npm run build` in `console/` clean (tsc + vite).
- All 10 pages render with new system; nav pill, page transitions, step
  stream, count-ups, toasts, skeletons working.
- `prefers-reduced-motion` respected.
- No API contract changes; no RBAC behavior changes.

## Reference

Visual reference is maintained as a Claude Design project synced via
`/design-sync` (component previews with `@dsCard` markers under
`console/design/`).
