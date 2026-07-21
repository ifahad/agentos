import { useState } from "react";
import type { PageProps } from "../App";
import { CopyButton, ErrorNotice, NeedsKey, PageHead, errorMessage, useLoad } from "../components/common";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
import { budgetFraction, formatUSD } from "../lib/format";
import { can } from "../lib/rbac";
import type { CreatedKey, KeyInfo } from "../lib/types";

export function Keys({ adminKey, role, openSettings }: PageProps) {
  const canCreate = can(role, "key.create");
  const { data, error, loading, reload } = useLoad(
    () =>
      adminKey
        ? apiFetch<KeyInfo[]>(gatewayAdminRequest("/admin/keys", adminKey))
        : Promise.resolve<KeyInfo[]>([]),
    [adminKey],
  );

  const [name, setName] = useState("");
  const [budget, setBudget] = useState("25");
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [created, setCreated] = useState<CreatedKey | null>(null);

  const create = async () => {
    const budgetNum = Number(budget);
    if (!name.trim() || !Number.isFinite(budgetNum) || budgetNum < 0) {
      setCreateError("A name and a non-negative monthly budget are required.");
      return;
    }
    setCreating(true);
    setCreateError(null);
    try {
      const res = await apiFetch<CreatedKey>(
        gatewayAdminRequest("/admin/keys", adminKey, {
          name: name.trim(),
          monthly_budget_usd: budgetNum,
        }),
      );
      setCreated(res);
      setName("");
      reload();
    } catch (err) {
      setCreateError(errorMessage(err));
    } finally {
      setCreating(false);
    }
  };

  const keys = data ?? [];

  return (
    <>
      <PageHead
        title="Keys"
        subtitle="Virtual gateway keys with monthly budgets. Secrets are shown exactly once at creation."
      />
      {!adminKey && <NeedsKey openSettings={openSettings} />}
      <ErrorNotice error={error} />

      {created && (
        <div className="secret-reveal">
          <strong>
            Key <span className="mono">{created.name}</span> created
          </strong>
          <div className="secret">
            <code>{created.key}</code>
            <CopyButton text={created.key} />
          </div>
          <div className="muted">
            Store this secret now — it will not be shown again.{" "}
            <a
              href="#dismiss"
              onClick={(e) => {
                e.preventDefault();
                setCreated(null);
              }}
            >
              Dismiss
            </a>
          </div>
        </div>
      )}

      {adminKey && (
        <>
          {canCreate && (
          <div className="panel">
            <div className="panel-head">
              <h2>Create key</h2>
            </div>
            <div className="panel-body">
              <ErrorNotice error={createError} />
              <div className="form-row">
                <label className="field">
                  <span>Name</span>
                  <input
                    type="text"
                    value={name}
                    placeholder="team-analytics"
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
                {creating ? "Creating…" : "Create key"}
              </button>
            </div>
          </div>
          )}

          <div className="panel">
            <div className="panel-head">
              <h2>Existing keys</h2>
              {loading && <span className="spin">loading…</span>}
            </div>
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Name</th>
                    <th className="num">Monthly budget</th>
                    <th className="num">Spend</th>
                    <th>Budget used</th>
                  </tr>
                </thead>
                <tbody>
                  {keys.map((k) => {
                    const frac = budgetFraction(k.spend_usd, k.monthly_budget_usd);
                    return (
                      <tr key={k.name}>
                        <td className="mono">{k.name}</td>
                        <td className="num">{formatUSD(k.monthly_budget_usd)}</td>
                        <td className="num">{formatUSD(k.spend_usd)}</td>
                        <td>
                          <span className={`meter${frac >= 0.9 ? " hot" : ""}`}>
                            <div style={{ width: `${Math.round(frac * 100)}%` }} />
                          </span>{" "}
                          <span className="dim muted">{Math.round(frac * 100)}%</span>
                        </td>
                      </tr>
                    );
                  })}
                  {keys.length === 0 && !loading && (
                    <tr>
                      <td colSpan={4} className="empty">
                        No keys yet — create one above.
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
