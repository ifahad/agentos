# Console S3 — Production Data Tooling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the console's ~12 fetch-once tables production data tooling — search, filter, sort, client pagination, and CSV/JSON export — via one reusable client-side pipeline; wire the two built-but-unrendered charts (`UsageChart`, `SpendBreakdown`); and add a ⌘K command palette.

**Architecture:** A pure, composable pipeline in `lib/table/` (`filter → search → sort → paginate`) with a `computeTableView` entry point and a `lib/export.ts` (`toCSV`/`downloadBlob`); a thin `useTableView` hook + `<TableToolbar>` + `<SortableTh>` over the existing `Table/Tbody/Tr` primitives; and a `<CommandPalette>` reusing the `Modal` portal + window-keydown pattern. Everything is client-side over already-loaded arrays — no new endpoints.

**Tech Stack:** React 19, TypeScript 5.8, framer-motion 12, Vitest 3 (node env). No new dependencies.

## Global Constraints

From `docs/superpowers/specs/2026-07-26-agentos-console-track-a-design.md` §5 + design invariants.

- **No new endpoints / deps.** All tooling is client-side over the arrays pages already load (audit is `?limit=100`, keys/orgs/secrets/users are full lists).
- **Chroma reserved for machine state.** Toolbar controls, sort arrows, charts-at-rest are monochrome (`--text`/`--text-dim`/`--text-faint`/`--border`); only state carries `--live/--ok/--hold/--deny`. The S3 action glyphs (`search`/`filter`/`sort`/`export`) already exist from S2 — reuse them.
- **Reuse primitives:** `Table/Tbody/Tr` (there is NO `Th` — pages write `<th>` with `className="num"` for numerics); `ui/Field` `Input`/`Select`; `ui/Button` (now with `icon`); `ui/Modal` portal + its window-keydown pattern; `charts/transforms.ts` (`seriesFromEvents`/`normalizeSeries`/`breakdownFromRows`, already tested); the `PanelHead` `actions` slot as the toolbar host; `Tbody` `staggerKey` (feed sort state so a re-sort replays the stagger).
- **Motion** imported from `ui/motion.ts`, reduced-motion honored. Mono+tabular for values.
- **Test env is node** (`.test.ts`, pure/`describe/it/expect`). The pipeline + export are pure → TDD. React (hook/toolbar/SortableTh/palette) + wiring verified by `tsc --noEmit` + full suite (no regression) + `npm run build` + a manual pass. Test: `npm test`. Build: `npm run build`. Run from `console/`.

## File Structure

New pure (`console/src/lib/table/`): `search.ts`, `sort.ts`, `filter.ts`, `paginate.ts`, `view.ts` (+ co-located `.test.ts`). New `console/src/lib/export.ts` (+ test). New React: `console/src/hooks/useTableView.ts`, `console/src/components/TableToolbar.tsx`, `console/src/components/SortableTh.tsx`, `console/src/components/CommandPalette.tsx`. Modify: `pages/Audit.tsx` (flagship), `pages/Overview.tsx` (SpendBreakdown), `pages/Keys.tsx`/`pages/Orgs.tsx` (adopt toolbar), `App.tsx` (mount palette + ⌘K).

---

### Task 1: Search + sort (pure)

**Files:** Create `console/src/lib/table/search.ts`, `sort.ts`; Test `search.test.ts`, `sort.test.ts`.

**Interfaces:**
- `filterBySearch<T>(rows: readonly T[], fields: readonly (keyof T)[], query: string): T[]` — case-insensitive substring over the chosen fields; empty/whitespace query → all rows (copied).
- `type SortDir = "asc" | "desc"`; `interface SortSpec<T> { key: keyof T; dir: SortDir; numeric?: boolean }`; `sortRows<T>(rows: readonly T[], spec: SortSpec<T> | null): T[]` — stable; `null` → shallow copy; `numeric` compares as numbers, else `localeCompare` on `String(value)`.

- [ ] **Step 1: Write the failing tests**

