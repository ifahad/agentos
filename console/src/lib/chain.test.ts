import { describe, expect, it } from "vitest";
import {
  CHAIN_STAGES,
  IDLE_CHAIN,
  chainStateFromStatus,
  latestChainState,
  stageRenders,
} from "./chain";

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

  it("treats every 2xx as a pass", () => {
    for (const status of [200, 201, 204, 299]) {
      expect(chainStateFromStatus(status).outcome).toBe("pass");
    }
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
      "unlit",
    ]);
  });

  it("returns one render per stage", () => {
    expect(stageRenders(IDLE_CHAIN)).toHaveLength(CHAIN_STAGES.length);
  });
});
