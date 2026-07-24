import { useEffect, useRef, useState } from "react";
import "./charts.css";

export interface UsageChartProps {
  /** Plain series values, oldest first. */
  values: readonly number[];
  /** Accessible name, e.g. "Requests per day". */
  label: string;
  /** Optional x labels; only the first and last are rendered. */
  xLabels?: readonly string[];
  /** Plot height in px (the svg fills its container's width). Default 160. */
  height?: number;
  /** Formatter for the y tick labels. */
  formatValue?: (v: number) => string;
  className?: string;
}

const PAD_LEFT = 36;
const PAD_RIGHT = 8;
const PAD_TOP = 8;
const TICK_COUNT = 4; // hard cap per the dataviz rules

/**
 * Line/area chart over time. Minimal axes (max 4 dim y ticks, hairline
 * gridlines, optional first/last x labels), thin accent stroke, soft
 * accent-dim area fill. The stroke draws in on mount via stroke-dashoffset;
 * the animation is CSS-driven and disabled under prefers-reduced-motion.
 */
export function UsageChart({
  values,
  label,
  xLabels,
  height = 160,
  formatValue = defaultFormat,
  className,
}: UsageChartProps) {
  const [ref, width] = useContainerWidth();
  const clean = values.filter((v) => Number.isFinite(v));

  const padBottom = xLabels && xLabels.length > 0 ? 18 : 6;
  const plotW = Math.max(width - PAD_LEFT - PAD_RIGHT, 1);
  const plotH = Math.max(height - PAD_TOP - padBottom, 1);

  const max = clean.length > 0 ? Math.max(...clean) : 0;
  const step = niceStep(max / (TICK_COUNT - 1) || 1);
  const top = step * (TICK_COUNT - 1);
  const ticks = Array.from({ length: TICK_COUNT }, (_, i) => step * i);

  const n = clean.length;
  const x = (i: number) =>
    PAD_LEFT + (n <= 1 ? 0 : (i / (n - 1)) * plotW);
  const y = (v: number) => PAD_TOP + (1 - v / top) * plotH;

  let line = "";
  let area = "";
  if (n > 0) {
    const coords = clean.map((v, i) => [x(i), y(v)] as const);
    const pts =
      n === 1
        ? ([[PAD_LEFT, coords[0][1]], [PAD_LEFT + plotW, coords[0][1]]] as const)
        : coords;
    line = pts.map(([px, py], i) => `${i === 0 ? "M" : "L"}${r(px)} ${r(py)}`).join(" ");
    area = `${line} L${r(pts[pts.length - 1][0])} ${r(PAD_TOP + plotH)} L${r(pts[0][0])} ${r(
      PAD_TOP + plotH,
    )} Z`;
  }

  return (
    <div ref={ref} className={className} style={{ width: "100%" }}>
      {width > 0 && (
        <svg
          className="chart"
          width={width}
          height={height}
          viewBox={`0 0 ${width} ${height}`}
          role="img"
          aria-label={label}
        >
          {ticks.slice(1).map((t) => (
            <line
              key={t}
              className="chart-grid"
              x1={PAD_LEFT}
              x2={PAD_LEFT + plotW}
              y1={y(t)}
              y2={y(t)}
            />
          ))}
          {ticks.map((t) => (
            <text
              key={t}
              className="chart-axis-label"
              x={PAD_LEFT - 6}
              y={y(t)}
              textAnchor="end"
              dominantBaseline="central"
            >
              {formatValue(t)}
            </text>
          ))}
          {area && <path className="chart-area chart-area--fade" d={area} />}
          {line && <path className="chart-line chart-line--draw" pathLength={1} d={line} />}
          {xLabels && xLabels.length > 0 && (
            <>
              <text
                className="chart-axis-label"
                x={PAD_LEFT}
                y={height - 4}
                textAnchor="start"
              >
                {xLabels[0]}
              </text>
              {xLabels.length > 1 && (
                <text
                  className="chart-axis-label"
                  x={PAD_LEFT + plotW}
                  y={height - 4}
                  textAnchor="end"
                >
                  {xLabels[xLabels.length - 1]}
                </text>
              )}
            </>
          )}
        </svg>
      )}
    </div>
  );
}

/** Measured container width so the svg stays crisp at any size (no viewBox scaling). */
function useContainerWidth(): [React.RefObject<HTMLDivElement | null>, number] {
  const ref = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(0);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    setWidth(el.clientWidth);
    const ro = new ResizeObserver((entries) => {
      setWidth(entries[0]?.contentRect.width ?? 0);
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  return [ref, width];
}

/** Round `raw` up to a 1 / 2 / 2.5 / 5 × 10^k step so ticks stay human-readable. */
function niceStep(raw: number): number {
  if (!Number.isFinite(raw) || raw <= 0) return 1;
  const mag = 10 ** Math.floor(Math.log10(raw));
  const m = raw / mag;
  return (m <= 1 ? 1 : m <= 2 ? 2 : m <= 2.5 ? 2.5 : m <= 5 ? 5 : 10) * mag;
}

function defaultFormat(v: number): string {
  return v.toLocaleString("en-US", { maximumFractionDigits: 2 });
}

function r(n: number): number {
  return Math.round(n * 100) / 100;
}
