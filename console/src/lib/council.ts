// Multiverse council API (Task 14). Request builders target the runtime through
// the same-origin /api/runtime proxy, so nginx injects the runtime token
// server-side and the browser never holds it — same discipline as the rest of
// the console. Pure verdict helpers live here too, unit-tested without a DOM.

import { RUNTIME_BASE, buildRequest, type RequestSpec } from "./api";
import type { CouncilMemberRun } from "./types";

export function listMembersRequest(): RequestSpec {
  return buildRequest(RUNTIME_BASE, "/council/members");
}

export function listObjectivesRequest(): RequestSpec {
  return buildRequest(RUNTIME_BASE, "/council/objectives");
}

export function getObjectiveRequest(id: string): RequestSpec {
  return buildRequest(RUNTIME_BASE, `/council/objectives/${encodeURIComponent(id)}`);
}

export function createObjectiveRequest(input: string): RequestSpec {
  return buildRequest(RUNTIME_BASE, "/council/objectives", { body: { input } });
}

export function cancelObjectiveRequest(id: string): RequestSpec {
  return buildRequest(RUNTIME_BASE, `/council/objectives/${encodeURIComponent(id)}/cancel`, {
    method: "POST",
  });
}

// Pause and resume are distinct endpoints, so the kill switch reads as an
// explicit verb in the audit trail rather than a toggled flag.
export function pauseRequest(paused: boolean): RequestSpec {
  return buildRequest(RUNTIME_BASE, paused ? "/council/pause" : "/council/resume", {
    method: "POST",
  });
}

export function listProposalsRequest(): RequestSpec {
  return buildRequest(RUNTIME_BASE, "/council/proposals");
}

export function approveProposalRequest(id: string): RequestSpec {
  return buildRequest(RUNTIME_BASE, `/council/proposals/${encodeURIComponent(id)}/approve`, {
    method: "POST",
  });
}

// Agreement bands. The council reports a fraction of concurring members; these
// labels are what an operator scans for in the timeline.
export function agreementLabel(agreement: number): string {
  const value = Math.max(0, Math.min(1, agreement));
  if (value >= 1) return "unanimous";
  if (value >= 0.75) return "strong";
  if (value >= 0.5) return "majority";
  return "split";
}

// A member that timed out may still succeed next cycle; one that failed hit a
// hard error. Both are non-fatal — quorum decides the cycle.
export function memberStatusTone(status: string): "ok" | "warn" | "error" {
  if (status === "answered") return "ok";
  if (status === "failed") return "error";
  return "warn";
}

// Answered vs not, for a cycle's member runs. "failed" here means "did not
// answer" — a timeout counts against the answered tally too.
export function summarizeMembers(runs: CouncilMemberRun[]): { answered: number; failed: number } {
  let answered = 0;
  for (const run of runs) {
    if (run.status === "answered") answered += 1;
  }
  return { answered, failed: runs.length - answered };
}
