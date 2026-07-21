import { useState } from "react";
import { SSO_LOGIN_URL } from "../lib/api";
import type { AuthRole } from "../lib/rbac";
import { roleLabel } from "../lib/rbac";
import { ErrorNotice } from "./common";

interface Props {
  adminKey: string;
  role: AuthRole;
  orgId: string;
  email: string;
  ssoEnabled: boolean;
  identityError: string | null;
  onSave: (key: string) => void;
  onClose: () => void;
}

export function SettingsModal({
  adminKey,
  role,
  orgId,
  email,
  ssoEnabled,
  identityError,
  onSave,
  onClose,
}: Props) {
  const [mode, setMode] = useState<"root" | "user">(role === "root" ? "root" : "user");
  const [value, setValue] = useState(adminKey);

  const save = () => onSave(value.trim());

  // Once whoami has resolved a token, show the identity the gateway reports —
  // read-only. The console no longer asks the caller to type their role/org.
  const resolved = adminKey && !identityError;
  const identityLine =
    role === "root"
      ? "Root admin · global superuser"
      : [email, roleLabel(role), orgId].filter(Boolean).join(" · ");

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h2>Settings</h2>
        <p>
          Authenticate calls to the gateway admin API. Use the{" "}
          <strong>root admin key</strong> (global superuser) or a{" "}
          <strong>user token</strong> (<code>agu-…</code>) issued from the Users page. Your role and
          org are read from the gateway (<code>whoami</code>) — no need to enter them. The token is
          sent as a Bearer credential and stored in this browser only.
        </p>

        {identityError && <ErrorNotice error={identityError} />}

        {resolved && identityLine && (
          <div className="secret-reveal" style={{ marginBottom: 16 }}>
            <strong>Signed in</strong>
            <div className="muted mono" style={{ marginTop: 6 }}>
              {identityLine}
            </div>
          </div>
        )}

        {ssoEnabled && (
          <>
            <button
              className="btn primary"
              style={{ width: "100%" }}
              onClick={() => window.location.assign(SSO_LOGIN_URL)}
            >
              Sign in with SSO
            </button>
            <p className="muted" style={{ fontSize: "12px", margin: "10px 0 16px" }}>
              Redirects to your identity provider and returns with a user token issued for you.
            </p>
          </>
        )}

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
          <p className="muted" style={{ fontSize: "12px", marginBottom: 0 }}>
            Your role and org come from the token itself — the gateway enforces your real
            permissions and answers 403 if you exceed them.
          </p>
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
