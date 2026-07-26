import { useState } from "react";
import type { PageProps } from "../App";
import {
  CopyButton,
  ErrorNotice,
  ForbiddenNotice,
  NeedsKey,
  PageHead,
  errorMessage,
  useLoad,
} from "../components/common";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
import { formatTimestamp } from "../lib/format";
import { activeBadge } from "../lib/provisioning";
import { ROLES, can, roleLabel } from "../lib/rbac";
import type { CreatedUser, Org, Role, User } from "../lib/types";
import { Badge, Button, Input, Panel, PanelHead, Select, Skeleton, Table, Tbody, Tr, useToast } from "../ui";

export function Users({ adminKey, role, orgId, openSettings }: PageProps) {
  const allowed = can(role, "user.view");
  const isRoot = role === "root";
  const toast = useToast();

  // Root can enumerate every org and pick one; a user-token caller is scoped to
  // their own org (resolved from whoami — the gateway rejects any other).
  const orgsLoad = useLoad(
    () =>
      adminKey && allowed && isRoot
        ? apiFetch<Org[]>(gatewayAdminRequest("/admin/orgs", adminKey))
        : Promise.resolve<Org[]>([]),
    [adminKey, allowed, isRoot],
  );

  const orgs = orgsLoad.data ?? [];
  const [chosen, setChosen] = useState("");
  const activeOrg = isRoot ? chosen || orgs[0]?.id || "" : orgId;

  const usersLoad = useLoad(
    () =>
      adminKey && allowed && activeOrg
        ? apiFetch<User[]>(gatewayAdminRequest(`/admin/orgs/${activeOrg}/users`, adminKey))
        : Promise.resolve<User[]>([]),
    [adminKey, allowed, activeOrg],
  );

  const [email, setEmail] = useState("");
  const [inviteRole, setInviteRole] = useState<Role>("member");
  const [inviting, setInviting] = useState(false);
  const [created, setCreated] = useState<CreatedUser | null>(null);
  const [removing, setRemoving] = useState<string | null>(null);

  const canInvite = can(role, "user.invite");
  const canRemove = can(role, "user.remove");

  const invite = async () => {
    if (!email.trim()) {
      toast.error("An email address is required.");
      return;
    }
    setInviting(true);
    try {
      const res = await apiFetch<CreatedUser>(
        gatewayAdminRequest(`/admin/orgs/${activeOrg}/users`, adminKey, {
          email: email.trim(),
          role: inviteRole,
        }),
      );
      setCreated(res);
      setEmail("");
      toast.success(`Invited ${res.email} as ${res.role}.`);
      usersLoad.reload();
    } catch (err) {
      toast.error(errorMessage(err));
    } finally {
      setInviting(false);
    }
  };

  const remove = async (user: User) => {
    if (!window.confirm(`Remove ${user.email} (${user.role}) from this org? This revokes their token.`)) {
      return;
    }
    setRemoving(user.id);
    try {
      await apiFetch<unknown>(
        gatewayAdminRequest(`/admin/orgs/${activeOrg}/users/${user.id}`, adminKey, undefined, {
          method: "DELETE",
        }),
      );
      toast.success(`Removed ${user.email}.`);
      usersLoad.reload();
    } catch (err) {
      toast.error(errorMessage(err));
    } finally {
      setRemoving(null);
    }
  };

  const users = usersLoad.data ?? [];
  const showSkeleton = usersLoad.loading && users.length === 0 && !!activeOrg;

  return (
    <>
      <PageHead
        title="Users"
        subtitle="Members of an org and their roles. Inviting a user issues a one-time agu- token; removing one revokes it."
      />
      {!adminKey && <NeedsKey openSettings={openSettings} />}
      {adminKey && !allowed && (
        <ForbiddenNotice message="Your role can't view org members. Owner, admin, or member is required." />
      )}
      <ErrorNotice error={orgsLoad.error} />

      {adminKey && allowed && (
        <>
          <div className="thread-line">
            {isRoot ? (
              <>
                <span>Org</span>
                <Select value={activeOrg} onChange={(e) => setChosen(e.target.value)} aria-label="Org">
                  {orgs.length === 0 && <option value="">no orgs</option>}
                  {orgs.map((o) => (
                    <option key={o.id} value={o.id}>
                      {o.name} ({o.id})
                    </option>
                  ))}
                </Select>
              </>
            ) : (
              <span>
                Org <span className="mono">{activeOrg || "— resolving your identity…"}</span>
              </span>
            )}
          </div>

          {created && (
            <div className="secret-reveal">
              <strong>
                Invited <span className="mono">{created.email}</span> as {created.role}
              </strong>
              <div className="secret">
                <code>{created.token}</code>
                <CopyButton text={created.token} />
              </div>
              <div className="muted">
                Give this token to the user now — it will not be shown again.{" "}
                <Button small iconOnly icon="close" aria-label="Dismiss" onClick={() => setCreated(null)} />
              </div>
            </div>
          )}

          {canInvite && (
            <Panel>
              <PanelHead title="Invite user" />
              <div className="panel-body">
                <div className="form-row">
                  <Input
                    label="Email"
                    type="text"
                    value={email}
                    placeholder="person@acme.com"
                    onChange={(e) => setEmail(e.target.value)}
                  />
                  <Select
                    label="Role"
                    value={inviteRole}
                    onChange={(e) => setInviteRole(e.target.value as Role)}
                  >
                    {ROLES.map((r) => (
                      <option key={r} value={r}>
                        {roleLabel(r)}
                      </option>
                    ))}
                  </Select>
                </div>
                <Button
                  variant="primary"
                  icon="plus"
                  onClick={() => void invite()}
                  disabled={inviting || !activeOrg}
                >
                  {inviting ? "Inviting…" : "Invite user"}
                </Button>
              </div>
            </Panel>
          )}

          <Panel>
            <PanelHead title="Members" />
            <ErrorNotice error={usersLoad.error} />
            <Table>
              <thead>
                <tr>
                  <th>Email</th>
                  <th>Role</th>
                  <th>Status</th>
                  <th>Created</th>
                  {canRemove && <th></th>}
                </tr>
              </thead>
              <Tbody staggerKey={`${activeOrg}:${users.length}`}>
                {showSkeleton &&
                  Array.from({ length: 3 }, (_, i) => (
                    <Tr key={`skel-${i}`} animate={false}>
                      <td><Skeleton width={180} /></td>
                      <td><Skeleton width={56} /></td>
                      <td><Skeleton width={56} /></td>
                      <td><Skeleton width={120} /></td>
                      {canRemove && <td className="num"><Skeleton width={64} /></td>}
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
                      <td className="dim mono">{formatTimestamp(u.created_at)}</td>
                      {canRemove && (
                        <td className="num">
                          <Button
                            small
                            variant="danger"
                            onClick={() => void remove(u)}
                            disabled={removing === u.id}
                          >
                            {removing === u.id ? "Removing…" : "Remove"}
                          </Button>
                        </td>
                      )}
                    </Tr>
                  );
                })}
                {users.length === 0 && !usersLoad.loading && (
                  <Tr animate={false}>
                    <td colSpan={canRemove ? 5 : 4} className="empty">
                      {activeOrg ? "No users in this org yet." : "Select an org to list its users."}
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
