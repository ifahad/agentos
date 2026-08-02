# Console Information Design (Cycle 3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every console page open on what exists and its state, rather than on a form for making a new thing, and give every empty slot an explanation instead of a blank.

**Architecture:** Two new pieces — a `summary` line on `PanelHead` and a `Disclosure` that collapses create forms — plus a convention for the `EmptyState` primitive that already exists. Applied across four page archetypes. Because this cycle changes structure rather than colour, rendering is no longer sufficient verification: the summary logic moves into pure functions with unit tests, and the render harness gains an assertion on each route's first panel heading.

**Tech Stack:** React 18 + TypeScript, Vite, vitest (node environment, `.test.ts` only — no DOM), framer-motion, plain CSS custom properties.

**Spec:** [`docs/superpowers/specs/2026-08-02-console-information-design.md`](../specs/2026-08-02-console-information-design.md)

## Global Constraints

Every task's requirements implicitly include this section.

- **No new dependencies — including test-only ones.** `console/package.json` dependencies are exactly `framer-motion`, `react`, `react-dom`. Do not add `jsdom`, `@testing-library/react`, or anything else. This has held for three cycles.
- **No new data or API calls.** Every summary derives from data the page already fetches.
- **No visual-language changes.** Tokens, colour, spacing and motion belong to Cycle 2. If something looks wrong, add a row to `docs/superpowers/specs/cycle-3-backlog.md`; do not fix it here.
- **The chroma partition still holds**, enforced by `console/src/styles.partition.test.ts`: no violet (`--v`, `--v2`, `--v3`, `--accent`) in `components/Chain.css`, `pages/Audit.css`, `charts/charts.css`, or any `.badge` rule; and `--v`/`--accent` never as the value of a `color:` declaration.
- **`color-mix(in srgb, …)` is the house idiom** for translucent variants. Never hand-expand a token to `rgba()`.
- **No `Math.random`** in animation.
- **vitest config:** `environment: "node"`, `include: ["src/**/*.test.ts"]`. Tests cannot render React. Logic that needs testing must live in a `.ts` module as a pure function.
- **Both gates, every task.** `npm run build` must exit 0 AND `npx vitest run` must pass. Baseline is 25 files / 312 tests. A green suite over a red build already shipped once on this project — run both.
- **Git:** work happens in a worktree. Use `git -C <worktree>` for every git command and absolute paths for every file operation; the shell's working directory resets between calls.

## File Structure

**Created:**

| File | Responsibility |
|---|---|
| `console/src/lib/summaries.ts` | Pure functions producing every panel summary line. No React, no fetching. |
| `console/src/lib/summaries.test.ts` | Unit tests for the above — zero, one, many, and the attention case. |
| `console/src/ui/Disclosure.tsx` | Collapsible wrapper for create forms. One boolean, motion presets, focus return. |

**Modified:**

| File | Change |
|---|---|
| `console/src/ui/Card.tsx` | `PanelHead` gains a `summary` prop |
| `console/src/ui/index.ts` | Export `Disclosure` |
| `console/src/ui/ui.css` | `.panel-head-summary`, `.ui-disclosure` styles |
| `console/src/ui/Field.tsx` | `Input` defaults to `type="text"` |
| `console/src/pages/{Keys,Documents,Operators,Orgs,Users}.tsx` | Invert; summary; disclosure; empty states |
| `console/src/pages/Multiverse.tsx` | Inner pair inverted only — page lead-in unchanged |
| `console/src/pages/{Overview,Audit,Secrets,Provisioning}.tsx` | Scaffold fix; summaries; empty-state convention |
| `console/src/pages/{Playground,Improve,Docs}.tsx` | Scaffold fix; Improve also takes the validation convention |
| `console/scripts/render.py` | First-panel-heading assertion |

## The thirteen routes

`/`, `/keys`, `/audit`, `/playground`, `/documents`, `/improve`, `/multiverse`, `/operators`, `/docs`, `/orgs`, `/users`, `/secrets`, `/provisioning`

---

### Task 0: Carried-over defects

Two items from the backlog in files this cycle opens anyway.

**Files:**
- Modify: `console/src/ui/Field.tsx`
- Modify: `console/src/pages/Users.tsx` (the `PageHead` `subtitle`)

**Interfaces:**
- Produces: `Input` renders `type="text"` when no `type` prop is given. Later tasks rely on console form styling actually applying to `<Input>`.