```ts
// console/src/lib/table/search.test.ts
import { describe, expect, it } from "vitest";
import { filterBySearch } from "./search";
type Row = { name: string; model: string; n: number };
const rows: Row[] = [
  { name: "prod", model: "gpt-4o", n: 3 },
  { name: "dev", model: "claude", n: 10 },
];
describe("filterBySearch", () => {
  it("returns all rows (copied) for an empty/whitespace query", () => {
    expect(filterBySearch(rows, ["name"], "")).toEqual(rows);
    expect(filterBySearch(rows, ["name"], "  ")).toEqual(rows);
    expect(filterBySearch(rows, ["name"], "")).not.toBe(rows);
  });
  it("matches a case-insensitive substring in any chosen field", () => {
    expect(filterBySearch(rows, ["name", "model"], "CLA").map((r) => r.name)).toEqual(["dev"]);
    expect(filterBySearch(rows, ["model"], "gpt").map((r) => r.name)).toEqual(["prod"]);
  });
  it("coerces non-string fields to string for matching", () => {
    expect(filterBySearch(rows, ["n"], "10").map((r) => r.name)).toEqual(["dev"]);
  });
  it("returns [] when nothing matches", () => {
    expect(filterBySearch(rows, ["name"], "zzz")).toEqual([]);
  });
});
```

```ts
// console/src/lib/table/sort.test.ts
import { describe, expect, it } from "vitest";
import { sortRows } from "./sort";
type Row = { name: string; n: number };
const rows: Row[] = [
  { name: "b", n: 2 },
  { name: "a", n: 10 },
  { name: "c", n: 2 },
];
describe("sortRows", () => {
  it("returns a shallow copy for a null spec", () => {
    const out = sortRows(rows, null);
    expect(out).toEqual(rows);
    expect(out).not.toBe(rows);
  });
  it("sorts strings with localeCompare, both directions", () => {
    expect(sortRows(rows, { key: "name", dir: "asc" }).map((r) => r.name)).toEqual(["a", "b", "c"]);
    expect(sortRows(rows, { key: "name", dir: "desc" }).map((r) => r.name)).toEqual(["c", "b", "a"]);
  });
  it("sorts numerically when numeric is set (not lexically)", () => {
    expect(sortRows(rows, { key: "n", dir: "asc", numeric: true }).map((r) => r.n)).toEqual([2, 2, 10]);
    expect(sortRows(rows, { key: "n", dir: "desc", numeric: true }).map((r) => r.n)).toEqual([10, 2, 2]);
  });
  it("is stable for equal keys (preserves input order)", () => {
    const asc = sortRows(rows, { key: "n", dir: "asc", numeric: true });
    expect(asc.filter((r) => r.n === 2).map((r) => r.name)).toEqual(["b", "c"]);
  });
});
```

- [ ] **Step 2: Run to verify they fail** — `cd console && npx vitest run src/lib/table/search.test.ts src/lib/table/sort.test.ts` → FAIL (missing modules).

- [ ] **Step 3: Implement**

```ts
// console/src/lib/table/search.ts
/** Case-insensitive substring filter over chosen fields. Empty query → all rows (copied). */
export function filterBySearch<T>(rows: readonly T[], fields: readonly (keyof T)[], query: string): T[] {
  const q = query.trim().toLowerCase();
  if (q === "") return rows.slice();
  return rows.filter((row) =>
    fields.some((f) => String(row[f] ?? "").toLowerCase().includes(q)),
  );
}
```

```ts
// console/src/lib/table/sort.ts
export type SortDir = "asc" | "desc";
export interface SortSpec<T> {
  key: keyof T;
  dir: SortDir;
  /** Compare as numbers rather than by locale string order. */
  numeric?: boolean;
}

/** Stable sort by one column. `null` → a shallow copy (original order). */
export function sortRows<T>(rows: readonly T[], spec: SortSpec<T> | null): T[] {
  const out = rows.slice();
  if (!spec) return out;
  const sign = spec.dir === "desc" ? -1 : 1;
  out.sort((a, b) => {
    const av = a[spec.key];
    const bv = b[spec.key];
    let cmp: number;
    if (spec.numeric) {
      cmp = (Number(av) || 0) - (Number(bv) || 0);
    } else {
      cmp = String(av ?? "").localeCompare(String(bv ?? ""));
    }
    return cmp * sign;
  });
  return out;
}
```

