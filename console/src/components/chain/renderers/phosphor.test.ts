import { describe, expect, it } from "vitest";
import { IDLE_CHAIN } from "../../../lib/chain";
import { createPhosphorRenderer } from "./phosphor";
import { frame, livePacket, stubCtx } from "./testHarness";

describe("phosphor renderer", () => {
  it("draws an idle frame without throwing", () => {
    const r = createPhosphorRenderer();
    expect(() => r.draw(frame())).not.toThrow();
  });

  it("fades rather than clears, so persistence survives between frames", () => {
    const r = createPhosphorRenderer();
    const { ctx, calls } = stubCtx();
    r.draw(frame({ ctx }));
    expect(calls).toContain("fillRect");
    expect(calls).not.toContain("clearRect");
  });

  it("draws a travelling packet", () => {
    const r = createPhosphorRenderer();
    const { ctx, calls } = stubCtx();
    r.draw(frame({ ctx, packets: [livePacket({ progress: 0.4 })] }));
    expect(calls).toContain("stroke");
  });

  it("survives a denied packet at every stage index", () => {
    const r = createPhosphorRenderer();
    for (let i = 0; i < 6; i++) {
      const p = livePacket({
        packet: { ...livePacket().packet, stopIndex: i, outcome: "deny" },
        progress: 1,
        progressLimit: (i + 1) / 6,
        deadFor: 0.2,
      });
      expect(() => r.draw(frame({ packets: [p] }))).not.toThrow();
    }
  });

  it("draws a meaningful still when reduced motion is preferred", () => {
    const r = createPhosphorRenderer();
    const { ctx, calls } = stubCtx();
    r.draw(frame({ ctx, still: true, state: { ...IDLE_CHAIN, cleared: 6 } }));
    expect(calls.length).toBeGreaterThan(0);
    expect(calls).toContain("fillText");
  });

  it("survives a zero-size canvas without dividing by zero", () => {
    const r = createPhosphorRenderer();
    expect(() => r.draw(frame({ w: 0, h: 0 }))).not.toThrow();
  });

  it("reset clears accumulated state and stays drawable", () => {
    const r = createPhosphorRenderer();
    r.draw(frame({ packets: [livePacket()] }));
    r.reset();
    expect(() => r.draw(frame())).not.toThrow();
  });
});
