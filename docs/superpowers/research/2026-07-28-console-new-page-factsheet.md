# AgentOS console — authoritative fact sheet for adding a new page, four animated SVG visuals, an icon, a route, and node-environment tests

**Scope.** Everything an engineer needs to build that work WITHOUT opening the codebase. Every signature, class name, token, string literal and `file:line` below is preserved verbatim from source-verified subsystem sweeps. Where two sweeps disagree, both are given with citations (see `Source disagreements` at the end of the relevant section and in TRAPS).

**Path resolution.** The orchestrating prompt used a literal `undefined` prefix. It resolves to `/home/iofahd/code/agentos`. So `undefined/console` = `/home/iofahd/code/agentos/console`. Unless stated otherwise, paths in this document are relative to `/home/iofahd/code/agentos/console/` (e.g. `src/App.tsx` = `/home/iofahd/code/agentos/console/src/App.tsx`). Landing-page paths are relative to `/home/iofahd/code/agentos/landing/`.

**Context.** A spec for this work already exists at `/home/iofahd/code/agentos/docs/superpowers/specs/2026-07-27-platform-docs-and-console-docs-tab.md`. It plans `interface DocSection { id: string; title: string; icon: IconName; blocks: DocBlock[] }` (spec:139), `export const DOC_SECTIONS: DocSection[];` (spec:140), `export const DIAGRAM_KEYS = [...] as const;` (spec:150), sanctions "edits to `src/ui/icons.tsx`" (spec:126), and calls the exhaustive-`Record` trick "the same compile-enforced pattern as `GLYPHS`" (spec:156). **None of that code exists yet** — `console/src/pages/docs/` DOES NOT EXIST; `grep -rni "docsection"` hits only the spec.

---

## Routing & shell

### Stack

No router library. `package.json:12-16` runtime deps are exactly `framer-motion`, `react`, `react-dom`. **react-router DOES NOT EXIST in this project.** Routing is hand-rolled in `src/App.tsx`.

`src/main.tsx` is the entire bootstrap — no provider or router wraps `<App/>`:

```tsx
createRoot(document.getElementById("root")!).render(
  <StrictMode><App /></StrictMode>,
);
```

Deep links work in production: `console/nginx.conf.template:45-46` has `location / { try_files $uri $uri/ /index.html; }`.

### `PageProps` (exported) — `src/App.tsx:45-51`

```ts
export interface PageProps {
  adminKey: string;
  role: AuthRole;
  orgId: string;
  openSettings: () => void;
  navigate: (path: string) => void;
}
```

### `Route` (NOT exported, module-private) — `src/App.tsx:53-60`, verbatim

```ts
interface Route {
  path: string;
  label: string;
  icon: IconName;
  Component: (props: PageProps) => React.JSX.Element;
  // When set, the nav item is shown only if the predicate holds for the role.
  visible?: (role: AuthRole) => boolean;
}
```

Only `visible` is optional. `Component` is a plain function component type returning `React.JSX.Element` — **not** `FC`, no `children`.

### `ROUTES: Route[]` in order — `src/App.tsx:62-99`

| # | line | path | label | icon | Component | `visible` |
|---|---|---|---|---|---|---|
| 1 | 63 | `/` | Overview | `overview` | `Overview` | — |
| 2 | 64 | `/keys` | Keys | `keys` | `Keys` | — |
| 3 | 65 | `/audit` | Audit | `audit` | `Audit` | — |
| 4 | 66 | `/playground` | Playground | `playground` | `Playground` | — |
| 5 | 67 | `/documents` | Documents | `documents` | `Documents` | — |
| 6 | 68 | `/improve` | Improve | `improve` | `Improve` | — |
| 7 | 69 | `/multiverse` | Multiverse | `multiverse` | `Multiverse` | — |
| 8 | 70 | `/operators` | Operators | `operators` | `Operators` | — |
| 9 | 71-77 | `/orgs` | Orgs | `orgs` | `Orgs` | `(r) => can(r, "org.view")` |
| 10 | 78-84 | `/users` | Users | `users` | `Users` | `(r) => can(r, "user.view")` |
| 11 | 85-91 | `/secrets` | Secrets | `secrets` | `Secrets` | `(r) => can(r, "secret.view")` |
| 12 | 92-98 | `/provisioning` | Provisioning | `provisioning` | `Provisioning` | `(r) => can(r, "provisioning.view")` |

The first 8 have **no** `visible` predicate (always shown). Page imports live at `src/App.tsx:32-43`, all named imports from `./pages/<Name>`.

Verbatim single-line entry shape (`src/App.tsx:69`):

```ts
{ path: "/multiverse", label: "Multiverse", icon: "multiverse", Component: Multiverse },
```

Gated entry shape (`src/App.tsx:71-77`): adds `visible: (r) => can(r, "org.view"),`.

**There is no `/docs` route.** `grep -rn '"/docs' console/src` returns zero hits. `/documents` is the Documents page (`App.tsx:67`).

`can` / `asAuthRole` / `roleLabel` live at `src/lib/rbac.ts:62-85`; `AuthRole = Role | "root"` (`src/lib/rbac.ts:14`); `can` returns `true` for every action when role is `"root"` (`rbac.ts:63`). `Action` union starts at `src/lib/rbac.ts:17`.

### `usePath` — `src/App.tsx:121-133`, verbatim

```ts
function usePath(): [string, (p: string) => void] {
  const [path, setPath] = useState(window.location.pathname);
  useEffect(() => {
    const onPop = () => setPath(window.location.pathname);
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);
  const navigate = useCallback((p: string) => {
    window.history.pushState({}, "", p);
    setPath(p);
  }, []);
  return [path, navigate];
}
```

### Active-route matching — `src/App.tsx:150`

```ts
const route = ROUTES.find((r) => r.path === path) ?? ROUTES[0];
```

Exact string equality against the raw `path` state. No prefix matching, no params, no wildcards, no trailing-slash normalisation. `/keys/` does NOT match `/keys`. An unknown path silently renders `ROUTES[0]` (Overview, `App.tsx:63`) with **no URL rewrite** — the address bar keeps the bogus path.

**Query/hash hazard (asymmetric).** `navigate("/documents?s=gateway")` calls `window.history.pushState({}, "", p)` **and** `setPath(p)` with the full string including the query (`App.tsx:128-131`). `"/documents" === "/documents?s=gateway"` is false → `?? ROUTES[0]` → Overview renders while the URL bar reads `/documents?s=gateway`. But `usePath` seeds from `window.location.pathname` (`App.tsx:122`) and `popstate` re-reads `window.location.pathname` (`App.tsx:124`) — both strip query and hash — so a hard load, a reload, or back/forward onto the same URL renders Documents correctly. **The same URL yields two different pages depending on how you arrived.** Never pass a query string or hash to `navigate()`. Minimal fix if you need one: `const base = path.split(/[?#]/)[0];` then `ROUTES.find((r) => r.path === base)`. Only two `navigate()` call sites exist today — `App.tsx:163` (`navigate(r.path)` from ⌘K) and `src/pages/Overview.tsx:225` (`navigate("/keys")`) — neither passes a query, so normalising regresses nothing. There is no query-param parsing anywhere in the shell.

### Shell render structure — `src/App.tsx:254-323`

- `<ToastProvider>` (imported from `./ui`, `App.tsx:31`) wraps everything, including the modals. Closes at `:322`.
- `<div className="shell">` (`:256`) — `display:flex; height:100%` (`src/styles.css:163-166`).
  - `<a className="skip-link" href="#main-content">Skip to content</a>` (`App.tsx:258-260`).
  - `<Sidebar …/>` (`App.tsx:261-268`) — pinned on every route.
  - `<main className="main" id="main-content">` (`App.tsx:269`) — `flex:1; overflow-y:auto; padding: 0 40px 64px` (`styles.css:233-237`).
    - `<header className="topbar">` (`App.tsx:273-282`) — pinned on every route; `position:sticky; top:0; z-index:20; max-width:1080px; margin:0 auto` (`styles.css:241-253`). Contents in order: `<span className="eyebrow topbar-legend">governance chain</span>`; conditionally `<span className="topbar-status"><StateIcon state={…} size={12}/><span className="eyebrow">{label}</span></span>`; `<Chain adminKey={adminKey} />`.
    - The page wrapper, `className="page"` — `max-width:1080px; margin:0 auto; padding-top:28px` (`styles.css:275-279`).
- Siblings of `.shell`, outside it, present on every route: `<SettingsModal …/>` (`App.tsx:301-316`) and `<CommandPalette open={paletteOpen} onClose={…} commands={commands} />` (`App.tsx:317-321`).

**Exact page call site — `src/App.tsx:244-252`** (five props, not zero):

```tsx
const page = (
  <route.Component
    adminKey={adminKey}
    role={role}
    orgId={orgId}
    openSettings={openSettings}
    navigate={navigate}
  />
);
```

Built once per render and inserted into whichever wrapper branch wins. No `children`, no route/params object, no context provider other than `ToastProvider` — pages get state only via these five props.

Sidebar call site (`App.tsx:261-268`): `routes={visibleRoutes} activePath={route.path} onNavigate={navigate} adminKey={adminKey} role={role} openSettings={openSettings}`, where `const visibleRoutes = ROUTES.filter((r) => !r.visible || r.visible(role));` (`App.tsx:152`). **`activePath` is fed the RESOLVED `route.path`, not the raw `path` state** — so on a fallback the Overview pill lights and the shell looks like a deliberate Overview navigation.

Topbar connection glyph from `connectionGlyph(conn)` (`App.tsx:106-119`): `live → {state:"live", label:"synced"}`, `stale → {state:"hold", label:"reconnecting"}`, `offline → {state:"deny", label:"offline"}`, `idle → null` (renders nothing).

Other shell-wide effects: SSO fragment pickup + `oidcStatusRequest()` probe + `refreshIdentity` on mount (`App.tsx:209-223`); `useEffect(() => installVisibilityPause(defaultRegistry), [])` (`App.tsx:227`); global ⌘K/Ctrl-K toggle (`App.tsx:231-240`) which calls `e.preventDefault()` and `setPaletteOpen((o) => !o)`.

### Page-transition wrapper — `src/App.tsx:283-298`, verbatim

```tsx
          {reducedMotion ? (
            <div className="page">{page}</div>
          ) : (
            <AnimatePresence mode="wait">
              <motion.div
                key={route.path}
                className="page"
                variants={pageTransition}
                initial="hidden"
                animate="show"
                exit="exit"
              >
                {page}
              </motion.div>
            </AnimatePresence>
          )}
```

`reducedMotion = useReducedMotion()` at `App.tsx:242`. `AnimatePresence mode="wait"`, remount keyed on `route.path` — **a route change fully unmounts the old page, so page-local state does not survive navigation.** The sidebar and topbar sit outside this wrapper and never re-mount.

### Sidebar nav-pill

Props (`SidebarProps` is not exported; `NavRoute` is) — `src/components/Sidebar.tsx:13-26`:

```ts
export interface NavRoute { path: string; label: string; icon: IconName; }
interface SidebarProps {
  routes: NavRoute[];
  activePath: string;
  onNavigate: (path: string) => void;
  adminKey: string;
  role: AuthRole;
  openSettings: () => void;
}
```

Nav item render — `src/components/Sidebar.tsx:94-120`, verbatim (includes the reduced-motion branch):

```tsx
      <nav className="nav">
        {routes.map((r) => {
          const active = r.path === activePath;
          return (
            <button
              key={r.path}
              className={`nav-item${active ? " active" : ""}`}
              aria-current={active ? "page" : undefined}
              onClick={() => onNavigate(r.path)}
            >
              {active &&
                (reduced ? (
                  <span className="nav-pill" aria-hidden />
                ) : (
                  <motion.span
                    className="nav-pill"
                    aria-hidden
                    layoutId="nav-pill"
                    transition={transition}
                  />
                ))}
              <Icon name={r.icon} size={15} className="nav-icon" />
              <span className="nav-item-label">{r.label}</span>
            </button>
          );
        })}
      </nav>
```

- layoutId string is the bare **`"nav-pill"`**; the CSS class is also `nav-pill`.
- `reduced = useReducedMotion()` (`Sidebar.tsx:84`); the reduced branch is a plain `<span className="nav-pill" aria-hidden />` with no layout animation and **no layoutId**.
- `transition` is `{ duration: DUR_MED, ease: EASE }` = `{ duration: 0.2, ease: [0.2,0.8,0.2,1] }` (`src/ui/motion.ts:26`, `:13-19`).
- Classes used by Sidebar: `sidebar`, `brand`, `brand-name`, `live`, `nav`, `nav-item`, `nav-item active`, `nav-pill`, `nav-icon`, `nav-item-label`, `sidebar-footer`, `dim`, `nav-note-hold`.
- Pill styling — `src/components/Sidebar.css:47-53`: `position:absolute; inset:0; border-radius: var(--radius-sm); background: var(--raised-2); box-shadow: inset 2px 0 0 0 var(--text-dim);`. The item itself is forced transparent by `Sidebar.css:39-43` (`.nav .nav-item:hover, .nav .nav-item.active, .sidebar-footer .nav-item:hover { background: transparent; }`), overriding `.nav-item.active { background: var(--accent-dim); }` in `src/styles.css:222-225`. `.nav .nav-item { position: relative; }` at `Sidebar.css:33-35`; label/icon get `position:relative; z-index:1` (`Sidebar.css:55-59`).
- Footer button (`Sidebar.tsx:121-135`): a `nav-item` (no pill) calling `openSettings`, label `Settings` plus `<span className="dim"> · {roleLabel(role).toLowerCase()}</span>` when `adminKey` is set, else `<span className="nav-note-hold"> · no key</span>`.
- Brand row (`Sidebar.tsx:89-93`): `<BrandMark size={18} />`, `<span className="brand-name">AgentOS</span>`, and `{live && <span className="live" title="run in flight" aria-label="agent run in flight" />}`.

### CommandPalette — no edit needed to add a route

`src/components/CommandPalette.tsx:10-21`:

```ts
export interface Command { id: string; label: string; icon?: IconName; run: () => void; }
export interface CommandPaletteProps { open: boolean; onClose: () => void; commands: Command[]; }
```

Entries are derived in **App.tsx**, `src/App.tsx:157-176`:

```tsx
const commands: Command[] = [
  ...visibleRoutes.map((r) => ({
    id: r.path,
    label: `Go to ${r.label}`,
    icon: r.icon,
    run: () => { navigate(r.path); setPaletteOpen(false); },
  })),
  { id: "settings", label: "Open Settings", icon: "settings", run: () => { openSettings(); setPaletteOpen(false); } },
];
```

**Adding a route requires NO edit in `CommandPalette.tsx`** — it is picked up automatically from `visibleRoutes` (already role-filtered), labeled `Go to <label>`, id = path. `run()` already calls `setPaletteOpen(false)` and `runAndClose` also calls `onClose()` (`CommandPalette.tsx:65-68`) — double close, harmless.

Palette internals: `createPortal(…, document.body)` (`CommandPalette.tsx:70`, `:144`); filtering is `filterBySearch(commands, ["label"], query)` from `../lib/table/search` (`:62`); ArrowDown/ArrowUp wrap modulo, Enter runs `filtered[activeIdx]`, Escape closes (`:100-116`) plus a window-level Escape listener (`:53-60`); classes `cmdk-backdrop`, `cmdk-panel`, `cmdk-search`, `cmdk-search-icon`, `cmdk-list`, `cmdk-item`, `cmdk-item active`, `empty`; motion variants `modalBackdrop` / `modalPanel` / `modalPanelReduced` from `../ui`; panel is `role="dialog" aria-modal="true" aria-label="Command palette"`, list is `role="listbox" aria-label="Commands"`, empty-state text is exactly `No matching commands`.

### Polling that runs on every route

Two live-resource subscriptions are mounted by shell chrome and run regardless of route. Both go through the shared registry, which dedupes by key with any page subscribing to the same key/cadence.

| Component | file:line | key | endpoint | cadence | enabled |
|---|---|---|---|---|---|
| `Chain` (topbar) | `src/components/Chain.tsx:65-69` | `` `admin/audit?limit=100#${adminKey}` `` | `gatewayAdminRequest("/admin/audit?limit=100", adminKey)` | `5000` ms | `Boolean(adminKey)` |
| `Sidebar` → `useRunsInFlight` | `src/components/Sidebar.tsx:39-43` | `` `admin/usage#${adminKey}` `` | `gatewayAdminRequest("/admin/usage", adminKey)` | `5000` ms | `Boolean(adminKey)` |

Both note they share the poll with Overview's identical key+cadence (`Chain.tsx:63-64`, `Sidebar.tsx:33-37`). Linger constants: `Chain.tsx:12 const LINGER_MS = 6000;`, `Sidebar.tsx:29 const LIVE_LINGER_MS = 8000;`.

Additional shell-wide timers (not endpoint polls):
- `useConnectionState()` at `src/App.tsx:148` → `src/hooks/useLiveResource.ts:76-79`: `useNowTick(1000)` then `registry.connectionState()` — a 1 s re-render tick on every route.
- Every `useLiveResource` also runs `useNowTick(1000)` for status derivation (`useLiveResource.ts:55`).
- One-shot on mount only: `apiFetch<{enabled:boolean}>(oidcStatusRequest())` at `App.tsx:218`; `whoamiRequest(key)` inside `refreshIdentity` at `App.tsx:192`.
- `installVisibilityPause(defaultRegistry)` (`App.tsx:227` → `src/lib/live/registry.ts:169-174`) pauses/resumes all polls on `document.hidden`.
- `DEFAULT_CADENCE = 4000` (`useLiveResource.ts:9`) when a caller omits one — both shell pollers pass `5000` explicitly.

### Checklist to add a route

1. Add the icon name to the `IconName` union **and** a matching entry in `GLYPHS`, `src/ui/icons.tsx:22-50` / `:53`.
2. Create `src/pages/<Name>.tsx` exporting a **named** component of type `(props: PageProps) => React.JSX.Element`.
3. Import it in `src/App.tsx:32-43` and add an entry to `ROUTES` (`App.tsx:62-99`), optionally with `visible: (r) => can(r, "<action>")` using an `Action` from `src/lib/rbac.ts:17+`.
4. Nothing else. Sidebar (`visibleRoutes`), CommandPalette (`commands`), and the page wrapper all read from `ROUTES` automatically.

---

## UI primitives & CSS tokens

### The barrel — `src/ui/index.ts:1-57`

Line 1 is `import "./ui.css";` — **importing anything from `"../ui"` pulls in `ui.css` as a side effect.**

Exported values: `Button`, `Card`, `Panel`, `PanelHead`, `Stat`, `Table`, `Tbody`, `Tr`, `Badge`, `Input`, `Select`, `Textarea`, `Tabs`, `Modal`, `ToastProvider`, `useToast`, `Skeleton`, `EmptyState`, `useCountUp`, `easeOutCubic`, `formatCount`, `countUpValue`, plus every motion constant (`DUR_FAST`, `DUR_MED`, `DUR_PAGE`, `EASE`, `STAGGER`, `STAGGER_MAX_ITEMS`, `transition`, `transitionFast`, `fadeRise`, `fadeRiseReduced`, `fade`, `staggerContainer`, `staggerItem`, `staggerItemReduced`, `modalBackdrop`, `modalPanel`, `modalPanelReduced`, `toastItem`, `toastItemReduced`, `pageTransition`) — re-exported at `src/ui/index.ts:36-57`.

Exported types: `ButtonProps`, `CardProps`, `PanelHeadProps`, `StatProps`, `TbodyProps`, `TrProps`, `BadgeProps`, `BadgeVariant`, `InputProps`, `SelectProps`, `TextareaProps`, `TabsProps`, `TabItem`, `ModalProps`, `ToastApi`, `ToastOptions`, `ToastVariant`, `SkeletonProps`, `EmptyStateProps`, `UseCountUpOptions`, `CountFormatOptions`.

**`icons.tsx` is NOT in the barrel** — import directly: `import { Icon } from "../ui/icons";` (`src/pages/Operators.tsx:35`), `import { BrandMark, Icon } from "../ui/icons";` (`src/components/Sidebar.tsx:10`), `import { StateIcon } from "../ui/icons";` (`src/components/Chain.tsx:8`).

**`common.tsx` is a separate module** — `import { ErrorNotice, PageHead, errorMessage, useLoad } from "../components/common";` (`src/pages/Documents.tsx:3`).

### `Button` — `src/ui/Button.tsx:5-45`

```ts
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
```

Emits `<button className="btn [primary] [danger] [small] [icon-only] {className}">` (built at `Button.tsx:29-38`) with an optional leading `<Icon name={icon} size={small ? 12 : 13} className="btn-icon" />` (`Button.tsx:41`). `variant="ghost"` adds **no** class — ghost is the bare `.btn`.

```tsx
<Button variant="primary" icon="plus" onClick={() => void add()} disabled={adding}>
  {adding ? "Ingesting…" : "Ingest document"}
</Button>
```
(`src/pages/Documents.tsx:120`)

```tsx
<Button small iconOnly icon="close" aria-label="Dismiss" onClick={() => setCreated(null)} />
```
(`src/pages/Users.tsx:147`)

