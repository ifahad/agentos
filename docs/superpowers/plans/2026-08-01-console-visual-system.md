# Console Visual System (Cycle 2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Propagate the Cycle 1 violet design language across the console's thirteen pages, in both a dark and a light theme, without changing what any page says or how its information is ordered.

**Architecture:** The console is already ~90% token-driven, so the work is a token-layer change plus a primitive re-skin, and most pages restyle for free. Chroma is *partitioned*: violet marks interface, and the four state hues remain reserved for machine outcomes — a rule enforced by a test, not a comment. Verification is rendering every page in both themes and measuring composited contrast, because that is what caught every real defect in Cycle 1.

**Tech Stack:** React 18 + TypeScript, Vite, vitest (node environment), framer-motion, plain CSS custom properties. No CSS framework, no preprocessor.

**Spec:** [`docs/superpowers/specs/2026-08-01-console-visual-system-design.md`](../specs/2026-08-01-console-visual-system-design.md)

## Global Constraints

Every task's requirements implicitly include this section.

- **No new dependencies.** `console/package.json` dependencies are exactly `framer-motion`, `react`, `react-dom`. Adding any package fails the task.
- **The console is air-gapped.** No CDN fonts, no external requests, no network calls added.
- **`--v` is fill-only.** It measures 4.43:1 on `#07080B`, below the 4.5:1 text floor. It may never be the value of a `color:` declaration. `--v2` is the accent text token.
- **Violet is banned from** `src/components/Chain.css`, `src/pages/Audit.css`, `src/charts/charts.css`, and any rule in `styles.css` whose selector contains `.badge`.
- **No `Math.random`** in any animation. Every sequence is a fixed script or a pure function of an index.
- **`color-mix(in srgb, …)` is the house idiom** for translucent variants of a token. It is already used at `src/pages/Audit.css:11,14`. Never hand-expand a token to `rgba()`.
- **Visual only.** Nothing changes about what a page leads with, how content is ordered, or what an empty state says. Structural findings go to `docs/superpowers/specs/cycle-3-backlog.md`, never into the diff.
- **The suite stays green.** Baseline is **24 files, 305 tests** (`cd console && npx vitest run`). It must be ≥305 passing at every commit.
- **vitest config:** `environment: "node"`, `include: ["src/**/*.test.ts"]`. Tests may read files from disk with `node:fs`; they cannot render DOM.
- **Both themes, every time.** Any visual check runs twice — once per ground.
- **Git:** work happens in a worktree. Use `git -C <worktree>` for every git command and absolute paths for every file operation. The shell's working directory resets between calls.

## File Structure

**Created:**

| File | Responsibility |
|---|---|
| `console/src/styles.partition.test.ts` | The guard test. Reads CSS as text; asserts the two partition rules. |
| `console/scripts/render.py` | Render + contrast-audit harness. Drives Chromium over a route list in both themes. Used by every later task. |
| `docs/superpowers/specs/cycle-3-backlog.md` | Structural findings deferred out of this cycle. |

**Modified:**

| File | Change |
|---|---|
| `console/src/styles.css` | Token block rewritten (both grounds, aliases); header rewritten; 12 literals converted; `:focus-visible`, `a`, `.btn`, `.badge` repointed |
| `console/index.html` | `color-scheme` meta, anti-flash background, favicon ground, theme bootstrap script |
| `console/src/ui/ui.css` | 2 literals; primitive re-skin |
| `console/src/components/Sidebar.css` | 3 literals; violet active marker; header comment |
| `console/src/charts/charts.css` | 7 literals (fallback values are dark-specific) |
| `console/src/pages/{Playground,Overview,Operators,Documents,Docs}.css` | 21 literals |
| `console/src/components/Chain.css` | 2 literals; stale comment |
| `console/src/components/Chain.tsx` | Pulse counts the filtered feed |
| `console/src/lib/chain.ts` | New `chainFeedCount` helper |
| `console/src/components/SettingsModal.tsx` | Theme control |
| 11 animating files | Motion consolidation onto `ui/motion` presets |

## Conversion Inventory

**43 hardcoded colour literals**, not 31 — the spec's §2 figure counted page and component CSS only and excluded `styles.css` itself.

| File | Count |
|---|---|
| `styles.css` (below the `:root` block) | 12 |
| `pages/Playground.css` | 7 |
| `charts/charts.css` | 7 |
| `pages/Overview.css` | 3 |
| `pages/Operators.css` | 3 |
| `components/Sidebar.css` | 3 |
| `ui/ui.css` | 2 |
| `pages/Documents.css` | 2 |
| `pages/Docs.css` | 2 |
| `components/Chain.css` | 2 |

Almost all are hand-expanded **dark-theme state hues** — `rgba(90, 209, 196, 0.4)` is `--live`, `rgba(227, 168, 81, …)` is `--hold`, `rgba(226, 104, 95, …)` is `--deny`, `rgba(108, 196, 143, …)` is `--ok`. Under a light theme those tokens change value, so an unconverted literal is not a style nit — it is a **wrong colour**. This is why conversion precedes the page sweep.

## The thirteen routes

`/`, `/keys`, `/audit`, `/playground`, `/documents`, `/improve`, `/multiverse`, `/operators`, `/docs`, `/orgs`, `/users`, `/secrets`, `/provisioning`

---

### Task 0: Carried-over chain follow-ups

