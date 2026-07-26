# Console S1 — Liveness Contract & Pass Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the whole console one liveness contract — a shared, deduped resource layer that keeps every metric/list surface updating and animating on real change — replacing the fetch-once `useLoad` and the four hand-rolled `setInterval` pollers.

**Architecture:** One `useLiveResource` React hook reads from a module-level **registry** that owns timers and subscriptions, so N components reading the same endpoint share one poll and one in-flight request. The registry drives its data through a pluggable **transport** (a `pollTransport` now; an `sseTransport` can slot in later per-key with zero page edits). All time/status/dedup logic lives in pure or timer-injectable modules that are unit-tested in node; the hook and components are thin wrappers verified by typecheck + the existing suite + manual check.

**Tech Stack:** React 19, TypeScript 5.8, framer-motion 12, Vitest 3 (node env). No new dependencies.

## Global Constraints

Copied verbatim from `docs/superpowers/specs/2026-07-26-agentos-console-track-a-design.md`. Every task's requirements implicitly include these.

- **Chroma is reserved for machine state.** Only `--live #5ad1c4`, `--ok #6cc48f`, `--hold #e3a851`, `--deny #e2685f` may carry hue; `--accent` resolves to ink. New non-state UI is monochrome (`--text` / `--text-dim` / `--text-faint` / `--border`).
- **Motion is telemetry, not decoration.** Import all timings/variants from `src/ui/motion.ts` (`EASE`, `transition`, `transitionFast`, `staggerItem`, `staggerItemReduced`, …); never hardcode durations/easings.
- **Reduced motion honored on two layers:** the global CSS cap in `styles.css` *and* per-component `useReducedMotion()` swapping the `*Reduced` variant. Both must stay true.
- **Machine values are mono + tabular** (`--mono`, `font-variant-numeric: tabular-nums`); eyebrow labels follow the `.eyebrow` treatment. Radii from `--radius-sm/md/lg`.
- **Air-gap invariant:** no external requests, no new runtime deps, no CDN.
- **No new gateway/runtime endpoints.** All liveness is client-side over the existing `/admin/*` and `/api/runtime/*` endpoints.
- **Test env is node** (`vite.config.ts` → `test.environment: "node"`, `include: ["src/**/*.test.ts"]`). New unit tests are `.test.ts`, pure-function or fake-timer style (`describe/it/expect` from `vitest`). No DOM/render tests.
- Test command: `npm test` (`vitest run`). Build/typecheck: `npm run build` (`tsc && vite build`). Run both from `console/`.

## File Structure

New (`console/src/lib/live/`) — the contract, all pure or timer-injectable:
- `status.ts` — `ResourceStatus`, `deriveStatus()`, `reduceConnection()`. No deps.
- `relativeTime.ts` — `formatAgo()`. No deps.
- `transport.ts` — `Transport`, `TransportArgs`, `TransportFactory`, `TransportKind` (types only). No deps.
- `pollTransport.ts` — `pollTransport` factory (chained `setTimeout`). Imports `transport.ts`.
- `registry.ts` — `Snapshot`, `Registry`, `createRegistry()`, `defaultRegistry`, `installVisibilityPause()`. Imports `transport.ts`, `pollTransport.ts`, `status.ts`.

New (`console/src/hooks/`) — thin React wrappers:
- `useLiveResource.ts` — `useLiveResource()`, `useConnectionState()`.
- `useNowTick.ts` — `useNowTick(intervalMs)` shared re-render ticker.

New (`console/src/components/`):
- `Freshness.tsx` — `<Freshness updatedAt />`.
- `LiveList.tsx` — `<LiveList>` (AnimatePresence + staggerItem wrapper).

Modified:
- `pages/Overview.tsx` — cards + feed onto `useLiveResource` / `<LiveList>`.
- `components/Chain.tsx`, `components/Sidebar.tsx` — drop private timers, read shared registry.
- `App.tsx` — topbar connection indicator; install visibility-pause.
- `pages/Audit.tsx` — live streaming table.
- `pages/Operators.tsx` — live + next-run ETA (adds pure `operatorEta` in `lib/operators.ts`).
- `pages/Keys.tsx`, `pages/Multiverse.tsx`, `pages/Improve.tsx` — mechanical `useLoad → useLiveResource` swap.

---

### Task 1: Pure status derivation (`lib/live/status.ts`)

**Files:**
- Create: `console/src/lib/live/status.ts`
- Test: `console/src/lib/live/status.test.ts`

**Interfaces:**
- Produces: `type ResourceStatus = "idle" | "loading" | "live" | "stale" | "error"`; `const STALE_FACTOR = 1.5`; `interface StatusInput { updatedAt: number | null; lastErrorAt: number | null; now: number; cadence: number }`; `function deriveStatus(i: StatusInput): ResourceStatus`; `function reduceConnection(statuses: ResourceStatus[]): "idle" | "live" | "stale" | "offline"`.

- [ ] **Step 1: Write the failing test**

```ts
// console/src/lib/live/status.test.ts
import { describe, expect, it } from "vitest";
import { deriveStatus, reduceConnection, STALE_FACTOR } from "./status";

describe("deriveStatus", () => {
  const cadence = 4000;
  it("is loading before any success or error", () => {
    expect(deriveStatus({ updatedAt: null, lastErrorAt: null, now: 0, cadence })).toBe("loading");
  });
  it("is error when it never got data and last poll failed", () => {
    expect(deriveStatus({ updatedAt: null, lastErrorAt: 10, now: 10, cadence })).toBe("error");
  });
  it("is live with fresh data", () => {
    expect(deriveStatus({ updatedAt: 1000, lastErrorAt: null, now: 1000, cadence })).toBe("live");
  });
  it("is stale once data is older than cadence * STALE_FACTOR", () => {
    const now = 1000 + cadence * STALE_FACTOR + 1;
    expect(deriveStatus({ updatedAt: 1000, lastErrorAt: null, now, cadence })).toBe("stale");
  });
  it("is error when data is overdue AND the most recent poll failed", () => {
    const now = 1000 + cadence * STALE_FACTOR + 1;
    expect(deriveStatus({ updatedAt: 1000, lastErrorAt: now, now, cadence })).toBe("error");
  });
  it("holds live when a stale error is older than the last success", () => {
    expect(deriveStatus({ updatedAt: 2000, lastErrorAt: 1000, now: 2000, cadence })).toBe("live");
  });
});

describe("reduceConnection", () => {
  it("is idle when nothing is active", () => {
    expect(reduceConnection(["idle", "idle"])).toBe("idle");
  });
  it("is offline when every active resource errored", () => {
    expect(reduceConnection(["idle", "error", "error"])).toBe("offline");
  });
  it("is stale when any active resource is stale or errored but not all error", () => {
    expect(reduceConnection(["live", "stale"])).toBe("stale");
    expect(reduceConnection(["live", "error"])).toBe("stale");
  });
  it("is live when all active resources are live or loading", () => {
    expect(reduceConnection(["live", "loading", "idle"])).toBe("live");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd console && npx vitest run src/lib/live/status.test.ts`