### `Card` — `src/ui/Card.tsx:5-32`

```ts
export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
  /** Set false to render without the fade-rise entrance. */
  animate?: boolean;
}
```

Emits `<motion.div className={`card ${className}`.trim()}>` with `variants={reduced ? fadeRiseReduced : fadeRise} initial="hidden" animate="show"` (`Card.tsx:22-27`). With `animate={false}` it emits a plain `<div className="card …">` (`Card.tsx:16`).

```tsx
<Card key={i}>
  <Skeleton width={72} height={11} />
  <div style={{ marginTop: 10 }}>
    <Skeleton width={120} height={22} />
  </div>
</Card>
```
(`src/pages/Overview.tsx:128-133`)

### `Panel` — `src/ui/Card.tsx:35-55`

Same `CardProps` interface as `Card`. Emits `<motion.div className={`panel ${className}`.trim()}>` with identical fadeRise variants (`Card.tsx:45-50`); `animate={false}` → plain `<div className="panel …">`.

### `PanelHead` — `src/ui/Card.tsx:57-70`

```ts
export interface PanelHeadProps {
  title: ReactNode;
  /** Right-aligned slot (actions, badges, filters). */
  actions?: ReactNode;
}
```

Emits exactly:

```tsx
<div className="panel-head">
  <h2>{title}</h2>
  {actions}
</div>
```

No wrapper around `actions` — it renders raw as the second flex child. `title` accepts a node (`src/pages/Improve.tsx:296-303` passes `<span className="head-group">Active prompt<Badge …/></span>`).

**`PanelHead` does NOT render a body. There is NO `PanelBody` component — DOES NOT EXIST.** Panel body content is hand-written as `<div className="panel-body">…</div>` (`src/pages/Documents.tsx:101`, `src/pages/Users.tsx:155`, `src/pages/Improve.tsx:211`, `:268`, `:319`). `Table` supplies its own padding and is placed as a direct `Panel` child with no `panel-body` wrapper (`src/pages/Secrets.tsx:76`, `src/pages/Improve.tsx:226`).

### `Stat` — `src/ui/Stat.tsx:5-31`

```ts
export interface StatProps {
  label: string;
  value: number;
  decimals?: number;
  prefix?: string;
  suffix?: string;
  /** Stagger delay in ms (e.g. index * 60). */
  delay?: number;
  /** Optional sparkline slot rendered under the value. */
  spark?: ReactNode;
}
```

Emits a `<Card>` containing `<div className="label">`, then `<div className="ui-stat-row">` wrapping `<div className="value">{display}</div>` and (if `spark`) `<div className="ui-stat-spark">{spark}</div>` (`Stat.tsx:21-30`). Value comes from `useCountUp(value, { decimals, prefix, suffix, delay })`.

```tsx
<Stat label="Tokens" value={totals.tokens} delay={60} />
<Stat label="Spend" value={totals.spend} decimals={2} prefix="$" delay={120} />
<Stat label="Active keys" value={usage.length} delay={180} />
```
(`src/pages/Overview.tsx:150-152`)

### `Table` / `Tbody` / `Tr` — `src/ui/Table.tsx`

```ts
export function Table({ children, className = "", ...rest }: TableHTMLAttributes<HTMLTableElement>)  // :19

export interface TbodyProps {                      // :29-33
  children: ReactNode;
  /** Replay the entrance stagger when this value changes. */
  staggerKey?: unknown;
}

export interface TrProps extends HTMLAttributes<HTMLTableRowElement> {   // :52-56
  children: ReactNode;
  /** Set false for rows that should not participate in the stagger. */
  animate?: boolean;
}
```

- `Table` emits `<div className="table-wrap"><table className={className} …>{children}</table></div>` (`Table.tsx:20-26`). The wrapper class is fixed; `className` lands on the inner `<table>`.
- `Tbody` emits `<motion.tbody key={String(staggerKey ?? "static")} variants={staggerContainer} initial="hidden" animate="show">` (`Table.tsx:41-48`); under `useReducedMotion()` it emits a bare `<tbody>` (`Table.tsx:37-39`).
- `Tr` emits `<motion.tr className={className} variants={staggerItem} …>` (`Table.tsx:68`); with `animate={false}` **or** reduced motion → plain `<tr>` (`Table.tsx:60-66`).
- `<thead>`, `<tr>`, `<th>`, `<td>` are **plain HTML**, styled globally in `styles.css:370-414`.
- Re-exports the types `TdHTMLAttributes`, `ThHTMLAttributes` (`Table.tsx:75`).

Full pattern with skeleton + empty row — `src/pages/Secrets.tsx:76-120`:

```tsx
<Table>
  <thead>
    <tr>
      <th>Name</th>
      <th>Present</th>
      <th>Source</th>
    </tr>
  </thead>
  <Tbody staggerKey={secrets.length}>
    …
    {secrets.map((s) => (
      <Tr key={s.name}>
        <td className="mono">{s.name}</td>
        …
      </Tr>
    ))}
    {secrets.length === 0 && !loading && (
      <Tr animate={false}>
        <td colSpan={3} className="empty">
          No secrets reported.
        </td>
      </Tr>
    )}
  </Tbody>
</Table>
```

Improve uses a composite stagger key: `` staggerKey={runRows.length === 0 ? "empty" : `${runRows.length}-${runRows[0].id}`} `` (`src/pages/Improve.tsx:236`).

### `Badge` — `src/ui/Badge.tsx:4-25`

```ts
export type BadgeVariant =
  | "chat"
  | "embeddings"
  | "guardrail_flag"
  | "guardrail_block"
  | "pass"
  | "fail"
  | "passed_evals"
  | "failed_evals"
  | "approved"
  | "denied"
  | "inactive";

export interface BadgeProps {
  variant?: BadgeVariant;
  children: ReactNode;
}
```

Emits exactly one node (`Badge.tsx:24`):

```tsx
<span className={variant ? `badge ${variant}` : "badge"}>{children}</span>
```

No wrapper, no icon, **no `className` passthrough** (`className` is not in `BadgeProps`).

### `Input` / `Select` / `Textarea` — `src/ui/Field.tsx`

```ts
export interface InputProps extends InputHTMLAttributes<HTMLInputElement> { label?: ReactNode; }        // :9-11
export interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> { label?: ReactNode; }     // :24-26
export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> { label?: ReactNode; } // :43-45
```

Identical behaviour across all three: with **no** `label` prop the bare element is returned; with a label it wraps in

```tsx
<label className="field" htmlFor={id}>
  <span>{label}</span>
  {input}
</label>
```

(`Field.tsx:16-21`, `:34-40`, `:49-55`). `htmlFor={id}` is only meaningful if you also pass `id`.

### `Tabs` — `src/ui/Tabs.tsx:4-46`

```ts
export interface TabItem {
  id: string;
  label: string;
}

export interface TabsProps {
  tabs: TabItem[];
  active: string;
  onChange: (id: string) => void;
  /** layoutId namespace — must be unique per Tabs instance on the page. */
  id?: string;   // default "tabs"
}
```

Emits `<div className="ui-tabs" role="tablist">` → per tab `<button role="tab" aria-selected={isActive} className={`ui-tab${isActive ? " active" : ""}`} type="button">`, and for the active one a `<motion.span className="ui-tab-pill" layoutId={`${id}-pill`} transition={transitionFast} />` plus `<span className="ui-tab-label">{t.label}</span>` (`Tabs.tsx:21-43`). Under reduced motion `layoutId={undefined}` (`Tabs.tsx:36`).

**Real usage example: DOES NOT EXIST** — `grep -rn "Tabs" --include="*.tsx"` hits only `src/ui/Tabs.tsx`. Exported but unused by any page.

### `Modal` — `src/ui/Modal.tsx:6-64`

```ts
export interface ModalProps {
  open: boolean;
  onClose: () => void;
  title?: ReactNode;
  children: ReactNode;
  /** Right-aligned action row (usually Buttons). */
  actions?: ReactNode;
  width?: number;   // default 440
}
```

`createPortal(..., document.body)`. Emits `<motion.div className="modal-backdrop">` (closes on mousedown when `e.target === e.currentTarget`, `Modal.tsx:41-43`) → `<motion.div className="modal" role="dialog" aria-modal="true" style={{ width }}>` → optional `<h2>{title}</h2>`, `{children}`, optional `<div className="modal-actions">{actions}</div>` (`Modal.tsx:55-57`). ESC closes via a `window` keydown listener (`Modal.tsx:23-30`). Only usage: `src/components/SettingsModal.tsx:55-67`.

### `ToastProvider` / `useToast` — `src/ui/Toast.tsx`

```ts
export type ToastVariant = "default" | "error" | "success";              // :14

export interface ToastOptions {                                          // :16-20
  variant?: ToastVariant;
  /** Auto-dismiss delay. Default 4000ms; 0 = sticky. */
  duration?: number;
}

export interface ToastApi {                                              // :29-34
  toast: (message: string, opts?: ToastOptions) => void;
  success: (message: string, opts?: Omit<ToastOptions, "variant">) => void;
  error: (message: string, opts?: Omit<ToastOptions, "variant">) => void;
  dismiss: (id: number) => void;
}

export function ToastProvider({ children }: { children: ReactNode })     // :45
export function useToast(): ToastApi                                     // :39 — throws "useToast must be used within <ToastProvider>"
```

Portals `<div className="ui-toast-stack" role="status" aria-live="polite">` into `document.body`; each toast is `<motion.div className={`ui-toast ui-toast-${t.variant}`} onClick={() => dismiss(t.id)}>` containing `<span className="ui-toast-dot" aria-hidden />` + `<span>{t.message}</span>` (`Toast.tsx:83-99`). Stack capped at 5 (`prev.slice(-4)`, `Toast.tsx:61`). Mounted once at `src/App.tsx:255`. Consumed as `const toast = useToast();` (`src/pages/Users.tsx:22`) then `toast.error(errorMessage(err));` (`src/pages/Users.tsx:73`).

### `Skeleton` — `src/ui/Skeleton.tsx:3-32`

```ts
export interface SkeletonProps {
  width?: number | string;
  height?: number | string;   // default 14
  /** Render as a circle (avatars, dots). */
  circle?: boolean;
  /** Number of stacked text lines (overrides width/height). */
  lines?: number;
  style?: CSSProperties;
}
```

Single: `<span className="ui-skeleton" style={{ ...size, ...style }} />` (`:31`). Multi-line (`lines > 1`): `<span className="ui-skeleton-lines">` wrapping N `<span className="ui-skeleton">` where the last is `width: "60%"` (`:17-25`).

### `EmptyState` — `src/ui/Skeleton.tsx:34-50`

```ts
export interface EmptyStateProps {
  title: ReactNode;
  description?: ReactNode;
  /** Optional call-to-action (usually a Button). */
  action?: ReactNode;
}
```

Emits `<div className="ui-empty">` → `<div className="ui-empty-title">`, optional `<div className="ui-empty-desc">`, optional `<div className="ui-empty-action">`. `description` accepts JSX (Improve relies on this). Documents places one inside a `<td colSpan>` (`src/pages/Documents.tsx:160-163`).

### `useCountUp` + helpers — `src/ui/useCountUp.ts`

```ts
export function easeOutCubic(t: number): number                                        // :4
export interface CountFormatOptions { decimals?: number; prefix?: string; suffix?: string; }  // :8-12
export function formatCount(value: number, opts: CountFormatOptions = {}): string      // :15
export function countUpValue(target: number, elapsed: number, duration: number): number // :31
export interface UseCountUpOptions extends CountFormatOptions {                         // :45-50
  /** Total animation time. Default 700ms (per approved design card). */
  duration?: number;
  /** Start delay, e.g. index * 60 for staggered stat cards. */
  delay?: number;
}
export function useCountUp(target: number, opts: UseCountUpOptions = {}): string        // :57
```

Formats via `toLocaleString("en-US", …)` (`:19`). Under `prefers-reduced-motion` the final value renders immediately (`:66-69`). Reduced-motion is detected by a manual `matchMedia` helper at `:36-42` (the only place framer-motion is not used for this).

### `src/components/common.tsx` — exports

```ts
export function PageHead({ title, subtitle }: { title: string; subtitle?: string })   // :6
export function ErrorNotice({ error }: { error: string | null })                      // :16
export function NeedsKey({ openSettings }: { openSettings: () => void })              // :22
export function ForbiddenNotice({ message }: { message?: string })                    // :41
export function errorMessage(err: unknown): string                                    // :50
export function useLoad<T>(loader: () => Promise<T>, deps: unknown[])                 // :63
export function CopyButton({ text }: { text: string })                                // :93
```

`PageHead` (`common.tsx:6-13`) emits exactly:

```tsx
<header className="page-head">
  <h1>{title}</h1>
  {subtitle && <p>{subtitle}</p>}
</header>
```

**`title` is `string`, not ReactNode. `subtitle` is `string` only. There is no `actions` slot — DOES NOT EXIST.**

`ErrorNotice` returns `null` when `error` is falsy, else `<div className="notice error">{error}</div>` (`common.tsx:17-18`).

`NeedsKey` emits `<div className="notice warn">` with literal text `"No admin key configured. "`, an `<a href="#settings">Open settings</a>` (preventDefault + `openSettings()`), and `" and paste the gateway admin key to use this page."` (`common.tsx:23-37`).

`ForbiddenNotice` emits `<div className="notice warn">` with default copy `"You don't have permission to perform this action. Sign in with a higher-privileged token in Settings."` (`common.tsx:42-47`).

`errorMessage` maps `ApiError` statuses to exact strings (`common.tsx:51-56`):
- 401 → `"Unauthorized — set your credentials in Settings."`
- 403 → `"You don't have permission to perform this action."`
- 429 → `` `Rate limited — retry in ${formatRetryAfter(err.retryAfter)}.` ``
- otherwise → `` `${err.message} (${err.type}, HTTP ${err.status})` ``

`useLoad` returns `{ data, error, loading, reload }` where `data: T | null`, `error: string | null`, `loading: boolean`, `reload: () => void` (`common.tsx:89`). Deps are spread with an internal `tick`: `[...deps, tick]` (`common.tsx:87`).

`CopyButton` renders `<Button small icon="copy">` whose label flips to `"Copied"` for 1500ms (`common.tsx:95-108`).

`Freshness` (separate module) — `<Freshness updatedAt={objectives.updatedAt} />` (`src/pages/Multiverse.tsx:193`); signature `{ updatedAt: number | null; className?: string }` (`src/components/Freshness.tsx:8`); renders `null` until first load.

`useLiveResource<T>(key: string, fetcher: () => Promise<T>, opts: { cadence?: number; enabled?: boolean; registry?: Registry } = {})` returns `status`, `updatedAt`, `transport`, `reload` among others (`src/hooks/useLiveResource.ts:28-32`, interface fields `:23-25`). **The `key` MUST encode the auth token** — comment at `src/hooks/useLiveResource.ts:37-39`; Multiverse does `` `council/objectives#${adminKey}` `` (`src/pages/Multiverse.tsx:55`).

### Theme: dark only

**There is NO light theme. DOES NOT EXIST.** Exactly one `:root` block (`styles.css:17-74`), no `@media (prefers-color-scheme: …)`, no `[data-theme]`, no theme toggle anywhere in `src/`. `index.html:6` hard-declares `<meta name="color-scheme" content="dark" />` and `index.html:9-13` inlines `html { background: #121517; }` to pre-paint. Every token has exactly one value.

### Complete token list — `styles.css:17-74`

| Token | Value | Line |
|---|---|---|
| `--bg` | `#121517` | 19 |
| `--raised` | `#191d20` | 20 |
| `--raised-2` | `#20252a` | 21 |
| `--inset` | `#0d0f11` | 22 |
| `--bg-raised` | `var(--raised)` (legacy alias) | 25 |
| `--bg-inset` | `var(--inset)` (legacy alias) | 26 |
| `--border` | `rgba(255, 255, 255, 0.07)` | 29 |
| `--border-strong` | `rgba(255, 255, 255, 0.13)` | 30 |
| `--text` | `#e6e9eb` | 33 |
| `--text-dim` | `#98a1a8` | 34 |
| `--text-faint` | `#7e878e` | 37 |
| `--live` | `#5ad1c4` /* in flight right now */ | 41 |
| `--ok` | `#6cc48f` /* completed / allowed */ | 42 |
| `--hold` | `#e3a851` /* held, awaiting a human */ | 43 |
| `--deny` | `#e2685f` /* denied / failed / over budget */ | 44 |
| `--green` | `var(--ok)` (legacy alias) | 47 |
| `--amber` | `var(--hold)` (legacy alias) | 48 |
| `--red` | `var(--deny)` (legacy alias) | 49 |
| `--accent` | `var(--text)` — **not a brand color; resolves to ink** | 53 |
| `--accent-dim` | `rgba(255, 255, 255, 0.07)` | 54 |
| `--radius-sm` | `3px` | 57 |
| `--radius-md` | `4px` | 58 |
| `--radius-lg` | `6px` | 59 |
| `--dur-fast` | `110ms` | 62 |
| `--dur-med` | `180ms` | 63 |
| `--dur-slow` | `420ms` | 64 |
| `--ease` | `cubic-bezier(0.2, 0.8, 0.2, 1)` | 65 |
| `--mono` | `"IBM Plex Mono", ui-monospace, "Cascadia Mono", Menlo, Consolas, monospace` | 68 |
| `--sans` | `"Archivo", ui-sans-serif, system-ui, "Segoe UI", Helvetica, Arial, sans-serif` | 69 |
| `--eyebrow-size` | `10px` | 72 |
| `--eyebrow-track` | `0.16em` | 73 |

Two further custom properties exist but are **page-local, set inline by JS**: `--w` consumed by `src/pages/Keys.css:15` and `:22` (meter fill width). No other CSS file in `src/` declares any custom property (verified by grep).

The design law, `styles.css:7-12`: *"chroma is reserved for machine state… Do not add a brand color; it would make the signals lie."*

### Exact global rules you will need

`code, pre, .mono` — `styles.css:100-106`:

```css
code,
pre,
.mono {
  font-family: var(--mono);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}
```

That is the **entire** global `code`/`pre` treatment — no background, no padding, no border, no `white-space`, no overflow handling.

`.panel` — `styles.css:334-340`:

```css
.panel {
  background: var(--raised);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  overflow: hidden;
  margin-bottom: 24px;
}
```

`.panel-head` — `styles.css:342-349`, plus its `h2` at `:353-360`:

```css
.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 18px;
  border-bottom: 1px solid var(--border);
}

.panel-head h2 {
  font-family: var(--mono);
  font-size: var(--eyebrow-size);
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: var(--eyebrow-track);
  color: var(--text-dim);
}
```

`.panel-body` — `styles.css:362-364`:

```css
.panel-body {
  padding: 18px;
}
```

`.notice` — `styles.css:602-618`:

```css
.notice {
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-md);
  padding: 12px 16px;
  margin-bottom: 20px;
  color: var(--text-dim);
}

.notice.error {
  border-color: rgba(216, 96, 96, 0.5);
  color: var(--red);
}

.notice.warn {
  border-color: rgba(210, 162, 74, 0.5);
  color: var(--amber);
}
```

`.prompt-text` — `styles.css:774-783`:

```css
.prompt-text {
  background: var(--inset);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  padding: 12px 14px;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 320px;
  overflow-y: auto;
}
```

Modifiers: `.proposal .prompt-text { margin-top: 12px; }` (`styles.css:841-843`, legacy — `.proposal` is not used by `Improve.tsx`) and `.improve-proposal .prompt-text { margin-top: 12px; }` (`src/pages/Improve.css:89-91`).

`.card` — `styles.css:306-311` — **has no `overflow` rule** (unlike `.panel`).

Other utility classes available to any page: `.num` (`td.num, th.num { text-align: right }`, `styles.css:407-410`), `td.dim` (`styles.css:412-413`), `.status-ok` / `.status-err` (`styles.css:594-600`), `.empty` (`styles.css:982-986`), `.eyebrow` (`styles.css:110`), `.head-group` (`styles.css:768-772`), `.freshness` (`styles.css:1007-1011`), `.secret-reveal code` (`styles.css:637-645`), `.mono` (31 uses across `src/`, the standard way to force the mono face on a `<td>`/`<span>`).

### Global reduced-motion cap — `styles.css:1108-1123`, verbatim

```css
/* ---- Reduced motion: all non-essential animation off ---- */

@media (prefers-reduced-motion: reduce) {
  :root {
    --dur-fast: 0ms;
    --dur-med: 0ms;
  }

  *,
  *::before,
  *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }
}
```

Consequence: any CSS transition/animation you write is already neutralised globally; you only need extra gating for JS-driven (framer-motion) animation, which components do via `useReducedMotion()`. **`--dur-slow` is NOT zeroed. `animation-delay` is NOT reset. `scroll-behavior` is NOT reset.** Belt-and-braces local blocks exist at `src/ui/ui.css:190-194`, `src/pages/Audit.css:18-22`, `src/pages/Improve.css:101-105`, `src/pages/Keys.css:19-24`, `src/pages/Playground.css:150-160`, `src/components/Chain.css:180-184`.

### Shared code-block styling — what actually exists

