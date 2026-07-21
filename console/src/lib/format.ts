// Pure formatting helpers for the console UI.

/** "$12.34" for dollars-and-up, "$0.0042" for sub-dollar amounts, "$0.00" for zero. */
export function formatUSD(n: number): string {
  if (!Number.isFinite(n)) return "$0.00";
  const abs = Math.abs(n);
  if (abs !== 0 && abs < 1) return `$${n.toFixed(4)}`;
  return `$${n.toFixed(2)}`;
}

/** Thousands-separated integer: 1234567 -> "1,234,567". */
export function formatInt(n: number): string {
  if (!Number.isFinite(n)) return "0";
  return Math.round(n).toLocaleString("en-US");
}

/** "834 ms" below one second, "1.24 s" at or above. */
export function formatLatency(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return "–";
  if (ms < 1000) return `${Math.round(ms)} ms`;
  return `${(ms / 1000).toFixed(2)} s`;
}

/** RFC3339 timestamp -> "YYYY-MM-DD HH:MM:SS" in local time; input on parse failure. */
export function formatTimestamp(ts: string): string {
  const d = new Date(ts);
  if (Number.isNaN(d.getTime())) return ts;
  const p = (x: number) => String(x).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(
    d.getMinutes(),
  )}:${p(d.getSeconds())}`;
}

/** Compact one-line JSON for tool inputs, truncated with an ellipsis. */
export function compactJSON(value: unknown, maxLen = 400): string {
  let s: string;
  try {
    s = JSON.stringify(value) ?? String(value);
  } catch {
    s = String(value);
  }
  return s.length > maxLen ? `${s.slice(0, maxLen - 1)}…` : s;
}

/** Fraction of budget spent, clamped to [0, 1]; 0 when the budget is 0. */
export function budgetFraction(spend: number, budget: number): number {
  if (!Number.isFinite(spend) || !Number.isFinite(budget) || budget <= 0) return 0;
  return Math.min(1, Math.max(0, spend / budget));
}