Expected: FAIL — cannot find module `./status`.

- [ ] **Step 3: Write minimal implementation**

```ts
// console/src/lib/live/status.ts
/** Lifecycle of a single live resource, derived from timestamps (pure). */
export type ResourceStatus = "idle" | "loading" | "live" | "stale" | "error";

/** Data older than cadence * this reads as stale. */
export const STALE_FACTOR = 1.5;

export interface StatusInput {
  updatedAt: number | null; // ms epoch of last success, or null
  lastErrorAt: number | null; // ms epoch of last failure, or null
  now: number;
  cadence: number; // poll interval in ms
}

export function deriveStatus({ updatedAt, lastErrorAt, now, cadence }: StatusInput): ResourceStatus {
  if (updatedAt === null) return lastErrorAt === null ? "loading" : "error";
  const overdue = now - updatedAt > cadence * STALE_FACTOR;
  if (!overdue) return "live";
  // Overdue: an error more recent than the last success means we are actively failing.
  if (lastErrorAt !== null && lastErrorAt >= updatedAt) return "error";
  return "stale";
}

export type ConnectionState = "idle" | "live" | "stale" | "offline";

export function reduceConnection(statuses: ResourceStatus[]): ConnectionState {
  const active = statuses.filter((s) => s !== "idle");
  if (active.length === 0) return "idle";
  if (active.every((s) => s === "error")) return "offline";
  if (active.some((s) => s === "stale" || s === "error")) return "stale";
  return "live";
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd console && npx vitest run src/lib/live/status.test.ts`
Expected: PASS (all cases).

- [ ] **Step 5: Commit**

```bash
git add console/src/lib/live/status.ts console/src/lib/live/status.test.ts
git commit -m "feat(console): pure liveness status derivation"
```

---

### Task 2: Relative-time formatting (`lib/live/relativeTime.ts`)

**Files:**
- Create: `console/src/lib/live/relativeTime.ts`
- Test: `console/src/lib/live/relativeTime.test.ts`

**Interfaces:**
- Produces: `function formatAgo(deltaMs: number): string` — "just now" | "Ns ago" | "Nm ago" | "Nh ago" | "Nd ago".

- [ ] **Step 1: Write the failing test**

