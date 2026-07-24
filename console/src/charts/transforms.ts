// Pure data transforms for the console's hand-rolled SVG charts.
// Structural types only — no API contracts leak into the chart layer.

export interface SeriesPoint {
  /** Bucket start, epoch ms. */
  t: number;
  value: number;
}

/** Anything with a timestamp; `value` defaults to 1 (event count). */
export interface UsageEventLike {
  timestamp: string | number | Date;
  value?: number;
}

export type BucketUnit = "hour" | "day";

export interface SeriesOptions {
  /** Bucket granularity. Default "day". */
  bucket?: BucketUnit;
  /** Range start (inclusive); floored to the bucket. Defaults to the earliest event. */
  from?: number | string | Date;
  /** Range end (inclusive); floored to the bucket. Defaults to the latest event. */
  to?: number | string | Date;
}

export interface BreakdownRow {
  label: string;
  value: number;
}

export interface BreakdownOptions<T> {
  label: (row: T) => string;
  value: (row: T) => number;
  /** Max rows returned; the remainder folds into "Other". Default 8. */
  limit?: number;
  /** Label for the folded remainder. Default "Other". */
  otherLabel?: string;
}

const HOUR_MS = 3_600_000;
const DAY_MS = 86_400_000;
/** Safety cap so a pathological from/to range can't hang the renderer. */
const MAX_BUCKETS = 10_000;

function toMillis(ts: number | string | Date): number | null {
  const ms = ts instanceof Date ? ts.getTime() : new Date(ts).getTime();
  return Number.isNaN(ms) ? null : ms;
}

/** Floor to the bucket boundary. Day buckets are UTC-aligned (deterministic). */
function floorTo(ms: number, unit: BucketUnit): number {
  return unit === "day" ? Math.floor(ms / DAY_MS) * DAY_MS : Math.floor(ms / HOUR_MS) * HOUR_MS;
}

/**
 * Bucket timestamped usage/audit events into a contiguous ascending series.
 * Missing buckets between the first and last bucket are zero-filled; events
 * with unparseable timestamps are skipped. Empty input (and no from/to)
 * yields an empty series.
 */
export function seriesFromEvents(
  events: readonly UsageEventLike[],
  opts: SeriesOptions = {},
): SeriesPoint[] {
  const bucket = opts.bucket ?? "day";
  const step = bucket === "day" ? DAY_MS : HOUR_MS;

  const sums = new Map<number, number>();
  let min = Number.POSITIVE_INFINITY;
  let max = Number.NEGATIVE_INFINITY;
  for (const ev of events) {
    const ms = toMillis(ev.timestamp);
    if (ms === null) continue;
    const b = floorTo(ms, bucket);
    const v = ev.value !== undefined && Number.isFinite(ev.value) ? ev.value : 1;
    sums.set(b, (sums.get(b) ?? 0) + v);
    if (b < min) min = b;
    if (b > max) max = b;
  }

  const fromMs = opts.from !== undefined ? toMillis(opts.from) : null;
  const toMs = opts.to !== undefined ? toMillis(opts.to) : null;
  const start = fromMs !== null ? floorTo(fromMs, bucket) : min;
  const end = toMs !== null ? floorTo(toMs, bucket) : max;
  if (!Number.isFinite(start) || !Number.isFinite(end) || end < start) return [];

  const out: SeriesPoint[] = [];
  for (let t = start; t <= end && out.length < MAX_BUCKETS; t += step) {
    out.push({ t, value: sums.get(t) ?? 0 });
  }
  return out;
}

/**
 * Pad or truncate a plain number array to exactly `n` points, keeping the
 * most recent values: a longer input is sliced from the end, a shorter one
 * is left-padded with zeros. Non-finite values become 0; n <= 0 yields [].
 */
export function normalizeSeries(values: readonly number[], n: number): number[] {
  if (!Number.isInteger(n) || n <= 0) return [];
  const clean = values.map((v) => (Number.isFinite(v) ? v : 0));
  if (clean.length >= n) return clean.slice(clean.length - n);
  return new Array<number>(n - clean.length).fill(0).concat(clean);
}

/**
 * Aggregate arbitrary rows into a labelled breakdown: sum values per label,
 * sort descending (ties alphabetical), and fold everything beyond `limit`
 * into a trailing "Other" row. Non-finite values count as 0.
 */
export function breakdownFromRows<T>(
  rows: readonly T[],
  opts: BreakdownOptions<T>,
): BreakdownRow[] {
  const totals = new Map<string, number>();
  for (const row of rows) {
    const label = opts.label(row) || "unknown";
    const v = opts.value(row);
    totals.set(label, (totals.get(label) ?? 0) + (Number.isFinite(v) ? v : 0));
  }
  const sorted = [...totals.entries()]
    .map(([label, value]) => ({ label, value }))
    .sort((a, b) => b.value - a.value || a.label.localeCompare(b.label));

  const limit = opts.limit ?? 8;
  if (limit <= 0) return [];
  if (sorted.length <= limit) return sorted;

  const kept = sorted.slice(0, limit);
  const rest = sorted.slice(limit).reduce((s, r) => s + r.value, 0);
  if (rest !== 0) kept.push({ label: opts.otherLabel ?? "Other", value: rest });
  return kept;
}
