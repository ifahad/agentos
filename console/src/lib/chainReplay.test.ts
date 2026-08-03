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
