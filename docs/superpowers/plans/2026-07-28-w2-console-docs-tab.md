# W2 — Console Visual Docs Tab Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a top-level **Docs** tab to the AgentOS console — a visual, animated, in-app handbook covering every platform capability — and correct the live governance Chain instrument so the console stops depicting a pipeline the gateway does not run.

**Architecture:** A typed content model (`DOC_SECTIONS`) in a pure `.ts` module drives a two-pane page: a sticky section rail plus a rendered section. A pure block→markup mapper renders six block kinds; one of them resolves a `DiagramKey` against a compile-enforced registry of four inline-SVG visuals. All animation is CSS keyframes (the house technique) except the rail pill and section reveal, which use the existing framer-motion presets. Every pure decision — content, geometry, timelines, stage lists — lives in a `.ts` sibling so it is testable in the node-environment vitest suite.

**Tech Stack:** TypeScript, React 19, framer-motion 12.42.2 (already present — **the only three runtime deps are `framer-motion`, `react`, `react-dom`, and no dependency may be added**), plain global CSS, vitest 3 in `environment: "node"`.

## Global Constraints

From `docs/superpowers/specs/2026-07-27-platform-docs-and-console-docs-tab.md` §3, §4, §7, and from the verified console fact sheet at `docs/superpowers/research/2026-07-28-console-new-page-factsheet.md`. Every task's requirements implicitly include this section.

- **Private/local:** everything commits to the working branch and merges to `main` locally. **Never push to origin.**
- **Air-gap:** the console makes **no external requests** and gains **no new runtime dependencies**. No CDN, no web fonts, no remote images. Visuals are inline SVG authored by hand.
- **No new npm dependency of any kind** — no charting lib, no SVG lib, no markdown renderer, no syntax highlighter.
- **Chroma is reserved for machine state.** The only hues are `--live` (in flight), `--ok` (allowed), `--hold` (awaiting a human), `--deny` (denied). `--accent` resolves to ink (`var(--text)`), not a brand color. Doc notes/callouts are **monochrome** (`.notice`, no `.warn`/`.error` modifier unless the content is genuinely a warning). Using a hue decoratively corrupts the console's signal vocabulary.
- **There is no light theme.** One `:root` block, `<meta name="color-scheme" content="dark">`. **Never write `@media (prefers-color-scheme: light)`.** Never add a token.
- **Determinism:** no `Math.random()` anywhere. Animated sequences are fixed scripts held in pure `.ts` modules.
- **Reduced motion:** the global CSS cap (`styles.css:1108-1123`) zeroes durations but sets `animation-iteration-count: 1`, which turns an infinite loop into a single 0.01 ms cycle rather than a good-looking still — and it does **not** reset `animation-delay` or `--dur-slow`. Every visual must therefore ship its own `@media (prefers-reduced-motion: reduce)` block that produces a deliberate static still, and every JS timer must be gated on `useReducedMotion()` so it never starts.
- **Class and keyframe names are global** — no CSS modules. Prefix every class and every `@keyframes` in this work-stream with `docs-`. Existing keyframe names that must not be collided with: `live-pulse`, `chain-pulse`, `state-live-pulse`, `op-live-pulse`, `ov-pulse`, `pg-live-pulse`, `pg-live-pulse-amber`, `pg-shimmer`, `ui-shimmer`, `keys-meter-fill`, `audit-badge-flash`, `doc-drop-pulse`, `doc-progress-slide`, `chart-draw`, `chart-fade`, `chart-bar`.
- **`layoutId` is document-global** (no `LayoutGroup` exists in the app). Never use `layoutId="nav-pill"` or render `<Tabs id="nav" />` — either makes the sidebar pill fly into this page. Namespace every `layoutId` introduced here as `docs-*`.
- **Tests must be named `*.test.ts`.** A `*.test.tsx` is silently **not collected** (`include: ["src/**/*.test.ts"]`) and reports as passing by never running. **No JSX inside a test file** — the `.ts` esbuild loader rejects it. There is **no DOM**: `document`, `window`, `requestAnimationFrame` are `undefined`. Nothing may be rendered or mounted.
- **`npm run build` is `tsc && vite build`.** `noUnusedLocals` and `noUnusedParameters` are on and `tsconfig` includes test files, so an unused const in a test breaks the production build. `vite build` alone, `npm run dev`, and `vitest` are transpile-only and will not surface these. **Run `npx tsc` before claiming done.**
- **Never write a file extension in an import path.** No `.ts`/`.tsx` extensions anywhere; only `.css` side-effect imports carry one.
- **React types come from explicit type imports** — `import type { JSX, ReactNode } from "react";` — matching `ui/Card.tsx`, `ui/Table.tsx`, `ui/Modal.tsx`, `ui/Skeleton.tsx`. (`App.tsx` uses the bare `React.JSX.Element` namespace form and compiles, but do not mix the two conventions in new files.)
- **`import type { PageProps } from "../App";` must stay type-only** — a value import creates a real runtime cycle.
- **Never pass a query string or hash to the router's `navigate()`.** It stores the full string as the route key and matches by exact equality, so the page silently falls back to Overview while the URL bar disagrees.
- **`icons.tsx` is not in the `src/ui` barrel** — import `Icon` from `"../ui/icons"`, everything else from `"../ui"`.

---

## Reference: verified console facts

Checked against source at plan time by seven parallel readers and six adversarial verifiers. Full detail: `docs/superpowers/research/2026-07-28-console-new-page-factsheet.md`. **Do not re-derive these and do not contradict them.**

### R1. Page contract

Pages receive **five props**, not zero (`src/App.tsx:244-252`):

```ts
export interface PageProps {         // src/App.tsx:45-51
  adminKey: string;
  role: AuthRole;
  orgId: string;
  openSettings: () => void;
  navigate: (path: string) => void;
}
```

Every page is a **named** export, `export function <Name>(props: PageProps)`; there is no default export in `src/pages/`. Pages that use no props name the parameter `_props` — the underscore is the `noUnusedParameters` escape hatch and the existing convention (`Documents.tsx:23`, `Operators.tsx:49`, `Playground.tsx:75`, `Improve.tsx:66`).

A page returns a bare `<>…</>` fragment — **no wrapper div**; `App.tsx` supplies `<div className="page">`. Order is `PageHead` → notices → `Panel`s.

Registration is manual in exactly two places: the import at `src/App.tsx:32-43` and the `ROUTES` entry at `:62-99`. **The Sidebar and the ⌘K palette are derived from `ROUTES` automatically — do not edit `CommandPalette.tsx`.**

A route change fully unmounts the page (`key={route.path}` on `AnimatePresence mode="wait"`), so page state does not survive navigation.

### R2. Routing hazard

```ts
const route = ROUTES.find((r) => r.path === path) ?? ROUTES[0];   // src/App.tsx:150
const navigate = useCallback((p: string) => {                      // src/App.tsx:127-131
  window.history.pushState({}, "", p);
  setPath(p);
}, []);
```

`usePath` seeds from `window.location.pathname` and its `popstate` handler re-reads `window.location.pathname` — both strip query and hash. So **an inbound `/docs?s=gateway` URL renders Docs correctly**, but an in-app `navigate("/docs?s=gateway")` stores the full string, fails the exact match, and silently renders Overview. Update the query with `window.history.replaceState` directly instead. `/documents` is the existing RAG page and is unrelated to `/docs`.

### R3. UI primitives (exact)

- `PageHead` takes `{ title: string; subtitle?: string }` only. `title` is a `string`, not a ReactNode. **There is no `actions` slot.**
- `Panel` wraps children; `PanelHead` takes `{ title, actions? }` and renders `actions` raw as the second flex child.
- **There is no `PanelBody` component.** Write `<div className="panel-body">` by hand. A `Table` goes directly inside `Panel` with no `panel-body` wrapper.
- `.panel` has `overflow: hidden` (`styles.css:334-340`) — a bare `<pre>` inside it is **silently clipped, with no scrollbar**.
- The only container-safe block style is `.prompt-text` (`styles.css:774-783`): `background: var(--inset)`, `1px solid var(--border)`, `var(--radius-sm)`, `padding: 12px 14px`, `white-space: pre-wrap`, `word-break: break-word`, `max-height: 320px`, `overflow-y: auto`. Use `<pre className="prompt-text mono">`.
- `.notice` (`styles.css:602-618`) is monochrome by default: `1px solid var(--border-strong)`, `var(--radius-md)`, `padding: 12px 16px`, `margin-bottom: 20px`, `color: var(--text-dim)`. The `.warn` and `.error` modifiers add hue — use them only for genuine warnings.
- Import from `"../ui"`; import `Icon` from `"../ui/icons"`.

### R4. Tokens (dark only — the complete set you may use)

`--bg #121517`, `--raised #191d20`, `--raised-2 #20252a`, `--inset #0d0f11`, `--border rgba(255,255,255,0.07)`, `--border-strong rgba(255,255,255,0.13)`, `--text #e6e9eb`, `--text-dim #98a1a8`, `--text-faint #7e878e`, `--live #5ad1c4`, `--ok #6cc48f`, `--hold #e3a851`, `--deny #e2685f`, `--accent (= --text)`, `--accent-dim rgba(255,255,255,0.07)`, `--radius-sm 3px`, `--radius-md 4px`, `--radius-lg 6px`, `--dur-fast 110ms`, `--dur-med 180ms`, `--dur-slow 420ms`, `--ease cubic-bezier(0.2,0.8,0.2,1)`, `--mono`, `--sans`, `--eyebrow-size 10px`, `--eyebrow-track 0.16em`.

**Add no tokens.** `--dur-slow` is not zeroed by the reduced-motion cap.

### R5. Motion presets — `src/ui/motion.ts`, exact

```ts
export const DUR_FAST = 0.12;
export const DUR_MED = 0.2;
export const DUR_PAGE = 0.16;
export const EASE: [number, number, number, number] = [0.2, 0.8, 0.2, 1];
export const STAGGER = 0.032;
export const STAGGER_MAX_ITEMS = 6;
export const transition: Transition = { duration: DUR_MED, ease: EASE };
export const transitionFast: Transition = { duration: DUR_FAST, ease: EASE };
export const fadeRise: Variants;          // hidden {opacity 0, y 6} / show / exit
export const fadeRiseReduced: Variants;   // all states {opacity 1, y 0}
export const staggerContainer: Variants;
export const staggerItem: Variants;
export const staggerItemReduced: Variants;
```

Variant state names are exactly `"hidden"`, `"show"`, `"exit"`. The reduced-motion convention is a ternary on `variants` (`reduced ? fadeRiseReduced : fadeRise`) or bailing to a plain DOM element.

**framer-motion APIs in use here:** `motion.div`, `motion.span`, `AnimatePresence`, `layoutId`, `variants`, `useReducedMotion`. **NOT used anywhere in the app and not to be introduced:** `motion.path`, `motion.svg`, `motion.circle`, `whileHover`, `whileInView`, `useAnimate`, `useMotionValue`, `LayoutGroup`, `MotionConfig`.

### R6. The house technique for animated SVG

CSS keyframes + `pathLength={1}` + `stroke-dasharray`/`stroke-dashoffset`. **SMIL (`<animate>`, `<animateTransform>`) does not exist in this codebase — do not introduce it.** Real precedent, `src/charts/UsageChart.tsx:102` + `src/charts/charts.css:34-59`:

```tsx
<path className="chart-line chart-line--draw" pathLength={1} d={line} />
```

```css
.chart-line--draw {
  stroke-dasharray: 1;
  stroke-dashoffset: 1;
  animation: chart-draw var(--dur-med) var(--ease) forwards;
}
@keyframes chart-draw { to { stroke-dashoffset: 0; } }
```

Per-item stagger is done with an **inline** `animationDelay` (`src/charts/SpendBreakdown.tsx:50`: `` animationDelay: `${Math.min(i, 10) * 40}ms` ``).

### R7. The live Chain instrument — currently wrong, fixed by Task 1

`src/lib/chain.ts:24` today:

```ts
export const CHAIN_STAGES = ["auth", "rbac", "budget", "rate", "audit"] as const;
```

Against the gateway's real `/v1/chat/completions` path (`Auth → Rate limit → Budget hold → Guardrail (when AGENTOS_GUARDRAILS_MODE != off) → Upstream/Council → Audit`):