- [ ] **Step 1: Default the input type**

In `console/src/ui/Field.tsx`, replace the `Input` function:

```tsx
export function Input({ label, id, className = "", type = "text", ...rest }: InputProps) {
  // type defaults to "text" because styles.css keys form styling on
  // input[type="text"], input[type="number"], … — a bare <input> matches none
  // of it and falls back to user-agent styling. Since color-scheme is bound to
  // the theme, that meant UA dark fields with a 2.43:1 placeholder.
  const input = <input id={id} className={className} type={type} {...rest} />;
  if (label == null) return input;
  return (
    <label className="field" htmlFor={id}>
      <span>{label}</span>
      {input}
    </label>
  );
}
```

- [ ] **Step 2: Find the inputs this changes**

```bash
cd <worktree>/console/src && grep -rn '<Input' pages/ components/ | grep -v 'type=' 
```

Record the list in your report — these are the call sites that were previously unstyled and now pick up console styling.

- [ ] **Step 3: Fix the Users copy**

In `console/src/pages/Users.tsx`, the `PageHead` `subtitle` reads:

```
Members of an org and their roles. Inviting a user issues a one-time agu- token; removing one revokes it.
```

`agu-` is a token prefix dropped into prose. Replace the subtitle with:

```
Members of an org and their roles. Inviting a user issues a one-time token prefixed agu-; removing one revokes it.
```

- [ ] **Step 4: Both gates**

```bash
cd <worktree>/console && npm run build && npx vitest run
```

Expected: build exit 0; 25 files / 312 tests passing.

- [ ] **Step 5: Verify the styling actually applies now**

Start a dev server (`npm run dev -- --port 5199`) and, with the Playwright interpreter at `/home/iofahd/code/agentos/connectors/browser/.venv/bin/python`, load `/operators` and report the computed `background-color` and `border-color` of a previously-bare `<input>` in BOTH themes. They must now match the console's field styling rather than user-agent defaults. Report the measured placeholder contrast too — the backlog requires it be re-measured after this fix.

- [ ] **Step 6: Commit**

```bash
git add console/src/ui/Field.tsx console/src/pages/Users.tsx
git commit -m "fix(console): default Input to type=text, fix Users copy

A bare <input> matched none of styles.css's input[type=...] selectors and
fell back to user-agent styling — which, since color-scheme was bound to the
theme, meant UA dark fields with a 2.43:1 placeholder."
```

---

### Task 1: The two primitives

**Files:**
- Modify: `console/src/ui/Card.tsx` (`PanelHead`)
- Create: `console/src/ui/Disclosure.tsx`
- Modify: `console/src/ui/index.ts`
- Modify: `console/src/ui/ui.css`

**Interfaces:**
- Produces, used by Tasks 3–5:
  - `PanelHead({ title, summary?, actions? })` — `summary?: ReactNode`
  - `Disclosure({ open, onOpenChange, children, id })` where
    `open: boolean`, `onOpenChange: (next: boolean) => void`, `id: string`.
    The trigger is NOT rendered by `Disclosure` — pages put their own `<Button>`
    in `PanelHead`'s `actions` and drive `open`. This keeps the trigger in the
    panel head where the design puts it, without `Disclosure` needing to know
    about `PanelHead`.

- [ ] **Step 1: Add `summary` to `PanelHead`**

In `console/src/ui/Card.tsx`:

```tsx
export interface PanelHeadProps {
  title: ReactNode;
  /**
   * One line stating what this panel contains and whether anything needs
   * attention — "12 keys · 2 inactive". Derived from data the page already
   * has; never fetched. Omit the attention clause when the count is zero so a
   * quiet panel reads quiet.
   */
  summary?: ReactNode;
  /** Right-aligned slot (actions, badges, filters). */
  actions?: ReactNode;
}

export function PanelHead({ title, summary, actions }: PanelHeadProps) {
  return (
    <div className="panel-head">
      <div className="panel-head-text">
        <h2>{title}</h2>
        {summary != null && <div className="panel-head-summary">{summary}</div>}
      </div>
      {actions}
    </div>
  );
}
```

- [ ] **Step 2: Style it**

Append to `console/src/ui/ui.css`:

