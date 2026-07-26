# Console S2 — Action Icons & Logomark Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the hand-drawn "Instrument" glyph set from 13 nav marks to a full action-icon family (~15 glyphs), give `Button` first-class icon support, add a real `BrandMark` logomark, and replace text-only action buttons across the console with icon + label — without importing a stock icon set.

**Architecture:** Action glyphs are added to the existing `IconName` union + `GLYPHS` map in `console/src/ui/icons.tsx`, obeying the file's stated grid rules; `Button` gains an `icon`/`iconOnly` prop that renders a decorative leading `<Icon>`; call sites swap text for icon+label. The `BrandMark` is a new monochrome emblem export built from the governance-chain motif, slotted into the sidebar brand row with the teal live-dot kept as a separate `--live` signal.

**Tech Stack:** React 19, TypeScript 5.8, framer-motion 12 (unchanged), Vitest 3 (node env). No new dependencies.

## Nature of this work (read first)

Unlike S1 (pure logic, fake-timer tested), **S2 is design work.** Glyph geometry is a visual artifact: `TypeScript` proves the set is *complete* and *typed* (`GLYPHS: Record<IconName, JSX.Element>` fails to compile if any name lacks a glyph) and `vite build` proves it *renders*, but neither proves a glyph is *legible or distinct*. So every glyph/brand task has two gates: (1) automated — `tsc --noEmit` clean + full suite green (231/231, no regression) + `npm run build` succeeds; (2) **visual — a review on the running console** (the human partner's call), made easy by a temporary icon-sheet route added in Task 2 and removed before merge. Candidate geometry in this plan is a conforming *starting point*; the implementer refines shape for legibility as long as the grid rules and collision constraints below hold. There are no glyph unit tests (the node test env cannot render SVG, and type-completeness is already compile-enforced).

## Global Constraints

Copied verbatim from `docs/superpowers/specs/2026-07-26-agentos-console-track-a-design.md` §4 + design invariants. Every task implicitly includes these.

- **The house grid (every glyph, no exception):** `viewBox="0 0 16 16"`; artwork confined to the **2..14** band; `strokeWidth={1.25}`, `strokeLinecap="square"`, `strokeLinejoin="miter"` ("machined, never rounded"); geometry snaps to whole or half units; `fill="none"`, `stroke="currentColor"`. Each `GLYPHS` entry is ONLY the inner artwork fragment; the `Icon` wrapper supplies the `<svg>`. Voice: schematic panel-legend / wiring-diagram marks, not friendly pictograms.
- **Chroma is reserved for machine state.** Action icons are monochrome (`currentColor`, inheriting their button's ink) — they must NEVER carry `--live/--ok/--hold/--deny` or any hue. Only `StateIcon` carries color. The `BrandMark` emblem is monochrome; the teal live-dot stays a separate `--live` element.
- **Collision constraints (enforced):** `refresh` must NOT echo the `improve` return-loop arc; `run` must NOT be a media play-triangle (the `playground` glyph comment deliberately rejects one); the standalone `chevron` should be factored out of the shape already inside the `playground` glyph and reused there; the `close`(dismiss) and `deny`(reject) glyphs must be visually distinct from each other AND from `StateIcon`'s `deny` barred-ring.
- **Accessibility:** a glyph passing `title` becomes `role="img"`+aria-label — pass it ONLY when the icon is the sole carrier of meaning. An icon beside a visible text label stays decorative (no `title`). An `iconOnly` button therefore requires an `aria-label` (or `title`) on the BUTTON, with the inner `<Icon>` left decorative. (This is the exact lesson from S1 Task 10.)
- No new dependencies, no new endpoints, air-gap preserved. Test env is node (`.test.ts` pure only). Test: `npm test`. Build/typecheck: `npm run build`. Run from `console/`.

## File Structure

- Modify: `console/src/ui/icons.tsx` — extend `IconName` + `GLYPHS` with the action family; add a `BrandMark` export.
- Modify: `console/src/ui/Button.tsx` — `icon`/`iconOnly` props.
- Modify: `console/src/ui/ui.css` (or `styles.css`) — `.btn-icon` spacing, `.btn.icon-only` square padding.
- Modify: `console/src/components/Sidebar.tsx` (+ `Sidebar.css`) — slot `BrandMark` into `.brand`.
- Modify: `console/src/components/common.tsx` — `CopyButton` shows the `copy` glyph.
- Modify call sites: `pages/Audit.tsx`, `pages/Secrets.tsx`, `pages/Documents.tsx`, `pages/Operators.tsx`, `pages/Improve.tsx`, `pages/Keys.tsx`, `pages/Users.tsx`, `pages/Provisioning.tsx` — swap text action buttons for icon+label.
- Temporary: `pages/_IconSheet.tsx` + a dev-only route (added Task 2, **removed Task 6**) — renders every glyph at 13/15/20px in both inks for the visual review.

---

### Task 1: `Button` icon support

**Files:**
- Modify: `console/src/ui/Button.tsx`
- Modify: `console/src/ui/ui.css`

**Interfaces:**
- Consumes: `Icon`, `IconName` from `./icons`.
- Produces: `ButtonProps` gains `icon?: IconName`, `iconOnly?: boolean`; `children` becomes optional. When `icon` is set, a decorative `<Icon>` renders before `children`. When `iconOnly`, the caller MUST supply `aria-label` (or `title`); the button gets an `icon-only` class for square padding.

- [ ] **Step 1: Extend `ButtonProps` and render the icon**

```tsx
// console/src/ui/Button.tsx
import type { ButtonHTMLAttributes, ReactNode } from "react";
import { Icon } from "./icons";
import type { IconName } from "./icons";

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  /** default = ghost (inset surface, hairline border) */
  variant?: "primary" | "ghost" | "danger";
  small?: boolean;
  /** Optional leading glyph. Decorative — the button's text (or aria-label) names the action. */
  icon?: IconName;
  /** No visible label. REQUIRES an aria-label (or title) on the button for accessibility. */
  iconOnly?: boolean;
  children?: ReactNode;
}

export function Button({
  variant = "ghost",
  small,
  icon,
  iconOnly,
  className = "",
  children,
  ...rest
}: ButtonProps) {
  const cls = [
    "btn",
    variant === "primary" ? "primary" : "",
    variant === "danger" ? "danger" : "",
    small ? "small" : "",
    iconOnly ? "icon-only" : "",
    className,
  ]
    .filter(Boolean)
    .join(" ");
  return (
    <button className={cls} {...rest}>
      {icon ? <Icon name={icon} size={small ? 12 : 13} className="btn-icon" /> : null}
      {children}
    </button>
  );
}
```

- [ ] **Step 2: Add `.btn-icon` / `.btn.icon-only` CSS**

In `console/src/ui/ui.css`, near the `.btn` rules:

```css
/* Leading glyph in a button — separated from the label, inherits button ink. */
.btn-icon {
  margin-right: 6px;
  vertical-align: -1px;
}
/* Icon-only buttons are square: no label, symmetric padding. */
.btn.icon-only {
  padding-left: 7px;
  padding-right: 7px;
}
.btn.icon-only .btn-icon {
  margin-right: 0;
}
```

- [ ] **Step 3: Typecheck + suite + build**

Run: `cd console && npx tsc --noEmit && npm test && npm run build`
Expected: clean; 231/231; build succeeds. (No behavior change to existing buttons — `icon`/`iconOnly` are optional.)

- [ ] **Step 4: Commit**

```bash
git add console/src/ui/Button.tsx console/src/ui/ui.css
git commit -m "feat(console): Button icon + iconOnly support"
```

---

### Task 2: The action-glyph family + icon sheet

**Files:**
- Modify: `console/src/ui/icons.tsx`
- Create (temporary): `console/src/pages/_IconSheet.tsx`
- Modify (temporary): `console/src/App.tsx` (register a dev-only `/_icons` route)

**Interfaces:**
- Produces: `IconName` gains `"copy" | "refresh" | "run" | "pause" | "approve" | "deny" | "export" | "search" | "filter" | "sort" | "close" | "external-link" | "chevron" | "trash" | "plus"`; each has a `GLYPHS` entry. (`GLYPHS: Record<IconName, JSX.Element>` compile-enforces completeness.)

- [ ] **Step 1: Add the 15 action glyphs to `IconName` + `GLYPHS`**

Extend the union and add entries. Candidate geometry below is a grid-conforming starting point; refine for legibility, keeping the grid rules + collision constraints. Draw them in the same schematic voice as the nav set.

```tsx
// Add to the IconName union:
//   | "copy" | "refresh" | "run" | "pause" | "approve" | "deny"
//   | "export" | "search" | "filter" | "sort" | "close"
//   | "external-link" | "chevron" | "trash" | "plus"

// Candidate GLYPHS entries (inner artwork only):
copy: (<><rect x="5.5" y="5.5" width="7" height="7" /><path d="M9.5 5.5V3.5h-6v6h2" /></>),
// refresh — two arcs + arrowheads; NOT the single improve return-loop.
refresh: (<><path d="M12.5 7A4.5 4.5 0 0 0 4.3 4.6" /><path d="M3.5 9A4.5 4.5 0 0 0 11.7 11.4" /><path d="M4.3 2.2v2.4h2.4" /><path d="M11.7 13.8v-2.4H9.3" /></>),
// run (execute now) — arrow striking INTO a node; NOT a play triangle. Echoes the operators "strike into a square" motif.
run: (<><path d="M3 8h5" /><path d="M6 6 8 8 6 10" /><rect x="9.5" y="5.5" width="4" height="5" /></>),
pause: (<><path d="M6 4.5v7" /><path d="M10 4.5v7" /></>),
// approve — bare check (StateIcon.ok is a ringed check; this action mark has no ring).
approve: <path d="M3.5 8.5 6.5 11.5 12.5 4.5" />,
// deny (reject) — bare cross. Distinct from close (below) by weight/scale, and from StateIcon.deny's barred ring.
deny: (<><path d="M4.5 4.5 11.5 11.5" /><path d="M11.5 4.5 4.5 11.5" /></>),
export: (<><path d="M8 3v6" /><path d="M5.5 6.5 8 9 10.5 6.5" /><path d="M3.5 11.5h9" /></>),
search: (<><circle cx="7" cy="7" r="3.5" /><path d="M9.6 9.6 13 13" /></>),
filter: <path d="M2.5 4h11l-4.2 4.7v3.8l-2.6 1.3V8.7z" />,
sort: (<><path d="M3.5 4.5h6" /><path d="M3.5 8h4" /><path d="M3.5 11.5h2" /><path d="M11.5 4.5v7" /><path d="M10 10 11.5 11.5 13 10" /></>),
// close (dismiss) — a compact cross inset from the deny cross so the two read differently at 13px.
close: (<><path d="M5 5 11 11" /><path d="M11 5 5 11" /></>),
"external-link": (<><path d="M8 3.5H4.5v9h9V9" /><path d="M9.5 2.5h4v4" /><path d="M13.5 2.5 8 8" /></>),
chevron: <path d="M6 4.5 9.5 8 6 11.5" />, // factor this shape out of the `playground` glyph and reuse it there.
trash: (<><path d="M3.5 4.5h9" /><path d="M5.5 4.5V3h5v1.5" /><path d="M4.5 4.5 5 13h6l.5-8.5" /></>),
plus: (<><path d="M8 3.5v9" /><path d="M3.5 8h9" /></>),
```

- [ ] **Step 2: Deduplicate the `chevron` shape inside `playground`**

The `playground` glyph currently inlines `M3 4.5 6.5 8 3 11.5`. Leave `playground` visually identical, but note in a comment that `chevron` is the reusable standalone version. (Do not break `playground`.)

- [ ] **Step 3: Add a temporary icon sheet for visual review**

```tsx
// console/src/pages/_IconSheet.tsx  (TEMPORARY — removed in Task 6)
import { Icon } from "../ui/icons";
import type { IconName } from "../ui/icons";

const NAMES: IconName[] = [
  "overview","keys","audit","playground","documents","improve","orgs","users",
  "secrets","provisioning","settings","multiverse","operators",
  "copy","refresh","run","pause","approve","deny","export","search","filter",
  "sort","close","external-link","chevron","trash","plus",
];

export function IconSheet() {
  return (
    <div style={{ padding: 24, display: "grid", gridTemplateColumns: "repeat(6, 1fr)", gap: 20 }}>
      {NAMES.map((n) => (
        <div key={n} style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 6 }}>
          <div style={{ display: "flex", gap: 10, alignItems: "center" }}>
            <Icon name={n} size={13} />
            <Icon name={n} size={15} />
            <Icon name={n} size={20} />
          </div>
          <span className="mono" style={{ fontSize: 10, color: "var(--text-faint)" }}>{n}</span>
        </div>
      ))}
    </div>
  );
}
```

Register a dev-only route in `App.tsx` (e.g. render `<IconSheet />` when `window.location.pathname === "/_icons"`, before the normal shell) so the sheet is reachable at `/_icons`. Keep this change small and clearly marked `// TEMP: icon review, remove in S2 Task 6`.

- [ ] **Step 4: Typecheck + suite + build**

Run: `cd console && npx tsc --noEmit && npm test && npm run build`
Expected: clean (the `Record<IconName,…>` completeness check passes only if all 15 are present); 231/231; build succeeds.

- [ ] **Step 5: VISUAL REVIEW GATE (human partner)**

Open `/_icons` on the running console. Confirm each glyph is legible at 13px, distinct from its neighbours, and obeys the collision constraints (refresh≠improve, run≠play, close≠deny≠StateIcon.deny). Refine geometry until approved. **This is the task's real acceptance gate.**

- [ ] **Step 6: Commit**

```bash
git add console/src/ui/icons.tsx console/src/pages/_IconSheet.tsx console/src/App.tsx
git commit -m "feat(console): action-icon glyph family (+ temp icon sheet)"
```

---

### Task 3: `BrandMark` logomark

**Files:**
- Modify: `console/src/ui/icons.tsx` (add `BrandMark`)
- Modify: `console/src/components/Sidebar.tsx`, `console/src/components/Sidebar.css`

**Interfaces:**
- Produces: `export function BrandMark({ size }: { size?: number }): JSX.Element` — a monochrome emblem on the house grid, built from the governance-chain motif (nodes seated on a rule). Rendered as the FIRST child of `.brand` in `Sidebar.tsx`, before `.brand-name`; the existing `gap:8px` spaces it. The teal `.live` dot stays a SEPARATE sibling (per spec recommendation — the emblem must stay monochrome; the dot is the `--live` signal).

- [ ] **Step 1: Add `BrandMark` (candidate — expect a design pass)**

```tsx
// console/src/ui/icons.tsx — export alongside Icon/StateIcon.
// Candidate: three nodes on a rule (the governance chain, compressed to a mark).
export function BrandMark({ size = 18 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none" stroke="currentColor"
      strokeWidth={1.25} strokeLinecap="square" strokeLinejoin="miter" aria-hidden focusable="false">
      <path d="M2.5 8h11" />
      <rect x="3" y="6.5" width="3" height="3" />
      <rect x="6.75" y="6.5" width="2.5" height="3" />
      <rect x="10" y="6.5" width="3" height="3" />
    </svg>
  );
}
```

- [ ] **Step 2: Slot it into the sidebar brand**

In `Sidebar.tsx`, render `<BrandMark size={18} />` as the first child of `.brand`, before `<span className="brand-name">`; keep the `.live` dot exactly where it is. Adjust `.brand`/`Sidebar.css` only if alignment needs it (reuse existing spacing tokens; no new chroma).

- [ ] **Step 3: Typecheck + build + VISUAL REVIEW GATE**

Run: `cd console && npx tsc --noEmit && npm test && npm run build` (clean; 231/231; build ok).
Then, on the running console: confirm the emblem reads well beside the `AGENTOS` wordmark at the sidebar size, is monochrome, and the live-dot still pulses separately. **Present 2–3 emblem variants for the human partner to choose** before finalizing — the logomark is a brand decision.

- [ ] **Step 4: Commit**

```bash
git add console/src/ui/icons.tsx console/src/components/Sidebar.tsx console/src/components/Sidebar.css
git commit -m "feat(console): BrandMark logomark in the sidebar brand row"
```

---

### Task 4: Swap text action buttons for icon + label

**Files:**
- Modify: `console/src/components/common.tsx` (`CopyButton`)
- Modify: `pages/Audit.tsx`, `pages/Secrets.tsx`, `pages/Documents.tsx`, `pages/Operators.tsx`, `pages/Improve.tsx`, `pages/Keys.tsx`, `pages/Users.tsx`, `pages/Provisioning.tsx`

**Interfaces:**
- Consumes: `Button` `icon`/`iconOnly` (Task 1); the action glyphs (Task 2).

Swap each text-only action for icon + label (keep the label — icon+label is more scannable than icon-only for these), preserving behavior and a11y. Concrete mapping (READ each file first; keep every handler/disabled/label unchanged except adding the glyph):

- `CopyButton` (`common.tsx:92`) → `icon="copy"` (label stays "Copy"/"Copied"). Migrate it to the `ui/Button` component if it isn't already, or add an inline `<Icon name="copy">`.
- Refresh/Reload buttons → `icon="refresh"`: `Audit.tsx` (Refresh), `Secrets.tsx` (Reload secrets / Refresh), `Documents.tsx`, `Keys.tsx` if present.
- `Operators.tsx` → Run `icon="run"`, Pause/Enable `icon="pause"`, Delete `icon="trash"` (danger variant).
- `Improve.tsx` → Approve `icon="approve"`, Deny `icon="deny"`, Run evals `icon="run"`.
- Dismiss links on created-credential panels (`Keys.tsx`, `Users.tsx`) → an `iconOnly` `close` button with `aria-label="Dismiss"` (replacing the bare `<a href="#dismiss">` text).
- `Provisioning.tsx` external ids / out-links → `icon="external-link"` (or an inline `<Icon>` on the link).
- Create actions (create key/user/operator/document) → `icon="plus"` on the primary submit/reveal button where it reads naturally (light touch; don't restructure forms).

For any `iconOnly` button, ensure an `aria-label` is present (S1 Task 10 lesson: never double-announce; keep the inner Icon decorative).

- [ ] **Step 1: Apply the swaps** (one page at a time; keep handlers/labels/variants unchanged).

- [ ] **Step 2: Typecheck + full suite + build**

Run: `cd console && npx tsc --noEmit && npm test && npm run build`
Expected: clean; 231/231 (no test references button text in a way that breaks — confirm; if a test asserts on button text, keep the label text so it still matches); build succeeds.

- [ ] **Step 3: VISUAL REVIEW GATE (human partner)** — spot-check each page: icons align with labels, danger actions read correctly, dismiss buttons are reachable/announced.

- [ ] **Step 4: Commit**

```bash
git add console/src/components/common.tsx console/src/pages/*.tsx
git commit -m "feat(console): icons on action buttons across pages"
```

---

### Task 5: Final verification

- [ ] **Step 1: Full suite + build**

Run: `cd console && npm test && npm run build`
Expected: 231/231; `tsc && vite build` clean.

- [ ] **Step 2: Chroma discipline sweep** — grep the new glyphs/BrandMark for any `--live/--ok/--hold/--deny` or literal hue: there must be none (action icons + emblem are `currentColor` only).

Run: `rg -n "live|ok|hold|deny|#[0-9a-fA-F]{3,6}|rgb" console/src/ui/icons.tsx` → expect only the pre-existing `StateIcon` state classes, nothing on the action glyphs or `BrandMark`.

- [ ] **Step 3: a11y sweep** — every `iconOnly` button has an `aria-label`/`title`; no icon beside a visible label carries its own `title` (no double-announce).

---

### Task 6: Remove the temporary icon sheet

**Files:**
- Delete: `console/src/pages/_IconSheet.tsx`
- Modify: `console/src/App.tsx` (remove the `/_icons` dev route)

- [ ] **Step 1: Remove the sheet + route** (revert the `// TEMP` App.tsx change; delete the file).
- [ ] **Step 2:** `cd console && npx tsc --noEmit && npm test && npm run build` → clean; 231/231; build ok.
- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "chore(console): remove temporary icon sheet"
```

---

## Self-Review

- **Spec coverage (§4):** the 15 action glyphs → Task 2; `Button` icon support → Task 1; `BrandMark` logomark with live-dot kept separate → Task 3; text→icon swaps at the spec's enumerated call sites → Task 4; collision constraints (refresh≠improve, run≠play, close/deny distinct, chevron factored out) → Task 2 constraints + Global Constraints. search/filter/sort/export glyphs are drawn here (Task 2) and consumed by S3.
- **Placeholder scan:** every task has concrete code (Button, sheet) or concrete candidate geometry + explicit design briefs and collision rules; the "refine for legibility" latitude is bounded by the grid rules and the visual gate, not a TODO. No "TBD".
- **Type consistency:** `IconName` additions match the `GLYPHS` keys and the `_IconSheet` `NAMES` list; `BrandMark` signature matches its Sidebar usage; `Button` `icon: IconName` matches the glyph names used at call sites.
- **Honest verification:** glyph/brand tasks are gated on tsc-completeness + build + a human visual review (not on absent unit tests); the visual gates and the temporary sheet (added Task 2, removed Task 6) are called out explicitly rather than pretending SVG art is machine-verifiable.