- **`rbac` is not on the model-call path.** Role checks gate `/admin/*` only.
- **`budget` and `rate` are transposed** — rate limiting runs first.
- **The guardrail stage is missing**, and it is the one denial reliably present in the audit feed.
- `chainStateFromStatus` maps `401 → auth`, `403 → rbac`, `402 → budget`, but the gateway **writes no audit entry** for any of those, and the Chain's only evidence source is `GET /admin/audit`. Those branches cannot fire from their own data source.
- `400 → budget` is wrong: a 400 in that feed is a **guardrail block** (`KindGuardrailBlock`).

The audit kinds that actually exist (`gateway/internal/store/store.go`): `chat`, `embeddings`, `guardrail_flag`, `guardrail_block`, `guardrail_error`, `rate_limited`, `secret_reload`.

The file's own design law (`chain.ts:18-20`, `:44-47`, `:88-90`; `Chain.tsx:56-59`): *"a chain that under-reports is worse than no chain, because it implies checks ran that did not"*; *"the console must never draw checks it cannot prove ran"*; *"stages after it stay unlit — the request never reached them, so showing them as anything but dark would be a lie."*

Existing helpers to preserve: `ChainOutcome = "pass" | "deny" | "fail"`, `ChainState { cleared, stoppedAt, outcome }`, `IDLE_CHAIN`, `StageRender = "cleared" | "stopped" | "unlit"`, `latestChainState(entries)`, `stageRenders(state)`.

Chain rendering facts: nodes are **7px squares, `border-radius: 1px`** (not circles); a clean pass leaves `.chain-fill` **graphite, not green** (`Chain.css:39-40`); `data-active` is present-or-absent, never `"false"`; `.chain-label` is hidden under `max-width: 720px`.

### R8. Icons