```css
/* ---- PanelHead summary ---- */

/* The title and its summary stack; actions stay on the right, so the head
   keeps its existing two-column shape. */
.panel-head-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

/* A reading, not a label: mono for the machine-produced counts, --text-dim so
   it sits under the title without competing with it. */
.panel-head-summary {
  font-family: var(--mono);
  font-size: 11.5px;
  color: var(--text-dim);
  font-variant-numeric: tabular-nums;
}
```

Check `styles.css`'s existing `.panel-head` rule (around line 468) — if it sets `align-items: center`, the stacked text will centre against the actions, which is correct. If it sets `align-items: baseline`, change nothing there; report what you found.

- [ ] **Step 3: Create the Disclosure**

Create `console/src/ui/Disclosure.tsx`:

```tsx
import { useEffect, useRef } from "react";
import type { ReactNode } from "react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { DUR_MED, EASE } from "./motion";

export interface DisclosureProps {
  /** Whether the body is shown. Owned by the page. */
  open: boolean;
  /** Called with the next state. The page also renders the trigger. */
  onOpenChange: (next: boolean) => void;
  /** Must match the trigger's aria-controls. */
  id: string;
  children: ReactNode;
}

/**
 * Collapsible body for a create form.
 *
 * The trigger lives in the panel head, not here — the design puts it beside
 * the panel title, and rendering it here would make Disclosure know about
 * PanelHead's layout. The page owns `open` and renders its own button with
 * aria-expanded and aria-controls={id}.
 */
export function Disclosure({ open, onOpenChange, id, children }: DisclosureProps) {
  const reduced = useReducedMotion();
  const bodyRef = useRef<HTMLDivElement>(null);

  // Escape closes an open form, matching the modal's affordance. The page
  // returns focus to its own trigger, which it owns.
  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onOpenChange(false);
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onOpenChange]);

  return (
    <AnimatePresence initial={false}>
      {open && (
        <motion.div
          id={id}
          ref={bodyRef}
          className="ui-disclosure"
          initial={reduced ? { opacity: 1 } : { opacity: 0, height: 0 }}
          animate={reduced ? { opacity: 1 } : { opacity: 1, height: "auto" }}
          exit={reduced ? { opacity: 1 } : { opacity: 0, height: 0 }}
          transition={{ duration: DUR_MED, ease: EASE }}
        >
          {children}
        </motion.div>
      )}
    </AnimatePresence>
  );
}
```

- [ ] **Step 4: Style and export it**

Append to `console/src/ui/ui.css`:

```css
/* ---- Disclosure ---- */

/* Height is animated, so the body must not spill while collapsing. */
.ui-disclosure {
  overflow: hidden;
}
```

Add to `console/src/ui/index.ts`:

```ts
export { Disclosure } from "./Disclosure";
export type { DisclosureProps } from "./Disclosure";
```

- [ ] **Step 5: Both gates**

```bash
cd <worktree>/console && npm run build && npx vitest run
```

Expected: build exit 0 (this catches type errors in the new component); 312 tests passing.

- [ ] **Step 6: Commit**

```bash
git add console/src/ui/Card.tsx console/src/ui/Disclosure.tsx console/src/ui/index.ts console/src/ui/ui.css
git commit -m "feat(console): add PanelHead summary and a Disclosure primitive"
```

---

### Task 2: Summary functions

Every summary line is a pure function so it can be tested — the console has no way to test React.

**Files:**
- Create: `console/src/lib/summaries.ts`
- Create: `console/src/lib/summaries.test.ts`

**Interfaces:**
- Produces, used by Tasks 3–5:
  - `countLine(total: number, noun: string, attention?: { count: number; label: string }): string`

  Plus one wrapper per page, each taking the array the page already holds and returning a string. Their exact element types come from `console/src/lib/types.ts` — read it and use the real types, never `any`.

- [ ] **Step 1: Write the failing tests**

Create `console/src/lib/summaries.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { countLine } from "./summaries";

describe("countLine", () => {
  it("pluralises the noun", () => {
    expect(countLine(0, "key")).toBe("0 keys");
    expect(countLine(1, "key")).toBe("1 key");
    expect(countLine(12, "key")).toBe("12 keys");
  });

  it("appends an attention clause when it is non-zero", () => {
    expect(countLine(12, "key", { count: 2, label: "inactive" })).toBe("12 keys · 2 inactive");
  });

  it("omits the attention clause when it is zero", () => {
    // A quiet panel must read quiet. "· 0 inactive" invites a second look at
    // something that needs none.
    expect(countLine(12, "key", { count: 0, label: "inactive" })).toBe("12 keys");
  });

  it("omits the attention clause when it is absent", () => {
    expect(countLine(3, "document")).toBe("3 documents");
  });

  it("does not pluralise the attention label", () => {
    // The label is written already-plural by the caller ("inactive", "held"),
    // because these are adjectives, not nouns.
    expect(countLine(1, "operator", { count: 1, label: "held" })).toBe("1 operator · 1 held");
  });
});
```

