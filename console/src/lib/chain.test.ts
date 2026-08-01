import { describe, expect, it } from "vitest";
import {
  CHAIN_STAGES,
  IDLE_CHAIN,
  chainFeedCount,
  chainStateFromEntry,
  latestChainState,
  stageRenders,
} from "./chain";
import type { ChainEvidence, GatewayAuditKind } from "./chain";

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
      unproven: ["upstream"],
    });
  });

  it("proves guardrail ran on a guardrail_error row (classifier failed open)", () => {
    expect(chainStateFromEntry({ status: 200, kind: "guardrail_error" })).toEqual({
      cleared: CHAIN_STAGES.length,
      stoppedAt: null,
      outcome: "pass",
      unproven: ["upstream"],
    });
  });

  it("leaves upstream unproven on guardrail rows — they are written before proxy() is called", () => {
    // The guard.Screen() switch in handleChatCompletions runs entirely ahead
    // of proxyCouncil()/proxy(), so neither row is evidence the provider
    // answered. In log mode on a slow or streaming completion this row is the
    // newest for the whole in-flight window; the chain must not assert the
    // provider answered while the request is still open.
    for (const kind of ["guardrail_flag", "guardrail_error"] as const) {
      expect(chainStateFromEntry({ status: 200, kind }).unproven).toContain("upstream");
    }
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
      cleared: CHAIN_STAGES.length,
      stoppedAt: "upstream",
      outcome: "fail",
      unproven: ["guardrail"],
    });
  });

  it("stops at upstream on a chat row with a provider 429 — not a caller rate-limit denial", () => {
    expect(chainStateFromEntry({ status: 429, kind: "chat" })).toEqual({
      cleared: CHAIN_STAGES.length,
      stoppedAt: "upstream",
      outcome: "fail",
      unproven: ["guardrail"],
    });
  });

  it("stops at upstream on a chat row with a provider 400 — not a guardrail denial", () => {
    expect(chainStateFromEntry({ status: 400, kind: "chat" })).toEqual({
      cleared: CHAIN_STAGES.length,
      stoppedAt: "upstream",
      outcome: "fail",
      unproven: ["guardrail"],
    });
  });

  it("stops at upstream on a 5xx embeddings row: governance cleared, the provider failed", () => {
    expect(chainStateFromEntry({ status: 502, kind: "embeddings" })).toEqual({
      cleared: CHAIN_STAGES.length,
      stoppedAt: "upstream",
      outcome: "fail",
      unproven: ["guardrail"],
    });
  });

  it("still proves audit on an upstream failure — the row being read is the audit record", () => {
    // proxy() wrote this very row, so the outcome demonstrably was recorded.
    // Leaving audit dark would under-report a check we hold the evidence for.
    for (const status of [400, 401, 429, 502]) {
      const state = chainStateFromEntry({ status, kind: "chat" });
      expect(state.unproven).not.toContain("audit");
      expect(state.cleared).toBe(CHAIN_STAGES.length);
    }
  });

  it("places no request for a secret_reload row — it is an admin action, not a /v1/* call", () => {
    // handleSecretsReload writes this kind when an operator clicks Reload on
    // the Secrets page. Reading it as a denial would announce a refusal for a
    // request nobody made.
    expect(chainStateFromEntry({ status: 200, kind: "secret_reload" })).toEqual(IDLE_CHAIN);
  });

  it("fails closed on a genuinely unknown kind, never as a pass", () => {
    const unknown = "quantum_flux" as GatewayAuditKind;
    expect(chainStateFromEntry({ status: 200, kind: unknown })).toEqual({
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

  it("stays idle on a feed of nothing but a secret_reload — no request, no claim", () => {
    // Clicking Reload on the Secrets page must not turn the topbar chain red
    // and announce "denied at auth" for a request that was never made.
    expect(latestChainState([{ status: 200, kind: "secret_reload" }])).toEqual(IDLE_CHAIN);
  });

  it("does not hide the last real request behind a newer admin-plane row", () => {
    // Filter first, then take the head: a reload landing after a chat pass
    // must neither place a request of its own nor blank out the chat's.
    const state = latestChainState([
      { status: 200, kind: "secret_reload" },
      { status: 429, kind: "rate_limited" },
    ]);
    expect(state.stoppedAt).toBe("rate");
    expect(state.outcome).toBe("deny");
  });

  it("still fails closed on an unknown kind at the head — filtering is not a blanket amnesty", () => {
    const unknown = "quantum_flux" as GatewayAuditKind;
    expect(latestChainState([{ status: 200, kind: unknown }])).toEqual({
      cleared: 0,
      stoppedAt: "auth",
      outcome: "deny",
      unproven: [],
    });
  });
});

describe("stageRenders", () => {
  it("lights guardrail — but not upstream — on a guardrail_flag row", () => {
    // The screener provably ran; the provider provably was not reached yet.
    expect(stageRenders(chainStateFromEntry({ status: 200, kind: "guardrail_flag" }))).toEqual([
      "cleared",
      "cleared",
      "cleared",
      "cleared",
      "unlit",
      "cleared",
    ]);
  });

  it("leaves upstream unlit on a guardrail_error row too", () => {
    expect(stageRenders(chainStateFromEntry({ status: 200, kind: "guardrail_error" }))).toEqual([
      "cleared",
      "cleared",
      "cleared",
      "cleared",
      "unlit",
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
    // audit does light, though: proxy() wrote the row being read.
    expect(stageRenders(chainStateFromEntry({ status: 502, kind: "chat" }))).toEqual([
      "cleared",
      "cleared",
      "cleared",
      "unlit",
      "stopped",
      "cleared",
    ]);
  });

  it("draws nothing at all for an admin-plane row", () => {
    expect(stageRenders(chainStateFromEntry({ status: 200, kind: "secret_reload" }))).toEqual([
      "unlit",
      "unlit",
      "unlit",
      "unlit",
      "unlit",
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

describe("chainFeedCount", () => {
  it("counts only entries that describe a /v1 request", () => {
    const entries: ChainEvidence[] = [
      { status: 200, kind: "chat" },
      { status: 200, kind: "secret_reload" },
      { status: 200, kind: "embeddings" },
    ];
    expect(chainFeedCount(entries)).toBe(2);
  });

  it("returns 0 for an admin-plane-only feed", () => {
    expect(chainFeedCount([{ status: 200, kind: "secret_reload" }])).toBe(0);
  });

  it("agrees with latestChainState on what it ignores", () => {
    // A feed that grew by an admin-plane row alone places nothing on the
    // chain, so it must not be reported as growth either.
    const before: ChainEvidence[] = [{ status: 200, kind: "chat" }];
    const after: ChainEvidence[] = [{ status: 200, kind: "secret_reload" }, ...before];
    expect(latestChainState(after)).toEqual(latestChainState(before));
    expect(chainFeedCount(after)).toBe(chainFeedCount(before));
  });
});