**There is NO neutrally-named shared code-block component or `.code-block` / `.codeblock` class — DOES NOT EXIST.** But it is *not* true that every page rolls its own: three reusable block rules live in `styles.css`, each scoped by ancestor selector or prompt-specific name.

| Class | Where defined | Where used | What it does |
|---|---|---|---|
| `.prompt-text` | `styles.css:774-783` (global, under the `/* ---- Improve page ---- */` heading at `:766`) | `src/pages/Improve.tsx:320`, `:481` — both `<pre className="prompt-text mono">` | inset bg, hairline, `pre-wrap`, `max-height: 320px`, scroll — the only container-safe block style |
| `.event pre` | `styles.css:712-716` | `src/pages/Playground.tsx:249`, `:267` — **bare `<pre>`, no class**; `:307` `<pre className="muted">` | `white-space: pre-wrap; word-break: break-word; color: var(--text);` |
| `.secret-reveal code` | `styles.css:637-645` | `src/pages/Keys.tsx:115` `<code>{created.key}</code>`, `src/pages/Users.tsx:142` `<code>{created.token}</code>` — **bare `<code>`, no page-local CSS** | `background: var(--bg)`, `border: 1px solid var(--border-strong)`, `border-radius: var(--radius-sm)`, `padding: 8px 12px`, `word-break: break-all`, `white-space: normal`, `flex: 1` |
| `.pg-tool-pre` | `src/pages/Playground.css:142-148` | `src/pages/Playground.tsx:67-68` | `padding: 4px 12px 10px; color: var(--text-dim); font-size: 12.5px; white-space: pre-wrap; word-break: break-word;` — **no background, no border**. The only genuinely page-local code-block class. |
| `.pg-tool-summary` | `src/pages/Playground.css:136-142` | `src/pages/Playground.tsx:55`, on a `<code>` | single-line ellipsis for collapsed JSON |
| `.mv-answer` | `src/pages/Multiverse.css:139-144` | Multiverse | `white-space: pre-wrap` on a div (not `<pre>`) |
| `.op-run-output` | `src/pages/Operators.css:105-110` | Operators | `white-space: pre-wrap` on a div |

A bare `<pre>` outside `.event` is **not** unstyled — it inherits `styles.css:100-106` (mono / 12px / tabular-nums) and the reset `* { box-sizing: border-box; margin: 0; padding: 0; }` (`styles.css:76-79`). But it gets **no** `white-space`, `word-break` or overflow handling, so UA `white-space: pre` applies and a long line overflows. Inside `.panel` (`overflow: hidden`) that is **silently clipped — no scrollbar, no visible overflow**; inside `.card` (no overflow rule) it genuinely overflows and can force page-level horizontal scroll. Reuse `.prompt-text`, or promote a neutrally-named shared class.

### CSS file organisation & import convention

**Two-tier, no CSS modules, no Tailwind, no CSS-in-JS.** Plain global stylesheets imported by the component that owns them; Vite injects them.

Tier 1 — global, imported once:
- `src/main.tsx:4` — `import "./styles.css";` (1123 lines: tokens, reset, layout, `.card`/`.panel`/`table`/forms/`.btn`/`.badge`/`.notice`/modal/cmdk/toolbar/pager, reduced-motion cap).
- `src/styles.css:15` — `@import "./fonts/fonts.css";` (self-hosted `@font-face` for Archivo + IBM Plex Mono; `src/fonts/fonts.css:1-4` explicitly forbids CDN fonts).
- `src/ui/index.ts:1` — `import "./ui.css";` (194 lines, only primitive-specific `.ui-*` classes plus `.btn-icon` / `.btn.icon-only`).

Tier 2 — one CSS file per page/component, colocated, imported **last** in the `.tsx` import block as a bare side-effect import. Complete list of every CSS import in the app:

```
main.tsx:4                  import "./styles.css";
ui/index.ts:1               import "./ui.css";
charts/Sparkline.tsx:1      import "./charts.css";
charts/UsageChart.tsx:2     import "./charts.css";
charts/SpendBreakdown.tsx:1 import "./charts.css";
components/Chain.tsx:9      import "./Chain.css";
components/Sidebar.tsx:11   import "./Sidebar.css";
pages/Improve.tsx:45        import "./Improve.css";
pages/Documents.tsx:20      import "./Documents.css";
pages/Operators.tsx:36      import "./Operators.css";
pages/Multiverse.tsx:28     import "./Multiverse.css";
pages/Playground.tsx:18     import "./Playground.css";
pages/Keys.tsx:29           import "./Keys.css";
pages/Audit.tsx:17          import "./Audit.css";
pages/Overview.tsx:27       import "./Overview.css";
```

There are exactly 14 `*.css` files under `src/`: 8 in `src/pages/` (`Audit.css`, `Documents.css`, `Improve.css`, `Keys.css`, `Multiverse.css`, `Operators.css`, `Overview.css`, `Playground.css`), 2 in `src/components/` (`Chain.css`, `Sidebar.css`), plus `src/styles.css`, `src/ui/ui.css`, `src/charts/charts.css`, `src/fonts/fonts.css`. No `Users.css`, `Secrets.css`, `Orgs.css`, or `Provisioning.css` — those pages use only global + `ui` classes. **A page CSS file is optional.**

**Class naming convention: a short per-page prefix, kebab-case, on every class in the file.** These are global stylesheets, so the prefix is the only collision guard — **and keyframes must be prefixed too.**

| File | Prefix | Example classes |
|---|---|---|
| `src/pages/Overview.css` | `ov-` | `.ov-grid`, `.ov-feed`, `.ov-dot.live` |
| `src/pages/Playground.css` | `pg-` | `.pg-stream`, `.pg-tool-pre`, `.pg-chevron`, `.pg-live-dot` |
| `src/pages/Multiverse.css` | `mv-` | `.mv-controls`, `.mv-pad`, `.mv-answer`, `.mv-dissent`, `.mv-split` |
| `src/pages/Operators.css` | `op-` | `.op-form`, `.op-run-output`, `.op-eta`, `.op-live-dot` |
| `src/pages/Documents.css` | `doc-` | `.doc-drop`, `.doc-drop.dragging`, `.doc-progress`, `.doc-hint` |
| `src/pages/Improve.css` | `improve-` | `.improve-run-result`, `.improve-proposal`, `.improve-proposal-body.decided`, `.improve-prompt-reveal` |
| `src/pages/Keys.css` | `keys-` | `.keys-meter` |
| `src/pages/Audit.css` | `audit-` | `.audit-badge-flash` |
| `src/components/Chain.css` | `chain-` | `.chain-fill`, `.chain-stage`, `.chain-node`, `.chain-label` |

Every page CSS file opens with a one-line comment naming the page and pointing at the token source:

```css
/* ---- Playground step stream ---- */
/* Base .timeline/.event tokens live in styles.css. This file adds the stream
   presentation: staggered-entrance layout, live pulse dot, thinking shimmer,
   and the expandable tool-call treatment. Keyframes are pg- prefixed to avoid
   collisions with other pages. */
```
(`src/pages/Playground.css:1-5`)

```css
/* Overview page — activity feed, budget meters, layout. Tokens from styles.css. */
```
(`src/pages/Overview.css:1`)

```css
/* Multiverse — the council. Tokens and primitives from styles.css / ui.css. */
```
(`src/pages/Multiverse.css:1`)

```css
/* Improve page — proposal cards, run-result stream. Tokens from styles.css. */
```
(`src/pages/Improve.css:1`)

Page CSS may **extend** a global class rather than redefine it — `src/pages/Keys.css:8-11` layers an animation onto `.meter`'s child via `.keys-meter > div`; `src/pages/Playground.css:12` re-opens `.event .event-tag`. The stated rule (`Keys.css:2-5`): base styling stays in `styles.css`, the page file adds only page-specific behaviour.

App shell classes come from `styles.css` and are applied in `src/App.tsx`: `.shell` (`:256`), `.skip-link` (`:258`), `.main#main-content` (`:269`), `.topbar` + `.topbar-legend` + `.topbar-status` (`:273-280`), `.page` (`:284`, `:289`).

---

## Icons

### Location & export surface

- `src/ui/icons.tsx` — 456 lines, the whole icon system. `find src -iname "*icon*"` returns only this file.
- **NOT re-exported from the UI barrel.** `src/ui/index.ts` exports Button, Card, Stat, Table, Badge, Field, Tabs, Modal, Toast, Skeleton, useCountUp, motion — and nothing from `./icons`. Consumers import the module path directly, e.g. `src/components/Sidebar.tsx:9-10`:
  ```ts
  import type { IconName } from "../ui/icons";
  import { BrandMark, Icon } from "../ui/icons";
  ```
- Exported symbols, complete and exhaustive (verified by grepping every `export` token in the file): `IconName` (type, `:22`), `Icon` (`:340`), `BrandMark` (`:373`), `StateName` (type, `:396`), `StateIcon` (`:435`). The two other `export` hits — `:42 | "export"` and `:251 export: (` — are the icon *named* "export", not declarations.
- Only one import in the whole file — `src/ui/icons.tsx:20`: `import type { JSX } from "react";`

### `IconName` — `src/ui/icons.tsx:22-50`, verbatim, in source order

```ts
export type IconName =
  | "overview"
  | "keys"
  | "audit"
  | "playground"
  | "documents"
  | "improve"
  | "orgs"
  | "users"
  | "secrets"
  | "provisioning"
  | "settings"
  | "multiverse"
  | "operators"
  | "copy"
  | "refresh"
  | "run"
  | "pause"
  | "approve"
  | "deny"
  | "export"
  | "search"
  | "filter"
  | "sort"
  | "close"
  | "external-link"
  | "chevron"
  | "trash"
  | "plus";
```

28 members. `"external-link"` is the only hyphenated key and is quoted in the `GLYPHS` record (`:297`). It is a **type-only union — fully erased at runtime.**

`StateName` (`:396`) is separate: `export type StateName = "live" | "ok" | "hold" | "deny";` — **`"deny"` exists in BOTH unions with different artwork.**

### `GLYPHS` — module-private, no runtime name array

Exact declaration, `src/ui/icons.tsx:53`:

```ts
const GLYPHS: Record<IconName, JSX.Element> = {
```

No `export`. `STATE_GLYPHS` likewise, `:398`:

```ts
const STATE_GLYPHS: Record<StateName, JSX.Element> = {
```

`grep -rn "GLYPHS"` across the whole repo returns only `icons.tsx:53`, `:358` (`{GLYPHS[name]}`), `:398`, `:453`. There is no `export { GLYPHS }` anywhere.

**A runtime array of icon names DOES NOT EXIST** — no `ICON_NAMES`, no `Object.keys(GLYPHS)` export, no iteration helper anywhere in `src`. Anything needing to enumerate icons must build its own literal list, or add an export.

Because `GLYPHS` is `Record<IconName, JSX.Element>` — an exhaustive mapped type — **adding a member to `IconName` without adding a `GLYPHS` entry is a tsc error**, and `tsc` gates the build (`"build": "tsc && vite build"`). Symmetrically, any field typed `IconName` (e.g. the planned `DocSection.icon`) can only hold a real glyph name; a runtime test asserting that is strictly redundant with the type checker.

### `Icon` — props `:330-337`, component `:339-361`, verbatim

```ts
interface IconProps {
  name: IconName;
  /** Edge length in px. Nav uses 15; inline labels use 13. */
  size?: number;
  /** Accessible name. Omit when adjacent text already names the thing. */
  title?: string;
  className?: string;
}
```

`IconProps` is **not exported**.

```tsx
/** Render one glyph from the set. */
export function Icon({ name, size = 15, title, className }: IconProps) {
  return (
    <svg
      className={className}
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.25}
      strokeLinecap="square"
      strokeLinejoin="miter"
      role={title ? "img" : undefined}
      aria-hidden={title ? undefined : true}
      aria-label={title}
      focusable="false"
    >
      {title ? <title>{title}</title> : null}
      {GLYPHS[name]}
    </svg>
  );
}
```

The wrapper supplies `viewBox="0 0 16 16"`, `fill="none"`, `stroke="currentColor"`, `strokeWidth={1.25}`, `strokeLinecap="square"`, `strokeLinejoin="miter"`, plus a11y. Default `size = 15`. **Glyph entries must NOT set stroke, fill, width or viewBox themselves** — only geometry. The sole exceptions are the two `STATE_GLYPHS` entries that opt out with `fill="currentColor" stroke="none"` (`:400`, `:412`).

Consumers of the `IconName` type: `Button` (`src/ui/Button.tsx:10` `icon?: IconName`), `Sidebar` (`src/components/Sidebar.tsx:16` `icon: IconName`), `CommandPalette` (`src/components/CommandPalette.tsx:13` `icon?: IconName`), `App.tsx:56` (`icon: IconName`), `src/ui/Button.tsx:3` and `src/App.tsx:8` import the type.

### `BrandMark` — `src/ui/icons.tsx:373-393`, verbatim

```tsx
export function BrandMark({ size = 18 }: { size?: number }): JSX.Element {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.25}
      strokeLinecap="square"
      strokeLinejoin="miter"
      aria-hidden
      focusable="false"
    >
      <path d="M2.5 8h11" />
      <rect x="2.5" y="6.5" width="3" height="3" />
      <rect x="7" y="6.5" width="2" height="3" />
      <rect x="10.5" y="6.5" width="3" height="3" />
    </svg>
  );
}
```

Inline literal artwork (three nodes seated on a rule), **not** part of `GLYPHS`. No `title`, no `className`, no `role` — always `aria-hidden`, always decorative. Default `size = 18`. Only call site: `Sidebar.tsx:90` → `<BrandMark size={18} />`. It has **no animation**.

### `StateIcon` — props `:424-428`, component `:430-456`, verbatim

```tsx
interface StateIconProps {
  state: StateName;
  size?: number;
  title?: string;
}

/**
 * Status glyph. These are the only marks in the console allowed to carry color,
 * and they take it from the matching --live/--ok/--hold/--deny token via the
 * `.state-<name>` class, so a state can never be drawn in the wrong hue.
 */
export function StateIcon({ state, size = 13, title }: StateIconProps) {
  return (
    <svg
      className={`state-icon state-${state}`}
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.25}
      strokeLinecap="square"
      strokeLinejoin="miter"
      role={title ? "img" : undefined}
      aria-hidden={title ? undefined : true}
      aria-label={title}
      focusable="false"
    >
      {title ? <title>{title}</title> : null}
      {STATE_GLYPHS[state]}
    </svg>
  );
}
```

Differences from `Icon`: **no `className` prop** (className is computed and fixed as `` `state-icon state-${state}` ``), default `size = 13` (vs 15), pulls from `STATE_GLYPHS`.

The four state glyphs, `:398-422`, verbatim:

```tsx
const STATE_GLYPHS: Record<StateName, JSX.Element> = {
  // Filled — something is happening right now. The ring is animated by CSS.
  live: <circle cx="8" cy="8" r="3.5" fill="currentColor" stroke="none" />,
  // Closed ring with a tick: finished, and allowed.
  ok: (
    <>
      <circle cx="8" cy="8" r="5.5" />
      <path d="M5.5 8.2 7.2 10 10.5 6.2" />
    </>
  ),
  // Half-filled: the system did its part and stopped, waiting on a person.
  hold: (
    <>
      <circle cx="8" cy="8" r="5.5" />
      <path d="M8 2.5a5.5 5.5 0 0 1 0 11z" fill="currentColor" stroke="none" />
    </>
  ),
  // Barred: refused. A cross would read as "error"; this reads as "not permitted".
  deny: (
    <>
      <circle cx="8" cy="8" r="5.5" />
      <path d="M4.5 11.5 11.5 4.5" />
    </>
  ),
};
```

Colour/animation live in CSS, not in this file — `src/components/Chain.css:149-184`: `.state-icon.state-live { color: var(--live); }`, `.state-ok → var(--ok)`, `.state-hold → var(--hold)`, `.state-deny → var(--deny)`, plus `animation: state-live-pulse 1.8s var(--ease) infinite;` (opacity 0.5↔1) with a `prefers-reduced-motion` opt-out. Call sites: `App.tsx:277` `<StateIcon state={connGlyph.state} size={12} />`, `Chain.tsx:129` `<StateIcon state={glyph.state} title={glyph.label} size={12} />`.

### Three representative glyph entries, VERBATIM — copy this idiom

Simplest, a bare single element with no fragment, comment above (`:236-238`):

```tsx
  // Bare check. `StateIcon.ok` rings its check because that's a persisted
  // verdict; this is a momentary action, so no ring.
  approve: <path d="M3.5 8.5 6.5 11.5 12.5 4.5" />,
```

Mid complexity, fragment with mixed primitives (`:194-203`):

```tsx
  // Two overlapping squares. The back sheet is drawn only where the front one
  // doesn't already cover it — that partial outline is what reads as
  // "duplicate" rather than "two files".
  copy: (
    <>
      <rect x="5.5" y="5.5" width="7" height="7" />
      <path d="M9.5 5.5V3.5h-6v6h2" />
    </>
  ),
```

Most complex, five elements (`:113-122`):

```tsx
  // Hierarchy — one parent, two children. Orgs own keys and users.
  orgs: (
    <>
      <rect x="6" y="2.5" width="4" height="3" />
      <rect x="2.5" y="10.5" width="4" height="3" />
      <rect x="9.5" y="10.5" width="4" height="3" />
      <path d="M8 5.5v2.5" />
      <path d="M4.5 10.5V8h7v2.5" />
    </>
  ),
```

Idiom rules visible in all 28 entries: every entry is preceded by a prose `//` comment explaining the *reason* for the shape (often naming which other glyph it must not be confused with); multi-element glyphs use a bare fragment `<>…</>`; single-element glyphs are inline with no fragment (`approve` `:238`, `filter` `:271`, `chevron` `:308`); primitives are only `<path>`, `<rect>`, `<circle>` — **no `<g>`, no `<line>`, no `<polyline>`, no `<polygon>`, no transforms, no ids, no gradients**, and no per-element stroke/fill outside `STATE_GLYPHS`.

### House grid rules (inferred from actual path data + the file header)

- **viewBox is `0 0 16 16`** on all three wrappers — `:346` (`Icon`), `:379` (`BrandMark`), `:441` (`StateIcon`). No glyph declares its own viewBox.
- **Stroke width exactly `1.25`**, `strokeLinecap="square"`, `strokeLinejoin="miter"`, `fill="none"`, `stroke="currentColor"` — identical in all three wrappers (`:347-351`, `:381-385`, `:442-446`). Header at `:11`: "1.25 stroke, square caps, mitre joins — machined, never rounded".
- **Artwork band.** Header `:10`: "16x16 viewBox, artwork confined to 2..14 so glyphs optically align". In practice *centerlines* sit at **2.5 … 13.5**, which with a 1.25 stroke (0.625 half-width) paints ≈1.875…14.125. Evidence: `audit` `M3.5 2.5v11` ends at y=13.5 (`:77`); `documents` `M2.5 5.5v8h8` (`:100`); `overview` arc `M2.5 12.5a5.5 5.5 0 0 1 11 0` spans x 2.5→13.5 (`:58`); `secrets` `<rect x="3" y="7" width="10" height="6.5" />` bottom = 13.5 (`:139`); `keys` `M6.8 7.8 12.5 13.5` (`:68`); `settings` `M2.5 5.5h11` (`:160`); `BrandMark` `M2.5 8h11` (`:387`); `StateIcon` rings `r="5.5"` at cx/cy 8 → 2.5…13.5 (`:410`, `:417`).
  Documented exceptions: `deny` uses the full 3…13, comment calls it out — "Bare cross at full scale (3..13 — as wide as a glyph gets on this grid)" (`:240`); `close` is the same mark at 6…10 (`:287-291`); `multiverse` satellites at `cx="2.8" r="1.3"` / `cx="13.2"` reach 1.5 and 14.5 (`:172-175`); `improve` reaches y=2 at `M13.2 2v2.6h-2.6` (`:109`).
- **Snapping: whole or half units, `.0`/`.5` only**, per header `:12` ("geometry snaps to whole or half units; no arbitrary curves"). True for every coordinate in `orgs`, `settings`, `secrets`, `copy`, `run`, `pause`, `export`, `sort`, `close`, `chevron`, `trash`, `plus`, `filter`, `approve`, `deny`, `search`, `external-link`, `audit`, `playground`, `documents`, `operators`, `provisioning`, `BrandMark`. The **only** off-grid decimals in the file all sit where a diagonal meets a curve — the file names this tolerance at `:259-261` ("The handle's start point sits just off the ring's true tangent — the same tolerance `keys` and `users` already use where a diagonal meets a curve"):
  - `keys` `M6.8 7.8 12.5 13.5` (`:68`)
  - `improve` `M13 8a5 5 0 1 1-1.8-3.85` and `M13.2 2v2.6h-2.6` (`:108-109`)
  - `users` `M10.5 4.2a2.4 2.4 0 0 1 0 4.6`, `M11.2 10.4a3.5 3.5 0 0 1 2.3 3.1` (`:130-131`)
  - `multiverse` `cx="2.8"` / `cx="13.2"` / `r="1.3"` (`:172-175`)
  - `STATE_GLYPHS.ok` `M5.5 8.2 7.2 10 10.5 6.2` (`:405`)