- [ ] **Step 2: Run and verify it fails**

```bash
cd <worktree>/console && npx vitest run src/lib/summaries.test.ts
```

Expected: FAIL — cannot resolve `./summaries`.

- [ ] **Step 3: Implement `countLine`**

Create `console/src/lib/summaries.ts`:

```ts
/**
 * Panel summary lines.
 *
 * Every function here is pure and takes data the page already holds — no
 * fetching, no React. That is deliberate: the console's vitest runs in a node
 * environment with no DOM, so logic that needs testing has to live outside
 * components. The summary line is the part worth testing; the boolean beside
 * it is not.
 */

/**
 * "12 keys · 2 inactive".
 *
 * `noun` is singular; it is pluralised by adding "s". `attention.label` is
 * written already-plural by the caller because these are adjectives
 * ("inactive", "held"), not nouns. The attention clause is omitted entirely
 * when its count is zero — a quiet panel should read quiet.
 */
export function countLine(
  total: number,
  noun: string,
  attention?: { count: number; label: string },
): string {
  const head = `${total} ${total === 1 ? noun : `${noun}s`}`;
  if (!attention || attention.count === 0) return head;
  return `${head} · ${attention.count} ${attention.label}`;
}
```

- [ ] **Step 4: Run and verify it passes**

```bash
cd <worktree>/console && npx vitest run src/lib/summaries.test.ts
```

Expected: 5 tests passing.

- [ ] **Step 5: Add the per-page wrappers, with tests**

Read `console/src/lib/types.ts` for the real element types first. Then add one wrapper per page below `countLine`, each with a matching test. Write the wrapper for `Keys` exactly like this, and follow its shape for the rest:

```ts
import type { ApiKey } from "./types";

/** "12 keys · 2 inactive" — inactive keys are the ones worth surfacing. */
export function keysSummary(keys: readonly ApiKey[]): string {
  const inactive = keys.filter((k) => !k.active).length;
  return countLine(keys.length, "key", { count: inactive, label: "inactive" });
}
```

Wrappers required, with the attention signal each should surface:

| Function | Noun | Attention clause |
|---|---|---|
| `keysSummary` | key | inactive keys |
| `documentsSummary` | document | none — a count alone |
| `operatorsSummary` | operator | operators in a held state |
| `orgsSummary` | org | orgs over budget |
| `usersSummary` | member | none — a count alone |
| `objectivesSummary` | objective | objectives with held write actions |
| `auditEventsSummary` | event | denied events |
| `budgetsSummary` | budget | budgets over their limit |

If a field the attention clause needs does not exist on the type, DO NOT invent it and DO NOT add a fetch. Fall back to a plain count and say so in your report — a wrong signal is worse than no signal.

Each wrapper gets at least three test cases: empty, one, and many-with-attention.

- [ ] **Step 6: Both gates**

```bash
cd <worktree>/console && npm run build && npx vitest run
```

Expected: build exit 0; test count risen by at least 20 from 312.

- [ ] **Step 7: Commit**

```bash
git add console/src/lib/summaries.ts console/src/lib/summaries.test.ts
git commit -m "feat(console): pure summary-line functions with tests

The console's vitest has no DOM, so the logic worth testing lives outside
components. The summary line is that logic; the disclosure boolean is not."
```

---

### Task 3: Always render the scaffold

Seven pages wrap their entire body in `{adminKey && (…)}`, so an unconfigured console shows a notice and nothing else.

**Files:**
- Modify: `console/src/pages/{Overview,Keys,Audit,Orgs,Users,Secrets,Provisioning}.tsx`

**Interfaces:**
- Consumes: `EmptyState` from `../ui` (already exists: `title`, `description?`, `action?`).
- Produces: nothing importable. Tasks 4 and 5 assume the scaffold renders unconditionally.

- [ ] **Step 1: Find every gate**

```bash
cd <worktree>/console/src && grep -n 'adminKey && (' pages/*.tsx
```