```ts
// console/src/lib/live/relativeTime.test.ts
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

- [ ] **Step 2: Run test to verify it fails**

Run: `cd console && npx vitest run src/lib/live/relativeTime.test.ts`
Expected: FAIL — cannot find module `./relativeTime`.

- [ ] **Step 3: Write minimal implementation**

```ts
// console/src/lib/live/relativeTime.ts
/** Compact "time since" for freshness labels. Pure; clock skew reads "just now". */
export function formatAgo(deltaMs: number): string {
  const s = Math.floor(deltaMs / 1000);
  if (s < 3) return "just now";
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd console && npx vitest run src/lib/live/relativeTime.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add console/src/lib/live/relativeTime.ts console/src/lib/live/relativeTime.test.ts
git commit -m "feat(console): relative-time freshness formatter"
```

---

### Task 3: Poll transport (`lib/live/transport.ts` + `lib/live/pollTransport.ts`)

**Files:**
- Create: `console/src/lib/live/transport.ts` (types only)
- Create: `console/src/lib/live/pollTransport.ts`
- Test: `console/src/lib/live/pollTransport.test.ts`

**Interfaces:**
- Produces: `interface Transport { stop(): void }`; `interface TransportArgs<T> { fetcher: () => Promise<T>; cadence: number; onData: (d: T) => void; onError: (e: unknown) => void }`; `type TransportFactory = <T>(args: TransportArgs<T>) => Transport`; `type TransportKind = "poll" | "sse"`; `const pollTransport: TransportFactory`.
- Consumes: nothing.

- [ ] **Step 1: Write the transport types (no test needed — pure interfaces)**

```ts
// console/src/lib/live/transport.ts
/** A running data source for one resource. Stopping it releases its timer/stream. */
export interface Transport {
  stop(): void;
}

export interface TransportArgs<T> {
  fetcher: () => Promise<T>;
  cadence: number; // ms
  onData: (data: T) => void;
  onError: (err: unknown) => void;
}

/** Builds a Transport. `pollTransport` now; an `sseTransport` can implement this later. */
export type TransportFactory = <T>(args: TransportArgs<T>) => Transport;

export type TransportKind = "poll" | "sse";
```

- [ ] **Step 2: Write the failing test**

```ts
// console/src/lib/live/pollTransport.test.ts
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { pollTransport } from "./pollTransport";

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe("pollTransport", () => {
  it("fetches immediately, then once per cadence", async () => {
    const values: number[] = [];
    const fetcher = vi.fn().mockResolvedValue(42);
    const t = pollTransport({ fetcher, cadence: 1000, onData: (v) => values.push(v), onError: () => {} });
    await vi.advanceTimersByTimeAsync(0); // flush immediate fetch
    expect(values).toEqual([42]);
    await vi.advanceTimersByTimeAsync(1000);
    expect(values).toEqual([42, 42]);
    t.stop();
  });

  it("routes rejections to onError and keeps polling", async () => {
    const errs: unknown[] = [];
    const fetcher = vi.fn().mockRejectedValue(new Error("boom"));
    const t = pollTransport({ fetcher, cadence: 1000, onData: () => {}, onError: (e) => errs.push(e) });
    await vi.advanceTimersByTimeAsync(0);
    expect(errs).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(1000);
    expect(errs).toHaveLength(2);
    t.stop();
  });

  it("stop() prevents further callbacks", async () => {
    const values: number[] = [];
    const fetcher = vi.fn().mockResolvedValue(1);
    const t = pollTransport({ fetcher, cadence: 1000, onData: (v) => values.push(v), onError: () => {} });
    await vi.advanceTimersByTimeAsync(0);
    t.stop();
    await vi.advanceTimersByTimeAsync(5000);
    expect(values).toEqual([1]); // no growth after stop
  });
});
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd console && npx vitest run src/lib/live/pollTransport.test.ts`
Expected: FAIL — cannot find module `./pollTransport`.

- [ ] **Step 4: Write minimal implementation**

```ts
// console/src/lib/live/pollTransport.ts
import type { TransportArgs, TransportFactory } from "./transport";

/**
 * Chained-setTimeout poller: fetch immediately, then schedule the next fetch
 * only after the current one settles, so a slow response never stacks requests.
 */
export const pollTransport: TransportFactory = <T>({
  fetcher,
  cadence,
  onData,
  onError,
}: TransportArgs<T>) => {
  let stopped = false;
  let timer: ReturnType<typeof setTimeout> | null = null;

  const tick = () => {
    fetcher()
      .then((d) => {
        if (!stopped) onData(d);
      })
      .catch((e) => {
        if (!stopped) onError(e);
      })
      .finally(() => {
        if (!stopped) timer = setTimeout(tick, cadence);
      });
  };

  tick();

  return {
    stop() {
      stopped = true;
      if (timer) clearTimeout(timer);
    },
  };
};
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd console && npx vitest run src/lib/live/pollTransport.test.ts`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add console/src/lib/live/transport.ts console/src/lib/live/pollTransport.ts console/src/lib/live/pollTransport.test.ts
git commit -m "feat(console): poll transport for the liveness registry"
```

---

### Task 4: The shared registry (`lib/live/registry.ts`)

**Files:**
- Create: `console/src/lib/live/registry.ts`
- Test: `console/src/lib/live/registry.test.ts`

**Interfaces:**
- Consumes: `Transport`, `TransportArgs`, `TransportFactory`, `TransportKind` (Task 3); `deriveStatus`, `reduceConnection`, `ConnectionState`, `ResourceStatus` (Task 1); `errorMessage` from `../api`.
- Produces:
  - `interface Snapshot<T> { data: T | null; error: string | null; updatedAt: number | null; lastErrorAt: number | null; transport: TransportKind }`
  - `interface Registry { subscribe<T>(key: string, fetcher: () => Promise<T>, cadence: number, onChange: () => void): () => void; getSnapshot<T>(key: string): Snapshot<T>; reload(key: string): void; pauseAll(): void; resumeAll(): void; connectionState(): ConnectionState }`
  - `function createRegistry(opts?: { transportFactory?: TransportFactory; now?: () => number }): Registry`
  - `const defaultRegistry: Registry`
  - `function installVisibilityPause(reg: Registry): () => void`

- [ ] **Step 1: Write the failing test**

```ts
// console/src/lib/live/registry.test.ts
import { describe, expect, it, vi } from "vitest";
import { createRegistry } from "./registry";
import type { Transport, TransportArgs } from "./transport";

/** A hand-driven transport so tests can push data/errors deterministically. */
function fakeFactory() {
  const created: Array<TransportArgs<unknown> & Transport & { stopped: boolean }> = [];
  const factory = <T,>(args: TransportArgs<T>): Transport => {
    const inst = {
      ...(args as TransportArgs<unknown>),
      stopped: false,
      stop() {
        this.stopped = true;
      },
    };
    created.push(inst);
    return inst;
  };
  return { factory, created };
}

describe("createRegistry", () => {
  it("shares one transport across subscribers to the same key and stops on last unsubscribe", () => {
    const { factory, created } = fakeFactory();
    const reg = createRegistry({ transportFactory: factory, now: () => 1000 });
    const a = vi.fn();
    const b = vi.fn();
    const unsubA = reg.subscribe("k", async () => 1, 4000, a);
    const unsubB = reg.subscribe("k", async () => 1, 4000, b);
    expect(created).toHaveLength(1);
    created[0].onData({ x: 1 });
    expect(a).toHaveBeenCalled();
    expect(b).toHaveBeenCalled();
    expect(reg.getSnapshot<{ x: number }>("k").data).toEqual({ x: 1 });
    unsubA();
    expect(created[0].stopped).toBe(false);
    unsubB();
    expect(created[0].stopped).toBe(true);
  });

  it("holds last-known data on error and stamps timestamps from now()", () => {
    let clock = 1000;
    const { factory, created } = fakeFactory();
    const reg = createRegistry({ transportFactory: factory, now: () => clock });
    reg.subscribe("k", async () => 1, 4000, () => {});
    created[0].onData({ x: 1 });
    clock = 5000;
    created[0].onError(new Error("boom"));
    const s = reg.getSnapshot<{ x: number }>("k");
    expect(s.data).toEqual({ x: 1 }); // held
    expect(s.error).toContain("boom");
    expect(s.updatedAt).toBe(1000); // unchanged by the error
    expect(s.lastErrorAt).toBe(5000);
  });

  it("returns a stable snapshot reference until something changes", () => {
    const { factory, created } = fakeFactory();
    const reg = createRegistry({ transportFactory: factory, now: () => 1 });
    reg.subscribe("k", async () => 1, 4000, () => {});
    const s1 = reg.getSnapshot("k");
    expect(reg.getSnapshot("k")).toBe(s1);
    created[0].onData({ x: 2 });
    expect(reg.getSnapshot("k")).not.toBe(s1);
  });

  it("derives connection state across keys", () => {
    let clock = 1000;
    const { factory, created } = fakeFactory();
    const reg = createRegistry({ transportFactory: factory, now: () => clock });
    reg.subscribe("k", async () => 1, 4000, () => {});
    created[0].onData({ x: 1 });
    expect(reg.connectionState()).toBe("live");
    clock = 1000 + 4000 * 1.5 + 1;
    expect(reg.connectionState()).toBe("stale");
  });

  it("pauseAll stops transports; resumeAll rebuilds them for live keys", () => {
    const { factory, created } = fakeFactory();
    const reg = createRegistry({ transportFactory: factory, now: () => 1 });
    reg.subscribe("k", async () => 1, 4000, () => {});
    reg.pauseAll();
    expect(created[0].stopped).toBe(true);
    reg.resumeAll();
    expect(created).toHaveLength(2);
    expect(created[1].stopped).toBe(false);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd console && npx vitest run src/lib/live/registry.test.ts`
Expected: FAIL — cannot find module `./registry`.

- [ ] **Step 3: Write minimal implementation**

```ts
// console/src/lib/live/registry.ts
import { errorMessage } from "../../components/common";
import { deriveStatus, reduceConnection } from "./status";
import type { ConnectionState } from "./status";
import { pollTransport } from "./pollTransport";
import type { Transport, TransportFactory, TransportKind } from "./transport";

export interface Snapshot<T> {
  data: T | null;
  error: string | null;
  updatedAt: number | null;
  lastErrorAt: number | null;
  transport: TransportKind;
}

interface Entry {
  fetcher: () => Promise<unknown>;
  cadence: number;
  subscribers: Set<() => void>;
  transport: Transport | null;
  data: unknown;
  error: string | null;
  updatedAt: number | null;
  lastErrorAt: number | null;
  kind: TransportKind;
  snapshot: Snapshot<unknown>;
}

export interface Registry {
  subscribe<T>(key: string, fetcher: () => Promise<T>, cadence: number, onChange: () => void): () => void;
  getSnapshot<T>(key: string): Snapshot<T>;
  reload(key: string): void;
  pauseAll(): void;
  resumeAll(): void;
  connectionState(): ConnectionState;
}

const IDLE: Snapshot<unknown> = {
  data: null,
  error: null,
  updatedAt: null,
  lastErrorAt: null,
  transport: "poll",
};

export function createRegistry(
  opts: { transportFactory?: TransportFactory; now?: () => number } = {},
): Registry {
  const makeTransport = opts.transportFactory ?? pollTransport;
  const now = opts.now ?? (() => Date.now());
  const entries = new Map<string, Entry>();

  function rebuildSnapshot(e: Entry) {
    e.snapshot = {
      data: e.data,
      error: e.error,
      updatedAt: e.updatedAt,
      lastErrorAt: e.lastErrorAt,
      transport: e.kind,
    };
  }

  function notify(e: Entry) {
    for (const cb of e.subscribers) cb();
  }

  function start(e: Entry) {
    e.transport = makeTransport({
      fetcher: e.fetcher,
      cadence: e.cadence,
      onData: (d) => {
        e.data = d;
        e.error = null;
        e.updatedAt = now();
        rebuildSnapshot(e);
        notify(e);
      },
      onError: (err) => {
        e.error = errorMessage(err);
        e.lastErrorAt = now();
        rebuildSnapshot(e); // data held; only error/lastErrorAt change
        notify(e);
      },
    });
  }

  return {
    subscribe(key, fetcher, cadence, onChange) {
      let e = entries.get(key);
      if (!e) {
        e = {
          fetcher: fetcher as () => Promise<unknown>,
          cadence,
          subscribers: new Set(),
          transport: null,
          data: null,
          error: null,
          updatedAt: null,
          lastErrorAt: null,
          kind: "poll",
          snapshot: { ...IDLE },
        };
        entries.set(key, e);
        start(e);
      }
      e.subscribers.add(onChange);
      return () => {
        const entry = entries.get(key);
        if (!entry) return;
        entry.subscribers.delete(onChange);
        if (entry.subscribers.size === 0) {
          entry.transport?.stop();
          entries.delete(key);
        }
      };
    },

    getSnapshot<T>(key: string): Snapshot<T> {
      const e = entries.get(key);
      return (e ? e.snapshot : IDLE) as Snapshot<T>;
    },

    reload(key) {
      const e = entries.get(key);
      if (!e) return;
      e.transport?.stop();
      start(e); // immediate refetch
    },

    pauseAll() {
      for (const e of entries.values()) {
        e.transport?.stop();
        e.transport = null;
      }
    },

    resumeAll() {
      for (const e of entries.values()) {
        if (!e.transport) start(e);
      }
    },

    connectionState() {
      const t = now();
      const statuses = [...entries.values()].map((e) =>
        deriveStatus({ updatedAt: e.updatedAt, lastErrorAt: e.lastErrorAt, now: t, cadence: e.cadence }),
      );
      return reduceConnection(statuses);
    },
  };
}

/** The console's single shared registry. */
export const defaultRegistry: Registry = createRegistry();

/**
 * Pause polling while the tab is hidden; refetch on return. Browser-only;
 * returns a teardown. No-op where `document` is absent (node tests never call it).
 */
export function installVisibilityPause(reg: Registry): () => void {
  if (typeof document === "undefined") return () => {};
  const onChange = () => (document.hidden ? reg.pauseAll() : reg.resumeAll());
  document.addEventListener("visibilitychange", onChange);
  return () => document.removeEventListener("visibilitychange", onChange);
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd console && npx vitest run src/lib/live/registry.test.ts`
Expected: PASS (all five cases).

- [ ] **Step 5: Run the full suite (no regressions)**

Run: `cd console && npm test`
Expected: PASS — existing tests plus the four new `lib/live` files.

- [ ] **Step 6: Commit**

```bash
git add console/src/lib/live/registry.ts console/src/lib/live/registry.test.ts
git commit -m "feat(console): shared liveness registry (dedupe, hold-on-error, pause)"
```

---

### Task 5: React hooks (`hooks/useLiveResource.ts`, `hooks/useNowTick.ts`)

**Files:**
- Create: `console/src/hooks/useNowTick.ts`
- Create: `console/src/hooks/useLiveResource.ts`

**Interfaces:**
- Consumes: `defaultRegistry`, `Snapshot` (Task 4); `deriveStatus`, `ResourceStatus`, `ConnectionState` (Task 1).
- Produces:
  - `function useNowTick(intervalMs: number): number`
  - `interface LiveResource<T> { data: T | null; error: string | null; status: ResourceStatus; updatedAt: number | null; transport: "poll" | "sse"; reload: () => void }`
  - `function useLiveResource<T>(key: string, fetcher: () => Promise<T>, opts?: { cadence?: number; enabled?: boolean }): LiveResource<T>`
  - `function useConnectionState(): ConnectionState`

This task is a thin wrapper over already-tested logic; there is no unit test (the node env renders no React). It is verified by typecheck + manual check in Task 8.

- [ ] **Step 1: Write `useNowTick`**

```ts
// console/src/hooks/useNowTick.ts
import { useEffect, useState } from "react";

/** Re-render every `intervalMs` and return the current time — for freshness/stale UI. */
export function useNowTick(intervalMs: number): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), intervalMs);
    return () => window.clearInterval(id);
  }, [intervalMs]);
  return now;
}
```

- [ ] **Step 2: Write `useLiveResource` + `useConnectionState`**

```ts
// console/src/hooks/useLiveResource.ts
import { useCallback, useSyncExternalStore } from "react";
import { defaultRegistry } from "../lib/live/registry";
import type { Registry, Snapshot } from "../lib/live/registry";
import { deriveStatus } from "../lib/live/status";
import type { ConnectionState, ResourceStatus } from "../lib/live/status";
import { useNowTick } from "./useNowTick";

/** Default poll cadence — matches the legacy Overview/Chain intervals. */
export const DEFAULT_CADENCE = 4000;

const IDLE_SNAPSHOT: Snapshot<unknown> = {
  data: null,
  error: null,
  updatedAt: null,
  lastErrorAt: null,
  transport: "poll",
};

export interface LiveResource<T> {
  data: T | null;
  error: string | null;
  status: ResourceStatus;
  updatedAt: number | null;
  transport: "poll" | "sse";
  reload: () => void;
}

export function useLiveResource<T>(
  key: string,
  fetcher: () => Promise<T>,
  opts: { cadence?: number; enabled?: boolean; registry?: Registry } = {},
): LiveResource<T> {
  const cadence = opts.cadence ?? DEFAULT_CADENCE;
  const enabled = opts.enabled ?? true;
  const reg = opts.registry ?? defaultRegistry;

  // Subscribe is stable per (key, cadence, enabled). The registry keys by
  // `key`, so `key` MUST encode everything that changes the fetch (endpoint +
  // auth token); the fetcher captured at first-subscribe is reused thereafter.
  const subscribe = useCallback(
    (onChange: () => void) => {
      if (!enabled) return () => {};
      return reg.subscribe(key, fetcher, cadence, onChange);
      // eslint-disable-next-line react-hooks/exhaustive-deps
    },
    [key, cadence, enabled, reg],
  );

  const snapshot = useSyncExternalStore<Snapshot<T>>(
    subscribe,
    () => (enabled ? reg.getSnapshot<T>(key) : (IDLE_SNAPSHOT as Snapshot<T>)),
    () => IDLE_SNAPSHOT as Snapshot<T>,
  );

  const now = useNowTick(1000);
  const status = enabled
    ? deriveStatus({
        updatedAt: snapshot.updatedAt,
        lastErrorAt: snapshot.lastErrorAt,
        now,
        cadence,
      })
    : "idle";

  return {
    data: snapshot.data,
    error: snapshot.error,
    status,
    updatedAt: snapshot.updatedAt,
    transport: snapshot.transport,
    reload: () => reg.reload(key),
  };
}

/** Shell-wide live/stale/offline signal, recomputed on a 1s tick. */
export function useConnectionState(registry: Registry = defaultRegistry): ConnectionState {
  useNowTick(1000);
  return registry.connectionState();
}
```

- [ ] **Step 3: Typecheck**

Run: `cd console && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add console/src/hooks/useNowTick.ts console/src/hooks/useLiveResource.ts
git commit -m "feat(console): useLiveResource + useConnectionState hooks"
```

---

### Task 6: `<Freshness>` component

**Files:**
- Create: `console/src/components/Freshness.tsx`

**Interfaces:**
- Consumes: `formatAgo` (Task 2), `useNowTick` (Task 5).
- Produces: `function Freshness(props: { updatedAt: number | null; className?: string }): JSX.Element | null`.

- [ ] **Step 1: Write the component**

```tsx
// console/src/components/Freshness.tsx
import { formatAgo } from "../lib/live/relativeTime";
import { useNowTick } from "../hooks/useNowTick";

/**
 * "updated Ns ago" for a panel. Mono + faint, recomputed every second.
 * Renders nothing until the first successful load stamps `updatedAt`.
 */
export function Freshness({ updatedAt, className }: { updatedAt: number | null; className?: string }) {
  const now = useNowTick(1000);
  if (updatedAt === null) return null;
  return (
    <span className={`freshness mono${className ? ` ${className}` : ""}`} aria-live="off">
      updated {formatAgo(now - updatedAt)}
    </span>
  );
}
```

- [ ] **Step 2: Add its style (monochrome, mono, faint)**

Append to `console/src/components/` styles — add to `console/src/styles.css` near other small utilities:

```css
/* Panel freshness label — faint, mono, no chroma (not a machine-state signal). */
.freshness {
  font-size: 10px;
  color: var(--text-faint);
  letter-spacing: 0.02em;
}
```

- [ ] **Step 3: Typecheck**

Run: `cd console && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add console/src/components/Freshness.tsx console/src/styles.css
git commit -m "feat(console): Freshness 'updated Ns ago' label"
```

---

### Task 7: `<LiveList>` component

**Files:**
- Create: `console/src/components/LiveList.tsx`

**Interfaces:**
- Consumes: framer-motion `AnimatePresence`, `motion`, `useReducedMotion`; `staggerItem`, `staggerItemReduced`, `transitionFast` from `../ui`.
- Produces: `function LiveList<T>(props: { items: readonly T[]; getKey: (item: T) => string; renderItem: (item: T) => React.ReactNode; as?: "ul" | "ol"; className?: string }): JSX.Element`. New items animate in at the top; removed items animate out. Standardizes the pattern currently inlined in `Overview.tsx`.

- [ ] **Step 1: Write the component**

```tsx
// console/src/components/LiveList.tsx
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import type { ReactNode } from "react";
import { staggerItem, staggerItemReduced, transitionFast } from "../ui";

/**
 * A list whose rows animate in/out as items enter/leave — the streaming-feed
 * motion from Overview, reusable across tables and feeds. Motion is telemetry:
 * a new row appearing = a real new event. Reduced-motion collapses to instant.
 */
export function LiveList<T>({
  items,
  getKey,
  renderItem,
  as = "ul",
  className,
}: {
  items: readonly T[];
  getKey: (item: T) => string;
  renderItem: (item: T) => ReactNode;
  as?: "ul" | "ol";
  className?: string;
}) {
  const reduced = useReducedMotion();
  const List = as === "ol" ? motion.ol : motion.ul;
  return (
    <List className={className} layout={!reduced}>
      <AnimatePresence initial={false}>
        {items.map((item) => (
          <motion.li
            key={getKey(item)}
            layout={!reduced}
            variants={reduced ? staggerItemReduced : staggerItem}
            initial="hidden"
            animate="show"
            exit={{ opacity: 0, transition: transitionFast }}
          >
            {renderItem(item)}
          </motion.li>
        ))}
      </AnimatePresence>
    </List>
  );
}
```

- [ ] **Step 2: Typecheck**

Run: `cd console && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add console/src/components/LiveList.tsx
git commit -m "feat(console): LiveList streaming row-in/out wrapper"
```

---

### Task 8: Migrate Overview to the liveness contract

**Files:**
- Modify: `console/src/pages/Overview.tsx`

**Interfaces:**
- Consumes: `useLiveResource` (Task 5), `LiveList` (Task 7), `Freshness` (Task 6), existing `mergeFeedEntries`/`feedEntryId`/`isInflight`/`budgetMeters` (`overviewFeed.ts`).

This is a wiring task; verification is typecheck + full suite (no regression) + manual check against the running stack.

- [ ] **Step 1: Replace the fetch-once `useLoad` cards with live resources**

In `Overview.tsx`, replace the two `useLoad` calls (usage + keys) and the bespoke `useActivity` hook with `useLiveResource`, keyed so the admin key is part of the key string:

```tsx
// Usage totals — was useLoad (fetch-once); now live.
const usageRes = useLiveResource<KeyUsage[]>(
  `admin/usage#${adminKey}`,
  () => apiFetch<KeyUsage[]>(gatewayAdminRequest("/admin/usage", adminKey)),
  { enabled: Boolean(adminKey), cadence: 5000 },
);
const usage = usageRes.data ?? [];

