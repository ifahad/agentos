import { CHAIN_STAGES } from "../../../lib/chain";
import type { ChainStage } from "../../../lib/chain";

/* ---------- architecture ---------- */

export interface ArchNode {
  id: string;
  label: string;
  sub: string;
  x: number;
  y: number;
  w: number;
  h: number;
}

/** viewBox is 0 0 640 320 for the architecture visual. */
export const ARCH_NODES: ArchNode[] = [
  { id: "console", label: "console", sub: ":3000", x: 16, y: 24, w: 104, h: 44 },
  { id: "client", label: "any OpenAI client", sub: "", x: 16, y: 132, w: 104, h: 44 },
  { id: "gateway", label: "gateway", sub: "Go · :8080", x: 184, y: 78, w: 128, h: 60 },
  { id: "providers", label: "providers", sub: "Anthropic · OpenAI · Ollama", x: 184, y: 232, w: 128, h: 52 },
  { id: "runtime", label: "runtime", sub: "Python · :8000", x: 376, y: 78, w: 128, h: 60 },
  { id: "sandbox", label: "sandbox", sub: "Rust · :8070 · no egress", x: 376, y: 232, w: 128, h: 52 },
  { id: "connectors", label: "connectors", sub: "MCP · :8090-8094", x: 540, y: 78, w: 84, h: 60 },
];

export interface ArchEdge {
  from: string;
  to: string;
  d: string;
  note?: string;
}

export const ARCH_EDGES: ArchEdge[] = [
  { from: "console", to: "gateway", d: "M120 46H184" },
  { from: "client", to: "gateway", d: "M120 154H152V108H184" },
  { from: "gateway", to: "providers", d: "M248 138V232", note: "the only egress to a model" },
  { from: "runtime", to: "gateway", d: "M376 108H312", note: "models only via the gateway" },
  { from: "runtime", to: "sandbox", d: "M440 138V232" },
  { from: "runtime", to: "connectors", d: "M504 108H540" },
];

/** One continuous path a request pulse traces: client → gateway → runtime → connectors → back. */
export const ARCH_PULSE_PATH = "M120 154H152V108H184H312H376H504H540";

/* ---------- request lifecycle ---------- */

export interface LifecycleHop {
  id: string;
  label: string;
  detail: string;
  x: number;
}

/** viewBox is 0 0 640 190. Hops sit on a single rule at y=70. */
export const LIFECYCLE_HOPS: LifecycleHop[] = [
  { id: "client", label: "client", detail: "presents an agos- virtual key", x: 56 },
  { id: "gateway", label: "gateway", detail: "authorises, meters, screens, records", x: 216 },
  { id: "runtime", label: "runtime", detail: "runs the agent; holds no provider key", x: 392 },
  { id: "tool", label: "tool", detail: "connector or sandbox, constrained in-process", x: 568 },
];

/* ---------- council fanout ---------- */

export interface CouncilMember {
  id: string;
  label: string;
  y: number;
}

/** viewBox is 0 0 640 260. Members fan out from x=150 to a judge at x=430. */
export const COUNCIL_MEMBERS: CouncilMember[] = [
  { id: "m1", label: "model A", y: 40 },
  { id: "m2", label: "model B", y: 88 },
  { id: "m3", label: "model C", y: 136 },
  { id: "m4", label: "model D", y: 184 },
  { id: "m5", label: "model E", y: 232 },
];

/* ---------- governance conveyor ---------- */

/**
 * A fixed script. No Math.random anywhere: the sequence is authored so a reader
 * sees a clean pass, a rate-limit denial, a guardrail block, and an upstream
 * failure without waiting on chance, and so two people looking at the same frame
 * see the same thing.
 *
 * `stoppedAt: null` means the request cleared every stage.
 */
export interface GovernanceFrame {
  stoppedAt: ChainStage | null;
  outcome: "pass" | "deny" | "fail";
  caption: string;
}

export const GOVERNANCE_SCRIPT: GovernanceFrame[] = [
  { stoppedAt: null, outcome: "pass", caption: "cleared every stage" },
  { stoppedAt: null, outcome: "pass", caption: "cleared every stage" },
  { stoppedAt: "rate", outcome: "deny", caption: "429 — over the org's rate limit" },
  { stoppedAt: null, outcome: "pass", caption: "cleared every stage" },
  { stoppedAt: "guardrail", outcome: "deny", caption: "400 — prompt flagged by the guardrail" },
  { stoppedAt: null, outcome: "pass", caption: "cleared every stage" },
  { stoppedAt: "upstream", outcome: "fail", caption: "502 — governance cleared, provider failed" },
  { stoppedAt: "budget", outcome: "deny", caption: "402 — key budget exhausted" },
];

/** Milliseconds a single frame is held. */
export const GOVERNANCE_FRAME_MS = 2200;

/** Stages cleared before the halting stage, for a given frame. */
export function clearedCount(frame: GovernanceFrame): number {
  if (frame.stoppedAt === null) return CHAIN_STAGES.length;
  return CHAIN_STAGES.indexOf(frame.stoppedAt);
}
