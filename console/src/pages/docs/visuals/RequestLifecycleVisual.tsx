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
 * Widest a centred detail column may be: it must not run off either end of the
 * viewBox, and two neighbouring columns must not collide. Every hop's detail is
 * longer than this (the shortest is 174 units of text), so all of them wrap.
 */
const DETAIL_W = Math.min(
  2 * FIRST_HOP.x,
  2 * (VIEW_W - LAST_HOP.x),
  ...LIFECYCLE_HOPS.slice(1).map((hop, i) => hop.x - LIFECYCLE_HOPS[i].x),
);

const DETAIL_CHARS = monoCharsInWidth(DETAIL_W, DETAIL_SIZE);

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
            {wrapMono(hop.detail, DETAIL_CHARS).map((line, li) => (
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
