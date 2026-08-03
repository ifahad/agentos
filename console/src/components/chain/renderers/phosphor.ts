/**
 * Phosphor: the chain as a trace on lab equipment.
 *
 * Each request is a pulse propagating along the trace, leaving persistence
 * that decays — the canvas is dimmed each frame rather than cleared, which is
 * what produces the afterglow. A denial clips hard and burns a mark that fades
 * over a couple of seconds.
 *
 * As the pulse's leading edge crosses each stage, it deposits a small mark at
 * that tick if the evidence proves the stage ran. The unproven rule (see
 * lib/chain.ts): a pulse crossing a stage the evidence does not prove ran
 * deposits NOTHING at that tick. The pulse still passes — the request was not
 * stopped there — but the console must not draw a mark implying a check fired
 * when it may never have run. Deposits decay along with the canvas fade,
 * producing the characteristic phosphor afterglow.
 */

import { CHAIN_STAGES, stageRenders } from "../../../lib/chain";
import type { ChainOutcome } from "../../../lib/chain";
import { alpha, stopColor } from "../color";
import type { ChainRenderer, RenderFrame } from "./types";

const BURN_DECAY_MS = 2600;
const PULSE_HALF_WIDTH = 30;
const PULSE_HEIGHT = 30;
const DEPOSIT_HEIGHT = 6;
const DEPOSIT_WIDTH = 2;

/**
 * Cap on per-packet bookkeeping, matching the host's own (ChainCanvas.tsx:
 * 197-209). `seen` and `deposited` otherwise hold one entry per packet
 * forever, and at MAX_REPLAY_PER_POLL on a 5s poll that is ~8.6k entries an
 * hour on a console left open.
 */
const MAX_TRACKED_IDS = 512;

interface Burn {
  x: number;
  life: number; // 1 -> 0
  /** Carried, not resolved, so a theme flip recolours a burn already fading. */
  outcome: ChainOutcome;
}

