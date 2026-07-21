import { useState } from "react";
import type { PageProps } from "../App";
import { ErrorNotice, ForbiddenNotice, NeedsKey, PageHead, errorMessage, useLoad } from "../components/common";
import { ApiError, apiFetch, gatewayAdminRequest, reloadSecretsRequest } from "../lib/api";
import { can } from "../lib/rbac";
import type { SecretStatus } from "../lib/types";

export function Secrets({ adminKey, role, openSettings }: PageProps) {
  const allowed = can(role, "secret.view");

  const { data, error, loading, reload } = useLoad(
    () =>
      adminKey && allowed
        ? apiFetch<SecretStatus[]>(gatewayAdminRequest("/admin/secrets/status", adminKey))
        : Promise.resolve<SecretStatus[]>([]),
    [adminKey, allowed],
  );

  const [reloading, setReloading] = useState(false);
  const [reloaded, setReloaded] = useState(false);
  const [reloadError, setReloadError] = useState<string | null>(null);
  const [forbidden, setForbidden] = useState(false);

  const reloadSecrets = async () => {
    setReloading(true);
    setReloaded(false);
    setReloadError(null);
    setForbidden(false);
    try {
      // The endpoint returns the fresh status array; refetch to reflect it.
      await apiFetch<SecretStatus[]>(reloadSecretsRequest(adminKey));
      reload();
      setReloaded(true);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setReloadError(errorMessage(err));
      }
    } finally {
      setReloading(false);
    }
  };

  const secrets = data ?? [];

  return (
    <>
      <PageHead
        title="Secrets"
        subtitle="Provider secrets resolved by the gateway — presence and backend only. Values are never exposed. Root admin only."
      />
      {!adminKey && <NeedsKey openSettings={openSettings} />}
      {adminKey && !allowed && <ForbiddenNotice message="Secret status is visible to the root admin only." />}
      <ErrorNotice error={error} />

      {adminKey && allowed && (
        <div className="panel">
          <div className="panel-head">
            <h2>Secret status</h2>
            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
              {reloaded && <span className="status-ok">reloaded</span>}
              <button className="btn small" onClick={() => void reloadSecrets()} disabled={reloading || loading}>
                {reloading ? "Reloading…" : "Reload secrets"}
              </button>
              <button className="btn small" onClick={reload} disabled={loading}>
                {loading ? "Loading…" : "Refresh"}
              </button>
            </div>
          </div>
          {forbidden && <ForbiddenNotice message="Reloading secrets is available to the root admin only." />}
          <ErrorNotice error={reloadError} />
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Present</th>
                  <th>Source</th>
                </tr>
              </thead>
              <tbody>
                {secrets.map((s) => (
                  <tr key={s.name}>
                    <td className="mono">{s.name}</td>
                    <td>
                      <span className={`badge ${s.present ? "pass" : "fail"}`}>
                        {s.present ? "present" : "missing"}
                      </span>
                    </td>
                    <td>
                      <span className="badge">{s.source}</span>
                    </td>
                  </tr>
                ))}
                {secrets.length === 0 && !loading && (
                  <tr>
                    <td colSpan={3} className="empty">
                      No secrets reported.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </>
  );
}
