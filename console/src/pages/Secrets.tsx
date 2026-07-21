import type { PageProps } from "../App";
import { ErrorNotice, ForbiddenNotice, NeedsKey, PageHead, useLoad } from "../components/common";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
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
            <button className="btn small" onClick={reload} disabled={loading}>
              {loading ? "Loading…" : "Refresh"}
            </button>
          </div>
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
