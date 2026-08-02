import type { ReactNode } from "react";
import { Card } from "./Card";
import { useCountUp } from "./useCountUp";

export interface StatProps {
  label: string;
  value: number;
  decimals?: number;
  prefix?: string;
  suffix?: string;
  /** Stagger delay in ms (e.g. index * 60). */
  delay?: number;
  /** Optional sparkline slot rendered under the value. */
  spark?: ReactNode;
  /**
   * The reading is unavailable — not zero. Renders an em dash instead of a
   * number. A 0 here would be a lie: it reports a measurement that was never
   * taken, and reads identically to a genuinely idle system.
   */
  unavailable?: boolean;
}

/** Label + count-up value + optional sparkline, per the approved stat card. */
export function Stat({
  label,
  value,
  decimals,
  prefix,
  suffix,
  delay = 0,
  spark,
  unavailable,
}: StatProps) {
  // Always called, even when unavailable — the count-up is just unused in
  // that render, keeping hook order stable across the two states.
  const display = useCountUp(value, { decimals, prefix, suffix, delay });
  return (
    <Card>
      <div className="label">{label}</div>
      {/* Reading and trace share a baseline, so a card with a series is exactly
          as tall as one without and the row of stats stays level. */}
      <div className="ui-stat-row">
        {/* aria-label overrides the punctuation as the accessible name, so a
            screen reader announces "Not available" rather than "em dash". */}
        <div className="value" aria-label={unavailable ? "Not available" : undefined}>
          {unavailable ? "—" : display}
        </div>
        {spark ? <div className="ui-stat-spark">{spark}</div> : null}
      </div>
    </Card>
  );
}