- [ ] **Step 4: Run to verify they pass** — same command → PASS.

- [ ] **Step 5: Commit**

```bash
git add console/src/lib/table/search.ts console/src/lib/table/search.test.ts console/src/lib/table/sort.ts console/src/lib/table/sort.test.ts
git commit -m "feat(console): pure table search + sort"
```

---

### Task 2: Facet filter + paginate + computeTableView (pure)

**Files:** Create `console/src/lib/table/filter.ts`, `paginate.ts`, `view.ts`; Test `filter.test.ts`, `view.test.ts`.

**Interfaces:**
- `type FacetSelection<T> = Partial<Record<keyof T, string>>`; `filterByFacets<T>(rows, facets): T[]` — for each facet whose value is a non-empty string, keep rows where `String(row[field]) === value`.
- `interface Page<T> { rows: T[]; page: number; pageCount: number }`; `paginate<T>(rows, page, size): Page<T>` — `size <= 0` → one page of all rows; `page` clamped to `[0, pageCount-1]`.
- `interface TableViewConfig<T> { searchFields?: readonly (keyof T)[]; query?: string; facets?: FacetSelection<T>; sort?: SortSpec<T> | null; page?: number; pageSize?: number }`; `interface TableView<T> { rows: T[]; filtered: T[]; total: number; filteredCount: number; page: number; pageCount: number }`; `computeTableView<T>(rows, config): TableView<T>` — pipeline `filterByFacets → filterBySearch → sortRows → paginate`; `filtered` is the sorted result *before* paginate (what export uses); `rows` is the visible page.

- [ ] **Step 1: Write the failing tests**

```ts
// console/src/lib/table/filter.test.ts
import { describe, expect, it } from "vitest";
import { filterByFacets } from "./filter";
import { paginate } from "./paginate";
type Row = { kind: string; status: number };
const rows: Row[] = [
  { kind: "chat", status: 200 },
  { kind: "chat", status: 500 },
  { kind: "embeddings", status: 200 },
];
describe("filterByFacets", () => {
  it("ignores facets with an empty value (all rows)", () => {
    expect(filterByFacets(rows, { kind: "" })).toEqual(rows);
    expect(filterByFacets(rows, {})).toEqual(rows);
  });
  it("keeps rows matching every active facet (string-equality)", () => {
    expect(filterByFacets(rows, { kind: "chat" })).toHaveLength(2);
    expect(filterByFacets(rows, { kind: "chat", status: "200" })).toHaveLength(1);
  });
});
describe("paginate", () => {
  it("returns one page of all rows when size <= 0", () => {
    const p = paginate(rows, 0, 0);
    expect(p.rows).toHaveLength(3);
    expect(p.pageCount).toBe(1);
  });
  it("slices the requested page and clamps out-of-range pages", () => {
    expect(paginate(rows, 0, 2).rows.map((r) => r.status)).toEqual([200, 500]);
    expect(paginate(rows, 1, 2).rows.map((r) => r.status)).toEqual([200]);
    expect(paginate(rows, 9, 2).page).toBe(1); // clamped to last page
  });
});
```

