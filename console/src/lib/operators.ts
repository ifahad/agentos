// Operators API (native autonomy). Request builders target the runtime through
// the same-origin /api/runtime proxy so nginx injects the runtime token
// server-side. Pure helpers (trigger summary, status tone) are unit-tested
// without a DOM.

import { RUNTIME_BASE, buildRequest, type RequestSpec } from "./api";
import type { OperatorTrigger } from "./types";

export function listOperatorsRequest(): RequestSpec {
  return buildRequest(RUNTIME_BASE, "/operators");
}

export function getOperatorRequest(id: string): RequestSpec {
  return buildRequest(RUNTIME_BASE, `/operators/${encodeURIComponent(id)}`);
}

export function createOperatorRequest(body: {
  name: string;
  goal: string;
  trigger: OperatorTrigger;
  max_cycles?: number;
}): RequestSpec {
  return buildRequest(RUNTIME_BASE, "/operators", { body });
}

export function runOperatorRequest(id: string): RequestSpec {
  return buildRequest(RUNTIME_BASE, `/operators/${encodeURIComponent(id)}/run`, {
    method: "POST",
  });
}

export function setEnabledRequest(id: string, enabled: boolean): RequestSpec {
  return buildRequest(RUNTIME_BASE, `/operators/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: { enabled },
  });
}

export function deleteOperatorRequest(id: string): RequestSpec {
  return buildRequest(RUNTIME_BASE, `/operators/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

// One-line human description of a trigger, for the operators list.
export function triggerSummary(trigger: OperatorTrigger): string {
  if (trigger.type === "interval") return `every ${trigger.interval_s}s`;
  if (trigger.type === "cron") return `cron ${trigger.cron}`;
  return "webhook";
}

// A run's status maps to a machine-state tone. needs_approval is a held state
// (a human is implicated), error is a failure, completed is a pass.
export function runStatusTone(status: string): "ok" | "hold" | "error" {
  if (status === "completed") return "ok";
  if (status === "needs_approval") return "hold";
  return "error";
}
