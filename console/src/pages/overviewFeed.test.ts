import { describe, expect, it } from "vitest";
import type { AuditEntry, KeyInfo } from "../lib/types";
import {
  budgetMeters,
  feedEntryId,
  isInflight,
  mergeFeedEntries,
  INFLIGHT_WINDOW_MS,
} from "./overviewFeed";

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

describe("feedEntryId", () => {
  it("is stable for identical entries and differs across fields", () => {
    expect(feedEntryId(entry())).toBe(feedEntryId(entry()));
    expect(feedEntryId(entry({ status: 500 }))).not.toBe(feedEntryId(entry()));
    expect(feedEntryId(entry({ key_name: "dev" }))).not.toBe(feedEntryId(entry()));
  });
});

describe("mergeFeedEntries", () => {
  it("sorts newest first", () => {
    const older = entry({ ts: "2026-07-24T09:00:00Z" });
    const newer = entry({ ts: "2026-07-24T11:00:00Z" });
    const merged = mergeFeedEntries([], [older, newer], 10);
    expect(merged.map((e) => e.ts)).toEqual([newer.ts, older.ts]);
  });

  it("dedupes entries already in the feed", () => {
    const a = entry({ ts: "2026-07-24T11:00:00Z" });
    const b = entry({ ts: "2026-07-24T10:00:00Z" });
    const merged = mergeFeedEntries([a, b], [a], 10);
    expect(merged).toHaveLength(2);
    expect(merged[0]).toBe(a);
  });

  it("caps the feed, dropping the oldest entries", () => {
    const current = [entry({ ts: "2026-07-24T10:00:00Z" })];
    const incoming = [
      entry({ ts: "2026-07-24T12:00:00Z" }),
      entry({ ts: "2026-07-24T11:00:00Z" }),
    ];
    const merged = mergeFeedEntries(current, incoming, 2);
    expect(merged.map((e) => e.ts)).toEqual([
      "2026-07-24T12:00:00Z",
      "2026-07-24T11:00:00Z",
    ]);
  });

  it("treats unparseable timestamps as oldest and tolerates cap <= 0", () => {
    const bad = entry({ ts: "not-a-date" });
    const good = entry({ ts: "2026-07-24T10:00:00Z" });
    const merged = mergeFeedEntries([], [bad, good], 0);
    expect(merged.map((e) => e.ts)).toEqual([good.ts, bad.ts]);
  });
});

describe("isInflight", () => {
  const now = Date.parse("2026-07-24T10:00:10Z");

  it("is true inside the window, false outside it", () => {
    expect(isInflight(entry({ ts: "2026-07-24T10:00:05Z" }), now)).toBe(true);
    expect(isInflight(entry({ ts: "2026-07-24T09:00:00Z" }), now)).toBe(false);
  });

  it("includes the window boundary and rejects future timestamps", () => {
    const edge = new Date(now - INFLIGHT_WINDOW_MS).toISOString();
    expect(isInflight(entry({ ts: edge }), now)).toBe(true);
    expect(isInflight(entry({ ts: "2026-07-24T10:00:30Z" }), now)).toBe(false);
    expect(isInflight(entry({ ts: "garbage" }), now)).toBe(false);
  });
});

describe("budgetMeters", () => {
  const keys: KeyInfo[] = [
    { name: "unlimited", monthly_budget_usd: 0, spend_usd: 12 },
    { name: "prod", monthly_budget_usd: 100, spend_usd: 90 },
    { name: "dev", monthly_budget_usd: 50, spend_usd: 5 },
    { name: "staging", monthly_budget_usd: 50, spend_usd: 25 },
  ];

  it("drops budget-less keys and sorts fullest first", () => {
    const rows = budgetMeters(keys);
    expect(rows.map((r) => r.name)).toEqual(["prod", "staging", "dev"]);
    expect(rows[0].fraction).toBeCloseTo(0.9);
  });

  it("clamps over-budget fractions to 1 and caps the list", () => {
    const over: KeyInfo[] = [
      { name: "a", monthly_budget_usd: 10, spend_usd: 30 },
      { name: "b", monthly_budget_usd: 10, spend_usd: 5 },
    ];
    const rows = budgetMeters(over, 1);
    expect(rows).toHaveLength(1);
    expect(rows[0]).toMatchObject({ name: "a", fraction: 1 });
  });

  it("returns no rows for empty input", () => {
    expect(budgetMeters([])).toEqual([]);
  });
});