Two items in files this cycle opens anyway. Doing them first means `Chain.*` is touched once.

**Files:**
- Modify: `console/src/lib/chain.ts` (add helper after `latestChainState`, ~line 256)
- Modify: `console/src/components/Chain.tsx:88-97`
- Modify: `console/src/components/Chain.css:56`
- Test: `console/src/lib/chain.test.ts`

**Interfaces:**
- Consumes: `ChainEvidence`, `ADMIN_PLANE_KINDS` (module-private) from `lib/chain.ts`
- Produces: `chainFeedCount(entries: readonly ChainEvidence[]): number` — the number of entries that describe a `/v1` request, i.e. the same filtered set `latestChainState` places on the chain. Task 4 and Task 5 do not depend on this.

**NOT in scope — do not "fix" it.** `lib/types.ts:26` declares `AuditKind` with four of the gateway's seven kinds. This is deliberate and documented at `lib/chain.ts:100-104`: the chain keeps its own wider `GatewayAuditKind` precisely so the shared type — and every `Badge variant={kind}` call site — does not have to widen. Leave it alone.

- [ ] **Step 1: Write the failing test**

Append to `console/src/lib/chain.test.ts`:

```ts
describe("chainFeedCount", () => {
  it("counts only entries that describe a /v1 request", () => {
    const entries: ChainEvidence[] = [
      { status: 200, kind: "chat" },
      { status: 200, kind: "secret_reload" },
      { status: 200, kind: "embeddings" },
    ];
    expect(chainFeedCount(entries)).toBe(2);
  });

  it("returns 0 for an admin-plane-only feed", () => {
    expect(chainFeedCount([{ status: 200, kind: "secret_reload" }])).toBe(0);
  });

  it("agrees with latestChainState on what it ignores", () => {
    // A feed that grew by an admin-plane row alone places nothing on the
    // chain, so it must not be reported as growth either.
    const before: ChainEvidence[] = [{ status: 200, kind: "chat" }];
    const after: ChainEvidence[] = [{ status: 200, kind: "secret_reload" }, ...before];
    expect(latestChainState(after)).toEqual(latestChainState(before));
    expect(chainFeedCount(after)).toBe(chainFeedCount(before));
  });
});
```

Add `chainFeedCount` to the existing import from `./chain` at the top of the file.

- [ ] **Step 2: Run the test and verify it fails**

```bash
cd /home/iofahd/code/agentos/console && npx vitest run src/lib/chain.test.ts
```

Expected: FAIL — `chainFeedCount is not a function`.

- [ ] **Step 3: Implement the helper**

In `console/src/lib/chain.ts`, immediately after `latestChainState`:

```ts
/**
 * How many entries in this feed describe a `/v1` request.
 *
 * The chain lights from the filtered feed, so anything watching the feed for
 * *new traffic* has to filter identically. Counting raw rows would announce a
 * request whenever an operator clicks Reload on the Secrets page — traffic the
 * chain itself, correctly, refuses to place.
 */
export function chainFeedCount(entries: readonly ChainEvidence[]): number {
  return entries.filter((entry) => !ADMIN_PLANE_KINDS.has(entry.kind)).length;
}
```

- [ ] **Step 4: Run the test and verify it passes**

```bash
cd /home/iofahd/code/agentos/console && npx vitest run src/lib/chain.test.ts
```

Expected: PASS.

- [ ] **Step 5: Use the helper in the component**

In `console/src/components/Chain.tsx`, add `chainFeedCount` to the existing import from `../lib/chain`, then replace lines 88-97:

```tsx
    if (!entries) return;
    // Growth in the feed means new traffic since the last poll; that is the
    // only thing that lights the chain. Count the same filtered set the chain
    // draws from — an admin-plane row is not traffic.
    const count = chainFeedCount(entries);
    if (seenCount.current !== null && count > seenCount.current) {
      activeUntil.current = Date.now() + LINGER_MS;
    }
    seenCount.current = count;
```

- [ ] **Step 6: Fix the stale comment**

In `console/src/components/Chain.css:56`, replace:

```css
/* Equal stages keep the fill fraction (cleared / 5) landing exactly on a
```

with:

```css
/* Equal stages keep the fill fraction (cleared / 6) landing exactly on a
```

- [ ] **Step 7: Run the full suite**

```bash
cd /home/iofahd/code/agentos/console && npx vitest run
```

Expected: 24 files, ≥308 tests, all passing.

- [ ] **Step 8: Commit**

```bash
git add console/src/lib/chain.ts console/src/lib/chain.test.ts \
        console/src/components/Chain.tsx console/src/components/Chain.css
git commit -m "fix(console): pulse the chain from the filtered feed

An admin-plane row (secret_reload) grew the raw entry count and lit the
in-flight pulse, announcing a request the chain itself refuses to place.
Count the same filtered set the chain draws from."
```

---

### Task 1: Token layer, both grounds

**Files:**
- Modify: `console/src/styles.css:17-74` (the `:root` block)
- Modify: `console/index.html`
- Modify: `console/src/components/SettingsModal.tsx`

**Interfaces:**
- Produces: the token names every later task uses — `--v`, `--v2`, `--v3` (violet ladder); the aliases `--ink`, `--dim`, `--faint`, `--line`, `--line2`, `--raised2`; and the `data-theme` attribute contract on `<html>` (`"dark"` | `"light"`).