// Keys/budgets — degrade to empty on a role that can't list keys.
const keysRes = useLiveResource<KeyInfo[]>(
  `admin/keys#${adminKey}`,
  () => apiFetch<KeyInfo[]>(gatewayAdminRequest("/admin/keys", adminKey)).catch(() => [] as KeyInfo[]),
  { enabled: Boolean(adminKey), cadence: 5000 },
);

// Activity feed — the shared audit resource; keep the client-side merge/cap.
const auditRes = useLiveResource<AuditEntry[]>(
  `admin/audit?limit=${AUDIT_LIMIT}#${adminKey}`,
  () => apiFetch<AuditEntry[]>(gatewayAdminRequest(`/admin/audit?limit=${AUDIT_LIMIT}`, adminKey)),
  { enabled: Boolean(adminKey), cadence: POLL_MS },
);
```

The feed keeps its capped/deduped/sorted list by folding each poll through the existing pure helper — store it in a `useRef` + `useState` updated in a `useEffect` on `auditRes.data`:

```tsx
const [feedEntries, setFeedEntries] = useState<AuditEntry[]>([]);
useEffect(() => {
  if (auditRes.data) setFeedEntries((cur) => mergeFeedEntries(cur, auditRes.data!, AUDIT_LIMIT));
}, [auditRes.data]);
```

- [ ] **Step 2: Render the feed through `<LiveList>` and add `<Freshness>`**

Replace the inline `<AnimatePresence>`/`motion.li` block with `<LiveList items={feed} getKey={feedEntryId} renderItem={...} />`, and add `<Freshness updatedAt={auditRes.updatedAt} />` into the Activity `PanelHead` actions. The stat `<Stat value={totals.requests} …>` cards are unchanged — `useCountUp` inside `Stat` now animates because `value` actually changes between polls.

- [ ] **Step 3: Typecheck + full suite**

Run: `cd console && npx tsc --noEmit && npm test`
Expected: no type errors; all tests pass (the pure `overviewFeed` tests still cover the merge).

- [ ] **Step 4: Manual verification**

Build and open the console against the running stack; send traffic through the gateway (e.g. a Playground run). Confirm: the Requests/Tokens/Spend cards **count up on each poll** without a reload, the feed streams new rows at the top, and "updated Ns ago" ticks. Confirm the Network tab shows **one** `/admin/audit` poll shared with the Chain/Sidebar (Task 9), not three.

- [ ] **Step 5: Commit**

```bash
git add console/src/pages/Overview.tsx
git commit -m "feat(console): Overview cards + feed on the liveness contract"
```

---

### Task 9: Move Chain + Sidebar onto the shared registry

**Files:**
- Modify: `console/src/components/Chain.tsx`
- Modify: `console/src/components/Sidebar.tsx`

**Interfaces:**
- Consumes: `useLiveResource` (Task 5); existing `latestChainState` (`lib/chain.ts`); existing `KeyUsage`/`AuditEntry` types.

Wiring task; verification is typecheck + suite + manual (the pure `chain.ts` logic is already tested).

- [ ] **Step 1: Replace Chain's private `setInterval` with the shared audit resource**

In `Chain.tsx`, delete the `useEffect` poll loop and read the same key Overview uses:

```tsx
const audit = useLiveResource<AuditEntry[]>(
  `admin/audit?limit=20#${adminKey}`,
  () => apiFetch<AuditEntry[]>(gatewayAdminRequest("/admin/audit?limit=20", adminKey)),
  { enabled: Boolean(adminKey) },
);
// Derive chain state from the shared data; keep the existing "grew since last
// poll → light for LINGER_MS" behavior using a ref on audit.data.length.
```

Keep `latestChainState`, `stageRenders`, `outcomeGlyph` exactly as-is; only the data source changes.

- [ ] **Step 2: Replace Sidebar's `useRunsInFlight` interval with the shared usage resource**

In `Sidebar.tsx`, replace the `useRunsInFlight` interval with a read of `admin/usage#${adminKey}`, computing the live-dot from growth in total request count across polls (same logic, shared data).

