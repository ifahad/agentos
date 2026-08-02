import { useState } from "react";
import type { PageProps } from "../App";
import { ErrorNotice, ForbiddenNotice, NeedsKey, PageHead, useLoad } from "../components/common";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
import { formatTimestamp } from "../lib/format";
import { activeBadge, formatExternalId } from "../lib/provisioning";
import { can } from "../lib/rbac";
import type { Org, User } from "../lib/types";
import { Badge, Button, EmptyState, Panel, PanelHead, Select, Skeleton, Table, Tbody, Tr } from "../ui";

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
  const showSkeleton = usersLoad.loading && users.length === 0 && !!activeOrg;

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

      {(!adminKey || allowed) && (
        <>
          <Panel>
            <PanelHead title="How provisioning works" />
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
          </Panel>

          <div className="thread-line">
            <span>Org</span>
            <Select value={activeOrg} onChange={(e) => setChosen(e.target.value)} aria-label="Org">
              {orgs.length === 0 && <option value="">no orgs</option>}
              {orgs.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.name} ({o.id})
                </option>
              ))}
            </Select>
          </div>

          <Panel>
            <PanelHead title="Provisioned users" />
            <ErrorNotice error={usersLoad.error} />
            {!adminKey ? (
              <EmptyState
                title="No admin key configured"
                description="The roster your identity provider has synced over SCIM appears here once the console can reach the admin API."
                action={
                  <Button variant="primary" onClick={openSettings}>
                    Open settings
                  </Button>
                }
              />
            ) : (
              <Table>
                <thead>
                  <tr>
                    <th>Email</th>
                    <th>Role</th>
                    <th>Status</th>
                    <th>Source</th>
                    <th>Created</th>
                  </tr>
                </thead>
                <Tbody staggerKey={`${activeOrg}:${users.length}`}>
                  {showSkeleton &&
                    Array.from({ length: 3 }, (_, i) => (
                      <Tr key={`skel-${i}`} animate={false}>
                        <td><Skeleton width={180} /></td>
                        <td><Skeleton width={56} /></td>
                        <td><Skeleton width={56} /></td>
                        <td><Skeleton width={80} /></td>
                        <td><Skeleton width={120} /></td>
                      </Tr>
                    ))}
                  {users.map((u) => {
                    const b = activeBadge(u.active);
                    return (
                      <Tr key={u.id}>
                        <td>{u.email}</td>
                        <td>
                          <Badge>{u.role}</Badge>
                        </td>
                        <td>
                          <Badge variant={u.active ? "pass" : "inactive"}>{b.label}</Badge>
                        </td>
                        <td className="dim">{formatExternalId(u.external_id)}</td>
                        <td className="dim mono">{formatTimestamp(u.created_at)}</td>
                      </Tr>
                    );
                  })}
                  {users.length === 0 && !usersLoad.loading && (
                    <Tr animate={false}>
                      <td colSpan={5} className="empty">
                        {activeOrg ? "No users in this org yet." : "Select an org to list its users."}
                      </td>
                    </Tr>
                  )}
                </Tbody>
              </Table>
            )}
          </Panel>
        </>
      )}
    </>
  );
}