Record the list. It should be the seven pages named above; report any difference.

- [ ] **Step 2: Invert each gate**

For each page, the panels must render regardless of `adminKey`. What changes is what goes INSIDE a panel body: when there is no key, the body shows an `EmptyState` instead of data.

Worked example for `console/src/pages/Keys.tsx` — replace the `{adminKey && (` wrapper so the panels are always present:

```tsx
{canCreate && (
  <Panel>
    <PanelHead title="Create key" />
    <div className="panel-body">
      {/* form unchanged */}
    </div>
  </Panel>
)}

<Panel>
  <PanelHead title="Existing keys" />
  <div className="panel-body">
    {!adminKey ? (
      <EmptyState
        title="No admin key configured"
        description="Gateway keys and their monthly budgets appear here once the console can reach the admin API."
        action={<Button variant="primary" onClick={openSettings}>Open settings</Button>}
      />
    ) : (
      /* existing table / content unchanged */
    )}
  </div>
</Panel>
```

Do NOT remove the existing `<NeedsKey>` notice — it explains the state at page level and stays where it is.

Keep every existing behaviour: `canCreate`, `allowed`, `ForbiddenNotice`, loading skeletons and error notices all keep working exactly as before. This task moves a conditional inward; it does not change any logic.

- [ ] **Step 3: Both gates**

```bash
cd <worktree>/console && npm run build && npx vitest run
```

- [ ] **Step 4: Verify by rendering, with no admin key**

Start the dev server and drive all seven routes with the Playwright interpreter. For each, report the number of `.panel` elements present. Before this task, an unconfigured `/keys` renders 0 panels; after, it must render its full scaffold. Capture screenshots in both themes and look at them at FULL RESOLUTION — a downscaled dark screenshot washes `#07080b` toward grey and has produced false defect reports on this project twice.

- [ ] **Step 5: Commit**

```bash
git add console/src/pages
git commit -m "feat(console): render the page scaffold without an admin key

Seven pages wrapped their whole body in {adminKey && (...)}, so an evaluator
saw a notice and a blank page instead of what the page is for."
```

---

### Task 4: Invert the five List pages

**Files:**
- Modify: `console/src/pages/{Keys,Documents,Operators,Orgs,Users}.tsx`
- Modify: `console/src/pages/Multiverse.tsx` (inner pair only)

**Interfaces:**
- Consumes: `Disclosure` and `PanelHead`'s `summary` from Task 1; the summary functions from Task 2.

- [ ] **Step 1: Invert Keys, as the worked example**

In `console/src/pages/Keys.tsx`, the reading panel moves above the create panel, the create panel's body goes inside a `Disclosure`, and its trigger moves into the reading panel's head:

```tsx
const [createOpen, setCreatingOpen] = useState(false);

// …

<Panel>
  <PanelHead
    title="Existing keys"
    summary={keysSummary(keys)}
    actions={
      <span className="head-group">
        <TableToolbar query={t.query} onQuery={t.setQuery} onExport={onExport} />
        {canCreate && (
          <Button
            variant="primary"
            icon="plus"
            aria-expanded={createOpen}
            aria-controls="keys-create"
            onClick={() => setCreatingOpen((v) => !v)}
          >
            New key
          </Button>
        )}
      </span>
    }
  />
  <Disclosure open={createOpen} onOpenChange={setCreatingOpen} id="keys-create">
    <div className="panel-body">
      {/* the existing create form, moved here verbatim */}
    </div>
  </Disclosure>
  <div className="panel-body">
    {/* the existing table, unchanged */}
  </div>
</Panel>
```

The two panels become one: the reading panel absorbs the create form as a disclosure. Delete the now-empty `Create key` `<Panel>` wrapper.

After a successful create, close the disclosure (`setCreatingOpen(false)`) so the page returns to its reading state.

- [ ] **Step 2: Apply the same shape to the other four**

`Documents` (Add document → Ingested documents), `Operators` (New operator → Operators), `Orgs` (Create org → Organizations), `Users` (Invite user → Members). Each uses its own summary function from Task 2 and a unique `id` for the disclosure (`documents-create`, `operators-create`, `orgs-create`, `users-create`).

- [ ] **Step 3: Multiverse — inner pair only**

