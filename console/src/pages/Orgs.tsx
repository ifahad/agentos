import { useState } from "react";
import type { PageProps } from "../App";
import { ErrorNotice, ForbiddenNotice, NeedsKey, PageHead, errorMessage, useLoad } from "../components/common";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
import { budgetFraction, formatUSD } from "../lib/format";
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
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  const canCreate = can(role, "org.create");

  const create = async () => {
    const budgetNum = Number(budget);
    if (!name.trim() || !Number.isFinite(budgetNum) || budgetNum < 0) {
      setCreateError("A name and a non-negative monthly budget are required.");
      return;
    }
    setCreating(true);
    setCreateError(null);
    try {
      await apiFetch<Org>(
        gatewayAdminRequest("/admin/orgs", adminKey, {
          name: name.trim(),
          monthly_budget_usd: budgetNum,
        }),
      );
      setName("");
      reload();
    } catch (err) {
      setCreateError(errorMessage(err));
    } finally {
      setCreating(false);
    }
  };

  const orgs = data ?? [];

  return (
    <>
      <PageHead
        title="Orgs"
        subtitle="Tenants of the gateway. Each org caps the total spend of its keys and owns its users. Root admin only."
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
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Id</th>
                    <th className="num">Monthly budget</th>
                    <th className="num">Aggregate spend</th>
                    <th>Budget used</th>
                  </tr>
                </thead>
                <tbody>
                  {orgs.map((o) => {
                    const frac = budgetFraction(o.spend_usd, o.monthly_budget_usd);
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
                      </tr>
                    );
                  })}
                  {orgs.length === 0 && !loading && (
                    <tr>
                      <td colSpan={5} className="empty">
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
