import { useState } from "react";
import type { PageProps } from "../App";
import { ErrorNotice, PageHead, errorMessage, useLoad } from "../components/common";
import { Freshness } from "../components/Freshness";
import { useLiveResource } from "../hooks/useLiveResource";
import { apiFetch, apiFetchRaw } from "../lib/api";
import {
  agreementLabel,
  approveProposalRequest,
  cancelObjectiveRequest,
  createObjectiveRequest,
  getObjectiveRequest,
  listMembersRequest,
  listObjectivesRequest,
  listProposalsRequest,
  pauseRequest,
} from "../lib/council";
import { formatTimestamp } from "../lib/format";
import { objectivesSummary } from "../lib/summaries";
import type {
  CouncilCycle,
  CouncilMember,
  CouncilObjective,
  CouncilProposal,
} from "../lib/types";
import {
  Badge,
  Button,
  Disclosure,
  EmptyState,
  Input,
  Panel,
  PanelHead,
  Skeleton,
  Table,
  Tbody,
  Tr,
  useToast,
} from "../ui";
import type { BadgeVariant } from "../ui";
import { Icon } from "../ui/icons";
import "./Multiverse.css";

// The council 503s when it is not configured (no council.yaml or no checkpoint
// database). That is a normal state, not an error — the same convention the
// Improve page uses for self-improvement being off.
const DISABLED = Symbol("council-disabled");

async function orDisabled<T>(promise: Promise<T>): Promise<T | typeof DISABLED> {
  try {
    return await promise;
  } catch (err) {
    if (String(err).includes("503")) return DISABLED;
    throw err;
  }
}

