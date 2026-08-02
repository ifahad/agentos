/**
 * Panel summary lines.
 *
 * Every function here is pure and takes data the page already holds — no
 * fetching, no React. That is deliberate: the console's vitest runs in a node
 * environment with no DOM, so logic that needs testing has to live outside
 * components. The summary line is the part worth testing; the boolean beside
 * it is not.
 */

import type { BudgetMeter } from "../pages/overviewFeed";
import type { AuditEntry, CouncilObjective, DocumentInfo, KeyInfo, Operator, Org, User } from "./types";

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
 * "12 keys" — KeyInfo (GET /admin/keys) carries only name/monthly_budget_usd
 * /spend_usd, no active/inactive flag. Rather than invent one, this is a
 * plain count with no attention clause.
 */
export function keysSummary(keys: readonly KeyInfo[]): string {
  return countLine(keys.length, "key");
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
 * "5 orgs · 1 over budget" — an org is over budget once aggregate spend
 * exceeds its monthly budget. A budget of 0 means unlimited (the same
 * convention budgetFraction uses in lib/format.ts), so it is excluded.
 */
export function orgsSummary(orgs: readonly Org[]): string {
  const over = orgs.filter((o) => o.monthly_budget_usd > 0 && o.spend_usd > o.monthly_budget_usd).length;
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
 * "2 objectives" — CouncilObjective (GET /council/objectives) has no field
 * naming held write actions; that lives on CouncilProposal, a separate array
 * this wrapper does not receive. Plain count, no invented field.
 */
export function objectivesSummary(objectives: readonly CouncilObjective[]): string {
  return countLine(objectives.length, "objective");
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
 * "6 budgets · 1 over limit" — BudgetMeter (Overview's per-key budget rows,
 * built by budgetMeters() in pages/overviewFeed.ts) is the array the
 * Overview "Budgets" panel already holds. It keeps spend/budget unclamped
 * alongside the [0,1]-clamped `fraction`, so "over" compares the raw
 * numbers directly rather than trusting a value that saturates at 1.
 */
export function budgetsSummary(meters: readonly BudgetMeter[]): string {
  const over = meters.filter((m) => m.spend > m.budget).length;
  return countLine(meters.length, "budget", { count: over, label: "over limit" });
}
