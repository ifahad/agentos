import { describe, expect, it } from "vitest";
import { CHAIN_STAGES } from "../../../lib/chain";
import {
  ARCH_EDGES,
  ARCH_NODES,
  COUNCIL_FANOUT_GAP,
  COUNCIL_LAYOUT,
  COUNCIL_MEMBERS,
  GOVERNANCE_FRAME_MS,
  GOVERNANCE_SCRIPT,
  LIFECYCLE_DETAIL_LEAD,
  LIFECYCLE_DETAIL_Y,
  LIFECYCLE_HOPS,
  LIFECYCLE_LABEL_Y,
  LIFECYCLE_RULE_Y,
  LIFECYCLE_VIEW_H,
  clearedCount,
} from "./geometry";

interface Point {
  x: number;
  y: number;
}

interface Box {
  x: number;
  y: number;
  w: number;
  h: number;
}

/**
 * Walks a `d` string built only from M/H/V/L commands (the only commands any
 * path in this module uses) and returns the point the pen started at and the
 * point it ended at. Good enough for these axis-aligned edges without pulling
 * in an SVG path-parsing dependency.
 */
function pathEndpoints(d: string): { start: Point; end: Point } {
  const tokens = d.match(/[MHVL][^MHVL]*/g) ?? [];
  let cx = 0;
  let cy = 0;
  let start: Point | null = null;
  for (const token of tokens) {
    const cmd = token[0];
    const nums = token
      .slice(1)
      .trim()
      .split(/[\s,]+/)
      .filter((s) => s.length > 0)
      .map(Number);
    if (cmd === "M" || cmd === "L") {
      [cx, cy] = nums;
    } else if (cmd === "H") {
      [cx] = nums;
    } else if (cmd === "V") {
      [cy] = nums;
    }
    if (start === null) start = { x: cx, y: cy };
  }
  return { start: start ?? { x: cx, y: cy }, end: { x: cx, y: cy } };
}

/** Every edge in this module is axis-aligned and snaps exactly onto its node's
 * boundary, so an exact perimeter match is practical here (no bounding-box
 * fallback needed). */
function onPerimeter(box: Box, p: Point): boolean {
  const withinX = p.x >= box.x && p.x <= box.x + box.w;
  const withinY = p.y >= box.y && p.y <= box.y + box.h;
  const onVerticalEdge = (p.x === box.x || p.x === box.x + box.w) && withinY;
  const onHorizontalEdge = (p.y === box.y || p.y === box.y + box.h) && withinX;
  return onVerticalEdge || onHorizontalEdge;
}