- [ ] **Step 1: Rewrite the surface and add the violet ladder**

In `console/src/styles.css`, replace the surface block at lines 19-26:

```css
  /* ---- Surfaces: near-black ground, panels lifted off it. Matches
     landing/index.html so moving between the two reads as one product. ---- */
  --bg: #07080b;
  --raised: #0e1015;
  --raised-2: #151821;
  --inset: #040507;
  /* Legacy aliases (pages migrate incrementally) */
  --bg-raised: var(--raised);
  --bg-inset: var(--inset);
```

Then replace the `--accent` block at lines 50-53:

```css
  /* ---- INTERFACE. Violet marks what is interface: controls, navigation,
     links, focus, headings. It is the other half of the partition — see the
     header comment. ---- */
  --v: #7a5af8;   /* FILL ONLY. 4.43:1 on --bg, below the text floor. */
  --v2: #a78bfa;  /* accent text, eyebrows, active labels. 7.36:1. */
  --v3: #c7b6ff;  /* display type. 11.03:1. */

  /* --accent is the pre-partition name. It now resolves to the violet fill so
     the ten existing call sites pick up the accent without being rewritten. */
  --accent: var(--v);
  --accent-dim: color-mix(in srgb, var(--v) 16%, transparent);
```

- [ ] **Step 2: Add the cross-surface aliases**

Append inside the same `:root` block, before its closing brace:

```css
  /* ---- Aliases for the landing page's token names, so a rule copied from
     landing/index.html works here unchanged and vice versa. ---- */
  --ink: var(--text);
  --dim: var(--text-dim);
  --faint: var(--text-faint);
  --line: var(--border);
  --line2: var(--border-strong);
  --raised2: var(--raised-2);
```

- [ ] **Step 3: Add the light ground**

Immediately after the closing `}` of `:root`, add:

```css
/* ---- Light ground. Values are Cycle 1's tuned light palette; the violet
   ladder is computed for this ground and INVERTS — on a dark ground emphasis
   gets brighter, here it gets darker. Derivation is recorded in the spec,
   §4.3: hue locked to 252.2deg at saturation 0.637, value walked down until
   the colour clears 4.5:1 both as a fill under a white label and as text on
   --bg. #6D50DE is the first passing value; the source #7A5AF8 fails at
   3.76:1 and cannot be reused. ---- */
:root[data-theme="light"] {
  --bg: #eceae4;
  --raised: #f6f5f1;
  --raised-2: #ffffff;
  --inset: #e4e1d9;
  --border: rgba(0, 0, 0, 0.1);
  --border-strong: rgba(0, 0, 0, 0.18);
  --text: #1a1d1f;
  --text-dim: #565d63;
  --text-faint: #8a9198;

  --live: #12897c;
  --ok: #2f8f57;
  --hold: #a8721c;
  --deny: #bb4038;

  --v: #6d50de;  /* white-on-it 5.45:1, on --bg 4.53:1 */
  --v2: #5f46c2; /* on --bg 5.56:1 */
  --v3: #503ba3; /* on --bg 6.99:1 */
}

@media (prefers-color-scheme: light) {
  /* Default for anyone who has not chosen. An explicit data-theme always
     wins, because it is set on the same element with higher specificity. */
  :root:not([data-theme]) {
    --bg: #eceae4;
    --raised: #f6f5f1;
    --raised-2: #ffffff;
    --inset: #e4e1d9;
    --border: rgba(0, 0, 0, 0.1);
    --border-strong: rgba(0, 0, 0, 0.18);
    --text: #1a1d1f;
    --text-dim: #565d63;
    --text-faint: #8a9198;

    --live: #12897c;
    --ok: #2f8f57;
    --hold: #a8721c;
    --deny: #bb4038;

    --v: #6d50de;
    --v2: #5f46c2;
    --v3: #503ba3;
  }
}
```

- [ ] **Step 4: Repoint focus and links onto the partition**

`:focus-visible` currently uses `--live`, a *state* hue on interface chrome — a violation of the rule this cycle establishes. In `console/src/styles.css`, replace lines 121-138:

```css
/* Links carry the interface accent and an underline. Under the partition a
   link is interface, so violet is correct here and a state hue would not be. */
a {
  color: var(--v2);
  text-decoration: underline;
  text-decoration-color: color-mix(in srgb, var(--v2) 45%, transparent);
  text-underline-offset: 3px;
  transition: text-decoration-color var(--dur-fast) var(--ease);
}

a:hover {
  text-decoration-color: var(--v2);
}

/* Focus reports where input will land. It is interface, not machine state:
   before the partition this used --live, which made a focus ring read as an
   in-flight signal. */
:focus-visible {
  outline: 1px solid var(--v);
  outline-offset: 2px;
  border-radius: 2px;
}
```

- [ ] **Step 5: Make the shell theme-aware**

In `console/index.html`, replace the `color-scheme` meta and the inline anti-flash style:

