import { describe, expect, it } from "vitest";
import { CHAIN_STAGES, IDLE_CHAIN } from "../../lib/chain";
import type { ChainState } from "../../lib/chain";
import type { ChainPacket } from "../../lib/chainReplay";
import { FALLBACK_PALETTE } from "./palette";
import { createCorridorRenderer } from "./renderers/corridor";
import { createPhosphorRenderer } from "./renderers/phosphor";
import type { DrawCall } from "./renderers/testHarness";
import { stubCtx } from "./renderers/testHarness";
import type { ChainRenderer } from "./renderers/types";
import { drawStill, stillPackets } from "./stillFrame";

/**
 * The still frame is what `prefers-reduced-motion` users see — one frame,
 * permanently, until new evidence lands. It is pure, so every defect it can
 * have is reachable from vitest's node environment with the renderers' own
 * stub 2D context. The exemption for the canvas host (rAF, ResizeObserver,
 * visibilitychange all need a DOM) never covered this.
 */

const W = 900;
const H = 76;
const PAD = Math.min(26, W * 0.05);

const DENY_RGB = "226,104,95"; // FALLBACK_PALETTE.deny  #e2685f
const LIVE_RGB = "90,209,196"; // FALLBACK_PALETTE.live  #5ad1c4

/** Centre of stage `i`, where ticks, labels, gates and deposits sit. */
function stageX(i: number): number {
  return PAD + (W - PAD * 2) * ((i + 0.5) / CHAIN_STAGES.length);
}

/** Far boundary of stage `i` — where a packet stopped there comes to rest. */
function stopX(i: number): number {
  return PAD + (W - PAD * 2) * ((i + 1) / CHAIN_STAGES.length);
}

/**
 * Index of the last full-canvas repaint. Everything after it is what the
 * operator actually sees; anything before it was painted over.
 * (`Array.findLastIndex` is not in this project's `lib` target.)
 */
function lastClearIndex(draws: DrawCall[]): number {
  for (let i = draws.length - 1; i >= 0; i--) {
    const d = draws[i];
    if (d.method === "fillRect" && d.args[0] === 0 && d.args[1] === 0 && d.args[2] === W) return i;
  }
  return -1;
}

function packet(over: Partial<ChainPacket> = {}): ChainPacket {
  return {
    id: "p1",
    ts: 1785740000000,
    stopIndex: -1,
    outcome: "pass",
    latencyMs: 320,
    unproven: ["guardrail"],
    ...over,
  };
}

const PASSING_STATE: ChainState = {
  cleared: CHAIN_STAGES.length,
  stoppedAt: null,
  outcome: "pass",
  unproven: ["guardrail"],
};

const DENIED_AT_RATE: ChainState = {
  cleared: 1,
  stoppedAt: "rate",
  outcome: "deny",
  unproven: [],
};

function still(
  renderer: ChainRenderer,
  ctx: CanvasRenderingContext2D,
  packets: readonly ChainPacket[],
  state: ChainState,
) {
  drawStill(renderer, { ctx, w: W, h: H, packets, state, palette: FALLBACK_PALETTE });
}

describe("stillPackets", () => {
  it("rests a cleared packet at the end of the run, alive", () => {
    const [lp] = stillPackets([packet()]);
    expect(lp.progress).toBe(1);
    expect(lp.progressLimit).toBe(1);
    // deadFor 0 keeps it an ordinary travelling mark resting at the end.
    expect(lp.deadFor).toBe(0);
  });

  it("rests a stopped packet at its own ceiling, with a positive deadFor", () => {
    const [lp] = stillPackets([packet({ stopIndex: 1, outcome: "deny" })]);
    expect(lp.progressLimit).toBeCloseTo(2 / CHAIN_STAGES.length);
    expect(lp.progress).toBe(lp.progressLimit);
    // Every renderer gates its stop visuals on deadFor > 0; a still frame with
    // deadFor 0 would draw a denial as an unflagged pulse.
    expect(lp.deadFor).toBeGreaterThan(0);
    // ...but far under flow's 0.9s full-decay window, or it would render as a
    // nearly-faded mark instead of a denial.
    expect(lp.deadFor).toBeLessThan(0.9 * 0.2);
  });
});

describe("drawStill policy", () => {
  it("resets the renderer before drawing, then draws twice", () => {
    // Order is the whole contract: reset -> pass 1 registers this frame's
    // stopped packets -> pass 2 draws what pass 1 registered.
    const order: string[] = [];
    const fake: ChainRenderer = {
      reset: () => order.push("reset"),
      draw: () => order.push("draw"),
    };
    const { ctx } = stubCtx();
    still(fake, ctx, [], IDLE_CHAIN);
    expect(order).toEqual(["reset", "draw", "draw"]);
  });

  it("passes dt 0 and still: true to the renderer", () => {
    let seen: { dt: number; still: boolean } | null = null;
    const fake: ChainRenderer = {
      reset: () => {},
      draw: (f) => {
        seen = { dt: f.dt, still: f.still };
      },
    };
    const { ctx } = stubCtx();
    still(fake, ctx, [], IDLE_CHAIN);
    // dt 0 is why nothing may be carried between still frames: every decay
    // term in every renderer is `x -= dt / K`, so none of them ever fire here.
    expect(seen).toEqual({ dt: 0, still: true });
  });
});

