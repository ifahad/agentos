import { useState } from "react";
import type { PageProps } from "../App";
import { ErrorNotice, PageHead, errorMessage, useLoad } from "../components/common";
import { useLiveResource } from "../hooks/useLiveResource";
import { useNowTick } from "../hooks/useNowTick";
import { apiFetch, apiFetchRaw } from "../lib/api";
import { formatTimestamp } from "../lib/format";
import {
  createOperatorRequest,
  deleteOperatorRequest,
  getOperatorRequest,
  listOperatorsRequest,
  operatorEta,
  runOperatorRequest,
  setEnabledRequest,
  triggerSummary,
} from "../lib/operators";
import { operatorsSummary } from "../lib/summaries";
import type { Operator, OperatorRun, OperatorTrigger } from "../lib/types";
import {
  Badge,
  Button,
  Disclosure,
  EmptyState,
  Input,
  Panel,
  PanelHead,
  Select,
  Skeleton,
  Table,
  Tbody,
  Textarea,
  Tr,
  useToast,
} from "../ui";
import type { BadgeVariant } from "../ui";
import { Icon } from "../ui/icons";
import "./Operators.css";

const DISABLED = Symbol("operators-disabled");

async function orDisabled<T>(promise: Promise<T>): Promise<T | typeof DISABLED> {
  try {
    return await promise;
  } catch (err) {
    if (String(err).includes("503")) return DISABLED;
    throw err;
  }
}

