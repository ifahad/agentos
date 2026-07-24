import "./charts.css";

export interface SparklineProps {
  /** Plain series values, oldest first. */
  values: readonly number[];
  /** Accessible name, e.g. "Requests over the last 24 hours". */
  label: string;
  width?: number;
  height?: number;
  className?: string;
}

/**
 * Thin accent stroke + soft accent-dim area fill, no axes. Matches the
 * design-card Stat sparkline: 1.5px round-joined segments.
 */
export function Sparkline({
  values,
  label,
  width = 120,
  height = 24,
  className,
}: SparklineProps) {
  const pad = 1; // keep the 1.5px stroke inside the viewBox
  const { line, area } = buildPaths(values, width, height, pad);

  return (
    <svg
      className={className ? `chart ${className}` : "chart"}
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      role="img"
      aria-label={label}
    >
      {line && <path className="chart-area" d={area} />}
      {line && <path className="chart-line" d={line} />}
    </svg>
  );
}

/** Map values to a line path plus its closed area path. Empty input yields empty paths. */
function buildPaths(
  values: readonly number[],
  w: number,
  h: number,
  pad: number,
): { line: string; area: string } {
  const clean = values.filter((v) => Number.isFinite(v));
  if (clean.length === 0) return { line: "", area: "" };

  const n = clean.length;
  const max = Math.max(...clean);
  const min = Math.min(...clean);
  const range = max - min;
  const x = (i: number) => (n === 1 ? pad : pad + (i / (n - 1)) * (w - 2 * pad));
  const y = (v: number) =>
    range === 0 ? h / 2 : pad + (1 - (v - min) / range) * (h - 2 * pad);

  const coords = clean.map((v, i) => [x(i), y(v)] as const);
  // A single value reads as a constant: draw it across the full width.
  const pts = n === 1 ? ([[pad, coords[0][1]], [w - pad, coords[0][1]]] as const) : coords;

  const line = pts.map(([px, py], i) => `${i === 0 ? "M" : "L"}${r(px)} ${r(py)}`).join(" ");
  const area = `${line} L${r(pts[pts.length - 1][0])} ${h - pad} L${r(pts[0][0])} ${h - pad} Z`;
  return { line, area };
}

/** Round to 2 decimals — keeps path strings short without visible error. */
function r(n: number): number {
  return Math.round(n * 100) / 100;
}
