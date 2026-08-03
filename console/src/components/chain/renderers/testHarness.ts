/**
 * Test support for renderers.
 *
 * Vitest runs under environment: "node" — there is no canvas and no DOM. A
 * Proxy-backed stub records which 2D-context methods were called and tolerates
 * any property a renderer sets, which is enough to prove a renderer runs a
 * frame without throwing across every input shape it must survive.
 */

import { IDLE_CHAIN } from "../../../lib/chain";
import { FALLBACK_PALETTE } from "../palette";
import type { LivePacket, RenderFrame } from "./types";

export interface DrawCall {
  method: string;
  args: unknown[];
  fillStyle: unknown;
  strokeStyle: unknown;
  globalAlpha: unknown;
}

export interface StubCtx {
  ctx: CanvasRenderingContext2D;
  calls: string[];
  draws: DrawCall[];
}

export function stubCtx(): StubCtx {
  const calls: string[] = [];
  const draws: DrawCall[] = [];
  const target: Record<string, unknown> = {
    canvas: { width: 900, height: 76 },
    createLinearGradient: () => ({ addColorStop: () => {} }),
    createRadialGradient: () => ({ addColorStop: () => {} }),
    measureText: () => ({ width: 24 }),
  };
  const ctx = new Proxy(target, {
    get(t, prop) {
      const key = String(prop);
      if (key in t) return t[key];
      return (...args: unknown[]) => {
        calls.push(key);
        draws.push({
          method: key,
          args,
          fillStyle: target.fillStyle,
          strokeStyle: target.strokeStyle,
          globalAlpha: target.globalAlpha,
        });
      };
    },
    set(t, prop, value) {
      t[String(prop)] = value;
      return true;
    },
  }) as unknown as CanvasRenderingContext2D;
  return { ctx, calls, draws };
}

export function livePacket(over: Partial<LivePacket> = {}): LivePacket {
  return {
    packet: {
      id: "p1",
      ts: 1785740000000,
      stopIndex: -1,
      outcome: "pass",
      latencyMs: 320,
      unproven: ["guardrail"],
    },
    progress: 0.5,
    progressLimit: 1,
    deadFor: 0,
    ...over,
  };
}

export function frame(over: Partial<RenderFrame> = {}): RenderFrame {
  const { ctx } = stubCtx();
  return {
    ctx,
    w: 900,
    h: 76,
    dt: 16,
    elapsed: 1000,
    packets: [],
    state: IDLE_CHAIN,
    palette: FALLBACK_PALETTE,
    still: false,
    ...over,
  };
}