`Multiverse` already LEADS with `Council`, which is a reading. Do NOT move `Council`. Only invert the inner pair: `Objectives` moves above `New objective`, and the create form becomes a disclosure with id `multiverse-create`, exactly as above. The page's first panel stays `Council`.

- [ ] **Step 4: Add summaries to the two Reading pages**

`Overview` and `Audit` already lead with a reading, so they are NOT inverted and get no `Disclosure`. They do get summary lines, which is the rest of the pattern:

- `Audit`'s `Events` panel takes `auditEventsSummary(events)`.
- `Overview`'s `Budgets` panel takes `budgetsSummary(budgets)`.

Nothing else on either page changes. Do not reorder their panels.

- [ ] **Step 5: Both gates**

```bash
cd <worktree>/console && npm run build && npx vitest run
```

- [ ] **Step 6: Verify the order actually changed**

Render each of the six routes and report the text of the FIRST `.panel-head h2` on each. Expected: `Existing keys`, `Ingested documents`, `Operators`, `Organizations`, `Members`, and `Council` for Multiverse. If any still reports a create title, the inversion did not take.

Also verify the disclosure: with it closed, the create form's inputs must not be in the DOM; opening the trigger must reveal them; `aria-expanded` must track the state.

- [ ] **Step 7: Commit**

```bash
git add console/src/pages
git commit -m "feat(console): lead with the reading, put creation behind a disclosure

Five pages opened on a form for making a new thing while the list of what
already exists sat below it. An operator's constant activity is reading."
```

---

### Task 5: The EmptyState convention

**Files:**
- Modify: every page with an `EmptyState` call site (14 exist), plus `Keys`, `Orgs`, `Users` which have none.

**Interfaces:**
- Consumes: `EmptyState` from `../ui` — `{ title, description?, action? }`, unchanged.

- [ ] **Step 1: Inventory the call sites**

```bash
cd <worktree>/console/src && grep -rn '<EmptyState' pages/ components/
```

14 exist; 4 pass a `description`. Record which is which.

- [ ] **Step 2: Bring each to the three-part form**

Every `EmptyState` gets all three props:

| Prop | Says |
|---|---|
| `title` | what is absent — "No keys yet" |
| `description` | what will appear here, and why it matters |
| `action` | the control that fills it |

Worked example, replacing a bare title:

```tsx
<EmptyState
  title="No objectives yet"
  description="An objective is a goal you hand to the council; each one runs as cycles you can inspect and approve."
  action={<Button variant="primary" icon="plus" onClick={() => setCreatingOpen(true)}>New objective</Button>}
/>
```

Where an action genuinely does not exist — a read-only feed like `Audit` with no data — omit `action` and say why in a comment. Do not invent a control that does nothing.

Write descriptions that say what the thing IS, not that it is missing. "No documents ingested yet" already says absence; the description should explain what ingesting a document does for the operator.

- [ ] **Step 3: Add the three missing empty states**

`Keys`, `Orgs` and `Users` have no `EmptyState` at all — with an admin key configured and no rows, they render an empty table. Add one to each, in the reading panel's body, shown when the collection is empty and not loading.

- [ ] **Step 4: Both gates**

```bash
cd <worktree>/console && npm run build && npx vitest run
```

- [ ] **Step 5: Verify no bare EmptyState remains**

```bash
cd <worktree>/console/src && grep -rn -A3 '<EmptyState' pages/ components/ | grep -B1 -A2 'title=' | grep -c 'description='
```

Compare against the total call-site count. Every site must have a description; list any exception with the reason.

- [ ] **Step 6: Commit**

```bash
git add console/src/pages console/src/components
git commit -m "feat(console): empty states say what appears there, not just that it's absent"
```

---

### Task 6: The validation convention

**Files:**
- Modify: `console/src/pages/Documents.tsx`, `console/src/pages/Improve.tsx`

- [ ] **Step 1: Confirm the split**

```bash
cd <worktree>/console/src && grep -n 'disabled=' pages/Documents.tsx pages/Improve.tsx pages/Playground.tsx pages/Multiverse.tsx pages/Operators.tsx
```

`Playground`, `Multiverse` and `Operators` disable their primary action until required input is present AND while in flight. `Documents` and `Improve` disable only while in flight. Record what you find.

- [ ] **Step 2: Bring the two into line**

Extend each primary action's `disabled` expression so it also covers empty required input. For `Documents` ("Ingest document"), the required fields are the document name and its text. For `Improve` ("Run evals" / "Propose improvement"), read the component to determine which fields are genuinely required — do not guess; if a field is optional, leave it out of the condition.

