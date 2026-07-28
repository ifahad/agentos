/**
 * Governance chain: how far a request got before the gateway stopped it.
 *
 * Every call through AgentOS runs an ordered gauntlet, and the gateway reports
 * where it ended purely through what it writes to the audit log. This module
 * turns one audit row back into a stage, so the console can show the chain
 * without inventing a new endpoint or a new field.
 *
 * This is the gateway's actual `/v1/*` proxy pipeline — not the `/admin/*`
 * pipeline, which is the only place rbac runs:
 *   auth      -> Authenticate() resolved the bearer token to a virtual key
 *   rate      -> rateLimited() found the key's org within its per-org bucket
 *   budget    -> admitSpend() found the key and its org within budget
 *   guardrail -> guard.Screen() did not block the prompt (chat only)
 *   upstream  -> the routed provider (or council) answered
 *   audit     -> the outcome was written to the audit log
 *
 * The mapping is evidence-aware, not status-aware: it reads the audit row's
 * `kind` first and its `status` only to tell a clean pass from a provider
 * failure, because raw status alone conflates two things that must not be
 * conflated:
 *
 *  1. A `kind: chat` or `kind: embeddings` row only exists once auth, rate,
 *     budget, and (for chat, when enabled) guardrail all cleared — proxy()
 *     writes it with `Status: resp.StatusCode`, the *provider's* status
 *     verbatim (gateway/internal/server/server.go). So a chat row carrying
 *     401, 429, or 400 is the upstream provider's own credential, rate-limit,
 *     or request-shape problem — never the caller's. Reading it as "denied at
 *     auth" (or rate, or guardrail) would blame the wrong side of the proxy.
 *  2. guard.Screen() is called exactly once, from handleChatCompletions, and
 *     never from handleEmbeddings — so embeddings never runs a screener at
 *     all, and even a `chat` row with no guardrail_flag/guardrail_block/
 *     guardrail_error row next to it is not proof the screener ran (it also
 *     runs it when AGENTOS_GUARDRAILS_MODE=off). A chain that lit "guardrail"
 *     on every chat pass would draw a check that provably did not always run.
 *     `ChainState.unproven` exists to say "not stopped here" without also
 *     claiming "verified to have run."
 *
 * The seven kinds the gateway actually writes (gateway/internal/store/store.go:
 * KindChat, KindEmbeddings, KindGuardrailFlag, KindGuardrailBlock,
 * KindGuardrailError, KindRateLimited, KindSecretReload) are `chat`,
 * `embeddings`, `guardrail_flag`, `guardrail_block`, `guardrail_error`,
 * `rate_limited`, `secret_reload`. auth (401) and budget (402) denials are
 * real, honest stages — but Authenticate() and admitSpend() write no audit
 * row on their early-return paths, so no row with a real kind will ever
 * carry them; there is nothing to switch on for those two statuses today.
 *
 * Six of those seven kinds describe a `/v1/*` request. `secret_reload` does
 * not — it is the administration plane, written when an operator clicks
 * Reload on the Secrets page — and the audit feed carries both, unfiltered.
 * So placing "the newest row" on the chain is only honest once the admin-plane
 * rows are out of the way; see ADMIN_PLANE_KINDS and `latestChainState`.
 *
 * Keep this in sync with the gateway's /v1/* pipeline and its audit kinds. If
 * a stage or a kind is added there, add it here — a chain that under-reports,
 * or over-claims what a lit stage proves, is worse than no chain, because it
 * implies checks ran that did not.
 */

/** Ordered stages of the gauntlet. Index is position in the chain. */
export const CHAIN_STAGES = ["auth", "rate", "budget", "guardrail", "upstream", "audit"] as const;

export type ChainStage = (typeof CHAIN_STAGES)[number];

/** What became of a request once it stopped moving. */
export type ChainOutcome = "pass" | "deny" | "fail";

