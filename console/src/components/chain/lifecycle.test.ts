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