```html
    <meta name="color-scheme" content="light dark" />
    <!-- Paint the ground before any CSS arrives: an instrument must never
         flash the wrong colour while it loads. Mirrors --bg in src/styles.css
         for both grounds. -->
    <style>
      html {
        background: #07080b;
      }
      @media (prefers-color-scheme: light) {
        html:not([data-theme]) {
          background: #eceae4;
        }
      }
      html[data-theme="light"] {
        background: #eceae4;
      }
    </style>
    <script>
      // Applied before first paint so a stored choice never flashes the other
      // ground. Wrapped because localStorage throws in some privacy modes.
      try {
        var t = localStorage.getItem("agentos-theme");
        if (t === "light" || t === "dark") {
          document.documentElement.setAttribute("data-theme", t);
        }
      } catch (e) {}
    </script>
```

Also replace the two `%23121517` occurrences in the favicon `href` with `%2307080b`.

- [ ] **Step 6: Add the theme control**

In `console/src/components/SettingsModal.tsx`, add above the existing key field:

```tsx
      <div className="field">
        <span className="eyebrow">Appearance</span>
        <div className="theme-toggle">
          {(["dark", "light"] as const).map((t) => (
            <button
              key={t}
              type="button"
              className={`btn small${theme === t ? " primary" : ""}`}
              aria-pressed={theme === t}
              onClick={() => setTheme(t)}
            >
              {t}
            </button>
          ))}
        </div>
      </div>
```

and, at the top of the component body:

```tsx
  const [theme, setThemeState] = useState<"dark" | "light">(
    () => (document.documentElement.getAttribute("data-theme") as "dark" | "light" | null) ?? "dark",
  );

  function setTheme(next: "dark" | "light") {
    document.documentElement.setAttribute("data-theme", next);
    try {
      localStorage.setItem("agentos-theme", next);
    } catch {
      // Privacy mode: the choice applies for this session only.
    }
    setThemeState(next);
  }
```

Import `useState` from react if it is not already imported.

- [ ] **Step 7: Verify the build and suite**

```bash
cd /home/iofahd/code/agentos/console && npm run build && npx vitest run
```

Expected: build succeeds; ≥308 tests pass.

- [ ] **Step 8: Commit**

```bash
git add console/src/styles.css console/index.html console/src/components/SettingsModal.tsx
git commit -m "feat(console): violet token layer and a light ground

Adopts the landing page's near-black ground and violet ladder, adds a
computed light palette, a prefers-color-scheme default and a persisted
toggle. Focus rings move off --live: a state hue on interface chrome made
a focus ring read as an in-flight signal."
```

---

### Task 2: The guard test and the rewritten headers

**Files:**
- Create: `console/src/styles.partition.test.ts`
- Modify: `console/src/styles.css:1-13` (header)
- Modify: `console/src/components/Sidebar.css:41-42` (comment)

**Interfaces:**
- Consumes: the token names from Task 1.
- Produces: nothing importable. The test constrains every later task.

- [ ] **Step 1: Write the guard test**

Create `console/src/styles.partition.test.ts`:

```ts
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = join(process.cwd(), "src");

/** Every .css file under src/, as [relative path, contents]. */
function cssFiles(): Array<[string, string]> {
  const out: Array<[string, string]> = [];
  (function walk(dir: string) {
    for (const name of readdirSync(dir)) {
      const full = join(dir, name);
      if (statSync(full).isDirectory()) walk(full);
      else if (name.endsWith(".css")) out.push([full.slice(SRC.length + 1), readFileSync(full, "utf8")]);
    }
  })(SRC);
  return out;
}

/** --v, --v2 or --v3, but not --vertical and not --v-something. */
const VIOLET = /--v[23]?\b/;

/** Surfaces that depict machine outcomes. Chroma there means state, not brand. */
const INSTRUMENT_FILES = ["components/Chain.css", "pages/Audit.css", "charts/charts.css"];

describe("chroma partition", () => {
  it("finds the instrument files it is meant to guard", () => {
    const present = cssFiles().map(([p]) => p);
    for (const f of INSTRUMENT_FILES) expect(present).toContain(f);
  });

  it("keeps violet off instrument surfaces", () => {
    const offenders: string[] = [];
    for (const [path, css] of cssFiles()) {
      if (!INSTRUMENT_FILES.includes(path)) continue;
      css.split("\n").forEach((line, i) => {
        if (VIOLET.test(line)) offenders.push(`${path}:${i + 1} ${line.trim()}`);
      });
    }
    expect(offenders, "these files depict machine state; violet is banned there").toEqual([]);
  });

  it("keeps violet out of badge rules", () => {
    const css = readFileSync(join(SRC, "styles.css"), "utf8");
    // Crude but sufficient: split on }, keep blocks whose selector names .badge.
    for (const block of css.split("}")) {
      const selector = block.slice(block.lastIndexOf("*/") + 1).split("{")[0] ?? "";
      if (!selector.includes(".badge")) continue;
      const body = block.split("{")[1] ?? "";
      expect(VIOLET.test(body), `badge rule "${selector.trim()}" must stay monochrome or signal`).toBe(false);
    }
  });

  it("never uses --v as a text colour", () => {
    // --v is 4.43:1 on --bg, below the 4.5:1 floor. It is a fill.
    const COLOR_DECL = /(?<![-\w])color\s*:\s*[^;{}]*var\(\s*--v\s*\)/;
    const offenders: string[] = [];
    for (const [path, css] of cssFiles()) {
      css.split("\n").forEach((line, i) => {
        if (COLOR_DECL.test(line)) offenders.push(`${path}:${i + 1} ${line.trim()}`);
      });
    }
    expect(offenders, "--v is fill-only; use --v2 for text").toEqual([]);
  });
});
```