export interface ChainState {
  /**
   * How far the rule inks, 0..CHAIN_STAGES.length: the stages this row places
   * the request at or past. `stoppedAt` and `unproven` carve individual nodes
   * back out of it, so a full `cleared` is not by itself a claim that every
   * node ran — see `stageRenders`.
   */
  cleared: number;
  /** Stage that halted it, or null when the whole chain cleared. */
  stoppedAt: ChainStage | null;
  outcome: ChainOutcome;
  /**
   * Stages sitting below `cleared` (or past a `fail`) that the evidence does
   * not actually prove ran. `stageRenders` draws these unlit rather than
   * cleared: a stage the request merely was not stopped at is not the same
   * claim as a stage the console can prove executed.
   */
  unproven: readonly ChainStage[];
}

/** The chain at rest: nothing observed yet, nothing claimed. */
export const IDLE_CHAIN: ChainState = {
  cleared: 0,
  stoppedAt: null,
  outcome: "pass",
  unproven: [],
};

/**
 * The gateway's audit kinds (gateway/internal/store/store.go: KindChat,
 * KindEmbeddings, KindGuardrailFlag, KindGuardrailBlock, KindGuardrailError,
 * KindRateLimited, KindSecretReload). The console's shared `AuditKind` (see
 * lib/types.ts) only declares the four kinds today's audit-page badges style;
 * this chain needs the full seven to place a request honestly, so it keeps
 * its own wider view of the same field rather than widen the shared type
 * (and, with it, every unrelated `Badge variant={kind}` call site).
 */
export type GatewayAuditKind =
  | "chat"
  | "embeddings"
  | "guardrail_flag"
  | "guardrail_block"
  | "guardrail_error"
  | "rate_limited"
  | "secret_reload";

/** The slice of an audit row this module needs to place a request on the chain. */
export interface ChainEvidence {
  status: number;
  kind: GatewayAuditKind;
}

/**
 * Audit kinds the gateway writes from its administration plane rather than
 * from a `/v1/*` request.
 *
 * `handleSecretsReload` writes `secret_reload` whenever an operator clicks
 * Reload on the Secrets page (gateway/internal/server/secrets.go), and
 * `AuditList` applies no kind filter (gateway/internal/store/postgres.go), so
 * that row lands in the very feed this module reads and — on a quiet system —
 * sits at its head until the next `/v1` call. It describes no request, so it
 * must place none: reporting it as a denial would announce a refusal for a
 * request nobody made.
 */
const ADMIN_PLANE_KINDS: ReadonlySet<string> = new Set<string>(["secret_reload"]);

/**
 * Map one audit row onto the stage the request reached.
 *
 * A kind this module does not recognise is treated as a denial at the first
 * stage rather than as a pass: the console must never draw checks it cannot
 * prove ran. Admin-plane kinds are the one exception, and they are not an
 * exception to that rule but an application of it — they evidence no `/v1`
 * request at all, so the honest reading is the idle chain, which claims
 * nothing in either direction.
 */