```ts
// console/src/lib/table/view.test.ts
import { describe, expect, it } from "vitest";
import { computeTableView } from "./view";
type Row = { name: string; kind: string; n: number };
const rows: Row[] = [
  { name: "prod", kind: "chat", n: 3 },
  { name: "dev", kind: "chat", n: 10 },
  { name: "stg", kind: "embeddings", n: 1 },
];
describe("computeTableView", () => {
  it("composes filter → search → sort → paginate and reports counts", () => {
    const v = computeTableView(rows, {
      facets: { kind: "chat" },
      searchFields: ["name"],
      query: "",
      sort: { key: "n", dir: "desc", numeric: true },
      page: 0,
      pageSize: 1,
    });
    expect(v.total).toBe(3);
    expect(v.filteredCount).toBe(2);         // kind=chat
    expect(v.filtered.map((r) => r.n)).toEqual([10, 3]); // sorted, pre-paginate
    expect(v.rows.map((r) => r.n)).toEqual([10]);        // page 0, size 1
    expect(v.pageCount).toBe(2);
  });
  it("no config → all rows, single page", () => {
    const v = computeTableView(rows, {});
    expect(v.rows).toHaveLength(3);
    expect(v.pageCount).toBe(1);
  });
});
```

- [ ] **Step 2: Run to verify fail** — `cd console && npx vitest run src/lib/table/filter.test.ts src/lib/table/view.test.ts` → FAIL.

- [ ] **Step 3: Implement**

```ts
// console/src/lib/table/filter.ts
export type FacetSelection<T> = Partial<Record<keyof T, string>>;

/** Keep rows matching every facet with a non-empty value (string-equality). */
export function filterByFacets<T>(rows: readonly T[], facets: FacetSelection<T>): T[] {
  const active = (Object.entries(facets) as [keyof T, string | undefined][]).filter(
    ([, v]) => v !== undefined && v !== "",
  );
  if (active.length === 0) return rows.slice();
  return rows.filter((row) => active.every(([k, v]) => String(row[k] ?? "") === v));
}
```

```ts
// console/src/lib/table/paginate.ts
export interface Page<T> {
  rows: T[];
  page: number;
  pageCount: number;
}
/** Slice one page. size <= 0 → a single page of everything. page clamped in range. */
export function paginate<T>(rows: readonly T[], page: number, size: number): Page<T> {
  if (size <= 0) return { rows: rows.slice(), page: 0, pageCount: 1 };
  const pageCount = Math.max(1, Math.ceil(rows.length / size));
  const p = Math.min(Math.max(0, Math.floor(page)), pageCount - 1);
  return { rows: rows.slice(p * size, p * size + size), page: p, pageCount };
}
```

```ts
// console/src/lib/table/view.ts
import { filterByFacets } from "./filter";
import type { FacetSelection } from "./filter";
import { filterBySearch } from "./search";
import { paginate } from "./paginate";
import { sortRows } from "./sort";
import type { SortSpec } from "./sort";

export interface TableViewConfig<T> {
  searchFields?: readonly (keyof T)[];
  query?: string;
  facets?: FacetSelection<T>;
  sort?: SortSpec<T> | null;
  page?: number;
  /** 0 (or omitted) = no pagination. */
  pageSize?: number;
}

export interface TableView<T> {
  /** The visible page after the full pipeline. */
  rows: T[];
  /** Filtered + sorted, BEFORE pagination — what export uses. */
  filtered: T[];
  total: number;
  filteredCount: number;
  page: number;
  pageCount: number;
}

/** filter → search → sort → paginate, as pure data. */
export function computeTableView<T>(rows: readonly T[], config: TableViewConfig<T>): TableView<T> {
  const faceted = filterByFacets(rows, config.facets ?? {});
  const searched =
    config.searchFields && config.query
      ? filterBySearch(faceted, config.searchFields, config.query)
      : faceted;
  const sorted = sortRows(searched, config.sort ?? null);
  const paged = paginate(sorted, config.page ?? 0, config.pageSize ?? 0);
  return {
    rows: paged.rows,
    filtered: sorted,
    total: rows.length,
    filteredCount: sorted.length,
    page: paged.page,
    pageCount: paged.pageCount,
  };
}
```

- [ ] **Step 4: Run to verify pass** → PASS. Then the full suite: `npm test` (green).

- [ ] **Step 5: Commit**

```bash
git add console/src/lib/table/filter.ts console/src/lib/table/filter.test.ts console/src/lib/table/paginate.ts console/src/lib/table/view.ts console/src/lib/table/view.test.ts
git commit -m "feat(console): facet filter, paginate, composed table view"
```

---