Follow the exact idiom the other three pages already use rather than inventing a new one.

- [ ] **Step 3: Both gates**

```bash
cd <worktree>/console && npm run build && npx vitest run
```

- [ ] **Step 4: Verify by driving the page**

Render `/documents` and `/improve`, and report the `disabled` property of each primary action with the form empty and again with required fields filled. Empty must be `true`; filled must be `false`.

- [ ] **Step 5: Commit**

```bash
git add console/src/pages/Documents.tsx console/src/pages/Improve.tsx
git commit -m "fix(console): disable primary actions until required input is present

Documents and Improve enabled a request that could only fail. The other three
create pages already gated on input; this makes all five agree."
```

---

### Task 7: Assert what each page leads with

**Files:**
- Modify: `console/scripts/render.py`

- [ ] **Step 1: Add the expected-lead table**

In `console/scripts/render.py`, beside the existing `ROUTES` list, add:

```python
# What each route must LEAD with — the text of its first .panel-head h2.
# This is the assertion that makes "what does this page lead with" mechanical
# rather than a matter of opinion. A page that gets reinverted fails here.
EXPECTED_LEAD = {
    "/keys": "Existing keys",
    "/documents": "Ingested documents",
    "/operators": "Operators",
    "/orgs": "Organizations",
    "/users": "Members",
    "/multiverse": "Council",
}
```

Only these six are asserted. The other seven have no create/read inversion to protect, and pinning their headings would make the harness fail on ordinary copy edits.

- [ ] **Step 2: Read the lead heading during the audit**

Extend the page evaluation to also return the first panel heading:

```python
LEAD_JS = """
() => {
  const h = document.querySelector('.panel-head h2');
  return h ? h.textContent.trim() : null;
}
"""
```

Call it in `render_and_audit` after the mount check, and include the value in the returned dict.

- [ ] **Step 3: Fail the run on a wrong lead**

In `main`, after the contrast check for each route, compare against `EXPECTED_LEAD`. On mismatch, print a message that is clearly distinct from both a contrast failure and a harness failure, and count it into a separate `lead_failures` counter. Exit non-zero if any of the three counters is non-zero. Follow the existing message conventions — `FAIL` for contrast, `HARNESS FAILURE` for a route that did not render — and use a third distinct prefix here.

- [ ] **Step 4: PROVE THE ASSERTION CAN FAIL**

This is the step that matters. A harness assertion that cannot fail certifies nothing, and this project has already nearly shipped a guard test that passed vacuously by reading every file as an empty string.

Temporarily change one entry of `EXPECTED_LEAD` to a wrong value, run the harness, and confirm it names that route and exits non-zero. Then restore it and confirm exit 0. Paste both outputs into your report.

Then do the stronger version: temporarily reinvert one real page (move its create panel back above its reading panel), run, and confirm the harness catches it. Revert and confirm clean with `git status --porcelain`.

- [ ] **Step 5: Full run and both gates**

```bash
cd <worktree>/console && npm run build && npx vitest run
```
Plus the harness over all 13 routes × 2 themes, which must exit 0 with all 26 cells non-zero.

- [ ] **Step 6: Commit**

```bash
git add console/scripts/render.py
git commit -m "test(console): assert what each page leads with

A screenshot cannot tell you whether a page leads with the right thing. Six
routes now pin their first panel heading, so a reinversion fails the run."
```

---

### Task 8: Whole-branch review

- [ ] **Step 1: Assemble the package**

```bash
cd <worktree> && git diff main...HEAD --stat && git diff main...HEAD
```

- [ ] **Step 2: Request the review**

Use `superpowers:requesting-code-review` with the whole-branch diff, the spec, and this plan. Point the reviewer at the fact that this cycle changed STRUCTURE, so the questions are different from Cycle 2's: does each page lead with the right thing, does every empty state explain rather than merely report absence, and did any behaviour change that should not have.

- [ ] **Step 3: Apply fixes**

Use `superpowers:receiving-code-review`. Verify each finding before implementing it.

- [ ] **Step 4: Final gate**

Build exit 0; suite ≥332 passing; harness exits 0 with all 26 cells non-zero and all six lead assertions satisfied.

- [ ] **Step 5: Finish the branch**

Use `superpowers:finishing-a-development-branch`.
