/**
 * The renderer seam.
 *
 * A renderer owns pixels and nothing else: it never fetches, never decides
 * what a stage proved, and never invents a request. It is handed already-
 * adjudicated packets and draws them. Adding a style is one new file
 * implementing this interface plus one registry entry.
 */

import type { ChainPacket } from "../../../lib/chainReplay";
import type { ChainState } from "../../../lib/chain";
import type { ChainPalette } from "../palette";

/** A packet currently on screen, with its traversal progress. */
export interface LivePacket {
  packet: ChainPacket;
  /**
   * 0..1 across the whole six-stage run. A packet that stops early still
   * advances only to its own stop boundary — see progressLimit.
   */
  progress: number;
  /** Fraction of the full run this packet may reach: (stopIndex + 1) / 6, or 1. */
  progressLimit: number;
  /** Seconds since this packet finished, for decay effects. 0 while travelling. */
  deadFor: number;
}

export interface RenderFrame {
  ctx: CanvasRenderingContext2D;
  /** CSS pixels, already DPR-corrected by the host's transform. */
  w: number;
  h: number;
  /** Milliseconds since the previous frame, clamped to <= 50 by the host. */
  dt: number;
  /** Total ms since the renderer was last reset, for ambient cycles. */
  elapsed: number;
  packets: readonly LivePacket[];
  /** The newest row's adjudicated state — drives the resting appearance. */
  state: ChainState;
  palette: ChainPalette;
  /**
   * True when the host is drawing a single static frame because the user
   * prefers reduced motion. Renderers must draw a meaningful still, not a
   * blank one, and must not rely on `elapsed` advancing.
   */
  still: boolean;
}

export interface ChainRenderer {
  draw(frame: RenderFrame): void;
  /** Drop accumulated visual state — style switch, resize, remount. */
  reset(): void;
}
