import { useState } from "react";
import type { PageProps } from "../App";
import { ErrorNotice, ForbiddenNotice, NeedsKey, PageHead, errorMessage, useLoad } from "../components/common";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
import { budgetFraction, formatRpm, formatUSD } from "../lib/format";
import { can } from "../lib/rbac";
import type { Org } from "../lib/types";

export function Orgs({ adminKey, role, openSettings }: PageProps) {
  const allowed = can(role, "org.view");

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
  const [createError, setCreateError] = useState<string | null>(null);

  // Per-row rate-limit edits (org id -> draft string) and the id being saved.
  const [rpmEdits, setRpmEdits] = useState<Record<string, string>>({});
  const [savingRpm, setSavingRpm] = useState<string | null>(null);
  const [rowError, setRowError] = useState<string | null>(null);

  const canCreate = can(role, "org.create");

  const create = async () => {
    const budgetNum = Number(budget);
    const rpmNum = Number(rpm || "0");
    if (!name.trim() || !Number.isFinite(budgetNum) || budgetNum < 0) {
      setCreateError("A name and a non-negative monthly budget are required.");
      return;
    }
    if (!Number.isFinite(rpmNum) || rpmNum < 0) {
      setCreateError("Rate limit must be a non-negative number (0 = unlimited).");
      return;
    }
    setCreating(true);
    setCreateError(null);
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
      reload();
    } catch (err) {
      setCreateError(errorMessage(err));
    } finally {
      setCreating(false);
    }
  };

  const saveRpm = async (org: Org) => {
    const draft = rpmEdits[org.id] ?? String(org.rate_limit_rpm);
    const next = Number(draft);
    if (!Number.isFinite(next) || next < 0) {
      setRowError("Rate limit must be a non-negative number (0 = unlimited).");
      return;
    }
    setSavingRpm(org.id);
    setRowError(null);
    try {
      await apiFetch<Org>(
        gatewayAdminRequest(`/admin/orgs/${org.id}`, adminKey, { rate_limit_rpm: next }, { method: "PATCH" }),
      );
      setRpmEdits((e) => {
        const copy = { ...e };
        delete copy[org.id];
        return copy;
      });
      reload();
    } catch (err) {
      setRowError(errorMessage(err));
    } finally {
      setSavingRpm(null);
    }
  };

  const orgs = data ?? [];

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
            <div className="panel">
              <div className="panel-head">
                <h2>Create org</h2>
              </div>
              <div className="panel-body">
                <ErrorNotice error={createError} />
                <div className="form-row">
                  <label className="field">
                    <span>Name</span>
                    <input
                      type="text"
                      value={name}
                      placeholder="acme-corp"
                      onChange={(e) => setName(e.target.value)}
                    />
                  </label>
                  <label className="field">
                    <span>Monthly budget (USD)</span>
                    <input
                      type="number"
                      min="0"
                      step="1"
                      value={budget}
                      onChange={(e) => setBudget(e.target.value)}
                    />
                  </label>
                  <label className="field">
                    <span>Rate limit (rpm, 0 = unlimited)</span>
                    <input
                      type="number"
                      min="0"
                      step="1"
                      value={rpm}
                      onChange={(e) => setRpm(e.target.value)}
                    />
                  </label>
                </div>
                <button className="btn primary" onClick={() => void create()} disabled={creating}>
                  {creating ? "Creating…" : "Create org"}
                </button>
              </div>
            </div>
          )}

          <div className="panel">
            <div className="panel-head">
              <h2>Organizations</h2>
              {loading && <span className="spin">loading…</span>}
            </div>
            <ErrorNotice error={rowError} />
            <div className="table-wrap">
              <table>
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
                <tbody>
                  {orgs.map((o) => {
                    const frac = budgetFraction(o.spend_usd, o.monthly_budget_usd);
                    const draft = rpmEdits[o.id] ?? String(o.rate_limit_rpm);
                    const dirty = draft !== String(o.rate_limit_rpm);
                    return (
                      <tr key={o.id}>
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
                            <input
                              type="number"
                              min="0"
                              step="1"
                              className="rpm-input"
                              value={draft}
                              onChange={(e) =>
                                setRpmEdits((edits) => ({ ...edits, [o.id]: e.target.value }))
                              }
                            />
                            <span className="dim muted">{formatRpm(o.rate_limit_rpm)}</span>
                            {dirty && (
                              <button
                                className="btn small primary"
                                onClick={() => void saveRpm(o)}
                                disabled={savingRpm === o.id}
                              >
                                {savingRpm === o.id ? "Saving…" : "Save"}
                              </button>
                            )}
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                  {orgs.length === 0 && !loading && (
                    <tr>
                      <td colSpan={6} className="empty">
                        No orgs yet{canCreate ? " — create one above." : "."}
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </>
      )}
    </>
  );
}
