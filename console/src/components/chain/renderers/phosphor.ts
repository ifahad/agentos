/**
 * Phosphor: the chain as a trace on lab equipment.
 *
 * Each request is a pulse propagating along the trace, leaving persistence
 * that decays — the canvas is dimmed each frame rather than cleared, which is
 * what produces the afterglow. A denial clips hard and burns a mark that fades
 * over a couple of seconds.
 *
 * The unproven rule (see lib/chain.ts): a pulse crossing a stage the evidence
 * does not prove ran deposits NO phosphor at that tick. The pulse still passes
 * — the request was not stopped there — but the console must not draw a mark
 * implying a check fired when it may never have run.
 */

import { CHAIN_STAGES, stageRenders } from "../../../lib/chain";
import { alpha } from "../color";
import type { ChainRenderer, RenderFrame } from "./types";

const BURN_DECAY_MS = 2600;
const PULSE_HALF_WIDTH = 30;
const PULSE_HEIGHT = 30;

interface Burn {
  x: number;
  life: number; // 1 -> 0
  denied: boolean;
}

export function createPhosphorRenderer(): ChainRenderer {
  let burns: Burn[] = [];
  let seen = new Set<string>();

  function stageX(i: number, w: number, pad: number): number {
    return pad + (w - pad * 2) * ((i + 0.5) / CHAIN_STAGES.length);
  }

  return {
    reset() {
      burns = [];
      seen = new Set();
    },

    draw({ ctx, w, h, dt, packets, state, palette, still }: RenderFrame) {
      if (w <= 0 || h <= 0) return;
      const pad = Math.min(26, w * 0.05);
      const mid = h * 0.46;
      const labelY = Math.max(h - 8, mid + 12);

      // Persistence: dim the previous frame instead of clearing it. A still
      // frame must not accumulate, so it paints opaque.
      ctx.fillStyle = still ? palette.bg : alpha(palette.bg, 0.14);
      ctx.fillRect(0, 0, w, h);

      // Graticule
      ctx.strokeStyle = alpha(palette.edge, 0.22);
      ctx.lineWidth = 1;
      for (let i = 0; i <= 10; i++) {
        const x = pad + ((w - pad * 2) * i) / 10;
        ctx.beginPath();
        ctx.moveTo(x, Math.max(0, mid - h * 0.3));
        ctx.lineTo(x, mid + h * 0.3);
        ctx.stroke();
      }

      // Baseline trace
      ctx.strokeStyle = alpha(palette.live, 0.2);
      ctx.lineWidth = 1.25;
      ctx.beginPath();
      ctx.moveTo(pad, mid);
      ctx.lineTo(w - pad, mid);
      ctx.stroke();

      // Stage ticks and labels. `stageRenders` decides which may be lit at
      // rest: cleared stages take ink, unproven and pending stay faint.
      const renders = stageRenders(state);
      ctx.textAlign = "center";
      ctx.font = '600 8px "IBM Plex Mono", ui-monospace, monospace';
      for (let i = 0; i < CHAIN_STAGES.length; i++) {
        const x = stageX(i, w, pad);
        const render = renders[i];
        const tickColor =
          render === "stopped"
            ? state.outcome === "fail"
              ? palette.hold
              : palette.deny
            : render === "cleared"
              ? palette.ink
              : palette.edge;
        ctx.strokeStyle = tickColor;
        ctx.lineWidth = render === "stopped" ? 2 : 1;
        ctx.beginPath();
        ctx.moveTo(x, mid - 9);
        ctx.lineTo(x, mid + 9);
        ctx.stroke();
        ctx.fillStyle = render === "cleared" ? palette.ink : palette.faint;
        ctx.fillText(CHAIN_STAGES[i].toUpperCase(), x, labelY);
      }

      // Burns decay
      for (let i = burns.length - 1; i >= 0; i--) {
        const b = burns[i];
        b.life -= dt / BURN_DECAY_MS;
        if (b.life <= 0) {
          burns.splice(i, 1);
          continue;
        }
        ctx.fillStyle = alpha(b.denied ? palette.deny : palette.hold, 0.55 * b.life);
        ctx.fillRect(b.x - 1.5, mid - 22, 3, 44);
      }

      // Pulses
      for (const lp of packets) {
        const { packet } = lp;
        const reach = Math.min(lp.progress, lp.progressLimit);
        const x = pad + (w - pad * 2) * reach;

        if (lp.deadFor > 0 && packet.stopIndex >= 0 && !seen.has(packet.id)) {
          seen.add(packet.id);
          burns.push({ x, life: 1, denied: packet.outcome === "deny" });
        }
        if (lp.deadFor > 0) continue;

        ctx.strokeStyle = alpha(palette.live, 0.95);
        ctx.lineWidth = 1.6;
        ctx.lineCap = "round";
        ctx.beginPath();
        for (let dx = -PULSE_HALF_WIDTH; dx <= PULSE_HALF_WIDTH; dx += 2) {
          const xx = x + dx;
          if (xx < pad || xx > w - pad) continue;
          const yy = mid - Math.exp(-(dx * dx) / 150) * PULSE_HEIGHT;
          if (dx === -PULSE_HALF_WIDTH) ctx.moveTo(xx, yy);
          else ctx.lineTo(xx, yy);
        }
        ctx.stroke();
      }

      // Readout
      ctx.font = '500 9px "IBM Plex Mono", ui-monospace, monospace';
      ctx.textAlign = "left";
      ctx.fillStyle = alpha(palette.faint, 0.7);
      const flying = packets.filter((p) => p.deadFor === 0).length;
      ctx.fillText(`IN FLIGHT ${String(flying).padStart(2, "0")}`, pad, 12);
      ctx.textAlign = "right";
      ctx.fillStyle =
        state.outcome === "deny"
          ? palette.deny
          : state.outcome === "fail"
            ? palette.hold
            : alpha(palette.faint, 0.7);
      ctx.fillText(
        state.outcome === "deny" ? "DENIED" : state.outcome === "fail" ? "PROVIDER FAILED" : "NOMINAL",
        w - pad,
        12,
      );
    },
  };
}
