# Governance Chain Overhaul Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the console's inert hairline governance chain with three switchable animated canvas renderers (phosphor, corridor, flow) plus a retained `minimal` style, driven entirely by real audit evidence on the existing 5s poll.

**Architecture:** A pure replay layer (`lib/chainReplay.ts`) diffs successive audit-feed snapshots into `ChainPacket`s scheduled at their real `ts` offsets. A canvas host (`ChainCanvas.tsx`) owns the rAF loop, DPR sizing, visibility pausing and reduced-motion handling, and hands each frame to one `ChainRenderer` implementation. `lib/chain.ts` — the evidence model — is not modified. Renderers receive a `ChainPalette` resolved from CSS custom properties so themes work and the chroma partition guard stays meaningful.

**Tech Stack:** React 19, TypeScript 5.8, Vite 6, vitest 3 (`environment: "node"`), framer-motion 12, Canvas 2D.

## Global Constraints

- **Working directory is `/home/iofahd/code/agentos/console`.** All paths below are relative to it. Branch: `governance-chain-overhaul`.
- **`src/lib/chain.ts` MUST NOT be modified.** It is the evidence model. Import from it; never change its semantics.
- **Vitest is `environment: "node"`** — no `document`, `window`, `localStorage`, or canvas in tests. Test files must be `.ts` (config includes `src/**/*.test.ts` only; `.tsx` tests are NOT collected and will silently not run).
- **No color literals in renderer code.** All colour comes from the injected `ChainPalette`. `styles.partition.test.ts` guards CSS instrument surfaces against brand violet; canvas code is invisible to it, so the palette is the chokepoint that keeps that guarantee.
- **`src/components/Chain.css` must continue to exist** — `styles.partition.test.ts:45` lists it in `INSTRUMENT_FILES` and asserts its presence.
- **Accessibility must not regress.** The semantic `<ol className="chain-stages">` and the `aria-live="polite"` outcome glyph stay. Canvas is `aria-hidden`.
- **Poll cadence stays 5000ms** with resource key `admin/audit?limit=100#${adminKey}` — the registry dedupes Chain and Overview onto one poll; changing either diverges them and doubles admin API load.
- Run tests with `npm test -- --run`. Full gate: `npm test -- --run && npm run build`.
- Commit after every task. Conventional-commit prefixes, matching existing history (`feat(console):`, `fix(console):`, `test(console):`).

---

### Task 1: Pure replay layer

**Files:**
- Create: `src/lib/chainReplay.ts`
- Test: `src/lib/chainReplay.test.ts`

**Interfaces:**
- Consumes: `AuditEntry` from `src/lib/types.ts`; `ChainStage`, `ChainOutcome`, `CHAIN_STAGES`, `chainStateFromEntry` from `src/lib/chain.ts`.
- Produces: `ChainPacket`, `rowKey(entry)`, `diffFeed(prev, next)`, `toPacket(entry, id)`, `releaseSchedule(packets, windowMs)`, `MAX_REPLAY_PER_POLL`.

- [ ] **Step 1: Write the failing test**

Create `src/lib/chainReplay.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { AuditEntry } from "./types";
import {
  MAX_REPLAY_PER_POLL,
  diffFeed,
  releaseSchedule,
  rowKey,
  toPacket,
} from "./chainReplay";

/** An audit row. `kind` is widened because the gateway writes seven kinds. */
function row(over: Partial<AuditEntry> & { ts: string }): AuditEntry {
  return {
    key_name: "k1",
    model: "llama3",
    input_tokens: 10,
    output_tokens: 20,
    cost_usd: 0.001,
    latency_ms: 300,
    status: 200,
    kind: "chat",
    ...over,
  } as AuditEntry;
}

describe("rowKey", () => {
  it("distinguishes rows differing in any single field", () => {
    const base = row({ ts: "2026-08-03T10:00:00Z" });
    expect(rowKey(base)).toBe(rowKey(row({ ts: "2026-08-03T10:00:00Z" })));
    expect(rowKey(base)).not.toBe(rowKey(row({ ts: "2026-08-03T10:00:01Z" })));
    expect(rowKey(base)).not.toBe(rowKey(row({ ts: "2026-08-03T10:00:00Z", status: 429 })));
    expect(rowKey(base)).not.toBe(rowKey(row({ ts: "2026-08-03T10:00:00Z", latency_ms: 301 })));
  });
});

describe("diffFeed", () => {
  const a = row({ ts: "2026-08-03T10:00:03Z" });
  const b = row({ ts: "2026-08-03T10:00:02Z" });
  const c = row({ ts: "2026-08-03T10:00:01Z" });

  it("seeds without replaying history on first snapshot", () => {
    expect(diffFeed(null, [a, b, c])).toEqual([]);
  });

  it("returns only rows above the anchor, oldest first", () => {
    // prev head was c; a and b are new. Feed is newest-first.
    expect(diffFeed([c], [a, b, c])).toEqual([b, a]);
  });

  it("returns nothing when the feed has not moved", () => {
    expect(diffFeed([a, b, c], [a, b, c])).toEqual([]);
  });

  it("re-seeds rather than replaying when the anchor is gone", () => {
    const fresh = row({ ts: "2026-08-03T11:00:00Z" });
    expect(diffFeed([c], [fresh])).toEqual([]);
  });

  it("treats every row as new when the previous feed held no requests", () => {
    expect(diffFeed([], [a, b])).toEqual([b, a]);
  });

  it("excludes admin-plane rows from both sides", () => {
    const reload = row({ ts: "2026-08-03T10:00:04Z", kind: "secret_reload" as AuditEntry["kind"] });
    // The reload must neither be replayed nor hide `a` behind it.
    expect(diffFeed([b], [reload, a, b])).toEqual([a]);
  });

  it("keeps only the newest MAX_REPLAY_PER_POLL rows", () => {
    const many = Array.from({ length: MAX_REPLAY_PER_POLL + 5 }, (_, i) =>
      row({ ts: `2026-08-03T10:00:${String(10 + i).padStart(2, "0")}Z` }),
    ).reverse(); // newest-first
    const out = diffFeed([], many);
    expect(out).toHaveLength(MAX_REPLAY_PER_POLL);
    // Oldest-first output, and it is the newest slice that survived.
    expect(out[out.length - 1]).toEqual(many[0]);
  });
});

describe("toPacket", () => {
  it("carries the evidence model's verdict, never more", () => {
    const p = toPacket(row({ ts: "2026-08-03T10:00:00Z" }), "p1");
    expect(p.outcome).toBe("pass");
    expect(p.stopIndex).toBe(-1);
    // A plain 2xx chat row proves nothing about guardrail.
    expect(p.unproven).toContain("guardrail");
    expect(p.latencyMs).toBe(300);
  });

  it("marks the halting stage for a denial", () => {
    const p = toPacket(row({ ts: "2026-08-03T10:00:00Z", kind: "rate_limited" as AuditEntry["kind"], status: 429 }), "p2");
    expect(p.outcome).toBe("deny");
    expect(p.stopIndex).toBe(1); // CHAIN_STAGES index of "rate"
  });

  it("stops a provider failure at upstream, not at auth", () => {
    const p = toPacket(row({ ts: "2026-08-03T10:00:00Z", status: 401 }), "p3");
    expect(p.outcome).toBe("fail");
    expect(p.stopIndex).toBe(4); // "upstream"
  });
});

describe("releaseSchedule", () => {
  const mk = (ts: string) => toPacket(row({ ts }), ts);

  it("releases a lone packet immediately", () => {
    expect(releaseSchedule([mk("2026-08-03T10:00:00Z")], 5000)).toEqual([0]);
  });

  it("spreads packets across the window in proportion to their real ts", () => {
    const out = releaseSchedule(
      [mk("2026-08-03T10:00:00Z"), mk("2026-08-03T10:00:02Z"), mk("2026-08-03T10:00:04Z")],
      5000,
    );
    expect(out[0]).toBe(0);
    expect(out[2]).toBeGreaterThan(out[1]);
    expect(out[1]).toBeGreaterThan(out[0]);
    expect(Math.max(...out)).toBeLessThanOrEqual(5000);
  });

  it("spaces identical timestamps evenly rather than stacking them", () => {
    const same = "2026-08-03T10:00:00Z";
    const out = releaseSchedule([mk(same), mk(same), mk(same)], 5000);
    expect(new Set(out).size).toBe(3);
    expect(Math.max(...out)).toBeLessThanOrEqual(5000);
  });

  it("clamps a future timestamp into the window", () => {
    const out = releaseSchedule([mk("2026-08-03T10:00:00Z"), mk("2099-01-01T00:00:00Z")], 5000);
    expect(Math.max(...out)).toBeLessThanOrEqual(5000);
    expect(Math.min(...out)).toBeGreaterThanOrEqual(0);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- --run src/lib/chainReplay.test.ts`
Expected: FAIL — `Failed to resolve import "./chainReplay"`.

- [ ] **Step 3: Write the implementation**

Create `src/lib/chainReplay.ts`:

```ts
/**
 * Replay layer: turns successive audit-feed snapshots into individually
 * animatable requests.
 *
 * The chain animates one mark per real audit row. That means answering two
 * questions this module owns and lib/chain.ts does not: which rows are NEW
 * since the last poll, and WHEN within the poll window each should appear.
 *
 * Row identity is the hard part. AuditEntry has no id — every field is a
 * value, so two identical calls in the same millisecond are indistinguishable.
 * A key-based diff would therefore drop or duplicate marks under load. The
 * diff is positional instead: the feed is newest-first, so find the previous
 * head inside the new feed and take everything above it. A composite key is
 * used only to LOCATE that anchor, never to identify a row on its own.
 */

import { CHAIN_STAGES, chainStateFromEntry } from "./chain";
import type { ChainOutcome, ChainStage, GatewayAuditKind } from "./chain";
import type { AuditEntry } from "./types";

/**
 * Most rows replayed from a single poll. A quiet system never reaches this;
 * a burst would otherwise dump a hundred marks at once, which reads as noise
 * and costs frames. The newest are kept — the oldest are silently dropped,
 * because the alternative is animating a backlog as though it were live.
 */
export const MAX_REPLAY_PER_POLL = 12;

/** Admin-plane kinds, mirroring lib/chain.ts. A secret reload is not traffic. */
const ADMIN_PLANE_KINDS: ReadonlySet<string> = new Set<string>(["secret_reload"]);

export interface ChainPacket {
  /** Synthetic, for React keying and renderer bookkeeping only. Never dedupe on this. */
  id: string;
  /** Epoch ms parsed from AuditEntry.ts. NaN-safe: unparseable becomes 0. */
  ts: number;
  /** Index into CHAIN_STAGES where the request halted, or -1 when it cleared. */
  stopIndex: number;
  outcome: ChainOutcome;
  latencyMs: number;
  /** Stages the evidence does not prove ran. Renderers must not light these. */
  unproven: readonly ChainStage[];
}

/** Composite of every AuditEntry field — used solely to locate the diff anchor. */
export function rowKey(entry: AuditEntry): string {
  return [
    entry.ts,
    entry.key_name,
    entry.model,
    entry.input_tokens,
    entry.output_tokens,
    entry.cost_usd,
    entry.latency_ms,
    entry.status,
    entry.kind,
  ].join("\0");
}

function requestRows(entries: readonly AuditEntry[]): AuditEntry[] {
  return entries.filter((e) => !ADMIN_PLANE_KINDS.has(e.kind));
}

/**
 * Rows in `next` that were not in `prev`, oldest-first.
 *
 * `prev === null` means first load: adopt the snapshot as baseline and replay
 * nothing, because 100 rows of history are not live traffic. The same applies
 * when the anchor cannot be found — the feed advanced by more than its limit
 * between polls, or the key changed — which is normal under load and is not
 * an error.
 */
export function diffFeed(
  prev: readonly AuditEntry[] | null,
  next: readonly AuditEntry[],
): AuditEntry[] {
  if (prev === null) return [];
  const fresh = requestRows(next);
  const seen = requestRows(prev);

  let newRows: AuditEntry[];
  if (seen.length === 0) {
    // No prior requests at all: everything present is new.
    newRows = fresh.slice();
  } else {
    const anchor = rowKey(seen[0]);
    const idx = fresh.findIndex((e) => rowKey(e) === anchor);
    if (idx === -1) return []; // re-seed rather than replay a backlog
    newRows = fresh.slice(0, idx);
  }

  // Cap to the newest N, then flip newest-first -> oldest-first for replay.
  return newRows.slice(0, MAX_REPLAY_PER_POLL).reverse();
}

/** One audit row as a packet. The verdict comes from lib/chain.ts, unchanged. */
export function toPacket(entry: AuditEntry, id: string): ChainPacket {
  const state = chainStateFromEntry({
    status: entry.status,
    kind: entry.kind as GatewayAuditKind,
  });
  const parsed = Date.parse(entry.ts);
  return {
    id,
    ts: Number.isNaN(parsed) ? 0 : parsed,
    stopIndex: state.stoppedAt === null ? -1 : CHAIN_STAGES.indexOf(state.stoppedAt),
    outcome: state.outcome,
    latencyMs: entry.latency_ms,
    unproven: state.unproven,
  };
}

/**
 * Delay in ms before each packet is released, index-aligned with `packets`.
 *
 * Packets are spread across the poll window in proportion to their real
 * timestamps, so a burst replays with its own internal spacing rather than
 * arriving as a single clump. Identical timestamps — common, since `ts` has
 * second resolution — fall back to even spacing so marks never stack exactly.
 * 90% of the window is used, leaving headroom before the next poll lands.
 */
export function releaseSchedule(packets: readonly ChainPacket[], windowMs: number): number[] {
  const n = packets.length;
  if (n === 0) return [];
  if (n === 1) return [0];

  const span = windowMs * 0.9;
  const times = packets.map((p) => p.ts);
  const min = Math.min(...times);
  const max = Math.max(...times);

  if (max === min) {
    // No usable spread: distribute evenly.
    return packets.map((_, i) => (span * i) / (n - 1));
  }
  return packets.map((p) => {
    const frac = (p.ts - min) / (max - min);
    return Math.min(windowMs, Math.max(0, frac * span));
  });
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test -- --run src/lib/chainReplay.test.ts`
Expected: PASS, all cases green.

