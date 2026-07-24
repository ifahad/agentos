import { useState } from "react";
import type { PageProps } from "../App";
import { ErrorNotice, ForbiddenNotice, NeedsKey, PageHead, errorMessage, useLoad } from "../components/common";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
import { budgetFraction, formatRpm, formatUSD } from "../lib/format";
import { can } from "../lib/rbac";
import type { Org } from "../lib/types";
import { Button, Input, Panel, PanelHead, Skeleton, Table, Tbody, Tr, useToast } from "../ui";

export function Orgs({ adminKey, role, openSettings }: PageProps) {
  const allowed = can(role, "org.view");
  const toast = useToast();

  const { data, error, loading, reload } = useLoad(
    () =>
      adminKey && allowed
        ? apiFetch<Org[]>(gatewayAdminRequest("/admin/orgs", adminKey))
        : Promise.resolve<Org[]>([]),
    [adminKey, allowed],
  );

  const [name, setName] = useState("");
  const [budget, setBudget] = useState("500");
  const [rpm, setRpm] = useState("0");
  const [creating, setCreating] = useState(false);

  // Per-row rate-limit edits (org id -> draft string) and the id being saved.
  const [rpmEdits, setRpmEdits] = useState<Record<string, string>>({});
  const [savingRpm, setSavingRpm] = useState<string | null>(null);

  const canCreate = can(role, "org.create");

  const create = async () => {
    const budgetNum = Number(budget);
    const rpmNum = Number(rpm || "0");
    if (!name.trim() || !Number.isFinite(budgetNum) || budgetNum < 0) {
      toast.error("A name and a non-negative monthly budget are required.");
      return;
    }
    if (!Number.isFinite(rpmNum) || rpmNum < 0) {
      toast.error("Rate limit must be a non-negative number (0 = unlimited).");
      return;
    }
    setCreating(true);
    try {
      await apiFetch<Org>(
        gatewayAdminRequest("/admin/orgs", adminKey, {
          name: name.trim(),
          monthly_budget_usd: budgetNum,
          rate_limit_rpm: rpmNum,
        }),
      );
      setName("");
      setRpm("0");
      toast.success(`Org "${name.trim()}" created.`);
      reload();
    } catch (err) {
      toast.error(errorMessage(err));
    } finally {
      setCreating(false);
    }
  };

  const saveRpm = async (org: Org) => {
    const draft = rpmEdits[org.id] ?? String(org.rate_limit_rpm);
    const next = Number(draft);
    if (!Number.isFinite(next) || next < 0) {
      toast.error("Rate limit must be a non-negative number (0 = unlimited).");
      return;
    }
    setSavingRpm(org.id);
    try {
      await apiFetch<Org>(
        gatewayAdminRequest(`/admin/orgs/${org.id}`, adminKey, { rate_limit_rpm: next }, { method: "PATCH" }),
      );
      setRpmEdits((e) => {
        const copy = { ...e };
        delete copy[org.id];
        return copy;
      });
      toast.success(`Rate limit for ${org.name} updated.`);
      reload();
    } catch (err) {
      toast.error(errorMessage(err));
    } finally {
      setSavingRpm(null);
    }
  };

  const orgs = data ?? [];
  const showSkeleton = loading && orgs.length === 0;

  return (
    <>
      <PageHead
        title="Orgs"
        subtitle="Tenants of the gateway. Each org caps the total spend of its keys, sets a per-tenant request rate, and owns its users. Root admin only."
      />
      {!adminKey && <NeedsKey openSettings={openSettings} />}
      {adminKey && !allowed && <ForbiddenNotice message="Orgs are visible to the root admin only." />}
      <ErrorNotice error={error} />

      {adminKey && allowed && (
        <>
          {canCreate && (
            <Panel>
              <PanelHead title="Create org" />
              <div className="panel-body">
                <div className="form-row">
                  <Input
                    label="Name"
                    type="text"
                    value={name}
                    placeholder="acme-corp"
                    onChange={(e) => setName(e.target.value)}
                  />
                  <Input
                    label="Monthly budget (USD)"
                    type="number"
                    min="0"
                    step="1"
                    value={budget}
                    onChange={(e) => setBudget(e.target.value)}
                  />
                  <Input
                    label="Rate limit (rpm, 0 = unlimited)"
                    type="number"
                    min="0"
                    step="1"
                    value={rpm}
                    onChange={(e) => setRpm(e.target.value)}
                  />
                </div>
                <Button variant="primary" onClick={() => void create()} disabled={creating}>
                  {creating ? "Creating…" : "Create org"}
                </Button>
              </div>
            </Panel>
          )}

          <Panel>
            <PanelHead title="Organizations" />
            <Table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Id</th>
                  <th className="num">Monthly budget</th>
                  <th className="num">Aggregate spend</th>
                  <th>Budget used</th>
                  <th>Rate limit (rpm)</th>
                </tr>
              </thead>
              <Tbody staggerKey={orgs.length}>
                {showSkeleton &&
                  Array.from({ length: 3 }, (_, i) => (
                    <Tr key={`skel-${i}`} animate={false}>
                      <td><Skeleton width={120} /></td>
                      <td><Skeleton width={140} /></td>
                      <td className="num"><Skeleton width={64} /></td>
                      <td className="num"><Skeleton width={64} /></td>
                      <td><Skeleton width={140} /></td>
                      <td><Skeleton width={160} /></td>
                    </Tr>
                  ))}
                {orgs.map((o) => {
                  const frac = budgetFraction(o.spend_usd, o.monthly_budget_usd);
                  const draft = rpmEdits[o.id] ?? String(o.rate_limit_rpm);
                  const dirty = draft !== String(o.rate_limit_rpm);
                  return (
                    <Tr key={o.id}>
                      <td>{o.name}</td>
                      <td className="mono dim">{o.id}</td>
                      <td className="num">{formatUSD(o.monthly_budget_usd)}</td>
                      <td className="num">{formatUSD(o.spend_usd)}</td>
                      <td>
                        <span className={`meter${frac >= 0.9 ? " hot" : ""}`}>
                          <div style={{ width: `${Math.round(frac * 100)}%` }} />
                        </span>{" "}
                        <span className="dim muted">{Math.round(frac * 100)}%</span>
                      </td>
                      <td>
                        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                          <Input
                            type="number"
                            min="0"
                            step="1"
                            className="rpm-input"
                            aria-label={`Rate limit for ${o.name}`}
                            value={draft}
                            onChange={(e) =>
                              setRpmEdits((edits) => ({ ...edits, [o.id]: e.target.value }))
                            }
                          />
                          <span className="dim muted">{formatRpm(o.rate_limit_rpm)}</span>
                          {dirty && (
                            <Button
                              small
                              variant="primary"
                              onClick={() => void saveRpm(o)}
                              disabled={savingRpm === o.id}
                            >
                              {savingRpm === o.id ? "Saving…" : "Save"}
                            </Button>
                          )}
                        </div>
                      </td>
                    </Tr>
                  );
                })}
                {orgs.length === 0 && !loading && (
                  <Tr animate={false}>
                    <td colSpan={6} className="empty">
                      No orgs yet{canCreate ? " — create one above." : "."}
                    </td>
                  </Tr>
                )}
              </Tbody>
            </Table>
          </Panel>
        </>
      )}
    </>
  );
}
