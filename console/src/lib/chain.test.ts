import { describe, expect, it } from "vitest";
import {
  CHAIN_STAGES,
  IDLE_CHAIN,
  chainStateFromStatus,
  latestChainState,
  stageRenders,
} from "./chain";

describe("chainStateFromStatus", () => {
  it("clears the whole chain on success", () => {
    const state = chainStateFromStatus(200);
    expect(state).toEqual({ cleared: 5, stoppedAt: null, outcome: "pass" });
  });

  it("treats every 2xx as a pass", () => {
    for (const status of [200, 201, 204, 299]) {
      expect(chainStateFromStatus(status).outcome).toBe("pass");
    }
  });

  it("stops at auth when unauthenticated", () => {
    expect(chainStateFromStatus(401)).toEqual({
      cleared: 0,
      stoppedAt: "auth",
      outcome: "deny",
    });
  });

  it("stops at rbac when forbidden", () => {
    expect(chainStateFromStatus(403)).toEqual({
      cleared: 1,
      stoppedAt: "rbac",
      outcome: "deny",
    });
  });

  it("stops at budget when the key or org is exhausted", () => {
    expect(chainStateFromStatus(402)).toEqual({
      cleared: 2,
      stoppedAt: "budget",
      outcome: "deny",
    });
  });

  it("stops at rate when throttled", () => {
    expect(chainStateFromStatus(429)).toEqual({
      cleared: 3,
      stoppedAt: "rate",
      outcome: "deny",
    });
  });

  it("clears governance but reports failure on an upstream error", () => {
    // A 502 means the checks all passed and the provider broke. Drawing this as
    // a governance denial would blame the wrong component.
    const state = chainStateFromStatus(502);
    expect(state.cleared).toBe(CHAIN_STAGES.length);
    expect(state.stoppedAt).toBeNull();
    expect(state.outcome).toBe("fail");
  });

  it("fails closed on an unrecognised status", () => {
    // Never imply a check ran that we cannot prove ran.
    expect(chainStateFromStatus(418)).toEqual({
      cleared: 0,
      stoppedAt: "auth",
      outcome: "deny",
    });
  });
});

describe("latestChainState", () => {
  it("is idle for an empty feed", () => {
    expect(latestChainState([])).toEqual(IDLE_CHAIN);
  });

  it("does not report an empty feed as a pass", () => {
    // No traffic is not the same as healthy traffic.
    expect(latestChainState([]).cleared).toBe(0);
  });

  it("reads the newest entry, which is first", () => {
    const state = latestChainState([{ status: 429 }, { status: 200 }]);
    expect(state.stoppedAt).toBe("rate");
  });
});

describe("stageRenders", () => {
  it("lights every stage on a clean pass", () => {
    expect(stageRenders(chainStateFromStatus(200))).toEqual([
      "cleared",
      "cleared",
      "cleared",
      "cleared",
      "cleared",
    ]);
  });

  it("leaves stages after the halt unlit", () => {
    expect(stageRenders(chainStateFromStatus(402))).toEqual([
      "cleared",
      "cleared",
      "stopped",
      "unlit",
      "unlit",
    ]);
  });

  it("marks only the halting stage as stopped", () => {
    const renders = stageRenders(chainStateFromStatus(403));
    expect(renders.filter((r) => r === "stopped")).toHaveLength(1);
    expect(renders[1]).toBe("stopped");
  });

  it("returns one render per stage", () => {
    expect(stageRenders(IDLE_CHAIN)).toHaveLength(CHAIN_STAGES.length);
  });
});
