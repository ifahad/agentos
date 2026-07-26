# AgentOS Console — Track A: Production Polish (Liveness · Icons · Data Tooling · Landing)

- **Status:** Draft for review
- **Date:** 2026-07-26
- **Scope:** Track A of the two-track "living console + agents-understand-systems" effort. This spec is the console/landing production-polish work only. Track B (agents build a versioned System Model of connected systems and file report-only improvement proposals) is a **separate** spec, deferred until Track A lands.
- **Decomposes into:** four implementation plans (S1→S4), built in order, each independently shippable.

---

## 1. Context

The console already has the bones of a live, crafted interface: a hand-drawn 16×16 glyph set (`ui/icons.tsx`), a full motion system (`ui/motion.ts`), a self-hosted type pairing (Archivo + IBM Plex Mono), the "Instrument" token system (`styles.css`), and a governance-chain component driven by real audit statuses. But the polish is unevenly applied:

- **Liveness is wired on ~3 surfaces of ~12.** Overview's four stat cards use `useLoad` (`components/common.tsx:62`) — a **fetch-once** hook — so `Requests / Tokens / Spend / Active keys` freeze the instant they render, next to an Activity feed that re-polls every 5s and a governance chain that re-polls every 4s. `useCountUp` (`ui/useCountUp.ts`) already animates those numbers on change — but the value never changes, because it is never re-fetched.
- **There are zero action-icon glyphs.** Every action — Copy, Refresh, Run, Pause, Approve, Deny, Delete, Dismiss — is a text button. The brand is a bare wordmark with no logomark.
- **Every table (a dozen across the console) is fetch-once with no search, filter, sort, export, or pagination.** Two production charts (`UsageChart`, `SpendBreakdown`) are built and exported from `charts/index.ts` — but rendered **nowhere**. There is no command palette.
- **The landing page moves, but no data moves.** Its governance-chain hero animates on a fixed CSS timeline (`@keyframes sweep`/`litcore`); the council and operators panels are static snapshots. Nothing is event-driven.

The through-line: *the machinery for "live and crafted" exists; it is under-propagated.* Track A propagates it, consistently, across the whole surface — with one new abstraction (a liveness contract) that also leaves a clean seam for Track B and a future server-push transport.

### Design invariants carried from the "Instrument" system (non-negotiable)

Every part of this spec obeys these existing rules:

1. **Chroma is reserved for machine state.** Only `--live #5ad1c4` (in flight), `--ok #6cc48f` (allowed/done), `--hold #e3a851` (awaiting a human), `--deny #e2685f` (denied/failed/over-budget) may carry hue. `--accent` deliberately resolves to ink. **New icons and charts-at-rest are monochrome** (`--text` / `--text-dim` / `--text-faint` / `--border`). A brand color would make the signals lie.
2. **Motion is telemetry, not decoration.** Movement reports state; idle surfaces are calm (no spinners on quiet data). Import timings/variants from `ui/motion.ts` (`EASE`, `transition`, `fadeRise`, `staggerItem`, …); never hardcode.
3. **Reduced-motion is honored on two layers** — the global CSS cap in `styles.css` (`prefers-reduced-motion`) *and* per-component `useReducedMotion()` swapping the `*Reduced` variant. Both must remain true for anything new.
4. **Machine values are mono + tabular** (`--mono`, `font-variant-numeric: tabular-nums`); labels/eyebrows follow the `.eyebrow` treatment (10px, 0.16em track, uppercase, `--text-faint`). Radii come from `--radius-sm/md/lg` (3/4/6px).
5. **Air-gap invariant.** No external requests, ever — no CDN, no remote fonts/scripts. This governs the landing page (`S4`) especially: its default must stay a single self-contained file.

### Non-goals (explicitly out of this spec)

