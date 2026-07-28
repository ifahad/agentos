import { describe, expect, it } from "vitest";
import { CHAIN_STAGES } from "../../../lib/chain";
import {
  ARCH_EDGES,
  ARCH_NODES,
  COUNCIL_MEMBERS,
  GOVERNANCE_FRAME_MS,
  GOVERNANCE_SCRIPT,
  LIFECYCLE_HOPS,
  clearedCount,
} from "./geometry";

describe("architecture geometry", () => {
  it("has unique node ids", () => {
    const ids = ARCH_NODES.map((n) => n.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it("only draws edges between declared nodes", () => {
    const ids = new Set(ARCH_NODES.map((n) => n.id));
    for (const e of ARCH_EDGES) {
      expect(ids.has(e.from)).toBe(true);
      expect(ids.has(e.to)).toBe(true);
    }
  });

  it("keeps every node inside the 640x320 viewBox", () => {
    for (const n of ARCH_NODES) {
      expect(n.x).toBeGreaterThanOrEqual(0);
      expect(n.y).toBeGreaterThanOrEqual(0);
      expect(n.x + n.w).toBeLessThanOrEqual(640);
      expect(n.y + n.h).toBeLessThanOrEqual(320);
    }
  });

  it("states the invariant on the runtime-to-gateway edge", () => {
    const edge = ARCH_EDGES.find((e) => e.from === "runtime" && e.to === "gateway");
    expect(edge?.note).toMatch(/only via the gateway/i);
  });
});

describe("governance script", () => {
  it("only halts at stages the chain actually has", () => {
    for (const f of GOVERNANCE_SCRIPT) {
      if (f.stoppedAt !== null) expect(CHAIN_STAGES).toContain(f.stoppedAt);
    }
  });

  it("never halts at rbac, which is not on the model-call path", () => {
    for (const f of GOVERNANCE_SCRIPT) expect(f.stoppedAt).not.toBe("rbac");
  });

  it("shows a clean pass, a denial, and an upstream failure", () => {
    const outcomes = new Set(GOVERNANCE_SCRIPT.map((f) => f.outcome));
    expect(outcomes).toContain("pass");
    expect(outcomes).toContain("deny");
    expect(outcomes).toContain("fail");
  });

  it("gives every frame a caption", () => {
    for (const f of GOVERNANCE_SCRIPT) expect(f.caption.trim().length).toBeGreaterThan(0);
  });

  it("holds a frame long enough to read", () => {
    expect(GOVERNANCE_FRAME_MS).toBeGreaterThanOrEqual(1500);
  });

  it("clears every stage on a pass and stops short on a denial", () => {
    expect(clearedCount({ stoppedAt: null, outcome: "pass", caption: "" })).toBe(
      CHAIN_STAGES.length,
    );
    expect(clearedCount({ stoppedAt: "rate", outcome: "deny", caption: "" })).toBe(
      CHAIN_STAGES.indexOf("rate"),
    );
  });
});

describe("other visuals", () => {
  it("orders the lifecycle hops left to right", () => {
    const xs = LIFECYCLE_HOPS.map((h) => h.x);
    expect([...xs].sort((a, b) => a - b)).toEqual(xs);
  });

  it("gives the council more than one member, so 'dissent' means something", () => {
    expect(COUNCIL_MEMBERS.length).toBeGreaterThan(1);
  });
});
