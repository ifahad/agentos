import { useState } from "react";
import type { PageProps } from "../App";
import { ErrorNotice, ForbiddenNotice, NeedsKey, PageHead, useLoad } from "../components/common";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
import { formatTimestamp } from "../lib/format";
import { activeBadge, formatExternalId } from "../lib/provisioning";
import { can } from "../lib/rbac";
import type { Org, User } from "../lib/types";

// SCIM 2.0 provisioning view. There is no dedicated status endpoint that
// reports whether SCIM is enabled — that is an IdP-side concern — so this page
// explains the model and renders a READ-ONLY roster of a selected org's users
// (with their active state and whether they are IdP-managed). Users are created
// and deactivated in the identity provider, never here.
export function Provisioning({ adminKey, role, openSettings }: PageProps) {
  const allowed = can(role, "provisioning.view");

  const orgsLoad = useLoad(
    () =>
      adminKey && allowed
        ? apiFetch<Org[]>(gatewayAdminRequest("/admin/orgs", adminKey))
        : Promise.resolve<Org[]>([]),
    [adminKey, allowed],
  );

  const orgs = orgsLoad.data ?? [];
  const [chosen, setChosen] = useState("");
  const activeOrg = chosen || orgs[0]?.id || "";

  const usersLoad = useLoad(
    () =>
      adminKey && allowed && activeOrg
        ? apiFetch<User[]>(gatewayAdminRequest(`/admin/orgs/${activeOrg}/users`, adminKey))
        : Promise.resolve<User[]>([]),
    [adminKey, allowed, activeOrg],
  );

  const users = usersLoad.data ?? [];

  return (
    <>
      <PageHead
        title="Provisioning"
        subtitle="SCIM 2.0 user provisioning — a read-only view of who your identity provider has synced. Root admin only."
      />
      {!adminKey && <NeedsKey openSettings={openSettings} />}
      {adminKey && !allowed && (
        <ForbiddenNotice message="Provisioning is visible to the root admin only." />
      )}
      <ErrorNotice error={orgsLoad.error} />

      {adminKey && allowed && (
        <>
          <div className="panel">
            <div className="panel-head">
              <h2>How provisioning works</h2>
            </div>
            <div className="panel-body">
              <p className="muted">
                Users are provisioned by your identity provider (Okta, Entra, …) over SCIM 2.0 at{" "}
                <span className="mono">/scim/v2/</span>. The IdP creates users, updates their
                attributes, and deactivates them; deactivated users keep their record but their{" "}
                <span className="mono">agu-</span> tokens stop working immediately.
              </p>
              <p className="muted">
                SCIM is enabled on the gateway by setting <span className="mono">AGENTOS_SCIM_TOKEN</span>;
                its enabled state is not exposed to the console. There is nothing to create or delete
                here — manage the roster in your identity provider.
              </p>
            </div>
          </div>

          <div className="thread-line">
            <span>Org</span>
            <select value={activeOrg} onChange={(e) => setChosen(e.target.value)}>
              {orgs.length === 0 && <option value="">no orgs</option>}
              {orgs.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.name} ({o.id})
                </option>
              ))}
            </select>
          </div>

          <div className="panel">
            <div className="panel-head">
              <h2>Provisioned users</h2>
              {usersLoad.loading && <span className="spin">loading…</span>}
            </div>
            <ErrorNotice error={usersLoad.error} />
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Email</th>
                    <th>Role</th>
                    <th>Status</th>
                    <th>Source</th>
                    <th>Created</th>
                  </tr>
                </thead>
                <tbody>
                  {users.map((u) => {
                    const b = activeBadge(u.active);
                    return (
                      <tr key={u.id}>
                        <td>{u.email}</td>
                        <td>
                          <span className="badge">{u.role}</span>
                        </td>
                        <td>
                          <span className={b.className}>{b.label}</span>
                        </td>
                        <td className="dim">{formatExternalId(u.external_id)}</td>
                        <td className="dim mono">{formatTimestamp(u.created_at)}</td>
                      </tr>
                    );
                  })}
                  {users.length === 0 && !usersLoad.loading && (
                    <tr>
                      <td colSpan={5} className="empty">
                        {activeOrg ? "No users in this org yet." : "Select an org to list its users."}
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
