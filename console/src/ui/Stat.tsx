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
}

/** Label + count-up value + optional sparkline, per the approved stat card. */
export function Stat({ label, value, decimals, prefix, suffix, delay = 0, spark }: StatProps) {
  const display = useCountUp(value, { decimals, prefix, suffix, delay });
  return (
    <Card>
      <div className="label">{label}</div>
      <div className="value">{display}</div>
      {spark ? <div className="ui-stat-spark">{spark}</div> : null}
    </Card>
  );
}
