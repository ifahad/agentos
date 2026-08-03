import { describe, expect, it } from "vitest";
import { CHAIN_STAGES, IDLE_CHAIN } from "../../../lib/chain";
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

  it("bounds its per-packet bookkeeping instead of growing for the session", () => {
    // `seen` and `deposited` previously held one entry per packet forever —
    // ~8.6k an hour at MAX_REPLAY_PER_POLL on a console left open. The cap
    // prunes to the ids currently on screen, safe only because the host never
    // re-presents a retired packet (ChainCanvas.tsx:197-209). "It was pruned"
    // is observable as a forgotten id depositing afresh.
    const passing = (id: string) =>
      livePacket({ packet: { ...livePacket().packet, id }, progress: 1, progressLimit: 1 });
    const deposits = (draws: ReturnType<typeof stubCtx>["draws"]) =>
      draws.filter((d) => d.method === "fillRect" && d.args[2] === 2 && d.args[3] === 6).length;

    // Control: under the cap, a packet already deposited does not deposit again.
    const small = createPhosphorRenderer();
    const ctlCtx = stubCtx();
    for (let n = 0; n < 10; n++) small.draw(frame({ ctx: ctlCtx.ctx, packets: [passing(`p${n}`)] }));
    ctlCtx.draws.length = 0;
    small.draw(frame({ ctx: ctlCtx.ctx, packets: [passing("p0")] }));
    expect(deposits(ctlCtx.draws), "a remembered packet must not re-deposit").toBe(0);

    // Over the cap the oldest ids are gone, so the same id deposits afresh.
    const big = createPhosphorRenderer();
    const bigCtx = stubCtx();
    for (let n = 0; n < 600; n++) big.draw(frame({ ctx: bigCtx.ctx, packets: [passing(`p${n}`)] }));
    bigCtx.draws.length = 0;
    big.draw(frame({ ctx: bigCtx.ctx, packets: [passing("p0")] }));
    expect(deposits(bigCtx.draws), "the bookkeeping must have been pruned").toBeGreaterThan(0);
  });

  it("deposits nothing at unproven stages (guardrail on a passing chat)", () => {
    const r = createPhosphorRenderer();
    const { ctx, draws } = stubCtx();
    const w = 900;
    const h = 76;
    const pad = Math.min(26, w * 0.05);

    // Packet travels all the way across with guardrail unproven
    r.draw(
      frame({
        ctx,
        w,
        h,
        packets: [
          livePacket({
            packet: { ...livePacket().packet, unproven: ["guardrail"] },
            progress: 1,
            progressLimit: 1,
          }),
        ],
      })
    );

    // Calculate guardrail stage position (stage 3)
    const guardRailIndex = 3;
    const guardrailX = pad + (w - pad * 2) * ((guardRailIndex + 0.5) / CHAIN_STAGES.length);

    // Look for fillRect calls at guardrail position — there should be none
    const depositsAtGuardrail = draws.filter(
      (d) => d.method === "fillRect" && Math.abs((d.args[0] as number) + 1 - guardrailX) < 1.5
    );
    expect(depositsAtGuardrail).toHaveLength(0);
  });

  it("deposits at proven stages when pulse crosses (auth is always proven)", () => {
    const r = createPhosphorRenderer();
    const { ctx, draws } = stubCtx();
    const w = 900;
    const h = 76;
    const pad = Math.min(26, w * 0.05);

    // Packet travels all the way across, auth is proven
    r.draw(
      frame({
        ctx,
        w,
        h,
        packets: [
          livePacket({
            packet: { ...livePacket().packet, unproven: ["guardrail"] },
            progress: 1,
            progressLimit: 1,
          }),
        ],
      })
    );

    // Calculate auth stage position (stage 0)
    const authIndex = 0;
    const authX = pad + (w - pad * 2) * ((authIndex + 0.5) / CHAIN_STAGES.length);

    // Look for fillRect calls at auth position — there should be at least one deposit
    const depositsAtAuth = draws.filter(
      (d) => d.method === "fillRect" && Math.abs((d.args[0] as number) + 1 - authX) < 1.5
    );
    expect(depositsAtAuth.length).toBeGreaterThan(0);
  });
});