- **Arc radii are whole or half**: `5.5` (`overview :58`, state rings), `5` (`improve :108`), `4.5` (`refresh :210-211`), `3.5` (`users :129`/`:131`, `search :264`, `state live :400`), `2.5` (`keys :67`, `users :128`, `secrets :140`), `1.75` (`settings :162-163`), `1.5` (`provisioning :149-151`), `2` (`multiverse :171`).
- **Corner-arrow / tick idiom**: arrowheads are always open L-brackets or chevrons drawn as 3-point paths, **never filled wedges** — `run` `M6 6 8 8 6 10` (`:223`), `export` `M5.5 6.5 8 9 10.5 6.5` (`:254`), `sort` `M10 10 11.5 11.5 13 10` (`:281`), `refresh` `M4.5 2v2.5h2.5` / `M11.5 14v-2.5H9` (`:212-213`). Stated at `:294-296`: "the arrowhead is an open bracket rather than a filled wedge, matching this set's line-only vocabulary".

### There is no book / manual / docs glyph

**A book, manual, guide, help, or reference-sheet glyph DOES NOT EXIST.** The 28-member union has no such member. The closest semantic neighbours are `documents` and `audit`.

`documents`, verbatim, `:95-102`:

```tsx
  // Sheets — two offset planes. Corpus, not a single file, so no page fold.
  documents: (
    <>
      <path d="M5.5 2.5h5l3 3v6h-8z" />
      <path d="M10.5 2.5v3h3" />
      <path d="M2.5 5.5v8h8" />
    </>
  ),
```

Geometry for anyone drawing a sibling: the front sheet occupies x 5.5…13.5, y 2.5…11.5 with the corner cut from (10.5, 2.5) to (13.5, 5.5); the back plane is an open L from (2.5, 5.5) down to (2.5, 13.5) and right to (10.5, 13.5) — offset exactly −3 in x and +3 in y from the front sheet's top-left. Caveat: the comment says "no page fold", but `M10.5 2.5v3h3` does render the fold's two edges, so **a new book-like glyph must differentiate on silhouette (e.g. a spine + two leaves) rather than on presence/absence of a corner fold.**

The nearest "ledger/reference" motif is `audit` (`:75-82`), a vertical spine at x=3.5 with three ragged horizontal entries. The file notes at `:365-368` that this spine motif is the shared house language (`audit`'s spine, `operators`' struck node, `BrandMark`'s nodes-on-a-rule) — a new book/manual glyph should be built from that vocabulary.

---

## Motion & dependencies

### `src/ui/motion.ts` — every export, verbatim (107 lines; only import is `import type { Variants, Transition } from "framer-motion";` at `:10`)

```ts
export const DUR_FAST = 0.12;                                    // :13
export const DUR_MED = 0.2;                                      // :15
export const DUR_PAGE = 0.16;                                    // :17
export const EASE: [number, number, number, number] = [0.2, 0.8, 0.2, 1];  // :19
export const STAGGER = 0.032;                                    // :22
export const STAGGER_MAX_ITEMS = 6;                              // :24

export const transition: Transition = { duration: DUR_MED, ease: EASE };       // :26
export const transitionFast: Transition = { duration: DUR_FAST, ease: EASE };  // :27

export const fadeRise: Variants = {                              // :30
  hidden: { opacity: 0, y: 6 },
  show: { opacity: 1, y: 0, transition },
  exit: { opacity: 0, y: 4, transition: transitionFast },
};

export const fade: Variants = {                                  // :37
  hidden: { opacity: 0 },
  show: { opacity: 1, transition },
  exit: { opacity: 0, transition: transitionFast },
};

export const staggerContainer: Variants = {                      // :44
  hidden: {},
  show: {
    transition: { staggerChildren: STAGGER, delayChildren: 0.02 },
  },
};

export const staggerItem: Variants = {                           // :52
  hidden: { opacity: 0, y: 4 },
  show: { opacity: 1, y: 0, transition: { duration: DUR_MED, ease: EASE } },
};

export const fadeRiseReduced: Variants = {                       // :58
  hidden: { opacity: 1, y: 0 },
  show: { opacity: 1, y: 0 },
  exit: { opacity: 1, y: 0 },
};

export const staggerItemReduced: Variants = {                    // :64
  hidden: { opacity: 1, y: 0 },
  show: { opacity: 1, y: 0 },
};

export const modalBackdrop: Variants = {                         // :70
  hidden: { opacity: 0 },
  show: { opacity: 1, transition: { duration: DUR_MED, ease: EASE } },
  exit: { opacity: 0, transition: { duration: DUR_FAST, ease: EASE } },
};

export const modalPanel: Variants = {                            // :76
  hidden: { opacity: 0, scale: 0.97, y: 8 },
  show: { opacity: 1, scale: 1, y: 0, transition: { duration: DUR_MED, ease: EASE } },
  exit: { opacity: 0, scale: 0.98, y: 4, transition: { duration: DUR_FAST, ease: EASE } },
};

export const modalPanelReduced: Variants = {                     // :82
  hidden: { opacity: 1, scale: 1, y: 0 },
  show: { opacity: 1, scale: 1, y: 0 },
  exit: { opacity: 1, scale: 1, y: 0 },
};

export const toastItem: Variants = {                             // :89
  hidden: { opacity: 0, x: 24 },
  show: { opacity: 1, x: 0, transition: { duration: DUR_MED, ease: EASE } },
  exit: { opacity: 0, x: 16, transition: { duration: DUR_FAST, ease: EASE } },
};

export const toastItemReduced: Variants = {                      // :95
  hidden: { opacity: 1, x: 0 },
  show: { opacity: 1, x: 0 },
  exit: { opacity: 0, transition: { duration: 0.01 } },
};

export const pageTransition: Variants = {                        // :102
  hidden: { opacity: 0, y: 6 },
  show: { opacity: 1, y: 0, transition: { duration: DUR_PAGE, ease: EASE } },
  exit: { opacity: 0, transition: { duration: DUR_FAST, ease: EASE } },
};
```

Variant state names are exactly `"hidden"`, `"show"`, `"exit"` everywhere. **There is no `useMotion` hook, no `MotionConfig` usage, no `spring` preset — DOES NOT EXIST.** Pages import motion constants from `"../ui"`; `ui/*.tsx` siblings import from `"./motion"` directly.

### EASE / duration constants and the JS↔CSS mismatch

- `EASE` = `[0.2, 0.8, 0.2, 1]`, typed `[number, number, number, number]` — `src/ui/motion.ts:19`.
- CSS counterpart `--ease: cubic-bezier(0.2, 0.8, 0.2, 1);` — `src/styles.css:65`. Same curve.
- JS durations (seconds): `DUR_FAST = 0.12`, `DUR_MED = 0.2`, `DUR_PAGE = 0.16`.
- CSS durations (ms) **do NOT match**: `--dur-fast: 110ms;` `--dur-med: 180ms;` (`src/styles.css:62-63`). The doc comments at `motion.ts:13` and `:15` claim 120ms/200ms — **the comments are stale/wrong.** Do not "fix" one to match the other without a decision; just know they diverge.
- Other hard-coded durations at real call sites: `0.6` s with `delay: 0.1 + i * 0.06` (`src/pages/Overview.tsx:245`); `0.55` s with an inline literal ease array `[0.2, 0.8, 0.2, 1]` (`src/components/Chain.tsx:115`); `useCountUp` default `duration = 700` ms, `delay = 0` (`src/ui/useCountUp.ts:57`).

### Reduced motion — every distinct pattern in the codebase

**Pattern A — swap to a `*Reduced` variants object (ternary on `variants`).**
```tsx
const reduced = useReducedMotion();                 // ui/Card.tsx:13
variants={reduced ? fadeRiseReduced : fadeRise}     // ui/Card.tsx:24  and  ui/Card.tsx:47
```
Also `src/ui/Modal.tsx:50`, `src/components/CommandPalette.tsx:88`, `src/ui/Toast.tsx:89` (`toastItemReduced : toastItem`), `src/components/LiveList.tsx:32` (`staggerItemReduced : staggerItem`), `src/pages/Keys.tsx:107`, `src/pages/Improve.tsx:332` (`const fadeVariants = reduced ? fadeRiseReduced : fadeRise;`).

**Pattern B — bail out to a plain DOM element entirely.**
```tsx
export function Tbody({ children, staggerKey }: TbodyProps) {
  const reduced = useReducedMotion();
  if (reduced) {
    return <tbody>{children}</tbody>;          // ui/Table.tsx:36-39
```
Also `src/ui/Table.tsx:60`; `src/App.tsx:284-286`; `src/components/Sidebar.tsx:97-99`; `src/components/Chain.tsx:107-109` (`<div className="chain-fill" style={{ transform: \`scaleX(${progress})\` }} />`).

**Pattern C — `initial={reduced ? false : {...}}`.**
```tsx
initial={reduced ? false : { width: 0 }}                    // pages/Overview.tsx:243
initial={reduced ? false : { height: 0, opacity: 0 }}       // pages/Playground.tsx:62
initial={reduced ? false : { opacity: 0 }}                  // pages/Playground.tsx:316
initial: reduced ? false : { opacity: 0, y: 6 },            // pages/Playground.tsx:99 (eventMotion)
```

**Pattern D — `initial`/`exit` swapped to a no-op object.**
```tsx
initial={reduced ? { opacity: 1 } : { opacity: 0, height: 0 }}   // pages/Improve.tsx:314, :476
exit={reduced ? { opacity: 1 } : { opacity: 0, height: 0 }}      // pages/Improve.tsx:316, :478
initial={reduced ? { opacity: 1 } : { opacity: 0, scale: 0.9 }}  // pages/Improve.tsx:425
exit={reduced ? { opacity: 1 } : { opacity: 0, scale: 0.9 }}     // pages/Improve.tsx:427
exit: reduced ? { opacity: 0 } : { opacity: 0, y: 4, transition: transitionFast },  // pages/Playground.tsx:101
exit={reduced ? { opacity: 0 } : { height: 0, opacity: 0 }}      // pages/Playground.tsx:64
```

**Pattern E — disable `layout` / `layoutId`.**
```tsx
<List className={className} layout={!reduced}>            // components/LiveList.tsx:26
layout={!reduced}                                         // components/LiveList.tsx:31, ui/Toast.tsx:90
layoutId={reduced ? undefined : `${id}-pill`}             // ui/Tabs.tsx:36
```

**Pattern F — manual `matchMedia` (the only place framer-motion is not used).**
```ts
function prefersReducedMotion(): boolean {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}                                                          // ui/useCountUp.ts:36-42
```
Consumed at `ui/useCountUp.ts:61` (initial state) and `:67` (effect early-return sets the final value immediately).

**Per-file CSS reduced-motion blocks — 12 total**, all `@media (prefers-reduced-motion: reduce)`: `src/components/Sidebar.css:78` (`.live { animation: none }`), `src/components/Chain.css:141` (`.chain[data-active] .chain-fill`), `src/components/Chain.css:180` (`.state-icon.state-live`), `src/pages/Operators.css:147` (`.op-live-dot`), `src/pages/Audit.css:18` (`.audit-badge-flash .badge`), `src/pages/Keys.css:19` (`.keys-meter > div { animation: none; width: var(--w); }`), `src/pages/Documents.css:57` (`.doc-drop.dragging`, `.doc-progress::before`), `src/charts/charts.css:121` (`.chart-line--draw`, `.chart-area--fade`, `.breakdown-fill`), `src/pages/Overview.css:192` (`.ov-dot.live`), `src/pages/Playground.css:150` (`.pg-live-dot`, `.pg-shimmer`, plus `transition: none` on `.pg-chevron`, `.pg-tool-toggle`), `src/pages/Improve.css:101` (`.improve-proposal-body { transition: none }`), `src/ui/ui.css:190` (`.ui-skeleton { animation: none }`).

### framer-motion APIs actually in use

| API | Real call site |
|---|---|
| `motion.div` | `src/ui/Modal.tsx:35`, `src/ui/Card.tsx:22`, `src/pages/Overview.tsx:241` |
| `motion.span` | `src/ui/Tabs.tsx:34`, `src/components/Sidebar.tsx:100`, `src/pages/Improve.tsx:422` |
| `motion.tbody` / `motion.tr` | `src/ui/Table.tsx:41`, `:68` |
| `motion.ul` / `motion.ol` / `motion.li` | `src/components/LiveList.tsx:24` (`const List = as === "ol" ? motion.ol : motion.ul;`), `:29`; `src/pages/Improve.tsx:361`, `:370` |
| `AnimatePresence` | `src/App.tsx:286` `mode="wait"`; `src/ui/Modal.tsx:33` (bare); `src/components/LiveList.tsx:27` `initial={false}`; `src/ui/Toast.tsx:85`; `src/pages/Improve.tsx:421` `mode="wait" initial={false}` |
| `layoutId` | `src/components/Sidebar.tsx` `layoutId="nav-pill"`; `src/ui/Tabs.tsx:36` `` layoutId={reduced ? undefined : `${id}-pill`} `` |
| `layout` (boolean) | `src/components/LiveList.tsx:26`, `:31`; `src/ui/Toast.tsx:90` |
| `variants` + `initial="hidden"` / `animate="show"` / `exit="exit"` | `src/ui/Modal.tsx:37-40`, `src/ui/Card.tsx:24-26`, `src/App.tsx:290-293` |
| `useReducedMotion` | 15 call sites — `src/App.tsx:242` (`const reducedMotion = …`), and `const reduced = …` at `src/components/Chain.tsx:62`, `src/components/CommandPalette.tsx:34`, `src/components/LiveList.tsx:23`, `src/components/Sidebar.tsx:84`, `src/pages/Improve.tsx:292,331,409`, `src/pages/Keys.tsx:47`, `src/pages/Overview.tsx:53`, `src/pages/Playground.tsx:42,83`, `src/ui/Card.tsx:13,36`, `src/ui/Modal.tsx:21`, `src/ui/Table.tsx:36,59`, `src/ui/Tabs.tsx:19`, `src/ui/Toast.tsx:46` |
| Object `animate` prop | `src/components/Chain.tsx:114` `animate={{ scaleX: progress }}`; `src/pages/Overview.tsx:244` `` animate={{ width: `${m.fraction * 100}%` }} ``; `src/pages/Improve.tsx:315` `animate={{ opacity: 1, height: "auto" }}` |
| `initial={false}` | `src/components/Chain.tsx:113` |
| Inline `transition` object | `src/components/Chain.tsx:115` `transition={{ duration: 0.55, ease: [0.2, 0.8, 0.2, 1] }}`; `src/pages/Overview.tsx:245` `transition={{ duration: 0.6, ease: EASE, delay: 0.1 + i * 0.06 }}`; `src/pages/Improve.tsx:317,479` `transition={{ duration: DUR_MED, ease: EASE }}` |
| Shared `transition` const as prop | `src/ui/Tabs.tsx:37` `transition={transitionFast}`; `src/components/Sidebar.tsx:104` `transition={transition}`; `src/pages/Playground.tsx:65,319` |
| Spread-props motion helper | `src/pages/Playground.tsx:99-103`: `const eventMotion = (i: number) => ({ initial, animate, exit, transition: { ...transition, delay: enterDelay(i) } })`, spread at `:247,257,265,272,306` as `{...eventMotion(i)}` |
| Manual stagger cap | `src/pages/Playground.tsx:92-97`: `return Math.min(fresh, STAGGER_MAX_ITEMS - 1) * STAGGER;` |
| Remount-to-replay-stagger key | `src/ui/Table.tsx:42` `key={String(staggerKey ?? "static")}` |

**NOT used anywhere — DOES NOT EXIST in `src`**: `whileHover`, `whileTap`, `whileInView`, `useAnimate`, `useMotionValue`, `useTransform`, `useScroll`, `useSpring`, `MotionConfig`, `LayoutGroup`, `Reorder`, `motion.path`, `motion.svg`, `motion.circle`, `drag`.

### `layoutId` is a global namespace

`grep -rn "LayoutGroup\|layoutGroup" console/src` returns **zero hits**. `AnimatePresence` does **not** create a layoutId namespace — it only consumes `LayoutGroupContext` read-only (`node_modules/framer-motion/dist/es/components/AnimatePresence/index.mjs:3`). Namespacing is opt-in via `LayoutGroup` only:

```js
function useLayoutId({ layoutId }) {
    const layoutGroupId = useContext(LayoutGroupContext).id;
    return layoutGroupId && layoutId !== undefined
        ? layoutGroupId + "-" + layoutId
        : layoutId;
}
```
(`node_modules/framer-motion/dist/es/motion/index.mjs:77-82`; default context is `createContext({})` at `context/LayoutGroupContext.mjs:4`, so `.id` is `undefined` → no prefix.)

The lookup table lives on the projection **root**, which is a module-level singleton document node — effectively document-global, spanning portals and unrelated subtrees (`motion-dom/dist/es/projection/node/create-projection-node.mjs:215,217,280,1254-1257`; `HTMLProjectionNode.mjs:4-19`). Two co-mounted nodes with the same layoutId join ONE `NodeStack`; the newly mounted one is promoted to lead and `resumeFrom`s the other's measured box, so **the pill visibly flies from one component to the other**, and the follower crossfades out (crossfade defaults to `true`, `create-projection-node.mjs:748`; opacity handling at `:1386-1400`).

Concrete collision vector: rendering `<Tabs id="nav" />` produces exactly `"nav-pill"` and collides with the Sidebar. The codebase already documents the hazard — `src/ui/Tabs.tsx:13`: `/** layoutId namespace — must be unique per Tabs instance on the page. */`. The collision only exists when motion is not reduced (both Sidebar and Tabs drop the layoutId under `prefers-reduced-motion`). Fix pattern: wrap in `<LayoutGroup id="...">` or pass a per-instance id string.

### `package.json` dependencies

```json
"dependencies": {
  "framer-motion": "^12.42.2",
  "react": "^19.1.0",
  "react-dom": "^19.1.0"
}
```

Installed framer-motion resolves to exactly **12.42.2** (`package-lock.json:1643`). Runtime deps are only those three. devDependencies (not shippable): `@types/react ^19.1.0`, `@types/react-dom ^19.1.0`, `@vitejs/plugin-react ^4.4.0`, `typescript ^5.8.0`, `vite ^6.3.0`, `vitest ^3.1.0`. Installed versions from `node_modules`: vitest **3.2.7**, vite **6.4.3**, typescript **5.9.3**.

### Existing SVG animation — the house technique

**EXISTS, and it is all CSS-driven.** No SMIL anywhere: `grep` for `<animate`, `animateTransform`, `animateMotion` returns **nothing** — those DO NOT EXIST. No `requestAnimationFrame`-driven SVG either.

**Stroke draw-in on the line chart** — `src/charts/UsageChart.tsx:101-102`:

```tsx
{area && <path className="chart-area chart-area--fade" d={area} />}   // :101
{line && <path className="chart-line chart-line--draw" pathLength={1} d={line} />}  // :102
```

Driven by `src/charts/charts.css:34-59`:

```css
/* Mount draw-in: pathLength is normalized to 1 in the markup, so a dashoffset
   of 1 starts fully hidden and animates to fully drawn. */
.chart-line--draw {
  stroke-dasharray: 1;
  stroke-dashoffset: 1;
  animation: chart-draw var(--dur-med, 200ms) var(--ease, cubic-bezier(0.2, 0.8, 0.2, 1))
    forwards;
}

.chart-area--fade {
  opacity: 0;
  animation: chart-fade var(--dur-med, 200ms) var(--ease, cubic-bezier(0.2, 0.8, 0.2, 1))
    var(--dur-fast, 120ms) forwards;
}

@keyframes chart-draw { to { stroke-dashoffset: 0; } }
@keyframes chart-fade { to { opacity: 1; } }
```

Reduced-motion override at `charts.css:121-133` sets `animation: none`, `stroke-dashoffset: 0`, `opacity: 1`.

**SVG opacity pulse on the live status glyph** — `src/components/Chain.css:166-177`:

```css
.state-icon.state-live {
  animation: state-live-pulse 1.8s var(--ease) infinite;
}
@keyframes state-live-pulse { 0%, 100% { opacity: 0.5; } 50% { opacity: 1; } }
```

**Non-SVG (HTML) bar animation in the same charts file**: `.breakdown-fill { animation: chart-bar var(--dur-med) var(--ease) both; }` (`charts.css:90-97`, `@keyframes chart-bar` scaleX 0→1 at `:112`), with per-row delay set inline: `` animationDelay: `${Math.min(i, 10) * 40}ms` `` (`src/charts/SpendBreakdown.tsx:50`).

`Sparkline` (`src/charts/Sparkline.tsx:36-37`) renders `.chart-area` / `.chart-line` **without** the `--draw` / `--fade` modifiers — deliberately unanimated.

**Every CSS `@keyframes` name in the codebase — avoid collisions, prefix yours:** `live-pulse` (`Sidebar.css:19`), `chain-pulse` (`Chain.css:131`), `state-live-pulse` (`Chain.css:170`), `op-live-pulse` (`Operators.css:135`), `ov-pulse` (`Overview.css:63`), `pg-live-pulse` / `pg-live-pulse-amber` / `pg-shimmer` (`Playground.css:33,45,80`), `ui-shimmer` (`ui.css:135`), `keys-meter-fill` (`Keys.css:13`), `audit-badge-flash` (`Audit.css:9`), `doc-drop-pulse` / `doc-progress-slide` (`Documents.css:14,42`), `chart-draw` / `chart-fade` / `chart-bar` (`charts.css:49,55,112`).

---

## Test & build environment

### Vitest config — there is no `vitest.config.ts`

**`vitest.config.ts` DOES NOT EXIST.** The only config is inside `vite.config.ts`. Full file, `console/vite.config.ts:1-25`:

```ts
/// <reference types="vitest/config" />
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// Same-origin API paths. In production nginx proxies these to the gateway and
// runtime services; in dev the vite server does the same against localhost.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api/gateway": {
        target: "http://localhost:8080",
        rewrite: (path) => path.replace(/^\/api\/gateway/, ""),
      },
      "/api/runtime": {
        target: "http://localhost:18000",
        rewrite: (path) => path.replace(/^\/api\/runtime/, ""),
      },
    },
  },
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
  },
});
```

- `environment: "node"` — `vite.config.ts:22`
- `include: ["src/**/*.test.ts"]` — `vite.config.ts:23`
- **`setupFiles`: DOES NOT EXIST.** No setup file anywhere; `find src -name "setup*"` returns nothing; the runner reports `setup 0ms`.
- **`globals`: DOES NOT EXIST** (not enabled). Every test must `import { describe, expect, it } from "vitest"` explicitly. Also `types: ["vite/client"]` in tsconfig is a closed list — `vitest/globals` is not in it.

**`src/**/*.test.ts` matches `.ts` ONLY, not `.tsx`.** Verified empirically: probe files `src/__envprobe.test.ts` and `src/__envprobe.test.tsx` were both created and `npx vitest run src/__envprobe` reported `Test Files 1 passed (1)` — the `.tsx` file was silently **not collected**. Running it by explicit path gives: `No test files found, exiting with code 1 / filter: src/zz_probe3.test.tsx / include: src/**/*.test.ts`. **A new `*.test.tsx` file will never run.**

### DOM environment — NOT available

No `jsdom`, `happy-dom`, or `@testing-library/*` is installed:

```
$ ls -d node_modules/jsdom node_modules/happy-dom node_modules/@testing-library
ls: cannot access 'node_modules/jsdom': No such file or directory
ls: cannot access 'node_modules/happy-dom': No such file or directory
ls: cannot access 'node_modules/@testing-library': No such file or directory
```

`package.json:17-24` devDependencies are exactly `@types/react`, `@types/react-dom`, `@vitejs/plugin-react`, `typescript`, `vite`, `vitest`.

Probe run inside the real suite (then deleted):

```
HAS_document: undefined
HAS_window: undefined
HAS_requestAnimationFrame: undefined
HAS_localStorage: undefined
HAS_fetch: function
HAS_EventSource: undefined
```

So `document`, `window`, `requestAnimationFrame`, `localStorage`, `EventSource` are **undefined**. Global `fetch` **is** available (Node built-in). **Rendering React, mounting components, or calling any hook is impossible.**

### What a `.test.ts` CAN and CANNOT do (empirically established)

**CAN import a `.tsx` module.** Probe A: `src/zz_probe_tsx_import.test.ts` with `import { Badge } from "./ui/Badge";` (a `.tsx`) + `expect(typeof Badge).toBe("function")` → `✓ 1 passed`. Probe B: importing the barrel `import { Badge } from "./ui";` — which does `import "./ui.css";` at `src/ui/index.ts:1` and re-exports Toast.tsx/motion.ts (framer-motion) — **also `✓ 1 passed`.** Probe C: `createElement(Badge, {...}).type === Badge` passed. Vite's esbuild transform plus `@vitejs/plugin-react` (`vite.config.ts:8`) compiles `.tsx` transparently regardless of `environment: "node"`.

**CANNOT** (all verified):
- Be named `*.test.tsx` — silently not collected (include glob).
- Contain JSX syntax in a `.test.ts` — esbuild picks its loader by extension and the `.ts` loader rejects JSX: the file fails to transform, `Test Files 1 failed (1) / Tests no tests`. Use `React.createElement(...)` if you need elements.
- Render or mount anything — no DOM, no testing library.

Assertable surface for a `.tsx` module: exported pure helpers/constants, `typeof Comp === "function"`, or an unrendered element tree via `createElement(Comp, props).type` / `.props`.

**Source disagreement.** The testing sweep advises *"Do not import `src/ui/index.ts`"* (because line 1 is `import "./ui.css";` and it pulls framer-motion), and recommends importing `./useCountUp` directly as the existing test does. The adversarial verification of that same claim ran Probe B importing exactly that barrel and it **passed**. Both are reported here: importing the barrel *works*, but the established house convention is to import the narrowest module.

### The one existing test that touches React

`src/ui/useCountUp.test.ts:1-2`:

```ts
import { describe, it, expect } from "vitest";
import { easeOutCubic, formatCount, countUpValue } from "./useCountUp";
```

Target is `src/ui/useCountUp.ts` — a plain `.ts`. It imports **only the three pure functions**, never the hook. But `src/ui/useCountUp.ts:1` is `import { useEffect, useRef, useState } from "react";`, so importing this module **does** pull `react` into the node environment at module-evaluation time, and that works fine. **React can be imported in a test; it just cannot be rendered.**

What it asserts (`useCountUp.test.ts:4-55`): `easeOutCubic` endpoints `0→0`, `1→1`, ahead of linear, monotonic over a 0.05 step loop; `formatCount(12408) === "12,408"`, `formatCount(41.2, { decimals: 2, prefix: "$" }) === "$41.20"`, `formatCount(3, { suffix: " runs" }) === "3 runs"`, `formatCount(6204.321, { decimals: 0 }) === "6,204"`; `countUpValue(100, -50, 700) === 0`, `countUpValue(100, 800, 700) === 100`, `duration 0` returns target immediately (reduced-motion path), midpoint `> 50`.

### tsconfig — exactly one file, and it type-checks the tests

`tsconfig.app.json` / `tsconfig.node.json` **DO NOT EXIST**. Full file, `console/tsconfig.json:1-18`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "moduleResolution": "bundler",
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "skipLibCheck": true,
    "isolatedModules": true,
    "noEmit": true,
    "types": ["vite/client"]
  },
  "include": ["src", "vite.config.ts"]
}
```

- `noUnusedLocals: true` (`:9`) — build-breaking.
- `noUnusedParameters: true` (`:10`) — build-breaking. **Also fires on unused destructured binding elements.**
- `strict: true` (`:8`).
- `jsx: "react-jsx"` (`:7`) — no `import React` needed for JSX.
- `verbatimModuleSyntax`: **DOES NOT EXIST** (not set). `isolatedModules: true` (`:12`) still requires `import type` for type-only imports/re-exports that would otherwise be ambiguous.
- `moduleResolution: "bundler"` (`:6`) — extensionless relative imports are correct.
- `include: ["src", "vite.config.ts"]` (`:17`) — **test files ARE type-checked by `npm run build`.** Verified: adding `const unusedThing = 1;` to a throwaway `src/__tscprobe.test.ts` produced `src/__tscprobe.test.ts(2,7): error TS6133: 'unusedThing' is declared but its value is never read.` `npx tsc` on the clean tree produces zero output.

Empirical `noUnusedParameters` behaviour with the repo's own tsc 5.9.3 and a copy of these flags:

```
src/a.ts(4,23): error TS6133: 'props' is declared but its value is never read.      // function PageA(props: PageProps)
src/a.ts(10,23): error TS6133: 'adminKey' is declared but its value is never read.  // function PageC({ adminKey }: PageProps)
EXIT=2
```

`function PageB(_props: PageProps)` produced **no** error. Re-running with `--noUnusedParameters false` exits 0, proving both errors come from `noUnusedParameters` (not `noUnusedLocals`).

**Scope caveat:** only `tsc` enforces this. `vite build` alone, `npm run dev`, and `vitest` are transpile-only (esbuild) and will NOT flag it — the failure surfaces at `npm run build` / CI, not in dev or tests.

### npm scripts, `make test-console`, CI

`console/package.json:6-11`:

```json
"scripts": {
  "dev": "vite",
  "build": "tsc && vite build",
  "preview": "vite preview",
  "test": "vitest run"
},
```

`/home/iofahd/code/agentos/Makefile:14-15`:

```make
test-console:
	cd console && npm test -- --run && npm run build