### Task 3: CSV/JSON export (pure toCSV + thin downloadBlob)

**Files:** Create `console/src/lib/export.ts`; Test `console/src/lib/export.test.ts`.

**Interfaces:**
- `interface Column<T> { header: string; value: (row: T) => string | number }`
- `toCSV<T>(rows, columns): string` — header row + one row per record; cells containing `"`, `,`, `\r`, or `\n` are wrapped in double-quotes with internal `"` doubled; rows joined by `\r\n`.
- `toJSON<T>(rows, columns): string` — `JSON.stringify` of `rows.map(r => Object.fromEntries(columns.map(c => [c.header, c.value(r)])))`, pretty (2-space).
- `downloadBlob(filename, mime, text): void` — browser only (`Blob` + `URL.createObjectURL` + synthetic `<a>.click()` + `revokeObjectURL`). Not unit-tested (DOM); guarded by `typeof document`.

- [ ] **Step 1: Write the failing test**

```ts
// console/src/lib/export.test.ts
import { describe, expect, it } from "vitest";
import { toCSV, toJSON } from "./export";
type Row = { name: string; note: string; n: number };
const cols = [
  { header: "Name", value: (r: Row) => r.name },
  { header: "Note", value: (r: Row) => r.note },
  { header: "Count", value: (r: Row) => r.n },
];
describe("toCSV", () => {
  it("writes a header row then one row per record (CRLF-joined)", () => {
    const csv = toCSV([{ name: "prod", note: "ok", n: 3 }], cols);
    expect(csv).toBe("Name,Note,Count\r\nprod,ok,3");
  });
  it("quotes cells with commas, quotes, or newlines and doubles internal quotes", () => {
    const csv = toCSV([{ name: 'a,b', note: 'he said "hi"', n: 1 }], cols);
    expect(csv).toBe('Name,Note,Count\r\n"a,b","he said ""hi""",1');
  });
  it("handles empty rows (header only)", () => {
    expect(toCSV([], cols)).toBe("Name,Note,Count");
  });
});
describe("toJSON", () => {
  it("maps each row to a header-keyed object", () => {
    expect(JSON.parse(toJSON([{ name: "p", note: "n", n: 2 }], cols))).toEqual([
      { Name: "p", Note: "n", Count: 2 },
    ]);
  });
});
```

- [ ] **Step 2: Run to verify fail** — `cd console && npx vitest run src/lib/export.test.ts` → FAIL.

- [ ] **Step 3: Implement**

```ts
// console/src/lib/export.ts
export interface Column<T> {
  header: string;
  value: (row: T) => string | number;
}

function csvCell(raw: string | number): string {
  const s = String(raw);
  return /[",\r\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

/** RFC-4180-ish CSV: header + rows, CRLF-joined, minimal quoting. */
export function toCSV<T>(rows: readonly T[], columns: readonly Column<T>[]): string {
  const head = columns.map((c) => csvCell(c.header)).join(",");
  const body = rows.map((row) => columns.map((c) => csvCell(c.value(row))).join(","));
  return [head, ...body].join("\r\n");
}

/** Pretty JSON array of header-keyed objects. */
export function toJSON<T>(rows: readonly T[], columns: readonly Column<T>[]): string {
  return JSON.stringify(
    rows.map((row) => Object.fromEntries(columns.map((c) => [c.header, c.value(row)]))),
    null,
    2,
  );
}

/** Trigger a client-side download. Browser-only; no-op where document is absent. */
export function downloadBlob(filename: string, mime: string, text: string): void {
  if (typeof document === "undefined") return;
  const url = URL.createObjectURL(new Blob([text], { type: mime }));
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}
```

- [ ] **Step 4: Run to verify pass** → PASS.

- [ ] **Step 5: Commit**

```bash
git add console/src/lib/export.ts console/src/lib/export.test.ts
git commit -m "feat(console): CSV/JSON export helpers"
```

---

### Task 4: `useTableView` hook + `<SortableTh>` + `<TableToolbar>`

