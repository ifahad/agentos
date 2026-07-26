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