- [ ] **Step 2: Run it and verify it passes against Task 1's output**

```bash
cd /home/iofahd/code/agentos/console && npx vitest run src/styles.partition.test.ts
```

Expected: 4 tests pass. If "keeps violet off instrument surfaces" fails, Task 1 leaked violet into an instrument file — fix that, not the test.

- [ ] **Step 3: Prove the guard actually catches a violation**

Temporarily append `.chain-probe { color: var(--v2); }` to `console/src/components/Chain.css`, re-run the test, confirm it FAILS, then delete the line and confirm it passes again. A guard that cannot fail is not a guard.

- [ ] **Step 4: Rewrite the styles.css header**

Replace `console/src/styles.css:1-13` entirely:

```css
/* AgentOS Console — "Instrument".
 *
 * The premise of the platform is that nothing runs unauthorized, unattributed,
 * or unrecorded. The interface is built to match: it is a panel you read, not a
 * product you are sold.
 *
 * THE RULE THAT DEFINES THIS DESIGN: chroma is PARTITIONED. Violet means "this
 * is interface" — navigation, controls, links, focus, headings. The four state
 * hues (--live, --ok, --hold, --deny) mean "this is a machine outcome" and
 * appear only where a real outcome is depicted. A violet button and a red chain
 * node can share a screen without ambiguity because they speak different
 * languages.
 *
 * This replaces an earlier rule that reserved chroma outright and forbade any
 * accent. That rule protected something real — a quiet screen reads as a
 * healthy one — and the partition keeps it where it matters: instrument
 * surfaces stay graphite-plus-signal. What changed is that the protection is
 * now enforced by styles.partition.test.ts rather than by this comment. The
 * dead `--accent: var(--text)` alias this file used to carry is what an
 * unenforced convention decays into.
 */
```

- [ ] **Step 5: Fix the Sidebar comment**

In `console/src/components/Sidebar.css`, replace lines 41-42:

```css
/* Selection is the interface speaking about itself, so it carries the violet
   accent rather than a state hue — a teal marker would read as "in flight". */
```

- [ ] **Step 6: Run the full suite and commit**

```bash
cd /home/iofahd/code/agentos/console && npx vitest run
git add console/src/styles.partition.test.ts console/src/styles.css console/src/components/Sidebar.css
git commit -m "test(console): enforce the chroma partition

Two assertions: violet never reaches Chain, Audit, charts or badge rules,
and --v is never a text colour. Rewrites the two file headers that still
instructed the opposite of what the console now does."
```

---

### Task 3: Convert the 43 literals

Every literal here is a hand-expanded token. Under the light ground the tokens change value, so an unconverted literal renders a **wrong colour**, not merely an inconsistent one.

**Files:**
- Modify: `console/src/styles.css` (12), `console/src/pages/Playground.css` (7), `console/src/charts/charts.css` (7), `console/src/pages/Overview.css` (3), `console/src/pages/Operators.css` (3), `console/src/components/Sidebar.css` (3), `console/src/ui/ui.css` (2), `console/src/pages/Documents.css` (2), `console/src/pages/Docs.css` (2), `console/src/components/Chain.css` (2)

**Interfaces:**
- Consumes: tokens from Task 1. Produces: nothing importable.

- [ ] **Step 1: Convert the pulse keyframes**

Five files animate a ring pulse using `rgba(90, 209, 196, …)`, which is `--live` expanded by hand. In `Sidebar.css:21-27`, `Overview.css:65-73`, `Operators.css:137-143`, `Documents.css:16-19` and `Playground.css:35-53`, replace each `rgba(...)` with a `color-mix`. The Sidebar case, in full:

```css
@keyframes live-pulse {
  0% {
    box-shadow: 0 0 0 0 color-mix(in srgb, var(--live) 40%, transparent);
  }
  70% {
    box-shadow: 0 0 0 6px color-mix(in srgb, var(--live) 0%, transparent);
  }
  100% {
    box-shadow: 0 0 0 0 color-mix(in srgb, var(--live) 0%, transparent);
  }
}
```

Apply the same substitution everywhere, preserving each site's own percentage and radius: `rgba(90,209,196,X)` → `color-mix(in srgb, var(--live) {X*100}%, transparent)`; `rgba(227,168,81,X)` → `--hold`; `rgba(226,104,95,X)` → `--deny`; `rgba(108,196,143,X)` → `--ok`; `rgba(216,96,96,X)` → `--deny`; `rgba(210,162,74,X)` → `--hold`.

- [ ] **Step 2: Convert the two remaining non-state literals**

`Playground.css:67` — the skeleton shimmer highlight:

```css
  background: linear-gradient(
    90deg,
    var(--inset) 25%,
    color-mix(in srgb, var(--text) 8%, var(--inset)) 50%,
    var(--inset) 75%
  );
```

`styles.css:775` and `styles.css:818` — the modal scrim, which must deepen on light ground rather than stay a dark-ground value:

```css
  background: color-mix(in srgb, var(--bg) 70%, transparent);
```

- [ ] **Step 3: Fix the chart fallback values**

`charts/charts.css` uses `var(--token, <dark literal>)` fallbacks at lines 10, 18, 77, 85, 103, 109. The fallbacks only fire if the token is missing, which now cannot happen, and they encode dark-ground values. Delete each fallback, leaving `var(--border)`, `var(--text-faint)`, `var(--text-dim)`, `var(--inset)`. Convert line 30's `rgba(255, 255, 255, 0.05)` to `color-mix(in srgb, var(--text) 5%, transparent)`.