**Files:** Create `console/src/hooks/useTableView.ts`, `console/src/components/SortableTh.tsx`, `console/src/components/TableToolbar.tsx`; add CSS to `console/src/styles.css`.

**Interfaces:**
- `useTableView<T>(rows, opts: { searchFields?; facetKeys?; initialSort?; pageSize? }) → { view: TableView<T>; query; setQuery; facets; setFacet; sort; toggleSort; page; setPage }`. Wraps `computeTableView` with `useState` for query/facets/sort/page; recomputed via `useMemo`. `toggleSort(key, numeric)` cycles asc→desc→(same key) and resets `page` to 0 on any filter/search/sort change.
- `<SortableTh label sortKey active dir numeric onSort className?>` — a `<th>` containing a button with `label` + the `sort` glyph; calls `onSort(sortKey, numeric)`; shows dir via an arrow/rotation on the glyph when `active`.
- `<TableToolbar>` — renders (into a `PanelHead actions` slot): a `<SearchInput>` (Field `Input` `type="search"` + `search` glyph) bound to `query/setQuery`; zero-or-more facet `Select`s bound to `facets/setFacet`; and "Export CSV"/"Export JSON" `Button`s (`icon="export"`) that call a passed `onExport(format)`.

This task has no unit test (React, node env); verify by `tsc --noEmit` + full suite (no regression) + build. Provide the concrete component code:

- [ ] **Step 1: `useTableView` hook**

```ts
// console/src/hooks/useTableView.ts
import { useMemo, useState } from "react";
import { computeTableView } from "../lib/table/view";
import type { TableView } from "../lib/table/view";
import type { FacetSelection } from "../lib/table/filter";
import type { SortDir, SortSpec } from "../lib/table/sort";

export interface UseTableViewOpts<T> {
  searchFields?: readonly (keyof T)[];
  initialSort?: SortSpec<T> | null;
  pageSize?: number;
}

export function useTableView<T>(rows: readonly T[], opts: UseTableViewOpts<T> = {}) {
  const [query, setQuery] = useState("");
  const [facets, setFacets] = useState<FacetSelection<T>>({});
  const [sort, setSort] = useState<SortSpec<T> | null>(opts.initialSort ?? null);
  const [page, setPage] = useState(0);

  const view: TableView<T> = useMemo(
    () =>
      computeTableView(rows, {
        searchFields: opts.searchFields,
        query,
        facets,
        sort,
        page,
        pageSize: opts.pageSize ?? 0,
      }),
    [rows, opts.searchFields, opts.pageSize, query, facets, sort, page],
  );

  function setFacet(key: keyof T, value: string) {
    setFacets((f) => ({ ...f, [key]: value }));
    setPage(0);
  }
  function toggleSort(key: keyof T, numeric?: boolean) {
    setSort((cur) => {
      const dir: SortDir = cur && cur.key === key && cur.dir === "asc" ? "desc" : "asc";
      return { key, dir, numeric };
    });
    setPage(0);
  }
  function changeQuery(q: string) {
    setQuery(q);
    setPage(0);
  }

  return { view, query, setQuery: changeQuery, facets, setFacet, sort, toggleSort, page, setPage };
}
```

- [ ] **Step 2: `<SortableTh>`**

```tsx
// console/src/components/SortableTh.tsx
import { Icon } from "../ui/icons";

export function SortableTh<T>({
  label, sortKey, active, dir, numeric, onSort, className,
}: {
  label: string;
  sortKey: keyof T;
  active: boolean;
  dir: "asc" | "desc";
  numeric?: boolean;
  onSort: (key: keyof T, numeric?: boolean) => void;
  className?: string;
}) {
  return (
    <th className={className} aria-sort={active ? (dir === "asc" ? "ascending" : "descending") : "none"}>
      <button type="button" className={`th-sort${active ? " active" : ""}`} onClick={() => onSort(sortKey, numeric)}>
        {label}
        <Icon name="sort" size={12} className={`th-sort-icon${active ? ` dir-${dir}` : ""}`} />
      </button>
    </th>
  );
}
```

