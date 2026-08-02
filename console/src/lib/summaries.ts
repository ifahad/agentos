/**
 * Panel summary lines.
 *
 * Every function here is pure and takes data the page already holds — no
 * fetching, no React. That is deliberate: the console's vitest runs in a node
 * environment with no DOM, so logic that needs testing has to live outside
 * components. The summary line is the part worth testing; the boolean beside
 * it is not.
 */

import type {
  AuditEntry,
  CouncilObjective,
  CouncilProposal,
  DocumentInfo,
  KeyInfo,
  Operator,
  Org,
  User,
} from "./types";

/**
 * "12 keys · 2 inactive".
 *
 * `noun` is singular; it is pluralised by adding "s". `attention.label` is
 * written already-plural by the caller because these are adjectives
 * ("inactive", "held"), not nouns. The attention clause is omitted entirely
 * when its count is zero — a quiet panel should read quiet.
 */
export function countLine(
  total: number,
  noun: string,
  attention?: { count: number; label: string },
): string {
  const head = `${total} ${total === 1 ? noun : `${noun}s`}`;
  if (!attention || attention.count === 0) return head;
  return `${head} · ${attention.count} ${attention.label}`;
}

/**
 * "12 keys · 2 over budget".
 *
 * A key with no budget set (0) is unlimited, not overspent, so it is
 * excluded — confirmed against the gateway's own store semantics (see
 * gateway/internal/store/memory.go and store.go: "A budget of 0 means
 * unlimited, matching the rest of the gateway"). Budget exhaustion is what
 * an operator needs to see on this page: it is the state that makes a key
 * start refusing traffic.
 */
export function keysSummary(keys: readonly KeyInfo[]): string {
  const over = keys.filter((k) => k.monthly_budget_usd > 0 && k.spend_usd >= k.monthly_budget_usd).length;
  return countLine(keys.length, "key", { count: over, label: "over budget" });
}

/** "3 documents" — DocumentInfo (name + chunk count) has no signal worth surfacing. */
export function documentsSummary(docs: readonly DocumentInfo[]): string {
  return countLine(docs.length, "document");
}

/**
 * "4 operators" — Operator (GET /operators) carries enabled/disabled, not a
 * "held" state; a held/needs-approval run lives on OperatorRun, a different
 * array than the one this wrapper receives. Plain count, no invented field.
 */
export function operatorsSummary(operators: readonly Operator[]): string {
  return countLine(operators.length, "operator");
}

/**
 * "5 orgs · 1 over budget" — an org is over budget once aggregate spend is
 * at or above its monthly budget: the same `committed + estimate > budget`
 * shape the gateway checks before admitting the org's next request (see
 * gateway/internal/store/memory.go), so an org sitting exactly at budget
 * has its next non-free request rejected too. A budget of 0 means unlimited
 * (same convention as keysSummary and budgetFraction in lib/format.ts), so
 * it is excluded.
 */
export function orgsSummary(orgs: readonly Org[]): string {
  const over = orgs.filter((o) => o.monthly_budget_usd > 0 && o.spend_usd >= o.monthly_budget_usd).length;
  return countLine(orgs.length, "org", { count: over, label: "over budget" });
}

/**
 * "6 members" — User does carry `active`, but this panel is specified as a
 * plain count; surfacing deactivated members is a decision for a future
 * cycle, not something to add unasked.
 */
export function usersSummary(users: readonly User[]): string {
  return countLine(users.length, "member");
}

/**
 * "6 objectives · 2 with pending writes".
 *
 * A pending proposal is a write the council wants to make and a human has
 * not yet approved — the one thing on this page that is actually waiting on
 * the operator. `objectives` and `proposals` are the two sibling arrays
 * Multiverse.tsx already fetches at page top level (neither gated on the
 * selected objective, unlike the per-objective cycle detail), so this needs
 * no new fetch. "Pending" is the literal `CouncilProposal.status` value the
 * page itself checks before showing an Approve button (Multiverse.tsx:314,
 * `p.status === "pending"`), not a signal invented here. Counted by distinct
 * objective, not by proposal: three pending writes against one objective is
 * one objective needing attention.
 */
export function objectivesSummary(
  objectives: readonly CouncilObjective[],
  proposals: readonly CouncilProposal[],
): string {
  const pendingObjectiveIds = new Set(
    proposals.filter((p) => p.status === "pending").map((p) => p.objective_id),
  );
  const withPending = objectives.filter((o) => pendingObjectiveIds.has(o.id)).length;
  return countLine(objectives.length, "objective", { count: withPending, label: "with pending writes" });
}

/**
 * "100 events · 3 denied" — AuditEntry.kind === "guardrail_block" is the
 * wire signal for a blocked call, the same value the Audit page already
 * badges as a guardrail block.
 */
export function auditEventsSummary(events: readonly AuditEntry[]): string {
  const denied = events.filter((e) => e.kind === "guardrail_block").length;
  return countLine(events.length, "event", { count: denied, label: "denied" });
}

/**
 * The slice of a budget row this summary needs — just the raw spend/budget
 * numbers, not the [0,1]-clamped fraction. Declared structurally so lib/
 * never has to import from pages/: Overview's `BudgetMeter` (pages/
 * overviewFeed.ts) satisfies this shape as-is, so its call site needs no
 * change and no cast.
 */
interface BudgetLike {
  spend: number;
  budget: number;
}

/**
 * "6 budgets · 1 over limit" — the array the Overview "Budgets" panel
 * already holds (per-key budget rows built by budgetMeters() from the same
 * KeyInfo records keysSummary reads). "Over" is at-or-above the limit, the
 * same boundary keysSummary and orgsSummary use, so a key sitting exactly
 * at its budget reads as over on every panel that shows it rather than
 * flagged on one and silent on another. Compares the raw spend/budget
 * numbers directly rather than trusting the clamped fraction, which
 * saturates at 1 and can't tell "at" from "over."
 */
export function budgetsSummary(meters: readonly BudgetLike[]): string {
  const over = meters.filter((m) => m.spend >= m.budget).length;
  return countLine(meters.length, "budget", { count: over, label: "over limit" });
}
