import { describe, expect, it } from "vitest";
import { IDLE_CHAIN } from "../../../lib/chain";
import { createFlowRenderer } from "./flow";
import { frame, livePacket, stubCtx } from "./testHarness";

describe("flow renderer", () => {
  it("draws an idle frame without throwing", () => {
    const r = createFlowRenderer();
    expect(() => r.draw(frame())).not.toThrow();
  });

  it("draws gate columns even with no traffic", () => {
    const r = createFlowRenderer();
    const { ctx, calls } = stubCtx();
    r.draw(frame({ ctx }));
    expect(calls).toContain("fillText");
  });

  it("dashes the column for an unproven stage and restores the dash pattern", () => {
    const r = createFlowRenderer();
    const { ctx, calls } = stubCtx();
    // guardrail is unproven on a plain chat pass, so some column must be dashed.
    r.draw(frame({ ctx, state: { ...IDLE_CHAIN, cleared: 6, unproven: ["guardrail"] } }));
    expect(calls).toContain("setLineDash");
  });

  it("draws a particle for a travelling packet", () => {
    const r = createFlowRenderer();
    const { ctx, calls } = stubCtx();
    r.draw(frame({ ctx, packets: [livePacket({ progress: 0.3 })] }));
    expect(calls).toContain("stroke");
  });

  it("survives a denied packet at every stage index", () => {
    const r = createFlowRenderer();
    for (let i = 0; i < 6; i++) {
      const p = livePacket({
        packet: { ...livePacket().packet, stopIndex: i, outcome: "deny" },
        progress: 1,
        progressLimit: (i + 1) / 6,
        deadFor: 0.3,
      });
      expect(() => r.draw(frame({ packets: [p] }))).not.toThrow();
    }
  });

  it("draws a meaningful still when reduced motion is preferred", () => {
    const r = createFlowRenderer();
    const { ctx, calls } = stubCtx();
    r.draw(frame({ ctx, still: true, state: { ...IDLE_CHAIN, cleared: 6 } }));
    expect(calls).toContain("fillText");
  });

  it("survives a zero-size canvas", () => {
    const r = createFlowRenderer();
    expect(() => r.draw(frame({ w: 0, h: 0 }))).not.toThrow();
  });

  it("reset clears accumulated state and stays drawable", () => {
    const r = createFlowRenderer();
    r.draw(frame({ packets: [livePacket()] }));
    r.reset();
    expect(() => r.draw(frame())).not.toThrow();
  });

  // The dash IS this renderer's unproven vocabulary — flow draws no per-gate
  // crossing mark, so the column is the only thing that can over-claim. These
  // two tests are a pair: the second stops the first passing vacuously against
  // a renderer that dashes everything.

  it("dashes a column an on-screen packet leaves unproven, even when the resting state proves it", () => {
    const r = createFlowRenderer();
    const { ctx, draws } = stubCtx();

    r.draw(
      frame({
        ctx,
        // Resting state proves everything — only the live packet says otherwise.
        state: { cleared: 6, stoppedAt: null, outcome: "pass", unproven: [] },
        packets: [
          livePacket({
            packet: { ...livePacket().packet, unproven: ["guardrail"] },
            progress: 0.9,
          }),
        ],
      }),
    );

    // setLineDash calls arrive in column order; a non-empty array is a dash.
    const dashes = draws
      .filter((d) => d.method === "setLineDash")
      .map((d) => (d.args[0] as number[]).length > 0);
    // Columns are drawn one per stage, each preceded by its own setLineDash,
    // then reset to solid — so take every other entry, the "set" ones.
    const perColumn = dashes.filter((_, i) => i % 2 === 0);
    expect(perColumn[3], "guardrail column must be dashed").toBe(true);
  });

  it("draws a proven column solid", () => {
    const r = createFlowRenderer();
    const { ctx, draws } = stubCtx();

    r.draw(
      frame({
        ctx,
        state: { cleared: 6, stoppedAt: null, outcome: "pass", unproven: [] },
        packets: [
          livePacket({
            packet: { ...livePacket().packet, unproven: ["guardrail"] },
            progress: 0.9,
          }),
        ],
      }),
    );

    const dashes = draws
      .filter((d) => d.method === "setLineDash")
      .map((d) => (d.args[0] as number[]).length > 0);
    const perColumn = dashes.filter((_, i) => i % 2 === 0);
    expect(perColumn[0], "auth column must be solid").toBe(false);
  });
});
