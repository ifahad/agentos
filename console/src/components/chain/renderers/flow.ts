/**
 * Flow: every in-flight request is a particle crossing six gates.
 *
 * Density is throughput — the picture is made of information, so it is sparse
 * on a quiet system by design. Denied particles turn deny-red, deflect out at
 * their stage and fall away, so where requests die is legible without a legend.
 *
 * Unproven stages are drawn as DASHED columns rather than solid ones: the
 * particle crosses either way, but a solid column would read as a check that
 * fired, and on a plain chat row nothing proves the screener ran at all.
 *
 * The unproven reading is the union of state.unproven and every on-screen
 * packet's unproven: a column drawn solid because the newest row happened to
 * prove guardrail would over-claim for particles for which it was never proven.
 * state is the LATEST adjudicated row, while particles belong to a window of
 * earlier rows. The union is the only reading that never over-claims for
 * anything actually on screen.
 */

import { CHAIN_STAGES, stageRenders } from "../../../lib/chain";
import { alpha } from "../color";
import type { ChainRenderer, RenderFrame } from "./types";

interface Drift {
  y: number;
  vy: number;
  life: number;
}

/** Deterministic per-packet lane, so a particle does not jitter between frames. */
function laneFor(id: string): number {
  let hash = 0;
  for (let i = 0; i < id.length; i++) hash = (hash * 31 + id.charCodeAt(i)) | 0;
  return (Math.abs(hash) % 1000) / 1000;
}

export function createFlowRenderer(): ChainRenderer {
  let drifts = new Map<string, Drift>();

  function gateX(i: number, w: number, pad: number): number {
    return pad + (w - pad * 2) * ((i + 0.5) / CHAIN_STAGES.length);
  }

  return {
    reset() {
      drifts = new Map();
    },

    draw({ ctx, w, h, dt, packets, state, palette, still }: RenderFrame) {
      if (w <= 0 || h <= 0) return;
      const pad = Math.min(26, w * 0.05);
      const top = Math.min(22, h * 0.22);
      const bot = Math.max(top + 1, h - 26);
      const labelY = Math.max(h - 8, bot + 10);

      ctx.clearRect(0, 0, w, h);

      const renders = stageRenders(state);
      ctx.textAlign = "center";
      ctx.font = '600 8px "IBM Plex Mono", ui-monospace, monospace';

      for (let i = 0; i < CHAIN_STAGES.length; i++) {
        const x = gateX(i, w, pad);
        // Unproven if the resting reading says so, OR if any packet currently
        // on screen carries it unproven.
        //
        // Reading `state` alone would be wrong here in a way that is easy to
        // miss: `state` is the LATEST adjudicated row, while the particles
        // being drawn belong to a window of earlier rows. A column drawn solid
        // because the newest row happened to prove guardrail would assert that
        // claim over particles for which it was never proven. The union is the
        // only reading that never over-claims for anything actually on screen.
        const stage = CHAIN_STAGES[i];
        const unproven =
          state.unproven.includes(stage) ||
          packets.some((lp) => lp.packet.unproven.includes(stage));
        ctx.strokeStyle =
          renders[i] === "stopped"
            ? state.outcome === "fail"
              ? palette.hold
              : palette.deny
            : alpha(palette.edge, unproven ? 0.5 : 1);
        ctx.lineWidth = 1;
        // Dashed == "the evidence does not prove this check ran."
        ctx.setLineDash(unproven ? [2, 3] : []);
        ctx.beginPath();
        ctx.moveTo(x, top);
        ctx.lineTo(x, bot);
        ctx.stroke();
        ctx.setLineDash([]);

        ctx.fillStyle = renders[i] === "cleared" ? palette.ink : palette.faint;
        ctx.fillText(CHAIN_STAGES[i].toUpperCase(), x, labelY);
      }

      let flying = 0;
      const alive = new Set<string>();

      for (const lp of packets) {
        const { packet } = lp;
        alive.add(packet.id);
        const lane = top + laneFor(packet.id) * (bot - top);
        const reach = Math.min(lp.progress, lp.progressLimit);
        const x = pad + (w - pad * 2) * reach;

        let y = lane;
        let color = packet.outcome === "pass" ? palette.live : palette.deny;
        let life = 1;

        if (lp.deadFor > 0 && packet.stopIndex >= 0) {
          let d = drifts.get(packet.id);
          if (!d) {
            d = { y: lane, vy: 30 + laneFor(packet.id) * 70, life: 1 };
            drifts.set(packet.id, d);
          }
          d.y += (d.vy * dt) / 1000;
          d.vy += (90 * dt) / 1000;
          d.life = Math.max(0, 1 - lp.deadFor / 0.9);
          y = d.y;
          life = d.life;
          color = packet.outcome === "fail" ? palette.hold : palette.deny;
        } else if (lp.deadFor === 0) {
          flying++;
        }

        if (life <= 0) continue;

        const tail = lp.deadFor > 0 ? 6 : 16;
        const grad = ctx.createLinearGradient(x - tail, 0, x, 0);
        grad.addColorStop(0, alpha(color, 0));
        grad.addColorStop(1, alpha(color, 0.85 * life));
        ctx.strokeStyle = grad;
        ctx.lineWidth = 1.6;
        ctx.lineCap = "round";
        ctx.beginPath();
        ctx.moveTo(Math.max(pad, x - tail), y);
        ctx.lineTo(x, y);
        ctx.stroke();
      }

      // Drop drift bookkeeping for packets the host has retired.
      for (const id of drifts.keys()) if (!alive.has(id)) drifts.delete(id);

      ctx.font = '500 9px "IBM Plex Mono", ui-monospace, monospace';
      ctx.textAlign = "left";
      ctx.fillStyle = alpha(palette.faint, 0.7);
      ctx.fillText(still ? "STILL" : `IN FLIGHT ${String(flying).padStart(3, "0")}`, pad, 12);
    },
  };
}
