import { useEffect, useRef, useState } from "react";

/** easeOutCubic — the count-up curve from the approved stat-card design. */
export function easeOutCubic(t: number): number {
  return 1 - Math.pow(1 - t, 3);
}

export interface CountFormatOptions {
  decimals?: number;
  prefix?: string;
  suffix?: string;
}

/** Format a count-up value with tabular-friendly grouping. */
export function formatCount(value: number, opts: CountFormatOptions = {}): string {
  const { decimals = 0, prefix = "", suffix = "" } = opts;
  return (
    prefix +
    value.toLocaleString("en-US", {
      minimumFractionDigits: decimals,
      maximumFractionDigits: decimals,
    }) +
    suffix
  );
}

/**
 * Value at `elapsed` ms into a count-up to `target` over `duration` ms.
 * Pure — the hook is a thin rAF wrapper around this.
 */
export function countUpValue(target: number, elapsed: number, duration: number): number {
  if (duration <= 0) return target;
  const p = Math.min(1, Math.max(0, elapsed / duration));
  return target * easeOutCubic(p);
}

function prefersReducedMotion(): boolean {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

export interface UseCountUpOptions extends CountFormatOptions {
  /** Total animation time. Default 700ms (per approved design card). */
  duration?: number;
  /** Start delay, e.g. index * 60 for staggered stat cards. */
  delay?: number;
}

/**
 * Count up from 0 to `target` on mount / whenever `target` changes.
 * Returns the formatted display string. Under prefers-reduced-motion the
 * final value renders immediately.
 */
export function useCountUp(target: number, opts: UseCountUpOptions = {}): string {
  const { duration = 700, delay = 0, decimals = 0, prefix = "", suffix = "" } = opts;
  const fmtOpts: CountFormatOptions = { decimals, prefix, suffix };
  const [display, setDisplay] = useState(() =>
    prefersReducedMotion() ? formatCount(target, fmtOpts) : formatCount(0, fmtOpts),
  );
  const rafRef = useRef(0);

  useEffect(() => {
    if (prefersReducedMotion()) {
      setDisplay(formatCount(target, fmtOpts));
      return;
    }
    const t0 = performance.now() + delay;
    const tick = (now: number) => {
      const elapsed = now - t0;
      setDisplay(formatCount(countUpValue(target, elapsed, duration), fmtOpts));
      if (elapsed < duration) {
        rafRef.current = requestAnimationFrame(tick);
      } else {
        setDisplay(formatCount(target, fmtOpts));
      }
    };
    rafRef.current = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(rafRef.current);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [target, duration, delay, decimals, prefix, suffix]);

  return display;
}
