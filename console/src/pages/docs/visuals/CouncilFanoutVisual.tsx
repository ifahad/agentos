import { COUNCIL_LAYOUT, COUNCIL_MEMBERS } from "./geometry";
import { Illustration } from "./Illustration";

const { objective, member, judge, verdict, dissent } = COUNCIL_LAYOUT;

const OBJECTIVE_RIGHT = objective.x + objective.w;
const OBJECTIVE_MID_Y = objective.y + objective.h / 2;
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
 * Control-point offsets: half the run in every case, so each curve is a
 * symmetric S that leaves its box horizontally and arrives horizontally.
 */
const FAN_OUT_BEND = (member.x - OBJECTIVE_RIGHT) / 2;
const FAN_IN_BEND = (judge.x - MEMBER_RIGHT) / 2;
const OUT_BEND = (verdict.x - JUDGE_RIGHT) / 2;

/** What the judge emits. Two boxes, and the second one is the point of the visual. */
const OUTPUTS = [
  { label: "verdict", box: verdict },
  { label: "dissent", box: dissent },
];

/**
 * Many models, one verdict.
 *
 * Every box and every curve endpoint comes from COUNCIL_LAYOUT rather than from
 * JSX literals: the layout derives `member.x` from the objective box's right
 * edge plus COUNCIL_FANOUT_GAP, so the fan-out's origin and its landing points
 * cannot drift apart. One line per member leaves the objective and one returns
 * to the judge, and both live inside that member's own group so the line
 * reveals with the box it belongs to rather than appearing whole while the
 * boxes are still staggering in.
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

        {COUNCIL_MEMBERS.map((m, i) => {
          const midY = memberMidY(m.y);
          return (
            <g key={m.id} className="docs-hop" style={{ animationDelay: `${i * 60}ms` }}>
              {/* The fan-out: one branch per member, off the objective's right
                  edge. It reveals with its own member's box. */}
              <path
                className="docs-edge"
                d={`M${OBJECTIVE_RIGHT} ${OBJECTIVE_MID_Y}C${
                  OBJECTIVE_RIGHT + FAN_OUT_BEND
                } ${OBJECTIVE_MID_Y} ${member.x - FAN_OUT_BEND} ${midY} ${member.x} ${midY}`}
              />
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
                d={`M${MEMBER_RIGHT} ${midY}C${MEMBER_RIGHT + FAN_IN_BEND} ${midY} ${
                  judge.x - FAN_IN_BEND
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
