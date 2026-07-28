import {
  LIFECYCLE_DETAIL_Y,
  LIFECYCLE_HOPS,
  LIFECYCLE_LABEL_Y,
  LIFECYCLE_RULE_Y,
} from "./geometry";
import { Illustration } from "./Illustration";
import { monoCharsInWidth, wrapMono } from "./text";

const VIEW_W = 640;
const DETAIL_SIZE = 10;
const DETAIL_LEAD = 12;

const FIRST_HOP = LIFECYCLE_HOPS[0];
const LAST_HOP = LIFECYCLE_HOPS[LIFECYCLE_HOPS.length - 1];

/**
 * Line budget for each hop's centred detail column, computed from that hop's
 * own neighbours rather than from one global minimum.
 *
 * A column may run to the edge of the viewBox, but only halfway to the next
 * hop — both neighbours apply the same rule, so halves cannot collide. Taking
 * a single minimum across all four hops instead would let the first hop's
 * tight left clearance (112 units) govern the middle two, which have 160 and
 * 176, and that orphaned "key" on a line of its own.
 */
const DETAIL_CHARS = LIFECYCLE_HOPS.map((hop, i) => {
  const left = i === 0 ? hop.x : (hop.x - LIFECYCLE_HOPS[i - 1].x) / 2;
  const right =
    i === LIFECYCLE_HOPS.length - 1 ? VIEW_W - hop.x : (LIFECYCLE_HOPS[i + 1].x - hop.x) / 2;
  return monoCharsInWidth(2 * Math.min(left, right), DETAIL_SIZE);
});

/**
 * One governed request, hop by hop.
 *
 * Four hops on a rule, each with a callout naming what it clears. The callouts
 * carry the corrected governance facts — which refusals are recorded and which
 * are not — because those are the claims a reader is most likely to get wrong.
 *
 * Reveal is the same CSS keyframe + inline animationDelay stagger the
 * architecture visual uses; nothing here animates on a loop.
 */
export function RequestLifecycleVisual() {
  return (
    <Illustration diagram="requestLifecycle">
      <svg
        className="docs-svg"
        viewBox="0 0 640 190"
        role="img"
        aria-label="A governed request: the client presents a virtual key, the gateway authorises, meters, screens and records it, the runtime runs the agent without holding a provider key, and a tool call is constrained inside the connector or sandbox."
      >
        <path
          className="docs-edge"
          d={`M${FIRST_HOP.x} ${LIFECYCLE_RULE_Y}H${LAST_HOP.x}`}
        />

        {LIFECYCLE_HOPS.map((hop, i) => (
          <g key={hop.id} className="docs-hop" style={{ animationDelay: `${i * 80}ms` }}>
            <circle className="docs-hop-node" cx={hop.x} cy={LIFECYCLE_RULE_Y} r="6" />
            <text
              className="docs-node-label"
              x={hop.x}
              y={LIFECYCLE_LABEL_Y}
              textAnchor="middle"
            >
              {hop.label}
            </text>
            {wrapMono(hop.detail, DETAIL_CHARS[i]).map((line, li) => (
              <text
                key={`${hop.id}-${li}`}
                className="docs-node-sub"
                x={hop.x}
                y={LIFECYCLE_DETAIL_Y + li * DETAIL_LEAD}
                textAnchor="middle"
              >
                {line}
              </text>
            ))}
          </g>
        ))}
      </svg>

      <ul className="docs-callouts">
        <li>
          Denials are not all recorded. Among refusals only rate-limit rejections and guardrail
          events reach the audit log — a 401, a 400, and a 402 budget exhaustion do not.
        </li>
        <li>
          The budget hold fails open only on a store error, and unlike the guardrail&apos;s
          <span className="mono"> guardrail_error</span> that admission leaves no audit entry.
        </li>
        <li>
          The guardrail stage exists only when <span className="mono">AGENTOS_GUARDRAILS_MODE</span>{" "}
          is not <span className="mono">off</span>; <span className="mono">/v1/embeddings</span> runs
          the same chain without it.
        </li>
      </ul>
    </Illustration>
  );
}
