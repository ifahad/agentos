import { ARCH_EDGES, ARCH_NODES, ARCH_PULSE_PATH } from "./geometry";
import type { ArchNode } from "./geometry";
import { Illustration } from "./Illustration";
import { monoCharsInWidth, wrapMono } from "./text";

/** Inset from a node's border to its text, per side. */
const NODE_PAD = 16;
const LABEL_SIZE = 12;
const SUB_SIZE = 10;
const LABEL_LEAD = 15;
const SUB_LEAD = 12;

interface NodeLine {
  text: string;
  kind: "label" | "sub";
  y: number;
}

/**
 * The lines of one node's text, wrapped to the node's own width and centred as
 * a block in the node's own height.
 *
 * The nodes are not all the same size and their strings are not all the same
 * length, so fixed offsets (label at y+26, sub at y+42) would have pushed
 * "any OpenAI client" out through its box and across the client → gateway
 * edge, and "MCP · :8090-8094" past the right edge of the viewBox entirely.
 * Deriving both the wrap width and the baselines from `n` keeps every string
 * inside the box the geometry module drew for it.
 */
function nodeLines(n: ArchNode): NodeLine[] {
  const inner = n.w - 2 * NODE_PAD;
  const labels = wrapMono(n.label, monoCharsInWidth(inner, LABEL_SIZE));
  const subs = n.sub === "" ? [] : wrapMono(n.sub, monoCharsInWidth(inner, SUB_SIZE));
  const blockH = labels.length * LABEL_LEAD + subs.length * SUB_LEAD;
  const top = n.y + (n.h - blockH) / 2;
  return [
    ...labels.map((text, i) => ({
      text,
      kind: "label" as const,
      y: top + i * LABEL_LEAD + 11,
    })),
    ...subs.map((text, i) => ({
      text,
      kind: "sub" as const,
      y: top + labels.length * LABEL_LEAD + i * SUB_LEAD + 9,
    })),
  ];
}

/**
 * The four planes, drawn from ARCH_NODES / ARCH_EDGES.
 *
 * Reveal is CSS keyframes with an inline per-node animationDelay (the
 * SpendBreakdown precedent). The request pulse is a travelling dash on a
 * pathLength={1} route (the UsageChart precedent). No framer-motion elements
 * and no SMIL: neither exists anywhere in this codebase.
 */
export function ArchitectureVisual() {
  return (
    <Illustration diagram="architecture">
      <svg
        className="docs-svg"
        viewBox="0 0 640 320"
        role="img"
        aria-label="The four planes: a console and any OpenAI client reach the gateway, which is the only egress to a model provider; the runtime reaches models only through the gateway and calls connectors and the sandbox."
      >
        {ARCH_EDGES.map((e) => (
          <path key={`${e.from}-${e.to}`} className="docs-edge" d={e.d} />
        ))}

        {/* Drawn before the nodes so the dash disappears behind a plane it is
            passing through and reappears in the gap on the far side. */}
        <path className="docs-arch-pulse" pathLength={1} d={ARCH_PULSE_PATH} />

        {ARCH_NODES.map((n, i) => (
          <g key={n.id} className="docs-arch-node" style={{ animationDelay: `${i * 70}ms` }}>
            <rect className="docs-node-box" x={n.x} y={n.y} width={n.w} height={n.h} rx="4" />
            {nodeLines(n).map((line, li) => (
              <text
                key={`${line.kind}-${li}`}
                className={line.kind === "label" ? "docs-node-label" : "docs-node-sub"}
                x={n.x + n.w / 2}
                y={line.y}
                textAnchor="middle"
              >
                {line.text}
              </text>
            ))}
          </g>
        ))}

        {/* The two notes are placed by hand rather than mapped: each needs its
            own placement, and a .filter(e => e.note).map(...) would stack both
            strings at one point.

            The runtime → gateway note sits above the top row because the gap it
            annotates is 64 units wide and the string is 162; the gateway →
            providers note is set to the left of its own edge so that vertical
            rule does not strike through it. */}
        <text className="docs-edge-note" x="344" y="68" textAnchor="middle">
          {ARCH_EDGES.find((e) => e.from === "runtime" && e.to === "gateway")?.note}
        </text>
        <text className="docs-edge-note" x="240" y="190" textAnchor="end">
          {ARCH_EDGES.find((e) => e.from === "gateway" && e.to === "providers")?.note}
        </text>
      </svg>
    </Illustration>
  );
}