- [ ] **Step 3: Typecheck + suite + manual**

Run: `cd console && npx tsc --noEmit && npm test`
Manual: confirm the Chain still lights on real traffic and the brand live-dot still pulses, and the Network tab shows the audit/usage endpoints polled **once** each (shared), not per-component.

- [ ] **Step 4: Commit**

```bash
git add console/src/components/Chain.tsx console/src/components/Sidebar.tsx
git commit -m "refactor(console): Chain + Sidebar share the liveness registry"
```

---

### Task 10: Shell connection indicator + visibility-pause

**Files:**
- Modify: `console/src/App.tsx`

**Interfaces:**
- Consumes: `useConnectionState` (Task 5), `installVisibilityPause` + `defaultRegistry` (Task 4), existing `StateIcon` (`ui/icons.tsx`).

- [ ] **Step 1: Install visibility-pause once**

In `App.tsx`, add an effect:

```tsx
useEffect(() => installVisibilityPause(defaultRegistry), []);
```

- [ ] **Step 2: Render the connection state in the topbar**

Beside the existing `governance chain` legend in `header.topbar`, add a small indicator mapping connection state → `StateIcon` + label, honoring the chroma rule (live→`live`, stale→`hold`, offline→`deny`, idle→no chroma):

```tsx
const conn = useConnectionState();
// live → StateIcon "live" "synced"; stale → "hold" "reconnecting"; offline → "deny" "offline"; idle → nothing.
```

