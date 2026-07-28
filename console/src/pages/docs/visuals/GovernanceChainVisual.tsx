import { useEffect, useState } from "react";
import { useReducedMotion } from "framer-motion";
import { CHAIN_STAGES } from "../../../lib/chain";
import { GOVERNANCE_FRAME_MS, GOVERNANCE_SCRIPT, clearedCount } from "./geometry";
import { Illustration } from "./Illustration";

/**
 * The gateway's real /v1/* pipeline, stepped through a fixed script.
 *
 * Deliberately NOT the topbar chain: that one is lit by recorded audit statuses
 * and is evidence. This one is a scripted illustration, so it is drawn larger,
 * captioned, and marked as such. It shares CHAIN_STAGES with the instrument so
 * the two can never drift apart on what the stages are.
 */
export function GovernanceChainVisual() {
  const reduced = useReducedMotion();
  const [frameIndex, setFrameIndex] = useState(0);

  useEffect(() => {
    // Never start the timer under reduced motion: the global CSS cap does not
    // touch JS intervals, and a still is the whole point there.
    if (reduced) return;

    let id: number | undefined;
    const start = () => {
      if (id === undefined) {
        id = window.setInterval(() => {
          setFrameIndex((i) => (i + 1) % GOVERNANCE_SCRIPT.length);
        }, GOVERNANCE_FRAME_MS);
      }
    };
    const stop = () => {
      if (id !== undefined) {
        window.clearInterval(id);
        id = undefined;
      }
    };
    // installVisibilityPause only pauses live-resource polls, not our timers.
    const onVisibility = () => (document.hidden ? stop() : start());

    if (!document.hidden) start();
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      stop();
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [reduced]);

  // Under reduced motion, show the frame that teaches the most: a denial, so the
  // "stages after the halt stay unlit" rule is visible in the still.
  const frame = reduced
    ? (GOVERNANCE_SCRIPT.find((f) => f.stoppedAt !== null) ?? GOVERNANCE_SCRIPT[0])
    : GOVERNANCE_SCRIPT[frameIndex];
  const cleared = clearedCount(frame);

  return (
    <Illustration diagram="governanceChain">
      <ol className="docs-chain" data-outcome={frame.outcome}>
        {CHAIN_STAGES.map((stage, i) => {
          const render =
            frame.stoppedAt === stage ? "stopped" : i < cleared ? "cleared" : "unlit";
          return (
            <li key={stage} className="docs-chain-stage" data-render={render}>
              <span className="docs-chain-node" aria-hidden />
              <span className="docs-chain-label mono">{stage}</span>
            </li>
          );
        })}
      </ol>
      <p className="docs-chain-caption mono" aria-live="off">
        {frame.caption}
      </p>
    </Illustration>
  );
}