- [ ] **Step 4: Fix the ui.css literals**

`ui/ui.css:95` — the toast shadow, which must lighten on the light ground:

```css
  box-shadow: 0 8px 24px color-mix(in srgb, var(--bg) 55%, transparent);
```

`ui/ui.css:111`:

```css
  border-color: color-mix(in srgb, var(--deny) 50%, transparent);
```

- [ ] **Step 5: Verify no literals remain**

Two checks, because `styles.css` legitimately contains literals (they are the token *definitions*) and no other file does.

```bash
# 1. No CSS file except styles.css may contain any colour literal.
cd <worktree>/console/src && \
  grep -nE '#[0-9a-fA-F]{3,8}\b|rgba?\(' \
  $(find . -name '*.css' ! -name 'styles.css' ! -name 'fonts.css')
```

Expected: **no output at all** (grep exits 1).

```bash
# 2. In styles.css, every literal must sit inside a token-definition block.
cd <worktree>/console/src && \
  awk '/^:root|^\[data-theme|^@media|^\}/ { blk = $0 } \
       /#[0-9a-fA-F]{3,8}|rgba?\(/ { print NR": "blk" | "$0 }' styles.css
```

Expected: every line reports a `blk` of `:root`, `:root[data-theme="light"]` or `@media (prefers-color-scheme: light)`. **One deliberate exception:** `.btn.primary { color: #ffffff }` from Task 4, which is a literal on purpose — `--text` follows the theme and would turn near-black on the light ground, where the violet fill stays dark. If Task 4 has not run yet, that line will not be present.

- [ ] **Step 6: Run the suite and commit**

```bash
cd /home/iofahd/code/agentos/console && npx vitest run
git add console/src
git commit -m "refactor(console): convert 43 colour literals onto tokens

Nearly all were hand-expanded state hues — rgba(90,209,196,.4) is --live.
Under the light ground those tokens change value, so each literal was a
latent wrong colour rather than a style nit. color-mix() throughout, the
idiom Audit.css already used."
```

---

### Task 4: Primitive re-skin

**Files:**
- Modify: `console/src/styles.css` (`.btn` block ~line 488, `.nav-pill` consumers)
- Modify: `console/src/ui/ui.css` (`.ui-tab`, `.ui-tab-pill`)
- Modify: `console/src/components/Sidebar.css` (`.nav-pill`)

**Interfaces:**
- Consumes: tokens from Task 1, guarded by Task 2. Produces: nothing importable.

- [ ] **Step 1: Primary button becomes a violet fill**

In `console/src/styles.css`, replace the `.btn.primary` block:

```css
.btn.primary {
  background: var(--v);
  border-color: var(--v);
  /* White, not --text: --text follows the theme and would turn near-black on
     the light ground, where the violet fill stays dark. White on --v is
     4.52:1 dark and 5.45:1 light. */
  color: #ffffff;
  font-weight: 600;
}
```

- [ ] **Step 2: Ghost button gains a violet hover**

```css
.btn:hover {
  /* Solid, not a mix into --border-strong: that token is translucent, so
     mixing into it lets the button's own ground bleed through and drops the
     edge to 2.55:1 — below the 3:1 non-text floor, on the only hover cue
     plain buttons have. --v is opaque and clears the floor on every surface
     a button sits on: 4.21-4.51:1 dark, 4.17-5.00:1 light. */
  border-color: var(--v);
}
```

- [ ] **Step 3: Sidebar active marker becomes violet**

The sliding `layoutId="nav-pill"` marker already exists — only its colour changes. In `console/src/components/Sidebar.css`:

```css
.nav-pill {
  position: absolute;
  inset: 0;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--v) 12%, var(--raised-2));
  box-shadow: inset 2px 0 0 0 var(--v);
}
```

and the active label and icon take the accent text token:

```css
.nav-item.active .nav-item-label,
.nav-item.active .nav-icon {
  color: var(--v2);
}
```

- [ ] **Step 4: Tabs take the accent**

In `console/src/ui/ui.css`, the active tab:

```css
.ui-tab.active {
  color: var(--v2);
}

.ui-tab-pill {
  background: var(--v);
}
```

- [ ] **Step 5: Table sort indicator takes the accent**

In `console/src/styles.css`, the sortable-header indicator:

```css
.sortable-th[aria-sort] .sort-indicator {
  color: var(--v2);
}
```

Find the existing selector first — `grep -n 'sort' console/src/styles.css console/src/ui/ui.css` — and repoint whatever it actually is rather than adding a new rule. Field focus rings need no work here: they inherit the global `:focus-visible` from Task 1.

- [ ] **Step 6: Verify the guard still passes**

```bash
cd /home/iofahd/code/agentos/console && npx vitest run src/styles.partition.test.ts
```

Expected: 4 tests pass. `Badge` and `StateIcon` were not touched — all eleven badge variants are machine states, so the partition costs nothing there.

- [ ] **Step 7: Run the suite and commit**

```bash
cd /home/iofahd/code/agentos/console && npm run build && npx vitest run
git add console/src
git commit -m "feat(console): re-skin primitives onto the violet accent"
```