- [ ] **Step 3: Typecheck + suite + manual**

Run: `cd console && npx tsc --noEmit && npm test`
Manual: confirm the topbar shows "synced" on live traffic; stop the gateway and confirm it flips to "offline" (deny), then recovers; switch tabs away and back and confirm polling pauses/resumes (Network tab quiets while hidden).

- [ ] **Step 4: Commit**

```bash
git add console/src/App.tsx
git commit -m "feat(console): shell connection indicator + visibility-pause"
```

---

### Task 11: Audit page — live streaming table

**Files:**
- Modify: `console/src/pages/Audit.tsx`

**Interfaces:**
- Consumes: `useLiveResource` (Task 5), `LiveList` (Task 7), `Freshness` (Task 6), existing `mergeFeedEntries`/`feedEntryId` (`overviewFeed.ts`).

- [ ] **Step 1: Swap `useLoad` for `useLiveResource` and stream rows**

Replace the fetch-once table load with the shared `admin/audit?limit=100#${adminKey}` resource, fold polls through `mergeFeedEntries` (already tested) to keep a capped, deduped, newest-first list, and render rows through `<LiveList as="ol">` (or keep the `<table>` but wrap `<Tbody>` with the staggered entrance). Add a **live/paused** toggle that flips `enabled`/`reload`, and `<Freshness updatedAt={res.updatedAt} />` in the `PanelHead` actions beside the existing Refresh button (Refresh → `res.reload`).