`GLYPHS` is `const GLYPHS: Record<IconName, JSX.Element>` at `src/ui/icons.tsx:53` — **module-private and exhaustive**, so a bogus icon name is already a `tsc` error and `tsc` gates the build. **Do not add an `ICON_NAMES` export** (the spec's §7.5 suggestion): a runtime test asserting "every section icon is a real glyph" would be strictly redundant with the type checker.

The wrapper supplies `viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.25} strokeLinecap="square" strokeLinejoin="miter"`. **A glyph must not set `stroke`, `fill`, `width`, or `viewBox`.**

House grid: artwork band 2…14 (centerlines 2.5…13.5), coordinates snap to whole or half units, primitives are only `<path>`, `<rect>`, `<circle>` — **no `<g>`, `<line>`, `<polyline>`, `<polygon>`, transforms, ids, or gradients**. Arrowheads are open brackets, never filled wedges. Every entry carries a `//` comment explaining the reason for the shape.

**There is no book/manual/docs glyph.** The nearest neighbour is `documents` (two offset sheets: `M5.5 2.5h5l3 3v6h-8z`, `M10.5 2.5v3h3`, `M2.5 5.5v8h8`), so a new glyph **must differentiate on silhouette**, not on presence of a corner fold. The house reference motif is `audit`'s spine.

Existing `IconName` members this plan uses: `overview`, `keys`, `audit`, `playground`, `documents`, `improve`, `multiverse`, `operators`, `orgs`, `users`, `secrets`, `provisioning`, `settings`, `refresh`, `copy`, `run`, `pause`, `export`, `sort`, `close`, `chevron`, `trash`, `plus`, `filter`, `approve`, `deny`, `search`, `external-link`.

### R9. Sidebar pill — the pattern to replicate, with a different id

`src/components/Sidebar.tsx:94-120`:

```tsx
{active &&
  (reduced ? (
    <span className="nav-pill" aria-hidden />
  ) : (
    <motion.span className="nav-pill" aria-hidden layoutId="nav-pill" transition={transition} />
  ))}
```

Pill CSS (`Sidebar.css:47-53`): `position:absolute; inset:0; border-radius: var(--radius-sm); background: var(--raised-2); box-shadow: inset 2px 0 0 0 var(--text-dim);` with the item forced transparent and label/icon at `position:relative; z-index:1`.

### R10. Shell behaviour on `/docs`

The topbar `Chain` (`/admin/audit?limit=100`, 5000 ms) and the Sidebar live-dot (`/admin/usage`, 5000 ms) poll on **every** route, including `/docs`, whenever `adminKey` is set. `useConnectionState()` re-renders every 1 s. **The Docs page itself must issue no fetches and read no `adminKey`** — but "no network traffic" is not an achievable acceptance criterion.

`installVisibilityPause` pauses live-resource polls only — **it does not pause hand-rolled animation timers**, so a looping visual keeps running in a background tab unless it handles `document.hidden` itself.

### R11. Known carry-forward defects (fixed by Task 2)

- `console/src/pages/Improve.tsx:170` tells the user to set `AGENTOS_CHECKPOINT_DB`. **No backend reads that name.** The real variable is `AGENTOS_CHECKPOINT_DATABASE_URL` (`runtime/src/agentos_runtime/config.py`). W1 could not fix it (docs-only work-stream) and documented it as a dead name in `docs/configuration.md`.
- `console/nginx.conf.template` carries a comment stating the runtime requires a bearer "on every route (except /healthz)". There is a **second** exemption: `POST /operators/webhooks/{token}` (`OPEN_PREFIXES = ("/operators/webhooks/",)`), where the `whk-` path segment is the credential.
- `README.md` deliberately does not link the in-console Docs tab, because it did not exist. Once this plan ships, it does.

---

## File Structure

**Created:**

| File | Responsibility |
|---|---|
| `console/src/pages/docs/types.ts` | `DocBlock`, `DocSection` types + the pure `blockToText` helper. No React. |
| `console/src/pages/docs/content.ts` | `DOC_SECTIONS` — the twelve authored sections. Data only. No React. |
| `console/src/pages/docs/content.test.ts` | Content-model invariants + `blockToText` coverage. |
| `console/src/pages/docs/visuals/keys.ts` | `DIAGRAM_KEYS`, `DiagramKey`, `DIAGRAM_META`. Pure. |
| `console/src/pages/docs/visuals/geometry.ts` | Node/edge coordinates and the governance-chain script. Pure, testable. |
| `console/src/pages/docs/visuals/geometry.test.ts` | Determinism and stage-list conformance. |
| `console/src/pages/docs/visuals/Illustration.tsx` | Shared frame + the mandatory illustration marker. |
| `console/src/pages/docs/visuals/ArchitectureVisual.tsx` | The four planes + looping request pulse. |
| `console/src/pages/docs/visuals/GovernanceChainVisual.tsx` | The animated stage conveyor over the real pipeline. |
| `console/src/pages/docs/visuals/RequestLifecycleVisual.tsx` | Static annotated request path. |
| `console/src/pages/docs/visuals/CouncilFanoutVisual.tsx` | Objective → members → judge → verdict + dissent. |
| `console/src/pages/docs/visuals/registry.tsx` | `Record<DiagramKey, () => JSX.Element>` — compile-enforced completeness. |
| `console/src/pages/docs/DocBlocks.tsx` | Pure block→markup mapper with an exhaustive `switch`. |
| `console/src/pages/Docs.tsx` | The page: section rail + rendered section. |
| `console/src/pages/Docs.css` | All `docs-` prefixed layout, visual, and reduced-motion rules. |

**Modified:**

| File | Change |
|---|---|
| `console/src/lib/chain.ts` | Correct `CHAIN_STAGES` and `chainStateFromStatus` (Task 1) |
| `console/src/lib/chain.test.ts` | Update for the corrected stages (Task 1) |
| `console/src/components/Chain.tsx` | `STAGE_LABELS`, `STAGE_TITLES` for six stages (Task 1) |
| `console/src/pages/Improve.tsx` | One env-var name (Task 2) |
| `console/nginx.conf.template` | One comment (Task 2) |
| `console/src/ui/icons.tsx` | Add `"docs"` to `IconName` + a `GLYPHS` entry (Task 11) |
| `console/src/App.tsx` | Import `Docs`, add the `ROUTES` entry (Task 11) |
| `README.md`, `docs/console.md` | Link and describe the Docs tab (Task 12) |

---

## Task 1: Correct the Chain instrument to the real pipeline

The console currently draws a governance chain the gateway does not run. Fix it first: the Docs visual in Task 7 imports `CHAIN_STAGES` from here, so this task defines the truth both surfaces share.

**Files:**
- Modify: `console/src/lib/chain.ts`
- Modify: `console/src/components/Chain.tsx`
- Test: `console/src/lib/chain.test.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: `CHAIN_STAGES: readonly ["auth","rate","budget","guardrail","upstream","audit"]`, `ChainStage`, and an updated `chainStateFromStatus(status: number): ChainState`. `ChainState`, `ChainOutcome`, `IDLE_CHAIN`, `StageRender`, `latestChainState`, `stageRenders` keep their existing signatures. Task 7 imports `CHAIN_STAGES` and `ChainStage`.

- [ ] **Step 1: Read the current module and its test**

Run:
```bash
cd console && sed -n '1,110p' src/lib/chain.ts && echo "--- test ---" && cat src/lib/chain.test.ts
```
Note every existing test case; you are changing the expectations of several.

- [ ] **Step 2: Write the failing tests**

Replace the status-mapping cases in `console/src/lib/chain.test.ts` with these, keeping the file's existing import style (`import { describe, expect, it } from "vitest";` — `globals` is not enabled):

```ts
describe("CHAIN_STAGES", () => {
  it("matches the gateway's /v1/* pipeline order", () => {
    expect(CHAIN_STAGES).toEqual(["auth", "rate", "budget", "guardrail", "upstream", "audit"]);
  });

  it("does not include rbac, which never runs on the proxy path", () => {
    expect(CHAIN_STAGES).not.toContain("rbac");
  });
});

describe("chainStateFromStatus", () => {
  it("clears every stage on success", () => {
    expect(chainStateFromStatus(200)).toEqual({
      cleared: CHAIN_STAGES.length,
      stoppedAt: null,
      outcome: "pass",
    });
  });

  it("stops at rate on 429, having cleared auth only", () => {
    expect(chainStateFromStatus(429)).toEqual({ cleared: 1, stoppedAt: "rate", outcome: "deny" });
  });

  it("stops at guardrail on 400, because a 400 in the audit feed is a guardrail block", () => {
    expect(chainStateFromStatus(400)).toEqual({
      cleared: 3,
      stoppedAt: "guardrail",
      outcome: "deny",
    });
  });

  it("stops at upstream on 5xx: governance cleared, the provider failed", () => {
    expect(chainStateFromStatus(502)).toEqual({
      cleared: 4,
      stoppedAt: "upstream",
      outcome: "fail",
    });
  });

  it("stops at budget on 402, having cleared auth and rate", () => {
    expect(chainStateFromStatus(402)).toEqual({ cleared: 2, stoppedAt: "budget", outcome: "deny" });
  });

  it("stops at auth on 401", () => {
    expect(chainStateFromStatus(401)).toEqual({ cleared: 0, stoppedAt: "auth", outcome: "deny" });
  });

  it("treats an unknown status as a denial at the first stage, never as a pass", () => {
    expect(chainStateFromStatus(418)).toEqual({ cleared: 0, stoppedAt: "auth", outcome: "deny" });
  });
});
```

Keep any existing `latestChainState` / `stageRenders` / `IDLE_CHAIN` tests unchanged — those behaviours are not changing. If an existing test asserts `403 → rbac`, delete it and note the deletion in your report.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `cd console && npx vitest run src/lib/chain.test.ts`
Expected: FAIL — `CHAIN_STAGES` still contains `rbac`, and the 400/429/502 expectations do not match.

- [ ] **Step 4: Correct `chain.ts`**

Replace the stage constant and the status mapper. Keep the file's existing header comment block, and **update it** so the "keep this in sync with the gateway" note names the real pipeline:

```ts
export const CHAIN_STAGES = ["auth", "rate", "budget", "guardrail", "upstream", "audit"] as const;
```

```ts
/**
 * Map a recorded HTTP status onto the stage the request reached.
 *
 * The evidence source is the audit log, and the gateway only writes audit rows
 * for seven kinds: chat, embeddings, guardrail_flag, guardrail_block,
 * guardrail_error, rate_limited, secret_reload. So in practice the statuses
 * that reach us are 2xx, 400 (a guardrail block), 429 (a rate-limit rejection),
 * and 5xx (an upstream failure after governance cleared).
 *
 * 401 and 402 are mapped for completeness — they are the honest stage for those
 * statuses — but the gateway does not currently audit them, so they should not
 * appear in this feed. Unknown statuses are treated as denials at the first
 * stage rather than as passes: the console must never draw checks it cannot
 * prove ran.
 */
export function chainStateFromStatus(status: number): ChainState {
  if (status >= 200 && status < 300) {
    return { cleared: CHAIN_STAGES.length, stoppedAt: null, outcome: "pass" };
  }
  if (status >= 500) {
    // Governance cleared; the upstream provider is what broke.
    return { cleared: 4, stoppedAt: "upstream", outcome: "fail" };
  }
  switch (status) {
    case 401:
      return { cleared: 0, stoppedAt: "auth", outcome: "deny" };
    case 429:
      return { cleared: 1, stoppedAt: "rate", outcome: "deny" };
    case 402:
      return { cleared: 2, stoppedAt: "budget", outcome: "deny" };
    case 400:
      return { cleared: 3, stoppedAt: "guardrail", outcome: "deny" };
    default:
      return { cleared: 0, stoppedAt: "auth", outcome: "deny" };
  }
}
```

Note the `5xx` branch now reports `stoppedAt: "upstream"` where it previously reported `null` — this is the point of adding the stage. Check whether `Chain.tsx`'s `outcomeGlyph` relies on `stoppedAt === null` for the `fail` case; it keys on `state.outcome`, so it does not, but confirm by reading it.

- [ ] **Step 5: Update `Chain.tsx` labels and titles**

`STAGE_LABELS` is an identity map and `STAGE_TITLES` explains each stage on hover. Both are typed `Record<ChainStage, string>`, so both fail `tsc` until updated. Replace them with:

```ts
const STAGE_LABELS: Record<ChainStage, string> = {
  auth: "auth",
  rate: "rate",
  budget: "budget",
  guardrail: "guardrail",
  upstream: "upstream",
  audit: "audit",
};

const STAGE_TITLES: Record<ChainStage, string> = {
  auth: "auth — the caller presented a valid virtual key",
  rate: "rate — the key is within its rate limit",
  budget: "budget — the key and its org are within budget",
  guardrail: "guardrail — the prompt cleared injection screening (when enabled)",
  upstream: "upstream — the provider or council answered",
  audit: "audit — the outcome was recorded",
};
```

Change nothing else in `Chain.tsx`. Do not touch `Chain.css` — six nodes lay out on the same flex rule as five.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `cd console && npx vitest run src/lib/chain.test.ts`
Expected: PASS.

- [ ] **Step 7: Type-check**

Run: `cd console && npx tsc`
Expected: no output. If `Chain.tsx` still references `rbac` anywhere, `tsc` reports it — fix and re-run.

- [ ] **Step 8: Run the whole suite**

Run: `cd console && npx vitest run`
Expected: all files pass (22 before this change).

- [ ] **Step 9: Commit**

```bash
git add console/src/lib/chain.ts console/src/lib/chain.test.ts console/src/components/Chain.tsx
git commit -m "fix(console): correct the governance chain to the gateway's real pipeline"
```

---

## Task 2: Carry-forward console corrections

Two small factual defects W1 could not fix because it was a docs-only work-stream. Independent of everything else; done here so the branch leaves no known-wrong text behind.

**Files:**
- Modify: `console/src/pages/Improve.tsx` (the env var in user-facing copy, around line 170)
- Modify: `console/nginx.conf.template` (one comment)

**Interfaces:**
- Consumes: reference fact **R11**.
- Produces: nothing other tasks depend on.

- [ ] **Step 1: Confirm the dead variable**

Run:
```bash
cd /home/iofahd/code/agentos
grep -rn 'AGENTOS_CHECKPOINT_DB\b' --include='*.go' --include='*.py' --include='*.rs' --include='*.ts' --include='*.tsx' --include='*.yaml' . | grep -v node_modules
grep -rn 'AGENTOS_CHECKPOINT_DATABASE_URL' runtime/src | head -3
```
Expected: the only `AGENTOS_CHECKPOINT_DB` hit is the console copy; the real name is read by the runtime config. If source disagrees, stop and report.

- [ ] **Step 2: Fix the Improve empty-state copy**

In `console/src/pages/Improve.tsx`, the disabled-state `EmptyState` description reads `set <span className="mono">AGENTOS_CHECKPOINT_DB</span>`. Change the span's text to `AGENTOS_CHECKPOINT_DATABASE_URL`. Change nothing else — not the sentence structure, not the `EmptyState` title.

- [ ] **Step 3: Fix the nginx comment**

In `console/nginx.conf.template`, the comment stating the runtime requires a bearer on every route except `/healthz` is incomplete. Replace that comment's text with:

```
# The runtime requires a bearer token on every route except /healthz and
# POST /operators/webhooks/{token}, where the opaque whk- token in the path is
# itself the credential. This proxy injects the bearer server-side, so it never
# reaches the browser — and so anything that can reach this port has full
# runtime authority.
```

**Change only the comment.** Do not alter any `proxy_set_header`, `location`, or directive — this file is rendered at container start and a syntax error breaks the console entirely.

- [ ] **Step 4: Verify nothing functional changed**

Run:
```bash
cd /home/iofahd/code/agentos
git diff -- console/nginx.conf.template | grep -E '^[+-]' | grep -vE '^(\+\+\+|---)' | grep -vE '^[+-]\s*#'
```
Expected: no output — every changed line is a comment.

- [ ] **Step 5: Type-check and test**

Run: `cd console && npx tsc && npx vitest run`
Expected: clean; all tests pass.

- [ ] **Step 6: Commit**

```bash
git add console/src/pages/Improve.tsx console/nginx.conf.template
git commit -m "fix(console): correct the checkpoint env var and the runtime-auth comment"
```

---

## Task 3: Content model, diagram keys, and the pure text helper

Everything testable about the Docs tab lives here, in modules with no React import, so the node-environment suite can exercise it.

**Files:**
- Create: `console/src/pages/docs/types.ts`
- Create: `console/src/pages/docs/visuals/keys.ts`
- Test: `console/src/pages/docs/content.test.ts` (created here with a seed, extended in Task 4)
- Create: `console/src/pages/docs/content.ts` (seeded here with two sections, filled in Task 4)

**Interfaces:**
- Consumes: `IconName` from `../../ui/icons` (type-only).
- Produces:
  - `type DocBlock` — the six-member union below.
  - `interface DocSection { id: string; title: string; icon: IconName; blurb: string; blocks: DocBlock[] }`
  - `function blockToText(block: DocBlock): string`
  - `const DIAGRAM_KEYS = ["architecture","governanceChain","requestLifecycle","councilFanout"] as const`
  - `type DiagramKey = (typeof DIAGRAM_KEYS)[number]`
  - `const DIAGRAM_META: Record<DiagramKey, { title: string; marker: string; usesStateHues: boolean }>`
  - `const DOC_SECTIONS: DocSection[]`

- [ ] **Step 1: Write `console/src/pages/docs/visuals/keys.ts`**

Pure module — no React, no JSX, so a test may import it freely.

```ts
/**
 * Diagram identity, kept in a pure module so the content model and the tests can
 * reference a visual without pulling React in. The registry in `registry.tsx` is
 * typed `Record<DiagramKey, …>`, so adding a key here without a component is a
 * compile error — that is the completeness guarantee.
 */
export const DIAGRAM_KEYS = [
  "architecture",
  "governanceChain",
  "requestLifecycle",
  "councilFanout",
] as const;

export type DiagramKey = (typeof DIAGRAM_KEYS)[number];

/**
 * Every visual on this page is an illustration, not an instrument. The console's
 * live chain is evidence-driven by contract; these are scripted. The marker is
 * rendered on the visual itself so the distinction survives a screenshot.
 */
export const DIAGRAM_META: Record<
  DiagramKey,
  { title: string; marker: string; usesStateHues: boolean }
> = {
  architecture: {
    title: "The four planes",
    marker: "illustration · not live data",
    usesStateHues: true,
  },
  governanceChain: {
    title: "What a model call clears",
    marker: "illustration · scripted sequence",
    usesStateHues: true,
  },
  requestLifecycle: {
    title: "One governed request, end to end",
    marker: "illustration · not live data",
    usesStateHues: false,
  },
  councilFanout: {
    title: "Many models, one verdict",
    marker: "illustration · not live data",
    usesStateHues: false,
  },
};
```

- [ ] **Step 2: Write `console/src/pages/docs/types.ts`**

```ts
import type { IconName } from "../../ui/icons";
import type { DiagramKey } from "./visuals/keys";

/** One renderable unit of documentation. */
export type DocBlock =
  | { kind: "prose"; text: string }
  | { kind: "list"; items: string[] }
  | { kind: "code"; code: string; lang?: string }
  | { kind: "keyvals"; caption?: string; rows: { k: string; v: string }[] }
  | { kind: "note"; text: string }
  | { kind: "diagram"; diagram: DiagramKey; caption?: string };

export interface DocSection {
  /** Stable slug — appears in the `?s=` deep link, so never rename one casually. */
  id: string;
  title: string;
  icon: IconName;
  /** One line under the section title. Also the rail's accessible description. */
  blurb: string;
  blocks: DocBlock[];
}

/**
 * Flatten a block to plain text. Drives the content invariants below and gives a
 * future in-page search something to index without touching the DOM.
 */
export function blockToText(block: DocBlock): string {
  switch (block.kind) {
    case "prose":
      return block.text;
    case "list":
      return block.items.join(" ");
    case "code":
      return block.code;
    case "keyvals":
      return [block.caption ?? "", ...block.rows.map((r) => `${r.k} ${r.v}`)].join(" ").trim();
    case "note":
      return block.text;
    case "diagram":
      return block.caption ?? "";
    default: {
      const exhaustive: never = block;
      return exhaustive;
    }
  }
}
```

`lang` is **metadata only** — a mono label on the block. There is no syntax highlighting and no dependency may be added for it.

- [ ] **Step 3: Seed `console/src/pages/docs/content.ts`**

Two sections only; Task 4 fills the rest. This exists so the tests in Step 4 have something real to run against.

```ts
import type { DocSection } from "./types";

export const DOC_SECTIONS: DocSection[] = [
  {
    id: "overview",
    title: "Overview",
    icon: "overview",
    blurb: "What AgentOS is, and the one guarantee everything else serves.",
    blocks: [
      {
        kind: "prose",
        text: "AgentOS is a self-hostable agentic operating layer: any LLM provider in, any legacy system out, with governed autonomous agents in between. Every model call flows through one gateway that holds the credentials, the budget, and the audit log — the agent runtime never holds a provider key.",
      },
      { kind: "diagram", diagram: "requestLifecycle" },
    ],
  },
  {
    id: "architecture",
    title: "Architecture",
    icon: "orgs",
    blurb: "Four planes, and the wiring that makes the guarantee hold.",
    blocks: [{ kind: "diagram", diagram: "architecture" }],
  },
];
```

- [ ] **Step 4: Write the failing invariant tests**

`console/src/pages/docs/content.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { DOC_SECTIONS } from "./content";
import { blockToText } from "./types";
import type { DocBlock } from "./types";
import { DIAGRAM_KEYS, DIAGRAM_META } from "./visuals/keys";

describe("DOC_SECTIONS", () => {
  it("has unique ids", () => {
    const ids = DOC_SECTIONS.map((s) => s.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it("uses url-safe slugs, because ids are deep-link query values", () => {
    for (const s of DOC_SECTIONS) expect(s.id).toMatch(/^[a-z0-9-]+$/);
  });

  it("gives every section a title, a blurb, and at least one block", () => {
    for (const s of DOC_SECTIONS) {
      expect(s.title.length).toBeGreaterThan(0);
      expect(s.blurb.length).toBeGreaterThan(0);
      expect(s.blocks.length).toBeGreaterThan(0);
    }
  });

  it("references only real diagram keys", () => {
    for (const s of DOC_SECTIONS) {
      for (const b of s.blocks) {
        if (b.kind === "diagram") expect(DIAGRAM_KEYS).toContain(b.diagram);
      }
    }
  });

  it("has no empty block anywhere", () => {
    for (const s of DOC_SECTIONS) {
      for (const b of s.blocks) {
        if (b.kind === "diagram") continue;
        expect(blockToText(b).trim().length).toBeGreaterThan(0);
      }
    }
  });
});

describe("DIAGRAM_META", () => {
  it("covers every diagram key", () => {
    expect(Object.keys(DIAGRAM_META).sort()).toEqual([...DIAGRAM_KEYS].sort());
  });

  it("gives every visual a non-empty illustration marker", () => {
    for (const key of DIAGRAM_KEYS) {
      expect(DIAGRAM_META[key].marker.trim().length).toBeGreaterThan(0);
    }
  });
});

describe("blockToText", () => {
  const cases: DocBlock[] = [
    { kind: "prose", text: "a" },
    { kind: "list", items: ["a", "b"] },
    { kind: "code", code: "make up", lang: "bash" },
    { kind: "keyvals", caption: "cap", rows: [{ k: "k", v: "v" }] },
    { kind: "note", text: "n" },
    { kind: "diagram", diagram: "architecture", caption: "c" },
  ];

  it("covers every block kind", () => {
    const kinds = new Set(cases.map((c) => c.kind));
    expect(kinds.size).toBe(6);
  });

  it("returns text for each kind", () => {
    expect(cases.map(blockToText)).toEqual(["a", "a b", "make up", "cap k v", "n", "c"]);
  });

  it("returns empty string for a diagram with no caption", () => {
    expect(blockToText({ kind: "diagram", diagram: "architecture" })).toBe("");
  });
});
```

- [ ] **Step 5: Run to verify they pass**

Run: `cd console && npx vitest run src/pages/docs/content.test.ts`
Expected: PASS. (These are invariants over data that already satisfies them — the value is that they fail the moment Task 4's authoring breaks one.)

- [ ] **Step 6: Type-check**

Run: `cd console && npx tsc`
Expected: no output. A common failure here is an unused import in the test — `noUnusedLocals` is on and test files are type-checked.

- [ ] **Step 7: Commit**

```bash
git add console/src/pages/docs/types.ts console/src/pages/docs/content.ts \
        console/src/pages/docs/content.test.ts console/src/pages/docs/visuals/keys.ts
git commit -m "feat(console): add the docs content model, diagram keys, and invariants"
```

---

## Task 4: Author the twelve documentation sections

The content itself. It must agree with the GitHub docs W1 shipped — those are the canonical source, and a contradiction between the two surfaces is the failure mode this whole effort exists to prevent.

**Files:**
- Modify: `console/src/pages/docs/content.ts` (replace the two-section seed with twelve)
- Modify: `console/src/pages/docs/content.test.ts` (add the taxonomy and placement tests)

**Interfaces:**
- Consumes: `DocSection`, `DocBlock` from `./types`; `DiagramKey` from `./visuals/keys`.
- Produces: the finished `DOC_SECTIONS`. Task 9's renderer and Task 10's page consume it.

**Source of truth for the prose:** read these before authoring, and do not contradict them — `README.md`, `docs/concepts.md`, `docs/architecture.md`, `docs/api.md`, `docs/console.md`, `docs/configuration.md`, `docs/deployment.md`, `docs/operations.md`, `SECURITY.md`.

- [ ] **Step 1: Write the twelve sections in this exact order**

The order and titles are the canonical taxonomy and are asserted by a test in Step 2.

| # | `id` | `title` | `icon` | Must contain |
|---|---|---|---|---|
| 1 | `overview` | `Overview` | `overview` | the pitch; the credential invariant; `diagram: requestLifecycle` |
| 2 | `concepts` | `Concepts & Glossary` | `docs` | keyvals of the vocabulary |
| 3 | `architecture` | `Architecture` | `orgs` | `diagram: architecture`; the four planes; ports |
| 4 | `gateway` | `Gateway` | `keys` | `diagram: governanceChain`; the pipeline; the fail-open matrix |
| 5 | `runtime` | `Runtime` | `playground` | `diagram: councilFanout`; profiles, skills, operators, council |
| 6 | `sandbox` | `Sandbox` | `secrets` | isolation layers; `run_python`; no egress |
| 7 | `connectors` | `Connectors` | `external-link` | the five connectors; the tool catalog; the wiring gotcha |
| 8 | `console` | `Console` | `provisioning` | the twelve pages; role gating; the token boundary |
| 9 | `quickstart` | `Quickstart` | `run` | Compose path and the local-Ollama $0 path |
| 10 | `configuration` | `Configuration` | `settings` | the variable groups; where the full reference lives |
| 11 | `deploy` | `Deploy` | `export` | Compose → Helm → CI; the connector deployment tiers |
| 12 | `security` | `Security` | `audit` | trust boundaries; fail-open vs fail-closed; residual risk |

`icon` values must be existing `IconName` members except `docs`, which Task 11 adds. Authoring this before Task 11 means `tsc` will error on `"docs"` until Task 11 lands — that is expected; run `npx vitest run` in this task and defer the clean `npx tsc` to Task 11. Note it in your report.

**Facts that must appear and must be exactly right** (each was corrected during W1 after being found wrong; getting one wrong here re-introduces the defect into the product UI):

- The `/v1/chat/completions` pipeline is `Auth (agos- virtual key) → Rate limit → Budget hold → Guardrail (only when AGENTOS_GUARDRAILS_MODE != off) → Upstream provider or council → Audit`. `/v1/embeddings` runs the same chain minus the guardrail.
- The env var is **`AGENTOS_GUARDRAILS_MODE`** — plural `GUARDRAILS`. It reads like a typo and is not.
- **RBAC is never evaluated on `/v1/*`.** Role checks gate `/admin/*` only, via `agu-` user tokens and the root admin key. `/scim/v2/*` is gated by a static shared-secret bearer with no role evaluation. `/auth/oidc/*` has no auth wrapper — it is the public login/callback flow.
- Audit records outcomes as one of **seven kinds**: `chat`, `embeddings`, `guardrail_flag`, `guardrail_block`, `guardrail_error`, `rate_limited`, `secret_reload`. Among denials **only rate-limit rejections and guardrail events are audited** — a 401 auth failure, a 400 malformed request, and a 402 budget exhaustion are not.
- Budget: exhaustion is **enforced** with HTTP 402. **Only a store/DB error fails open**, and — unlike the guardrail's `guardrail_error` — that admission gets **no** audit entry. Never write "budget enforcement fails open".
- Skills are in-repo/image-baked and never fetched at runtime; each load records a sha256 **for provenance**. **Never write "sha256-pinned".**
- Council write-gating is fail-closed **for `react`-profile members**; `deep`-profile members are constrained by an explicit read-only `tools:` allowlist because deepagents exposes no `interrupt_before`.
- Connector tiers: Compose wires **SQL + REST** by default via a hardcoded `AGENTOS_MCP_SERVERS` literal with no `${}` override; `--profile connectors` starts SOAP and browser but does **not** make their tools reachable; **SSH has no Compose service and no Helm template**. Helm's `restConnector.enabled` defaults to `false`.
- The console proxy carries **full runtime authority**: nginx injects the runtime bearer unconditionally, and the runtime has no role concept — so reaching the console's port is equivalent to holding the runtime token. Page visibility is a UI affordance; the gateway enforces `/admin/*` permissions server-side regardless.
- The local-Ollama $0 path requires explicit `AGENTOS_MODEL`, `AGENTOS_EMBED_MODEL`, `AGENTOS_JUDGE_MODEL`, and `AGENTOS_OLLAMA_BASE_URL` overrides — the shipped defaults point at Anthropic.

**Block-kind guidance:** use `keyvals` for anything machine-shaped (ports, endpoints, env vars, tool names) — it renders mono. Use `code` for commands. Use `note` sparingly and keep it monochrome. Every section should open with one `prose` block.

Keep each section readable in one screen: aim for 4–8 blocks. This is orientation, not a replacement for `docs/`; where depth belongs elsewhere, say so in prose (there are no outbound links — the console is air-gapped and links to GitHub would 404 on a private repo).

- [ ] **Step 2: Add the taxonomy and placement tests**

Append to `console/src/pages/docs/content.test.ts`:

```ts
describe("taxonomy", () => {
  it("matches the canonical section order shared with the GitHub docs", () => {
    expect(DOC_SECTIONS.map((s) => s.id)).toEqual([
      "overview",
      "concepts",
      "architecture",
      "gateway",
      "runtime",
      "sandbox",
      "connectors",
      "console",
      "quickstart",
      "configuration",
      "deploy",
      "security",
    ]);
  });

  it("titles match the canonical taxonomy verbatim", () => {
    expect(DOC_SECTIONS.map((s) => s.title)).toEqual([
      "Overview",
      "Concepts & Glossary",
      "Architecture",
      "Gateway",
      "Runtime",
      "Sandbox",
      "Connectors",
      "Console",
      "Quickstart",
      "Configuration",
      "Deploy",
      "Security",
    ]);
  });
});

describe("visual placement", () => {
  const diagramsIn = (id: string) =>
    (DOC_SECTIONS.find((s) => s.id === id)?.blocks ?? [])
      .filter((b) => b.kind === "diagram")
      .map((b) => (b.kind === "diagram" ? b.diagram : ""));

  it("binds each visual to its section", () => {
    expect(diagramsIn("overview")).toContain("requestLifecycle");
    expect(diagramsIn("architecture")).toContain("architecture");
    expect(diagramsIn("gateway")).toContain("governanceChain");
    expect(diagramsIn("runtime")).toContain("councilFanout");
  });

  it("uses every declared diagram at least once", () => {
    const used = new Set(
      DOC_SECTIONS.flatMap((s) => s.blocks)
        .filter((b) => b.kind === "diagram")
        .map((b) => (b.kind === "diagram" ? b.diagram : "")),
    );
    for (const key of DIAGRAM_KEYS) expect(used).toContain(key);
  });
});

describe("content accuracy guards", () => {
  const all = DOC_SECTIONS.flatMap((s) => s.blocks).map(blockToText).join(" ");

  it("never claims skills are sha256-pinned", () => {
    expect(all).not.toMatch(/sha256-pinned/i);
  });

  it("never claims every outcome or every denial is audited", () => {
    expect(all).not.toMatch(/every outcome/i);
    expect(all).not.toMatch(/including denials/i);
  });

  it("spells the guardrail variable with the plural GUARDRAILS", () => {
    expect(all).not.toMatch(/AGENTOS_GUARDRAIL_MODE/);
  });

  it("never groups scim or oidc routes as role-checked", () => {
    expect(all).not.toMatch(/RBAC[^.]*\/scim/i);
  });
});
```

- [ ] **Step 3: Run the tests**

Run: `cd console && npx vitest run src/pages/docs/content.test.ts`
Expected: PASS. If an accuracy guard fires, fix the prose — do not weaken the guard.

- [ ] **Step 4: Commit**

```bash
git add console/src/pages/docs/content.ts console/src/pages/docs/content.test.ts
git commit -m "feat(console): author the twelve docs sections"
```

---

## Task 5: Visual geometry and the governance script

All coordinates and every animation timeline live in one pure module so they can be tested without a DOM.

**Files:**
- Create: `console/src/pages/docs/visuals/geometry.ts`
- Test: `console/src/pages/docs/visuals/geometry.test.ts`

**Interfaces:**
- Consumes: `CHAIN_STAGES`, `ChainStage` from `../../../lib/chain` (Task 1).
- Produces: `ARCH_NODES`, `ARCH_EDGES`, `ARCH_PULSE_PATH`, `LIFECYCLE_HOPS`, `COUNCIL_MEMBERS`, `GOVERNANCE_SCRIPT`, `GOVERNANCE_FRAME_MS`, and `type GovernanceFrame`. Tasks 6–8 import these.

- [ ] **Step 1: Write the module**

```ts
import { CHAIN_STAGES } from "../../../lib/chain";
import type { ChainStage } from "../../../lib/chain";

/* ---------- architecture ---------- */

export interface ArchNode {
  id: string;
  label: string;
  sub: string;
  x: number;
  y: number;
  w: number;
  h: number;
}

/** viewBox is 0 0 640 320 for the architecture visual. */
export const ARCH_NODES: ArchNode[] = [
  { id: "console", label: "console", sub: ":3000", x: 16, y: 24, w: 104, h: 44 },
  { id: "client", label: "any OpenAI client", sub: "", x: 16, y: 132, w: 104, h: 44 },
  { id: "gateway", label: "gateway", sub: "Go · :8080", x: 184, y: 78, w: 128, h: 60 },
  { id: "providers", label: "providers", sub: "Anthropic · OpenAI · Ollama", x: 184, y: 232, w: 128, h: 52 },
  { id: "runtime", label: "runtime", sub: "Python · :8000", x: 376, y: 78, w: 128, h: 60 },
  { id: "sandbox", label: "sandbox", sub: "Rust · :8070 · no egress", x: 376, y: 232, w: 128, h: 52 },
  { id: "connectors", label: "connectors", sub: "MCP · :8090-8094", x: 540, y: 78, w: 84, h: 60 },
];

export interface ArchEdge {
  from: string;
  to: string;
  d: string;
  note?: string;
}

export const ARCH_EDGES: ArchEdge[] = [
  { from: "console", to: "gateway", d: "M120 46H184" },
  { from: "client", to: "gateway", d: "M120 154H152V108H184" },
  { from: "gateway", to: "providers", d: "M248 138V232", note: "the only egress to a model" },
  { from: "runtime", to: "gateway", d: "M376 108H312", note: "models only via the gateway" },
  { from: "runtime", to: "sandbox", d: "M440 138V232" },
  { from: "runtime", to: "connectors", d: "M504 108H540" },
];

/** One continuous path a request pulse traces: client → gateway → runtime → connectors → back. */
export const ARCH_PULSE_PATH = "M120 154H152V108H184H312H376H504H540";

/* ---------- request lifecycle ---------- */

export interface LifecycleHop {
  id: string;
  label: string;
  detail: string;
  x: number;
}

/** viewBox is 0 0 640 190. Hops sit on a single rule at y=70. */
export const LIFECYCLE_HOPS: LifecycleHop[] = [
  { id: "client", label: "client", detail: "presents an agos- virtual key", x: 56 },
  { id: "gateway", label: "gateway", detail: "authorises, meters, screens, records", x: 216 },
  { id: "runtime", label: "runtime", detail: "runs the agent; holds no provider key", x: 392 },
  { id: "tool", label: "tool", detail: "connector or sandbox, constrained in-process", x: 568 },
];

/* ---------- council fanout ---------- */

export interface CouncilMember {
  id: string;
  label: string;
  y: number;
}

/** viewBox is 0 0 640 260. Members fan out from x=150 to a judge at x=430. */
export const COUNCIL_MEMBERS: CouncilMember[] = [
  { id: "m1", label: "model A", y: 40 },
  { id: "m2", label: "model B", y: 88 },
  { id: "m3", label: "model C", y: 136 },
  { id: "m4", label: "model D", y: 184 },
  { id: "m5", label: "model E", y: 232 },
];

/* ---------- governance conveyor ---------- */

/**
 * A fixed script. No Math.random anywhere: the sequence is authored so a reader
 * sees a clean pass, a rate-limit denial, a guardrail block, and an upstream
 * failure without waiting on chance, and so two people looking at the same frame
 * see the same thing.
 *
 * `stoppedAt: null` means the request cleared every stage.
 */
export interface GovernanceFrame {
  stoppedAt: ChainStage | null;
  outcome: "pass" | "deny" | "fail";
  caption: string;
}

export const GOVERNANCE_SCRIPT: GovernanceFrame[] = [
  { stoppedAt: null, outcome: "pass", caption: "cleared every stage" },
  { stoppedAt: null, outcome: "pass", caption: "cleared every stage" },
  { stoppedAt: "rate", outcome: "deny", caption: "429 — over the org's rate limit" },
  { stoppedAt: null, outcome: "pass", caption: "cleared every stage" },
  { stoppedAt: "guardrail", outcome: "deny", caption: "400 — prompt flagged by the guardrail" },
  { stoppedAt: null, outcome: "pass", caption: "cleared every stage" },
  { stoppedAt: "upstream", outcome: "fail", caption: "502 — governance cleared, provider failed" },
  { stoppedAt: "budget", outcome: "deny", caption: "402 — key budget exhausted" },
];

/** Milliseconds a single frame is held. */
export const GOVERNANCE_FRAME_MS = 2200;

/** Stages cleared before the halting stage, for a given frame. */
export function clearedCount(frame: GovernanceFrame): number {
  if (frame.stoppedAt === null) return CHAIN_STAGES.length;
  return CHAIN_STAGES.indexOf(frame.stoppedAt);
}
```

- [ ] **Step 2: Write the tests**

`console/src/pages/docs/visuals/geometry.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { CHAIN_STAGES } from "../../../lib/chain";
import {
  ARCH_EDGES,
  ARCH_NODES,
  COUNCIL_MEMBERS,
  GOVERNANCE_FRAME_MS,
  GOVERNANCE_SCRIPT,
  LIFECYCLE_HOPS,
  clearedCount,
} from "./geometry";

describe("architecture geometry", () => {
  it("has unique node ids", () => {
    const ids = ARCH_NODES.map((n) => n.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it("only draws edges between declared nodes", () => {
    const ids = new Set(ARCH_NODES.map((n) => n.id));
    for (const e of ARCH_EDGES) {
      expect(ids.has(e.from)).toBe(true);
      expect(ids.has(e.to)).toBe(true);
    }
  });

  it("keeps every node inside the 640x320 viewBox", () => {
    for (const n of ARCH_NODES) {
      expect(n.x).toBeGreaterThanOrEqual(0);
      expect(n.y).toBeGreaterThanOrEqual(0);
      expect(n.x + n.w).toBeLessThanOrEqual(640);
      expect(n.y + n.h).toBeLessThanOrEqual(320);
    }
  });

  it("states the invariant on the runtime-to-gateway edge", () => {
    const edge = ARCH_EDGES.find((e) => e.from === "runtime" && e.to === "gateway");
    expect(edge?.note).toMatch(/only via the gateway/i);
  });
});

describe("governance script", () => {
  it("only halts at stages the chain actually has", () => {
    for (const f of GOVERNANCE_SCRIPT) {
      if (f.stoppedAt !== null) expect(CHAIN_STAGES).toContain(f.stoppedAt);
    }
  });

  it("never halts at rbac, which is not on the model-call path", () => {
    for (const f of GOVERNANCE_SCRIPT) expect(f.stoppedAt).not.toBe("rbac");
  });

  it("shows a clean pass, a denial, and an upstream failure", () => {
    const outcomes = new Set(GOVERNANCE_SCRIPT.map((f) => f.outcome));
    expect(outcomes).toContain("pass");
    expect(outcomes).toContain("deny");
    expect(outcomes).toContain("fail");
  });

  it("gives every frame a caption", () => {
    for (const f of GOVERNANCE_SCRIPT) expect(f.caption.trim().length).toBeGreaterThan(0);
  });

  it("holds a frame long enough to read", () => {
    expect(GOVERNANCE_FRAME_MS).toBeGreaterThanOrEqual(1500);
  });

  it("clears every stage on a pass and stops short on a denial", () => {
    expect(clearedCount({ stoppedAt: null, outcome: "pass", caption: "" })).toBe(
      CHAIN_STAGES.length,
    );
    expect(clearedCount({ stoppedAt: "rate", outcome: "deny", caption: "" })).toBe(
      CHAIN_STAGES.indexOf("rate"),
    );
  });
});

describe("other visuals", () => {
  it("orders the lifecycle hops left to right", () => {
    const xs = LIFECYCLE_HOPS.map((h) => h.x);
    expect([...xs].sort((a, b) => a - b)).toEqual(xs);
  });

  it("gives the council more than one member, so 'dissent' means something", () => {
    expect(COUNCIL_MEMBERS.length).toBeGreaterThan(1);
  });
});
```

- [ ] **Step 3: Run to verify they pass**

Run: `cd console && npx vitest run src/pages/docs/visuals/geometry.test.ts`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add console/src/pages/docs/visuals/geometry.ts console/src/pages/docs/visuals/geometry.test.ts
git commit -m "feat(console): add docs visual geometry and the governance script"
```

---

## Task 6: The illustration frame, the registry, and the architecture visual

**Files:**
- Create: `console/src/pages/docs/visuals/Illustration.tsx`
- Create: `console/src/pages/docs/visuals/ArchitectureVisual.tsx`
- Create: `console/src/pages/docs/visuals/registry.tsx`
- Create: `console/src/pages/Docs.css` (the visual rules; layout rules are added in Task 10)

**Interfaces:**
- Consumes: `DIAGRAM_META`, `DiagramKey` from `./keys`; `ARCH_NODES`, `ARCH_EDGES`, `ARCH_PULSE_PATH` from `./geometry`.
- Produces:
  - `Illustration({ diagram, children }: { diagram: DiagramKey; children: ReactNode })`
  - `ArchitectureVisual(): JSX.Element`
  - `DIAGRAM_REGISTRY: Record<DiagramKey, () => JSX.Element>` — Tasks 7–9 add entries and consume it.

- [ ] **Step 1: Write `Illustration.tsx`**

Every visual is wrapped in this. It carries the marker that keeps a scripted picture from reading as the live instrument.

```tsx
import type { ReactNode } from "react";
import { DIAGRAM_META } from "./keys";
import type { DiagramKey } from "./keys";

/**
 * Shared frame for every docs visual.
 *
 * The console's live instruments are evidence-driven — the chain in the topbar
 * is lit by recorded audit statuses, never by a timer. These are the opposite:
 * scripted pictures. The marker says so on the visual itself rather than in
 * surrounding prose, so the distinction survives a screenshot.
 */
export function Illustration({
  diagram,
  children,
}: {
  diagram: DiagramKey;
  children: ReactNode;
}) {
  const meta = DIAGRAM_META[diagram];
  return (
    <figure className="docs-fig">
      <figcaption className="docs-fig-head">
        <span className="docs-fig-title">{meta.title}</span>
        <span className="docs-fig-marker eyebrow">{meta.marker}</span>
      </figcaption>
      <div className="docs-fig-body">{children}</div>
    </figure>
  );
}
```

- [ ] **Step 2: Write `ArchitectureVisual.tsx`**

Reveal is CSS keyframes with an inline per-node `animationDelay` (the `SpendBreakdown` precedent). The request pulse is a travelling dash on a `pathLength={1}` path — the `UsageChart` technique. **No framer-motion, no SMIL, no `motion.path`.**

```tsx
import { ARCH_EDGES, ARCH_NODES, ARCH_PULSE_PATH } from "./geometry";
import { Illustration } from "./Illustration";

export function ArchitectureVisual() {
  return (
    <Illustration diagram="architecture">
      <svg
        className="docs-svg"
        viewBox="0 0 640 320"
        role="img"
        aria-label="The four planes: a console and any OpenAI client reach the gateway, which is the only egress to a model provider; the runtime reaches models only through the gateway and calls connectors and the sandbox."
      >
        {ARCH_EDGES.map((e) => (
          <path key={`${e.from}-${e.to}`} className="docs-edge" d={e.d} />
        ))}

        <path className="docs-arch-pulse" pathLength={1} d={ARCH_PULSE_PATH} />

        {ARCH_NODES.map((n, i) => (
          <g key={n.id} className="docs-arch-node" style={{ animationDelay: `${i * 70}ms` }}>
            <rect className="docs-node-box" x={n.x} y={n.y} width={n.w} height={n.h} rx="4" />
            <text className="docs-node-label" x={n.x + 12} y={n.y + 26}>
              {n.label}
            </text>
            {n.sub && (
              <text className="docs-node-sub" x={n.x + 12} y={n.y + 42}>
                {n.sub}
              </text>
            )}
          </g>
        ))}

        <text className="docs-edge-note" x="344" y="98" textAnchor="middle">
          {ARCH_EDGES.find((e) => e.from === "runtime" && e.to === "gateway")?.note}
        </text>
        <text className="docs-edge-note" x="248" y="192" textAnchor="middle">
          {ARCH_EDGES.find((e) => e.from === "gateway" && e.to === "providers")?.note}
        </text>
      </svg>
    </Illustration>
  );
}
```

The `<g>` element is permitted here — the no-`<g>` rule in **R8** applies to 16×16 icon glyphs, not to page-level illustrations. Keep that distinction.

The two notes are rendered at fixed coordinates rather than mapped, because each needs its own placement and there are exactly two. Do **not** replace this with a `.filter(e => e.note).map(...)` — that stacks both strings at one point.

- [ ] **Step 3: Write `registry.tsx`**

```tsx
import type { JSX } from "react";
import { ArchitectureVisual } from "./ArchitectureVisual";
import type { DiagramKey } from "./keys";

/**
 * Compile-enforced completeness: this is `Record<DiagramKey, …>`, so adding a key
 * to DIAGRAM_KEYS without a component here is a tsc error, and a `diagram` block
 * can never name a visual that does not exist.
 */
export const DIAGRAM_REGISTRY: Record<DiagramKey, () => JSX.Element> = {
  architecture: ArchitectureVisual,
  governanceChain: ArchitectureVisual,
  requestLifecycle: ArchitectureVisual,
  councilFanout: ArchitectureVisual,
};
```

The three placeholder entries are replaced in Tasks 7 and 8. **Say so in a `// TODO(W2 Task 7/8)` comment on each placeholder line** so a reviewer can see they are deliberate, and remove the comments as they are replaced.

- [ ] **Step 4: Write the visual rules in `Docs.css`**

Create the file with this content. Layout rules are appended in Task 10.

```css
/* ---- Docs visuals ----
   Every class here is docs- prefixed: CSS is global in this app, and so are
   keyframe names. Colours come only from the tokens in styles.css; chroma stays
   reserved for machine state. */

.docs-fig {
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--inset);
  margin: 18px 0;
  overflow: hidden;
}

.docs-fig-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
}

.docs-fig-title {
  font-size: 12px;
  color: var(--text-dim);
}

/* The marker is deliberately quiet but always present: it is the difference
   between an illustration and a claim about the running system. */
.docs-fig-marker {
  color: var(--text-faint);
  white-space: nowrap;
}

.docs-fig-body {
  padding: 14px;
}

.docs-svg {
  display: block;
  width: 100%;
  height: auto;
  overflow: visible;
}

/* ---- shared SVG vocabulary ---- */

.docs-node-box {
  fill: var(--raised);
  stroke: var(--border-strong);
  stroke-width: 1;
}

.docs-node-label {
  fill: var(--text);
  font-family: var(--mono);
  font-size: 12px;
}

.docs-node-sub {
  fill: var(--text-faint);
  font-family: var(--mono);
  font-size: 10px;
}

.docs-edge {
  fill: none;
  stroke: var(--border-strong);
  stroke-width: 1;
}

.docs-edge-note {
  fill: var(--text-faint);
  font-family: var(--mono);
  font-size: 10px;
}

/* ---- architecture ---- */

.docs-arch-node {
  opacity: 0;
  animation: docs-node-in var(--dur-med) var(--ease) forwards;
}

@keyframes docs-node-in {
  from {
    opacity: 0;
    transform: translateY(4px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* A travelling dash on a pathLength=1 route: the house technique from the usage
   chart, reused as a request tracing its way through the planes. */
.docs-arch-pulse {
  fill: none;
  stroke: var(--live);
  stroke-width: 1.5;
  stroke-linecap: round;
  stroke-dasharray: 0.06 0.94;
  stroke-dashoffset: 1;
  animation: docs-arch-pulse 4.5s linear infinite;
}

@keyframes docs-arch-pulse {
  to {
    stroke-dashoffset: 0;
  }
}

/* The global cap sets animation-iteration-count: 1, which would leave the pulse
   frozen mid-route and the nodes mid-fade. Author the still explicitly. */
@media (prefers-reduced-motion: reduce) {
  .docs-arch-node {
    animation: none;
    opacity: 1;
  }

  .docs-arch-pulse {
    animation: none;
    stroke-dasharray: none;
    stroke-dashoffset: 0;
    opacity: 0.5;
  }
}
```

- [ ] **Step 5: Type-check**

Run: `cd console && npx tsc`
Expected: an error only on `content.ts`'s `"docs"` icon name, which Task 11 resolves. **No error may originate in any file this task created.** If one does, fix it.

- [ ] **Step 6: Commit**

```bash
git add console/src/pages/docs/visuals/Illustration.tsx \
        console/src/pages/docs/visuals/ArchitectureVisual.tsx \
        console/src/pages/docs/visuals/registry.tsx console/src/pages/Docs.css
git commit -m "feat(console): add the docs illustration frame and architecture visual"
```

---

## Task 7: The governance chain visual

The only visual with a running timer, and the one that sits closest to a live instrument — so it carries the strictest rules.

**Files:**
- Create: `console/src/pages/docs/visuals/GovernanceChainVisual.tsx`
- Modify: `console/src/pages/docs/visuals/registry.tsx` (replace the placeholder)
- Modify: `console/src/pages/Docs.css` (append the conveyor rules)

**Interfaces:**
- Consumes: `CHAIN_STAGES` from `../../../lib/chain` (Task 1 — the corrected list); `GOVERNANCE_SCRIPT`, `GOVERNANCE_FRAME_MS`, `clearedCount` from `./geometry`.
- Produces: `GovernanceChainVisual(): JSX.Element`.

- [ ] **Step 1: Write the component**

```tsx
import { useEffect, useState } from "react";
import { useReducedMotion } from "framer-motion";
import { CHAIN_STAGES } from "../../../lib/chain";
import { GOVERNANCE_FRAME_MS, GOVERNANCE_SCRIPT, clearedCount } from "./geometry";
import { Illustration } from "./Illustration";

/**
 * The gateway's real /v1/* pipeline, stepped through a fixed script.
 *
 * Deliberately NOT the topbar chain: that one is lit by recorded audit statuses
 * and is evidence. This one is a scripted illustration, so it is drawn larger,
 * captioned, and marked as such. It shares CHAIN_STAGES with the instrument so
 * the two can never drift apart on what the stages are.
 */
export function GovernanceChainVisual() {
  const reduced = useReducedMotion();
  const [frameIndex, setFrameIndex] = useState(0);

  useEffect(() => {
    // Never start the timer under reduced motion: the global CSS cap does not
    // touch JS intervals, and a still is the whole point there.
    if (reduced) return;

    let id: number | undefined;
    const start = () => {
      if (id === undefined) {
        id = window.setInterval(() => {
          setFrameIndex((i) => (i + 1) % GOVERNANCE_SCRIPT.length);
        }, GOVERNANCE_FRAME_MS);
      }
    };
    const stop = () => {
      if (id !== undefined) {
        window.clearInterval(id);
        id = undefined;
      }
    };
    // installVisibilityPause only pauses live-resource polls, not our timers.
    const onVisibility = () => (document.hidden ? stop() : start());

    start();
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      stop();
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [reduced]);

  // Under reduced motion, show the frame that teaches the most: a denial, so the
  // "stages after the halt stay unlit" rule is visible in the still.
  const frame = reduced
    ? (GOVERNANCE_SCRIPT.find((f) => f.stoppedAt !== null) ?? GOVERNANCE_SCRIPT[0])
    : GOVERNANCE_SCRIPT[frameIndex];
  const cleared = clearedCount(frame);

  return (
    <Illustration diagram="governanceChain">
      <ol className="docs-chain" data-outcome={frame.outcome}>
        {CHAIN_STAGES.map((stage, i) => {
          const render =
            frame.stoppedAt === stage ? "stopped" : i < cleared ? "cleared" : "unlit";
          return (
            <li key={stage} className="docs-chain-stage" data-render={render}>
              <span className="docs-chain-node" aria-hidden />
              <span className="docs-chain-label mono">{stage}</span>
            </li>
          );
        })}
      </ol>
      <p className="docs-chain-caption mono" aria-live="off">
        {frame.caption}
      </p>
    </Illustration>
  );
}
```

Note `aria-live="off"`: this is a decorative loop, and announcing every frame to a screen reader would be noise. The `Illustration` marker plus the `role="img"` label on the sibling visuals carry the accessible meaning.

- [ ] **Step 2: Replace the registry placeholder**

In `registry.tsx`, import `GovernanceChainVisual` and set `governanceChain: GovernanceChainVisual`. Remove that line's `TODO` comment.

- [ ] **Step 3: Append the conveyor rules to `Docs.css`**

```css
/* ---- governance conveyor ----
   Visually distinct from the topbar instrument by contract: bigger nodes, a
   caption, and the illustration marker above it. The hue vocabulary is shared,
   because the hues mean the same thing in both places. */

.docs-chain {
  display: flex;
  align-items: flex-start;
  gap: 4px;
  list-style: none;
  padding: 4px 0 0;
}

.docs-chain-stage {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  position: relative;
}

.docs-chain-stage::before {
  content: "";
  position: absolute;
  top: 6px;
  left: 50%;
  right: -50%;
  height: 1px;
  background: var(--border-strong);
}

.docs-chain-stage:last-child::before {
  display: none;
}

.docs-chain-node {
  width: 13px;
  height: 13px;
  border-radius: 2px;
  background: var(--bg);
  box-shadow: inset 0 0 0 1px var(--border-strong);
  position: relative;
  z-index: 1;
  transition: background var(--dur-med) var(--ease), box-shadow var(--dur-med) var(--ease);
}

.docs-chain-label {
  font-size: 10px;
  color: var(--text-faint);
  transition: color var(--dur-med) var(--ease);
}

.docs-chain-stage[data-render="cleared"] .docs-chain-node {
  background: var(--text-dim);
  box-shadow: inset 0 0 0 1px var(--text-dim);
}

.docs-chain-stage[data-render="cleared"] .docs-chain-label {
  color: var(--text-dim);
}

/* The halting stage takes the outcome colour; everything after it stays dark,
   because the request never reached it. */
.docs-chain[data-outcome="deny"] .docs-chain-stage[data-render="stopped"] .docs-chain-node {
  background: var(--deny);
  box-shadow: inset 0 0 0 1px var(--deny), 0 0 0 3px rgba(226, 104, 95, 0.16);
}

.docs-chain[data-outcome="deny"] .docs-chain-stage[data-render="stopped"] .docs-chain-label {
  color: var(--deny);
}

.docs-chain[data-outcome="fail"] .docs-chain-stage[data-render="stopped"] .docs-chain-node {
  background: var(--hold);
  box-shadow: inset 0 0 0 1px var(--hold), 0 0 0 3px rgba(227, 168, 81, 0.16);
}

.docs-chain[data-outcome="fail"] .docs-chain-stage[data-render="stopped"] .docs-chain-label {
  color: var(--hold);
}

.docs-chain-caption {
  margin-top: 14px;
  font-size: 11px;
  color: var(--text-faint);
}

/* Labels drop before nodes do — the same order the live chain uses. */
@media (max-width: 720px) {
  .docs-chain-label {
    display: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .docs-chain-node,
  .docs-chain-label {
    transition: none;
  }
}
```

A clean pass leaves the cleared nodes graphite — **do not** light the whole chain green on success. The shipped instrument's rule is that nothing needing attention stays quiet, and contradicting it here would teach the wrong reading of the real one.

- [ ] **Step 4: Type-check and test**

Run: `cd console && npx tsc && npx vitest run`
Expected: `tsc` clean except the known `"docs"` icon error; all tests pass.

- [ ] **Step 5: Commit**

```bash
git add console/src/pages/docs/visuals/GovernanceChainVisual.tsx \
        console/src/pages/docs/visuals/registry.tsx console/src/pages/Docs.css
git commit -m "feat(console): add the governance chain docs visual"
```

---

## Task 8: The request-lifecycle and council-fanout visuals

Both are static structure with a gentle reveal — no timers, no intervals.

**Files:**
- Create: `console/src/pages/docs/visuals/RequestLifecycleVisual.tsx`
- Create: `console/src/pages/docs/visuals/CouncilFanoutVisual.tsx`
- Modify: `console/src/pages/docs/visuals/registry.tsx` (replace the last two placeholders)
- Modify: `console/src/pages/Docs.css` (append)

**Interfaces:**
- Consumes: `LIFECYCLE_HOPS`, `COUNCIL_MEMBERS` from `./geometry`; `Illustration` from `./Illustration`.
- Produces: `RequestLifecycleVisual()`, `CouncilFanoutVisual()`. Completes `DIAGRAM_REGISTRY`.

- [ ] **Step 1: Write `RequestLifecycleVisual.tsx`**

Four hops on a rule, each with a callout naming what it clears. The callouts carry the corrected governance facts.

```tsx
import { LIFECYCLE_HOPS } from "./geometry";
import { Illustration } from "./Illustration";

export function RequestLifecycleVisual() {
  return (
    <Illustration diagram="requestLifecycle">
      <svg
        className="docs-svg"
        viewBox="0 0 640 190"
        role="img"
        aria-label="A governed request: the client presents a virtual key, the gateway authorises, meters, screens and records it, the runtime runs the agent without holding a provider key, and a tool call is constrained inside the connector or sandbox."
      >
        <path className="docs-edge" d="M56 70H568" />

        {LIFECYCLE_HOPS.map((hop, i) => (
          <g key={hop.id} className="docs-hop" style={{ animationDelay: `${i * 80}ms` }}>
            <circle className="docs-hop-node" cx={hop.x} cy="70" r="6" />
            <text className="docs-node-label" x={hop.x} y="46" textAnchor="middle">
              {hop.label}
            </text>
            <text className="docs-node-sub" x={hop.x} y="102" textAnchor="middle">
              {hop.detail}
            </text>
          </g>
        ))}
      </svg>

      <ul className="docs-callouts">
        <li>
          Denials are not all recorded. Among refusals only rate-limit rejections and guardrail
          events reach the audit log — a 401, a 400, and a 402 budget exhaustion do not.
        </li>
        <li>
          The budget hold fails open only on a store error, and unlike the guardrail&apos;s
          <span className="mono"> guardrail_error</span> that admission leaves no audit entry.
        </li>
        <li>
          The guardrail stage exists only when <span className="mono">AGENTOS_GUARDRAILS_MODE</span>{" "}
          is not <span className="mono">off</span>; <span className="mono">/v1/embeddings</span> runs
          the same chain without it.
        </li>
      </ul>
    </Illustration>
  );
}
```

- [ ] **Step 2: Write `CouncilFanoutVisual.tsx`**

```tsx
import { COUNCIL_MEMBERS } from "./geometry";
import { Illustration } from "./Illustration";

export function CouncilFanoutVisual() {
  return (
    <Illustration diagram="councilFanout">
      <svg
        className="docs-svg"
        viewBox="0 0 640 260"
        role="img"
        aria-label="One objective fans out to five model-bound members; a judge synthesises a single verdict and an explicit dissent report."
      >
        <rect className="docs-node-box" x="16" y="112" width="104" height="44" rx="4" />
        <text className="docs-node-label" x="28" y="139">
          objective
        </text>

        {COUNCIL_MEMBERS.map((m, i) => (
          <g key={m.id} className="docs-hop" style={{ animationDelay: `${i * 60}ms` }}>
            <path className="docs-edge" d={`M120 134C180 134 180 ${m.y + 16} 240 ${m.y + 16}`} />
            <rect className="docs-node-box" x="240" y={m.y} width="120" height="32" rx="4" />
            <text className="docs-node-label" x="252" y={m.y + 21}>
              {m.label}
            </text>
            <path className="docs-edge" d={`M360 ${m.y + 16}C400 ${m.y + 16} 400 134 430 134`} />
          </g>
        ))}

        <rect className="docs-node-box" x="430" y="112" width="88" height="44" rx="4" />
        <text className="docs-node-label" x="442" y="139">
          judge
        </text>

        <path className="docs-edge" d="M518 134H556" />
        <rect className="docs-node-box" x="556" y="98" width="72" height="34" rx="4" />
        <text className="docs-node-sub" x="566" y="119">
          verdict
        </text>
        <rect className="docs-node-box" x="556" y="140" width="72" height="34" rx="4" />
        <text className="docs-node-sub" x="566" y="161">
          dissent
        </text>
      </svg>

      <ul className="docs-callouts">
        <li>
          Disagreement is recorded, not averaged away — the dissent report ships beside the verdict.
        </li>
        <li>
          Write-class tool calls become human-approved proposals. That gating is enforced in-graph
          for <span className="mono">react</span>-profile members; <span className="mono">deep</span>
          -profile members are held to an explicit read-only tool allowlist instead, because
          deepagents exposes no interrupt point.
        </li>
      </ul>
    </Illustration>
  );
}
```

- [ ] **Step 3: Complete the registry**

Replace the last two placeholders so `DIAGRAM_REGISTRY` reads:

```tsx
export const DIAGRAM_REGISTRY: Record<DiagramKey, () => JSX.Element> = {
  architecture: ArchitectureVisual,
  governanceChain: GovernanceChainVisual,
  requestLifecycle: RequestLifecycleVisual,
  councilFanout: CouncilFanoutVisual,
};
```

No `TODO` comments should remain in the file.

- [ ] **Step 4: Append the shared reveal and callout rules to `Docs.css`**

```css
/* ---- shared reveal for static visuals ---- */

.docs-hop {
  opacity: 0;
  animation: docs-node-in var(--dur-med) var(--ease) forwards;
}

.docs-hop-node {
  fill: var(--raised-2);
  stroke: var(--border-strong);
  stroke-width: 1;
}

.docs-callouts {
  list-style: none;
  margin: 14px 0 0;
  padding: 0;
  display: grid;
  gap: 8px;
}

.docs-callouts li {
  font-size: 12px;
  line-height: 1.55;
  color: var(--text-dim);
  padding-left: 12px;
  border-left: 1px solid var(--border-strong);
}

@media (prefers-reduced-motion: reduce) {
  .docs-hop {
    animation: none;
    opacity: 1;
  }
}
```

- [ ] **Step 5: Type-check and test**

Run: `cd console && npx tsc && npx vitest run`
Expected: `tsc` clean except the known `"docs"` icon error; all tests pass.

- [ ] **Step 6: Commit**

```bash
git add console/src/pages/docs/visuals/RequestLifecycleVisual.tsx \
        console/src/pages/docs/visuals/CouncilFanoutVisual.tsx \
        console/src/pages/docs/visuals/registry.tsx console/src/pages/Docs.css
git commit -m "feat(console): add the request-lifecycle and council-fanout visuals"
```

---

## Task 9: The block renderer

**Files:**
- Create: `console/src/pages/docs/DocBlocks.tsx`
- Modify: `console/src/pages/Docs.css` (append block rules)

**Interfaces:**
- Consumes: `DocBlock` from `./types`; `DIAGRAM_REGISTRY` from `./visuals/registry`.
- Produces: `DocBlocks({ blocks }: { blocks: DocBlock[] }): JSX.Element`. Task 10 renders it.

- [ ] **Step 1: Write the component**

```tsx
import { DIAGRAM_REGISTRY } from "./visuals/registry";
import type { DocBlock } from "./types";

function Block({ block }: { block: DocBlock }) {
  switch (block.kind) {
    case "prose":
      return <p className="docs-prose">{block.text}</p>;

    case "list":
      return (
        <ul className="docs-list">
          {block.items.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      );

    case "code":
      return (
        <div className="docs-code">
          {block.lang && <span className="docs-code-lang eyebrow">{block.lang}</span>}
          {/* prompt-text is the only container-safe block style in this app:
              .panel is overflow:hidden, so a bare <pre> is silently clipped. */}
          <pre className="prompt-text mono">{block.code}</pre>
        </div>
      );

    case "keyvals":
      return (
        <div className="docs-keyvals">
          {block.caption && <span className="docs-keyvals-cap eyebrow">{block.caption}</span>}
          <dl>
            {block.rows.map((row) => (
              <div key={row.k} className="docs-kv">
                <dt className="mono">{row.k}</dt>
                <dd>{row.v}</dd>
              </div>
            ))}
          </dl>
        </div>
      );

    case "note":
      // Monochrome by default: .warn and .error carry hue, and hue is reserved
      // for machine state.
      return <div className="notice docs-note">{block.text}</div>;

    case "diagram": {
      const Visual = DIAGRAM_REGISTRY[block.diagram];
      return (
        <>
          <Visual />
          {block.caption && <p className="docs-fig-caption">{block.caption}</p>}
        </>
      );
    }

    default: {
      const exhaustive: never = block;
      return exhaustive;
    }
  }
}

export function DocBlocks({ blocks }: { blocks: DocBlock[] }) {
  return (
    <>
      {blocks.map((block, i) => (
        <Block key={`${block.kind}-${i}`} block={block} />
      ))}
    </>
  );
}
```

The `default: never` arm is what makes adding a seventh block kind a compile error rather than a silently blank render.

- [ ] **Step 2: Append the block rules to `Docs.css`**

```css
/* ---- blocks ---- */

.docs-prose {
  font-size: 13px;
  line-height: 1.62;
  color: var(--text-dim);
  margin: 0 0 14px;
  max-width: 68ch;
}

.docs-list {
  margin: 0 0 14px 18px;
  padding: 0;
  display: grid;
  gap: 6px;
  font-size: 13px;
  line-height: 1.55;
  color: var(--text-dim);
  max-width: 68ch;
}

.docs-code {
  margin: 0 0 14px;
}

.docs-code-lang {
  display: block;
  margin-bottom: 6px;
  color: var(--text-faint);
}

.docs-keyvals {
  margin: 0 0 14px;
}

.docs-keyvals-cap {
  display: block;
  margin-bottom: 8px;
  color: var(--text-faint);
}

.docs-keyvals dl {
  display: grid;
  gap: 0;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  overflow: hidden;
}

.docs-kv {
  display: grid;
  grid-template-columns: minmax(140px, 240px) 1fr;
  gap: 14px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border);
  font-size: 12px;
}

.docs-kv:last-child {
  border-bottom: none;
}

.docs-kv dt {
  color: var(--text);
  word-break: break-word;
}

.docs-kv dd {
  margin: 0;
  color: var(--text-dim);
  line-height: 1.5;
}

.docs-note {
  margin: 0 0 14px;
  font-size: 12px;
  line-height: 1.55;
}

.docs-fig-caption {
  font-size: 11px;
  color: var(--text-faint);
  margin: -6px 0 14px;
}

@media (max-width: 720px) {
  .docs-kv {
    grid-template-columns: 1fr;
    gap: 4px;
  }
}
```

- [ ] **Step 3: Type-check**

Run: `cd console && npx tsc`
Expected: clean except the known `"docs"` icon error.

- [ ] **Step 4: Commit**

```bash
git add console/src/pages/docs/DocBlocks.tsx console/src/pages/Docs.css
git commit -m "feat(console): add the docs block renderer"
```

---

## Task 10: The Docs page

**Files:**
- Create: `console/src/pages/Docs.tsx`
- Modify: `console/src/pages/Docs.css` (append the layout and rail rules)

**Interfaces:**
- Consumes: `PageProps` (type-only) from `../App`; `DOC_SECTIONS` from `./docs/content`; `DocBlocks` from `./docs/DocBlocks`; `PageHead` from `../components/common`; `Panel`, `PanelHead`, `transition`, `fadeRise`, `fadeRiseReduced` from `../ui`; `Icon` from `../ui/icons`.
- Produces: `export function Docs(_props: PageProps): JSX.Element`. Task 11 registers it.

- [ ] **Step 1: Write the page**

```tsx
import { useEffect, useState } from "react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import type { PageProps } from "../App";
import { PageHead } from "../components/common";
import { Panel, PanelHead, fadeRise, fadeRiseReduced, transition } from "../ui";
import { Icon } from "../ui/icons";
import { DOC_SECTIONS } from "./docs/content";
import { DocBlocks } from "./docs/DocBlocks";
import "./Docs.css";

/** Read ?s=<id> once on mount. The router only ever looks at pathname, so an
 *  inbound /docs?s=gateway link lands here correctly. */
function initialSectionId(): string {
  if (typeof window === "undefined") return DOC_SECTIONS[0].id;
  const wanted = new URLSearchParams(window.location.search).get("s");
  return DOC_SECTIONS.some((s) => s.id === wanted) ? (wanted as string) : DOC_SECTIONS[0].id;
}

export function Docs(_props: PageProps) {
  const reduced = useReducedMotion();
  const [activeId, setActiveId] = useState(initialSectionId);

  const active = DOC_SECTIONS.find((s) => s.id === activeId) ?? DOC_SECTIONS[0];

  useEffect(() => {
    // replaceState, never the router's navigate(): navigate stores its whole
    // argument as the route key and matches by exact equality, so a query-bearing
    // path silently falls through to Overview.
    window.history.replaceState(null, "", `/docs?s=${active.id}`);
  }, [active.id]);

  return (
    <>
      <PageHead
        title="Docs"
        subtitle="What this platform does, how the governance works, and how to run it — without leaving the console."
      />

      <div className="docs-layout">
        <nav className="docs-rail" aria-label="Documentation sections">
          {DOC_SECTIONS.map((s) => {
            const isActive = s.id === active.id;
            return (
              <button
                key={s.id}
                type="button"
                className={`docs-rail-item${isActive ? " active" : ""}`}
                aria-current={isActive ? "true" : undefined}
                onClick={() => setActiveId(s.id)}
              >
                {isActive &&
                  (reduced ? (
                    <span className="docs-rail-pill" aria-hidden />
                  ) : (
                    <motion.span
                      className="docs-rail-pill"
                      aria-hidden
                      layoutId="docs-rail-pill"
                      transition={transition}
                    />
                  ))}
                <Icon name={s.icon} size={14} className="docs-rail-icon" />
                <span className="docs-rail-label">{s.title}</span>
              </button>
            );
          })}
        </nav>

        <div className="docs-body">
          <Panel>
            <PanelHead title={active.title} />
            <div className="panel-body">
              <p className="docs-blurb">{active.blurb}</p>
              <AnimatePresence mode="wait" initial={false}>
                <motion.div
                  key={active.id}
                  variants={reduced ? fadeRiseReduced : fadeRise}
                  initial="hidden"
                  animate="show"
                  exit="exit"
                >
                  <DocBlocks blocks={active.blocks} />
                </motion.div>
              </AnimatePresence>
            </div>
          </Panel>
        </div>
      </div>
    </>
  );
}
```

`layoutId="docs-rail-pill"` — **never** `"nav-pill"`. That string is document-global and would make the sidebar's pill fly into this rail.

- [ ] **Step 2: Append the layout rules to `Docs.css`**

```css
/* ---- page layout ---- */

.docs-layout {
  display: grid;
  grid-template-columns: 208px 1fr;
  gap: 24px;
  align-items: start;
}

.docs-rail {
  position: sticky;
  /* clears the sticky topbar */
  top: 76px;
  display: grid;
  gap: 2px;
}

.docs-rail-item {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 7px 10px;
  border: none;
  background: transparent;
  border-radius: var(--radius-sm);
  color: var(--text-faint);
  font-family: var(--sans);
  font-size: 12px;
  text-align: left;
  cursor: pointer;
  transition: color var(--dur-fast) var(--ease);
}

.docs-rail-item:hover {
  color: var(--text-dim);
}

.docs-rail-item.active {
  color: var(--text);
}

/* Own pill class and own layoutId — the sidebar's are global. */
.docs-rail-pill {
  position: absolute;
  inset: 0;
  border-radius: var(--radius-sm);
  background: var(--raised-2);
  box-shadow: inset 2px 0 0 0 var(--text-dim);
}

.docs-rail-icon,
.docs-rail-label {
  position: relative;
  z-index: 1;
}

.docs-rail-label {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.docs-blurb {
  font-size: 12px;
  color: var(--text-faint);
  margin: 0 0 18px;
}

@media (max-width: 900px) {
  .docs-layout {
    grid-template-columns: 1fr;
  }

  .docs-rail {
    position: static;
    grid-auto-flow: column;
    grid-auto-columns: max-content;
    overflow-x: auto;
    padding-bottom: 6px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .docs-rail-item {
    transition: none;
  }
}
```

- [ ] **Step 3: Type-check**

Run: `cd console && npx tsc`
Expected: clean except the known `"docs"` icon error in `content.ts`.

- [ ] **Step 4: Commit**

```bash
git add console/src/pages/Docs.tsx console/src/pages/Docs.css
git commit -m "feat(console): add the Docs page with a sticky section rail"
```

---

## Task 11: The icon and the route

The last compile error clears here.

**Files:**
- Modify: `console/src/ui/icons.tsx` (add `"docs"` to `IconName` and a `GLYPHS` entry)
- Modify: `console/src/App.tsx` (import + `ROUTES` entry)

**Interfaces:**
- Consumes: `Docs` from `./pages/Docs`.
- Produces: a reachable `/docs` route. The Sidebar and ⌘K palette pick it up with no further edit.

- [ ] **Step 1: Add the icon name**

In `console/src/ui/icons.tsx`, add `| "docs"` to the `IconName` union. Keep the union's existing formatting.

- [ ] **Step 2: Add the glyph**

Add to `GLYPHS`, following the file's idiom — a prose comment explaining the shape and why it is not the neighbouring glyph, then the entry:

```tsx
  // An open manual: one spine, two leaves. Deliberately not `documents`, which
  // is two offset sheets — a corpus you search. This is a book you read, so the
  // silhouette differs at the outline, not in a corner fold.
  docs: (
    <>
      <path d="M8 4.5 2.5 3.5v8L8 13" />
      <path d="M8 4.5 13.5 3.5v8L8 13" />
      <path d="M8 4.5v8.5" />
    </>
  ),
```

Every coordinate is a whole or half unit inside the 2…14 band. The glyph sets no `stroke`, `fill`, `width`, or `viewBox` — the wrapper supplies all four.

- [ ] **Step 3: Register the route**

In `console/src/App.tsx`, add the import alongside the other page imports:

```ts
import { Docs } from "./pages/Docs";
```

and add this entry to `ROUTES` **last among the always-visible routes — immediately after the `/operators` entry and before the `visible`-gated `/orgs` block**:

```ts
  { path: "/docs", label: "Docs", icon: "docs", Component: Docs },
```

**No `visible` predicate** — the handbook is available to every role. Do **not** edit `CommandPalette.tsx`; it derives its entries from `ROUTES` automatically and editing it produces a duplicate.

- [ ] **Step 4: Type-check — this must now be fully clean**

Run: `cd console && npx tsc`
Expected: **no output at all.** The `"docs"` icon error from Tasks 4–10 is now resolved. If anything remains, fix it before proceeding.

- [ ] **Step 5: Run the full suite and the production build**

Run: `cd console && npx vitest run && npm run build`
Expected: all test files pass; the build completes with no TypeScript errors.

- [ ] **Step 6: Commit**

```bash
git add console/src/ui/icons.tsx console/src/App.tsx
git commit -m "feat(console): add the docs glyph and register the /docs route"
```

---

## Task 12: Point the documentation at the new tab

W1 deliberately did not link an in-console Docs tab because none existed. It does now.

**Files:**
- Modify: `README.md` (hero)
- Modify: `docs/console.md` (pages table + prose)

**Interfaces:**
- Consumes: the shipped `/docs` route.
- Produces: documentation consistent with the product.

- [ ] **Step 1: Add the Docs tab to the README hero**

The hero currently links the landing page and `docs/console.md`. Add a clause noting that the running console carries the same handbook at `/docs` — reachable from the sidebar or ⌘K — so an operator never has to leave the product to learn what it does. Keep it to one sentence; do not restructure the hero.

- [ ] **Step 2: Update `docs/console.md`**

- Add a **Docs** row to the pages table: path `/docs`, purpose "in-console handbook — capabilities, architecture, governance, quickstart", backed by "nothing — static content, no API calls", visible to "every role".
- The file states the console has twelve pages; it now has **thirteen**. Update every count.
- Add one sentence to the page's prose noting that Docs is the only page that makes no API calls, so it works with no admin key set.

- [ ] **Step 3: Verify the counts and links**

Run:
```bash
cd /home/iofahd/code/agentos
grep -n '12 ' docs/console.md | head
grep -c '^| ' docs/console.md
grep -n '/docs' README.md docs/console.md
```
Expected: no surviving "12 pages" claim; the table has one more row than before; both files mention `/docs`.

- [ ] **Step 4: Commit**

```bash
git add README.md docs/console.md
git commit -m "docs: point the README and console reference at the in-console Docs tab"
```

---

## Task 13: Whole-feature verification

This task exists to catch what per-file review cannot: cross-file drift, animation that misbehaves only when running, and claims in the UI that contradict the shipped docs.

**Files:**
- Modify: whichever files the checks flag.

**Interfaces:**
- Consumes: everything Tasks 1–12 produced.
- Produces: a verified feature.

- [ ] **Step 1: Clean type-check, full suite, production build**

Run:
```bash
cd console && npx tsc && npx vitest run && npm run build
```
Expected: `tsc` silent; every test file passes; the build succeeds. Paste the real output into your report.

- [ ] **Step 2: Assert the work-stream's own invariants mechanically**

Run:
```bash
cd console
echo "--- no test may be .test.tsx (silently uncollected) ---"
find src -name '*.test.tsx' | grep . && echo "FAIL" || echo "OK: none"
echo "--- no Math.random in the docs feature ---"
grep -rn 'Math.random' src/pages/docs src/pages/Docs.tsx && echo "FAIL" || echo "OK: none"
echo "--- no SMIL ---"
grep -rnE '<animate|animateTransform|animateMotion' src && echo "FAIL" || echo "OK: none"
echo "--- no light-theme rules ---"
grep -rn 'prefers-color-scheme' src && echo "FAIL" || echo "OK: none"
echo "--- the sidebar layoutId is not reused ---"
grep -rn 'layoutId="nav-pill"' src | grep -v 'components/Sidebar.tsx' && echo "FAIL" || echo "OK"
echo "--- every docs class and keyframe is prefixed ---"
grep -oE '^\.[a-z-]+|^@keyframes [a-z-]+' src/pages/Docs.css | grep -vE '^(\.docs-|@keyframes docs-)' || echo "OK: all prefixed"
echo "--- the docs page issues no API calls and reads no adminKey ---"
grep -rnE 'apiFetch|useLiveResource|useLoad|fetch\(|adminKey' src/pages/Docs.tsx src/pages/docs && echo "FAIL" || echo "OK: none"
```
Expected: every line reports OK. Fix anything that does not.

- [ ] **Step 3: Confirm the docs content agrees with the shipped GitHub docs**

Run:
```bash
cd /home/iofahd/code/agentos
echo "--- forbidden claims anywhere in the console content ---"
grep -rniE 'sha256-pinned|every outcome|including denials|AGENTOS_GUARDRAIL_MODE' console/src/pages/docs && echo "FAIL" || echo "OK: none"
echo "--- the pipeline order matches the docs ---"
grep -rn 'guardrail' console/src/lib/chain.ts docs/architecture.md | head
```
Expected: no forbidden claims; the stage vocabulary matches `docs/architecture.md`.

- [ ] **Step 4: Manual check in the running console**

The stack is already running; the console is served at `http://192.168.100.15:13000` on this host (ports were overridden — 3000 and 8000 were taken). Rebuild the console image or run the dev server, then verify each of the following and record the result in your report:

1. **Nav** — "Docs" appears in the sidebar between Operators and the role-gated block, with the new glyph. ⌘K lists "Go to Docs".
2. **Rail** — clicking through all twelve sections renders each one; the pill moves within the rail and **the sidebar pill never moves**.
3. **Deep link** — loading `http://192.168.100.15:13000/docs?s=gateway` directly opens the Gateway section. Clicking other sections updates the URL without a route change and without falling back to Overview.
4. **Animation** — the architecture pulse traces the route and loops; the governance conveyor advances through its script and shows a clean pass, a rate denial, a guardrail block, and an upstream failure; stages after a halt stay dark.
5. **Reduced motion** — with the OS preference set to reduce, every visual shows a deliberate static still, the governance conveyor shows a denial frame, and **no interval is running** (confirm in devtools: the conveyor's `setInterval` must never be created).
6. **Background tab** — switch away for 30 s and back; the conveyor is not queued up or racing.
7. **Overflow** — no horizontal scrollbar at 1280 px, 1024 px, and 720 px. At 720 px the chain labels drop and the rail becomes a horizontal scroller.
8. **No key** — sign out / clear the admin key. The Docs page still renders completely; the only degraded chrome is the topbar chain and sidebar dot, which are shell concerns.
9. **Network** — with devtools open on `/docs`, the only requests are the pre-existing shell polls (`/admin/audit`, `/admin/usage`, 5 s each, and only when a key is set). **No third-party, CDN, font, or image request.**

- [ ] **Step 5: Fix anything the manual check found, then re-run Step 1**

- [ ] **Step 6: Commit any fixes**

```bash
git add -A
git commit -m "fix(console): address findings from the docs-tab verification pass"
```

If Steps 1–4 found nothing, skip the commit and say so.

---

## Completion

When Task 13 passes: merge to `main` locally (fast-forward), delete the working branch and the SDD workspace, and **do not push**.
