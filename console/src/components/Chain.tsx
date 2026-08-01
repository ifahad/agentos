import { motion, useReducedMotion } from "framer-motion";
import { useEffect, useRef, useState } from "react";
import { useLiveResource } from "../hooks/useLiveResource";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
import type { ChainStage, ChainState } from "../lib/chain";
import { CHAIN_STAGES, IDLE_CHAIN, chainFeedCount, latestChainState, stageRenders } from "../lib/chain";
import type { AuditEntry } from "../lib/types";
import { EASE } from "../ui";
import { StateIcon } from "../ui/icons";
import "./Chain.css";

/** How long a newly observed request keeps the chain lit before it dims. */
const LINGER_MS = 6000;

/** Human-readable stage names. Short enough to sit on one line at 1024px. */
const STAGE_LABELS: Record<ChainStage, string> = {
  auth: "auth",
  rate: "rate",
  budget: "budget",
  guardrail: "guardrail",
  upstream: "upstream",
  audit: "audit",
};

/** What each stage actually checks — the tooltip for someone new to the gauntlet. */
const STAGE_TITLES: Record<ChainStage, string> = {
  auth: "auth — the caller presented a valid virtual key",
  rate: "rate — the key's org is within its per-org rate limit",
  budget: "budget — the key and its org are within budget",
  guardrail: "guardrail — not halted by injection screening (a lit stage is not proof it ran)",
  upstream: "upstream — the provider or council answered",
  audit: "audit — the outcome was recorded",
};

/** What the trailing status glyph should say about the last observed request. */
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
}

/**
 * The governance chain — the console's signature element.
 *
 * Every AgentOS request runs an ordered gauntlet before a provider is ever
 * called, and that ordering is the whole product. So the shell renders it
 * permanently: six stages, joined by a rule that fills as far as the most
 * recent request actually got.
 *
 * It is driven entirely by recorded audit evidence (see lib/chain.ts) rather
 * than by an animation timer, so a lit chain is evidence rather than decoration.
 * With no key, or no traffic, it sits unlit — deliberately, because "quiet" and
 * "healthy" must not look the same as "unknown".
 */
export function Chain({ adminKey }: ChainProps) {
  const reduced = useReducedMotion();
  // Same audit resource Overview subscribes to (identical key + cadence), so
  // the registry dedupes the two onto a single shared poll.
  const audit = useLiveResource<AuditEntry[]>(
    `admin/audit?limit=100#${adminKey}`,
    () => apiFetch<AuditEntry[]>(gatewayAdminRequest("/admin/audit?limit=100", adminKey)),
    { enabled: Boolean(adminKey), cadence: 5000 },
  );

  const [state, setState] = useState<ChainState>(IDLE_CHAIN);
  const [active, setActive] = useState(false);
  const seenCount = useRef<number | null>(null);
  const activeUntil = useRef(0);

  useEffect(() => {
    if (!adminKey) {
      setState(IDLE_CHAIN);
      setActive(false);
      seenCount.current = null;
      activeUntil.current = 0;
      return;
    }
    const entries = audit.data;
    // A failed (or still in-flight) poll tells us nothing about governance, so
    // the chain holds its last known reading rather than falsely reporting a
    // denial.
    if (!entries) return;
    // Growth in the feed means new traffic since the last poll; that is the
    // only thing that lights the chain. Count the same filtered set the chain
    // draws from — an admin-plane row is not traffic.
    const count = chainFeedCount(entries);
    if (seenCount.current !== null && count > seenCount.current) {
      activeUntil.current = Date.now() + LINGER_MS;
    }
    seenCount.current = count;
    setState(latestChainState(entries));
    setActive(Date.now() < activeUntil.current);
  }, [adminKey, audit.data]);

  const renders = stageRenders(state);
  const glyph = outcomeGlyph(state, active);
  // Fraction of the rule that should be inked, 0..1.
  const progress = state.cleared / CHAIN_STAGES.length;

  return (
    <div className="chain" data-outcome={state.outcome} data-active={active || undefined}>
      <div className="chain-track" aria-hidden>
        <div className="chain-rule" />
        {reduced ? (
          <div className="chain-fill" style={{ transform: `scaleX(${progress})` }} />
        ) : (
          <motion.div
            className="chain-fill"
            initial={false}
            animate={{ scaleX: progress }}
            /* Not transitionFast/transition: this is the chain filling to reflect
               real cleared-stage progress, deliberately slower (550ms) than any
               UI-chrome preset so the ink read as tracking evidence, not a blip. */
            transition={{ duration: 0.55, ease: EASE }}
          />
        )}
      </div>
      <ol className="chain-stages">
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
    </div>
  );
}
