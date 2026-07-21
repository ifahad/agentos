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
  EvalRunResult,
  EvalRunSummary,
  Proposal,
} from "../lib/types";

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
        <div className="notice">
          Self-improvement is not enabled on this runtime. It needs the checkpoint database
          (set <span className="mono">AGENTOS_CHECKPOINT_DB</span>) to store eval runs and
          prompt proposals — once configured, this page lights up.
        </div>
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

      <div className="panel">
        <div className="panel-head">
          <div className="head-group">
            <h2>Eval runs</h2>
            {runs.loading && <span className="spin">loading…</span>}
          </div>
          <button className="btn small primary" onClick={() => void runEvals()} disabled={running}>
            {running ? "Running evals…" : "Run evals"}
          </button>
        </div>
        {(runError || lastRun) && (
          <div className="panel-body">
            <ErrorNotice error={runError} />
            {lastRun && <RunResult run={lastRun} />}
          </div>
        )}
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Time</th>
                <th>Suite</th>
                <th className="num">Score</th>
                <th className="num">Passed / failed</th>
                <th>Prompt source</th>
              </tr>
            </thead>
            <tbody>
              {runRows.map((r) => (
                <tr key={r.id}>
                  <td className="dim mono">{formatTimestamp(r.created_at)}</td>
                  <td className="mono">{r.suite}</td>
                  <td className="num">{formatScore(r.score)}</td>
                  <td className="num">
                    <span className="status-ok">{r.passed}</span> /{" "}
                    <span className={r.failed > 0 ? "status-err" : "dim"}>{r.failed}</span>
                  </td>
                  <td className="mono dim">{r.prompt_source}</td>
                </tr>
              ))}
              {runRows.length === 0 && !runs.loading && (
                <tr>
                  <td colSpan={5} className="empty">
                    No eval runs yet — run the default suite to get a baseline.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      <div className="panel">
        <div className="panel-head">
          <div className="head-group">
            <h2>Proposals</h2>
            {proposals.loading && <span className="spin">loading…</span>}
          </div>
          <button
            className="btn small primary"
            onClick={() => void propose()}
            disabled={improving}
          >
            {improving ? "Proposing… (model call, may take a while)" : "Propose improvement"}
          </button>
        </div>
        <div className="panel-body">
          <ErrorNotice error={improveError} />
          <ErrorNotice error={decisionError} />
          {decisionWarning && <div className="notice warn">{decisionWarning}</div>}
          {proposalRows.map((p) => (
            <ProposalCard key={p.id} proposal={p} busy={deciding} onDecide={decide} />
          ))}
          {proposalRows.length === 0 && !proposals.loading && (
            <div className="empty">
              No proposals yet — “Propose improvement” asks the model to rewrite the system
              prompt, then auto-evaluates the candidate against the baseline.
            </div>
          )}
        </div>
      </div>
    </>
  );
}

function ActivePromptBanner({ active }: { active: ActivePrompt }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="panel">
      <div className="panel-head">
        <div className="head-group">
          <h2>Active prompt</h2>
          <span className={`badge${active.source === "proposal" ? " approved" : ""}`}>
            {promptSourceLabel(active.source, active.proposal_id)}
          </span>
        </div>
        <button className="btn small" onClick={() => setOpen((v) => !v)}>
          {open ? "Hide prompt" : "Show prompt"}
        </button>
      </div>
      {open && (
        <div className="panel-body">
          <pre className="prompt-text">{active.prompt}</pre>
        </div>
      )}
    </div>
  );
}

function RunResult({ run }: { run: EvalRunResult }) {
  const [open, setOpen] = useState(true);
  return (
    <div className="run-result">
      <div className="head-group">
        <span>
          Run <span className="mono">#{run.run_id}</span> — {formatScore(run.score)} (
          <span className="status-ok">{run.passed} passed</span>,{" "}
          <span className={run.failed > 0 ? "status-err" : "dim"}>{run.failed} failed</span>)
        </span>
        <button className="btn small" onClick={() => setOpen((v) => !v)}>
          {open ? "Hide cases" : "Show cases"}
        </button>
      </div>
      {open && (
        <ul className="case-list">
          {run.cases.map((c) => (
            <li key={c.name}>
              <span className={`badge ${c.passed ? "pass" : "fail"}`}>
                {c.passed ? "pass" : "fail"}
              </span>
              <span className="mono">{c.name}</span>
              <span className="mono case-output">{c.output_snippet}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
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
  const badge = proposalBadge(proposal.status);
  const gained = proposal.candidate_score >= proposal.baseline_score;
  return (
    <div className="proposal">
      <div className="proposal-head">
        <span className="mono dim">#{proposal.id}</span>
        <span className={badge.className}>{badge.label}</span>
        <span>
          baseline {formatScore(proposal.baseline_score)} → candidate{" "}
          {formatScore(proposal.candidate_score)}{" "}
          <span className={gained ? "status-ok" : "status-err"}>
            ({scoreDelta(proposal.baseline_score, proposal.candidate_score)})
          </span>
        </span>
        <span className="dim mono proposal-time">{formatTimestamp(proposal.created_at)}</span>
      </div>
      <p className="rationale">{proposal.rationale}</p>
      <div className="proposal-actions">
        <button className="btn small" onClick={() => setShowPrompt((v) => !v)}>
          {showPrompt ? "Hide prompt" : "Show prompt"}
        </button>
        {isUndecided(proposal.status) && (
          <>
            <button
              className="btn small primary"
              disabled={busy}
              onClick={() => void onDecide(proposal, true)}
            >
              Approve
            </button>
            <button
              className="btn small danger"
              disabled={busy}
              onClick={() => void onDecide(proposal, false)}
            >
              Deny
            </button>
          </>
        )}
      </div>
      {showPrompt && <pre className="prompt-text">{proposal.prompt_text}</pre>}
    </div>
  );
}
