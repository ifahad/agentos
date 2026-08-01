import { useEffect, useState } from "react";
import { SSO_LOGIN_URL } from "../lib/api";
import type { AuthRole } from "../lib/rbac";
import { roleLabel } from "../lib/rbac";
import { Button, Input, Modal } from "../ui";
import { ErrorNotice } from "./common";

interface Props {
  open: boolean;
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
  open,
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
  const [theme, setThemeState] = useState<"dark" | "light">(() => {
    const attr = document.documentElement.getAttribute("data-theme") as "dark" | "light" | null;
    if (attr === "dark" || attr === "light") return attr;
    // No explicit choice stored: fall back to the same media query the
    // pre-paint bootstrap in index.html defers to, so the toggle's pressed
    // state always matches what the console is actually rendering.
    return window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
  });

  function setTheme(next: "dark" | "light") {
    document.documentElement.setAttribute("data-theme", next);
    try {
      localStorage.setItem("agentos-theme", next);
    } catch {
      // Privacy mode: the choice applies for this session only.
    }
    setThemeState(next);
  }

  // The modal now stays mounted between opens (so the Modal primitive's exit
  // animation can run) — reset the form to the current credentials each time
  // it opens, matching the previous mount-per-open behavior.
  useEffect(() => {
    if (open) {
      setValue(adminKey);
      setMode(role === "root" ? "root" : "user");
    }
  }, [open, adminKey, role]);

  const save = () => onSave(value.trim());

  // Once whoami has resolved a token, show the identity the gateway reports —
  // read-only. The console no longer asks the caller to type their role/org.
  const resolved = adminKey && !identityError;
  const identityLine =
    role === "root"
      ? "Root admin · global superuser"
      : [email, roleLabel(role), orgId].filter(Boolean).join(" · ");

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Settings"
      actions={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="primary" onClick={save}>
            Save
          </Button>
        </>
      }
    >
      <p>
        Authenticate calls to the gateway admin API. Use the <strong>root admin key</strong> (global
        superuser) or a <strong>user token</strong> (<code>agu-…</code>) issued from the Users page.
        Your role and org are read from the gateway (<code>whoami</code>) — no need to enter them.
        The token is sent as a Bearer credential and stored in this browser only.
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
          <Button
            variant="primary"
            style={{ width: "100%" }}
            onClick={() => window.location.assign(SSO_LOGIN_URL)}
          >
            Sign in with SSO
          </Button>
          <p className="muted" style={{ fontSize: "12px", margin: "10px 0 16px" }}>
            Redirects to your identity provider and returns with a user token issued for you.
          </p>
        </>
      )}

      <div className="field">
        <span className="eyebrow">Appearance</span>
        <div className="theme-toggle">
          {(["dark", "light"] as const).map((t) => (
            <button
              key={t}
              type="button"
              className={`btn small${theme === t ? " primary" : ""}`}
              aria-pressed={theme === t}
              onClick={() => setTheme(t)}
            >
              {t}
            </button>
          ))}
        </div>
      </div>

      <div className="mode-toggle">
        <Button small variant={mode === "root" ? "primary" : "ghost"} onClick={() => setMode("root")}>
          Root admin key
        </Button>
        <Button small variant={mode === "user" ? "primary" : "ghost"} onClick={() => setMode("user")}>
          User token
        </Button>
      </div>

      <Input
        label={mode === "root" ? "Gateway admin key" : "User token (agu-…)"}
        type="password"
        className="mono"
        value={value}
        placeholder={mode === "root" ? "AGENTOS_ADMIN_KEY" : "agu-…"}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => e.key === "Enter" && save()}
        autoFocus
      />
      {mode === "user" && (
        <p className="muted" style={{ fontSize: "12px", marginBottom: 0 }}>
          Your role and org come from the token itself — the gateway enforces your real permissions
          and answers 403 if you exceed them.
        </p>
      )}
    </Modal>
  );
}