- [ ] **Step 2: Typecheck + suite + manual**

Run: `cd console && npx tsc --noEmit && npm test`
Manual: with traffic flowing, confirm new audit rows slide in at the top without a manual refresh; toggling "paused" halts the stream; Refresh forces an immediate fetch.

- [ ] **Step 3: Commit**

```bash
git add console/src/pages/Audit.tsx
git commit -m "feat(console): live streaming Audit table"
```

---

### Task 12: Operators — live state + next-run ETA

**Files:**
- Modify: `console/src/lib/operators.ts` (add pure `operatorEta`)
- Test: `console/src/lib/operators.test.ts` (extend)
- Modify: `console/src/pages/Operators.tsx`

**Interfaces:**
- Produces: `function operatorEta(op: { trigger: OperatorTrigger; last_fired_at: string | null }, now: number): number | null` — ms until the next interval fire, or `null` for non-interval/unknown triggers.
- Consumes: `useLiveResource` (Task 5), `useNowTick` (Task 5), `formatAgo` pattern.

- [ ] **Step 1: Write the failing test for `operatorEta`**

```ts
// add to console/src/lib/operators.test.ts
import { operatorEta } from "./operators";

describe("operatorEta", () => {
  const now = Date.parse("2026-07-26T10:00:00Z");
  it("returns ms remaining for an interval operator that has fired", () => {
    const op = {
      trigger: { type: "interval" as const, interval_s: 300 },
      last_fired_at: "2026-07-26T09:56:00Z", // 240s ago → 60s left
    };
    expect(operatorEta(op, now)).toBe(60_000);
  });
  it("returns 0 when overdue", () => {
    const op = {
      trigger: { type: "interval" as const, interval_s: 60 },
      last_fired_at: "2026-07-26T09:56:00Z",
    };
    expect(operatorEta(op, now)).toBe(0);
  });
  it("returns null for cron/webhook or missing last_fired_at", () => {
    expect(operatorEta({ trigger: { type: "cron", cron: "* * * * *" }, last_fired_at: null }, now)).toBeNull();
    expect(operatorEta({ trigger: { type: "interval", interval_s: 60 }, last_fired_at: null }, now)).toBeNull();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd console && npx vitest run src/lib/operators.test.ts`
Expected: FAIL — `operatorEta` is not exported.