export function chainStateFromEntry(entry: ChainEvidence): ChainState {
  const { status, kind } = entry;
  const providerOK = status >= 200 && status < 300;

  switch (kind) {
    case "rate_limited":
      // rateLimited() writes this kind for a 429 the gateway itself produced
      // by rejecting the org's per-org token bucket — only auth ran first.
      return { cleared: 1, stoppedAt: "rate", outcome: "deny", unproven: [] };

    case "guardrail_block":
      // handleChatCompletions writes this kind when guard.Screen() flags the
      // prompt in block/model mode: auth, rate, and budget all cleared first.
      return { cleared: 3, stoppedAt: "guardrail", outcome: "deny", unproven: [] };

    case "guardrail_flag":
    case "guardrail_error":
      // Both rows are written only from inside the guard.Screen() branch of
      // handleChatCompletions (a flag forwarded in log mode, or a classifier
      // failing open), so the row's mere existence proves the screener ran
      // and let the request through — auth, rate, budget, and guardrail are
      // all evidenced, and audit is too, because the row being read *is* the
      // audit record.
      //
      // upstream is not. That switch runs entirely before proxyCouncil() and
      // proxy() are ever called (gateway/internal/server/server.go), so the
      // row proves nothing whatever about the provider. For a slow or
      // streaming completion in log mode it is the newest row for the entire
      // in-flight window; lighting upstream there would assert "the provider
      // or council answered" — the node's own tooltip — while the request is
      // still open. Nothing in the row says upstream failed either, so it is
      // unproven rather than stopped.
      return {
        cleared: CHAIN_STAGES.length,
        stoppedAt: null,
        outcome: "pass",
        unproven: ["upstream"],
      };

    case "chat":
    case "embeddings":
      if (providerOK) {
        // auth, rate, and budget cleared (a chat/embeddings row cannot exist
        // otherwise). guardrail is unproven either way: embeddings has no
        // screener at all, and a bare `chat` row carries no evidence it ran
        // either — no guardrail_flag/guardrail_block/guardrail_error row
        // means only "not flagged and not errored," which is indistinguishable
        // from "screening is off" from this row alone.
        return {
          cleared: CHAIN_STAGES.length,
          stoppedAt: null,
          outcome: "pass",
          unproven: ["guardrail"],
        };
      }
      // proxy() records Status: resp.StatusCode verbatim on this kind — this
      // status is whatever the upstream provider returned. A chat/embeddings
      // row only exists once governance already cleared, so a 401/429/400
      // here is the provider's own credential, rate limit, or request-shape
      // problem, never the caller's — it stops at upstream, not at auth,
      // rate, or guardrail. guard.Screen() runs (or is skipped under
      // ModeOff) entirely before proxy() is ever called, so its evidentiary
      // status does not depend on whether the provider then succeeds or
      // fails: guardrail is exactly as unproven here as on the 2xx pass
      // above, and must render exactly as dark.
      //
      // audit, by contrast, is proven: proxy() wrote the very row being read,
      // so the outcome demonstrably was recorded. Leaving that node dark
      // would under-report a check the console is holding the evidence for.
      // `stoppedAt` still marks upstream, and the fail hue still colors the
      // rule, so a full extent here reads as "ran the whole gauntlet, the
      // provider broke" rather than as a clean pass.
      return {
        cleared: CHAIN_STAGES.length,
        stoppedAt: "upstream",
        outcome: "fail",
        unproven: ["guardrail"],
      };

    case "secret_reload":
      // An administration action, not a /v1/* request: reloading secrets
      // proves nothing about the proxy pipeline and denies nothing on it.
      // `latestChainState` filters this kind out before ever reaching here,
      // but this function is exported, so it must not lie to a direct caller
      // either.
      return IDLE_CHAIN;

    default:
      // Any kind this module does not recognise. Fail closed rather than draw
      // a check we cannot prove: never treat an unrecognised kind as a pass.
      return { cleared: 0, stoppedAt: "auth", outcome: "deny", unproven: [] };
  }
}

/**
 * Chain position for the most recent entry in an audit feed.
 *
 * Entries are assumed newest-first, matching GET /admin/audit. An empty feed
 * yields the idle chain — no activity is not the same as a passing request.
 *
 * Admin-plane rows are dropped before the head is taken, not merely skipped
 * when they land first: a `secret_reload` must neither place a request of its
 * own nor hide the last real one behind it. Anything not known to be
 * admin-plane stays in — including a kind this module has never seen — so an
 * unknown kind still reaches the fail-closed default rather than being
 * quietly dropped into an idle chain.
 */
export function latestChainState(entries: readonly ChainEvidence[]): ChainState {
  const requests = entries.filter((entry) => !ADMIN_PLANE_KINDS.has(entry.kind));
  if (requests.length === 0) return IDLE_CHAIN;
  return chainStateFromEntry(requests[0]);
}

/**
 * Per-stage render state, in chain order.
 *
 * "cleared" stages are drawn in ink, the halting stage takes the outcome
 * color, and stages after it — or that the evidence does not prove ran, even
 * if they sit below `cleared` — stay unlit. Showing an unproven stage as
 * anything but dark would be a lie.
 */
export type StageRender = "cleared" | "stopped" | "unlit";

export function stageRenders(state: ChainState): StageRender[] {
  return CHAIN_STAGES.map((stage, i) => {
    if (state.stoppedAt === stage) return "stopped";
    if (state.unproven.includes(stage)) return "unlit";
    return i < state.cleared ? "cleared" : "unlit";
  });
}
