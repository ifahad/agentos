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
  // Drops from the console's right edge to the same x=152 channel the client
  // edge uses, but lands on gateway's left edge higher up (y=90 vs the
  // client's y=108) so the two vertical runs (y 46-90 and y 108-154) never
  // overlap or cross.
  { from: "console", to: "gateway", d: "M120 46H152V90H184" },
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

/**
 * Hops sit on a single rule at LIFECYCLE_RULE_Y; the hop label sits above it at
 * LIFECYCLE_LABEL_Y and the detail text sits below it at LIFECYCLE_DETAIL_Y.
 * Exported so the component reads these instead of re-hardcoding 70/46/102 as
 * JSX literals.
 *
 * LIFECYCLE_VIEW_H is here for the same reason, and is not a free number: the
 * drawing ends at the descender of the second detail line (baseline
 * LIFECYCLE_DETAIL_Y + LIFECYCLE_DETAIL_LEAD = 114), and the tallest glyph
 * above it starts a little under LIFECYCLE_LABEL_Y. At the 190 this was first
 * authored at, 73 of the 190 units — 38% of the frame, ~80px on a 1280px
 * viewport — were empty below the detail text, so the illustration sat in the
 * top two-thirds of its own panel and read as if something had failed to draw.
 * 150 leaves matched bands above and below, and still clears a third detail
 * line (descender ~129) if a string ever grows.
 */
export const LIFECYCLE_RULE_Y = 70;
export const LIFECYCLE_LABEL_Y = 46;
export const LIFECYCLE_DETAIL_Y = 102;
export const LIFECYCLE_DETAIL_LEAD = 12;
export const LIFECYCLE_VIEW_H = 150;

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

/**
 * viewBox is 0 0 640 280 for the council visual. (Raised from 260: the last
 * member box, at y=232 with COUNCIL_LAYOUT.member.h=32, reaches 264 — the
 * spacing between members is kept as originally authored, so the viewBox
 * grew to fit it instead.)
 *
 * Members fan out from x=240 to a judge at x=430. That x=240 is not a
 * standalone magic number — see COUNCIL_LAYOUT.member.x below, which is the
 * objective box's own right edge plus COUNCIL_FANOUT_GAP, so the origin and
 * the objective can never drift apart again.
 */
export const COUNCIL_MEMBERS: CouncilMember[] = [
  { id: "m1", label: "model A", y: 40 },
  { id: "m2", label: "model B", y: 88 },
  { id: "m3", label: "model C", y: 136 },
  { id: "m4", label: "model D", y: 184 },
  { id: "m5", label: "model E", y: 232 },
];

export interface CouncilBox {
  x: number;
  y: number;
  w: number;
  h: number;
}

/** Every member box shares these dimensions; only `y` (see COUNCIL_MEMBERS) varies. */
export interface CouncilMemberBox {
  x: number;
  w: number;
  h: number;
}

export interface CouncilLayout {
  /** The prompt/ask box the fan-out originates from. */
  objective: CouncilBox;
  /** Shared x/width/height for every member box; combine with a member's own `y`. */
  member: CouncilMemberBox;
  /** The single judge box every member's line fans back into. */
  judge: CouncilBox;
  /** The majority verdict, drawn once the judge resolves. */
  verdict: CouncilBox;
  /** The dissent note — the council is only interesting if not every model agrees. */
  dissent: CouncilBox;
}

const COUNCIL_OBJECTIVE_BOX: CouncilBox = { x: 16, y: 108, w: 134, h: 64 };

/**
 * Horizontal run between the objective's right edge and the member column —
 * the space the fan-out itself is drawn in.
 *
 * It must not be zero. With the member column starting exactly at the
 * objective's right edge, m3's whole left border was the objective's right
 * border and m2 shared twelve more units of it: three rects fused into one
 * glyph, and the left half of a picture captioned "fans out to five
 * model-bound members" showed no branching at all. 90 units gives every member
 * a visible curve of its own and still leaves the column clear of the judge
 * (240 + 110 = 350 <= 430).
 */
export const COUNCIL_FANOUT_GAP = 90;

/**
 * So the renderer never hardcodes the objective box, the member box, the
 * judge, or the verdict/dissent boxes as JSX literals. `member.x` is derived
 * from the objective box's right edge plus the fan-out gap, not duplicated as
 * a bare 240.
 */
export const COUNCIL_LAYOUT: CouncilLayout = {
  objective: COUNCIL_OBJECTIVE_BOX,
  member: {
    x: COUNCIL_OBJECTIVE_BOX.x + COUNCIL_OBJECTIVE_BOX.w + COUNCIL_FANOUT_GAP,
    w: 110,
    h: 32,
  },
  judge: { x: 430, y: 108, w: 100, h: 64 },
  verdict: { x: 560, y: 100, w: 64, h: 44 },
  dissent: { x: 560, y: 156, w: 64, h: 44 },
};

/* ---------- governance conveyor ---------- */

/**
 * A fixed script. No Math.random anywhere: the sequence is authored so a reader
 * sees a clean pass, a rate-limit denial, a guardrail block, an upstream
 * failure, and a budget denial without waiting on chance, and so two people
 * looking at the same frame see the same thing.
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
