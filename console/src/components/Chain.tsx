import { useState } from "react";
import type { ChainStage, ChainState } from "../lib/chain";
import { CHAIN_STAGES, stageRenders } from "../lib/chain";
import type { ChainStyle } from "../lib/chainStyle";
import { StateIcon } from "../ui/icons";
import { ChainCanvas } from "./chain/ChainCanvas";
import { useChainPackets } from "./chain/useChainPackets";
import "./Chain.css";

/** Canvas height in the always-on topbar strip, and when expanded. */
const STRIP_HEIGHT = 76;
const EXPANDED_HEIGHT = 220;

const STAGE_LABELS: Record<ChainStage, string> = {
  auth: "auth",
  rate: "rate",
  budget: "budget",
  guardrail: "guardrail",
  upstream: "upstream",
  audit: "audit",
};

const STAGE_TITLES: Record<ChainStage, string> = {
  auth: "auth — the caller presented a valid virtual key",
  rate: "rate — the key's org is within its per-org rate limit",
  budget: "budget — the key and its org are within budget",
  guardrail: "guardrail — not halted by injection screening (a lit stage is not proof it ran)",
  upstream: "upstream — the provider or council answered",
  audit: "audit — the outcome was recorded",
};

function outcomeGlyph(state: ChainState, active: boolean) {
  if (active) return { state: "live" as const, label: "request in flight" };
  if (state.outcome === "deny") {
    return { state: "deny" as const, label: `denied at ${state.stoppedAt}` };
  }
  if (state.outcome === "fail") {
    return { state: "hold" as const, label: "cleared governance, provider failed" };
  }
  return { state: "ok" as const, label: "cleared" };
}

interface ChainProps {
  adminKey: string;
  style: ChainStyle;
}

/**
 * The governance chain — the console's signature element.
 *
 * Every AgentOS request runs an ordered gauntlet before a provider is ever
 * called, and that ordering is the whole product, so the shell renders it
 * permanently.
 *
 * The visible treatment is now one of four styles (see lib/chainStyle.ts), but
 * what any of them may draw is unchanged: it is driven by recorded audit
 * evidence (lib/chain.ts), never by an animation timer. A lit stage is
 * evidence. With no key, or no traffic, the chain rests — deliberately,
 * because "quiet" and "healthy" must not look the same as "unknown".
 *
 * The DOM stage list below is the accessible representation and is rendered
 * for every style, including the canvas ones, where it is visually hidden but
 * still read. The canvas is decoration over it, never a replacement.
 */
export function Chain({ adminKey, style }: ChainProps) {
  const { packets, state, active } = useChainPackets(adminKey);
  const [expanded, setExpanded] = useState(false);

  const renders = stageRenders(state);
  const glyph = outcomeGlyph(state, active);
  const progress = state.cleared / CHAIN_STAGES.length;
  const animated = style !== "minimal";

  return (
    <div
      className="chain"
      data-outcome={state.outcome}
      data-active={active || undefined}
      data-style={style}
      data-expanded={expanded || undefined}
    >
      {animated && (
        <ChainCanvas
          style={style}
          packets={packets}
          state={state}
          height={expanded ? EXPANDED_HEIGHT : STRIP_HEIGHT}
        />
      )}

      {!animated && (
        <div className="chain-track" aria-hidden>
          <div className="chain-rule" />
          <div className="chain-fill" style={{ transform: `scaleX(${progress})` }} />
        </div>
      )}

      {/* The accessible chain. Visually hidden under a canvas style — the
          canvas draws the same six stages — but never removed, so screen
          readers get the identical reading in every style. */}
      <ol className="chain-stages" data-hidden={animated || undefined}>
        {CHAIN_STAGES.map((stage, i) => (
          <li key={stage} className="chain-stage" data-render={renders[i]} title={STAGE_TITLES[stage]}>
            <span className="chain-node" aria-hidden />
            <span className="chain-label">{STAGE_LABELS[stage]}</span>
          </li>
        ))}
      </ol>

      {/* A denial is the most important sentence the console speaks — announce it. */}
      <div className="chain-outcome" aria-live="polite">
        <StateIcon state={glyph.state} title={glyph.label} size={12} />
      </div>

      {animated && (
        <button
          type="button"
          className="chain-expand"
          aria-expanded={expanded}
          aria-label={expanded ? "Collapse governance chain" : "Expand governance chain"}
          onClick={() => setExpanded((v) => !v)}
        >
          <span aria-hidden>{expanded ? "▲" : "▼"}</span>
        </button>
      )}
    </div>
  );
}
