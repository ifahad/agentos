/**
 * Replay layer: turns successive audit-feed snapshots into individually
 * animatable requests.
 *
 * The chain animates one mark per real audit row. That means answering two
 * questions this module owns and lib/chain.ts does not: which rows are NEW
 * since the last poll, and WHEN within the poll window each should appear.
 * Admin-plane rows (ADMIN_PLANE_KINDS from chain.ts) are filtered to keep
 * the diff layer and stage-render layer in agreement about what counts as traffic.
 *
 * Row identity is the hard part. AuditEntry has no id — every field is a
 * value, so two identical calls in the same millisecond are indistinguishable.
 * A key-based diff would therefore drop or duplicate marks under load. The
 * diff is positional instead: the feed is newest-first, so find the previous
 * head inside the new feed and take everything above it. A composite key is
 * used only to LOCATE that anchor, never to identify a row on its own.
 */

import { ADMIN_PLANE_KINDS, CHAIN_STAGES, chainStateFromEntry } from "./chain";
import type { ChainOutcome, ChainStage, GatewayAuditKind } from "./chain";
import type { AuditEntry } from "./types";

/**
 * Most rows replayed from a single poll. A quiet system never reaches this;
 * a burst would otherwise dump a hundred marks at once, which reads as noise
 * and costs frames. The newest are kept — the oldest are silently dropped,
 * because the alternative is animating a backlog as though it were live.
 */
export const MAX_REPLAY_PER_POLL = 12;

export interface ChainPacket {
  /** Synthetic, for React keying and renderer bookkeeping only. Never dedupe on this. */
  id: string;
  /** Epoch ms parsed from AuditEntry.ts. NaN-safe: unparseable becomes 0. */
  ts: number;
  /** Index into CHAIN_STAGES where the request halted, or -1 when it cleared. */
  stopIndex: number;
  outcome: ChainOutcome;
  latencyMs: number;
  /** Stages the evidence does not prove ran. Renderers must not light these. */
  unproven: readonly ChainStage[];
}

/**
 * Composite of every AuditEntry field — used solely to locate the diff anchor.
 *
 * Joined on NUL, not a space: every part is attacker- or operator-influenced
 * text (`key_name`, `model`), and a separator those values can themselves
 * contain lets two different rows collide on one key. A key name of
 * `alpha gpt-4o` beside an empty model is indistinguishable from `alpha` beside
 * `gpt-4o` under a space join, and a collision here makes `diffFeed` anchor on
 * the wrong row — replaying marks that already played, or dropping ones that
 * never did. NUL cannot appear in these fields.
 */
export function rowKey(entry: AuditEntry): string {
  return [
    entry.ts,
    entry.key_name,
    entry.model,
    entry.input_tokens,
    entry.output_tokens,
    entry.cost_usd,
    entry.latency_ms,
    entry.status,
    entry.kind,
  ].join("\0");
}

function requestRows(entries: readonly AuditEntry[]): AuditEntry[] {
  return entries.filter((e) => !ADMIN_PLANE_KINDS.has(e.kind));
}

/**
 * Rows in `next` that were not in `prev`, oldest-first.
 *
 * `prev === null` means first load: adopt the snapshot as baseline and replay
 * nothing, because 100 rows of history are not live traffic. The same applies
 * when the anchor cannot be found — the feed advanced by more than its limit
 * between polls, or the key changed — which is normal under load and is not
 * an error.
 */
export function diffFeed(
  prev: readonly AuditEntry[] | null,
  next: readonly AuditEntry[],
): AuditEntry[] {
  if (prev === null) return [];
  const fresh = requestRows(next);
  const seen = requestRows(prev);

  let newRows: AuditEntry[];
  if (seen.length === 0) {
    // No prior requests at all: everything present is new.
    newRows = fresh.slice();
  } else {
    const anchor = rowKey(seen[0]);
    const idx = fresh.findIndex((e) => rowKey(e) === anchor);
    if (idx === -1) return []; // re-seed rather than replay a backlog
    newRows = fresh.slice(0, idx);
  }

  // Cap to the newest N, then flip newest-first -> oldest-first for replay.
  return newRows.slice(0, MAX_REPLAY_PER_POLL).reverse();
}

/** One audit row as a packet. The verdict comes from lib/chain.ts, unchanged. */
export function toPacket(entry: AuditEntry, id: string): ChainPacket {
  const state = chainStateFromEntry({
    status: entry.status,
    kind: entry.kind as GatewayAuditKind,
  });
  const parsed = Date.parse(entry.ts);
  return {
    id,
    ts: Number.isNaN(parsed) ? 0 : parsed,
    stopIndex: state.stoppedAt === null ? -1 : CHAIN_STAGES.indexOf(state.stoppedAt),
    outcome: state.outcome,
    latencyMs: entry.latency_ms,
    unproven: state.unproven,
  };
}

/**
 * Delay in ms before each packet is released, index-aligned with `packets`.
 *
 * The batch is min-max NORMALISED onto `0 .. 0.9 * windowMs`: the oldest row
 * in the batch always releases at 0 and the newest at the end of that span,
 * whatever the real elapsed time between them. So what survives is the
 * batch's internal *proportions* and ordering — three rows clustered at the
 * start still replay clustered at the start — but not its real spacing. Two
 * rows 40ms apart and two rows 4s apart both stretch to fill the same span.
 * Absolute timing is deliberately not preserved: the window is fixed at one
 * poll cadence, and a burst has to fit inside it whatever it really spanned.
 *
 * The `max === min` branch is a divide-by-zero guard, not a common path.
 * `ChainPacket.ts` is `audit_log.created_at` (TIMESTAMPTZ DEFAULT now(), so
 * microsecond resolution at the source) parsed down to epoch milliseconds, so
 * two rows collide only when they landed in the same millisecond — possible
 * under a burst, but rare, not the norm. It stays because the alternative
 * when it does fire is `0/0` NaN offsets.
 *
 * 90% of the window is used, leaving headroom before the next poll lands.
 */
export function releaseSchedule(packets: readonly ChainPacket[], windowMs: number): number[] {
  const n = packets.length;
  if (n === 0) return [];
  if (n === 1) return [0];

  const span = windowMs * 0.9;
  const times = packets.map((p) => p.ts);
  const min = Math.min(...times);
  const max = Math.max(...times);

  if (max === min) {
    // No usable spread (every row in the same millisecond): distribute evenly
    // rather than divide by zero.
    return packets.map((_, i) => (span * i) / (n - 1));
  }
  return packets.map((p) => {
    const frac = (p.ts - min) / (max - min);
    return Math.min(windowMs, Math.max(0, frac * span));
  });
}
