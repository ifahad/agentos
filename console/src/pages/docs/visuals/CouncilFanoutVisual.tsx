import { COUNCIL_LAYOUT, COUNCIL_MEMBERS } from "./geometry";
import { Illustration } from "./Illustration";

const { objective, member, judge, verdict, dissent } = COUNCIL_LAYOUT;

const JUDGE_MID_Y = judge.y + judge.h / 2;
const MEMBER_RIGHT = member.x + member.w;
const JUDGE_RIGHT = judge.x + judge.w;

/** Baseline that centres a line of `size` text in a box of height `h`. */
function centredBaseline(y: number, h: number, size: number): number {
  return y + h / 2 + size * 0.36;
}

/** Vertical centre of one member's box. */
function memberMidY(y: number): number {
  return y + member.h / 2;
}

/**
 * Horizontal control-point offset for the fan-in curves: 40% of the run, so
 * every member's line leaves its box horizontally and arrives at the judge
 * horizontally.
 */
const FAN_BEND = (judge.x - MEMBER_RIGHT) * 0.4;
const OUT_BEND = (verdict.x - JUDGE_RIGHT) / 2;

/** What the judge emits. Two boxes, and the second one is the point of the visual. */
const OUTPUTS = [
  { label: "verdict", box: verdict },
  { label: "dissent", box: dissent },
];

/**
 * Many models, one verdict.
 *
 * Every box, and the fan-out origin, comes from COUNCIL_LAYOUT rather than from
 * JSX literals — the layout derives `member.x` from the objective box's own
 * right edge, so the objective and the member column share a single vertical
 * rule. That rule is the fan-out: the objective hands the ask to a bus and each
 * member taps it, which is also why no member line can drift off the origin.
 *
 * Reveal is the shared docs-hop keyframe with an inline per-member
 * animationDelay. Nothing loops, and nothing here carries a state hue: a
 * council is not a machine state.
 */
export function CouncilFanoutVisual() {
  return (
    <Illustration diagram="councilFanout">
      <svg
        className="docs-svg"
        viewBox="0 0 640 280"
        role="img"
        aria-label={`One objective fans out to ${COUNCIL_MEMBERS.length} model-bound members; a judge synthesises a single verdict and an explicit dissent report.`}
      >
        <rect
          className="docs-node-box"
          x={objective.x}
          y={objective.y}
          width={objective.w}
          height={objective.h}
          rx="4"
        />
        <text
          className="docs-node-label"
          x={objective.x + objective.w / 2}
          y={centredBaseline(objective.y, objective.h, 12)}
          textAnchor="middle"
        >
          objective
        </text>

        {/* The fan-out spine: the objective's right edge and every member box's
            left edge sit on this line, so it reads as one run the members hang
            off rather than five lines of zero length. */}
        <path
          className="docs-edge"
          d={`M${member.x} ${memberMidY(COUNCIL_MEMBERS[0].y)}V${memberMidY(
            COUNCIL_MEMBERS[COUNCIL_MEMBERS.length - 1].y,
          )}`}
        />

        {COUNCIL_MEMBERS.map((m, i) => {
          const midY = memberMidY(m.y);
          return (
            <g key={m.id} className="docs-hop" style={{ animationDelay: `${i * 60}ms` }}>
              <rect
                className="docs-node-box"
                x={member.x}
                y={m.y}
                width={member.w}
                height={member.h}
                rx="4"
              />
              <text
                className="docs-node-label"
                x={member.x + member.w / 2}
                y={centredBaseline(m.y, member.h, 12)}
                textAnchor="middle"
              >
                {m.label}
              </text>
              <path
                className="docs-edge"
                d={`M${MEMBER_RIGHT} ${midY}C${MEMBER_RIGHT + FAN_BEND} ${midY} ${
                  judge.x - FAN_BEND
                } ${JUDGE_MID_Y} ${judge.x} ${JUDGE_MID_Y}`}
              />
            </g>
          );
        })}

        <rect
          className="docs-node-box"
          x={judge.x}
          y={judge.y}
          width={judge.w}
          height={judge.h}
          rx="4"
        />
        <text
          className="docs-node-label"
          x={judge.x + judge.w / 2}
          y={centredBaseline(judge.y, judge.h, 12)}
          textAnchor="middle"
        >
          judge
        </text>

        {OUTPUTS.map(({ label, box }) => {
          const midY = box.y + box.h / 2;
          return (
            <g key={label}>
              <path
                className="docs-edge"
                d={`M${JUDGE_RIGHT} ${JUDGE_MID_Y}C${JUDGE_RIGHT + OUT_BEND} ${JUDGE_MID_Y} ${
                  box.x - OUT_BEND
                } ${midY} ${box.x} ${midY}`}
              />
              <rect
                className="docs-node-box"
                x={box.x}
                y={box.y}
                width={box.w}
                height={box.h}
                rx="4"
              />
              <text
                className="docs-node-sub"
                x={box.x + box.w / 2}
                y={centredBaseline(box.y, box.h, 10)}
                textAnchor="middle"
              >
                {label}
              </text>
            </g>
          );
        })}
      </svg>

      <ul className="docs-callouts">
        <li>
          Disagreement is recorded, not averaged away — the dissent report ships beside the verdict.
        </li>
        <li>
          Write-class tool calls become human-approved proposals. That gating is enforced in-graph
          for <span className="mono">react</span>-profile members; <span className="mono">deep</span>
          -profile members are held to an explicit read-only tool allowlist instead, because
          deepagents exposes no interrupt point.
        </li>
      </ul>
    </Illustration>
  );
}
