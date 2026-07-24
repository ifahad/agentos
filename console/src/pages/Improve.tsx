import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { useState } from "react";
import type { PageProps } from "../App";
import { ErrorNotice, PageHead, errorMessage, useLoad } from "../components/common";
import { apiFetch, runtimeRequest } from "../lib/api";
import { formatTimestamp } from "../lib/format";
import {
  formatScore,
  isSelfImprovementDisabled,
  isUndecided,
  promptSourceLabel,
  proposalBadge,
  scoreDelta,
} from "../lib/improve";
import type {
  ActivePrompt,
  ApproveResponse,
  EvalCaseResult,
  EvalRunResult,
  EvalRunSummary,
  Proposal,
} from "../lib/types";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  Panel,
  PanelHead,
  Skeleton,
  Table,
  Tbody,
  Tr,
  fadeRise,
  fadeRiseReduced,
  staggerContainer,
  staggerItem,
  transitionFast,
  DUR_MED,
  EASE,
} from "../ui";
import type { BadgeVariant } from "../ui";
import "./Improve.css";

// Sentinel for "the runtime answered 503": self-improvement is off, which is a
// normal state (no checkpoint DB) rather than an error.
const DISABLED = "disabled";

async function orDisabled<T>(promise: Promise<T>): Promise<T | typeof DISABLED> {
  try {
    return await promise;
  } catch (err) {
    if (isSelfImprovementDisabled(err)) return DISABLED;
    throw err;
  }
}

/** lib/improve returns a CSS className ("badge approved"); map it to a Badge variant. */
function badgeVariantOf(className: string): BadgeVariant | undefined {
  const v = className.replace(/^badge\s*/, "");
  return v === "" ? undefined : (v as BadgeVariant);
}