```

So `make test-console` = `vitest run --run` **then** `tsc && vite build`. Both must pass. CI does the same: `/home/iofahd/code/agentos/.github/workflows/ci.yml:101-107` runs `npm test -- --run` then `npm run build`.

**`lint`: DOES NOT EXIST.** No `lint` script; no eslint/prettier/biome config in `console/` or the repo root (`ls -a /home/iofahd/code/agentos | grep -iE "eslint|prettier|biome"` → nothing). The `// eslint-disable-next-line react-hooks/exhaustive-deps` at `src/ui/useCountUp.ts:82` is vestigial — no linter runs. `make fmt` (`Makefile:56-59`) only touches Go and Python.

**Current baseline:** `npx vitest run` → `Test Files 22 passed (22)`, `Tests 249 passed (249)`, ~425 ms.

### Import style across the codebase

- **Type imports use `import type`** — 78 occurrences of a leading `import type` across `src/**/*.{ts,tsx}`. Examples: `src/pages/overviewFeed.test.ts:2` `import type { AuditEntry, KeyInfo } from "../lib/types";`; `src/charts/transforms.test.ts:7` `import type { BreakdownOptions } from "./transforms";`; `src/lib/rbac.test.ts:2,4`; `src/components/Chain.tsx:5` `import type { ChainState } from "../lib/chain";`.
- **Inline `type` specifiers** when mixing value + type in one statement — only 5 sites: `src/ui/Modal.tsx:2` `import { useEffect, type ReactNode } from "react";`; `src/lib/operators.ts:6` `import { RUNTIME_BASE, buildRequest, type RequestSpec } from "./api";`; `src/lib/council.ts:6`; `src/pages/Documents.tsx:1`; `src/pages/Keys.tsx:2`.
- **File extensions in import paths: NONE.** `grep -rnE 'from "[^"]*\.(ts|tsx|js|jsx)"'` over all `.ts`/`.tsx` in `src` returns **zero matches**. Always extensionless relative paths. The only extensions that appear are `.css` side-effect imports.
- **Vitest import form** across the 22 test files: 19 use `import { describe, expect, it } from "vitest";` (alphabetical), 1 uses `import { describe, it, expect } from "vitest";` (`src/ui/useCountUp.test.ts:1`), 1 adds `vi` (`src/lib/live/registry.test.ts:2`), 1 uses `import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";` (`src/lib/live/pollTransport.test.ts:1`). **Preferred form: `import { describe, expect, it } from "vitest";`**
- `vi` is used in only two files, for fake timers and mock fns: `vi.useFakeTimers()`, `vi.advanceTimersByTimeAsync(1000)`, `vi.fn<() => Promise<number>>().mockResolvedValue(42)` (`src/lib/live/pollTransport.test.ts:4-36`), `vi.fn()` (`src/lib/live/registry.test.ts:27-28`).
- Test files are **colocated** with their source, named `<module>.test.ts` next to `<module>.ts` (e.g. `src/lib/chain.ts` / `src/lib/chain.test.ts`, `src/pages/overviewFeed.ts` / `src/pages/overviewFeed.test.ts`). All 22 live under `src/` in `charts/`, `lib/`, `lib/live/`, `lib/table/`, `pages/`, `ui/`.
- Every relative import in every existing test resolves to a `.ts` file: `lib/sso.ts`, `lib/types.ts`, `lib/sse.ts`, `lib/live/pollTransport.ts`, `lib/live/status.ts`, `lib/table/{filter,paginate,view,search,sort}.ts`, `lib/api.ts`, `pages/overviewFeed.ts`, `ui/useCountUp.ts`, `lib/{chain,council,operators,rbac,format,improve,export,provisioning}.ts`, `lib/live/{registry,transport,relativeTime}.ts`, `charts/transforms.ts`. **Zero `.tsx` resolutions** today (37 `.tsx` files exist under `src/`; none is imported by a test).

### Complete short test file, verbatim — the style reference

`console/src/lib/live/relativeTime.test.ts`, all 19 lines:

```ts
import { describe, expect, it } from "vitest";
import { formatAgo } from "./relativeTime";

describe("formatAgo", () => {
  it("reads 'just now' under 3s and for clock skew", () => {
    expect(formatAgo(0)).toBe("just now");
    expect(formatAgo(2_999)).toBe("just now");
    expect(formatAgo(-500)).toBe("just now");
  });
  it("counts seconds, minutes, hours, days", () => {
    expect(formatAgo(3_000)).toBe("3s ago");
    expect(formatAgo(59_000)).toBe("59s ago");
    expect(formatAgo(60_000)).toBe("1m ago");
    expect(formatAgo(59 * 60_000)).toBe("59m ago");
    expect(formatAgo(60 * 60_000)).toBe("1h ago");
    expect(formatAgo(23 * 3_600_000)).toBe("23h ago");
    expect(formatAgo(24 * 3_600_000)).toBe("1d ago");
  });
});
```

Style: named `vitest` import on line 1, extensionless relative import of the unit under test on line 2, one `describe` per exported function named exactly after it, `it("...")` descriptions as behavioural sentences (lowercase, no "should"), numeric separators (`2_999`, `3_600_000`), 2-space indent, double quotes, trailing semicolons. Several files (e.g. `src/lib/chain.test.ts:55-56`, `src/pages/overviewFeed.test.ts:96-97`) add short `//` comments explaining *why* a behaviour matters, not what the code does.

Test-helper-factory pattern — `src/pages/overviewFeed.test.ts:11-24`:

```ts
function entry(over: Partial<AuditEntry> = {}): AuditEntry {
  return {
    ts: "2026-07-24T10:00:00Z",
    key_name: "prod",
    model: "gpt-4o",
    input_tokens: 10,
    output_tokens: 5,
    cost_usd: 0.01,
    latency_ms: 120,
    status: 200,
    kind: "chat",
    ...over,
  };
}
```

### What is testable for this work, without a DOM

Design the new page so the *logic* lives in plain `.ts` modules that a `.test.ts` can import directly — the same split the codebase already uses (`src/pages/overviewFeed.ts` + `overviewFeed.test.ts`, `src/lib/chain.ts` + `chain.test.ts`, `src/charts/transforms.ts` + `transforms.test.ts`). Candidates: a `DOC_SECTIONS` / `DIAGRAM_KEYS` content module, section-id uniqueness, anchor-slug derivation, search/filter over sections, and any geometry math for the four SVG visuals (point/path builders returning strings are perfectly testable). **No test covers App routing today** — there is no `App.test.tsx`; test files are all `lib/`, `charts/`, `ui/`, `pages/overviewFeed`.

---

## Landing-page animation technique

Source: `/home/iofahd/code/agentos/landing/index.html` (865 lines; lines 17 and 24 are base64 woff2 `@font-face` data URIs, not animation). There is **exactly one `<script>` block**: lines **542–863**, containing **five IIFEs**. No `<script src=...>`, no module, no build step.

### Every distinct animation on the landing page

| # | What it depicts | Mechanism | Location |
|---|---|---|---|
| 1 | Brand dot + chain-head "Live" dot: expanding ring heartbeat | CSS `@keyframes pulse`, `2s ease-out infinite` / `1.8s ease-out infinite` | `index.html:122-123`, `:160` |
| 2 | Rail sweep: gradient wipe travelling left→right along the chain rule | CSS `@keyframes sweep`, `4.2s cubic-bezier(.5,0,.2,1) infinite` | `index.html:166-167` |
| 3 | Stage node lighting (auth/rbac/budget/rate/audit going live/hold/deny) | **JS `setInterval(tick, 520)`** toggling `.active`/`.hold`/`.deny`; the visual change is a CSS `transition: opacity .18s ease, transform .18s ease` on `.stage .node .core` | `index.html:170-176` (CSS), `:759` (driver) |
| 4 | Audit-tail feed rows appearing at the top | DOM `insertBefore` from the sim's settle callback; entrance is CSS `@keyframes feedIn .28s ease` via a transient `.new` class | `index.html:201-202` (CSS), `:616-632` (JS) |
| 5 | Live readouts (`requests governed`, `spend reserved`, `p95 latency`, `audit rows`) counting up | Plain `textContent` writes on each settle — **no timer of their own** | `index.html:602-613` |
| 6 | Throughput sparkline (SVG polyline redrawn as buckets roll) | `setAttribute('points', …)` on each settle, bucketed by `Date.now()`; **no interval, no rAF** | `index.html:634-683` |
| 7 | Scroll reveal (`.reveal` → `.reveal.in`) | **`IntersectionObserver`** `{ threshold: 0.12 }`, `io.unobserve` after first intersect; motion is CSS `transition: opacity .6s ease, transform .6s ease` | `index.html:296-297` (CSS), `:555-558` (JS) |
| 8 | Council quorum rotation (which member pill reads `dissent` vs `answered`) | **`setInterval(tick, 3200)`** | `index.html:800` |
| 9 | Operators run-history prepend (synthetic run row pushed on, capped) | **`setInterval(tick, 4000)`** | `index.html:861` |
| 10 | Copy-button label flip to `copied ✓` and back | `setTimeout(…, 1400)` (one-shot) | `index.html:560-565` |
| 11 | Card hover lift, nav link color, button press | CSS `transition` only | `index.html:125`, `:130`, `:227-229` |

**`requestAnimationFrame` DOES NOT EXIST anywhere in `landing/` (grep count 0). Web Animations API (`.animate(`) DOES NOT EXIST (count 0).**

### The governance conveyor — markup, verbatim, `landing/index.html:334-341`

```html
<div class="chain">
  <div class="rail"><span class="fill"></span></div>
  <div class="stage"><span class="node"><span class="core"></span></span><div class="lbl">Auth</div><div class="sub">Virtual key or SSO token</div></div>
  <div class="stage"><span class="node"><span class="core"></span></span><div class="lbl">RBAC</div><div class="sub">Org &amp; role scoping</div></div>
  <div class="stage"><span class="node"><span class="core"></span></span><div class="lbl">Budget</div><div class="sub">Atomic spend reserve</div></div>
  <div class="stage"><span class="node"><span class="core"></span></span><div class="lbl">Rate</div><div class="sub">Per-tenant limits</div></div>
  <div class="stage"><span class="node"><span class="core"></span></span><div class="lbl">Audit</div><div class="sub">Every call recorded</div></div>
</div>
```

Stage labels verbatim, in order: `Auth`, `RBAC`, `Budget`, `Rate`, `Audit`. Sub-captions verbatim, in order: `Virtual key or SSO token`, `Org & role scoping`, `Atomic spend reserve`, `Per-tenant limits`, `Every call recorded`.

Enclosing panel (`index.html:328-355`): `.chain-panel.reveal` > `.chain-head` (eyebrow `Governance chain` + `<span class="status"><span class="d"></span> Live</span>`) + `.chain-body` > `.chain`, `.chain-caption`, `.chain-live` (`#audit-feed` + `.spark-wrap` with `<polyline class="spark-line" id="spark-line" points="">` inside `viewBox="0 0 240 40"`).

Caption verbatim (`index.html:342`):

```html
<div class="chain-caption">Every request runs the gauntlet <b>before a provider is ever called</b> — retries and autonomous runs included.</div>
```

### Chain CSS — `landing/index.html:162-188`

```css
.chain { position: relative; display: grid; grid-template-columns: repeat(5, 1fr); }
.rail { position: absolute; left: 5%; right: 5%; top: 7px; height: 2px; background: var(--border-strong); border-radius: 2px; overflow: hidden; }
.rail .fill { position: absolute; inset: 0 100% 0 0; background: linear-gradient(90deg, transparent, var(--live)); animation: sweep 4.2s cubic-bezier(.5,0,.2,1) infinite; }
@keyframes sweep { 0%{right:100%;} 55%,100%{right:0;} }
.stage { text-align: center; position: relative; }
.node { width: 16px; height: 16px; border-radius: 50%; margin: 0 auto; background: var(--bg); border: 2px solid var(--border-strong); position: relative; z-index: 2; }
.stage .node .core { position: absolute; inset: 3px; border-radius: 50%; background: var(--live); opacity: 0; transform: scale(.4); transition: opacity .18s ease, transform .18s ease; }
.stage.active .node { border-color: var(--live); }
.stage.active .node .core { opacity: 1; transform: scale(1); background: var(--live); }
.stage.hold  .node { border-color: var(--hold); }
.stage.hold  .node .core { opacity: 1; transform: scale(1); background: var(--hold); }
.stage.deny  .node { border-color: var(--deny); }
.stage.deny  .node .core { opacity: 1; transform: scale(1); background: var(--deny); }
```

State classes are exactly three — **`active`**, **`hold`**, **`deny`** — added to `.stage`, not `.node`.