---

### Task 5: Motion consolidation

`ui/motion.ts` already exports `DUR_FAST`, `DUR_MED`, `DUR_PAGE`, `EASE`, `STAGGER`, `STAGGER_MAX_ITEMS`, `transition`, `transitionFast`, `fadeRise`, `fadeRiseReduced`, `fade`, `staggerContainer`, `staggerItem`, `staggerItemReduced`, `modalBackdrop`, `modalPanel`, `modalPanelReduced`, `toastItem`, `toastItemReduced`, `pageTransition` — all re-exported from `ui/index.ts`. Only `ui/Toast.tsx` and `ui/Tabs.tsx` use them.

**Files:**
- Modify: `App.tsx`, `pages/Overview.tsx`, `pages/Keys.tsx`, `pages/Docs.tsx`, `pages/Improve.tsx`, `pages/Playground.tsx`, `components/Sidebar.tsx`, `components/Chain.tsx`, `components/LiveList.tsx`, `components/CommandPalette.tsx`, `pages/docs/visuals/GovernanceChainVisual.tsx`

**Interfaces:**
- Consumes: the preset names listed above, imported from `../ui` (or `./ui` from `App.tsx`).
- Produces: nothing importable.

- [ ] **Step 1: Inventory the inline variants**

```bash
cd /home/iofahd/code/agentos/console/src && \
  grep -nE 'initial=|animate=|exit=|transition=\{|variants=' \
  App.tsx pages/*.tsx components/*.tsx pages/docs/visuals/*.tsx
```

Record every site. Each one either maps to an existing preset or is genuinely bespoke.

- [ ] **Step 2: Replace each mappable site**

For an entrance that fades and rises, replace the inline form:

```tsx
<motion.div initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.2 }}>
```

with the preset:

```tsx
<motion.div variants={reduced ? fadeRiseReduced : fadeRise} initial="hidden" animate="visible">
```

where `reduced` is the existing `useReducedMotion()` result in that component. Import `fadeRise`, `fadeRiseReduced` from `../ui`.

For a staggered list, use `staggerContainer` on the parent and `staggerItem` (or `staggerItemReduced`) on each child, capping animated children at `STAGGER_MAX_ITEMS`.

For a bare duration, replace `transition={{ duration: 0.12 }}` with `transition={transitionFast}` and `transition={{ duration: 0.2 }}` with `transition={transition}`.

- [ ] **Step 3: Leave genuinely bespoke motion alone, but document it**

If a site's motion has no matching preset — a chain conveyor, a sparkline draw — leave it and add a one-line comment saying why it is not a preset. Do not invent new presets in this task; that is authoring, and this task is consolidation.

- [ ] **Step 4: Verify no raw durations remain**

```bash
cd /home/iofahd/code/agentos/console/src && \
  grep -nE 'duration:\s*0?\.[0-9]+' App.tsx pages/*.tsx components/*.tsx
```

Expected: no hits, or only hits inside a block carrying the Step 3 comment.

- [ ] **Step 5: Confirm reduced motion still holds**

```bash
cd /home/iofahd/code/agentos/console/src && grep -c 'useReducedMotion' App.tsx pages/*.tsx components/*.tsx
```

Expected: every file that animates still calls it. A file that lost its call during consolidation is a regression.

- [ ] **Step 6: Build, test, commit**

```bash
cd /home/iofahd/code/agentos/console && npm run build && npx vitest run
git add console/src
git commit -m "refactor(console): route animation through the shared presets

ui/motion.ts had two consumers while eleven files hand-rolled variants
around it. No new animation is authored."
```

---

### Task 6: The page sweep

**Files:**
- Create: `console/scripts/render.py`
- Create: `docs/superpowers/specs/cycle-3-backlog.md`
- Modify: whichever page CSS the renders expose

**Interfaces:**
- Consumes: everything above. Produces: the render harness, reused by Task 7.

- [ ] **Step 1: Write the render harness**

Create `console/scripts/render.py`:

