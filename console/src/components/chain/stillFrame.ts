/**
 * The reduced-motion still frame.
 *
 * `prefers-reduced-motion: reduce` means no rAF loop at all (design spec §6):
 * one static frame reflecting the current evidence, repainted whenever that
 * evidence changes. Everything about constructing that frame is pure
 * arithmetic over the props, so it lives here rather than inside the React
 * host — the host keeps rAF, DPR sizing, ResizeObserver and visibility, which
 * genuinely need a DOM; this needs only a 2D context, which the renderer test
 * harness already stubs. Every defect a still frame can have is therefore
 * reachable from vitest's node environment.
 *
 * That matters more than it usually would: a reduced-motion user sees exactly
 * one frame, forever, until new evidence lands. Any lie in it is not a flicker
 * — it is the permanent state of the chain for the users least able to check
 * it against motion.
 */

import type { ChainState } from "../../lib/chain";
import type { ChainPacket } from "../../lib/chainReplay";
import { progressLimitFor } from "./lifecycle";
import type { ChainPalette } from "./palette";
import type { ChainRenderer, LivePacket } from "./renderers/types";

/**
 * `deadFor` given to a stopped packet in a still frame.
 *
 * Stopped packets need a small POSITIVE value: phosphor's burn mark and
 * corridor's stop-flare/spark shower only fire on `deadFor > 0`, and flow's
 * fail-vs-deny colour override is gated the same way. 0.05s clears every one
 * of those gates while staying far under flow's 0.9s full-decay window
 * (`1 - deadFor / 0.9`), so opacity there is still ~94% — a stop reads as a
 * stop, not as a faded live mark indistinguishable from a pass.
 */
const STILL_DEAD_FOR_S = 0.05;

export interface StillFrameInput {
  ctx: CanvasRenderingContext2D;
  /** CSS pixels, already DPR-corrected by the host's transform. */
  w: number;
  h: number;
  packets: readonly ChainPacket[];
  state: ChainState;
  palette: ChainPalette;
}

/**
 * Place the current packets at rest and draw one static frame.
 *
 * Each on-screen packet sits at its own progress ceiling rather than
 * mid-flight, since there is no rAF to carry it there.
 */
export function stillPackets(packets: readonly ChainPacket[]): LivePacket[] {
  return packets.map((p) => {
    const limit = progressLimitFor(p);
    // Cleared packets (stopIndex < 0) keep deadFor 0, so they render as an
    // ordinary travelling pulse resting at the end of the trace.
    const deadFor = p.stopIndex >= 0 ? STILL_DEAD_FOR_S : 0;
    return { packet: p, progress: limit, progressLimit: limit, deadFor };
  });
}

/**
 * Draw one still frame with `renderer`, built fresh from the given props.
 *
 * Three policies, in order, and each of them is load-bearing:
 *
 * 1. **reset() first.** Every decay term in every renderer is `x -= dt / K`,
 *    and a still frame passes `dt: 0` — so no decay ever fires and no
 *    `life <= 0` splice ever runs. Renderer state carried between still
 *    frames therefore never fades: corridor's stop flare stayed set forever,
 *    and once set, `stopFlare[i] > 0` recolours that gate for the rest of the
 *    session. A later PASSING packet crossing it was drawn punching through a
 *    slammed, stop-tinted gate — the chain asserting a denial that never
 *    happened, permanently, to reduced-motion users specifically. Sparks and
 *    burn marks accumulated the same way, one more set per repaint.
 *
 *    A still frame is by definition built fresh from the current props — it is
 *    the *whole* picture, not an increment on the last one — so cross-frame
 *    accumulation is never wanted here. Resetting is not a workaround for the
 *    absent decay; it is the correct statement of what this frame is.
 *
 * 2. **Two passes, not one.** phosphor and corridor each register a newly
 *    stopped packet's visuals (phosphor's burn, corridor's gate flare) in a
 *    loop that runs AFTER the loop drawing that same state. That is a
 *    renderer-internal ordering, fine under a continuous rAF loop where the
 *    next tick draws what this one registered, but fatal to a single
 *    still-frame draw: a packet's FIRST appearance as stopped would register
 *    and draw as blank, and with no further trigger (it was the session's last
 *    request) it would stay blank permanently. Pass two has nothing left to
 *    register — phosphor's `seen` / corridor's `handled` already hold the id
 *    from pass one — so it only draws what pass one recorded.
 *
 *    Safe against double-compositing: both renderers fully clear or
 *    opaque-fill on every call (phosphor paints opaque specifically because
 *    `still` is true), so the visible result is exactly pass two's, not a
 *    blend. Two is provably enough: the only renderer state with a one-call
 *    registration lag is each renderer's own decay array, and nothing either
 *    renderer's draw() does can register something new to a THIRD internal
 *    array only revealed on a follow-up call — a third pass would draw
 *    pixel-identical output. flow has no such split (its per-packet state is
 *    read and drawn in the same loop iteration) and is unaffected by running
 *    twice.
 *
 * 3. **`still: true`,** which renderers read as "nothing you drew last call
 *    survives" — phosphor uses it to repaint opaque instead of fading, and to
 *    redraw its deposits rather than trusting canvas persistence to hold them.
 *
 * The ordering is reset -> pass 1 registers the current packets' stopped state
 * -> pass 2 draws it. Reset before pass 1 is what keeps pass 1's registration
 * about THIS frame's packets only.
 */
export function drawStill(renderer: ChainRenderer, input: StillFrameInput): void {
  const { ctx, w, h, packets, state, palette } = input;
  renderer.reset();
  const frame = {
    ctx,
    w,
    h,
    dt: 0,
    elapsed: 0,
    packets: stillPackets(packets),
    state,
    palette,
    still: true,
  };
  renderer.draw(frame);
  renderer.draw(frame);
}
