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
import { ROLES, can, roleLabel } from "../lib/rbac";
import type { CreatedUser, Org, Role, User } from "../lib/types";

export function Users({ adminKey, role, orgId, openSettings }: PageProps) {
  const allowed = can(role, "user.view");
  const isRoot = role === "root";

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
  const [inviteError, setInviteError] = useState<string | null>(null);
  const [created, setCreated] = useState<CreatedUser | null>(null);
  const [removing, setRemoving] = useState<string | null>(null);

  const canInvite = can(role, "user.invite");
  const canRemove = can(role, "user.remove");

  const invite = async () => {
    if (!email.trim()) {
      setInviteError("An email address is required.");
      return;
    }
    setInviting(true);
    setInviteError(null);
    try {
      const res = await apiFetch<CreatedUser>(
        gatewayAdminRequest(`/admin/orgs/${activeOrg}/users`, adminKey, {
          email: email.trim(),
          role: inviteRole,
        }),
      );
      setCreated(res);
      setEmail("");
      usersLoad.reload();
    } catch (err) {
      setInviteError(errorMessage(err));
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
      usersLoad.reload();
    } catch (err) {
      window.alert(errorMessage(err));
    } finally {
      setRemoving(null);
    }
  };

  const users = usersLoad.data ?? [];

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
                <select value={activeOrg} onChange={(e) => setChosen(e.target.value)}>
                  {orgs.length === 0 && <option value="">no orgs</option>}
                  {orgs.map((o) => (
                    <option key={o.id} value={o.id}>
                      {o.name} ({o.id})
                    </option>
                  ))}
                </select>
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

          {canInvite && (
            <div className="panel">
              <div className="panel-head">
                <h2>Invite user</h2>
              </div>
              <div className="panel-body">
                <ErrorNotice error={inviteError} />
                <div className="form-row">
                  <label className="field">
                    <span>Email</span>
                    <input
                      type="text"
                      value={email}
                      placeholder="person@acme.com"
                      onChange={(e) => setEmail(e.target.value)}
                    />
                  </label>
                  <label className="field">
                    <span>Role</span>
                    <select value={inviteRole} onChange={(e) => setInviteRole(e.target.value as Role)}>
                      {ROLES.map((r) => (
                        <option key={r} value={r}>
                          {roleLabel(r)}
                        </option>
                      ))}
                    </select>
                  </label>
                </div>
                <button
                  className="btn primary"
                  onClick={() => void invite()}
                  disabled={inviting || !activeOrg}
                >
                  {inviting ? "Inviting…" : "Invite user"}
                </button>
              </div>
            </div>
          )}

          <div className="panel">
            <div className="panel-head">
              <h2>Members</h2>
              {usersLoad.loading && <span className="spin">loading…</span>}
            </div>
            <ErrorNotice error={usersLoad.error} />
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Email</th>
                    <th>Role</th>
                    <th>Created</th>
                    {canRemove && <th></th>}
                  </tr>
                </thead>
                <tbody>
                  {users.map((u) => (
                    <tr key={u.id}>
                      <td>{u.email}</td>
                      <td>
                        <span className="badge">{u.role}</span>
                      </td>
                      <td className="dim mono">{formatTimestamp(u.created_at)}</td>
                      {canRemove && (
                        <td className="num">
                          <button
                            className="btn small danger"
                            onClick={() => void remove(u)}
                            disabled={removing === u.id}
                          >
                            {removing === u.id ? "Removing…" : "Remove"}
                          </button>
                        </td>
                      )}
                    </tr>
                  ))}
                  {users.length === 0 && !usersLoad.loading && (
                    <tr>
                      <td colSpan={canRemove ? 4 : 3} className="empty">
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