Add to `styles.css`: `.th-sort` (transparent button, inherits header type, `cursor:pointer`, faint `sort` icon that goes full-ink when `.active`; `.dir-desc .th-sort-icon` rotates 180deg; gate rotation off under `prefers-reduced-motion` if animated).

- [ ] **Step 3: `<TableToolbar>`**

```tsx
// console/src/components/TableToolbar.tsx
import type { ReactNode } from "react";
import { Button } from "../ui";
import { Input, Select } from "../ui/Field";
import { Icon } from "../ui/icons";

export interface FacetDef { key: string; label: string; options: readonly string[]; value: string; onChange: (v: string) => void; }

export function TableToolbar({
  query, onQuery, facets = [], onExport, extra,
}: {
  query: string;
  onQuery: (q: string) => void;
  facets?: readonly FacetDef[];
  onExport?: (format: "csv" | "json") => void;
  extra?: ReactNode;
}) {
  return (
    <div className="table-toolbar head-group">
      <span className="tt-search">
        <Icon name="search" size={13} className="tt-search-icon" />
        <Input type="search" placeholder="Search" value={query} onChange={(e) => onQuery(e.target.value)} aria-label="Search table" />
      </span>
      {facets.map((f) => (
        <Select key={f.key} value={f.value} onChange={(e) => f.onChange(e.target.value)} aria-label={f.label}>
          <option value="">All {f.label}</option>
          {f.options.map((o) => <option key={o} value={o}>{o}</option>)}
        </Select>
      ))}
      {extra}
      {onExport && (
        <>
          <Button small icon="export" onClick={() => onExport("csv")}>CSV</Button>
          <Button small icon="export" onClick={() => onExport("json")}>JSON</Button>
        </>
      )}
    </div>
  );
}
```