### Full driving script, verbatim — `landing/index.html:692-761`

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

  // onSettled is defined in Task 2 (readouts + feed). For Task 1, stub it:
  window.__agentosOnSettled = window.__agentosOnSettled || function () {};
  function onSettled(r) { window.__agentosOnSettled(r); }

  if (reduce) {
    // one representative still: a cleared request lighting all five, no loop.
    stages.forEach(function (s) { s.classList.add('active'); });
    onSettled({ stop: 4, outcome: 'ok', model: MODELS[0], org: ORGS[0], tokens: 1024, cost: 2.4, latency: 380 });
    return;
  }
  setInterval(tick, 520);
  tick();
})();
```

**Technique in one sentence:** one interval advances a cursor `at` one stage per tick through a pre-decided request `{stop, outcome}`; when `at === req.stop` the request settles (emitting a payload to a global callback) and `req` is nulled so the next tick draws a fresh one. The class applied is `active` unless the tick is the terminal one AND outcome !== `'ok'`, in which case it is the literal outcome string (`hold` / `deny`).

Ordering contract, stated at `landing/index.html:568-574`: the readouts/feed IIFE (line 575) defines the real `window.__agentosOnSettled` **before** the chain IIFE (line 694) runs, so the `|| function () {}` stub never engages.

### Determinism / PRNG

`mulberry32` appears **three times, verbatim identical body, three different seeds** — one per independent sim, deliberately not shared:

```js
function mulberry32(a) {
  return function () {
    a |= 0; a = (a + 0x6D2B79F5) | 0;
    var t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
```

| Seed | Consumer | Definition | Seeding line |
|---|---|---|---|
| `0x5EED` | governance chain sim | `index.html:701-708` | `index.html:709` — `var rand = mulberry32(0x5EED);` |
| `0xC0DE1` | council quorum rotation | `index.html:778-785` | `index.html:786` — `var rand = mulberry32(0xC0DE1);` |
| `0xA0BA5` | operators run-history prepend | `index.html:818-825` | `index.html:826` — `var rand = mulberry32(0xA0BA5);` |

Seeds are **hard-coded literals** — no `Date.now()`, no `performance.now()` seeding anywhere.

`grep -rn 'Math\.random' landing/` returns four hits — `index.html:572`, `:693`, `:767`, `:806` — **all four are comments asserting its absence**. `grep -rn 'Math\.random(' landing/` returns **no matches (exit 1)**. There is zero `Math.random()` call site. (`Math.floor` and `Math.imul` are used; `Math.imul` is required by mulberry32.)

The one non-deterministic input on the whole page is `Date.now()` at `index.html:670`, used **only** to bucket the sparkline into wall-clock windows — documented at `:573-574` as introducing "no interval and nothing awaits it".

### Reduced-motion handling on the landing page

The JS check, identical string in three places:

```js
var reduce = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
```

- `index.html:584` (readouts/feed IIFE) — used only as `row.className = 'feed-row' + (reduce ? '' : ' new');` at `:618`, suppressing the `feedIn` entrance class.
- `index.html:699` (chain sim) — the branch at `:753-758`.
- `index.html:775` (council) — `if (reduce) return;` at `:776`.
- `index.html:815` (operators) — `if (reduce) return;` at `:816`.

It is a **one-shot `.matches` read at IIFE time**; there is **no `addEventListener('change', …)` on the media query** — the page does not react to a live preference flip. (`addEventListener` appears exactly once in the file, `:547`, for the theme toggle click.)

The media queries:

```css
@media (prefers-reduced-motion: reduce) { html { scroll-behavior: auto; } }
```
(`index.html:86`)

```css
@media (prefers-reduced-motion: reduce) {
  .rail .fill { animation: none; right: 0; }
  .stage .node .core { transition: none !important; }
  .stage.active .node .core, .stage.hold .node .core, .stage.deny .node .core { opacity: 1; transform: scale(1); }
  .brand .dot, .chain-head .status .d { animation: none; }
  .feed-row.new { animation: none; }
}
```
(`index.html:179-185`)

```css
@media (prefers-reduced-motion: reduce){ .reveal { opacity: 1; transform: none; transition: none; } }
```
(`index.html:298`)

**What the static fallback shows, precisely:**
- **Chain**: all five stages get `.active` (`:755`) — a fully cleared request, five cores lit teal at full scale. The rail `.fill` is frozen at `right: 0`, i.e. **fully swept**, not empty. No interval is ever created.
- **Readouts / feed / sparkline**: `onSettled` is called **exactly once** with `{ stop: 4, outcome: 'ok', model: MODELS[0], org: ORGS[0], tokens: 1024, cost: 2.4, latency: 380 }` (`:756`), yielding `rd-count` = `1`, `rd-spend` = `$2.40`, `rd-p95` = `380 ms`, `rd-audit` = `1`, and exactly one feed row reading `qwen3.8-max · acme · 1024 tok · $2.40` with a `pill ok` chip — `class="feed-row"` **without** `new`, so no entrance animation.
- **Sparkline**: `pushThroughput()` runs once → 23 leading zeros + one point of value 1 at the far right. Documented at `:636-639` as "a flat baseline with a single live point at the end — a representative still, no interval involved."
- **Council panel**: `if (reduce) return;` — the authored static markup stands: A `answered` / B `answered` / C `dissent` (`index.html:457-459`), "one honest dissent" (comment `:769`).
- **Operators panel**: `if (reduce) return;` — the shipped three rows stand: `nightly invoice report` (completed), `reconcile ledger` (needs approval), `pending orders` (completed) (`index.html:482-484`).
- **Reveal**: `.reveal` forced to `opacity: 1; transform: none; transition: none`. The `IntersectionObserver` at `:555-558` **still runs** under reduced motion (it is not guarded) — it just has no visible effect.
- **Pulses**: `.brand .dot` and `.chain-head .status .d` animations set to `none`.

### Timer cleanup / visibility handling — DOES NOT EXIST

Grep counts over `landing/`:

```
requestAnimationFrame    0
cancelAnimationFrame     0
clearInterval            0
clearTimeout             0
visibilitychange         0
document.hidden          0
pageshow                 0
```

The three `setInterval` handles (`:759`, `:800`, `:861`) are **not assigned to any variable** and are never cleared — they run for the document's lifetime. The only `setTimeout` (`:564`, copy-button label restore, 1400 ms) is likewise never stored or cleared. There is no pause-when-hidden logic of any kind.

**Porting consequences for the console page:**
1. Every interval becomes a `useEffect` that MUST return `() => clearInterval(id)` — React StrictMode double-mounts in dev, and hot-reload remounts would otherwise stack duplicate intervals. The landing page gets away with leaking only because it never unmounts.
2. The cross-IIFE coupling via the **global** `window.__agentosOnSettled` (`:685`, `:750-751`) must become a prop/callback or a single reducer. The "define the real one first, stub second" ordering trick has no equivalent under React's mount order.
3. Reduced motion must be read with framer-motion's `useReducedMotion()` (the console's 15-call-site convention), not a one-shot `matchMedia` read, so a live preference flip is honoured.
4. The console already has a visibility-pause facility for polls (`installVisibilityPause(defaultRegistry)`, `App.tsx:227`); a hand-rolled animation interval is **not** covered by it.

---

## Chain instrument (CHAIN_STAGES)

Source: `console/src/lib/chain.ts` (99 lines), `console/src/components/Chain.tsx` (133 lines), `console/src/components/Chain.css` (191 lines).

### The design law — never draw checks it cannot prove ran

`console/src/lib/chain.ts:18-20`:

```
 * Keep this in sync with the gateway. If a stage is added there, add it here —
 * a chain that under-reports is worse than no chain, because it implies checks
 * ran that did not.
```

`console/src/lib/chain.ts:44-47` (on `chainStateFromStatus`):

```
 * Unknown statuses are treated as denials at the first stage rather than as
 * passes: the console must never draw checks it cannot prove ran (the same
 * fail-closed posture the gateway itself takes).
```

`console/src/lib/chain.ts:88-90` (on `StageRender`):

```
 * "cleared" stages are drawn in ink, the halting stage takes the outcome color,
 * and stages after it stay unlit — the request never reached them, so showing
 * them as anything but dark would be a lie.
```

`console/src/components/Chain.tsx:56-59`:

```
 * It is driven entirely by recorded audit statuses (see lib/chain.ts) rather
 * than by an animation timer, so a lit chain is evidence rather than decoration.
 * With no key, or no traffic, it sits unlit — deliberately, because "quiet" and
 * "healthy" must not look the same as "unknown".
```

### `CHAIN_STAGES` and types — verbatim

`console/src/lib/chain.ts:24`:

```ts
export const CHAIN_STAGES = ["auth", "rbac", "budget", "rate", "audit"] as const;
```

**Lowercase, `as const`.** This differs in case from the landing page's `Auth` / `RBAC` / `Budget` / `Rate` / `Audit` display labels.

`console/src/lib/chain.ts:26`:

```ts
export type ChainStage = (typeof CHAIN_STAGES)[number];
```

i.e. `"auth" | "rbac" | "budget" | "rate" | "audit"`.

`console/src/lib/chain.ts:29-40`, `:92`:

```ts
export type ChainOutcome = "pass" | "deny" | "fail";

export interface ChainState {
  /** Count of stages cleared, 0..CHAIN_STAGES.length. */
  cleared: number;
  /** Stage that halted it, or null when the whole chain cleared. */
  stoppedAt: ChainStage | null;
  outcome: ChainOutcome;
}

export const IDLE_CHAIN: ChainState = { cleared: 0, stoppedAt: null, outcome: "pass" };

export type StageRender = "cleared" | "stopped" | "unlit";
```

### Deriving state from evidence

**Evidence source: the HTTP status recorded in the audit log.** No dedicated endpoint, no extra field (`chain.ts:5-7`).

`console/src/lib/chain.ts:49-72`, verbatim:

```ts
export function chainStateFromStatus(status: number): ChainState {
  if (status >= 200 && status < 300) {
    return { cleared: CHAIN_STAGES.length, stoppedAt: null, outcome: "pass" };
  }
  if (status >= 500) {
    // Governance allowed it; the upstream provider is what broke. The chain is
    // fully cleared, but the outcome is not a success.
    return { cleared: CHAIN_STAGES.length, stoppedAt: null, outcome: "fail" };
  }
  switch (status) {
    case 401:
      return { cleared: 0, stoppedAt: "auth", outcome: "deny" };
    case 403:
      return { cleared: 1, stoppedAt: "rbac", outcome: "deny" };
    case 402:
      return { cleared: 2, stoppedAt: "budget", outcome: "deny" };
    case 429:
      return { cleared: 3, stoppedAt: "rate", outcome: "deny" };
    case 400:
      return { cleared: 2, stoppedAt: "budget", outcome: "deny" };
    default:
      return { cleared: 0, stoppedAt: "auth", outcome: "deny" };
  }
}
```

`console/src/lib/chain.ts:80-83`:

```ts
export function latestChainState(entries: readonly { status: number }[]): ChainState {
  if (entries.length === 0) return IDLE_CHAIN;
  return chainStateFromStatus(entries[0].status);
}
```

Entries are assumed newest-first, matching `GET /admin/audit` (`chain.ts:76-78`).

`console/src/lib/chain.ts:94-99`:

```ts
export function stageRenders(state: ChainState): StageRender[] {
  return CHAIN_STAGES.map((stage, i) => {
    if (state.stoppedAt === stage) return "stopped";
    return i < state.cleared ? "cleared" : "unlit";
  });
}
```

**Liveness derivation** (`Chain.tsx:76-97`): polls `` `admin/audit?limit=100#${adminKey}` `` at `cadence: 5000` via `useLiveResource`; `active` is set **only** when `entries.length > seenCount.current` (feed growth), which sets `activeUntil = Date.now() + LINGER_MS` where `const LINGER_MS = 6000` (`Chain.tsx:12`). A null/failed poll `return`s early and **holds the last reading rather than reporting a denial** (`Chain.tsx:85-88`). No `adminKey` resets to `IDLE_CHAIN`.

**Fill fraction** (`Chain.tsx:102`): `const progress = state.cleared / CHAIN_STAGES.length;`

### Exact classes Chain.tsx renders

Root (`Chain.tsx:105`):

```tsx
<div className="chain" data-outcome={state.outcome} data-active={active || undefined}>
```

`data-outcome` ∈ `"pass" | "deny" | "fail"`; **`data-active` is present or absent, never `"false"`.**

Children:
- `chain-track` (`:106`, `aria-hidden`)
- `chain-rule` (`:107`)
- `chain-fill` (`:109` static branch, `:112` motion branch) — reduced motion uses a plain `` <div style={{ transform: `scaleX(${progress})` }}> ``; otherwise `<motion.div initial={false} animate={{ scaleX: progress }} transition={{ duration: 0.55, ease: [0.2, 0.8, 0.2, 1] }} />`. `reduced` comes from `useReducedMotion()` (`Chain.tsx:1`, `:62`).
- `chain-stages` (`<ol>`, `:119`)
- `chain-stage` (`<li>`, `:121`) with `data-render={renders[i]}` (`"cleared" | "stopped" | "unlit"`), `key={stage}`, `title={STAGE_TITLES[stage]}`
- `chain-node` (`<span aria-hidden>`, `:122`)
- `chain-label` (`<span>{STAGE_LABELS[stage]}</span>`, `:123`)
- `chain-outcome` (`<div aria-live="polite">`, `:128`) containing `<StateIcon state={glyph.state} title={glyph.label} size={12} />`
- `StateIcon` itself emits `` className={`state-icon state-${state}`} `` (`ui/icons.tsx:437`) → `state-icon state-live` / `state-ok` / `state-hold` / `state-deny`

`STAGE_LABELS` (`Chain.tsx:15-21`) is an identity map: `auth: "auth", rbac: "rbac", budget: "budget", rate: "rate", audit: "audit"`.

`STAGE_TITLES` verbatim (`Chain.tsx:24-30`):

```ts
auth: "auth — the caller presented a valid credential",
rbac: "rbac — the caller's role may make this call",
budget: "budget — the key is within its spend budget",
rate: "rate — the key is within its rate limit",
audit: "audit — the request was recorded",
```

`outcomeGlyph` (`Chain.tsx:33-42`): `active` → `{ state: "live", label: "request in flight" }`; `deny` → `` { state: "deny", label: `denied at ${state.stoppedAt}` } ``; `fail` → `{ state: "hold", label: "cleared governance, provider failed" }`; else `{ state: "ok", label: "cleared" }`.

### Hues

| Token | Console value (`console/src/styles.css:30-44`) | Comment in source |
|---|---|---|
| `--live` | `#5ad1c4` | `/* in flight right now */` |
| `--ok` | `#6cc48f` | `/* completed / allowed */` |
| `--hold` | `#e3a851` | `/* held, awaiting a human */` |
| `--deny` | `#e2685f` | `/* denied / failed / over budget */` |
| `--text-dim` | `#98a1a8` | |
| `--text-faint` | `#7e878e` | |
| `--border-strong` | `rgba(255, 255, 255, 0.13)` | |

Identical to the landing page's dark values (`landing/index.html:36-39`). Landing's **light theme** overrides them to `--live: #12897c; --ok: #2f8f57; --hold: #a8721c; --deny: #bb4038;` (`landing/index.html:57-60`) — **the console has no light theme, so those values have no console counterpart.**

Chain.css hue application (`console/src/components/Chain.css`):
- `.chain-rule` → `var(--border-strong)` (`:30`)
- `.chain-fill` → `var(--text-faint)` by default (`:34`); `.chain[data-outcome="deny"] .chain-fill` → `var(--deny)` (`:41-43`); `.chain[data-outcome="fail"] .chain-fill` → `var(--hold)` (`:45-47`). **A clean pass stays graphite** — comment at `:39-40`: "a clean pass stays graphite, because nothing needs your attention."
- `.chain-node` unlit → `background: var(--bg); box-shadow: inset 0 0 0 1px var(--border-strong);` (`:68-78`)
- `[data-render="cleared"]` → node `background: var(--text-dim)` + `inset 0 0 0 1px var(--text-dim)`; label `color: var(--text-dim)` (`:91-98`)
- `[data-render="stopped"]` → node `background: var(--deny)` + `inset 0 0 0 1px var(--deny), 0 0 0 3px rgba(226, 104, 95, 0.16)`; label `color: var(--deny)` (`:101-110`). Under `data-outcome="fail"` the stopped node switches to `var(--hold)` + `0 0 0 3px rgba(227, 168, 81, 0.16)` (`:112-117`).
- Nodes are **`7px` squares, `border-radius: 1px`** — comment `:66-67`: "Nodes are tap points on the run, not bullets: square, seated on the rule". **This differs from the landing page's `16px` `border-radius: 50%` circles.**
- `.chain[data-active] .chain-fill { animation: chain-pulse 1.6s var(--ease) infinite; }` with `@keyframes chain-pulse { 0%,100% { opacity: 0.55 } 50% { opacity: 1 } }` (`:127-139`), disabled under `prefers-reduced-motion` (`:141-145`).
- `.state-icon.state-live { animation: state-live-pulse 1.8s var(--ease) infinite; }` with `0%,100% { opacity: 0.5 } 50% { opacity: 1 }` (`:166-178`), also disabled under reduced motion (`:180-184`).
- `@media (max-width: 720px) { .chain-label { display: none; } }` (`:186-191`) — "Labels drop before nodes do".

`--ease` is `cubic-bezier(0.2, 0.8, 0.2, 1)` (`console/src/styles.css:65`) — the same curve hard-coded as the framer-motion array `[0.2, 0.8, 0.2, 1]` at `Chain.tsx:115`. `--dur-med` is `180ms` (`styles.css:63`); `console/src/ui/motion.ts:14` carries a **stale comment** claiming `--dur-med: 200ms`.

### Implication for the four new SVG visuals

If any of the four visuals depicts the governance chain, it must use `CHAIN_STAGES` (`["auth","rbac","budget","rate","audit"]`) as the source of stage identity, must not imply a check ran that has no evidence behind it, and — if it is decorative/illustrative rather than evidence-driven — must be visibly distinguishable from the live `Chain` instrument in the topbar, which is evidence-driven by contract (`Chain.tsx:56-59`).

---

## Page idiom

### Export signature

Every page is a **named export**, `export function <Name>(props: PageProps)`. **No default export exists on any page — `export default` DOES NOT EXIST in `src/pages/`.**

| File:line | Signature |
|---|---|
| `src/pages/Audit.tsx:37` | `export function Audit({ adminKey, openSettings }: PageProps) {` |
| `src/pages/Overview.tsx:52` | `export function Overview({ adminKey, openSettings, navigate }: PageProps) {` |
| `src/pages/Operators.tsx:49` | `export function Operators(_props: PageProps) {` |
| `src/pages/Playground.tsx:75` | `export function Playground(_props: PageProps) {` |
| `src/pages/Provisioning.tsx:16` | `export function Provisioning({ adminKey, role, openSettings }: PageProps) {` |
| `src/pages/Users.tsx:19` | `export function Users({ adminKey, role, orgId, openSettings }: PageProps) {` |
| `src/pages/Orgs.tsx:26` | `export function Orgs({ adminKey, role, openSettings }: PageProps) {` |
| `src/pages/Documents.tsx:23` | `export function Documents(_props: PageProps) {` |
| `src/pages/Improve.tsx:66` | `export function Improve(_props: PageProps) {` |
| `src/pages/Multiverse.tsx:44` | `export function Multiverse({ adminKey }: PageProps) {` |
| `src/pages/Secrets.tsx:9` | `export function Secrets({ adminKey, role, openSettings }: PageProps) {` |
| `src/pages/Keys.tsx:44` | `export function Keys({ adminKey, role, openSettings }: PageProps) {` |

**Convention:** destructure only the props you use; if you use none, name the parameter `_props` (Documents, Operators, Playground, Improve). Under `noUnusedParameters`, `props` (unprefixed and unused) and any unused destructured field are build-breaking errors; `_props` is not.

Registration is manual in exactly two places in `src/App.tsx`: an import at `:32-43` and a `ROUTES` entry at `:62-99`.

### Standard top-of-page structure

The page returns a bare `<>…</>` fragment — **no wrapper div**; `App.tsx` supplies `<div className="page">` (`src/App.tsx:284`/`:289`). Order is: `PageHead` → gating notices (`NeedsKey` / `ForbiddenNotice`) → `ErrorNotice`s → `Panel`s.

Real excerpt — `src/pages/Secrets.tsx:45-74`:

```tsx
  return (
    <>
      <PageHead
        title="Secrets"
        subtitle="Provider secrets resolved by the gateway — presence and backend only. Values are never exposed. Root admin only."
      />
      {!adminKey && <NeedsKey openSettings={openSettings} />}
      {adminKey && !allowed && <ForbiddenNotice message="Secret status is visible to the root admin only." />}
      <ErrorNotice error={error} />

      {adminKey && allowed && (
        <Panel>
          <PanelHead
            title="Secret status"
            actions={
              <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <Button
                  small
                  icon="refresh"
                  onClick={() => void reloadSecrets()}
                  disabled={reloading || loading}
                >
                  {reloading ? "Reloading…" : "Reload secrets"}
                </Button>
                <Button small icon="refresh" onClick={reload} disabled={loading}>
                  {loading ? "Loading…" : "Refresh"}
                </Button>
              </div>
            }
          />
```

Panels stack vertically by default (`.panel { margin-bottom: 24px }`). Side-by-side panels use a page-local grid class, e.g. `.mv-split` (`src/pages/Multiverse.css:82-93`) applied at `src/pages/Multiverse.tsx:189` — `grid-template-columns: 1fr 1fr` collapsing to `1fr` under `@media (max-width: 900px)`.

### The canonical import block — `src/pages/Multiverse.tsx:1-28`

```ts
import type { PageProps } from "../App";
import { ErrorNotice, PageHead, errorMessage, useLoad } from "../components/common";
import { Freshness } from "../components/Freshness";
import { useLiveResource } from "../hooks/useLiveResource";
import { apiFetch, apiFetchRaw } from "../lib/api";
import { formatTimestamp } from "../lib/format";
import { Badge, Button, EmptyState, Input, Panel, PanelHead, Skeleton, Table, Tbody, Tr, useToast } from "../ui";
import type { BadgeVariant } from "../ui";
import { Icon } from "../ui/icons";
import "./Multiverse.css";
```

Note: `import type { PageProps } from "../App";` — the page imports a type from the module that imports the page. It is **type-only**, so the cycle is erased at compile time. Keep it `import type`.

### Page CSS

One CSS file per page, same basename as the `.tsx`, sibling in `src/pages/`, imported **last** in the import block as a bare side-effect import (`src/pages/Improve.tsx:45` `import "./Improve.css";`, `src/pages/Multiverse.tsx:28` `import "./Multiverse.css";`). Classes are prefixed per page: Multiverse `mv-*` (`.mv-controls`, `.mv-pad`, `.mv-members`, `.mv-member`, `.mv-member-icon`, `.mv-member-body`, `.mv-member-id`, `.mv-member-model`, `.mv-member-tags`, `.mv-launch`, `.mv-split`, `.mv-input-cell`, `.mv-detail`, `.mv-detail-head`, `.mv-obj-input`, `.mv-cycle`, `.mv-cycle-head`, `.mv-answer`, `.mv-dissent`, `.mv-dissent-member`, `.mv-dissent-claim`, `.mv-dissent-basis`, `.mv-action-cell`); Improve `improve-*`; Playground `pg-*`. **CSS-modules are NOT used — plain global CSS.** Colors/spacing come only from custom properties defined in `src/styles.css`.

### Pages ↔ CSS ↔ routes

| Page file | Matching .css | Route path |
|---|---|---|
| `src/pages/Audit.tsx` | YES `Audit.css` | `/audit` |
| `src/pages/Documents.tsx` | YES `Documents.css` | `/documents` |
| `src/pages/Improve.tsx` | YES `Improve.css` | `/improve` |
| `src/pages/Keys.tsx` | YES `Keys.css` | `/keys` |
| `src/pages/Multiverse.tsx` | YES `Multiverse.css` | `/multiverse` |
| `src/pages/Operators.tsx` | YES `Operators.css` | `/operators` |
| `src/pages/Overview.tsx` | YES `Overview.css` | `/` |
| `src/pages/Playground.tsx` | YES `Playground.css` | `/playground` |
| `src/pages/Orgs.tsx` | **NO CSS FILE** | `/orgs` (gated `can(r, "org.view")`) |
| `src/pages/Provisioning.tsx` | **NO CSS FILE** | `/provisioning` (gated `can(r, "provisioning.view")`) |
| `src/pages/Secrets.tsx` | **NO CSS FILE** | `/secrets` (gated `can(r, "secret.view")`) |
| `src/pages/Users.tsx` | **NO CSS FILE** | `/users` (gated `can(r, "user.view")`) |

Full `src/pages/` listing: `Audit.css`, `Audit.tsx`, `Documents.css`, `Documents.tsx`, `Improve.css`, `Improve.tsx`, `Keys.css`, `Keys.tsx`, `Multiverse.css`, `Multiverse.tsx`, `Operators.css`, `Operators.tsx`, `Orgs.tsx`, `Overview.css`, `Overview.tsx`, `overviewFeed.test.ts`, `overviewFeed.ts`, `Playground.css`, `Playground.tsx`, `Provisioning.tsx`, `Secrets.tsx`, `Users.tsx`. **`src/pages/overviewFeed.ts` + `overviewFeed.test.ts` are the precedent for putting page logic in a testable `.ts` sibling.**

### A purely static page

**DOES NOT EXIST** — all 12 pages import from `src/lib/api` and issue API calls. Closest: **`src/pages/Playground.tsx`**, the only page making **zero API calls on mount** — it imports neither `useLoad` nor `useLiveResource` (`import { ErrorNotice, PageHead, errorMessage } from "../components/common";`, `Playground.tsx:4`). All three of its call sites fire from user handlers: `streamSSE(runtimeRequest("/runs/stream", body), …)` (`:155`), `apiFetchRaw(runtimeRequest("/runs", body))` (`:174`), `` runtimeRequest(`/runs/${threadId}/approve`, { approve }) `` (`:197`). It renders a `PageHead` + local `useState` timeline until the user submits. **A docs page with no API calls at all would be the first of its kind — that is allowed, but it is new territory; model it on Playground's structure minus the handlers.**

Call-site counts (`apiFetch|apiFetchRaw|useLiveResource|useLoad|fetch(`): Playground 3, Audit 4, Documents 5, Keys 5, Secrets 5, Orgs 6, Provisioning 6, Users 8, Overview 9, Operators 11, Improve 12, Multiverse 16.

### "Feature is off, not broken" idiom

`src/pages/Improve.tsx:154-159` hoists the head into a const:

```tsx
  const head = (
    <PageHead
      title="Improve"
      subtitle="Eval-gated self-improvement — run the eval suite, let the runtime propose a better system prompt, and approve or deny it. Nothing activates without you."
    />
  );
```

then `src/pages/Improve.tsx:161-177` (env var on line 170):

```tsx
  if (disabled) {
    return (
      <>
        {head}
        <EmptyState
          title="Self-improvement is not enabled on this runtime"
          description={
            <>
              It needs the checkpoint database (set{" "}
              <span className="mono">AGENTOS_CHECKPOINT_DB</span>) to store eval runs and
              prompt proposals — once configured, this page lights up.
            </>
          }
        />
      </>
    );
  }
```

The Multiverse variant uses a `div.notice.warn` instead of an `EmptyState`, and mentions `AGENTOS_COUNCIL_CONFIG` as bare text with no `.mono` span (`src/pages/Multiverse.tsx:107-112`):

```tsx
      {disabled && (
        <div className="notice warn">
          The council is not configured on this runtime. Set AGENTOS_COUNCIL_CONFIG and a checkpoint
          database to enable it.
        </div>
      )}
```

Both use the same "503 means off, not broken" sentinel pattern — `const DISABLED = "disabled";` (`src/pages/Improve.tsx:49`) vs `const DISABLED = Symbol("council-disabled");` (`src/pages/Multiverse.tsx:33`), each with an `async function orDisabled<T>(promise: Promise<T>): Promise<T | typeof DISABLED>` wrapper (`Improve.tsx:51`, `Multiverse.tsx:35`).

### Code / `<pre>` blocks in a page — the two existing patterns

**(a) `prompt-text mono`** — the standard long-text/prompt block, used twice in Improve:
- `src/pages/Improve.tsx:320` — `<pre className="prompt-text mono">{active.prompt}</pre>`
- `src/pages/Improve.tsx:481` — `<pre className="prompt-text mono">{proposal.prompt_text}</pre>`

Both wraps are height-animated by a `motion.div` with `className="improve-prompt-reveal"` (`Improve.tsx:313`, `:475`), styled `overflow: hidden;` at `src/pages/Improve.css:97-99`.

**(b) `pg-tool-pre`** — Playground's JSON tool-input block, `src/pages/Playground.tsx:68`:

```tsx
            <pre className="pg-tool-pre">{prettyJSON(input)}</pre>
```

Inline (non-block) machine values use `<code className="pg-tool-summary">` (`Playground.tsx:55`) or, far more commonly, `className="mono"` on a `span`/`td` — e.g. `<span className="mono">{c.name}</span>` (`Improve.tsx:393`), `<td className="mono">{r.suite}</td>` (`Improve.tsx:240`), `<td className="dim mono">{formatTimestamp(r.created_at)}</td>` (`Improve.tsx:239`).

---

## VERIFIED CLAIMS

Six claims were adversarially verified against source. Verdicts and corrections below. **Where a claim was PARTIALLY_TRUE, the correction is what you must act on — not the claim.**

### C1 — Router matches by exact string equality; `navigate("/docs?s=gateway")` falls back to Overview and drops the sidebar pill

**VERDICT: PARTIALLY_TRUE.**

TRUE: matching is exact string equality against the raw path state (`App.tsx:150`), and `navigate(p)` stores the FULL argument verbatim via `setPath(p)` (`App.tsx:128-131`), query string and hash included. `navigate("/documents?s=gateway")` → `"/documents" === "/documents?s=gateway"` is false → `?? ROUTES[0]` silently renders Overview (`App.tsx:63`). No 404, no warning.

WRONG EXAMPLE: **there is no `/docs` route.** The documents route path is `/documents` (`App.tsx:67`); `grep -rn '"/docs' console/src` → zero hits. `navigate("/docs")` falls back to Overview simply because `/docs` does not exist, so that example does not demonstrate the query-string bug. Use `navigate("/documents?s=gateway")`.

FALSE: **the Sidebar active pill is NOT dropped.** `App.tsx:263` passes `activePath={route.path}` (the resolved route), not the raw path, so on fallback `activePath` becomes `"/"` and `Sidebar.tsx:96` lights the Overview pill. The pill stays visible and consistent with the rendered page — **which is precisely why the bug is silent: the shell looks like a normal, deliberate Overview navigation.** Nothing anywhere reads the raw `path` state except the `ROUTES.find` on line 150. The same applies to the page-transition key (`App.tsx:288` `key={route.path}`).

ADDITIONAL NUANCE: the breakage is **asymmetric**. `usePath` seeds from `window.location.pathname` (`App.tsx:122`) and the popstate handler also reads `window.location.pathname` (`App.tsx:124`) — both strip query and hash. So a hard load, a reload, or browser back/forward onto `/documents?s=gateway` matches `/documents` correctly and renders Documents. **Only in-app `navigate()` with a query/hash mismatches.** Because `navigate` also does `window.history.pushState({}, "", p)` (`App.tsx:129`), the URL bar reads `/documents?s=gateway` while Overview is on screen, and pressing reload then renders Documents — **the same URL yields two different pages depending on how you arrived.**

MINIMAL FIX: normalise in the matcher — `const base = path.split(/[?#]/)[0];` then `ROUTES.find((r) => r.path === base)`, keeping the raw path (or a parsed `URLSearchParams`) available separately for pages that need the query. Only two `navigate()` call sites exist today (`App.tsx:163`, `src/pages/Overview.tsx:225`), neither passes a query, so nothing regresses.

### C2 — `GLYPHS` is module-private, so a `.test.ts` cannot assert every `DocSection` icon name is a real glyph; no runtime icon-name array is exported

**VERDICT: PARTIALLY_TRUE.**

The mechanical half is exactly right. `GLYPHS` at `src/ui/icons.tsx:53` is `const GLYPHS: Record<IconName, JSX.Element>` with no `export`, and is not re-exported by `src/ui/index.ts` (which does not export `./icons` at all). `icons.tsx` exports exactly five things: types `IconName` (`:22`) and `StateName` (`:396`), components `Icon` (`:340`), `BrandMark` (`:373`), `StateIcon` (`:435`). No runtime array/tuple of icon names exists.

Three corrections:

**(a) `DocSection` does not exist in the codebase.** It appears only as a planned interface at `docs/superpowers/specs/2026-07-27-platform-docs-and-console-docs-tab.md:139-140`. `console/src/pages/docs/` does not exist. The claim is phrased in the present tense about code not yet written.

**(b) The invariant is already enforced at compile time, more strongly than a runtime test would enforce it.** `GLYPHS` is `Record<IconName, JSX.Element>` (exhaustive: every `IconName` MUST have a glyph) and the planned `DocSection.icon` is typed `IconName` (spec:139), so a bogus icon name is a `tsc` error, and `tsc` gates the build (`"build": "tsc && vite build"`). The spec calls this out at `:156`: "the same compile-enforced pattern as `GLYPHS`". **A runtime test asserting "every DocSection icon name is a real glyph" would be strictly redundant with the type checker. Do not add an `ICON_NAMES` export solely to write that test.**

**(c) If a runtime test is wanted for some other reason, privacy is not a blocker** — the spec's file list at `:126` already sanctions "edits to `src/ui/icons.tsx`", so adding `export const ICON_NAMES = [...] as const; export type IconName = (typeof ICON_NAMES)[number];` (mirroring the planned `DIAGRAM_KEYS` pattern at spec:150) is in scope. Note the render-based workaround is NOT viable without further changes: `.test.tsx` is outside the include glob, there is no DOM environment, and jsdom/happy-dom/@testing-library are not installed.

### C3 — Vitest runs `environment: "node"` with no jsdom, and no existing `.test.ts` imports a `.tsx` module — so a new test must not import a `.tsx` file

**VERDICT: PARTIALLY_TRUE.**

Parts 1 and 2 CONFIRMED: `vite.config.ts:21-24` sets `environment: "node"` and `include: ["src/**/*.test.ts"]`; no jsdom/happy-dom/@testing-library installed; a probe asserting `expect(typeof document).toBe("undefined")` PASSED. All 22 existing test files are `*.test.ts` and every relative import in every one resolves to a `.ts` file.

Part 3 **REFUTED by execution.** Probe A: `import { Badge } from "./ui/Badge";` (a `.tsx`) + `expect(typeof Badge).toBe("function")` → `✓ 1 passed`. Probe B: importing the barrel `./ui` (which does `import "./ui.css"` and re-exports framer-motion consumers) → `✓ 1 passed`. Probe C: `createElement(Badge, {...}).type === Badge` → passed. Vite's esbuild transform plus `@vitejs/plugin-react` compiles `.tsx` transparently under `environment: "node"`.

WHAT ACTUALLY FAILS (verified):
- `.test.tsx` files are NOT collected — `npx vitest run src/zz_probe3.test.tsx` → `No test files found, exiting with code 1 / filter: src/zz_probe3.test.tsx / include: src/**/*.test.ts`. This is the include glob, not the environment.
- JSX syntax inside a `.test.ts` fails to build — esbuild picks its loader by extension and the `.ts` loader rejects JSX: `Test Files 1 failed (1) / Tests no tests`.
- No DOM APIs, so nothing can be rendered/mounted, and there is no testing library to do it with.

REAL CONSTRAINTS for a new test: name it `*.test.ts` under `console/src/`; write no JSX in it (use `React.createElement`); render/mount nothing; assert only on non-rendering surface (exported pure helpers/constants, `typeof Comp === "function"`, `createElement(Comp, props).type` / `.props`). To get real component rendering you must add jsdom/happy-dom + a testing library **and** change `environment` — none of that exists today.

Baseline: `npx vitest run` in `console/` reports `Test Files 22 passed`.

### C4 — The console has NO shared reusable styling for code/pre blocks; every page defines its own page-local class; a bare `<pre>` would be unstyled and could overflow

**VERDICT: PARTIALLY_TRUE.**

TRUE: the base element rule is font-only — `styles.css:100-106` sets `font-family: var(--mono); font-size: 12px; font-variant-numeric: tabular-nums;` (the claim omits `font-variant-numeric`). Exhaustive greps: `pre` appears in CSS at exactly `styles.css:101` and `styles.css:712`; `code` at exactly `styles.css:100` and `styles.css:637`. All JSX code blocks: `Users.tsx:142`, `Improve.tsx:320`/`:481`, `Keys.tsx:115`, `Playground.tsx:55`/`:67`/`:249`/`:267`/`:307`, `SettingsModal.tsx:70`/`:71`.

FALSE 1: **`styles.css` DOES contain block-level code styling shared across pages.** `.secret-reveal code` (`styles.css:637-645`) is consumed by a BARE `<code>` on two different pages with no page-local class — `Keys.tsx:115` `<code>{created.key}</code>` and `Users.tsx:142` `<code>{created.token}</code>`. `.event pre` (`styles.css:712-716`) is consumed by a BARE `<pre>` at `Playground.tsx:249` and `:267`.

FALSE 2: **the one full-featured code-block class is global, not page-local.** `.prompt-text` (`styles.css:774-783`) lives in the shared `styles.css` (imported once at `main.tsx:4`); `Improve.css:89-91` only adds `margin-top`. `styles.css:841` `.proposal .prompt-text` shows it was written for reuse across contexts. **The ONLY genuinely page-local code-block classes are Playground's** `.pg-tool-summary` / `.pg-tool-pre` (`Playground.css:130-148`).

LAST SENTENCE: "unstyled" is wrong, "could overflow" is right but container-dependent. A bare `<pre>` outside `.event` still inherits `styles.css:100-106` and the `* { box-sizing: border-box; margin: 0; padding: 0; }` reset (`styles.css:76-79`). It gets no `white-space`/`word-break`/overflow, so UA `white-space: pre` applies and a long line overflows its box. **Inside `.panel` (`overflow: hidden`, `styles.css:338`) the content is silently CLIPPED — no scrollbar, no visible overflow. Inside `.card` (no overflow rule, `styles.css:306`) it genuinely overflows and can push page-level horizontal scroll.**

ACTION: reuse `.prompt-text`, or promote `.prompt-text`/`.pg-tool-pre` into a neutrally-named shared class. There is no neutrally-named utility to reach for today.

### C5 — `layoutId` is a global namespace in this app; reusing the Sidebar nav-pill layoutId in a second simultaneously-mounted component would animate the pill between the two components

**VERDICT: CONFIRMED.** (No correction.)

The Sidebar pill uses a bare, unprefixed `layoutId="nav-pill"` (`src/components/Sidebar.tsx:108-113`). `grep -rn "LayoutGroup\|layoutGroup" console/src` returns zero hits; `AnimatePresence` consumes `LayoutGroupContext` read-only and never provides one. Namespacing is opt-in via `LayoutGroup` only (`framer-motion/dist/es/motion/index.mjs:77-82`; default context `createContext({})` → `.id` undefined → no prefix). The shared-node lookup table lives on the projection root, a module-level singleton document node — effectively document-global, spanning portals and unrelated subtrees (`motion-dom/.../create-projection-node.mjs:215,217,280,1254-1257`; `HTMLProjectionNode.mjs:4-19`). Two co-mounted nodes with the same layoutId join one `NodeStack`; the newly mounted one is promoted to lead and `resumeFrom`s the other's snapshot, firing a layout animation from the other component's box; the follower crossfades out (crossfade defaults true, `:748`; opacity handling `:1386-1400`). Neither Sidebar nor Tabs passes `layoutDependency`, so `prevDep === undefined` and `resumeFrom` is always taken.

The codebase already documents the hazard for the sibling component — `src/ui/Tabs.tsx:13`: `/** layoutId namespace — must be unique per Tabs instance on the page. */`, with `` layoutId={reduced ? undefined : `${id}-pill`} `` (`Tabs.tsx:36`, default `id = "tabs"` → `"tabs-pill"`). **Concrete collision vector: rendering `<Tabs id="nav" />` produces exactly `"nav-pill"` and collides with the Sidebar.**

OPERATIONAL CAVEAT (does not contradict the claim): the collision only exists when motion is not reduced. Under `prefers-reduced-motion` Sidebar renders a plain `<span className="nav-pill">` with no layoutId and Tabs sets `layoutId={undefined}`, so there is no shared stack and no cross-component animation. Fix pattern: wrap in `<LayoutGroup id="...">` or pass a per-instance id string.

### C6 — `Route.Component` is invoked with zero props, and `noUnusedParameters` means a page declaring an unused props parameter fails the build

**VERDICT: PARTIALLY_TRUE.** The premise is wrong; the consequence is right for the wrong reason; the conclusion is misleading in practice.

`route.Component` (`App.tsx:244-252`) is invoked with **FIVE** props — `adminKey`, `role`, `orgId`, `openSettings`, `navigate` — matching the exported `PageProps` (`App.tsx:45-51`). `tsconfig.json:10` does set `"noUnusedParameters": true`, and `npm run build` is `tsc && vite build` (enforced in CI at `.github/workflows/ci.yml:101-107` and `Makefile:14-15`), so a type error does fail the build.

The practical rules:

1. **A new page MUST still be typed `(props: PageProps)` or destructure from `PageProps`** — props are passed and available. Typing it `()` is legal but throws away `adminKey`/`role`/`navigate` and diverges from every existing page, all of which do `import type { PageProps } from "../App";`.
2. **`noUnusedParameters` does NOT fire on an underscore-prefixed name.** `export function NewPage(_props: PageProps) {` compiles clean — the existing convention in `Documents.tsx:23`, `Operators.tsx:49`, `Playground.tsx:75`, `Improve.tsx:66`.
3. **The flag also bites on destructured fields**: `function NewPage({ adminKey, role }: PageProps)` errors TS6133 on `role` if `role` is unused. Destructure only what you consume, or take `_props`.
4. **Scope caveat**: only `tsc` enforces this. `vite build` alone, `npm run dev`, and `vitest` are transpile-only and will NOT flag it — the failure surfaces at `npm run build` / CI.

### Source disagreements between the subsystem sweeps

These are citation-level conflicts. Both readings are given; trust the one with the verbatim excerpt attached.

- **`BrandMark` line range.** The icons sweep places `BrandMark` at `src/ui/icons.tsx:373-393` and quotes the full function body there; the motion sweep refers to "the `BrandMark` logomark SVG (`src/ui/icons.tsx:437-459`, four static children)". `:437-459` is `StateIcon`'s body per the icons sweep and the `StateIcon` verbatim excerpt. **Treat `:373-393` as correct for `BrandMark`.** Both agree it has four static children (one `<path d="M2.5 8h11" />` + three `<rect>`) and **no animation**.
- **`layoutId="nav-pill"` line number.** Routing sweep and the C5 verification both place it at `src/components/Sidebar.tsx:111` (C5 quotes `:108-113` with `layoutId` on the fourth line = `:111`); the motion sweep cites `src/components/Sidebar.tsx:103`. **The string and the component are not in dispute — only the line number.** Same for `motion.span` (`:100` per motion sweep, inside the `:108-113` block per C5) and `transition={transition}` (`:104`).
- **Reduced-motion block range in `styles.css`.** Cited as `:1108-1123` (primitives sweep), `:1108-1122` (motion sweep), and `:1109-1122` (landing sweep). The rule text is identical in all three; the block starts with the comment line `/* ---- Reduced motion: all non-essential animation off ---- */`.
- **`code, pre, .mono` rule range.** `styles.css:100-106` (primitives sweep) vs `:100-107` (page-idiom sweep). Same three declarations either way.
- **`PanelHead` location.** `src/ui/Card.tsx:57-70` (primitives sweep, interface + component) vs `:63` (page-idiom sweep, the function). Not a conflict — different anchors on the same block.
- **Should a test import `src/ui/index.ts`?** The testing sweep says do not (side-effect CSS import + framer-motion pull-in); the C3 verification ran exactly that import and it passed. **Both true: it works, but importing the narrowest module is the house convention.**

---

## TRAPS

Everything below silently breaks a build, silently breaks a test run, or produces a wrong-looking UI. Blunt, exhaustive, ordered roughly by how easy it is to hit.

### Build-breaking (tsc)

1. **`noUnusedParameters: true` (`tsconfig.json:10`) fails the build on an unused `props` parameter.** `export function DocsPage(props: PageProps)` with `props` unused → `error TS6133: 'props' is declared but its value is never read.` Use `_props` — the underscore is the escape hatch and the existing convention (`Documents.tsx:23`, `Operators.tsx:49`, `Playground.tsx:75`, `Improve.tsx:66`).
2. **Same flag fires on unused destructured fields.** `function DocsPage({ adminKey, role }: PageProps)` errors on `role` if `role` is unused. Destructure only what you consume.
3. **`noUnusedLocals: true` (`tsconfig.json:9`) fails the build on any unused const/import**, including in test files — verified: `const unusedThing = 1;` in `src/__tscprobe.test.ts` produced TS6133.
4. **`tsconfig.json:17` `include: ["src", "vite.config.ts"]` means test files are type-checked by `npm run build`.** A type error in a test breaks the production build even though the test passes.
5. **Only `tsc` enforces 1–4.** `npm run dev`, `vite build` alone, and `vitest` are transpile-only (esbuild). You will not see these errors until `npm run build` / CI / `make test-console`. Run `npx tsc` before claiming done — it produces zero output on a clean tree.
6. **Adding an `IconName` member without a `GLYPHS` entry is a tsc error** — `GLYPHS` is `Record<IconName, JSX.Element>` (`icons.tsx:53`), exhaustive. Conversely, using an icon name not in the union is also a tsc error. Both are good; just do them together.
7. **Adding a route without adding the icon name to `IconName` is a tsc error** (`Route.icon: IconName`, `App.tsx:56`).
8. **`isolatedModules: true` (`tsconfig.json:12`)** — type-only imports/re-exports must use `import type` or they can fail. `verbatimModuleSyntax` is NOT set, so it is not forced everywhere, but 78 existing sites use `import type`; follow that.
9. **Never write file extensions in import paths.** `grep` proves zero `.ts`/`.tsx`/`.js`/`.jsx` extensions in any import across `src`. `moduleResolution: "bundler"`. Only `.css` side-effect imports carry an extension.
10. **`import type { PageProps } from "../App";` must stay type-only.** A value import from `../App` into a page creates a real runtime cycle (`App.tsx` imports the page).
11. **No `lint` script exists.** No eslint/prettier/biome anywhere in the repo. Do not add lint-only fixes expecting a linter to catch anything; the `// eslint-disable-next-line` at `useCountUp.ts:82` is vestigial. `make fmt` only touches Go and Python.
12. **`jsx: "react-jsx"`** — do not add `import React from "react"` for JSX; it will trip `noUnusedLocals` if otherwise unused.
13. **Only three runtime dependencies exist: `framer-motion`, `react`, `react-dom`.** Adding any npm dependency for the visuals (charting libs, SVG libs, markdown renderers, syntax highlighters) breaks the project's shape. Everything must be hand-rolled or CSS.

### Test-run traps

14. **A `*.test.tsx` file is silently NOT collected.** `include: ["src/**/*.test.ts"]` (`vite.config.ts:23`) matches `.ts` only. Vitest reports `No test files found` even when handed the path explicitly. Your test will appear to "pass" by never running. **Name every test `*.test.ts`.**
15. **JSX syntax inside a `.test.ts` fails to transform** — esbuild picks its loader by extension and the `.ts` loader rejects JSX. Result: `Test Files 1 failed (1) / Tests no tests`. Use `React.createElement(...)` if you need elements.
16. **There is no DOM.** `document`, `window`, `requestAnimationFrame`, `localStorage`, `EventSource` are all `undefined` under `environment: "node"`. Global `fetch` IS defined. You cannot render, mount, or call a hook. No jsdom, no happy-dom, no @testing-library installed.
17. **`globals` is not enabled and `setupFiles` does not exist.** Every test must `import { describe, expect, it } from "vitest";` explicitly. `types: ["vite/client"]` is a closed list — `vitest/globals` is not in it.
18. **There is no `vitest.config.ts`** — do not create one expecting it to be picked up alongside `vite.config.ts`; the test config lives inside `vite.config.ts`.
19. **Do not design the page so its logic is only reachable through JSX.** Put anything you want tested (content tables, slug derivation, section lookup, SVG geometry math) in a plain `.ts` sibling — the `src/pages/overviewFeed.ts` + `overviewFeed.test.ts` precedent.
20. **Do not write a runtime test that "every section's icon name is a real glyph."** It is already enforced by `Record<IconName, JSX.Element>` + `tsc` in the build. Adding an `ICON_NAMES` export solely for that test is redundant work.
21. **`make test-console` runs BOTH** `vitest run --run` **and** `tsc && vite build`. Green tests alone are not done.

### Routing traps

22. **There is no router library.** No `react-router`, no `<Link>`, no `useParams`, no `useSearchParams`, no nested routes, no route params, no wildcards, no loaders. Anything that assumes one is wrong.
23. **Never pass a query string or hash to `navigate()`.** `navigate("/documents?s=gateway")` stores the full string in state, fails exact-match, and silently renders Overview while the URL bar shows `/documents?s=gateway`. Reloading that same URL renders Documents — **the same URL yields two different pages depending on how you arrived.** If the docs page needs deep-linkable sections, either normalise the matcher (`path.split(/[?#]/)[0]`) or manage section state internally without touching the URL.
24. **Trailing slashes do not match.** `/docs/` ≠ `/docs`. Falls back to Overview.
25. **An unknown path silently renders Overview with no URL rewrite and no warning** — and because `activePath={route.path}` is the *resolved* path, the sidebar lights Overview too, so the failure looks like a deliberate navigation. There is no 404 state to design against.
26. **Do not name the new route `/docs` expecting `/documents` to be unaffected — and do not confuse the two.** `/documents` is the existing RAG-corpus page. A `/docs` route does not exist yet; if you add one, the two paths are unrelated strings and both must be spelled exactly.
27. **A route change fully unmounts the page** (`key={route.path}` on the `AnimatePresence mode="wait"` wrapper, `App.tsx:288`). All page-local `useState` is lost on navigation and re-mounted fresh on return. Any animation that "continues" across navigation is impossible.
28. **`Route` is not exported** (`App.tsx:53`) — only `PageProps` is. Do not try to import the `Route` type.
29. **Registration is manual in exactly two places** (`App.tsx:32-43` import, `App.tsx:62-99` ROUTES). Forgetting the import is a tsc error; forgetting the ROUTES entry silently ships a page nobody can reach.
30. **Do not edit `CommandPalette.tsx` to add the route** — it derives entries from `visibleRoutes` automatically (`App.tsx:157-176`). Editing it produces a duplicate entry.
31. **If you add a `visible:` predicate, the page becomes unreachable for roles that fail it** — including via ⌘K, since `commands` is built from `visibleRoutes`. A docs page should almost certainly have **no** `visible` predicate (like the first 8 routes).

### Motion traps

32. **`layoutId` is document-global.** Reusing `"nav-pill"` — or producing it accidentally via `<Tabs id="nav" />`, which yields `` `${id}-pill` `` = `"nav-pill"` — makes the sidebar pill fly across the screen into your component. There is no `LayoutGroup` anywhere in the app to contain it. Namespace every `layoutId` you introduce.
33. **Under `prefers-reduced-motion` the layoutId collision disappears**, so you can ship the bug and never see it locally if you test with reduced motion on.
34. **The global CSS cap (`styles.css:1108-1123`) already neutralises every CSS transition/animation under reduced motion.** Do not re-implement that. But it does NOT cover framer-motion (JS-driven) animation — gate that with `useReducedMotion()`.
35. **The global cap does NOT zero `--dur-slow`, does NOT reset `animation-delay`, and does NOT reset `scroll-behavior`.** A long `animation-delay` or a `--dur-slow`-based animation still runs under reduced motion. Handle those yourself.
36. **`animation-iteration-count: 1 !important`** under reduced motion means an `infinite` pulse becomes a single 0.01ms cycle — it does not "freeze at a nice frame". If the reduced-motion still needs to look right, author it explicitly (the Chain and charts files all add their own `@media (prefers-reduced-motion: reduce)` blocks doing exactly that).
37. **JS durations and CSS duration tokens do not match**: `DUR_FAST = 0.12` / `DUR_MED = 0.2` (seconds) vs `--dur-fast: 110ms` / `--dur-med: 180ms`. The comments at `motion.ts:13,15` claiming 120/200 are stale. Mixing a CSS-animated element with a framer-motion-animated element and expecting them to land together will be off by 10–20ms.
38. **`motion.svg`, `motion.path`, `motion.circle`, `whileHover`, `whileTap`, `whileInView`, `useAnimate`, `useMotionValue`, `useTransform`, `useScroll`, `useSpring`, `MotionConfig`, `LayoutGroup`, `Reorder`, `drag` are NOT used anywhere in `src`.** The house technique for animated SVG is **CSS keyframes + `pathLength={1}` + `stroke-dasharray/dashoffset`** (`UsageChart.tsx:102` + `charts.css:34-59`). Introducing `motion.path` is a new pattern — justify it or follow the house one.
39. **SMIL (`<animate>`, `<animateTransform>`, `<animateMotion>`) DOES NOT EXIST in the codebase** and grep confirms zero occurrences. Do not introduce it.
40. **`requestAnimationFrame` is not used in `landing/` and is untestable in the console's node test env.** If you need per-frame work, it must be inside a component, cleaned up, and kept out of any `.test.ts`.
41. **Keyframe names are global.** All 14 CSS files are plain global stylesheets. Colliding with any of `live-pulse`, `chain-pulse`, `state-live-pulse`, `op-live-pulse`, `ov-pulse`, `pg-live-pulse`, `pg-live-pulse-amber`, `pg-shimmer`, `ui-shimmer`, `keys-meter-fill`, `audit-badge-flash`, `doc-drop-pulse`, `doc-progress-slide`, `chart-draw`, `chart-fade`, `chart-bar` silently changes another page's animation. Prefix yours.
42. **Class names are global too** — no CSS modules, no Tailwind, no CSS-in-JS. Prefix every class in your page CSS file. Unprefixed names like `.node`, `.stage`, `.rail`, `.fill`, `.core`, `.card`, `.panel`, `.label`, `.value`, `.empty`, `.mono`, `.dim`, `.num` will either collide or get clobbered.
43. **`AnimatePresence mode="wait"` at the shell level** means your page's enter animation starts only after the previous page's exit completes. Don't add a competing top-level `AnimatePresence` around the whole page.
44. **Intervals must be cleaned up.** React StrictMode double-mounts in dev (`main.tsx`), so any `setInterval` without a `return () => clearInterval(id)` runs twice and stacks on every hot reload. The landing page leaks all three of its intervals and gets away with it only because it never unmounts — **do not copy that.**
45. **The landing page's `window.__agentosOnSettled` global-callback trick does not port to React.** Its "define the real callback first, stub second" ordering depends on script order; React mount order gives you no equivalent. Use a prop, a callback, or one reducer.
46. **`installVisibilityPause(defaultRegistry)` only pauses live-resource polls**, not your hand-rolled animation timers. Four looping visuals will keep animating in a background tab unless you handle `document.hidden` yourself.
47. **Reduced motion must be read reactively.** The landing page's one-shot `matchMedia(...).matches` at IIFE time never reacts to a preference flip. Use `useReducedMotion()` — the console's 15-call-site convention.

### Visual / design-system traps

48. **There is no light theme, and adding one is a design violation.** One `:root` block, no `prefers-color-scheme` query, no `[data-theme]`, no toggle. `index.html:6` hard-declares `<meta name="color-scheme" content="dark" />`. Do not write `@media (prefers-color-scheme: light)` rules.
49. **`--accent` is `var(--text)` — ink, not a brand color** (`styles.css:53`). Using `var(--accent)` expecting a hue gives you plain foreground.
50. **Chroma is reserved for machine state.** `styles.css:7-12`: *"chroma is reserved for machine state… Do not add a brand color; it would make the signals lie."* `--live` / `--ok` / `--hold` / `--deny` mean *in flight / allowed / awaiting a human / denied*. Using teal, green, amber or red decoratively in the new visuals corrupts the console's signal vocabulary.
51. **`StateIcon` is the only mark allowed to carry color** (`icons.tsx` header comment on `StateIcon`), and it takes its hue from the `.state-<name>` class, not from props. There is no `className` prop on `StateIcon`.
52. **A bare `<pre>` gets font styling only** — no `white-space`, no `word-break`, no overflow handling. Inside `.panel` (`overflow: hidden`) long lines are **silently clipped with no scrollbar**; inside `.card` (no overflow rule) they **overflow and can force page-level horizontal scroll**. Use `<pre className="prompt-text mono">` or author a properly-scoped class.
53. **There is no `PanelBody` component — it DOES NOT EXIST.** Write `<div className="panel-body">` by hand. A `Table` goes directly inside `Panel` with no `panel-body` wrapper (it supplies its own padding); wrapping it double-pads.
54. **`PageHead` takes `{ title: string; subtitle?: string }` only.** `title` is `string`, not ReactNode. **There is no `actions` slot.** If you want a control in the header, it goes in a `PanelHead.actions` slot instead.
55. **`PanelHead.actions` is rendered raw as the second flex child** with no wrapper — multiple actions need your own `<div style={{ display: "flex", alignItems: "center", gap: 8 }}>` (the `Secrets.tsx:60` pattern).
56. **`Badge` has no `className` passthrough** and its `variant` is a closed 11-member union. You cannot style a badge from a page.
57. **`Tabs` has zero real usage in the app** — it is exported but unused by any page. If you use it for docs section switching you are the first; give it a unique `id` (see trap 32) and expect no precedent for its styling in context.
58. **`Icon` glyphs must not set `stroke`, `fill`, `width`, or `viewBox`** — the wrapper supplies `viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.25} strokeLinecap="square" strokeLinejoin="miter"`. A glyph that sets its own will look wrong at every size.
59. **Icon geometry must snap to whole or half units within the 2…14 band** (`icons.tsx:10-12`). Only five glyphs use off-grid decimals, all where a diagonal meets a curve, and the file names that tolerance explicitly. Arbitrary curves, `<g>`, `<line>`, `<polyline>`, `<polygon>`, transforms, ids and gradients are all outside the vocabulary.
60. **Arrowheads are open brackets, never filled wedges** (`icons.tsx:294-296`). A filled arrow breaks the line-only vocabulary.
61. **There is no book / manual / docs glyph — you must author one.** The closest neighbour is `documents` (two offset sheets with a corner fold at `M10.5 2.5v3h3`), so a new glyph **must differentiate on silhouette** (e.g. spine + two leaves), not on presence/absence of a fold. The house language for a reference motif is `audit`'s spine (`icons.tsx:75-82`, `:365-368`).
62. **`"deny"` exists in both `IconName` and `StateName` with different artwork.** `Icon name="deny"` is a bare cross at full 3…13 scale; `StateIcon state="deny"` is a barred ring. Picking the wrong one changes the meaning from "refuse this action" to "this was not permitted".
63. **`icons.tsx` is not in the `src/ui` barrel.** `import { Icon } from "../ui"` fails. Import from `"../ui/icons"`.
64. **Importing anything from `"../ui"` also injects `ui.css`** (`src/ui/index.ts:1`). Harmless in the app; noteworthy if you are minimising a module's side effects.
65. **The console `Chain` nodes are 7px squares with `border-radius: 1px`; the landing page's are 16px circles with `border-radius: 50%`.** Porting the landing markup verbatim produces a visual that contradicts the console's own instrument.
66. **`CHAIN_STAGES` is lowercase** (`["auth","rbac","budget","rate","audit"]`); the landing page's display labels are `Auth`/`RBAC`/`Budget`/`Rate`/`Audit`. Using the landing casing inside the console breaks `STAGE_LABELS`' identity-map convention and any string comparison against `ChainStage`.
67. **A clean pass leaves `.chain-fill` graphite, not green** (`Chain.css:39-40`: "a clean pass stays graphite, because nothing needs your attention"). A visual that lights the whole chain green on success contradicts the shipped instrument.
68. **`data-active` is present-or-absent (`active || undefined`), never `"false"`.** A CSS selector like `[data-active="false"]` never matches.
69. **The console must never draw a check it cannot prove ran** (`chain.ts:18-20`, `:44-47`, `:88-90`; `Chain.tsx:56-59`). If a new visual depicts governance, either drive it from evidence or make it unmistakably illustrative — an animated chain that always clears reads as a claim about the system.
70. **`.chain-label` is hidden under `max-width: 720px`** (`Chain.css:186-191`) — "labels drop before nodes do". Any responsive rule you write for a chain-like visual should follow the same order.
71. **Both the topbar and `.page` are capped at `max-width: 1080px`** (`styles.css:241-253`, `:275-279`), and `.main` has `padding: 0 40px 64px`. Full-bleed visuals are not a thing here.
72. **Fonts are self-hosted and CDN fonts are explicitly forbidden** (`src/fonts/fonts.css:1-4`). `--mono` is IBM Plex Mono, `--sans` is Archivo.

### Behavioural / runtime traps

73. **The shell polls two endpoints every 5 s on every route** (`admin/audit?limit=100` via Chain, `admin/usage` via Sidebar) plus 1 s re-render ticks from `useConnectionState()` and every `useLiveResource`. Your page re-renders at least once a second regardless. Anything expensive in the page body will be felt.
74. **If you add a `useLiveResource`, its `key` MUST encode the auth token** (`useLiveResource.ts:37-39`) — e.g. `` `docs/whatever#${adminKey}` `` — or data leaks across identities via the shared registry dedupe. `DEFAULT_CADENCE = 4000` if you omit `cadence`.
75. **A docs page with zero API calls would be the first in the codebase** — every one of the 12 existing pages imports `src/lib/api`. That is allowed, but there is no precedent to copy; model the structure on `Playground.tsx` (zero on-mount calls) minus the handlers.
76. **`useToast` throws `"useToast must be used within <ToastProvider>"`** if used outside the shell. It is fine inside a page (the provider wraps everything at `App.tsx:255`), but not in a module-level helper.
77. **`useLoad`'s deps array is spread with an internal `tick`** (`common.tsx:87`) — passing an unstable inline object/array in `deps` causes a refetch loop.
78. **Vite dev proxies `/api/gateway` → `localhost:8080` and `/api/runtime` → `localhost:18000`** (`vite.config.ts:10-19`), rewriting the prefix away. Hard-coded absolute API URLs bypass this and break in prod, where nginx does the same job.
79. **Deep links rely on `try_files $uri $uri/ /index.html`** (`console/nginx.conf.template:45-46`). A new route works on refresh in prod only because of that line — nothing to change, but nothing to rely on beyond it either.
80. **The pill CSS is layered**: `.nav-item.active { background: var(--accent-dim); }` in `styles.css:222-225` is deliberately overridden to transparent by `Sidebar.css:39-43` so the `.nav-pill` provides the surface. Adding a background to `.nav-item` re-introduces a double-highlight.