describe("architecture geometry", () => {
  it("has unique node ids", () => {
    const ids = ARCH_NODES.map((n) => n.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it("only draws edges between declared nodes", () => {
    const ids = new Set(ARCH_NODES.map((n) => n.id));
    for (const e of ARCH_EDGES) {
      expect(ids.has(e.from)).toBe(true);
      expect(ids.has(e.to)).toBe(true);
    }
  });

  it("keeps every node inside the 640x320 viewBox", () => {
    for (const n of ARCH_NODES) {
      expect(n.x).toBeGreaterThanOrEqual(0);
      expect(n.y).toBeGreaterThanOrEqual(0);
      expect(n.x + n.w).toBeLessThanOrEqual(640);
      expect(n.y + n.h).toBeLessThanOrEqual(320);
    }
  });

  it("states the invariant on the runtime-to-gateway edge", () => {
    const edge = ARCH_EDGES.find((e) => e.from === "runtime" && e.to === "gateway");
    expect(edge?.note).toMatch(/only via the gateway/i);
  });

  it("lands every edge's start on its `from` node and its end on its `to` node", () => {
    const byId = new Map(ARCH_NODES.map((n) => [n.id, n]));
    for (const e of ARCH_EDGES) {
      const fromNode = byId.get(e.from);
      const toNode = byId.get(e.to);
      expect(fromNode).toBeDefined();
      expect(toNode).toBeDefined();
      if (!fromNode || !toNode) continue;
      const { start, end } = pathEndpoints(e.d);
      expect(onPerimeter(fromNode, start)).toBe(true);
      expect(onPerimeter(toNode, end)).toBe(true);
    }
  });
});

describe("governance script", () => {
  it("only halts at stages the chain actually has", () => {
    for (const f of GOVERNANCE_SCRIPT) {
      if (f.stoppedAt !== null) expect(CHAIN_STAGES).toContain(f.stoppedAt);
    }
  });

  it("never halts at rbac, which is not on the model-call path", () => {
    for (const f of GOVERNANCE_SCRIPT) expect(f.stoppedAt).not.toBe("rbac");
  });

  it("shows a clean pass, a denial, and an upstream failure", () => {
    const outcomes = new Set(GOVERNANCE_SCRIPT.map((f) => f.outcome));
    expect(outcomes).toContain("pass");
    expect(outcomes).toContain("deny");
    expect(outcomes).toContain("fail");
  });

  it("gives every frame a caption", () => {
    for (const f of GOVERNANCE_SCRIPT) expect(f.caption.trim().length).toBeGreaterThan(0);
  });

  it("holds a frame long enough to read", () => {
    expect(GOVERNANCE_FRAME_MS).toBeGreaterThanOrEqual(1500);
  });

  it("clears every stage on a pass and stops short on a denial", () => {
    expect(clearedCount({ stoppedAt: null, outcome: "pass", caption: "" })).toBe(
      CHAIN_STAGES.length,
    );
    expect(clearedCount({ stoppedAt: "rate", outcome: "deny", caption: "" })).toBe(
      CHAIN_STAGES.indexOf("rate"),
    );
  });
});

describe("other visuals", () => {
  it("orders the lifecycle hops left to right", () => {
    const xs = LIFECYCLE_HOPS.map((h) => h.x);
    expect([...xs].sort((a, b) => a - b)).toEqual(xs);
  });

  it("gives the council more than one member, so 'dissent' means something", () => {
    expect(COUNCIL_MEMBERS.length).toBeGreaterThan(1);
  });
});

describe("lifecycle geometry", () => {
  it("orders the label above the rule and the detail text below it", () => {
    expect(LIFECYCLE_LABEL_Y).toBeLessThan(LIFECYCLE_RULE_Y);
    expect(LIFECYCLE_DETAIL_Y).toBeGreaterThan(LIFECYCLE_RULE_Y);
  });

  it("keeps the rule's label and detail text inside the viewBox", () => {
    expect(LIFECYCLE_LABEL_Y).toBeGreaterThanOrEqual(0);
    expect(LIFECYCLE_DETAIL_Y + LIFECYCLE_DETAIL_LEAD).toBeLessThanOrEqual(LIFECYCLE_VIEW_H);
  });

  /**
   * Fitting is not framing. The first authored height was 190, which fit
   * everything and still left 38% of the frame empty below the detail text —
   * the drawing sat in the top two-thirds of its panel and read as a failed
   * render. Nothing caught it, because every check asked only whether the
   * content was inside the box.
   *
   * The band below the deepest text row must not exceed the band above the
   * topmost one.
   */
  it("does not leave a dead band under the detail text", () => {
    const below = LIFECYCLE_VIEW_H - (LIFECYCLE_DETAIL_Y + LIFECYCLE_DETAIL_LEAD);
    expect(below).toBeLessThanOrEqual(LIFECYCLE_LABEL_Y);
  });
});

describe("council layout", () => {
  it("derives the member column from the objective box's right edge plus the gap, not a bare literal", () => {
    expect(COUNCIL_LAYOUT.member.x).toBe(
      COUNCIL_LAYOUT.objective.x + COUNCIL_LAYOUT.objective.w + COUNCIL_FANOUT_GAP,
    );
  });

  /**
   * The fan-out needs somewhere to be drawn. At a zero gap the member boxes'
   * left border IS the objective's right border, three rects fuse into one
   * glyph, and a diagram about five models branching off one ask shows no
   * branch at all on its left half.
   */
  it("leaves a real horizontal run for the fan-out", () => {
    expect(COUNCIL_FANOUT_GAP).toBeGreaterThan(0);
    expect(COUNCIL_LAYOUT.member.x).toBeGreaterThan(
      COUNCIL_LAYOUT.objective.x + COUNCIL_LAYOUT.objective.w,
    );
  });

  it("fits every member box inside the 640x280 viewBox", () => {
    for (const m of COUNCIL_MEMBERS) {
      expect(COUNCIL_LAYOUT.member.x).toBeGreaterThanOrEqual(0);
      expect(m.y).toBeGreaterThanOrEqual(0);
      expect(COUNCIL_LAYOUT.member.x + COUNCIL_LAYOUT.member.w).toBeLessThanOrEqual(640);
      expect(m.y + COUNCIL_LAYOUT.member.h).toBeLessThanOrEqual(280);
    }
  });

  it("fits the objective, judge, verdict, and dissent boxes inside the 640x280 viewBox", () => {
    const boxes = [
      COUNCIL_LAYOUT.objective,
      COUNCIL_LAYOUT.judge,
      COUNCIL_LAYOUT.verdict,
      COUNCIL_LAYOUT.dissent,
    ];
    for (const b of boxes) {
      expect(b.x).toBeGreaterThanOrEqual(0);
      expect(b.y).toBeGreaterThanOrEqual(0);
      expect(b.x + b.w).toBeLessThanOrEqual(640);
      expect(b.y + b.h).toBeLessThanOrEqual(280);
    }
  });

  it("keeps the judge clear of the member boxes, and verdict/dissent clear of the judge", () => {
    const memberRight = COUNCIL_LAYOUT.member.x + COUNCIL_LAYOUT.member.w;
    const judgeRight = COUNCIL_LAYOUT.judge.x + COUNCIL_LAYOUT.judge.w;
    expect(COUNCIL_LAYOUT.judge.x).toBeGreaterThanOrEqual(memberRight);
    expect(COUNCIL_LAYOUT.verdict.x).toBeGreaterThanOrEqual(judgeRight);
    expect(COUNCIL_LAYOUT.dissent.x).toBeGreaterThanOrEqual(judgeRight);
  });
});
