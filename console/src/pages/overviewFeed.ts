// Pure helpers for the Overview activity feed and budget meters.
// No fetch logic here — the page wires these to the existing endpoints.
import { budgetFraction } from "../lib/format";
import type { AuditEntry, KeyInfo } from "../lib/types";

/** How long a just-written audit entry reads as "in flight" (pulsing dot). */
export const INFLIGHT_WINDOW_MS = 15_000;

/**
 * Stable identity for an audit entry. The audit endpoint returns newest-first
 * rows with no ids, so identity is derived from the fields that together are
 * unique for a single recorded request.
 */
export function feedEntryId(e: AuditEntry): string {
  return [e.ts, e.key_name, e.model, e.kind, e.status, e.latency_ms].join("|");
}

function tsMillis(ts: string): number {
  const ms = Date.parse(ts);
  return Number.isNaN(ms) ? 0 : ms;
}

/**
 * Merge a freshly polled page of audit entries into the current feed:
 * dedupe by feedEntryId, sort newest-first (unparseable timestamps sink to
 * the bottom), and cap the total length so dropped entries can animate out.
 * Pure — the same inputs always yield the same feed.
 */
export function mergeFeedEntries(
  current: readonly AuditEntry[],
  incoming: readonly AuditEntry[],
  cap: number,
): AuditEntry[] {
  const seen = new Set<string>();
  const out: AuditEntry[] = [];
  for (const e of [...incoming, ...current]) {
    const id = feedEntryId(e);
    if (seen.has(id)) continue;
    seen.add(id);
    out.push(e);
  }
  out.sort((a, b) => tsMillis(b.ts) - tsMillis(a.ts));
  return cap > 0 ? out.slice(0, cap) : out;
}

/**
 * An entry counts as in-flight when it was recorded within `windowMs` of
 * `nowMs` — presentation heuristic only (the audit log is written at
 * completion; there is no in-flight endpoint).
 */
export function isInflight(
  e: AuditEntry,
  nowMs: number,
  windowMs: number = INFLIGHT_WINDOW_MS,
): boolean {
  const ms = Date.parse(e.ts);
  if (Number.isNaN(ms)) return false;
  const age = nowMs - ms;
  return age >= 0 && age <= windowMs;
}

export interface BudgetMeter {
  /**
   * Stable list identity. GET /admin/keys returns no key id and names are not
   * unique — several distinct keys are routinely called the same thing — so
   * rows are disambiguated by their position in the sorted result. Sorting is
   * total (fraction, then name), which makes this deterministic for a given
   * response rather than merely incidental.
   */
  id: string;
  name: string;
  spend: number;
  budget: number;
  /** spend / budget clamped to [0, 1]. */
  fraction: number;
}

/**
 * Budget-meter rows from GET /admin/keys: keys with a real monthly budget,
 * fullest first (ties alphabetical), capped. Keys with no budget (0) are
 * unlimited and don't get a meter.
 */
export function budgetMeters(keys: readonly KeyInfo[], cap = 6): BudgetMeter[] {
  const rows = keys
    .filter((k) => Number.isFinite(k.monthly_budget_usd) && k.monthly_budget_usd > 0)
    .map((k) => ({
      name: k.name,
      spend: k.spend_usd,
      budget: k.monthly_budget_usd,
      fraction: budgetFraction(k.spend_usd, k.monthly_budget_usd),
    }));
  rows.sort((a, b) => b.fraction - a.fraction || a.name.localeCompare(b.name));
  const capped = cap > 0 ? rows.slice(0, cap) : rows;
  return capped.map((row, i) => ({ ...row, id: `${row.name}#${i}` }));
}
