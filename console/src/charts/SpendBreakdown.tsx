import "./charts.css";
import type { BreakdownRow } from "./transforms";

export interface SpendBreakdownProps {
  /** Aggregated rows (see breakdownFromRows), largest first. */
  rows: readonly BreakdownRow[];
  /** Accessible name, e.g. "Spend by model". */
  label: string;
  /** Formatter for the value column (e.g. formatUSD for spend). */
  formatValue?: (v: number) => string;
  className?: string;
}

/**
 * Horizontal bar breakdown by model or key. Every row shows its label and
 * value as text — differentiation never relies on color (all bars share the
 * single accent; position + text carry identity). Bars grow in with a
 * staggered scaleX entrance, disabled under prefers-reduced-motion.
 */
export function SpendBreakdown({
  rows,
  label,
  formatValue = defaultFormat,
  className,
}: SpendBreakdownProps) {
  const max = rows.reduce((m, row) => Math.max(m, row.value), 0);
  const summary = rows
    .slice(0, 5)
    .map((row) => `${row.label} ${formatValue(row.value)}`)
    .join(", ");

  return (
    <div
      className={className ? `breakdown ${className}` : "breakdown"}
      role="img"
      aria-label={summary ? `${label}: ${summary}` : `${label}: no data`}
    >
      {rows.length === 0 && <div className="breakdown-empty">No data</div>}
      {rows.map((row, i) => (
        <div className="breakdown-row" key={row.label} aria-hidden="true">
          <span className="breakdown-label" title={row.label}>
            {row.label}
          </span>
          <span className="breakdown-track">
            <span
              className="breakdown-fill"
              style={{
                display: "block",
                width: max > 0 ? `${(row.value / max) * 100}%` : "0%",
                animationDelay: `${Math.min(i, 10) * 40}ms`,
              }}
            />
          </span>
          <span className="breakdown-value">{formatValue(row.value)}</span>
        </div>
      ))}
    </div>
  );
}

function defaultFormat(v: number): string {
  return Number.isFinite(v) ? v.toLocaleString("en-US", { maximumFractionDigits: 2 }) : "0";
}