- [ ] **Step 3: Implement `operatorEta`**

```ts
// add to console/src/lib/operators.ts
import type { OperatorTrigger } from "./types"; // if not already imported

/** ms until an interval operator's next fire (0 if overdue); null when not derivable. */
export function operatorEta(
  op: { trigger: OperatorTrigger; last_fired_at: string | null },
  now: number,
): number | null {
  if (op.trigger.type !== "interval" || !op.trigger.interval_s || !op.last_fired_at) return null;
  const last = Date.parse(op.last_fired_at);
  if (Number.isNaN(last)) return null;
  const next = last + op.trigger.interval_s * 1000;
  return Math.max(0, next - now);
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd console && npx vitest run src/lib/operators.test.ts`
Expected: PASS.

- [ ] **Step 5: Wire Operators live + render the ETA**

In `Operators.tsx`, replace the `useLoad`/`setInterval` operator-list load with `useLiveResource<{ operators: Operator[] }>("operators#…", listOperatorsRequest-based fetcher, { cadence: 4000 })`; render a ticking "next run in Ns" per interval operator using `operatorEta(op, useNowTick(1000))` → `formatAgo`-style seconds, and pulse a running row with the `--live` treatment.

- [ ] **Step 6: Typecheck + suite + manual**

Run: `cd console && npx tsc --noEmit && npm test`
Manual: confirm the operators list updates live and interval operators show a counting-down "next run in Ns".

- [ ] **Step 7: Commit**

```bash
git add console/src/lib/operators.ts console/src/lib/operators.test.ts console/src/pages/Operators.tsx
git commit -m "feat(console): live Operators list + next-run ETA"
```

---

### Task 13: Remaining poll surfaces — Keys, Multiverse, Improve

**Files:**
- Modify: `console/src/pages/Keys.tsx`, `console/src/pages/Multiverse.tsx`, `console/src/pages/Improve.tsx`

Each is the identical mechanical swap: `useLoad(loader, [adminKey])` → `useLiveResource(key, loader, { enabled, cadence })`, plus a `<Freshness>` in the panel head. Concrete per-page keys/cadences (no "similar to" — spelled out):

- **Keys** — `` `admin/keys#${adminKey}` ``, cadence 5000, fetcher `apiFetch<KeyInfo[]>(gatewayAdminRequest("/admin/keys", adminKey))`. Spend/budget meters re-animate on change; the created-key panel flow is untouched.
- **Multiverse** — `` `council/objectives#${adminKey}` `` and `` `council/proposals#${adminKey}` `` (two resources), cadence 4000, fetchers from `listObjectivesRequest()` / `listProposalsRequest()`. A run in flight now updates live.
- **Improve** — `` `evals/runs?limit=20` ``, cadence 5000, fetcher `apiFetch<EvalRunSummary[]>(runtimeRequest("/evals/runs?limit=20"))`. A running eval updates live.

- [ ] **Step 1: Apply the swap on all three pages** (keep every render/format detail; only the data hook changes).

- [ ] **Step 2: Typecheck + suite + manual**

Run: `cd console && npx tsc --noEmit && npm test`
Manual: confirm each page refreshes without a manual reload and shows "updated Ns ago".

- [ ] **Step 3: Commit**

```bash
git add console/src/pages/Keys.tsx console/src/pages/Multiverse.tsx console/src/pages/Improve.tsx
git commit -m "feat(console): Keys/Multiverse/Improve on the liveness contract"
```

---

### Task 14: Final verification

- [ ] **Step 1: Full suite + build**

Run: `cd console && npm test && npm run build`
Expected: all tests pass; `tsc && vite build` completes with no type errors.

- [ ] **Step 2: Shared-poll audit**

Manual: open the console with the stack running, watch the Network tab. Confirm `/admin/audit` and `/admin/usage` are each polled **once per cadence total** (shared across Overview + Chain + Sidebar), that hiding the tab pauses all polling, and refocus triggers an immediate refetch.

- [ ] **Step 3: Reduced-motion pass**

Manual: enable OS "reduce motion". Confirm cards snap (no count-up), rows appear without slide, and the connection indicator/freshness still update — no motion, but still live.

- [ ] **Step 4: Commit any final touch-ups, then the suite is green.**

```bash
git add -A
git commit -m "chore(console): S1 liveness pass final verification"
```

---

## Out of scope for this plan (S1)

- **Optimistic-create + freshness on the pure CRUD pages** (Documents, Orgs, Users, Secrets, Provisioning). These re-poll fine via a trivial `useLoad → useLiveResource` swap, but true optimistic mutation (row appears instantly on create, reconciles on next fetch) is a distinct pattern touching each page's create flow; it is deferred to a short S1-follow-up plan so S1 stays focused on the contract + the live/streaming surfaces.
- **SSE transport** (`sseTransport` / `/admin/stream`). The `TransportFactory` seam and the `transport: "poll" | "sse"` snapshot field exist so this drops in later per-key with zero page edits — explicitly not built here.

## Self-Review

- **Spec coverage:** §2 liveness contract → Tasks 1–5 (status, relative-time, transport, registry, hooks); §2.2 connection state → Tasks 1/4/10; §2.3 motion primitives → `useCountUp` reused (Task 8), `<LiveList>` (Task 7), `<Freshness>` (Task 6); §3 surface-by-surface → Overview (8), Chain/Sidebar (9), Audit (11), Operators (12), Keys/Multiverse/Improve (13); optimistic-CRUD row of the §3 table → explicitly deferred with rationale (Out of scope). Visibility-pause (§2.1) → Tasks 4/10.
- **Placeholder scan:** every code step contains real code; every migration task carries explicit keys/cadences/fetchers rather than "similar to". No TBD/TODO.
- **Type consistency:** `Snapshot`, `Transport`/`TransportArgs`/`TransportFactory`/`TransportKind`, `ResourceStatus`, `deriveStatus`/`reduceConnection`, `useLiveResource`/`LiveResource`/`useConnectionState`, `operatorEta` names/signatures match across producing and consuming tasks. `defaultRegistry`/`installVisibilityPause` used in Tasks 5/10 as defined in Task 4.
