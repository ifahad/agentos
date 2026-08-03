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
        packet: { ...livePacket().packet, id: `deny-${i}`, stopIndex: i, outcome: "deny" },
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

  it("deny tint decays and does not persist to later packets at the same gate", () => {
    const r = createCorridorRenderer();
    const { ctx, draws } = stubCtx();
    const w = 900;
    const pad = Math.min(26, w * 0.05);
    const rateX = pad + (w - pad * 2) * ((1 + 0.5) / CHAIN_STAGES.length);

    // Frame 1: register a denial at rate (index 1)
    r.draw(
      frame({
        ctx,
        w,
        dt: 16,
        packets: [
          livePacket({
            packet: { ...livePacket().packet, id: "denied-packet", stopIndex: 1, outcome: "deny" },
            progress: (1 + 0.5) / CHAIN_STAGES.length,
            progressLimit: (1 + 1) / CHAIN_STAGES.length,
            deadFor: 0.1,
          }),
        ],
      }),
    );

    // Frame 2: render with denial active (gates render after denial is registered)
    draws.length = 0;
    r.draw(frame({ ctx, w, dt: 16, packets: [] }));

    // Capture deny-tinted flares in frame 2 (should exist with fix, missing/live without)
    const denyFlares = draws.filter(
      (d) =>
        d.method === "fillRect" &&
        d.args[2] === 2 &&
        Math.abs((d.args[0] as number) + 1 - rateX) < 1.5,
    );
    // At least one flare should be in deny color (rgba(226, 104, 95, ...))
    const denyColorInSecondFrame = denyFlares.some((f) => {
      const style = String(f.fillStyle);
      return style.includes("226") && style.includes("104") && style.includes("95");
    });
    expect(denyColorInSecondFrame).toBe(true);

    // Frame 3: advance time so deny flare decays completely (FLARE_DECAY_MS = 260ms)
    draws.length = 0;
    r.draw(frame({ ctx, w, dt: 300, packets: [] }));

    // Frame 4: different packet passes through rate with proof
    draws.length = 0;
    r.draw(
      frame({
        ctx,
        w,
        packets: [
          livePacket({
            packet: { ...livePacket().packet, id: "passing-packet", unproven: [], outcome: "pass" },
            progress: 1,
            progressLimit: 1,
          }),
        ],
      }),
    );

    // Look for the flare mark at rate's position in frame 4
    const liveFlares = draws.filter(
      (d) =>
        d.method === "fillRect" &&
        d.args[2] === 2 &&
        Math.abs((d.args[0] as number) + 1 - rateX) < 1.5,
    );

    // After decay, the flare mark should be in live color, not deny
    expect(liveFlares.length).toBeGreaterThan(0);
    const liveColorInFourthFrame = liveFlares.some((f) => {
      const style = String(f.fillStyle);
      // FALLBACK_PALETTE.live is "#5ad1c4" which converts to rgba(90, 209, 196, ...)
      return style.includes("90") && style.includes("209") && style.includes("196");
    });
    expect(liveColorInFourthFrame).toBe(true);

    // And it should NOT still be in deny color (the bug would make it permanently deny)
    const stillDenyColored = liveFlares.some((f) => {
      const style = String(f.fillStyle);
      return style.includes("226") && style.includes("104") && style.includes("95");
    });
    expect(stillDenyColored).toBe(false);
  });

  it("bounds its per-packet bookkeeping instead of growing for the session", () => {
    // `handled` fires a gate's slam once per packet and previously held every
    // id forever — ~8.6k an hour at MAX_REPLAY_PER_POLL on a console left
    // open. The cap prunes to the ids currently on screen, which is only safe
    // because the host never re-presents a retired packet (ChainCanvas.tsx:
    // 197-209). "It was pruned" is observable as the slam being free to fire
    // again for an id the renderer has forgotten.
    const denied = (id: string) =>
      livePacket({
        packet: { ...livePacket().packet, id, stopIndex: 1, outcome: "deny" },
        progress: 2 / CHAIN_STAGES.length,
        progressLimit: 2 / CHAIN_STAGES.length,
        deadFor: 0.1,
      });
    const sparksIn = (draws: ReturnType<typeof stubCtx>["draws"]) =>
      draws.filter((d) => d.method === "fillRect" && d.args[2] === 1.6).length;

    // Control: under the cap, an id already handled never slams twice.
    const small = createCorridorRenderer();
    const ctlCtx = stubCtx();
    for (let n = 0; n < 10; n++) small.draw(frame({ ctx: ctlCtx.ctx, dt: 16, packets: [denied(`d${n}`)] }));
    // Let the existing sparks die (SPARK_LIFE_MS 620) before re-presenting.
    small.draw(frame({ ctx: ctlCtx.ctx, dt: 50, packets: [] }));
    for (let n = 0; n < 20; n++) small.draw(frame({ ctx: ctlCtx.ctx, dt: 50, packets: [] }));
    ctlCtx.draws.length = 0;
    small.draw(frame({ ctx: ctlCtx.ctx, dt: 16, packets: [denied("d0")] }));
    expect(sparksIn(ctlCtx.draws), "a remembered id must not slam twice").toBe(0);

    // Over the cap, the oldest ids have been pruned, so the same re-presented
    // id is treated as new — which is exactly the state the cap trades for a
    // bounded set, and is unreachable in the host.
    const big = createCorridorRenderer();
    const bigCtx = stubCtx();
    for (let n = 0; n < 600; n++) big.draw(frame({ ctx: bigCtx.ctx, dt: 16, packets: [denied(`d${n}`)] }));
    bigCtx.draws.length = 0;
    big.draw(frame({ ctx: bigCtx.ctx, dt: 16, packets: [denied("d0")] }));
    expect(sparksIn(bigCtx.draws), "the bookkeeping must have been pruned").toBeGreaterThan(0);
  });

  it("slams a provider failure in the hold hue, not the governance-denial hue", () => {
    // A `chat` row with a 5xx: chain.ts returns outcome "fail", stoppedAt
    // "upstream" — governance cleared end to end and the PROVIDER broke.
    // Corridor used to paint every stop in palette.deny, so the same frame
    // contradicted itself: the upstream label rendered --hold while the gate
    // flare and its sparks rendered --deny, announcing a refusal AgentOS
    // never made. See lib/chain.ts:20-29.
    const r = createCorridorRenderer();
    const { ctx, draws } = stubCtx();
    const w = 900;
    const pad = Math.min(26, w * 0.05);
    const upstreamIndex = 4;
    const upstreamX = pad + (w - pad * 2) * ((upstreamIndex + 0.5) / CHAIN_STAGES.length);

    const failed = livePacket({
      packet: {
        ...livePacket().packet,
        id: "provider-500",
        stopIndex: upstreamIndex,
        outcome: "fail",
      },
      progress: (upstreamIndex + 1) / CHAIN_STAGES.length,
      progressLimit: (upstreamIndex + 1) / CHAIN_STAGES.length,
      deadFor: 0.1,
    });

    // Frame 1 registers the stop; frame 2 is the one that draws it (the
    // renderer's flare is set after the loop that paints it).
    r.draw(frame({ ctx, w, dt: 16, packets: [failed] }));
    draws.length = 0;
    r.draw(frame({ ctx, w, dt: 16, packets: [failed] }));

    const hold = "227,168,81"; // FALLBACK_PALETTE.hold #e3a851
    const deny = "226,104,95"; // FALLBACK_PALETTE.deny #e2685f

    // The gate flare at upstream: a 2px-wide fillRect (the 3px gate body is
    // furniture drawn every frame regardless).
    const flares = draws.filter(
      (d) =>
        d.method === "fillRect" &&
        d.args[2] === 2 &&
        Math.abs((d.args[0] as number) + 1 - upstreamX) < 1.5,
    );
    expect(flares.length, "the failed request must still slam its gate").toBeGreaterThan(0);
    expect(flares.every((f) => String(f.fillStyle).includes(hold))).toBe(true);
    expect(flares.some((f) => String(f.fillStyle).includes(deny))).toBe(false);

    // The spark shower carries the same claim and must agree with the flare.
    const sparks = draws.filter((d) => d.method === "fillRect" && d.args[2] === 1.6);
    expect(sparks.length, "a stop still shatters into sparks").toBeGreaterThan(0);
    expect(sparks.every((s) => String(s.fillStyle).includes(hold))).toBe(true);
    expect(sparks.some((s) => String(s.fillStyle).includes(deny))).toBe(false);
  });

  it("still slams a real governance denial in the deny hue", () => {
    // Pair to the test above: without this, a renderer that painted every
    // stop in `hold` would satisfy it vacuously.
    const r = createCorridorRenderer();
    const { ctx, draws } = stubCtx();
    const w = 900;
    const pad = Math.min(26, w * 0.05);
    const rateX = pad + (w - pad * 2) * ((1 + 0.5) / CHAIN_STAGES.length);

    const denied = livePacket({
      packet: { ...livePacket().packet, id: "rate-limited", stopIndex: 1, outcome: "deny" },
      progress: 2 / CHAIN_STAGES.length,
      progressLimit: 2 / CHAIN_STAGES.length,
      deadFor: 0.1,
    });

    r.draw(frame({ ctx, w, dt: 16, packets: [denied] }));
    draws.length = 0;
    r.draw(frame({ ctx, w, dt: 16, packets: [denied] }));

    const flares = draws.filter(
      (d) =>
        d.method === "fillRect" &&
        d.args[2] === 2 &&
        Math.abs((d.args[0] as number) + 1 - rateX) < 1.5,
    );
    expect(flares.length).toBeGreaterThan(0);
    expect(flares.every((f) => String(f.fillStyle).includes("226,104,95"))).toBe(true);
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