export function Operators(_props: PageProps) {
  const toast = useToast();
  const [name, setName] = useState("");
  const [goal, setGoal] = useState("");
  const [kind, setKind] = useState<OperatorTrigger["type"]>("interval");
  const [intervalS, setIntervalS] = useState("300");
  const [cron, setCron] = useState("0 9 * * *");
  const [selected, setSelected] = useState<string | null>(null);
  const [tick, setTick] = useState(0);
  const [createOpen, setCreateOpen] = useState(false);
  // Manual "Run" clicks in flight — the only "currently running" signal the
  // API surfaces client-side (a run's own status only exists once it has
  // already completed, needs approval, or errored).
  const [runningIds, setRunningIds] = useState<Set<string>>(new Set());

  const operators = useLiveResource<{ operators: Operator[] } | typeof DISABLED>(
    "operators",
    () => orDisabled(apiFetch<{ operators: Operator[] }>(listOperatorsRequest())),
    { cadence: 4000 },
  );
  const detail = useLoad(
    () =>
      selected
        ? apiFetch<{ operator: Operator; recent_runs: OperatorRun[] }>(
            getOperatorRequest(selected),
          )
        : Promise.resolve(null),
    [selected, tick],
  );

  const disabled = operators.data === DISABLED;
  const list = operators.data !== DISABLED ? (operators.data?.operators ?? []) : [];
  const operatorsLoading = operators.status === "loading";
  const now = useNowTick(1000);

  async function act(run: () => Promise<void>, ok: string) {
    try {
      await run();
      toast.success(ok);
      setTick((n) => n + 1);
      operators.reload();
    } catch (err) {
      toast.error(errorMessage(err));
    }
  }

  function runNow(op: Operator) {
    setRunningIds((prev) => new Set(prev).add(op.id));
    act(async () => {
      await apiFetchRaw(runOperatorRequest(op.id));
    }, "Operator run started").finally(() => {
      setRunningIds((prev) => {
        if (!prev.has(op.id)) return prev;
        const next = new Set(prev);
        next.delete(op.id);
        return next;
      });
    });
  }

  function buildTrigger(): OperatorTrigger {
    if (kind === "interval") return { type: "interval", interval_s: Number(intervalS) };
    if (kind === "cron") return { type: "cron", cron };
    return { type: "webhook" };
  }

  const create = () =>
    act(async () => {
      const res = await apiFetchRaw(
        createOperatorRequest({ name: name.trim(), goal: goal.trim(), trigger: buildTrigger() }),
      );
      // A webhook operator returns its token once — surface it immediately.
      const created = res.body as Operator;
      if (created?.trigger?.webhook_token) {
        toast.success(`Webhook token (shown once): ${created.trigger.webhook_token}`, {
          duration: 30000,
        });
      }
      setName("");
      setGoal("");
      setCreateOpen(false);
    }, "Operator created");

  return (
    <>
      <PageHead
        title="Operators"
        subtitle="Standing objectives the runtime pursues on its own — on an interval, a schedule, or an inbound webhook. Every run is governed and bounded."
      />
      <ErrorNotice error={operators.error} />

      {disabled && (
        <div className="notice warn">
          Operators require a checkpoint database (AGENTOS_CHECKPOINT_DATABASE_URL). The scheduler is
          separately opt-in via AGENTOS_AUTONOMY_ENABLED.
        </div>
      )}

      {!disabled && (
        <>
          <div className="op-split">
            <Panel>
              <PanelHead
                title="Operators"
                summary={operatorsSummary(list)}
                actions={
                  <Button
                    variant="primary"
                    icon="plus"
                    aria-expanded={createOpen}
                    aria-controls="operators-create"
                    onClick={() => setCreateOpen((v) => !v)}
                  >
                    New operator
                  </Button>
                }
              />
              <Disclosure open={createOpen} onOpenChange={setCreateOpen} id="operators-create">
                <div className="op-form">
                  <div className="op-form-row">
                    <Input
                      label="Name"
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      placeholder="nightly-invoice-report"
                    />
                    <Select
                      label="Trigger"
                      value={kind}
                      onChange={(e) => setKind(e.target.value as OperatorTrigger["type"])}
                    >
                      <option value="interval">interval</option>
                      <option value="cron">cron</option>
                      <option value="webhook">webhook</option>
                    </Select>
                    {kind === "interval" && (
                      <Input
                        label="Interval (s)"
                        value={intervalS}
                        onChange={(e) => setIntervalS(e.target.value)}
                      />
                    )}
                    {kind === "cron" && (
                      <Input label="Cron" value={cron} onChange={(e) => setCron(e.target.value)} />
                    )}
                    {kind === "webhook" && (
                      <div className="op-webhook-note eyebrow">token issued on create</div>
                    )}
                  </div>
                  <Textarea
                    label="Goal"
                    value={goal}
                    onChange={(e) => setGoal(e.target.value)}
                    placeholder="What should this operator do each time it fires?"
                    rows={3}
                  />
                  <Button icon="plus" onClick={create} disabled={!name.trim() || !goal.trim()}>
                    Create operator
                  </Button>
                </div>
              </Disclosure>
              {operatorsLoading && !operators.data ? (
                <div className="op-pad">
                  <Skeleton lines={3} height={14} />
                </div>
              ) : list.length === 0 ? (
                <EmptyState title="No operators" description="Create one above to begin." />
              ) : (
                <Table>
                  <thead>
                    <tr>
                      <th>Name</th>
                      <th>Trigger</th>
                      <th>State</th>
                      <th></th>
                    </tr>
                  </thead>
                  <Tbody staggerKey={list.length}>
                    {list.map((op) => {
                      const eta = op.enabled ? operatorEta(op, now) : null;
                      const running = runningIds.has(op.id);
                      return (
                        <Tr
                          key={op.id}
                          onClick={() => setSelected(op.id)}
                          data-selected={op.id === selected || undefined}
                        >
                          <td>
                            <Icon name="improve" size={13} className="op-row-icon" /> {op.name}
                            {running && <span className="op-live-dot" aria-hidden="true" />}
                          </td>
                          <td className="mono">
                            {triggerSummary(op.trigger)}
                            {eta !== null && (
                              <div className="freshness op-eta">
                                next run in {Math.floor(eta / 1000)}s
                              </div>
                            )}
                          </td>
                          <td>
                            <Badge variant={op.enabled ? "pass" : "inactive"}>
                              {op.enabled ? "enabled" : "paused"}
                            </Badge>
                          </td>
                          <td className="op-actions">
                            <Button
                              variant="ghost"
                              icon="run"
                              onClick={(e) => {
                                e.stopPropagation();
                                runNow(op);
                              }}
                            >
                              Run
                            </Button>
                            <Button
                              variant="ghost"
                              icon="pause"
                              onClick={(e) => {
                                e.stopPropagation();
                                act(async () => {
                                  await apiFetchRaw(setEnabledRequest(op.id, !op.enabled));
                                }, op.enabled ? "Paused" : "Enabled");
                              }}
                            >
                              {op.enabled ? "Pause" : "Enable"}
                            </Button>
                          </td>
                        </Tr>
                      );
                    })}
                  </Tbody>
                </Table>
              )}
            </Panel>

            <Panel>
              <PanelHead title={selected ? "Runs" : "Select an operator"} />
              {!selected ? (
                <EmptyState
                  title="No operator selected"
                  description="Pick an operator to see its run history."
                />
              ) : detail.loading && !detail.data ? (
                <div className="op-pad">
                  <Skeleton lines={4} height={14} />
                </div>
              ) : detail.data ? (
                <div className="op-detail">
                  <div className="op-detail-head">
                    <span className="mono op-goal">{detail.data.operator.goal}</span>
                    <Button
                      variant="ghost"
                      icon="trash"
                      onClick={() =>
                        act(async () => {
                          await apiFetchRaw(deleteOperatorRequest(selected));
                          setSelected(null);
                        }, "Operator deleted")
                      }
                    >
                      Delete
                    </Button>
                  </div>
                  {detail.data.recent_runs.length === 0 ? (
                    <EmptyState title="No runs yet" description="Run it now, or wait for its trigger." />
                  ) : (
                    detail.data.recent_runs.map((r) => (
                      <div key={r.id} className="op-run">
                        <div className="op-run-head">
                          <Badge variant={runVariant(r.status)}>{r.status}</Badge>
                          <span className="eyebrow">{r.trigger_source}</span>
                          <span className="mono op-run-cycles">{r.cycles} cycles</span>
                          <span className="mono op-run-time">{formatTimestamp(r.created_at)}</span>
                        </div>
                        {r.output && <div className="op-run-output">{r.output}</div>}
                        {r.error && <div className="op-run-error">{r.error}</div>}
                      </div>
                    ))
                  )}
                </div>
              ) : null}
            </Panel>
          </div>
        </>
      )}
    </>
  );
}

function runVariant(status: string): BadgeVariant {
  if (status === "completed") return "pass";
  if (status === "needs_approval") return "failed_evals";
  return "fail";
}
