/**
 * Corridor: six luminous gates a request punches through.
 *
 * A gate flares only when a packet's evidence proves that stage fired. A
 * packet crossing an unproven stage passes through a gate that stays a dim
 * outline — the request was not stopped there, but nothing proves the check
 * ran, and a flare would assert exactly that.
 *
 * A denial momentarily slams its gate with a deny tint and then shatters into
 * sparks. Both the tint and sparks decay over FLARE_DECAY_MS and SPARK_LIFE_MS
 * respectively, so a gate returns to its live colour once the denial has fully
 * faded. A later packet passing the same gate is never rendered as denied.
 */

import { CHAIN_STAGES, stageRenders } from "../../../lib/chain";
import { alpha } from "../color";
import type { ChainRenderer, RenderFrame } from "./types";

const SPARKS_PER_DENIAL = 16;
const SPARK_LIFE_MS = 620;
const FLARE_DECAY_MS = 260;

interface Spark {
  x: number;
  y: number;
  vx: number;
  vy: number;
  life: number;
}

export function createCorridorRenderer(): ChainRenderer {
  let flare = new Array<number>(CHAIN_STAGES.length).fill(0);
  let denyFlare = new Array<number>(CHAIN_STAGES.length).fill(0);
  let sparks: Spark[] = [];
  let handled = new Set<string>();

  function gateX(i: number, w: number, pad: number): number {
    return pad + (w - pad * 2) * ((i + 0.5) / CHAIN_STAGES.length);
  }

  return {
    reset() {
      flare = new Array<number>(CHAIN_STAGES.length).fill(0);
      denyFlare = new Array<number>(CHAIN_STAGES.length).fill(0);
      sparks = [];
      handled = new Set();
    },

    draw({ ctx, w, h, dt, packets, state, palette, still }: RenderFrame) {
      if (w <= 0 || h <= 0) return;
      const pad = Math.min(26, w * 0.05);
      const mid = h * 0.46;
      const labelY = Math.max(h - 8, mid + 12);
      const gateHalf = Math.min(34, h * 0.32);

      ctx.clearRect(0, 0, w, h);

      // Light each gate a packet has proven it cleared this frame.
      for (const lp of packets) {
        if (lp.deadFor > 0) continue;
        const reached = Math.floor(lp.progress * CHAIN_STAGES.length);
        for (let i = 0; i < Math.min(reached, CHAIN_STAGES.length); i++) {
          // An unproven stage never flares, however far the packet got.
          if (lp.packet.unproven.includes(CHAIN_STAGES[i])) continue;
          if (lp.packet.stopIndex >= 0 && i > lp.packet.stopIndex) continue;
          flare[i] = 1;
        }
      }

      const renders = stageRenders(state);
      ctx.textAlign = "center";
      ctx.font = '600 8px "IBM Plex Mono", ui-monospace, monospace';

      for (let i = 0; i < CHAIN_STAGES.length; i++) {
        const x = gateX(i, w, pad);
        const f = flare[i];
        const d = denyFlare[i];
        flare[i] = Math.max(0, f - dt / FLARE_DECAY_MS);
        denyFlare[i] = Math.max(0, d - dt / FLARE_DECAY_MS);
        const col = d > 0 ? palette.deny : palette.live;

        // Gate body — a vertical slit that brightens with the flare.
        const grad = ctx.createLinearGradient(x, mid - gateHalf, x, mid + gateHalf);
        grad.addColorStop(0, alpha(col, 0));
        grad.addColorStop(0.5, alpha(col, 0.16 + f * 0.55));
        grad.addColorStop(1, alpha(col, 0));
        ctx.fillStyle = grad;
        ctx.fillRect(x - 1.5, mid - gateHalf, 3, gateHalf * 2);

        // The flare mark. Drawn ONLY while this gate is actually flaring, as a
        // discrete rect rather than a gradient stop — a gradient's stops are
        // invisible to the test harness's stub context, so a flare expressed
        // only through `grad` could not be asserted, and the unproven rule
        // would again be untestable. This is the corridor's per-stage claim:
        // it appears exactly when the evidence proves the stage fired.
        if (f > 0) {
          ctx.fillStyle = alpha(col, 0.5 * f);
          ctx.fillRect(x - 1, mid - gateHalf, 2, gateHalf * 2);
        }

        const render = renders[i];
        ctx.fillStyle =
          render === "stopped"
            ? state.outcome === "fail"
              ? palette.hold
              : palette.deny
            : render === "cleared" || f > 0
              ? palette.ink
              : palette.faint;
        ctx.fillText(CHAIN_STAGES[i].toUpperCase(), x, labelY);
      }

      // Streaks
      for (const lp of packets) {
        if (lp.deadFor > 0) {
          if (lp.packet.stopIndex >= 0 && !handled.has(lp.packet.id)) {
            handled.add(lp.packet.id);
            denyFlare[lp.packet.stopIndex] = 1;
            flare[lp.packet.stopIndex] = 1;
            const sx = gateX(lp.packet.stopIndex, w, pad);
            for (let k = 0; k < SPARKS_PER_DENIAL; k++) {
              sparks.push({
                x: sx,
                y: mid,
                vx: (k / SPARKS_PER_DENIAL - 0.7) * 90,
                vy: (k % 2 === 0 ? 1 : -1) * (30 + k * 6),
                life: 1,
              });
            }
          }
          continue;
        }
        const reach = Math.min(lp.progress, lp.progressLimit);
        const x = pad + (w - pad * 2) * reach;
        const tail = 46;
        const grad = ctx.createLinearGradient(x - tail, 0, x, 0);
        grad.addColorStop(0, alpha(palette.live, 0));
        grad.addColorStop(1, alpha(palette.live, 0.95));
        ctx.strokeStyle = grad;
        ctx.lineWidth = 2;
        ctx.lineCap = "round";
        ctx.beginPath();
        ctx.moveTo(Math.max(pad, x - tail), mid);
        ctx.lineTo(x, mid);
        ctx.stroke();
      }

      // Sparks
      for (let i = sparks.length - 1; i >= 0; i--) {
        const s = sparks[i];
        s.x += (s.vx * dt) / 1000;
        s.y += (s.vy * dt) / 1000;
        s.vy += (170 * dt) / 1000;
        s.life -= dt / SPARK_LIFE_MS;
        if (s.life <= 0) {
          sparks.splice(i, 1);
          continue;
        }
        ctx.fillStyle = alpha(palette.deny, s.life);
        ctx.fillRect(s.x, s.y, 1.6, 1.6);
      }

      if (still) {
        ctx.font = '500 9px "IBM Plex Mono", ui-monospace, monospace';
        ctx.textAlign = "left";
        ctx.fillStyle = alpha(palette.faint, 0.7);
        ctx.fillText("STILL", pad, 12);
      }
    },
  };
}