- **No new gateway/runtime endpoints.** Every table already loads a full client-side array; all data tooling is client-side over already-loaded data. (Matches the repo's stated no-new-endpoints ethos.)
- **No SSE backend** (`/admin/stream`). The liveness contract is designed so SSE can slot in later per-resource with no page changes, but we do **not** build it now.
- **No Track B** (System Model, comprehension, improvement proposals).
- **No new color, no rounded-icon restyle, no font change.** We extend the existing system; we do not redesign it.

---

## 2. Architecture — the liveness contract (the spine of S1)

One hook replaces both the fetch-once `useLoad` and the four hand-rolled `setInterval` pollers (Overview, `components/Chain.tsx`, `components/Sidebar.tsx`, `pages/Operators.tsx`).

### 2.1 `useLiveResource<T>(key, fetcher, opts)`

```
useLiveResource<T>(
  key: string,                       // stable identity, e.g. "admin/audit?limit=100"
  fetcher: () => Promise<T>,
  opts?: {
    cadence?: number;                // ms; default 4000
    transport?: TransportFactory;    // default pollTransport; the swap seam
    enabled?: boolean;               // gate on adminKey / role
  },
): {
  data: T | null;
  error: string | null;
  status: "idle" | "loading" | "live" | "stale" | "error";
  updatedAt: number | null;          // ms epoch of last successful data
  transport: "poll" | "sse";         // which source is active (for UI honesty)
  reload: () => void;                 // manual refetch (Refresh buttons)
}
```

**Shared registry (dedupe).** A module-level `Map<key, Entry>` means every component reading the same `key` shares **one** timer and **one** in-flight request; subscribers get fan-out. Today Overview's cards, the Sidebar live-dot, and the Chain each poll `admin/audit`/`admin/usage` on their own interval — **three timers for two endpoints**. After: one poll per endpoint. Net fewer requests than today.

**The transport seam (the whole point of the chosen approach).** `transport` is an interface:

```
interface Transport<T> {
  subscribe(onData: (d: T) => void, onError: (e: unknown) => void): () => void; // returns unsubscribe
}
type TransportFactory = <T>(key: string, fetcher: () => Promise<T>, cadence: number) => Transport<T>;
```

- `pollTransport` (default, built now) wraps `setInterval(fetcher, cadence)` with immediate first fetch.
- `sseTransport` (later, not this spec) wraps the existing `streamSSE` from `lib/sse.ts`.

**Pages never see which transport is active.** Switching a resource to push later is a one-line registry change, zero page edits.

**Visibility-pause.** The registry listens to `document.visibilitychange`; while hidden, all transports pause; on refocus, every subscribed resource refetches immediately. Kills background-tab request spam and makes "return to tab → instantly fresh" feel alive.

**`updatedAt` / `status` derivation.** `updatedAt` stamps each successful payload. `status` is `live` if a fetch landed within ~1.5× cadence, `stale` if overdue, `error` if the last N fetches threw. A failed poll never clobbers the last-known `data` (mirrors the Chain's existing "hold last reading" behavior — a failed poll tells us nothing about governance).

### 2.2 Shell connection state

The registry exposes a `useConnectionState()` selector returning one global `live | stale | offline` (offline = *all* resources erroring; stale = any overdue; live otherwise). Rendered in the topbar (`App.tsx` `header.topbar`) beside the existing `governance chain` legend: a `StateIcon` + `synced Ns ago`. **Honesty rule:** `stale` must never look like `quiet` (healthy, no traffic) — same principle the Chain already embodies ("quiet ≠ unknown").

### 2.3 Shared motion primitives

- **`useCountUp` — already exists** (`ui/useCountUp.ts`); no change. It tweens on value change; once cards are live it animates real deltas instead of only the mount.
- **`<LiveList>` (new, thin).** Standardizes the `AnimatePresence` + `staggerItem` row-in/row-out already inlined in Overview's feed, so every table gets identical entrance motion. Wraps the existing `Tbody` `staggerKey` mechanism (see §5) rather than replacing it.
- **`<Freshness updatedAt />` (new, tiny).** Renders "updated Ns ago" (mono, `--text-faint`) with a one-frame pulse on each update; recomputes on a shared 1s ticker; reduced-motion drops the pulse.

### 2.4 Migration

Replace `useLoad` call sites and the four `setInterval` pollers with `useLiveResource`. `useLoad` stays available for genuinely one-shot loads (create-form option lists), but all **list/metric** surfaces move over. Chain and Sidebar drop their private timers and read the shared `admin/audit` / `admin/usage` resources.

---

## 3. S1 — The liveness pass (surface by surface)

Rule: *nothing that can change while you watch may sit frozen; motion is reserved for actual change.*

| Surface | Today | After |
|---|---|---|
| **Overview** — stat cards | `useLoad` fetch-once | `useLiveResource` → existing `useCountUp` animates real deltas |
| **Overview** — feed / meters | already polling | feed → `<LiveList>`; meters keep animated fills |
| **Chain / Sidebar** | private timers on audit/usage | subscribe to the shared registry — **zero new timers** |
| **Audit** (`/audit`) | fetch-once (`?limit=100`) | live: new rows stream in at top via `<LiveList>`, `<Freshness>`, live/paused toggle |
| **Keys** (`/keys`) | fetch-once | live spend/budget; `useCountUp` on spend; meter re-animates |
| **Operators** (`/operators`) | private `setInterval` | shared hook; next-run countdown ticks; running rows pulse `--live` |
| **Multiverse** (`/multiverse`) | fetch-once objectives + proposals | live while a council run is in flight (members' answers land), mirroring Playground streaming |
| **Improve** (`/improve`) | fetch-once eval runs | live refresh while an eval run executes; running row pulses |
| **Documents / Orgs / Users / Secrets / Provisioning** | fetch-once CRUD | **optimistic updates** (row appears on create, reconciles on next fetch) + `<Freshness>` — liveness here = "reflects my action instantly," not needless ticking |

**Idle vs live.** A surface with no in-flight activity shows a calm "synced" reading, not a spinner. Spend the motion budget on change.

---

## 4. S2 — Action icons & logomark

**Finding:** the console has **zero** action-icon glyphs; all 15 below are net-new. Two `StateIcon` marks are semantic cousins that must **not** be conflated: `ok` (ring+tick, a *status* readout locked to `--ok`) vs an `approve` *action* icon (monochrome, inherits button color); likewise `deny`.

### 4.1 The action-icon family

Extend the `IconName` union and the `GLYPHS` map in `ui/icons.tsx`, obeying the exact house grid: `viewBox="0 0 16 16"`, artwork in the 2..14 band, half-unit snapping, `strokeWidth={1.25}`, `strokeLinecap="square"`, `strokeLinejoin="miter"`, `fill="none"`, `stroke="currentColor"`. Keep the schematic "wiring-diagram" voice — not friendly pictograms.

| Glyph | Replaces text at | Collision to avoid |
|---|---|---|
| `copy` | `CopyButton` (`common.tsx:92`; Keys/Users) | — |
| `refresh` | Audit/Secrets/Documents/Keys reload buttons | must **not** echo the `improve` return-loop arc |
| `run` | Operators Run, Improve "Run evals" | must **not** become the play-triangle the `playground` glyph deliberately rejects |
| `pause` | Operators enable/pause | — |
| `approve` | Improve "Approve" | distinct from `StateIcon ok` |
| `deny` | Improve "Deny" | distinct from `StateIcon deny` |
| `export` | new CSV/JSON export (S3) | — |
| `search` | new table search (S3) | — |
| `filter` | new table filter (S3) | — |
| `sort` | new sortable headers (S3) | — |
| `close` | dismiss panels, modal/toast close | — |
| `external-link` | out-links (Provisioning IdP ids) | — |
| `chevron` | expand/collapse (Improve), palette | **factor out** the chevron shape currently hidden inside the `playground` glyph so both share it |
| `trash` | Operators Delete, future deletions | — |
| `plus` | create key/user/operator/document | — |

**Button integration.** Add an optional `icon?: IconName` (and `iconOnly?: boolean` with a required `title` for a11y) to `ui/Button.tsx`, so a button can render glyph + label. `CopyButton` shows `copy` alongside/instead of the word.

### 4.2 Logomark (`BrandMark`)

Add a `BrandMark` export to `ui/icons.tsx` sharing the glyph conventions (16×16, 1.25 stroke, square caps), rendered ~16–20px. Slot it as the **first child** of `.brand` in `Sidebar.tsx:94`, before `.brand-name` (the existing `gap:8px` already spaces it). Concept: a compact emblem built from the governance-chain motif (nodes on a rail) so the mark *is* the product thesis. Decide whether the teal live-dot stays a sibling of the wordmark or folds into the emblem (recommend: keep the live-dot as a separate signal, since it carries `--live` state and the emblem must stay monochrome). Set the browser-tab title emblem via the existing inline-SVG favicon path.

---

## 5. S3 — Production data tooling

**Ground truth:** every table (a dozen across the console — including two on Multiverse) shares the **same three primitives** (`Table` / `Tbody` / `Tr` from `ui/Table.tsx`; there is **no** `Th` primitive — pages write plain `<th>` with `className="num"` for numerics), each renders one flat array from one `useLoad→apiFetch`, and every array is already fully client-side. So we build **one reusable table-tools layer**, not a per-page rewrite, all client-side.

### 5.1 Composable pipeline (pure, independently testable)

`filter → search → sort → paginate`, each a pure function/hook (mirroring `charts/transforms.ts`, which already has `transforms.test.ts`):

- **`useTableSearch<T>(rows, fields)`** — case-insensitive substring over chosen string fields. New `<SearchInput>` reuses `ui/Field` Input with `type="search"`, dropped into the `PanelHead` `actions` slot (the slot Audit already uses for Refresh). Highest value: Audit, Keys, Users, Orgs, Documents.
- **`useSort<T>(rows, {key, dir})`** — returns sorted rows + a toggle. Add a `SortableTh` (or extend `Table.tsx` with a `Th` taking `sortKey`, active `dir`, `onClick`, rendering the `sort` glyph). Numeric vs `localeCompare` comparator chosen off the existing `className="num"` convention. Feed sort state into `Tbody`'s `staggerKey` so a re-sort **replays the row stagger** already built in.
- **`useFilter<T>`** — for small enumerable domains (Audit `kind`: chat/embeddings/guardrail_flag/guardrail_block; Users/Provisioning role+status; Secrets source+present), a `ui/Field` Select or chip filter over the same predicate.
- **`useTablePager<T>` (optional, default off)** — client pager (page size ~25), enabled on Audit (100 rows) and Improve/Multiverse (can grow); off for tiny lists (Keys/Orgs/Secrets). Display-only; server still caps via `?limit=`.

These compose behind one `useTableView<T>(rows, config)` returning the final view + toolbar state, so a page opts in with one hook + one `<TableToolbar>` in its `PanelHead actions`.

### 5.2 Export

New `lib/export.ts` (none exists): `toCSV(rows, columns)` + `downloadBlob(filename, mime, text)` via `Blob` + `URL.createObjectURL` + a synthetic `<a>.click()`. "Export CSV" / "Export JSON" as `Button small icon="export"` in each `PanelHead actions`. **Exports the current filtered+sorted view**, so tools compose. Column maps come straight off the typed fields (`AuditEntry`, `KeyUsage`, `KeyInfo`, `Org`, `User`, `SecretStatus`).

### 5.3 Wire the two unused charts

Both feeders are already exported from `charts/transforms.ts`; both source arrays are already in-page:

- **`UsageChart` → Audit**, in a Panel above the Events table. Feed `seriesFromEvents(entries.map(e => ({timestamp: e.ts, value: e.cost_usd})), {bucket: "day"})`, map `.value` into `values`, `xLabels` from bucket timestamps, `formatValue = formatUSD`. (Same transform Overview uses for the Sparkline, axed variant.)
- **`SpendBreakdown` → Overview**, inside the existing `ov-grid`. Overview already loads `KeyUsage[]`: `breakdownFromRows(usage, {label: u => u.name, value: u => u.spend_usd})`, `formatValue = formatUSD`, titled "Spend by key". (Alternatively "Spend by model" from audit entries.)

Charts stay monochrome at rest per the chroma invariant.

### 5.4 Command palette (⌘K)

**Finding:** no palette, no ⌘K; the only window-level keydown is Modal.tsx's Escape-to-close. Navigation is a single `navigate(path)` and `ROUTES` already carry `label` + `icon` + `visible`; `visibleRoutes` is role-filtered; `SettingsModal` + `Sidebar` already mount at `App` level — the natural palette mount point.

Mount `<CommandPalette>` at `App` level beside `<SettingsModal>`, reusing the `ui/Modal` portal + `AnimatePresence` + window-keydown pattern. Add the first global hotkey: `(e.metaKey||e.ctrlKey) && e.key === "k"` → `preventDefault` + open. Command source = `visibleRoutes` mapped to "Go to <label>" → `navigate(r.path)` (using each route's `icon`), plus "Open Settings" (`openSettings`); later, per-page actions. Query reuses `ui/Field` Input with the same substring filter as table search; ↑/↓ + Enter to run, Escape to close (mirror Modal). Thin filtered list over `ROUTES` — low effort, high leverage.

---

## 6. S4 — Landing real-preview

**Finding:** `landing/index.html` is a single self-contained file (fonts inlined as data URIs; zero external requests). Motion today is pure decorative CSS (`sweep`/`litcore`/`pulse`/`reveal`); the two viz panels (council, operators) are static snapshots; the one small IIFE only does theme-toggle + scroll-reveal + copy-command. **No data moves.**

**Design:** make the hero governance-chain event-driven by reusing its existing `.chain`/`.rail`/`.node .core` DOM, plus live-ticking readouts and an audit-tail feed — while keeping the file self-contained.

- **Option A — in-file seeded simulation (baseline, always on).** A small IIFE with a seeded PRNG + a canned pool of request records advances a synthetic request Auth→RBAC→Budget→Rate→Audit, toggling a `data-lit`/`.active` class per node as it arrives (replacing the fixed CSS delays), so the chain becomes a **data-synced conveyor**. Occasional Budget/Rate/Audit holds flash `--hold`/`--deny` and bump a counter. Live readouts (requests governed, spend reserved, p95 latency, audit rows) replace the static `5/$0/0/Any` tiles; a prepend-on-tick audit tail and a rolling inline-SVG throughput sparkline complete it. Deterministic seed = reproducible; `prefers-reduced-motion` freezes to a representative still. **No external requests → still a valid air-gapped, single-file, Artifact-safe page.**
- **Renderer takes an event source.** Build A's renderer to accept an event stream, so two enhancements layer on the same shape without a rewrite:
  - **Option C — record & replay** (optional): inline one short real gateway trace as a JSON literal and loop-replay it → genuine numbers, still zero-request.
  - **Option B — live fetch** (opt-in, self-hosted only): if `window.AGENTOS_GATEWAY` is configured, fetch real metrics/audit tail and feed the same renderer; `try/catch` → fall back to A. **Not Artifact-safe** (CSP blocks external hosts); documented as self-host-next-to-gateway only.

Default ships as A. The two static viz panels get the same treatment (council votes reaching quorum on rotation; operators run-history prepending rows).

---

## 7. Cross-cutting concerns

- **Accessibility.** New icons stay `aria-hidden` unless they're the sole carrier of meaning (then `title` → `role="img"`). Icon-only buttons require a `title`/`aria-label`. Palette and toolbars are keyboard-first (focus trap in palette via the Modal pattern; visible focus uses the existing `--live` focus-visible outline). `<Freshness>` and connection state use `aria-live="polite"` sparingly (denials/offline only, matching the Chain).
- **Reduced motion.** Every new animated piece ships a `*Reduced` path or relies on the global CSS cap; `<LiveList>`, `<Freshness>` pulse, count-up, palette, and the landing sim all freeze cleanly.
- **Air-gap.** No new external requests anywhere. Landing default stays self-contained; live mode is opt-in and documented as non-air-gapped.
- **Performance.** The shared registry reduces total polling; visibility-pause halts hidden tabs; client-side table tools operate on already-loaded arrays (≤100 rows), so no perf risk.

---

## 8. Testing strategy (vitest, matching the existing 207-test suite)

- **Liveness contract:** registry dedupe (two subscribers → one timer/fetch), `status` transitions (live→stale→error), `updatedAt` stamping, failed-poll-holds-last-data, visibility-pause/resume, transport-swap (inject a fake `sseTransport`, assert pages unchanged). Pure where possible.
- **Motion primitives:** `<LiveList>` add/remove keys; `<Freshness>` relative-time formatting; reduced-motion variant selection.
- **Icons:** snapshot each new glyph's path presence; assert conventions (viewBox, stroke, `currentColor`); `Button` icon/iconOnly a11y (title required when icon-only).
- **Data tooling:** unit-test each pure stage (`useTableSearch`/`useSort`/`useFilter`/pager) and the composed `useTableView`; `lib/export.ts` `toCSV` (quoting/escaping, column mapping) — extend `transforms.test.ts` style. Chart wiring: feeder transforms produce expected shapes.
- **Command palette:** ⌘K opens/closes, substring filter over routes, Enter navigates, role-filtered commands (a viewer never sees admin-only routes), Escape closes.
- **Landing:** seeded sim is deterministic (same seed → same sequence); reduced-motion renders a still; no `fetch`/external URL in default mode (static assertion).

---

## 9. Decomposition into implementation plans

Built in order; each is independently shippable and testable.

1. **Plan S1 — Liveness contract + pass.** `useLiveResource` + registry + `pollTransport` + connection state + `<LiveList>`/`<Freshness>`; migrate all list/metric surfaces; retire the four private timers. *(Foundation — everything else renders into a live console.)*
2. **Plan S2 — Action icons + logomark.** The 15 glyphs, `BrandMark`, `Button` icon support, swap text→icon at call sites.
3. **Plan S3 — Data tooling.** `useTableView` pipeline + `<TableToolbar>`/`SortableTh`/`<SearchInput>`, `lib/export.ts`, wire `UsageChart`+`SpendBreakdown`, `<CommandPalette>` + ⌘K. *(Uses S2's search/filter/sort/export glyphs.)*
4. **Plan S4 — Landing real-preview.** Seeded-sim renderer over the existing chain DOM + live readouts/feed/sparkline; optional inlined replay trace; documented opt-in live mode.

**Dependencies:** S3 depends on S2 (glyphs) and benefits from S1 (`Tbody staggerKey` re-animate on tool change). S4 is standalone (separate file) and can proceed in parallel after S1 establishes the motion patterns it mirrors.

---

## 10. Open questions

1. **Logomark direction** — fold the live-dot into the emblem, or keep it a separate `--live` signal beside the wordmark? (Recommend: keep separate; the emblem stays monochrome.)
2. **Audit pagination vs infinite-scroll** — client pager (spec default) or "load older" once we page the server `?limit=`? (Recommend: client pager now; server paging is a later, backend-touching change.)
3. **Landing default** — ship pure seeded sim (A), or capture one real trace and inline it (A+C) for authentic numbers? (Recommend: A now; C when a representative trace is easy to capture.)