export function Multiverse({ adminKey }: PageProps) {
  const toast = useToast();
  const [input, setInput] = useState("");
  const [selected, setSelected] = useState<string | null>(null);
  const [busy, setBusy] = useState(0);
  const [createOpen, setCreateOpen] = useState(false);

  const members = useLoad(
    () => orDisabled(apiFetch<{ members: CouncilMember[] }>(listMembersRequest())),
    [adminKey],
  );
  const objectives = useLiveResource<{ objectives: CouncilObjective[] } | typeof DISABLED>(
    `council/objectives#${adminKey}`,
    () => orDisabled(apiFetch<{ objectives: CouncilObjective[] }>(listObjectivesRequest())),
    { enabled: Boolean(adminKey), cadence: 4000 },
  );
  const proposals = useLiveResource<{ proposals: CouncilProposal[] } | typeof DISABLED>(
    `council/proposals#${adminKey}`,
    () => orDisabled(apiFetch<{ proposals: CouncilProposal[] }>(listProposalsRequest())),
    { enabled: Boolean(adminKey), cadence: 4000 },
  );
  const detail = useLoad(
    () =>
      selected
        ? apiFetch<{ objective: CouncilObjective; cycles: CouncilCycle[] }>(
            getObjectiveRequest(selected),
          )
        : Promise.resolve(null),
    [selected, busy],
  );

  const disabled = members.data === DISABLED;

  async function act(run: () => Promise<void>, ok: string) {
    try {
      await run();
      toast.success(ok);
      setBusy((n) => n + 1); // reload the detail view, which still depends on `busy`
      objectives.reload();
      proposals.reload();
    } catch (err) {
      toast.error(errorMessage(err));
    }
  }

  const launch = () =>
    act(async () => {
      await apiFetchRaw(createObjectiveRequest(input.trim()));
      setInput("");
      setCreateOpen(false);
    }, "Objective queued");

  const memberList = members.data !== DISABLED ? (members.data?.members ?? []) : [];
  const objectiveList = objectives.data !== DISABLED ? (objectives.data?.objectives ?? []) : [];
  const proposalList = proposals.data !== DISABLED ? (proposals.data?.proposals ?? []) : [];
  const enabledCount = memberList.filter((m) => m.enabled).length;

  return (
    <>
      <PageHead
        title="Multiverse"
        subtitle="A council of model-bound agents answers one objective; the judge synthesizes a verdict and reports dissent."
      />
      <ErrorNotice error={members.error} />

      {disabled && (
        <div className="notice warn">
          The council is not configured on this runtime. Set AGENTOS_COUNCIL_CONFIG and a checkpoint
          database to enable it.
        </div>
      )}

      {!disabled && (
        <>
          <Panel>
            <PanelHead
              title="Council"
              actions={
                <div className="mv-controls">
                  <span className="eyebrow">
                    {enabledCount}/{memberList.length} active
                  </span>
                  <Button
                    variant="ghost"
                    onClick={() =>
                      act(async () => {
                        await apiFetchRaw(pauseRequest(true));
                      }, "Council paused")
                    }
                  >
                    Pause
                  </Button>
                  <Button
                    variant="ghost"
                    onClick={() =>
                      act(async () => {
                        await apiFetchRaw(pauseRequest(false));
                      }, "Council resumed")
                    }
                  >
                    Resume
                  </Button>
                </div>
              }
            />
            {members.loading && !members.data ? (
              <div className="mv-pad">
                <Skeleton lines={3} height={14} />
              </div>
            ) : (
              <div className="mv-members">
                {memberList.map((m) => (
                  <div key={m.id} className="mv-member" data-enabled={m.enabled || undefined}>
                    <Icon name="multiverse" size={14} className="mv-member-icon" />
                    <div className="mv-member-body">
                      <div className="mv-member-id">{m.id}</div>
                      <div className="mono mv-member-model">{m.model}</div>
                    </div>
                    <div className="mv-member-tags">
                      <span className="eyebrow">{m.profile}</span>
                      <Badge variant={m.enabled ? "pass" : "inactive"}>
                        {m.enabled ? "active" : "off"}
                      </Badge>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </Panel>

          <div className="mv-split">
            <Panel>
              <PanelHead
                title="Objectives"
                summary={objectivesSummary(objectiveList, proposalList)}
                actions={
                  <span className="head-group">
                    <Freshness updatedAt={objectives.updatedAt} />
                    <Button
                      variant="primary"
                      icon="plus"
                      aria-expanded={createOpen}
                      aria-controls="multiverse-create"
                      onClick={() => setCreateOpen((v) => !v)}
                    >
                      New objective
                    </Button>
                  </span>
                }
              />
              <Disclosure open={createOpen} onOpenChange={setCreateOpen} id="multiverse-create">
                <div className="mv-launch">
                  <Input
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                    placeholder="Ask the council a question…"
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && input.trim()) launch();
                    }}
                  />
                  <Button onClick={launch} disabled={!input.trim()}>
                    Queue
                  </Button>
                </div>
              </Disclosure>
              {objectiveList.length === 0 ? (
                <EmptyState title="No objectives yet" description="Queue one above to begin." />
              ) : (
                <Table>
                  <thead>
                    <tr>
                      <th>Input</th>
                      <th>Status</th>
                      <th>Cycles</th>
                      <th>Created</th>
                    </tr>
                  </thead>
                  <Tbody staggerKey={objectiveList.length}>
                    {objectiveList.map((o) => (
                      <Tr
                        key={o.id}
                        onClick={() => setSelected(o.id)}
                        data-selected={o.id === selected || undefined}
                      >
                        <td className="mv-input-cell">{o.input}</td>
                        <td>
                          <Badge variant={statusVariant(o.status)}>
                            {o.stop_reason ?? o.status}
                          </Badge>
                        </td>
                        <td className="num">{o.cycles_run}</td>
                        <td className="mono">{formatTimestamp(o.created_at)}</td>
                      </Tr>
                    ))}
                  </Tbody>
                </Table>
              )}
            </Panel>

            <Panel>
              <PanelHead title={selected ? "Verdict" : "Select an objective"} />
              {!selected ? (
                <EmptyState
                  title="No objective selected"
                  description="Pick an objective to see its cycles and dissent."
                />
              ) : detail.loading && !detail.data ? (
                <div className="mv-pad">
                  <Skeleton lines={4} height={14} />
                </div>
              ) : detail.data ? (
                <div className="mv-detail">
                  <div className="mv-detail-head">
                    <span className="mono mv-obj-input">{detail.data.objective.input}</span>
                    {detail.data.objective.status === "running" && (
                      <Button
                        variant="ghost"
                        onClick={() =>
                          act(async () => {
                            await apiFetchRaw(cancelObjectiveRequest(selected));
                          }, "Objective cancelled")
                        }
                      >
                        Cancel
                      </Button>
                    )}
                  </div>
                  {detail.data.cycles.length === 0 ? (
                    <EmptyState title="No cycles yet" description="The council has not run yet." />
                  ) : (
                    detail.data.cycles.map((c) => (
                      <div key={c.id} className="mv-cycle">
                        <div className="mv-cycle-head">
                          <span className="eyebrow">cycle {c.cycle_no}</span>
                          <Badge variant={agreementVariant(c.agreement)}>
                            {agreementLabel(c.agreement)}
                          </Badge>
                        </div>
                        <div className="mv-answer">{c.verdict.answer || "—"}</div>
                        {c.dissent.length > 0 && (
                          <ul className="mv-dissent">
                            {c.dissent.map((d, i) => (
                              <li key={i}>
                                <span className="mv-dissent-member">{d.member}</span>
                                <span className="mv-dissent-claim">{d.claim}</span>
                                <span className="mv-dissent-basis">{d.basis}</span>
                              </li>
                            ))}
                          </ul>
                        )}
                      </div>
                    ))
                  )}
                </div>
              ) : null}
            </Panel>
          </div>

          {proposalList.length > 0 && (
            <Panel>
              <PanelHead
                title="Held write actions"
                actions={<Freshness updatedAt={proposals.updatedAt} />}
              />
              <Table>
                <thead>
                  <tr>
                    <th>Member</th>
                    <th>Tool</th>
                    <th>Status</th>
                    <th></th>
                  </tr>
                </thead>
                <Tbody staggerKey={proposalList.length}>
                  {proposalList.map((p) => (
                    <Tr key={p.id}>
                      <td>{p.member_id}</td>
                      <td className="mono">{p.tool}</td>
                      <td>
                        <Badge variant={p.status === "approved" ? "pass" : "failed_evals"}>
                          {p.status}
                        </Badge>
                      </td>
                      <td className="mv-action-cell">
                        {p.status === "pending" && (
                          <Button
                            variant="ghost"
                            onClick={() =>
                              act(async () => {
                                await apiFetchRaw(approveProposalRequest(p.id));
                              }, "Proposal approved")
                            }
                          >
                            Approve
                          </Button>
                        )}
                      </td>
                    </Tr>
                  ))}
                </Tbody>
              </Table>
            </Panel>
          )}
        </>
      )}
    </>
  );
}

function statusVariant(status: string): BadgeVariant {
  if (status === "completed") return "pass";
  if (status === "needs_review") return "failed_evals";
  if (status === "cancelled" || status === "paused") return "inactive";
  return "chat"; // running / pending — monochrome, not a signal
}

function agreementVariant(agreement: number): BadgeVariant {
  if (agreement >= 0.75) return "pass";
  if (agreement >= 0.5) return "failed_evals";
  return "fail";
}
