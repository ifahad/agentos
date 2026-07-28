/**
 * Governance chain: how far a request got before the gateway stopped it.
 *
 * Every call through AgentOS runs an ordered gauntlet, and the gateway reports
 * where it ended purely through the HTTP status it records in the audit log.
 * This module turns that status back into a stage, so the console can show the
 * chain without inventing a new endpoint or a new field.
 *
 * This is the gateway's actual `/v1/*` proxy pipeline — not the `/admin/*`
 * pipeline, which is the only place rbac runs:
 *   401 unauthenticated      -> stopped at auth
 *   429 too many requests    -> stopped at rate (rate limiting runs before budget)
 *   402 payment required     -> stopped at budget (key or org exhausted)
 *   400 malformed            -> stopped at guardrail (a guardrail_block audit row)
 *   5xx upstream failure     -> cleared governance, provider failed
 *   2xx                      -> cleared the chain and was recorded
 *
 * Keep this in sync with the gateway's /v1/* pipeline. If a stage is added
 * there, add it here — a chain that under-reports is worse than no chain,
 * because it implies checks ran that did not.
 */

/** Ordered stages of the gauntlet. Index is position in the chain. */
export const CHAIN_STAGES = ["auth", "rate", "budget", "guardrail", "upstream", "audit"] as const;

export type ChainStage = (typeof CHAIN_STAGES)[number];

/** What became of a request once it stopped moving. */
export type ChainOutcome = "pass" | "deny" | "fail";

export interface ChainState {
  /** Count of stages cleared, 0..CHAIN_STAGES.length. */
  cleared: number;
  /** Stage that halted it, or null when the whole chain cleared. */
  stoppedAt: ChainStage | null;
  outcome: ChainOutcome;
}

/** The chain at rest: nothing observed yet, nothing claimed. */
export const IDLE_CHAIN: ChainState = { cleared: 0, stoppedAt: null, outcome: "pass" };

/**
 * Map a recorded HTTP status onto the stage the request reached.
 *
 * The evidence source is the audit log, and the gateway only writes audit rows
 * for seven kinds: chat, embeddings, guardrail_flag, guardrail_block,
 * guardrail_error, rate_limited, secret_reload. So in practice the statuses
 * that reach us are 2xx, 400 (a guardrail block), 429 (a rate-limit rejection),
 * and 5xx (an upstream failure after governance cleared).
 *
 * 401 and 402 are mapped for completeness — they are the honest stage for those
 * statuses — but the gateway does not currently audit them, so they should not
 * appear in this feed. Unknown statuses are treated as denials at the first
 * stage rather than as passes: the console must never draw checks it cannot
 * prove ran.
 */
export function chainStateFromStatus(status: number): ChainState {
  if (status >= 200 && status < 300) {
    return { cleared: CHAIN_STAGES.length, stoppedAt: null, outcome: "pass" };
  }
  if (status >= 500) {
    // Governance cleared; the upstream provider is what broke.
    return { cleared: 4, stoppedAt: "upstream", outcome: "fail" };
  }
  switch (status) {
    case 401:
      return { cleared: 0, stoppedAt: "auth", outcome: "deny" };
    case 429:
      return { cleared: 1, stoppedAt: "rate", outcome: "deny" };
    case 402:
      return { cleared: 2, stoppedAt: "budget", outcome: "deny" };
    case 400:
      return { cleared: 3, stoppedAt: "guardrail", outcome: "deny" };
    default:
      return { cleared: 0, stoppedAt: "auth", outcome: "deny" };
  }
}

/**
 * Chain position for the most recent entry in an audit feed.
 *
 * Entries are assumed newest-first, matching GET /admin/audit. An empty feed
 * yields the idle chain — no activity is not the same as a passing request.
 */
export function latestChainState(entries: readonly { status: number }[]): ChainState {
  if (entries.length === 0) return IDLE_CHAIN;
  return chainStateFromStatus(entries[0].status);
}

/**
 * Per-stage render state, in chain order.
 *
 * "cleared" stages are drawn in ink, the halting stage takes the outcome color,
 * and stages after it stay unlit — the request never reached them, so showing
 * them as anything but dark would be a lie.
 */
export type StageRender = "cleared" | "stopped" | "unlit";

export function stageRenders(state: ChainState): StageRender[] {
  return CHAIN_STAGES.map((stage, i) => {
    if (state.stoppedAt === stage) return "stopped";
    return i < state.cleared ? "cleared" : "unlit";
  });
}
