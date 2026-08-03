import { describe, expect, it } from "vitest";
import { CHAIN_STAGES, IDLE_CHAIN } from "../../../lib/chain";
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
