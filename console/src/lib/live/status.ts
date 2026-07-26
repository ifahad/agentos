/** Lifecycle of a single live resource, derived from timestamps (pure). */
export type ResourceStatus = "idle" | "loading" | "live" | "stale" | "error";

/** Data older than cadence * this reads as stale. */
export const STALE_FACTOR = 1.5;

export interface StatusInput {
  updatedAt: number | null; // ms epoch of last success, or null
  lastErrorAt: number | null; // ms epoch of last failure, or null
  now: number;
  cadence: number; // poll interval in ms
}

export function deriveStatus({ updatedAt, lastErrorAt, now, cadence }: StatusInput): ResourceStatus {
  if (updatedAt === null) return lastErrorAt === null ? "loading" : "error";
  const overdue = now - updatedAt > cadence * STALE_FACTOR;
  if (!overdue) return "live";
  // Overdue: an error more recent than the last success means we are actively failing.
  if (lastErrorAt !== null && lastErrorAt >= updatedAt) return "error";
  return "stale";
}

export type ConnectionState = "idle" | "live" | "stale" | "offline";

export function reduceConnection(statuses: ResourceStatus[]): ConnectionState {
  const active = statuses.filter((s) => s !== "idle");
  if (active.length === 0) return "idle";
  if (active.every((s) => s === "error")) return "offline";
  if (active.some((s) => s === "stale" || s === "error")) return "stale";
  return "live";
}
