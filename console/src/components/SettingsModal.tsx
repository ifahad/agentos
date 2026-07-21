import { useState } from "react";
import { saveAdminKey, saveStoredOrgId, saveStoredRole } from "../lib/api";
import type { AuthRole } from "../lib/rbac";
import { ROLES, roleLabel } from "../lib/rbac";
import type { Role } from "../lib/types";

interface Props {
  adminKey: string;
  role: AuthRole;
  orgId: string;
  onSave: (key: string, role: AuthRole, orgId: string) => void;
  onClose: () => void;
}

export function SettingsModal({ adminKey, role, orgId, onSave, onClose }: Props) {
  const [mode, setMode] = useState<"root" | "user">(role === "root" ? "root" : "user");
  const [value, setValue] = useState(adminKey);
  const [userRole, setUserRole] = useState<Role>(role === "root" ? "member" : role);
  const [org, setOrg] = useState(orgId);

  const save = () => {
    const trimmed = value.trim();
    const nextRole: AuthRole = mode === "root" ? "root" : userRole;
    const nextOrg = mode === "root" ? "" : org.trim();
    saveAdminKey(trimmed);
    saveStoredRole(nextRole);
    saveStoredOrgId(nextOrg);
    onSave(trimmed, nextRole, nextOrg);
  };

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h2>Settings</h2>
        <p>
          Authenticate calls to the gateway admin API. Use the{" "}
          <strong>root admin key</strong> (global superuser) or a{" "}
          <strong>user token</strong> (<code>agu-…</code>) issued from the Users page. The token
          is sent as a Bearer credential and stored in this browser only.
        </p>

        <div className="mode-toggle">
          <button
            className={`btn small${mode === "root" ? " primary" : ""}`}
            onClick={() => setMode("root")}
          >
            Root admin key
          </button>
          <button
            className={`btn small${mode === "user" ? " primary" : ""}`}
            onClick={() => setMode("user")}
          >
            User token
          </button>
        </div>

        <label className="field">
          <span>{mode === "root" ? "Gateway admin key" : "User token (agu-…)"}</span>
          <input
            type="password"
            className="mono"
            value={value}
            placeholder={mode === "root" ? "AGENTOS_ADMIN_KEY" : "agu-…"}
            onChange={(e) => setValue(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && save()}
            autoFocus
          />
        </label>

        {mode === "user" && (
          <>
            <label className="field">
              <span>Your role</span>
              <select value={userRole} onChange={(e) => setUserRole(e.target.value as Role)}>
                {ROLES.map((r) => (
                  <option key={r} value={r}>
                    {roleLabel(r)}
                  </option>
                ))}
              </select>
            </label>
            <label className="field">
              <span>Your org id</span>
              <input
                type="text"
                className="mono"
                value={org}
                placeholder="org_default"
                onChange={(e) => setOrg(e.target.value)}
              />
            </label>
            <p className="muted" style={{ fontSize: "12px", marginBottom: 0 }}>
              The role only shapes what the console shows — the gateway enforces your real
              permissions and answers 403 if you exceed them.
            </p>
          </>
        )}

        <div className="modal-actions">
          <button className="btn" onClick={onClose}>
            Cancel
          </button>
          <button className="btn primary" onClick={save}>
            Save
          </button>
        </div>
      </div>
    </div>
  );
}
