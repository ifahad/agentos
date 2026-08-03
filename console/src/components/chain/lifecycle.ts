/**
 * How a packet moves and when it retires.
 *
 * Kept out of the React host so the timing rules are unit-testable under
 * vitest's node environment — the host owns rAF, this owns the arithmetic.
 */

import { CHAIN_STAGES } from "../../lib/chain";
import type { ChainPacket } from "../../lib/chainReplay";
import type { LivePacket } from "./renderers/types";

/** Traversal bounds. A 40ms call still has to be watchable; a 90s one must not crawl. */
const MIN_TRAVEL_MS = 400;
const MAX_TRAVEL_MS = 4000;

/** How long a finished packet lingers on screen for its decay effects. */
export const RETIRE_AFTER_S = 1.2;

/**
 * Traversal time for one packet, scaled to its real `latency_ms`.
 *
 * Compressed logarithmically rather than linearly: real latencies span three
 * orders of magnitude, and a linear map would make every fast call
 * indistinguishable while a slow one blocked the chain for a minute.
 */
export function travelDurationMs(packet: ChainPacket): number {
  const raw = Number.isFinite(packet.latencyMs) ? Math.max(0, packet.latencyMs) : 0;
  const frac = Math.log10(1 + raw) / Math.log10(1 + 10_000);
  const scaled = MIN_TRAVEL_MS + (MAX_TRAVEL_MS - MIN_TRAVEL_MS) * Math.min(1, frac);
  return Math.min(MAX_TRAVEL_MS, Math.max(MIN_TRAVEL_MS, scaled));
}

/**
 * Step one packet forward. Returns a new object; never mutates the input.
 *
 * `progressLimit` is the packet's own ceiling — a request denied at `rate` may
 * never be drawn past the rate boundary, however long it lingers.
 *
 * Note: denied packets traverse at asymmetric on-screen speeds. When stopped
 * early, the step is scaled by `progressLimit` rather than 1, so a packet
 * denied at stage 1 takes roughly 1/5 the on-screen speed of one denied at
 * stage 4, even for identical real latency. This keeps early rejections
 * watchable rather than snapping instantly to a near-origin stop point.
 */
export function advance(live: LivePacket, dtMs: number): LivePacket {
  const limit = live.progressLimit;
  if (live.progress >= limit) {
    return { ...live, progress: limit, deadFor: live.deadFor + dtMs / 1000 };
  }
  const total = travelDurationMs(live.packet);
  const step = (dtMs / total) * (live.packet.stopIndex >= 0 ? limit : 1);
  const progress = Math.min(limit, live.progress + step);
  return {
    ...live,
    progress,
    deadFor: progress >= limit ? live.deadFor + dtMs / 1000 : 0,
  };
}

/** Ceiling a packet may be drawn to, from its stop index. */
export function progressLimitFor(packet: ChainPacket): number {
  return packet.stopIndex < 0 ? 1 : (packet.stopIndex + 1) / CHAIN_STAGES.length;
}
