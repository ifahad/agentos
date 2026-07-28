import { describe, expect, it } from "vitest";
import {
  CHAIN_STAGES,
  IDLE_CHAIN,
  chainStateFromEntry,
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

describe("chainStateFromEntry", () => {
  it("clears every stage on a chat pass, but leaves guardrail unproven", () => {
    expect(chainStateFromEntry({ status: 200, kind: "chat" })).toEqual({
      cleared: CHAIN_STAGES.length,
      stoppedAt: null,
      outcome: "pass",
      unproven: ["guardrail"],
    });
  });

  it("clears every stage on an embeddings pass — that path has no screener at all", () => {
    expect(chainStateFromEntry({ status: 200, kind: "embeddings" })).toEqual({
      cleared: CHAIN_STAGES.length,
      stoppedAt: null,
      outcome: "pass",
      unproven: ["guardrail"],
    });
  });

  it("treats every 2xx chat status as a pass", () => {
    for (const status of [200, 201, 204, 299]) {
      expect(chainStateFromEntry({ status, kind: "chat" }).outcome).toBe("pass");
    }
  });

  it("proves guardrail ran on a guardrail_flag row (flagged but forwarded in log mode)", () => {
    expect(chainStateFromEntry({ status: 200, kind: "guardrail_flag" })).toEqual({
      cleared: CHAIN_STAGES.length,
      stoppedAt: null,
      outcome: "pass",
      unproven: [],
    });
  });

  it("proves guardrail ran on a guardrail_error row (classifier failed open)", () => {
    expect(chainStateFromEntry({ status: 200, kind: "guardrail_error" })).toEqual({
      cleared: CHAIN_STAGES.length,
      stoppedAt: null,
      outcome: "pass",
      unproven: [],
    });
  });

  it("stops at guardrail on guardrail_block, having cleared auth, rate, and budget", () => {
    expect(chainStateFromEntry({ status: 400, kind: "guardrail_block" })).toEqual({
      cleared: 3,
      stoppedAt: "guardrail",
      outcome: "deny",
      unproven: [],
    });
  });

  it("stops at rate on rate_limited, having cleared auth only", () => {
    expect(chainStateFromEntry({ status: 429, kind: "rate_limited" })).toEqual({
      cleared: 1,
      stoppedAt: "rate",
      outcome: "deny",
      unproven: [],
    });
  });

  it("stops at upstream on a chat row with a provider 401 — not a caller-auth denial", () => {
    expect(chainStateFromEntry({ status: 401, kind: "chat" })).toEqual({
      cleared: 4,
      stoppedAt: "upstream",
      outcome: "fail",
      unproven: ["guardrail"],
    });
  });

  it("stops at upstream on a chat row with a provider 429 — not a caller rate-limit denial", () => {
    expect(chainStateFromEntry({ status: 429, kind: "chat" })).toEqual({
      cleared: 4,
      stoppedAt: "upstream",
      outcome: "fail",
      unproven: ["guardrail"],
    });
  });

  it("stops at upstream on a chat row with a provider 400 — not a guardrail denial", () => {
    expect(chainStateFromEntry({ status: 400, kind: "chat" })).toEqual({
      cleared: 4,
      stoppedAt: "upstream",
      outcome: "fail",
      unproven: ["guardrail"],
    });
  });

  it("stops at upstream on a 5xx embeddings row: governance cleared, the provider failed", () => {
    expect(chainStateFromEntry({ status: 502, kind: "embeddings" })).toEqual({
      cleared: 4,
      stoppedAt: "upstream",
      outcome: "fail",
      unproven: ["guardrail"],
    });
  });

  it("fails closed on an unknown kind (secret_reload is a real gateway kind, but not a /v1/* request), never as a pass", () => {
    expect(chainStateFromEntry({ status: 200, kind: "secret_reload" })).toEqual({
      cleared: 0,
      stoppedAt: "auth",
      outcome: "deny",
      unproven: [],
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
    const state = latestChainState([
      { status: 429, kind: "rate_limited" },
      { status: 200, kind: "chat" },
    ]);
    expect(state.stoppedAt).toBe("rate");
  });
});

describe("stageRenders", () => {
  it("lights every stage when guardrail is proven to have run", () => {
    expect(stageRenders(chainStateFromEntry({ status: 200, kind: "guardrail_flag" }))).toEqual([
      "cleared",
      "cleared",
      "cleared",
      "cleared",
      "cleared",
      "cleared",
    ]);
  });

  it("leaves guardrail unlit on a bare chat pass — no evidence the screener ran", () => {
    expect(stageRenders(chainStateFromEntry({ status: 200, kind: "chat" }))).toEqual([
      "cleared",
      "cleared",
      "cleared",
      "unlit",
      "cleared",
      "cleared",
    ]);
  });

  it("leaves guardrail unlit on a bare embeddings pass — the path has no screener", () => {
    expect(stageRenders(chainStateFromEntry({ status: 200, kind: "embeddings" }))).toEqual([
      "cleared",
      "cleared",
      "cleared",
      "unlit",
      "cleared",
      "cleared",
    ]);
  });

  it("leaves stages after the halt unlit", () => {
    expect(stageRenders(chainStateFromEntry({ status: 400, kind: "guardrail_block" }))).toEqual([
      "cleared",
      "cleared",
      "cleared",
      "stopped",
      "unlit",
      "unlit",
    ]);
  });

  it("leaves guardrail unlit on an upstream failure too, not just on a pass", () => {
    // guard.Screen() runs (or is skipped) entirely before proxy() is called,
    // so a provider 502 is exactly as uninformative about guardrail as a 2xx
    // is — the node must not light up just because the halt landed later.
    expect(stageRenders(chainStateFromEntry({ status: 502, kind: "chat" }))).toEqual([
      "cleared",
      "cleared",
      "cleared",
      "unlit",
      "stopped",
      "unlit",
    ]);
  });

  it("marks only the halting stage as stopped", () => {
    const renders = stageRenders(chainStateFromEntry({ status: 429, kind: "rate_limited" }));
    expect(renders.filter((r) => r === "stopped")).toHaveLength(1);
    expect(renders[1]).toBe("stopped");
  });

  it("returns one render per stage", () => {
    expect(stageRenders(IDLE_CHAIN)).toHaveLength(CHAIN_STAGES.length);
  });
});