- [ ] **Step 5: Commit**

```bash
git add src/lib/chainReplay.ts src/lib/chainReplay.test.ts
git commit -m "feat(console): replay layer turns audit snapshots into per-request packets"
```

---

### Task 2: Style selection and persistence

**Files:**
- Create: `src/lib/chainStyle.ts`
- Test: `src/lib/chainStyle.test.ts`

**Interfaces:**
- Produces: `ChainStyle` (`"phosphor" | "corridor" | "flow" | "minimal"`), `CHAIN_STYLES`, `DEFAULT_CHAIN_STYLE`, `CHAIN_STYLE_KEY`, `parseChainStyle(raw)`, `readChainStyle(store?)`, `writeChainStyle(style, store?)`, `CHAIN_STYLE_LABELS`.

- [ ] **Step 1: Write the failing test**

Create `src/lib/chainStyle.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import {
  CHAIN_STYLES,
  CHAIN_STYLE_KEY,
  DEFAULT_CHAIN_STYLE,
  parseChainStyle,
  readChainStyle,
  writeChainStyle,
} from "./chainStyle";

/** Minimal in-memory Storage stand-in — vitest runs under environment: "node". */
function memStore(seed: Record<string, string> = {}) {
  const map = new Map(Object.entries(seed));
  return {
    getItem: (k: string) => map.get(k) ?? null,
    setItem: (k: string, v: string) => void map.set(k, v),
    removeItem: (k: string) => void map.delete(k),
  };
}

describe("parseChainStyle", () => {
  it("accepts every declared style", () => {
    for (const s of CHAIN_STYLES) expect(parseChainStyle(s)).toBe(s);
  });

  it("falls back to the default for unknown, empty or null input", () => {
    expect(parseChainStyle("hologram")).toBe(DEFAULT_CHAIN_STYLE);
    expect(parseChainStyle("")).toBe(DEFAULT_CHAIN_STYLE);
    expect(parseChainStyle(null)).toBe(DEFAULT_CHAIN_STYLE);
  });

  it("defaults to phosphor", () => {
    expect(DEFAULT_CHAIN_STYLE).toBe("phosphor");
  });
});

describe("read/write", () => {
  it("round-trips a stored choice", () => {
    const store = memStore();
    writeChainStyle("flow", store);
    expect(store.getItem(CHAIN_STYLE_KEY)).toBe("flow");
    expect(readChainStyle(store)).toBe("flow");
  });

  it("returns the default when nothing is stored", () => {
    expect(readChainStyle(memStore())).toBe(DEFAULT_CHAIN_STYLE);
  });

  it("returns the default when the stored value is corrupt", () => {
    expect(readChainStyle(memStore({ [CHAIN_STYLE_KEY]: "{}" }))).toBe(DEFAULT_CHAIN_STYLE);
  });

  it("survives a storage that throws (privacy mode)", () => {
    const hostile = {
      getItem: () => {
        throw new Error("denied");
      },
      setItem: () => {
        throw new Error("denied");
      },
      removeItem: () => {},
    };
    expect(readChainStyle(hostile)).toBe(DEFAULT_CHAIN_STYLE);
    expect(() => writeChainStyle("corridor", hostile)).not.toThrow();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- --run src/lib/chainStyle.test.ts`
Expected: FAIL — `Failed to resolve import "./chainStyle"`.

- [ ] **Step 3: Write the implementation**

Create `src/lib/chainStyle.ts`:

```ts
/**
 * Which governance-chain renderer the operator has chosen.
 *
 * Persisted per browser, following the `agentos-theme` precedent in
 * SettingsModal. Storage is injected so this stays testable under vitest's
 * node environment, and every access is guarded: a browser in privacy mode
 * throws on localStorage rather than returning null.
 */

export const CHAIN_STYLES = ["phosphor", "corridor", "flow", "minimal"] as const;

export type ChainStyle = (typeof CHAIN_STYLES)[number];

/**
 * Phosphor by default: it is the only renderer that reads as deliberate at
 * zero traffic, and the chain is unlit far more often than it is busy.
 */
export const DEFAULT_CHAIN_STYLE: ChainStyle = "phosphor";

export const CHAIN_STYLE_KEY = "agentos-chain-style";

/** One-line descriptions for the settings picker. */
export const CHAIN_STYLE_LABELS: Record<ChainStyle, string> = {
  phosphor: "Phosphor — an instrument trace with persistence",
  corridor: "Corridor — luminous gates a request punches through",
  flow: "Flow — every request a particle in the stream",
  minimal: "Minimal — the plain rule, no animation",
};

/** A stored value, or anything else, narrowed to a style. Never throws. */
export function parseChainStyle(raw: string | null | undefined): ChainStyle {
  return (CHAIN_STYLES as readonly string[]).includes(raw ?? "")
    ? (raw as ChainStyle)
    : DEFAULT_CHAIN_STYLE;
}

/** The subset of Storage this module needs — lets tests pass a plain object. */
export interface StyleStore {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

function defaultStore(): StyleStore | null {
  try {
    return typeof localStorage === "undefined" ? null : localStorage;
  } catch {
    return null;
  }
}

export function readChainStyle(store: StyleStore | null = defaultStore()): ChainStyle {
  if (!store) return DEFAULT_CHAIN_STYLE;
  try {
    return parseChainStyle(store.getItem(CHAIN_STYLE_KEY));
  } catch {
    return DEFAULT_CHAIN_STYLE;
  }
}

export function writeChainStyle(
  style: ChainStyle,
  store: StyleStore | null = defaultStore(),
): void {
  if (!store) return;
  try {
    store.setItem(CHAIN_STYLE_KEY, style);
  } catch {
    // Privacy mode: the choice applies for this session only.
  }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test -- --run src/lib/chainStyle.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/lib/chainStyle.ts src/lib/chainStyle.test.ts
git commit -m "feat(console): chain style selection, persisted and fail-safe"
```

---

### Task 3: Renderer seam, palette, and colour helper

**Files:**
- Create: `src/components/chain/renderers/types.ts`
- Create: `src/components/chain/palette.ts`
- Create: `src/components/chain/color.ts`
- Test: `src/components/chain/palette.test.ts`
- Test: `src/components/chain/color.test.ts`

**Interfaces:**
- Consumes: `ChainPacket` (Task 1), `ChainState`/`ChainStage` from `src/lib/chain.ts`.
- Produces: `ChainPalette`, `PALETTE_TOKENS`, `paletteFrom(get)`, `readPalette(el)`, `alpha(color, a)`, `ChainRenderer`, `RenderFrame`, `LivePacket`.

**Note:** `alpha()` is shared by all three renderers. It parses whatever form a
CSS custom property resolved to, so it is real logic with real edge cases and
gets its own test — it is not boilerplate worth duplicating three times.

- [ ] **Step 1a: Write the failing colour-helper test**

Create `src/components/chain/color.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { alpha } from "./color";

describe("alpha", () => {
  it("converts 6-digit hex", () => {
    expect(alpha("#5ad1c4", 0.5)).toBe("rgba(90,209,196,0.5)");
  });

  it("expands 3-digit hex", () => {
    expect(alpha("#abc", 1)).toBe("rgba(170,187,204,1)");
  });

  it("rewrites the alpha of an existing rgb()/rgba()", () => {
    expect(alpha("rgb(10, 20, 30)", 0.25)).toBe("rgba(10,20,30,0.25)");
    expect(alpha("rgba(10, 20, 30, 0.8)", 0.25)).toBe("rgba(10,20,30,0.25)");
  });

  it("trims the whitespace getComputedStyle leaves behind", () => {
    expect(alpha("  #5ad1c4  ", 1)).toBe("rgba(90,209,196,1)");
  });

  it("returns the input unchanged when it cannot be parsed", () => {
    // A named colour or an unsupported space must not become "rgba(NaN,...)".
    expect(alpha("rebeccapurple", 0.5)).toBe("rebeccapurple");
    expect(alpha("#zzz", 0.5)).toBe("#zzz");
    expect(alpha("", 0.5)).toBe("");
  });
});
```

- [ ] **Step 1b: Run it to verify it fails**

Run: `npm test -- --run src/components/chain/color.test.ts`
Expected: FAIL — `Failed to resolve import "./color"`.

- [ ] **Step 1c: Write the colour helper**

Create `src/components/chain/color.ts`:

```ts
/**
 * Alpha compositing for canvas colour.
 *
 * Renderers draw the same token at many opacities — a gate at 16% at rest and
 * 71% mid-flare — but a CSS custom property resolves to whatever the theme
 * declared: `#07080b`, `#abc`, or `rgba(255,255,255,0.13)`. This normalises
 * all of them to an rgba() string at the requested alpha.
 *
 * Anything it cannot parse is returned unchanged rather than coerced. A named
 * colour or an unsupported colour space must degrade to a visible wrong-alpha
 * mark, never to `rgba(NaN,NaN,NaN,a)`, which paints nothing at all and would
 * silently blank a stage the operator is relying on.
 */
export function alpha(color: string, a: number): string {
  const c = color.trim();

  if (c.startsWith("#")) {
    const hex = c.slice(1);
    if (!/^[0-9a-fA-F]{3}$|^[0-9a-fA-F]{6}$/.test(hex)) return color.trim();
    const full =
      hex.length === 3
        ? hex
            .split("")
            .map((ch) => ch + ch)
            .join("")
        : hex;
    const n = parseInt(full, 16);
    return `rgba(${(n >> 16) & 255},${(n >> 8) & 255},${n & 255},${a})`;
  }

  const nums = c.match(/[\d.]+/g);
  if (c.startsWith("rgb") && nums && nums.length >= 3) {
    return `rgba(${nums[0]},${nums[1]},${nums[2]},${a})`;
  }

  return c;
}
```

- [ ] **Step 1d: Run it to verify it passes**

Run: `npm test -- --run src/components/chain/color.test.ts`
Expected: PASS.

- [ ] **Step 1: Write the failing palette test**

Create `src/components/chain/palette.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { FALLBACK_PALETTE, PALETTE_TOKENS, paletteFrom } from "./palette";