describe("corridor still frames", () => {
  it("does not draw a later passing request as denied", () => {
    // The defect this exists for: a denial in still frame N left corridor's
    // stop flare set at that gate, and with dt 0 it never decayed. Every
    // still frame after it repainted that gate — and its spark shower — in
    // denial red for the rest of the session, so a request that PASSED was
    // drawn punching through a slammed, red gate. Permanently, and only for
    // reduced-motion users.
    const r = createCorridorRenderer();
    const { ctx, draws } = stubCtx();

    // Still #1: one request denied at rate.
    still(r, ctx, [packet({ id: "denied", stopIndex: 1, outcome: "deny" })], DENIED_AT_RATE);
    const deniedFrame = draws.filter((d) => String(d.fillStyle).includes(DENY_RGB));
    expect(deniedFrame.length, "the denial itself must be visible").toBeGreaterThan(0);

    // Still #2: the denial has scrolled off; only a clean pass is on screen.
    draws.length = 0;
    still(r, ctx, [packet({ id: "passing" })], PASSING_STATE);

    const denyMarks = draws.filter((d) => String(d.fillStyle).includes(DENY_RGB));
    expect(
      denyMarks.map((d) => `${d.method}(${d.args.join(",")}) ${String(d.fillStyle)}`),
      "nothing in a frame containing only a passing request may be deny-coloured",
    ).toEqual([]);

    // Non-vacuous: the passing request is genuinely drawn, in the live hue.
    const rateX = stageX(1);
    const liveFlares = draws.filter(
      (d) =>
        d.method === "fillRect" &&
        d.args[2] === 2 &&
        Math.abs((d.args[0] as number) + 1 - rateX) < 1.5 &&
        String(d.fillStyle).includes(LIVE_RGB),
    );
    expect(liveFlares.length, "the passing request must flare the rate gate").toBeGreaterThan(0);
  });

  it("does not accumulate sparks across still frames", () => {
    const r = createCorridorRenderer();
    const { ctx, draws } = stubCtx();
    const sparkCount = () => draws.filter((d) => d.method === "fillRect" && d.args[2] === 1.6).length;

    // Four successive denials, each still frame carrying exactly one — the
    // host's packet window rolls, the previous one has gone.
    const counts: number[] = [];
    for (let n = 0; n < 4; n++) {
      draws.length = 0;
      still(r, ctx, [packet({ id: `deny-${n}`, stopIndex: 1, outcome: "deny" })], DENIED_AT_RATE);
      counts.push(sparkCount());
    }

    expect(counts[0], "a denial must shower sparks at all").toBeGreaterThan(0);
    // Without the reset the spark array never decays (dt 0 never reaches the
    // `life <= 0` splice) and each frame adds another shower: 32, 64, 96, 128.
    expect(counts).toEqual([counts[0], counts[0], counts[0], counts[0]]);
  });
});

describe("phosphor still frames", () => {
  it("keeps deposits visible through the final opaque repaint", () => {
    // Pass 1 drew the deposits and recorded them; pass 2 repainted opaque
    // (`still ? palette.bg`), erasing them, then skipped redrawing because the
    // record said "already deposited". Result: reduced-motion users got none
    // of the deposit vocabulary spec §4 makes phosphor's entire `unproven`
    // language. A still frame's last pass is what is on screen.
    const r = createPhosphorRenderer();
    const { ctx, draws } = stubCtx();

    still(r, ctx, [packet({ id: "passing" })], PASSING_STATE);

    const lastClear = lastClearIndex(draws);
    expect(lastClear, "the still frame must repaint opaque").toBeGreaterThanOrEqual(0);

    // A deposit is the only 2x6 fillRect phosphor draws.
    const survivors = draws
      .slice(lastClear + 1)
      .filter((d) => d.method === "fillRect" && d.args[2] === 2 && d.args[3] === 6);
    const authX = stageX(0);
    expect(
      survivors.filter((d) => Math.abs((d.args[0] as number) + 1 - authX) < 1.5).length,
      "a deposit at auth must survive the final opaque clear",
    ).toBeGreaterThan(0);

    // The unproven rule survives the fix: guardrail is unproven on a plain
    // chat row and must have deposited nothing, in this pass or any other.
    const guardrailX = stageX(3);
    const allDeposits = draws.filter(
      (d) => d.method === "fillRect" && d.args[2] === 2 && d.args[3] === 6,
    );
    expect(
      allDeposits.filter((d) => Math.abs((d.args[0] as number) + 1 - guardrailX) < 1.5),
    ).toEqual([]);
  });

  it("draws no burn for a packet the host has already retired", () => {
    const r = createPhosphorRenderer();
    const { ctx, draws } = stubCtx();
    // A burn is the only 3x44 fillRect phosphor draws.
    const burns = () => draws.filter((d) => d.method === "fillRect" && d.args[2] === 3 && d.args[3] === 44);

    still(r, ctx, [packet({ id: "old-deny", stopIndex: 1, outcome: "deny" })], DENIED_AT_RATE);
    expect(burns().length, "the denial must burn a mark").toBeGreaterThan(0);

    // Next still frame: the old denial has scrolled out of the packet window
    // and a different request is denied at guardrail.
    draws.length = 0;
    still(r, ctx, [packet({ id: "new-deny", stopIndex: 3, outcome: "deny" })], {
      cleared: 3,
      stoppedAt: "guardrail",
      outcome: "deny",
      unproven: [],
    });

    const drawn = draws
      .slice(lastClearIndex(draws) + 1)
      .filter((d) => d.method === "fillRect" && d.args[2] === 3 && d.args[3] === 44);
    // Exactly one burn, at the CURRENT packet's stop boundary — not one per
    // denial the session has ever seen. Without the reset, burns never decay
    // (dt 0) and pile up: 1, 3, 5, 7 rects per repaint, marking stops for
    // requests that are no longer on screen at all.
    expect(drawn).toHaveLength(1);
    expect(Math.abs((drawn[0].args[0] as number) + 1.5 - stopX(3))).toBeLessThan(1.5);
  });
});