Add `.table-toolbar` / `.tt-search` CSS (flex, gap, the search icon sitting inside the input's left padding), monochrome.

- [ ] **Step 4: Typecheck + suite + build** — `cd console && npx tsc --noEmit && npm test && npm run build` (clean; 231+ green incl. the new pure tests; build ok).

- [ ] **Step 5: Commit**

```bash
git add console/src/hooks/useTableView.ts console/src/components/SortableTh.tsx console/src/components/TableToolbar.tsx console/src/styles.css
git commit -m "feat(console): useTableView hook, SortableTh, TableToolbar"
```

---

### Task 5: Wire the Audit page (flagship) + UsageChart

**Files:** Modify `console/src/pages/Audit.tsx`.

Audit already streams live (S1). Layer the toolkit on the live `rows`:
- `const t = useTableView(rows, { searchFields: ["key_name", "model"], pageSize: 25, initialSort: null });` — pass `t.view.rows` to the table body; key sort state into `Tbody` `staggerKey`.
- Toolbar in the `PanelHead actions` (grouped with the existing Live/Paused + Refresh + `<Freshness>`): search over key/model; a `kind` facet Select (options from `AuditKind`: chat/embeddings/guardrail_flag/guardrail_block); export via `onExport` → build `Column<AuditEntry>[]` (Time/Key/Model/Kind/Tokens/Cost/Latency/Status) then `downloadBlob("audit.csv","text/csv",toCSV(t.view.filtered, cols))` (export the FILTERED view, not just the page) / `toJSON`.
- Headers become `<SortableTh>` for sortable columns (Time desc default is fine; Key/Model/Cost/Latency/Status sortable, `numeric` on the numeric ones), driven by `t.sort`/`t.toggleSort`.
- A minimal pager under the table when `t.view.pageCount > 1` (Prev/Next + "page X of Y"), calling `t.setPage`.
- **UsageChart:** a Panel above the Events table — `seriesFromEvents(rows.map(e => ({ timestamp: e.ts, value: e.cost_usd })), { bucket: "day" })`, map `.value` into `values`, `xLabels` from bucket `t` timestamps (first/last), `formatValue={formatUSD}`, `label="Spend per day"`.

Verify: `tsc --noEmit` + `npm test` (231+ green) + build. Commit `feat(console): search/filter/sort/export + usage chart on Audit`.

---

### Task 6: Overview SpendBreakdown + adopt toolbar on Keys & Orgs

**Files:** Modify `console/src/pages/Overview.tsx`, `console/src/pages/Keys.tsx`, `console/src/pages/Orgs.tsx`.

- **Overview:** render `<SpendBreakdown rows={breakdownFromRows(usage, { label: (u) => u.name, value: (u) => u.spend_usd })} label="Spend by key" formatValue={formatUSD} />` inside the existing `ov-grid` (a new Panel titled "Spend by key").
- **Keys:** `useTableView(keys, { searchFields: ["name"], initialSort: { key: "name", dir: "asc" } })`; toolbar with search + CSV/JSON export (columns Name/Monthly budget/Spend/Budget used); `<SortableTh>` on Name/Spend/Budget. No pager (small list).
- **Orgs:** same pattern — search over `name`/`id`; export columns Name/Id/Monthly budget/Aggregate spend/Rate limit; sortable Name/Spend.

Verify: `tsc --noEmit` + `npm test` + build. Commit `feat(console): SpendBreakdown on Overview; toolbar on Keys & Orgs`.

---

### Task 7: `<CommandPalette>` + ⌘K

**Files:** Create `console/src/components/CommandPalette.tsx`; Modify `console/src/App.tsx`.

- `<CommandPalette open onClose commands navigate>` — reuse the `Modal` portal pattern (or a lean portal of its own) with a search `Input`, a filtered list of commands (substring over `command.label` using `filterBySearch`), ↑/↓ to move a highlighted index, Enter to run, Esc to close. Each command = `{ id, label, icon?, run: () => void }`.
- Command source in `App.tsx`: map `visibleRoutes` (already role-filtered) → `{ id: r.path, label: `Go to ${r.label}`, icon: r.icon, run: () => { navigate(r.path); close(); } }`, plus `{ id: "settings", label: "Open Settings", icon: "settings", run: openSettings }`.
- Mount `<CommandPalette>` beside `<SettingsModal>` at App level. Add the FIRST global hotkey: a `useEffect` window `keydown` — `if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") { e.preventDefault(); setPaletteOpen(true); }`.

Verify: `tsc --noEmit` + `npm test` + build + manual (⌘K opens; typing filters; Enter navigates; a viewer never sees admin-only routes; Esc closes). Commit `feat(console): Cmd-K command palette`.

---

### Task 8: Final verification

- [ ] `cd console && npm test && npm run build` → all green; `tsc && vite build` clean.
- [ ] Export sanity: exported CSV opens in a spreadsheet; export reflects the current filtered+sorted view (not just the page).
- [ ] Reduced-motion: sort re-stagger, toolbar, palette all respect it.
- [ ] Chroma: toolbar/sort/charts monochrome; grep the new components for stray `--live/--ok/--hold/--deny`.

---

## Self-Review

- **Spec coverage (§5):** search/sort/filter/paginate/compose → Tasks 1–2 (pure, TDD); export → Task 3; hook+toolbar+SortableTh → Task 4; Audit flagship (all four + UsageChart) → Task 5; SpendBreakdown + more tables → Task 6; ⌘K palette over visibleRoutes → Task 7. Reuses the S2 `search`/`filter`/`sort`/`export` glyphs. No new endpoints (all client-side over loaded arrays).
- **Placeholder scan:** pure modules + tests are concrete; React components carry full code; wiring tasks name exact fields/columns/facets. No TBD.
- **Type consistency:** `SortSpec`/`SortDir`/`FacetSelection`/`TableView`/`TableViewConfig`/`Column`/`computeTableView`/`useTableView`/`toCSV`/`downloadBlob` names are consistent across producing (Tasks 1–4) and consuming (Tasks 5–7) tasks.
- **Honest verification:** pure pipeline + export are TDD'd; React/wiring gated on tsc + full suite + build + a manual pass (export open, palette keyboard, reduced-motion) — no fake render tests in the node env.
