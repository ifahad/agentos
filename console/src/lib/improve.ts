// Pure helpers for the Improve page (eval-gated self-improvement loop).

import { ApiError } from "./api";
import type { ProposalStatus } from "./types";

/** Eval score fraction (0..1, clamped) → whole-number percentage; "–" when not finite. */
export function formatScore(score: number): string {
  if (!Number.isFinite(score)) return "–";
  const pct = Math.round(Math.min(1, Math.max(0, score)) * 100);
  return `${pct}%`;
}

/** Candidate-vs-baseline delta in percentage points: "+25 pts", "−25 pts", "±0 pts". */
export function scoreDelta(baseline: number, candidate: number): string {
  if (!Number.isFinite(baseline) || !Number.isFinite(candidate)) return "";
  const d = Math.round((candidate - baseline) * 100);
  if (d === 0) return "±0 pts";
  return `${d > 0 ? "+" : "−"}${Math.abs(d)} pts`;
}

export interface BadgeSpec {
  className: string;
  label: string;
}

/** Proposal status → badge CSS class + human-readable label. */
export function proposalBadge(status: string): BadgeSpec {
  switch (status as ProposalStatus) {
    case "passed_evals":
      return { className: "badge passed_evals", label: "passed evals" };
    case "failed_evals":
      return { className: "badge failed_evals", label: "failed evals" };
    case "approved":
      return { className: "badge approved", label: "approved" };
    case "denied":
      return { className: "badge denied", label: "denied" };
    default:
      return { className: "badge", label: status };
  }
}

/** A proposal still awaiting a human decision (Approve/Deny apply). */
export function isUndecided(status: string): boolean {
  return status === "passed_evals" || status === "failed_evals";
}

/** Active-prompt source → banner badge label ("default" or "proposal #3"). */
export function promptSourceLabel(source: string, proposalId: number | null | undefined): string {
  if (source === "proposal" && proposalId != null) return `proposal #${proposalId}`;
  return "default";
}

/**
 * The runtime answers 503 when self-improvement is disabled (no checkpoint
 * database configured) — an expected state, not an error.
 */
export function isSelfImprovementDisabled(err: unknown): boolean {
  return err instanceof ApiError && err.status === 503;
}