export function Improve(_props: PageProps) {
  const active = useLoad(
    () => orDisabled(apiFetch<ActivePrompt>(runtimeRequest("/prompts/active"))),
    [],
  );
  const runs = useLoad(
    () => orDisabled(apiFetch<EvalRunSummary[]>(runtimeRequest("/evals/runs?limit=20"))),
    [],
  );
  const proposals = useLoad(
    () => orDisabled(apiFetch<Proposal[]>(runtimeRequest("/proposals?limit=20"))),
    [],
  );

  const [running, setRunning] = useState(false);
  const [lastRun, setLastRun] = useState<EvalRunResult | null>(null);
  const [runError, setRunError] = useState<string | null>(null);

  const [improving, setImproving] = useState(false);
  const [improveError, setImproveError] = useState<string | null>(null);

  const [deciding, setDeciding] = useState(false);
  const [decisionWarning, setDecisionWarning] = useState<string | null>(null);
  const [decisionError, setDecisionError] = useState<string | null>(null);

  const disabled =
    active.data === DISABLED || runs.data === DISABLED || proposals.data === DISABLED;

  const runEvals = async () => {
    setRunning(true);
    setRunError(null);
    try {
      const res = await apiFetch<EvalRunResult>(
        runtimeRequest("/evals/run", { suite: "default" }),
      );
      setLastRun(res);
      runs.reload();
    } catch (err) {
      setRunError(errorMessage(err));
    } finally {
      setRunning(false);
    }
  };

  const propose = async () => {
    setImproving(true);
    setImproveError(null);
    try {
      await apiFetch<Proposal>(runtimeRequest("/improve", {}));
      proposals.reload();
      runs.reload(); // /improve records baseline + candidate eval runs
    } catch (err) {
      setImproveError(errorMessage(err));
    } finally {
      setImproving(false);
    }
  };

  const decide = async (proposal: Proposal, approve: boolean) => {
    if (
      approve &&
      proposal.status === "failed_evals" &&
      !window.confirm(
        `Proposal #${proposal.id}: the candidate scored below baseline ` +
          `(${formatScore(proposal.candidate_score)} vs ${formatScore(proposal.baseline_score)}). ` +
          "Approve and activate it anyway?",
      )
    ) {
      return;
    }
    setDeciding(true);
    setDecisionError(null);
    setDecisionWarning(null);
    try {
      const res = await apiFetch<ApproveResponse>(
        runtimeRequest(`/proposals/${proposal.id}/approve`, { approve }),
      );
      if (res.warning) setDecisionWarning(res.warning);
      proposals.reload();
      if (approve) active.reload(); // approval hot-swaps the active prompt
    } catch (err) {
      setDecisionError(errorMessage(err));
    } finally {
      setDeciding(false);
    }
  };

  const head = (
    <PageHead
      title="Improve"
      subtitle="Eval-gated self-improvement — run the eval suite, let the runtime propose a better system prompt, and approve or deny it. Nothing activates without you."
    />
  );

  if (disabled) {
    return (
      <>
        {head}
        <EmptyState
          title="Self-improvement is not enabled on this runtime"
          description={
            <>
              It needs the checkpoint database (set{" "}
              <span className="mono">AGENTOS_CHECKPOINT_DB</span>) to store eval runs and
              prompt proposals — once configured, this page lights up.
            </>
          }
        />
      </>
    );
  }

  const runRows = runs.data === DISABLED ? [] : (runs.data ?? []);
  const proposalRows = proposals.data === DISABLED ? [] : (proposals.data ?? []);
  const activeData = active.data === DISABLED ? null : active.data;

  return (
    <>
      {head}
      <ErrorNotice error={active.error} />
      <ErrorNotice error={runs.error} />
      <ErrorNotice error={proposals.error} />

      {activeData && <ActivePromptBanner active={activeData} />}

      <Panel>
        <PanelHead
          title="Eval runs"
          actions={
            <Button variant="primary" small onClick={() => void runEvals()} disabled={running}>
              {running ? "Running evals…" : "Run evals"}
            </Button>
          }
        />
        {(runError || lastRun) && (
          <div className="panel-body">
            <ErrorNotice error={runError} />
            {lastRun && <RunResult run={lastRun} />}
          </div>
        )}
        {runs.loading && runRows.length === 0 ? (
          <div className="panel-body">
            <Skeleton lines={4} />
          </div>
        ) : runRows.length === 0 ? (
          <EmptyState
            title="No eval runs yet"
            description="Run the default suite to get a baseline."
          />
        ) : (
          <Table>
            <thead>
              <tr>
                <th>Time</th>
                <th>Suite</th>
                <th className="num">Score</th>
                <th className="num">Passed / failed</th>
                <th>Prompt source</th>
              </tr>
            </thead>
            <Tbody staggerKey={runRows.length === 0 ? "empty" : `${runRows.length}-${runRows[0].id}`}>
              {runRows.map((r) => (
                <Tr key={r.id}>
                  <td className="dim mono">{formatTimestamp(r.created_at)}</td>
                  <td className="mono">{r.suite}</td>
                  <td className="num">{formatScore(r.score)}</td>
                  <td className="num">
                    <span className="status-ok">{r.passed}</span> /{" "}
                    <span className={r.failed > 0 ? "status-err" : "dim"}>{r.failed}</span>
                  </td>
                  <td className="mono dim">{r.prompt_source}</td>
                </Tr>
              ))}
            </Tbody>
          </Table>
        )}
      </Panel>

      <Panel>
        <PanelHead
          title="Proposals"
          actions={
            <Button
              variant="primary"
              small
              onClick={() => void propose()}
              disabled={improving}
            >
              {improving ? "Proposing… (model call, may take a while)" : "Propose improvement"}
            </Button>
          }
        />
        <div className="panel-body">
          <ErrorNotice error={improveError} />
          <ErrorNotice error={decisionError} />
          {decisionWarning && <div className="notice warn">{decisionWarning}</div>}
          {proposals.loading && proposalRows.length === 0 ? (
            <Skeleton lines={4} />
          ) : proposalRows.length === 0 ? (
            <EmptyState
              title="No proposals yet"
              description="“Propose improvement” asks the model to rewrite the system prompt, then auto-evaluates the candidate against the baseline."
            />
          ) : (
            proposalRows.map((p) => (
              <ProposalCard key={p.id} proposal={p} busy={deciding} onDecide={decide} />
            ))
          )}
        </div>
      </Panel>
    </>
  );
}