export function createPhosphorRenderer(): ChainRenderer {
  let burns: Burn[] = [];
  let seen = new Set<string>();
  let deposited = new Map<string, Set<number>>();

  function stageX(i: number, w: number, pad: number): number {
    return pad + (w - pad * 2) * ((i + 0.5) / CHAIN_STAGES.length);
  }

  return {
    reset() {
      burns = [];
      seen = new Set();
      deposited = new Map();
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

      // Stage ticks and labels. `stageRenders` decides which TICK may be lit
      // at rest: cleared takes ink, stopped takes the outcome hue, unproven
      // and pending stay at the edge colour. The label below never varies —
      // it is text, and it has a contrast floor to clear.
      const renders = stageRenders(state);
      ctx.textAlign = "center";
      ctx.font = '600 8px "IBM Plex Mono", ui-monospace, monospace';
      for (let i = 0; i < CHAIN_STAGES.length; i++) {
        const x = stageX(i, w, pad);
        const render = renders[i];
        const tickColor =
          render === "stopped"
            ? stopColor(state.outcome, palette)
            : render === "cleared"
              ? palette.ink
              : palette.edge;
        ctx.strokeStyle = tickColor;
        ctx.lineWidth = render === "stopped" ? 2 : 1;
        ctx.beginPath();
        ctx.moveTo(x, mid - 9);
        ctx.lineTo(x, mid + 9);
        ctx.stroke();
        // Label colour is always `ink` (--text-dim), never `faint`
        // (--text-faint): design spec §6 and Chain.css:86-90 — --text-faint
        // measures 2.65:1 on --bg, below the 4.5:1 floor, and at 8px uppercase
        // mono this is EVERY label on an idle chain, in the default style, on
        // every page. The tick above already carries which stage is lit, so
        // the label does not have to encode it by being unreadable. `faint`
        // stays for non-text furniture only.
        ctx.fillStyle = palette.ink;
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
        ctx.fillStyle = alpha(stopColor(b.outcome, palette), 0.55 * b.life);
        ctx.fillRect(b.x - 1.5, mid - 22, 3, 44);
      }

      // Pulses and deposits
      for (const lp of packets) {
        const { packet } = lp;
        const reach = Math.min(lp.progress, lp.progressLimit);
        const x = pad + (w - pad * 2) * reach;

        if (lp.deadFor > 0 && packet.stopIndex >= 0 && !seen.has(packet.id)) {
          seen.add(packet.id);
          burns.push({ x, life: 1, outcome: packet.outcome });
        }

        // Track deposits per packet: only deposit at stages the evidence proves ran
        if (!deposited.has(packet.id)) {
          deposited.set(packet.id, new Set());
        }
        const packetDeposits = deposited.get(packet.id)!;

        // A still frame draws stopped packets too. Under reduced motion there
        // is no rAF pass during which a denied packet was ever travelling, so
        // gating deposits on `deadFor === 0` alone would give a reduced-motion
        // user a burn mark with none of the deposits the live path lays down
        // on the way to it. Placing a stopped packet at its resting position
        // makes `reach === progressLimit`, so this deposits at exactly the
        // stages up to and including its stop — the same set the animated path
        // produces, and no more.
        if (lp.deadFor === 0 || still) {
          // Pulse is still travelling — check each stage to see if the pulse
          // has crossed it, and deposit if evidence proves it ran
          const maxStageIndex = packet.stopIndex >= 0 ? packet.stopIndex : CHAIN_STAGES.length - 1;
          for (let i = 0; i <= maxStageIndex; i++) {
            // A stage is crossed once the pulse has reached its far boundary
            const stageBoundary = (i + 1) / CHAIN_STAGES.length;
            const stageCrossed = reach >= stageBoundary;

            // `packetDeposits` is an "already painted, let the phosphor carry
            // it" memo, which only holds while the canvas PERSISTS between
            // frames. A still frame repaints opaque (`still ? palette.bg`
            // above) and is drawn twice — so pass 2 erased pass 1's deposits
            // and then skipped redrawing them, and every deposit vanished for
            // exactly the users who cannot see the animation. `still` is the
            // frame telling the renderer nothing carries over: redraw.
            if (stageCrossed && (still || !packetDeposits.has(i))) {
              // Only deposit if the stage is not in unproven
              if (!packet.unproven.includes(CHAIN_STAGES[i])) {
                const stageXPos = stageX(i, w, pad);
                ctx.fillStyle = alpha(palette.live, 0.6);
                ctx.fillRect(stageXPos - DEPOSIT_WIDTH / 2, mid - DEPOSIT_HEIGHT / 2, DEPOSIT_WIDTH, DEPOSIT_HEIGHT);
                packetDeposits.add(i);
              }
            }
          }
        }

        if (lp.deadFor > 0) continue;

        // Draw the pulse
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

      // Bookkeeping hygiene, same rule and same reasoning as the host's
      // (ChainCanvas.tsx:197-209). `seen` and `deposited` are keyed by packet
      // id and are only ever consulted for a packet the frame still carries;
      // the host retires a LivePacket permanently and never re-adopts one (its
      // own trim keeps the union of live and prop ids precisely so an id is
      // never seen twice). So pruning to the ids on screen is lossless, and
      // without it both structures grow for as long as the console is open.
      if (seen.size > MAX_TRACKED_IDS || deposited.size > MAX_TRACKED_IDS) {
        const onScreen = new Set(packets.map((lp) => lp.packet.id));
        seen = new Set([...seen].filter((id) => onScreen.has(id)));
        deposited = new Map([...deposited].filter(([id]) => onScreen.has(id)));
      }

      // Readout. `ink`, not `faint` — same contrast rule as the stage labels
      // above (design spec §6); at 9px mono an `alpha(faint, 0.7)` readout was
      // the worst-contrast text on the canvas.
      ctx.font = '500 9px "IBM Plex Mono", ui-monospace, monospace';
      ctx.textAlign = "left";
      ctx.fillStyle = palette.ink;
      const flying = packets.filter((p) => p.deadFor === 0).length;
      ctx.fillText(`IN FLIGHT ${String(flying).padStart(2, "0")}`, pad, 12);
      ctx.textAlign = "right";
      ctx.fillStyle =
        state.outcome === "pass" ? palette.ink : stopColor(state.outcome, palette);
      ctx.fillText(
        state.outcome === "deny" ? "DENIED" : state.outcome === "fail" ? "PROVIDER FAILED" : "NOMINAL",
        w - pad,
        12,
      );
    },
  };
}