describe("paletteFrom", () => {
  it("reads every channel from its CSS custom property", () => {
    const p = paletteFrom((name) => `<${name}>`);
    expect(p.bg).toBe("<--bg>");
    expect(p.ink).toBe("<--text-dim>");
    expect(p.faint).toBe("<--text-faint>");
    expect(p.edge).toBe("<--border-strong>");
    expect(p.live).toBe("<--live>");
    expect(p.ok).toBe("<--ok>");
    expect(p.hold).toBe("<--hold>");
    expect(p.deny).toBe("<--deny>");
  });

  it("falls back per-channel when a token resolves empty", () => {
    const p = paletteFrom((name) => (name === "--live" ? "" : "#123456"));
    expect(p.live).toBe(FALLBACK_PALETTE.live);
    expect(p.bg).toBe("#123456");
  });

  it("trims whitespace getComputedStyle leaves on custom properties", () => {
    expect(paletteFrom(() => "  #abcdef  ").bg).toBe("#abcdef");
  });

  it("sources no brand-violet token", () => {
    // The chroma partition guard (styles.partition.test.ts) polices CSS files
    // but cannot see canvas drawing. This is the canvas-side equivalent:
    // an instrument surface is graphite plus signal, never brand chroma.
    for (const token of PALETTE_TOKENS) {
      expect(token).not.toMatch(/^--(v|v2|v3|accent)/);
    }
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- --run src/components/chain/palette.test.ts`
Expected: FAIL — `Failed to resolve import "./palette"`.

- [ ] **Step 3: Write the implementations**

Create `src/components/chain/palette.ts`:

```ts
/**
 * Canvas colour, sourced from the same CSS custom properties the rest of the
 * console uses.
 *
 * Two reasons this exists rather than hex literals in the renderers. Themes:
 * --bg is #07080b dark and #eceae4 light, so baked values would be wrong half
 * the time. And the chroma partition: styles.partition.test.ts bans brand
 * violet on instrument surfaces, but it reads CSS files and canvas drawing is
 * invisible to it. Routing every colour through this one module keeps that
 * guarantee enforceable — see the token test in palette.test.ts.
 */

export interface ChainPalette {
  bg: string;
  ink: string;
  faint: string;
  edge: string;
  live: string;
  ok: string;
  hold: string;
  deny: string;
}

/** Channel -> CSS custom property. The single place canvas colour is decided. */
const TOKEN_OF: Record<keyof ChainPalette, string> = {
  bg: "--bg",
  ink: "--text-dim",
  faint: "--text-faint",
  edge: "--border-strong",
  live: "--live",
  ok: "--ok",
  hold: "--hold",
  deny: "--deny",
};

/** Every token this module reads. Asserted brand-violet-free by its test. */
export const PALETTE_TOKENS: readonly string[] = Object.values(TOKEN_OF);

/** Used when a token resolves empty — matches the dark theme's declared values. */
export const FALLBACK_PALETTE: ChainPalette = {
  bg: "#07080b",
  ink: "#98a1a8",
  faint: "#7e878e",
  edge: "rgba(255,255,255,0.13)",
  live: "#5ad1c4",
  ok: "#6cc48f",
  hold: "#e3a851",
  deny: "#e2685f",
};

/** Build a palette from a token resolver. Pure — the DOM lives in readPalette. */
export function paletteFrom(get: (token: string) => string): ChainPalette {
  const out = {} as ChainPalette;
  for (const channel of Object.keys(TOKEN_OF) as Array<keyof ChainPalette>) {
    const value = (get(TOKEN_OF[channel]) ?? "").trim();
    out[channel] = value || FALLBACK_PALETTE[channel];
  }
  return out;
}

/** Resolve the palette against a live element. Call on mount and theme change. */
export function readPalette(el: Element): ChainPalette {
  const cs = getComputedStyle(el);
  return paletteFrom((token) => cs.getPropertyValue(token));
}
```

Create `src/components/chain/renderers/types.ts`:

```ts
/**
 * The renderer seam.
 *
 * A renderer owns pixels and nothing else: it never fetches, never decides
 * what a stage proved, and never invents a request. It is handed already-
 * adjudicated packets and draws them. Adding a style is one new file
 * implementing this interface plus one registry entry.
 */

import type { ChainPacket } from "../../../lib/chainReplay";
import type { ChainState } from "../../../lib/chain";
import type { ChainPalette } from "../palette";

/** A packet currently on screen, with its traversal progress. */
export interface LivePacket {
  packet: ChainPacket;
  /**
   * 0..1 across the whole six-stage run. A packet that stops early still
   * advances only to its own stop boundary — see progressLimit.
   */
  progress: number;
  /** Fraction of the full run this packet may reach: (stopIndex + 1) / 6, or 1. */
  progressLimit: number;
  /** Seconds since this packet finished, for decay effects. 0 while travelling. */
  deadFor: number;
}

export interface RenderFrame {
  ctx: CanvasRenderingContext2D;
  /** CSS pixels, already DPR-corrected by the host's transform. */
  w: number;
  h: number;
  /** Milliseconds since the previous frame, clamped to <= 50 by the host. */
  dt: number;
  /** Total ms since the renderer was last reset, for ambient cycles. */
  elapsed: number;
  packets: readonly LivePacket[];
  /** The newest row's adjudicated state — drives the resting appearance. */
  state: ChainState;
  palette: ChainPalette;
  /**
   * True when the host is drawing a single static frame because the user
   * prefers reduced motion. Renderers must draw a meaningful still, not a
   * blank one, and must not rely on `elapsed` advancing.
   */
  still: boolean;
}

export interface ChainRenderer {
  draw(frame: RenderFrame): void;
  /** Drop accumulated visual state — style switch, resize, remount. */
  reset(): void;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test -- --run src/components/chain/palette.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/components/chain/palette.ts src/components/chain/palette.test.ts \
        src/components/chain/color.ts src/components/chain/color.test.ts \
        src/components/chain/renderers/types.ts
git commit -m "feat(console): chain renderer seam and theme-aware canvas palette"
```

---

### Task 4: Shared renderer test harness

**Files:**
- Create: `src/components/chain/renderers/testHarness.ts`

**Interfaces:**
- Consumes: `RenderFrame`, `LivePacket` (Task 3).
- Produces: `stubCtx()`, `frame(over?)`, `livePacket(over?)` — used by Tasks 5, 6, 7.

This is a test-support module, not shipped behaviour, so it has no test of its own; Tasks 5–7 exercise it immediately.

- [ ] **Step 1: Write the harness**

Create `src/components/chain/renderers/testHarness.ts`:

```ts
/**
 * Test support for renderers.
 *
 * Vitest runs under environment: "node" — there is no canvas and no DOM. A
 * Proxy-backed stub records which 2D-context methods were called and tolerates
 * any property a renderer sets, which is enough to prove a renderer runs a
 * frame without throwing across every input shape it must survive.
 */

import { IDLE_CHAIN } from "../../../lib/chain";
import { FALLBACK_PALETTE } from "../palette";
import type { LivePacket, RenderFrame } from "./types";

export interface StubCtx {
  ctx: CanvasRenderingContext2D;
  calls: string[];
}

export function stubCtx(): StubCtx {
  const calls: string[] = [];
  const target: Record<string, unknown> = {
    canvas: { width: 900, height: 76 },
    createLinearGradient: () => ({ addColorStop: () => {} }),
    createRadialGradient: () => ({ addColorStop: () => {} }),
    measureText: () => ({ width: 24 }),
    save: () => calls.push("save"),
    restore: () => calls.push("restore"),
  };
  const ctx = new Proxy(target, {
    get(t, prop) {
      const key = String(prop);
      if (key in t) return t[key];
      return (..._args: unknown[]) => {
        calls.push(key);
      };
    },
    set(t, prop, value) {
      t[String(prop)] = value;
      return true;
    },
  }) as unknown as CanvasRenderingContext2D;
  return { ctx, calls };
}

export function livePacket(over: Partial<LivePacket> = {}): LivePacket {
  return {
    packet: {
      id: "p1",
      ts: 1785740000000,
      stopIndex: -1,
      outcome: "pass",
      latencyMs: 320,
      unproven: ["guardrail"],
    },
    progress: 0.5,
    progressLimit: 1,
    deadFor: 0,
    ...over,
  };
}

export function frame(over: Partial<RenderFrame> = {}): RenderFrame {
  const { ctx } = stubCtx();
  return {
    ctx,
    w: 900,
    h: 76,
    dt: 16,
    elapsed: 1000,
    packets: [],
    state: IDLE_CHAIN,
    palette: FALLBACK_PALETTE,
    still: false,
    ...over,
  };
}
```

- [ ] **Step 2: Verify it compiles**

Run: `npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add src/components/chain/renderers/testHarness.ts
git commit -m "test(console): stub 2D context so renderers are testable under node"
```

---

### Task 5: Phosphor renderer (default)

**Files:**
- Create: `src/components/chain/renderers/phosphor.ts`
- Test: `src/components/chain/renderers/phosphor.test.ts`

**Interfaces:**
- Consumes: `ChainRenderer`, `RenderFrame` (Task 3); `stubCtx`, `frame`, `livePacket` (Task 4); `CHAIN_STAGES`, `stageRenders` from `src/lib/chain.ts`.
- Produces: `createPhosphorRenderer(): ChainRenderer`.

- [ ] **Step 1: Write the failing test**

Create `src/components/chain/renderers/phosphor.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { IDLE_CHAIN } from "../../../lib/chain";
import { createPhosphorRenderer } from "./phosphor";
import { frame, livePacket, stubCtx } from "./testHarness";

describe("phosphor renderer", () => {
  it("draws an idle frame without throwing", () => {
    const r = createPhosphorRenderer();
    expect(() => r.draw(frame())).not.toThrow();
  });

  it("fades rather than clears, so persistence survives between frames", () => {
    const r = createPhosphorRenderer();
    const { ctx, calls } = stubCtx();
    r.draw(frame({ ctx }));
    expect(calls).toContain("fillRect");
    expect(calls).not.toContain("clearRect");
  });

  it("draws a travelling packet", () => {
    const r = createPhosphorRenderer();
    const { ctx, calls } = stubCtx();
    r.draw(frame({ ctx, packets: [livePacket({ progress: 0.4 })] }));
    expect(calls).toContain("stroke");
  });

  it("survives a denied packet at every stage index", () => {
    const r = createPhosphorRenderer();
    for (let i = 0; i < 6; i++) {
      const p = livePacket({
        packet: { ...livePacket().packet, stopIndex: i, outcome: "deny" },
        progress: 1,
        progressLimit: (i + 1) / 6,
        deadFor: 0.2,
      });
      expect(() => r.draw(frame({ packets: [p] }))).not.toThrow();
    }
  });

  it("draws a meaningful still when reduced motion is preferred", () => {
    const r = createPhosphorRenderer();
    const { ctx, calls } = stubCtx();
    r.draw(frame({ ctx, still: true, state: { ...IDLE_CHAIN, cleared: 6 } }));
    expect(calls.length).toBeGreaterThan(0);
    expect(calls).toContain("fillText");
  });

  it("survives a zero-size canvas without dividing by zero", () => {
    const r = createPhosphorRenderer();
    expect(() => r.draw(frame({ w: 0, h: 0 }))).not.toThrow();
  });

  it("reset clears accumulated state and stays drawable", () => {
    const r = createPhosphorRenderer();
    r.draw(frame({ packets: [livePacket()] }));
    r.reset();
    expect(() => r.draw(frame())).not.toThrow();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- --run src/components/chain/renderers/phosphor.test.ts`
Expected: FAIL — `Failed to resolve import "./phosphor"`.

- [ ] **Step 3: Write the implementation**

Create `src/components/chain/renderers/phosphor.ts`:

```ts
/**
 * Phosphor: the chain as a trace on lab equipment.
 *
 * Each request is a pulse propagating along the trace, leaving persistence
 * that decays — the canvas is dimmed each frame rather than cleared, which is
 * what produces the afterglow. A denial clips hard and burns a mark that fades
 * over a couple of seconds.
 *
 * The unproven rule (see lib/chain.ts): a pulse crossing a stage the evidence
 * does not prove ran deposits NO phosphor at that tick. The pulse still passes
 * — the request was not stopped there — but the console must not draw a mark
 * implying a check fired when it may never have run.
 */

import { CHAIN_STAGES, stageRenders } from "../../../lib/chain";
import { alpha } from "../color";
import type { ChainRenderer, RenderFrame } from "./types";

const BURN_DECAY_MS = 2600;
const PULSE_HALF_WIDTH = 30;
const PULSE_HEIGHT = 30;

interface Burn {
  x: number;
  life: number; // 1 -> 0
  denied: boolean;
}

export function createPhosphorRenderer(): ChainRenderer {
  let burns: Burn[] = [];
  let seen = new Set<string>();

  function stageX(i: number, w: number, pad: number): number {
    return pad + (w - pad * 2) * ((i + 0.5) / CHAIN_STAGES.length);
  }

  return {
    reset() {
      burns = [];
      seen = new Set();
    },

    draw({ ctx, w, h, dt, packets, state, palette, still }: RenderFrame) {
      if (w <= 0 || h <= 0) return;
      const pad = Math.min(26, w * 0.05);
      const mid = h * 0.46;
      const labelY = Math.max(h - 8, mid + 12);

      // Persistence: dim the previous frame instead of clearing it. A still
      // frame must not accumulate, so it paints opaque.
      ctx.fillStyle = still ? palette.bg : alpha(palette.bg, 0.14);
      ctx.fillRect(0, 0, w, h);

      // Graticule
      ctx.strokeStyle = alpha(palette.edge, 0.22);
      ctx.lineWidth = 1;
      for (let i = 0; i <= 10; i++) {
        const x = pad + ((w - pad * 2) * i) / 10;
        ctx.beginPath();
        ctx.moveTo(x, Math.max(0, mid - h * 0.3));
        ctx.lineTo(x, mid + h * 0.3);
        ctx.stroke();
      }

      // Baseline trace
      ctx.strokeStyle = alpha(palette.live, 0.2);
      ctx.lineWidth = 1.25;
      ctx.beginPath();
      ctx.moveTo(pad, mid);
      ctx.lineTo(w - pad, mid);
      ctx.stroke();

      // Stage ticks and labels. `stageRenders` decides which may be lit at
      // rest: cleared stages take ink, unproven and pending stay faint.
      const renders = stageRenders(state);
      ctx.textAlign = "center";
      ctx.font = '600 8px "IBM Plex Mono", ui-monospace, monospace';
      for (let i = 0; i < CHAIN_STAGES.length; i++) {
        const x = stageX(i, w, pad);
        const render = renders[i];
        const tickColor =
          render === "stopped"
            ? state.outcome === "fail"
              ? palette.hold
              : palette.deny
            : render === "cleared"
              ? palette.ink
              : palette.edge;
        ctx.strokeStyle = tickColor;
        ctx.lineWidth = render === "stopped" ? 2 : 1;
        ctx.beginPath();
        ctx.moveTo(x, mid - 9);
        ctx.lineTo(x, mid + 9);
        ctx.stroke();
        ctx.fillStyle = render === "cleared" ? palette.ink : palette.faint;
        ctx.fillText(CHAIN_STAGES[i].toUpperCase(), x, labelY);
      }

      // Burns decay
      for (let i = burns.length - 1; i >= 0; i--) {
        const b = burns[i];
        b.life -= dt / BURN_DECAY_MS;
        if (b.life <= 0) {
          burns.splice(i, 1);
          continue;
        }
        ctx.fillStyle = alpha(b.denied ? palette.deny : palette.hold, 0.55 * b.life);
        ctx.fillRect(b.x - 1.5, mid - 22, 3, 44);
      }

      // Pulses
      for (const lp of packets) {
        const { packet } = lp;
        const reach = Math.min(lp.progress, lp.progressLimit);
        const x = pad + (w - pad * 2) * reach;

        if (lp.deadFor > 0 && packet.stopIndex >= 0 && !seen.has(packet.id)) {
          seen.add(packet.id);
          burns.push({ x, life: 1, denied: packet.outcome === "deny" });
        }
        if (lp.deadFor > 0) continue;

        ctx.strokeStyle = alpha(palette.live, 0.95);
        ctx.lineWidth = 1.6;
        ctx.lineCap = "round";
        ctx.beginPath();
        for (let dx = -PULSE_HALF_WIDTH; dx <= PULSE_HALF_WIDTH; dx += 2) {
          const xx = x + dx;
          if (xx < pad || xx > w - pad) continue;
          const yy = mid - Math.exp(-(dx * dx) / 150) * PULSE_HEIGHT;
          if (dx === -PULSE_HALF_WIDTH) ctx.moveTo(xx, yy);
          else ctx.lineTo(xx, yy);
        }
        ctx.stroke();
      }

      // Readout
      ctx.font = '500 9px "IBM Plex Mono", ui-monospace, monospace';
      ctx.textAlign = "left";
      ctx.fillStyle = alpha(palette.faint, 0.7);
      const flying = packets.filter((p) => p.deadFor === 0).length;
      ctx.fillText(`IN FLIGHT ${String(flying).padStart(2, "0")}`, pad, 12);
      ctx.textAlign = "right";
      ctx.fillStyle =
        state.outcome === "deny"
          ? palette.deny
          : state.outcome === "fail"
            ? palette.hold
            : alpha(palette.faint, 0.7);
      ctx.fillText(
        state.outcome === "deny" ? "DENIED" : state.outcome === "fail" ? "PROVIDER FAILED" : "NOMINAL",
        w - pad,
        12,
      );
    },
  };
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test -- --run src/components/chain/renderers/phosphor.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/components/chain/renderers/phosphor.ts src/components/chain/renderers/phosphor.test.ts
git commit -m "feat(console): phosphor renderer — persistence trace, unproven stages deposit nothing"
```

---

### Task 6: Corridor renderer

**Files:**
- Create: `src/components/chain/renderers/corridor.ts`
- Test: `src/components/chain/renderers/corridor.test.ts`

**Interfaces:**
- Consumes: same as Task 5.
- Produces: `createCorridorRenderer(): ChainRenderer`.

- [ ] **Step 1: Write the failing test**

Create `src/components/chain/renderers/corridor.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { IDLE_CHAIN } from "../../../lib/chain";
import { createCorridorRenderer } from "./corridor";
import { frame, livePacket, stubCtx } from "./testHarness";

describe("corridor renderer", () => {
  it("draws an idle frame without throwing", () => {
    const r = createCorridorRenderer();
    expect(() => r.draw(frame())).not.toThrow();
  });

  it("clears each frame — gates do not accumulate", () => {
    const r = createCorridorRenderer();
    const { ctx, calls } = stubCtx();
    r.draw(frame({ ctx }));
    expect(calls).toContain("clearRect");
  });

  it("draws a streak for a travelling packet", () => {
    const r = createCorridorRenderer();
    const { ctx, calls } = stubCtx();
    r.draw(frame({ ctx, packets: [livePacket({ progress: 0.5 })] }));
    expect(calls).toContain("stroke");
  });

  it("survives a denied packet at every stage index", () => {
    const r = createCorridorRenderer();
    for (let i = 0; i < 6; i++) {
      const p = livePacket({
        packet: { ...livePacket().packet, stopIndex: i, outcome: "deny" },
        progress: 1,
        progressLimit: (i + 1) / 6,
        deadFor: 0.1,
      });
      expect(() => r.draw(frame({ packets: [p] }))).not.toThrow();
    }
  });

  it("draws a meaningful still when reduced motion is preferred", () => {
    const r = createCorridorRenderer();
    const { ctx, calls } = stubCtx();
    r.draw(frame({ ctx, still: true, state: { ...IDLE_CHAIN, cleared: 6 } }));
    expect(calls).toContain("fillText");
  });

  it("survives a zero-size canvas", () => {
    const r = createCorridorRenderer();
    expect(() => r.draw(frame({ w: 0, h: 0 }))).not.toThrow();
  });

  it("reset clears accumulated state and stays drawable", () => {
    const r = createCorridorRenderer();
    r.draw(frame({ packets: [livePacket()] }));
    r.reset();
    expect(() => r.draw(frame())).not.toThrow();
  });

  // The two tests below are the point of the renderer. They must be written so
  // each would FAIL against a plausible wrong implementation, and they are a
  // PAIR on purpose: without the second, a renderer that flares nothing at all
  // would satisfy the first vacuously.

  it("never flares a gate the evidence does not prove ran", () => {
    const r = createCorridorRenderer();
    const { ctx, draws } = stubCtx();
    const w = 900;
    const pad = Math.min(26, w * 0.05);
    const guardrailX = pad + (w - pad * 2) * ((3 + 0.5) / CHAIN_STAGES.length);

    r.draw(
      frame({
        ctx,
        w,
        packets: [
          livePacket({
            packet: { ...livePacket().packet, unproven: ["guardrail"] },
            progress: 1,
            progressLimit: 1,
          }),
        ],
      }),
    );

    // The flare mark is a 2px-wide fillRect at x - 1. The gate BODY (3px at
    // x - 1.5) is drawn every frame regardless and must not be counted, so
    // match on width to tell the claim apart from the furniture.
    const flares = draws.filter(
      (d) =>
        d.method === "fillRect" &&
        d.args[2] === 2 &&
        Math.abs((d.args[0] as number) + 1 - guardrailX) < 1.5,
    );
    expect(flares).toHaveLength(0);
  });

  it("flares a gate the evidence does prove ran", () => {
    const r = createCorridorRenderer();
    const { ctx, draws } = stubCtx();
    const w = 900;
    const pad = Math.min(26, w * 0.05);
    const authX = pad + (w - pad * 2) * ((0 + 0.5) / CHAIN_STAGES.length);

    r.draw(
      frame({
        ctx,
        w,
        packets: [
          livePacket({
            packet: { ...livePacket().packet, unproven: ["guardrail"] },
            progress: 1,
            progressLimit: 1,
          }),
        ],
      }),
    );

    const flares = draws.filter(
      (d) =>
        d.method === "fillRect" &&
        d.args[2] === 2 &&
        Math.abs((d.args[0] as number) + 1 - authX) < 1.5,
    );
    expect(flares.length).toBeGreaterThan(0);
  });
});
```

The test file's imports must include `CHAIN_STAGES` from `../../../lib/chain` and `stubCtx` from `./testHarness`.

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- --run src/components/chain/renderers/corridor.test.ts`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the implementation**

Create `src/components/chain/renderers/corridor.ts`:

```ts
/**
 * Corridor: six luminous gates a request punches through.
 *
 * A gate flares only when a packet's evidence proves that stage fired. A
 * packet crossing an unproven stage passes through a gate that stays a dim
 * outline — the request was not stopped there, but nothing proves the check
 * ran, and a flare would assert exactly that.
 *
 * A denial slams its gate: the streak stops and shatters into sparks.
 */

import { CHAIN_STAGES, stageRenders } from "../../../lib/chain";
import { alpha } from "../color";
import type { ChainRenderer, RenderFrame } from "./types";

const SPARKS_PER_DENIAL = 16;
const SPARK_LIFE_MS = 620;
const FLARE_DECAY_MS = 260;

interface Spark {
  x: number;
  y: number;
  vx: number;
  vy: number;
  life: number;
}

export function createCorridorRenderer(): ChainRenderer {
  let flare = new Array<number>(CHAIN_STAGES.length).fill(0);
  let flareDenied = -1;
  let sparks: Spark[] = [];
  let handled = new Set<string>();

  function gateX(i: number, w: number, pad: number): number {
    return pad + (w - pad * 2) * ((i + 0.5) / CHAIN_STAGES.length);
  }

  return {
    reset() {
      flare = new Array<number>(CHAIN_STAGES.length).fill(0);
      flareDenied = -1;
      sparks = [];
      handled = new Set();
    },

    draw({ ctx, w, h, dt, packets, state, palette, still }: RenderFrame) {
      if (w <= 0 || h <= 0) return;
      const pad = Math.min(26, w * 0.05);
      const mid = h * 0.46;
      const labelY = Math.max(h - 8, mid + 12);
      const gateHalf = Math.min(34, h * 0.32);

      ctx.clearRect(0, 0, w, h);

      // Light each gate a packet has proven it cleared this frame.
      for (const lp of packets) {
        if (lp.deadFor > 0) continue;
        const reached = Math.floor(lp.progress * CHAIN_STAGES.length);
        for (let i = 0; i < Math.min(reached, CHAIN_STAGES.length); i++) {
          // An unproven stage never flares, however far the packet got.
          if (lp.packet.unproven.includes(CHAIN_STAGES[i])) continue;
          if (lp.packet.stopIndex >= 0 && i > lp.packet.stopIndex) continue;
          flare[i] = 1;
        }
      }

      const renders = stageRenders(state);
      ctx.textAlign = "center";
      ctx.font = '600 8px "IBM Plex Mono", ui-monospace, monospace';

      for (let i = 0; i < CHAIN_STAGES.length; i++) {
        const x = gateX(i, w, pad);
        const f = flare[i];
        flare[i] = Math.max(0, f - dt / FLARE_DECAY_MS);
        const denied = flareDenied === i;
        const col = denied ? palette.deny : palette.live;

        // Gate body — a vertical slit that brightens with the flare.
        const grad = ctx.createLinearGradient(x, mid - gateHalf, x, mid + gateHalf);
        grad.addColorStop(0, alpha(col, 0));
        grad.addColorStop(0.5, alpha(col, 0.16 + f * 0.55));
        grad.addColorStop(1, alpha(col, 0));
        ctx.fillStyle = grad;
        ctx.fillRect(x - 1.5, mid - gateHalf, 3, gateHalf * 2);

        // The flare mark. Drawn ONLY while this gate is actually flaring, as a
        // discrete rect rather than a gradient stop — a gradient's stops are
        // invisible to the test harness's stub context, so a flare expressed
        // only through `grad` could not be asserted, and the unproven rule
        // would again be untestable. This is the corridor's per-stage claim:
        // it appears exactly when the evidence proves the stage fired.
        if (f > 0) {
          ctx.fillStyle = alpha(col, 0.5 * f);
          ctx.fillRect(x - 1, mid - gateHalf, 2, gateHalf * 2);
        }

        const render = renders[i];
        ctx.fillStyle =
          render === "stopped"
            ? state.outcome === "fail"
              ? palette.hold
              : palette.deny
            : render === "cleared" || f > 0
              ? palette.ink
              : palette.faint;
        ctx.fillText(CHAIN_STAGES[i].toUpperCase(), x, labelY);
      }

      // Streaks
      for (const lp of packets) {
        if (lp.deadFor > 0) {
          if (lp.packet.stopIndex >= 0 && !handled.has(lp.packet.id)) {
            handled.add(lp.packet.id);
            flareDenied = lp.packet.stopIndex;
            flare[lp.packet.stopIndex] = 1;
            const sx = gateX(lp.packet.stopIndex, w, pad);
            for (let k = 0; k < SPARKS_PER_DENIAL; k++) {
              sparks.push({
                x: sx,
                y: mid,
                vx: (k / SPARKS_PER_DENIAL - 0.7) * 90,
                vy: (k % 2 === 0 ? 1 : -1) * (30 + k * 6),
                life: 1,
              });
            }
          }
          continue;
        }
        const reach = Math.min(lp.progress, lp.progressLimit);
        const x = pad + (w - pad * 2) * reach;
        const tail = 46;
        const grad = ctx.createLinearGradient(x - tail, 0, x, 0);
        grad.addColorStop(0, alpha(palette.live, 0));
        grad.addColorStop(1, alpha(palette.live, 0.95));
        ctx.strokeStyle = grad;
        ctx.lineWidth = 2;
        ctx.lineCap = "round";
        ctx.beginPath();
        ctx.moveTo(Math.max(pad, x - tail), mid);
        ctx.lineTo(x, mid);
        ctx.stroke();
      }

      // Sparks
      for (let i = sparks.length - 1; i >= 0; i--) {
        const s = sparks[i];
        s.x += (s.vx * dt) / 1000;
        s.y += (s.vy * dt) / 1000;
        s.vy += (170 * dt) / 1000;
        s.life -= dt / SPARK_LIFE_MS;
        if (s.life <= 0) {
          sparks.splice(i, 1);
          continue;
        }
        ctx.fillStyle = alpha(palette.deny, s.life);
        ctx.fillRect(s.x, s.y, 1.6, 1.6);
      }

      if (still) {
        ctx.font = '500 9px "IBM Plex Mono", ui-monospace, monospace';
        ctx.textAlign = "left";
        ctx.fillStyle = alpha(palette.faint, 0.7);
        ctx.fillText("STILL", pad, 12);
      }
    },
  };
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test -- --run src/components/chain/renderers/corridor.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/components/chain/renderers/corridor.ts src/components/chain/renderers/corridor.test.ts
git commit -m "feat(console): corridor renderer — gates flare only on proven stages"
```

---

### Task 7: Flow renderer

**Files:**
- Create: `src/components/chain/renderers/flow.ts`
- Test: `src/components/chain/renderers/flow.test.ts`

**Interfaces:**
- Consumes: same as Task 5.
- Produces: `createFlowRenderer(): ChainRenderer`.

- [ ] **Step 1: Write the failing test**

Create `src/components/chain/renderers/flow.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { IDLE_CHAIN } from "../../../lib/chain";
import { createFlowRenderer } from "./flow";
import { frame, livePacket, stubCtx } from "./testHarness";

describe("flow renderer", () => {
  it("draws an idle frame without throwing", () => {
    const r = createFlowRenderer();
    expect(() => r.draw(frame())).not.toThrow();
  });

  it("draws gate columns even with no traffic", () => {
    const r = createFlowRenderer();
    const { ctx, calls } = stubCtx();
    r.draw(frame({ ctx }));
    expect(calls).toContain("fillText");
  });

  it("dashes the column for an unproven stage and restores the dash pattern", () => {
    const r = createFlowRenderer();
    const { ctx, calls } = stubCtx();
    // guardrail is unproven on a plain chat pass, so some column must be dashed.
    r.draw(frame({ ctx, state: { ...IDLE_CHAIN, cleared: 6, unproven: ["guardrail"] } }));
    expect(calls).toContain("setLineDash");
  });

  it("draws a particle for a travelling packet", () => {
    const r = createFlowRenderer();
    const { ctx, calls } = stubCtx();
    r.draw(frame({ ctx, packets: [livePacket({ progress: 0.3 })] }));
    expect(calls).toContain("stroke");
  });

  it("survives a denied packet at every stage index", () => {
    const r = createFlowRenderer();
    for (let i = 0; i < 6; i++) {
      const p = livePacket({
        packet: { ...livePacket().packet, stopIndex: i, outcome: "deny" },
        progress: 1,
        progressLimit: (i + 1) / 6,
        deadFor: 0.3,
      });
      expect(() => r.draw(frame({ packets: [p] }))).not.toThrow();
    }
  });

  it("draws a meaningful still when reduced motion is preferred", () => {
    const r = createFlowRenderer();
    const { ctx, calls } = stubCtx();
    r.draw(frame({ ctx, still: true, state: { ...IDLE_CHAIN, cleared: 6 } }));
    expect(calls).toContain("fillText");
  });

  it("survives a zero-size canvas", () => {
    const r = createFlowRenderer();
    expect(() => r.draw(frame({ w: 0, h: 0 }))).not.toThrow();
  });

  it("reset clears accumulated state and stays drawable", () => {
    const r = createFlowRenderer();
    r.draw(frame({ packets: [livePacket()] }));
    r.reset();
    expect(() => r.draw(frame())).not.toThrow();
  });

  // The dash IS this renderer's unproven vocabulary — flow draws no per-gate
  // crossing mark, so the column is the only thing that can over-claim. These
  // two tests are a pair: the second stops the first passing vacuously against
  // a renderer that dashes everything.

  it("dashes a column an on-screen packet leaves unproven, even when the resting state proves it", () => {
    const r = createFlowRenderer();
    const { ctx, draws } = stubCtx();

    r.draw(
      frame({
        ctx,
        // Resting state proves everything — only the live packet says otherwise.
        state: { cleared: 6, stoppedAt: null, outcome: "pass", unproven: [] },
        packets: [
          livePacket({
            packet: { ...livePacket().packet, unproven: ["guardrail"] },
            progress: 0.9,
          }),
        ],
      }),
    );

    // setLineDash calls arrive in column order; a non-empty array is a dash.
    const dashes = draws
      .filter((d) => d.method === "setLineDash")
      .map((d) => (d.args[0] as number[]).length > 0);
    // Columns are drawn one per stage, each preceded by its own setLineDash,
    // then reset to solid — so take every other entry, the "set" ones.
    const perColumn = dashes.filter((_, i) => i % 2 === 0);
    expect(perColumn[3], "guardrail column must be dashed").toBe(true);
  });

  it("draws a proven column solid", () => {
    const r = createFlowRenderer();
    const { ctx, draws } = stubCtx();

    r.draw(
      frame({
        ctx,
        state: { cleared: 6, stoppedAt: null, outcome: "pass", unproven: [] },
        packets: [
          livePacket({
            packet: { ...livePacket().packet, unproven: ["guardrail"] },
            progress: 0.9,
          }),
        ],
      }),
    );

    const dashes = draws
      .filter((d) => d.method === "setLineDash")
      .map((d) => (d.args[0] as number[]).length > 0);
    const perColumn = dashes.filter((_, i) => i % 2 === 0);
    expect(perColumn[0], "auth column must be solid").toBe(false);
  });
});
```

The test file's imports must include `stubCtx` from `./testHarness`.

**Implementer note on the two dash tests:** they assume each column emits exactly two `setLineDash` calls — one to set the pattern, one to reset to solid — in stage order. If your implementation emits a different number, adjust the index arithmetic so the assertion still targets the guardrail and auth columns specifically. Do not weaken the assertion to make it pass; the point is to pin *which column* is dashed.

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- --run src/components/chain/renderers/flow.test.ts`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the implementation**

Create `src/components/chain/renderers/flow.ts`:

```ts
/**
 * Flow: every in-flight request is a particle crossing six gates.
 *
 * Density is throughput — the picture is made of information, so it is sparse
 * on a quiet system by design. Denied particles turn deny-red, deflect out at
 * their stage and fall away, so where requests die is legible without a legend.
 *
 * Unproven stages are drawn as DASHED columns rather than solid ones: the
 * particle crosses either way, but a solid column would read as a check that
 * fired, and on a plain chat row nothing proves the screener ran at all.
 */

import { CHAIN_STAGES, stageRenders } from "../../../lib/chain";
import { alpha } from "../color";
import type { ChainRenderer, RenderFrame } from "./types";

interface Drift {
  y: number;
  vy: number;
  life: number;
}

/** Deterministic per-packet lane, so a particle does not jitter between frames. */
function laneFor(id: string): number {
  let hash = 0;
  for (let i = 0; i < id.length; i++) hash = (hash * 31 + id.charCodeAt(i)) | 0;
  return (Math.abs(hash) % 1000) / 1000;
}

export function createFlowRenderer(): ChainRenderer {
  let drifts = new Map<string, Drift>();

  function gateX(i: number, w: number, pad: number): number {
    return pad + (w - pad * 2) * ((i + 0.5) / CHAIN_STAGES.length);
  }

  return {
    reset() {
      drifts = new Map();
    },

    draw({ ctx, w, h, dt, packets, state, palette, still }: RenderFrame) {
      if (w <= 0 || h <= 0) return;
      const pad = Math.min(26, w * 0.05);
      const top = Math.min(22, h * 0.22);
      const bot = Math.max(top + 1, h - 26);
      const labelY = Math.max(h - 8, bot + 10);

      ctx.clearRect(0, 0, w, h);

      const renders = stageRenders(state);
      ctx.textAlign = "center";
      ctx.font = '600 8px "IBM Plex Mono", ui-monospace, monospace';

      for (let i = 0; i < CHAIN_STAGES.length; i++) {
        const x = gateX(i, w, pad);
        // Unproven if the resting reading says so, OR if any packet currently
        // on screen carries it unproven.
        //
        // Reading `state` alone would be wrong here in a way that is easy to
        // miss: `state` is the LATEST adjudicated row, while the particles
        // being drawn belong to a window of earlier rows. A column drawn solid
        // because the newest row happened to prove guardrail would assert that
        // claim over particles for which it was never proven. The union is the
        // only reading that never over-claims for anything actually on screen.
        const stage = CHAIN_STAGES[i];
        const unproven =
          state.unproven.includes(stage) ||
          packets.some((lp) => lp.packet.unproven.includes(stage));
        ctx.strokeStyle =
          renders[i] === "stopped"
            ? state.outcome === "fail"
              ? palette.hold
              : palette.deny
            : alpha(palette.edge, unproven ? 0.5 : 1);
        ctx.lineWidth = 1;
        // Dashed == "the evidence does not prove this check ran."
        ctx.setLineDash(unproven ? [2, 3] : []);
        ctx.beginPath();
        ctx.moveTo(x, top);
        ctx.lineTo(x, bot);
        ctx.stroke();
        ctx.setLineDash([]);

        ctx.fillStyle = renders[i] === "cleared" ? palette.ink : palette.faint;
        ctx.fillText(CHAIN_STAGES[i].toUpperCase(), x, labelY);
      }

      let flying = 0;
      const alive = new Set<string>();

      for (const lp of packets) {
        const { packet } = lp;
        alive.add(packet.id);
        const lane = top + laneFor(packet.id) * (bot - top);
        const reach = Math.min(lp.progress, lp.progressLimit);
        const x = pad + (w - pad * 2) * reach;

        let y = lane;
        let color = packet.outcome === "pass" ? palette.live : palette.deny;
        let life = 1;

        if (lp.deadFor > 0 && packet.stopIndex >= 0) {
          let d = drifts.get(packet.id);
          if (!d) {
            d = { y: lane, vy: 30 + laneFor(packet.id) * 70, life: 1 };
            drifts.set(packet.id, d);
          }
          d.y += (d.vy * dt) / 1000;
          d.vy += (90 * dt) / 1000;
          d.life = Math.max(0, 1 - lp.deadFor / 0.9);
          y = d.y;
          life = d.life;
          color = packet.outcome === "fail" ? palette.hold : palette.deny;
        } else if (lp.deadFor === 0) {
          flying++;
        }

        if (life <= 0) continue;

        const tail = lp.deadFor > 0 ? 6 : 16;
        const grad = ctx.createLinearGradient(x - tail, 0, x, 0);
        grad.addColorStop(0, alpha(color, 0));
        grad.addColorStop(1, alpha(color, 0.85 * life));
        ctx.strokeStyle = grad;
        ctx.lineWidth = 1.6;
        ctx.lineCap = "round";
        ctx.beginPath();
        ctx.moveTo(Math.max(pad, x - tail), y);
        ctx.lineTo(x, y);
        ctx.stroke();
      }

      // Drop drift bookkeeping for packets the host has retired.
      for (const id of drifts.keys()) if (!alive.has(id)) drifts.delete(id);

      ctx.font = '500 9px "IBM Plex Mono", ui-monospace, monospace';
      ctx.textAlign = "left";
      ctx.fillStyle = alpha(palette.faint, 0.7);
      ctx.fillText(still ? "STILL" : `IN FLIGHT ${String(flying).padStart(3, "0")}`, pad, 12);
    },
  };
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test -- --run src/components/chain/renderers/flow.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/components/chain/renderers/flow.ts src/components/chain/renderers/flow.test.ts
git commit -m "feat(console): flow renderer — density is throughput, unproven gates dashed"
```

---

### Task 8: Renderer registry

**Files:**
- Create: `src/components/chain/renderers/index.ts`
- Test: `src/components/chain/renderers/index.test.ts`

**Interfaces:**
- Consumes: `createPhosphorRenderer`, `createCorridorRenderer`, `createFlowRenderer` (Tasks 5–7); `ChainStyle`, `CHAIN_STYLES` (Task 2).
- Produces: `createRenderer(style): ChainRenderer | null`.

- [ ] **Step 1: Write the failing test**

Create `src/components/chain/renderers/index.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { CHAIN_STYLES } from "../../../lib/chainStyle";
import { createRenderer } from "./index";
import { frame } from "./testHarness";

describe("createRenderer", () => {
  it("returns a drawable renderer for every animated style", () => {
    for (const style of CHAIN_STYLES) {
      if (style === "minimal") continue;
      const r = createRenderer(style);
      expect(r, `${style} must have a renderer`).not.toBeNull();
      expect(() => r!.draw(frame())).not.toThrow();
    }
  });

  it("returns null for minimal — it is DOM, not canvas", () => {
    expect(createRenderer("minimal")).toBeNull();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- --run src/components/chain/renderers/index.test.ts`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the implementation**

Create `src/components/chain/renderers/index.ts`:

```ts
/**
 * Style -> renderer. Adding a style is one file plus one line here.
 *
 * `minimal` deliberately has no renderer: it is the original DOM chain, kept
 * for low-power machines and anyone who does not want an animated canvas above
 * their content. Chain.tsx branches on the null.
 */

import type { ChainStyle } from "../../../lib/chainStyle";
import { createCorridorRenderer } from "./corridor";
import { createFlowRenderer } from "./flow";
import { createPhosphorRenderer } from "./phosphor";
import type { ChainRenderer } from "./types";

export function createRenderer(style: ChainStyle): ChainRenderer | null {
  switch (style) {
    case "phosphor":
      return createPhosphorRenderer();
    case "corridor":
      return createCorridorRenderer();
    case "flow":
      return createFlowRenderer();
    case "minimal":
      return null;
  }
}

export type { ChainRenderer, RenderFrame, LivePacket } from "./types";
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test -- --run src/components/chain/renderers/index.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/components/chain/renderers/index.ts src/components/chain/renderers/index.test.ts
git commit -m "feat(console): chain renderer registry"
```

---

### Task 9: Packet lifecycle

**Files:**
- Create: `src/components/chain/lifecycle.ts`
- Test: `src/components/chain/lifecycle.test.ts`

**Interfaces:**
- Consumes: `ChainPacket` (Task 1), `LivePacket` (Task 3), `CHAIN_STAGES`.
- Produces: `travelDurationMs(packet)`, `advance(live, dtMs)`, `RETIRE_AFTER_S`.

Splitting this out of the React host keeps the timing rules unit-testable under node.

- [ ] **Step 1: Write the failing test**

Create `src/components/chain/lifecycle.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { ChainPacket } from "../../lib/chainReplay";
import { RETIRE_AFTER_S, advance, travelDurationMs } from "./lifecycle";

function packet(over: Partial<ChainPacket> = {}): ChainPacket {
  return {
    id: "p1",
    ts: 0,
    stopIndex: -1,
    outcome: "pass",
    latencyMs: 300,
    unproven: [],
    ...over,
  };
}

describe("travelDurationMs", () => {
  it("scales with the row's real latency", () => {
    expect(travelDurationMs(packet({ latencyMs: 2000 }))).toBeGreaterThan(
      travelDurationMs(packet({ latencyMs: 100 })),
    );
  });

  it("clamps absurd latencies into a watchable range", () => {
    expect(travelDurationMs(packet({ latencyMs: 0 }))).toBeGreaterThanOrEqual(400);
    expect(travelDurationMs(packet({ latencyMs: 900_000 }))).toBeLessThanOrEqual(4000);
  });

  it("treats a missing or negative latency as the floor", () => {
    expect(travelDurationMs(packet({ latencyMs: -5 }))).toBeGreaterThanOrEqual(400);
    expect(Number.isFinite(travelDurationMs(packet({ latencyMs: NaN })))).toBe(true);
  });
});

describe("advance", () => {
  it("moves a packet toward its limit", () => {
    const live = { packet: packet(), progress: 0, progressLimit: 1, deadFor: 0 };
    const next = advance(live, 200);
    expect(next.progress).toBeGreaterThan(0);
    expect(next.deadFor).toBe(0);
  });

  it("never advances past the packet's own stop boundary", () => {
    const live = {
      packet: packet({ stopIndex: 1, outcome: "deny" as const }),
      progress: 0,
      progressLimit: 2 / 6,
      deadFor: 0,
    };
    let cur = live;
    for (let i = 0; i < 200; i++) cur = advance(cur, 50);
    expect(cur.progress).toBeLessThanOrEqual(2 / 6 + 1e-9);
  });

  it("starts counting deadFor once it reaches the limit", () => {
    let cur = { packet: packet(), progress: 0.999, progressLimit: 1, deadFor: 0 };
    cur = advance(cur, 500);
    expect(cur.progress).toBe(1);
    expect(cur.deadFor).toBeGreaterThan(0);
  });

  it("retires only after RETIRE_AFTER_S", () => {
    let cur = { packet: packet(), progress: 1, progressLimit: 1, deadFor: 0 };
    cur = advance(cur, RETIRE_AFTER_S * 1000 - 100);
    expect(cur.deadFor).toBeLessThan(RETIRE_AFTER_S);
    cur = advance(cur, 200);
    expect(cur.deadFor).toBeGreaterThanOrEqual(RETIRE_AFTER_S);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- --run src/components/chain/lifecycle.test.ts`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the implementation**

Create `src/components/chain/lifecycle.ts`:

```ts
/**
 * How a packet moves and when it retires.
 *
 * Kept out of the React host so the timing rules are unit-testable under
 * vitest's node environment — the host owns rAF, this owns the arithmetic.
 */

import { CHAIN_STAGES } from "../../lib/chain";
import type { ChainPacket } from "../../lib/chainReplay";
import type { LivePacket } from "./renderers/types";

/** Traversal bounds. A 40ms call still has to be watchable; a 90s one must not crawl. */
const MIN_TRAVEL_MS = 400;
const MAX_TRAVEL_MS = 4000;

/** How long a finished packet lingers on screen for its decay effects. */
export const RETIRE_AFTER_S = 1.2;

/**
 * Traversal time for one packet, scaled to its real `latency_ms`.
 *
 * Compressed logarithmically rather than linearly: real latencies span three
 * orders of magnitude, and a linear map would make every fast call
 * indistinguishable while a slow one blocked the chain for a minute.
 */
export function travelDurationMs(packet: ChainPacket): number {
  const raw = Number.isFinite(packet.latencyMs) ? Math.max(0, packet.latencyMs) : 0;
  const frac = Math.log10(1 + raw) / Math.log10(1 + 10_000);
  const scaled = MIN_TRAVEL_MS + (MAX_TRAVEL_MS - MIN_TRAVEL_MS) * Math.min(1, frac);
  return Math.min(MAX_TRAVEL_MS, Math.max(MIN_TRAVEL_MS, scaled));
}

/**
 * Step one packet forward. Returns a new object; never mutates the input.
 *
 * `progressLimit` is the packet's own ceiling — a request denied at `rate` may
 * never be drawn past the rate boundary, however long it lingers.
 */
export function advance(live: LivePacket, dtMs: number): LivePacket {
  const limit = live.progressLimit;
  if (live.progress >= limit) {
    return { ...live, progress: limit, deadFor: live.deadFor + dtMs / 1000 };
  }
  const total = travelDurationMs(live.packet);
  const step = (dtMs / total) * (live.packet.stopIndex >= 0 ? limit : 1);
  const progress = Math.min(limit, live.progress + step);
  return {
    ...live,
    progress,
    deadFor: progress >= limit ? live.deadFor + dtMs / 1000 : 0,
  };
}

/** Ceiling a packet may be drawn to, from its stop index. */
export function progressLimitFor(packet: ChainPacket): number {
  return packet.stopIndex < 0 ? 1 : (packet.stopIndex + 1) / CHAIN_STAGES.length;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test -- --run src/components/chain/lifecycle.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/components/chain/lifecycle.ts src/components/chain/lifecycle.test.ts
git commit -m "feat(console): packet lifecycle — traversal scaled to real latency"
```

---

### Task 10: Canvas host

**Files:**
- Create: `src/components/chain/ChainCanvas.tsx`

**Interfaces:**
- Consumes: `createRenderer` (Task 8), `readPalette` (Task 3), `advance`/`progressLimitFor`/`RETIRE_AFTER_S` (Task 9), `ChainPacket` (Task 1), `ChainState`.
- Produces: `ChainCanvas` React component with props `{ style, packets, state, height }`.

No unit test: this is DOM/rAF glue that vitest's node environment cannot exercise. Its logic lives in Tasks 1–9, which are all tested. Verified by `tsc` and by manual browser check in Task 13.

- [ ] **Step 1: Write the component**

Create `src/components/chain/ChainCanvas.tsx`:

```tsx
import { useEffect, useRef } from "react";
import { useReducedMotion } from "framer-motion";
import type { ChainState } from "../../lib/chain";
import type { ChainPacket } from "../../lib/chainReplay";
import type { ChainStyle } from "../../lib/chainStyle";
import { RETIRE_AFTER_S, advance, progressLimitFor } from "./lifecycle";
import { readPalette } from "./palette";
import { createRenderer } from "./renderers";
import type { LivePacket } from "./renderers/types";

interface Props {
  style: ChainStyle;
  /** Packets released so far. The host owns their on-screen lifetime. */
  packets: readonly ChainPacket[];
  state: ChainState;
  height: number;
}

/**
 * Canvas host for the animated chain.
 *
 * Owns exactly the things a renderer must not: the rAF loop, DPR-correct
 * sizing, pausing when the tab is hidden, and the reduced-motion still frame.
 * It is aria-hidden — the accessible representation of the chain is the DOM
 * stage list in Chain.tsx, which is unaffected by any of this.
 */
export function ChainCanvas({ style, packets, state, height }: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  // Live values the rAF loop reads without being torn down and rebuilt.
  const packetsRef = useRef(packets);
  const stateRef = useRef(state);
  packetsRef.current = packets;
  stateRef.current = state;
  const reduced = useReducedMotion();

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const renderer = createRenderer(style);
    if (!renderer) return;

    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    let palette = readPalette(canvas);
    let live: LivePacket[] = [];
    let seen = new Set<string>();
    let raf = 0;
    let last = performance.now();
    let elapsed = 0;
    let w = 0;
    let h = 0;

    function size() {
      const canvasEl = canvasRef.current;
      if (!canvasEl || !ctx) return;
      const rect = canvasEl.getBoundingClientRect();
      const dpr = window.devicePixelRatio || 1;
      w = rect.width;
      h = rect.height;
      canvasEl.width = Math.max(1, Math.round(w * dpr));
      canvasEl.height = Math.max(1, Math.round(h * dpr));
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      renderer!.reset();
    }
    size();

    const ro = new ResizeObserver(size);
    ro.observe(canvas);

    // The palette is theme-dependent; re-read it when the theme attribute flips.
    const themeObserver = new MutationObserver(() => {
      const canvasEl = canvasRef.current;
      if (canvasEl) palette = readPalette(canvasEl);
    });
    themeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });

    function drawOnce(dt: number) {
      if (!ctx) return;
      renderer!.draw({
        ctx,
        w,
        h,
        dt,
        elapsed,
        packets: live,
        state: stateRef.current,
        palette,
        still: Boolean(reduced),
      });
    }

    if (reduced) {
      // One static frame. No loop, no accumulation, nothing that moves.
      drawOnce(0);
      return () => {
        ro.disconnect();
        themeObserver.disconnect();
      };
    }

    function tick(now: number) {
      const dt = Math.min(50, now - last);
      last = now;
      elapsed += dt;

      // Adopt newly released packets.
      for (const p of packetsRef.current) {
        if (seen.has(p.id)) continue;
        seen.add(p.id);
        live.push({ packet: p, progress: 0, progressLimit: progressLimitFor(p), deadFor: 0 });
      }
      // Step and retire.
      live = live.map((lp) => advance(lp, dt)).filter((lp) => lp.deadFor < RETIRE_AFTER_S);
      // `seen` must not grow without bound across a long session.
      if (seen.size > 512) seen = new Set(live.map((lp) => lp.packet.id));

      drawOnce(dt);
      raf = requestAnimationFrame(tick);
    }

    function start() {
      if (raf) return;
      last = performance.now();
      raf = requestAnimationFrame(tick);
    }
    function stop() {
      if (!raf) return;
      cancelAnimationFrame(raf);
      raf = 0;
    }
    function onVisibility() {
      // Match the poll: a hidden tab burns no frames.
      if (document.hidden) stop();
      else start();
    }

    document.addEventListener("visibilitychange", onVisibility);
    if (!document.hidden) start();

    return () => {
      stop();
      ro.disconnect();
      themeObserver.disconnect();
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [style, reduced]);

  return <canvas ref={canvasRef} className="chain-canvas" style={{ height }} aria-hidden />;
}
```

- [ ] **Step 2: Verify it type-checks**

Run: `npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add src/components/chain/ChainCanvas.tsx
git commit -m "feat(console): canvas host — rAF, DPR sizing, visibility pause, reduced-motion still"
```

---

### Task 11: Packet feed hook

**Files:**
- Create: `src/components/chain/useChainPackets.ts`

**Interfaces:**
- Consumes: `useLiveResource` from `src/hooks/useLiveResource.ts`; `apiFetch`, `gatewayAdminRequest` from `src/lib/api.ts`; `diffFeed`, `toPacket`, `releaseSchedule` (Task 1); `latestChainState`, `IDLE_CHAIN` from `src/lib/chain.ts`; `AuditEntry`.
- Produces: `useChainPackets(adminKey): { packets, state, active }`.

No unit test: it is a React hook over a timer and a polling registry, neither of which vitest's node environment provides. The scheduling arithmetic it depends on is fully tested in Task 1.

- [ ] **Step 1: Write the hook**

Create `src/components/chain/useChainPackets.ts`:

```ts
import { useEffect, useRef, useState } from "react";
import { useLiveResource } from "../../hooks/useLiveResource";
import { apiFetch, gatewayAdminRequest } from "../../lib/api";
import { IDLE_CHAIN, latestChainState } from "../../lib/chain";
import type { ChainState } from "../../lib/chain";
import type { ChainPacket } from "../../lib/chainReplay";
import { diffFeed, releaseSchedule, toPacket } from "../../lib/chainReplay";
import type { AuditEntry } from "../../lib/types";

/**
 * MUST stay 5000 and MUST match Overview's audit subscription. The live
 * registry keys by (key, cadence) and dedupes identical subscriptions onto one
 * poll; diverging here would silently double admin API load.
 */
const CADENCE_MS = 5000;

/** How long a released packet stays in the returned array before it is dropped. */
const PACKET_TTL_MS = 8000;

/**
 * Real requests, released on their real timing.
 *
 * Every packet is one audit row. Nothing here invents traffic: with no key, a
 * failed poll, or an empty feed, no packet is ever produced.
 */
export function useChainPackets(adminKey: string): {
  packets: ChainPacket[];
  state: ChainState;
  active: boolean;
} {
  const audit = useLiveResource<AuditEntry[]>(
    `admin/audit?limit=100#${adminKey}`,
    () => apiFetch<AuditEntry[]>(gatewayAdminRequest("/admin/audit?limit=100", adminKey)),
    { enabled: Boolean(adminKey), cadence: CADENCE_MS },
  );

  const [packets, setPackets] = useState<ChainPacket[]>([]);
  const [state, setState] = useState<ChainState>(IDLE_CHAIN);
  const [active, setActive] = useState(false);

  const prevFeed = useRef<AuditEntry[] | null>(null);
  const timers = useRef<number[]>([]);
  const counter = useRef(0);

  // Drop everything when the credential changes — packets from the old key
  // describe traffic the new one may not even be allowed to see.
  useEffect(() => {
    prevFeed.current = null;
    counter.current = 0;
    for (const t of timers.current) window.clearTimeout(t);
    timers.current = [];
    setPackets([]);
    setState(IDLE_CHAIN);
    setActive(false);
  }, [adminKey]);

  useEffect(() => {
    const entries = audit.data;
    // A failed or in-flight poll says nothing about governance: hold the last
    // reading rather than reporting a denial nobody caused.
    if (!adminKey || !entries) return;

    const fresh = diffFeed(prevFeed.current, entries);
    prevFeed.current = entries.slice();
    setState(latestChainState(entries));

    if (fresh.length === 0) return;

    const newPackets = fresh.map((entry) => toPacket(entry, `pkt-${counter.current++}`));
    const delays = releaseSchedule(newPackets, CADENCE_MS);

    setActive(true);
    newPackets.forEach((packet, i) => {
      const release = window.setTimeout(() => {
        setPackets((cur) => [...cur, packet]);
        const expire = window.setTimeout(() => {
          setPackets((cur) => cur.filter((p) => p.id !== packet.id));
        }, PACKET_TTL_MS);
        timers.current.push(expire);
      }, delays[i]);
      timers.current.push(release);
    });

    const quiet = window.setTimeout(() => setActive(false), CADENCE_MS + PACKET_TTL_MS);
    timers.current.push(quiet);
  }, [adminKey, audit.data]);

  // Clear every pending timer on unmount so a navigated-away chain sets no state.
  useEffect(() => {
    const pending = timers.current;
    return () => {
      for (const t of pending) window.clearTimeout(t);
    };
  }, []);

  return { packets, state, active };
}
```

- [ ] **Step 2: Verify it type-checks**

Run: `npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add src/components/chain/useChainPackets.ts
git commit -m "feat(console): chain packet feed on the existing shared audit poll"
```

---

### Task 12: Rewire Chain.tsx

**Files:**
- Modify: `src/components/Chain.tsx` (full rewrite of the component body; keep the file path — `Chain.css` next to it is a guarded instrument file)
- Modify: `src/components/Chain.css` (append)

**Interfaces:**
- Consumes: `useChainPackets` (Task 11), `ChainCanvas` (Task 10), `readChainStyle`/`ChainStyle` (Task 2), existing `CHAIN_STAGES`/`stageRenders`/`IDLE_CHAIN`, `StateIcon`.
- Produces: `Chain` with props `{ adminKey, style }` — `App.tsx` passes the style down in Task 13.

- [ ] **Step 1: Rewrite the component**

Replace the body of `src/components/Chain.tsx` with:

```tsx
import { useState } from "react";
import type { ChainStage, ChainState } from "../lib/chain";
import { CHAIN_STAGES, stageRenders } from "../lib/chain";
import type { ChainStyle } from "../lib/chainStyle";
import { StateIcon } from "../ui/icons";
import { ChainCanvas } from "./chain/ChainCanvas";
import { useChainPackets } from "./chain/useChainPackets";
import "./Chain.css";

/** Canvas height in the always-on topbar strip, and when expanded. */
const STRIP_HEIGHT = 76;
const EXPANDED_HEIGHT = 220;

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
  rate: "rate — the key's org is within its per-org rate limit",
  budget: "budget — the key and its org are within budget",
  guardrail: "guardrail — not halted by injection screening (a lit stage is not proof it ran)",
  upstream: "upstream — the provider or council answered",
  audit: "audit — the outcome was recorded",
};

function outcomeGlyph(state: ChainState, active: boolean) {
  if (active) return { state: "live" as const, label: "request in flight" };
  if (state.outcome === "deny") {
    return { state: "deny" as const, label: `denied at ${state.stoppedAt}` };
  }
  if (state.outcome === "fail") {
    return { state: "hold" as const, label: "cleared governance, provider failed" };
  }
  return { state: "ok" as const, label: "cleared" };
}

interface ChainProps {
  adminKey: string;
  style: ChainStyle;
}

/**
 * The governance chain — the console's signature element.
 *
 * Every AgentOS request runs an ordered gauntlet before a provider is ever
 * called, and that ordering is the whole product, so the shell renders it
 * permanently.
 *
 * The visible treatment is now one of four styles (see lib/chainStyle.ts), but
 * what any of them may draw is unchanged: it is driven by recorded audit
 * evidence (lib/chain.ts), never by an animation timer. A lit stage is
 * evidence. With no key, or no traffic, the chain rests — deliberately,
 * because "quiet" and "healthy" must not look the same as "unknown".
 *
 * The DOM stage list below is the accessible representation and is rendered
 * for every style, including the canvas ones, where it is visually hidden but
 * still read. The canvas is decoration over it, never a replacement.
 */
export function Chain({ adminKey, style }: ChainProps) {
  const { packets, state, active } = useChainPackets(adminKey);
  const [expanded, setExpanded] = useState(false);

  const renders = stageRenders(state);
  const glyph = outcomeGlyph(state, active);
  const progress = state.cleared / CHAIN_STAGES.length;
  const animated = style !== "minimal";

  return (
    <div
      className="chain"
      data-outcome={state.outcome}
      data-active={active || undefined}
      data-style={style}
      data-expanded={expanded || undefined}
    >
      {animated && (
        <ChainCanvas
          style={style}
          packets={packets}
          state={state}
          height={expanded ? EXPANDED_HEIGHT : STRIP_HEIGHT}
        />
      )}

      {!animated && (
        <div className="chain-track" aria-hidden>
          <div className="chain-rule" />
          <div className="chain-fill" style={{ transform: `scaleX(${progress})` }} />
        </div>
      )}

      {/* The accessible chain. Visually hidden under a canvas style — the
          canvas draws the same six stages — but never removed, so screen
          readers get the identical reading in every style. */}
      <ol className="chain-stages" data-hidden={animated || undefined}>
        {CHAIN_STAGES.map((stage, i) => (
          <li key={stage} className="chain-stage" data-render={renders[i]} title={STAGE_TITLES[stage]}>
            <span className="chain-node" aria-hidden />
            <span className="chain-label">{STAGE_LABELS[stage]}</span>
          </li>
        ))}
      </ol>

      {/* A denial is the most important sentence the console speaks — announce it. */}
      <div className="chain-outcome" aria-live="polite">
        <StateIcon state={glyph.state} title={glyph.label} size={12} />
      </div>

      {animated && (
        <button
          type="button"
          className="chain-expand"
          aria-expanded={expanded}
          aria-label={expanded ? "Collapse governance chain" : "Expand governance chain"}
          onClick={() => setExpanded((v) => !v)}
        >
          <span aria-hidden>{expanded ? "▲" : "▼"}</span>
        </button>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Append the styles**

Append to `src/components/Chain.css`:

```css
/* ---- Animated styles -------------------------------------------------- */
/* NOTE: this file is an INSTRUMENT SURFACE. styles.partition.test.ts bans
   brand violet here at any strength — graphite plus signal tokens only. */

.chain[data-style="phosphor"],
.chain[data-style="corridor"],
.chain[data-style="flow"] {
  display: block;
  position: relative;
}

.chain-canvas {
  display: block;
  width: 100%;
  transition: height var(--dur-slow) var(--ease);
}

/* Under a canvas style the DOM chain is the screen-reader copy only. It is
   NOT display:none — that would drop it from the accessibility tree, and the
   canvas is aria-hidden, which would leave the chain unreadable entirely. */
.chain-stages[data-hidden] {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip-path: inset(50%);
  white-space: nowrap;
  border: 0;
}

.chain-expand {
  position: absolute;
  top: 4px;
  right: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  padding: 0;
  font-size: 8px;
  color: var(--text-faint);
  background: none;
  border: 0;
  border-radius: 3px;
  cursor: pointer;
  transition: color var(--dur-fast) var(--ease), background var(--dur-fast) var(--ease);
}

.chain-expand:hover {
  color: var(--text);
  background: var(--border);
}

.chain-expand:focus-visible {
  outline: 1px solid var(--text-dim);
  outline-offset: 1px;
}

@media (prefers-reduced-motion: reduce) {
  .chain-canvas {
    transition: none;
  }
}
```

- [ ] **Step 3: Verify the suite and the chroma guard**

Run: `npm test -- --run`
Expected: PASS — including `chroma partition > keeps violet off instrument surfaces`, which now also covers the appended rules.

- [ ] **Step 4: Commit**

```bash
git add src/components/Chain.tsx src/components/Chain.css
git commit -m "feat(console): chain renders one of four styles, a11y copy retained under canvas"
```

---

### Task 13: Wire the style through the shell and settings

**Files:**
- Modify: `src/App.tsx` (topbar region around line 275-284)
- Modify: `src/components/SettingsModal.tsx` (add picker after the Appearance field, ~line 134)
- Modify: `src/styles.css` (topbar height)

**Interfaces:**
- Consumes: `readChainStyle`, `writeChainStyle`, `CHAIN_STYLES`, `CHAIN_STYLE_LABELS`, `ChainStyle` (Task 2); `Chain` with its new `style` prop (Task 12).

- [ ] **Step 1: Hold the style in App and pass it down**

In `src/App.tsx`, add the import:

```tsx
import { readChainStyle } from "./lib/chainStyle";
import type { ChainStyle } from "./lib/chainStyle";
```

Inside the `App` component, alongside the other `useState` calls:

```tsx
const [chainStyle, setChainStyle] = useState<ChainStyle>(() => readChainStyle());
```

Replace `<Chain adminKey={adminKey} />` (line 283) with:

```tsx
<Chain adminKey={adminKey} style={chainStyle} />
```

Pass the setter to the settings modal — add `chainStyle={chainStyle}` and `onChainStyle={setChainStyle}` to the existing `<SettingsModal ... />` element.

- [ ] **Step 2: Add the picker to SettingsModal**

In `src/components/SettingsModal.tsx`, add to the imports:

```tsx
import { CHAIN_STYLES, CHAIN_STYLE_LABELS, writeChainStyle } from "../lib/chainStyle";
import type { ChainStyle } from "../lib/chainStyle";
```

Add to the `Props` interface:

```tsx
  chainStyle: ChainStyle;
  onChainStyle: (style: ChainStyle) => void;
```

Add `chainStyle` and `onChainStyle` to the destructured parameters.

Insert immediately after the closing `</div>` of the Appearance field (after line 134):

```tsx
      <div className="field">
        <span className="eyebrow">Governance chain</span>
        <select
          className="chain-style-select"
          value={chainStyle}
          aria-label="Governance chain style"
          onChange={(e) => {
            const next = e.target.value as ChainStyle;
            writeChainStyle(next);
            onChainStyle(next);
          }}
        >
          {CHAIN_STYLES.map((s) => (
            <option key={s} value={s}>
              {CHAIN_STYLE_LABELS[s]}
            </option>
          ))}
        </select>
        <p className="muted" style={{ fontSize: "12px", margin: "6px 0 0" }}>
          How the chain above your content is drawn. Every style shows the same
          evidence — only requests the audit log recorded ever appear. Choose{" "}
          <strong>minimal</strong> for the plain rule with no animation.
        </p>
      </div>
```

- [ ] **Step 3: Give the topbar room**

In `src/styles.css`, find the `.topbar` rule and allow it to grow: change its fixed height to

```css
  min-height: 76px;
  height: auto;
  align-items: flex-start;
  padding-top: 10px;
```

Leave every other `.topbar` declaration as-is. If `.topbar` has no `height`, add only the `min-height` line.

- [ ] **Step 4: Verify the full gate**

Run: `npm test -- --run && npm run build`
Expected: all tests PASS (the pre-existing suite plus the new files) and the build succeeds.

- [ ] **Step 5: Manual browser check**

```bash
npm run dev
```

Confirm in the browser:
- The chain renders phosphor by default and fills the topbar.
- Settings → Governance chain switches between all four styles live; a reload keeps the choice.
- `minimal` renders the original hairline chain with no canvas.
- Toggling Appearance dark/light re-colours the canvas without a reload.
- The expand control grows the canvas and `aria-expanded` flips.
- With traffic (send a request through the gateway), marks appear and denials stop at their stage.

- [ ] **Step 6: Commit**

```bash
git add src/App.tsx src/components/SettingsModal.tsx src/styles.css
git commit -m "feat(console): governance chain style picker in settings, topbar given room"
```

---

## Self-Review

**Spec coverage:**

| Spec section | Task |
|---|---|
| §1 Architecture — chainReplay, renderers, canvas host, hook | 1, 3, 5–8, 10, 11 |
| §2 Data flow — diff, ts scheduling, latency scaling | 1, 9, 11 |
| §2 Edge cases — failed poll, no key, empty feed, skew, first load, style switch | 1 (seed/anchor/clamp), 10 (`reset` on style change), 11 (hold on null, key change) |
| §3 Sizing — 76px strip, 220px expanded | 12, 13 |
| §4 `unproven` vocabulary — all three renderers | 5 (no phosphor deposited), 6 (no flare), 7 (dashed column) |
| §5 Settings — key, four values, fallback | 2, 13 |
| §6 Accessibility — aria-hidden canvas, DOM list retained, reduced motion | 10, 12 |
| §7 Performance — DPR, visibility pause, caps | 1 (`MAX_REPLAY_PER_POLL`), 10 |
| §8 Testing — replay, renderers, settings, suite green | 1, 2, 5–9, 13 |

**Gap found and closed:** the spec's §7 "degrade by dropping marks, never frames" had no task; `MAX_REPLAY_PER_POLL` in Task 1 and the `seen`-set trim in Task 10 now implement it, both covered by Task 1's cap test.

**Placeholder scan:** no TBD/TODO; every code step carries complete code. Tasks 10 and 11 ship without unit tests, stated explicitly with the reason (node environment has no DOM/rAF) rather than left implied.

**Type consistency:** `ChainPacket` fields are identical across Tasks 1, 3, 4, 9, 10, 11. `LivePacket`/`RenderFrame` as defined in Task 3 are consumed unchanged in 4–7 and constructed in 10. `createRenderer` returns `ChainRenderer | null` in Task 8 and the null is branched on in Tasks 10 and 12. `progressLimitFor` is defined in Task 9 and used in Task 10. `alpha()` lives once in `src/components/chain/color.ts` (Task 3) and is imported by all three renderers. It parses whatever a CSS custom property resolved to, so it carries real edge cases — unparseable input returns unchanged rather than becoming `rgba(NaN,…)`, which would silently blank a stage — and those are tested once, properly.

**Amendments after the pre-flight scan** (both ruled by the human partner before Task 1):
- `alpha()` extracted to a shared, tested module rather than duplicated three times.
- Tasks 10 and 11 ship without unit tests, reason documented in each. Recorded as a known deferral for the final review to triage, not re-raised per task.