function ActivePromptBanner({ active }: { active: ActivePrompt }) {
  const [open, setOpen] = useState(false);
  const reduced = useReducedMotion();
  return (
    <Panel>
      <PanelHead
        title={
          <span className="head-group">
            Active prompt
            <Badge variant={active.source === "proposal" ? "approved" : undefined}>
              {promptSourceLabel(active.source, active.proposal_id)}
            </Badge>
          </span>
        }
        actions={
          <Button small onClick={() => setOpen((v) => !v)}>
            {open ? "Hide prompt" : "Show prompt"}
          </Button>
        }
      />
      <AnimatePresence initial={false}>
        {open && (
          <motion.div
            className="improve-prompt-reveal"
            initial={reduced ? { opacity: 1 } : { opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={reduced ? { opacity: 1 } : { opacity: 0, height: 0 }}
            transition={{ duration: DUR_MED, ease: EASE }}
          >
            <div className="panel-body">
              <pre className="prompt-text mono">{active.prompt}</pre>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </Panel>
  );
}

function RunResult({ run }: { run: EvalRunResult }) {
  const [open, setOpen] = useState(true);
  const reduced = useReducedMotion();
  const fadeVariants = reduced ? fadeRiseReduced : fadeRise;
  return (
    <motion.div
      className="improve-run-result"
      variants={fadeVariants}
      initial="hidden"
      animate="show"
    >
      <div className="improve-run-head">
        <span className="improve-run-summary">
          <span>
            Run <span className="mono">#{run.run_id}</span> — {formatScore(run.score)}
          </span>
          <Badge variant="pass">{run.passed} passed</Badge>
          <Badge variant={run.failed > 0 ? "fail" : "inactive"}>{run.failed} failed</Badge>
        </span>
        <Button small onClick={() => setOpen((v) => !v)}>
          {open ? "Hide cases" : "Show cases"}
        </Button>
      </div>
      <AnimatePresence initial={false}>
        {open &&
          (reduced ? (
            <ul className="improve-case-list">
              {run.cases.map((c) => (
                <CaseRow key={c.name} c={c} />
              ))}
            </ul>
          ) : (
            <motion.ul
              key={run.run_id}
              className="improve-case-list"
              variants={staggerContainer}
              initial="hidden"
              animate="show"
              exit={{ opacity: 0, transition: transitionFast }}
            >
              {run.cases.map((c) => (
                <motion.li key={c.name} variants={staggerItem}>
                  <CaseRowInner c={c} />
                </motion.li>
              ))}
            </motion.ul>
          ))}
      </AnimatePresence>
    </motion.div>
  );
}

function CaseRow({ c }: { c: EvalCaseResult }) {
  return (
    <li>
      <CaseRowInner c={c} />
    </li>
  );
}

function CaseRowInner({ c }: { c: EvalCaseResult }) {
  return (
    <>
      <Badge variant={c.passed ? "pass" : "fail"}>{c.passed ? "pass" : "fail"}</Badge>
      <span className="mono">{c.name}</span>
      <span className="mono improve-case-output">{c.output_snippet}</span>
    </>
  );
}

function ProposalCard({
  proposal,
  busy,
  onDecide,
}: {
  proposal: Proposal;
  busy: boolean;
  onDecide: (p: Proposal, approve: boolean) => Promise<void>;
}) {
  const [showPrompt, setShowPrompt] = useState(false);
  const reduced = useReducedMotion();
  const badge = proposalBadge(proposal.status);
  const decided = !isUndecided(proposal.status);
  const gained = proposal.candidate_score >= proposal.baseline_score;
  return (
    <Card className="improve-proposal">
      {/* Settle lives on this inner wrapper: the Card root's opacity is
          framer-controlled (inline) after its entrance, so a CSS class there
          would lose. */}
      <div className={`improve-proposal-body${decided ? " decided" : ""}`}>
        <div className="improve-proposal-head">
          <span className="mono dim">#{proposal.id}</span>
          <AnimatePresence mode="wait" initial={false}>
            <motion.span
              key={proposal.status}
              className="improve-badge-swap"
              initial={reduced ? { opacity: 1 } : { opacity: 0, scale: 0.9 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={reduced ? { opacity: 1 } : { opacity: 0, scale: 0.9 }}
              transition={transitionFast}
            >
              <Badge variant={badgeVariantOf(badge.className)}>{badge.label}</Badge>
            </motion.span>
          </AnimatePresence>
          <span>
            baseline {formatScore(proposal.baseline_score)} → candidate{" "}
            {formatScore(proposal.candidate_score)}{" "}
            <span className={gained ? "status-ok" : "status-err"}>
              ({scoreDelta(proposal.baseline_score, proposal.candidate_score)})
            </span>
          </span>
          <span className="dim mono improve-proposal-time">
            {formatTimestamp(proposal.created_at)}
          </span>
        </div>
        <p className="rationale">{proposal.rationale}</p>
        <div className="improve-proposal-actions">
          <Button small onClick={() => setShowPrompt((v) => !v)}>
            {showPrompt ? "Hide prompt" : "Show prompt"}
          </Button>
          {isUndecided(proposal.status) && (
            <>
              <Button
                variant="primary"
                small
                disabled={busy}
                onClick={() => void onDecide(proposal, true)}
              >
                Approve
              </Button>
              <Button
                variant="danger"
                small
                disabled={busy}
                onClick={() => void onDecide(proposal, false)}
              >
                Deny
              </Button>
            </>
          )}
        </div>
        <AnimatePresence initial={false}>
          {showPrompt && (
            <motion.div
              className="improve-prompt-reveal"
              initial={reduced ? { opacity: 1 } : { opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: "auto" }}
              exit={reduced ? { opacity: 1 } : { opacity: 0, height: 0 }}
              transition={{ duration: DUR_MED, ease: EASE }}
            >
              <pre className="prompt-text mono">{proposal.prompt_text}</pre>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </Card>
  );
}