```python
"""Render every console route in both themes and audit text contrast.

Run against a dev server:  npm run dev -- --port 5199
Then:  connectors/browser/.venv/bin/python console/scripts/render.py http://localhost:5199 out/

getComputedStyle reports `color` ignoring `opacity`, so a ratio computed from
it alone overstates contrast on anything faded. Every measurement here
composites opacity against the resolved background first.
"""
import sys, pathlib
from playwright.sync_api import sync_playwright

ROUTES = ["/", "/keys", "/audit", "/playground", "/documents", "/improve",
          "/multiverse", "/operators", "/docs", "/orgs", "/users", "/secrets",
          "/provisioning"]

AUDIT_JS = r"""
() => {
  const lum = (r,g,b) => { const f = c => { c/=255; return c<=0.03928 ? c/12.92 : Math.pow((c+0.055)/1.055,2.4); };
    return 0.2126*f(r)+0.7152*f(g)+0.0722*f(b); };
  const parse = s => (s.match(/[\d.]+/g)||[]).map(Number);
  const over = (fg, bg, a) => fg.map((c,i) => c*a + bg[i]*(1-a));
  const bgOf = el => { let n = el; while (n) { const c = parse(getComputedStyle(n).backgroundColor);
      if (c.length >= 3 && (c[3] === undefined || c[3] > 0)) return c.slice(0,3); n = n.parentElement; }
    return [0,0,0]; };
  const out = [];
  for (const el of document.querySelectorAll('*')) {
    const t = [...el.childNodes].some(n => n.nodeType === 3 && n.textContent.trim());
    if (!t) continue;
    const cs = getComputedStyle(el);
    if (cs.visibility === 'hidden' || cs.display === 'none') continue;
    const a = parseFloat(cs.opacity);
    const bg = bgOf(el);
    const fg = over(parse(cs.color).slice(0,3), bg, isNaN(a) ? 1 : a);
    const L1 = lum(...fg), L2 = lum(...bg);
    const ratio = (Math.max(L1,L2)+0.05)/(Math.min(L1,L2)+0.05);
    const size = parseFloat(cs.fontSize), bold = parseInt(cs.fontWeight) >= 700;
    const floor = (size >= 24 || (size >= 18.66 && bold)) ? 3.0 : 4.5;
    if (ratio < floor) out.push({tag: el.tagName, cls: el.className, text: el.textContent.trim().slice(0,40),
                                 ratio: +ratio.toFixed(2), floor, size});
  }
  return out;
}
"""

def main():
    base, outdir = sys.argv[1], pathlib.Path(sys.argv[2])
    outdir.mkdir(parents=True, exist_ok=True)
    failures = 0
    with sync_playwright() as p:
        b = p.chromium.launch()
        for theme in ("dark", "light"):
            for route in ROUTES:
                pg = b.new_page(viewport={"width": 1440, "height": 900})
                pg.goto(base + route, wait_until="networkidle")
                pg.evaluate(f"document.documentElement.setAttribute('data-theme','{theme}')")
                pg.wait_for_timeout(600)
                name = (route.strip("/") or "overview").replace("/", "-")
                pg.screenshot(path=str(outdir / f"{theme}-{name}.png"), full_page=True)
                for bad in pg.evaluate(AUDIT_JS):
                    failures += 1
                    print(f"FAIL {theme}{route}: {bad['ratio']}:1 < {bad['floor']} "
                          f"{bad['tag']}.{bad['cls']} {bad['size']}px {bad['text']!r}")
                pg.close()
        b.close()
    print(f"\n{failures} contrast failures")
    sys.exit(1 if failures else 0)

main()
```

- [ ] **Step 2: Start a dev server and run it**

```bash
cd /home/iofahd/code/agentos/console && (npm run dev -- --port 5199 &) && sleep 4
/home/iofahd/code/agentos/connectors/browser/.venv/bin/python \
  /home/iofahd/code/agentos/console/scripts/render.py http://localhost:5199 /tmp/console-renders
```

Expected on first run: some failures. That is the point.

- [ ] **Step 3: Fix every contrast failure**

Each failure names the element, its computed ratio, its floor and its theme. Fix by moving the element to a higher-contrast token — never by changing a token's value, which would invalidate Task 1's recorded arithmetic. Re-run until the harness exits 0.

- [ ] **Step 4: Look at all 26 screenshots**

Read every PNG in `/tmp/console-renders`. Arithmetic does not catch a collapsed layout, an invisible element, or a violet that landed on the wrong surface. **Read them at full resolution** — a downscaled dark-theme screenshot washes `#07080b` toward white and will make you file a defect that does not exist.

- [ ] **Step 5: Log structural findings, do not fix them**

Create `docs/superpowers/specs/cycle-3-backlog.md`:

```markdown
# Cycle 3 backlog — console information design

Structural findings noticed during the Cycle 2 visual sweep. None were fixed
there: Cycle 2 is visual-only, which is what keeps rendering sufficient as its
verification.

| Page | Finding |
|---|---|
```

Append one row per structural problem seen. Anything about what a page leads with, how it orders content, or what an empty state says goes here — not into the diff.

- [ ] **Step 6: Commit**

```bash
cd /home/iofahd/code/agentos/console && npx vitest run
git add console/scripts/render.py console/src docs/superpowers/specs/cycle-3-backlog.md
git commit -m "feat(console): page sweep, both themes

Adds the render + composited-contrast harness and fixes what it found across
all thirteen routes in both grounds. Structural findings logged to the Cycle 3
backlog rather than fixed."
```

---

### Task 7: Whole-branch review

**Files:** none created or modified by default.

- [ ] **Step 1: Assemble the review package**

```bash
cd /home/iofahd/code/agentos && git diff main...HEAD --stat && git diff main...HEAD
```

- [ ] **Step 2: Request the review**

Use `superpowers:requesting-code-review` with the whole-branch diff, the spec, and this plan. The reviewer must verify by rendering, not only by reading — every real defect in Cycle 1 was found by looking.

- [ ] **Step 3: Apply fixes**

Use `superpowers:receiving-code-review`. Verify each finding before implementing it; a reviewer's claim can be wrong.

- [ ] **Step 4: Final gate**

```bash
cd /home/iofahd/code/agentos/console && npm run build && npx vitest run
/home/iofahd/code/agentos/connectors/browser/.venv/bin/python \
  /home/iofahd/code/agentos/console/scripts/render.py http://localhost:5199 /tmp/console-final
```

Expected: build clean, ≥309 tests pass (305 baseline + 3 from Task 0 + 4 from Task 2, minus any consolidation), harness exits 0.

- [ ] **Step 5: Finish the branch**

Use `superpowers:finishing-a-development-branch`.
