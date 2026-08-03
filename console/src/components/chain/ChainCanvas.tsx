import { useEffect, useRef } from "react";
import { useReducedMotion } from "framer-motion";
import type { ChainState } from "../../lib/chain";
import type { ChainPacket } from "../../lib/chainReplay";
import type { ChainStyle } from "../../lib/chainStyle";
import { RETIRE_AFTER_S, advance, progressLimitFor } from "./lifecycle";
import { readPalette } from "./palette";
import { createRenderer } from "./renderers";
import type { ChainRenderer, LivePacket } from "./renderers/types";

interface Props {
  style: ChainStyle;
  /** Packets released so far. The host owns their on-screen lifetime. */
  packets: readonly ChainPacket[];
  state: ChainState;
  height: number;
}

/**
 * Canvas host for the animated chain.
 *
 * Owns exactly the things a renderer must not: the rAF loop, DPR-correct
 * sizing, pausing when the tab is hidden, and the reduced-motion still frame.
 * It is aria-hidden — the accessible representation of the chain is the DOM
 * stage list in Chain.tsx, which is unaffected by any of this.
 */
export function ChainCanvas({ style, packets, state, height }: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  // Live values the rAF loop (and the reduced-motion repaint below) read
  // without the effect being torn down and rebuilt on every poll tick.
  const packetsRef = useRef(packets);
  const stateRef = useRef(state);
  packetsRef.current = packets;
  stateRef.current = state;
  const reduced = useReducedMotion();

  // Under reduced motion the mount effect below draws exactly one frame and
  // runs no loop. That frame must not go stale when new evidence arrives, so
  // it stores its own "repaint with current props" closure here, invoked by
  // the second effect further down. Left null whenever motion is not
  // reduced, so a packets/state change during normal (animated) operation
  // never triggers an extra out-of-band draw alongside the rAF loop.
  const repaintStillRef = useRef<(() => void) | null>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const createdRenderer = createRenderer(style);
    if (!createdRenderer) return;
    // Re-bound with an explicit non-null annotation: a plain narrowed `const`
    // only stays narrowed within this function's own body, not inside the
    // nested callbacks below (size, stillFrame, tick) that TypeScript can't
    // prove run after this check. Annotating the type directly here, rather
    // than leaning on control-flow narrowing, is what lets every closure see
    // a non-null type without a `!` assertion at each use.
    const renderer: ChainRenderer = createdRenderer;

    const createdCtx = canvas.getContext("2d");
    if (!createdCtx) return;
    const ctx: CanvasRenderingContext2D = createdCtx;

    let palette = readPalette(canvas);
    let live: LivePacket[] = [];
    let seen = new Set<string>();
    let raf = 0;
    let last = performance.now();
    let elapsed = 0;
    let w = 0;
    let h = 0;

    function size() {
      const canvasEl = canvasRef.current;
      if (!canvasEl) return;
      const rect = canvasEl.getBoundingClientRect();
      const dpr = window.devicePixelRatio || 1;
      w = rect.width;
      h = rect.height;
      canvasEl.width = Math.max(1, Math.round(w * dpr));
      canvasEl.height = Math.max(1, Math.round(h * dpr));
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      renderer.reset();
    }
    size();

    /**
     * One static frame built fresh from the current props — no loop, no
     * accumulation, nothing that moves. Each on-screen packet is placed at
     * its resting position (its own progress ceiling) rather than mid-flight,
     * since there is no rAF to carry it there.
     *
     * Reused from several places: the initial paint below, a resize, a theme
     * flip, and — via repaintStillRef — whenever new evidence arrives while
     * reduced motion stays on. Only ever called when `reduced` is true.
     */
    function stillFrame() {
      const stillLive: LivePacket[] = packetsRef.current.map((p) => {
        const limit = progressLimitFor(p);
        return { packet: p, progress: limit, progressLimit: limit, deadFor: 0 };
      });
      renderer.draw({
        ctx,
        w,
        h,
        dt: 0,
        elapsed: 0,
        packets: stillLive,
        state: stateRef.current,
        palette,
        still: true,
      });
    }

    const ro = new ResizeObserver(() => {
      size();
      // Changing canvas.width/height clears its pixel buffer, so a still
      // frame must be repainted immediately or the chain would go blank
      // until the next evidence change.
      if (reduced) stillFrame();
    });
    ro.observe(canvas);

    // The palette is theme-dependent; re-read it when the theme attribute
    // flips, and — under reduced motion — repaint immediately, since nothing
    // else will pick up the new colours until new evidence arrives.
    const themeObserver = new MutationObserver(() => {
      const canvasEl = canvasRef.current;
      if (canvasEl) palette = readPalette(canvasEl);
      if (reduced) stillFrame();
    });
    themeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });

    if (reduced) {
      repaintStillRef.current = stillFrame;
      stillFrame();
      return () => {
        repaintStillRef.current = null;
        ro.disconnect();
        themeObserver.disconnect();
      };
    }

    function tick(now: number) {
      const dt = Math.min(50, now - last);
      last = now;
      elapsed += dt;

      // Adopt newly released packets.
      for (const p of packetsRef.current) {
        if (seen.has(p.id)) continue;
        seen.add(p.id);
        live.push({ packet: p, progress: 0, progressLimit: progressLimitFor(p), deadFor: 0 });
      }
      // Step and retire.
      live = live.map((lp) => advance(lp, dt)).filter((lp) => lp.deadFor < RETIRE_AFTER_S);
      // `seen` must not grow without bound across a long session. Packets
      // are TTL-dropped from the `packets` prop well before this fires (see
      // useChainPackets), so anything outside `live` here can never be
      // re-adopted as "new" later — trimming to exactly `live`'s ids is safe.
      if (seen.size > 512) seen = new Set(live.map((lp) => lp.packet.id));

      renderer.draw({
        ctx,
        w,
        h,
        dt,
        elapsed,
        packets: live,
        state: stateRef.current,
        palette,
        still: false,
      });
      raf = requestAnimationFrame(tick);
    }

    function start() {
      if (raf) return;
      last = performance.now();
      raf = requestAnimationFrame(tick);
    }
    function stop() {
      if (!raf) return;
      cancelAnimationFrame(raf);
      raf = 0;
    }
    function onVisibility() {
      // Match the poll: a hidden tab burns no frames.
      if (document.hidden) stop();
      else start();
    }

    document.addEventListener("visibilitychange", onVisibility);
    if (!document.hidden) start();

    return () => {
      stop();
      ro.disconnect();
      themeObserver.disconnect();
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [style, reduced]);

  // Correction: under reduced motion the effect above draws exactly one
  // frame and then runs no loop, so without this a reduced-motion user would
  // see the chain freeze at whatever it showed on mount and never reflect
  // new evidence — a permanently stale governance reading. This effect does
  // not rebuild the renderer or canvas transform (that stays owned by the
  // effect above); it only asks the still frame already established there to
  // repaint with the latest packets/state. A no-op whenever motion is not
  // reduced, since repaintStillRef is null in that case.
  useEffect(() => {
    repaintStillRef.current?.();
  }, [packets, state]);

  return <canvas ref={canvasRef} className="chain-canvas" style={{ height }} aria-hidden />;
}
