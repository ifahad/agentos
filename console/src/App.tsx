import { useCallback, useEffect, useState } from "react";
import { SettingsModal } from "./components/SettingsModal";
import { errorMessage } from "./components/common";
import {
  apiFetch,
  getAdminKey,
  getStoredEmail,
  getStoredOrgId,
  getStoredRole,
  oidcStatusRequest,
  saveAdminKey,
  saveStoredEmail,
  saveStoredOrgId,
  saveStoredRole,
  whoamiRequest,
} from "./lib/api";
import type { AuthRole } from "./lib/rbac";
import { asAuthRole, can, roleLabel } from "./lib/rbac";
import { identityFromWhoAmI, parseAuthFragment } from "./lib/sso";
import type { WhoAmI } from "./lib/types";
import { Audit } from "./pages/Audit";
import { Documents } from "./pages/Documents";
import { Improve } from "./pages/Improve";
import { Keys } from "./pages/Keys";
import { Orgs } from "./pages/Orgs";
import { Overview } from "./pages/Overview";
import { Playground } from "./pages/Playground";
import { Provisioning } from "./pages/Provisioning";
import { Secrets } from "./pages/Secrets";
import { Users } from "./pages/Users";

export interface PageProps {
  adminKey: string;
  role: AuthRole;
  orgId: string;
  openSettings: () => void;
}

interface Route {
  path: string;
  label: string;
  Component: (props: PageProps) => React.JSX.Element;
  // When set, the nav item is shown only if the predicate holds for the role.
  visible?: (role: AuthRole) => boolean;
}

const ROUTES: Route[] = [
  { path: "/", label: "Overview", Component: Overview },
  { path: "/keys", label: "Keys", Component: Keys },
  { path: "/audit", label: "Audit", Component: Audit },
  { path: "/playground", label: "Playground", Component: Playground },
  { path: "/documents", label: "Documents", Component: Documents },
  { path: "/improve", label: "Improve", Component: Improve },
  { path: "/orgs", label: "Orgs", Component: Orgs, visible: (r) => can(r, "org.view") },
  { path: "/users", label: "Users", Component: Users, visible: (r) => can(r, "user.view") },
  { path: "/secrets", label: "Secrets", Component: Secrets, visible: (r) => can(r, "secret.view") },
  {
    path: "/provisioning",
    label: "Provisioning",
    Component: Provisioning,
    visible: (r) => can(r, "provisioning.view"),
  },
];

function usePath(): [string, (p: string) => void] {
  const [path, setPath] = useState(window.location.pathname);
  useEffect(() => {
    const onPop = () => setPath(window.location.pathname);
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);
  const navigate = useCallback((p: string) => {
    window.history.pushState({}, "", p);
    setPath(p);
  }, []);
  return [path, navigate];
}

export function App() {
  const [path, navigate] = usePath();
  const [adminKey, setAdminKey] = useState(getAdminKey);
  // role / orgId / email are DERIVED from whoami, not manual entry. They start
  // from the last resolved values (persisted) and refresh whenever the token
  // changes; whoami is authoritative.
  const [role, setRole] = useState<AuthRole>(() => asAuthRole(getStoredRole()));
  const [orgId, setOrgId] = useState(getStoredOrgId);
  const [email, setEmail] = useState(getStoredEmail);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [ssoEnabled, setSsoEnabled] = useState(false);
  const [identityError, setIdentityError] = useState<string | null>(null);

  const route = ROUTES.find((r) => r.path === path) ?? ROUTES[0];
  const openSettings = useCallback(() => setSettingsOpen(true), []);
  const visibleRoutes = ROUTES.filter((r) => !r.visible || r.visible(role));

  // Resolve the caller's identity from the gateway. On success the UI is driven
  // by the real role/org/email; on failure (401) we prompt for credentials.
  const refreshIdentity = useCallback(async (key: string) => {
    if (!key) {
      setRole("root");
      setOrgId("");
      setEmail("");
      saveStoredRole("");
      saveStoredOrgId("");
      saveStoredEmail("");
      setIdentityError(null);
      return;
    }
    try {
      const who = await apiFetch<WhoAmI>(whoamiRequest(key));
      const id = identityFromWhoAmI(who);
      setRole(id.role);
      setOrgId(id.orgId);
      setEmail(id.email);
      saveStoredRole(id.role);
      saveStoredOrgId(id.orgId);
      saveStoredEmail(id.email);
      setIdentityError(null);
    } catch (err) {
      setIdentityError(errorMessage(err));
      setSettingsOpen(true);
    }
  }, []);

  // On load: (1) pick up an SSO token from the URL fragment and clear it,
  // (2) probe whether SSO is enabled, (3) resolve identity from any token.
  useEffect(() => {
    const frag = parseAuthFragment(window.location.hash);
    let key = getAdminKey();
    if (frag.token) {
      saveAdminKey(frag.token);
      key = frag.token;
      setAdminKey(frag.token);
      history.replaceState(null, "", window.location.pathname + window.location.search);
    }
    apiFetch<{ enabled: boolean }>(oidcStatusRequest())
      .then((s) => setSsoEnabled(Boolean(s.enabled)))
      .catch(() => setSsoEnabled(false));
    if (key) void refreshIdentity(key);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-name">AgentOS</span>
          <span className="brand-tag">console</span>
        </div>
        <nav className="nav">
          {visibleRoutes.map((r) => (
            <button
              key={r.path}
              className={`nav-item${r.path === route.path ? " active" : ""}`}
              onClick={() => navigate(r.path)}
            >
              {r.label}
            </button>
          ))}
        </nav>
        <div className="sidebar-footer">
          <button className="nav-item" onClick={openSettings}>
            Settings
            {adminKey ? (
              <span className="dim"> · {roleLabel(role).toLowerCase()}</span>
            ) : (
              <span style={{ color: "var(--amber)" }}> · no key</span>
            )}
          </button>
        </div>
      </aside>
      <main className="main">
        <div className="page">
          <route.Component adminKey={adminKey} role={role} orgId={orgId} openSettings={openSettings} />
        </div>
      </main>
      {settingsOpen && (
        <SettingsModal
          adminKey={adminKey}
          role={role}
          orgId={orgId}
          email={email}
          ssoEnabled={ssoEnabled}
          identityError={identityError}
          onSave={(k) => {
            saveAdminKey(k);
            setAdminKey(k);
            setSettingsOpen(false);
            void refreshIdentity(k);
          }}
          onClose={() => setSettingsOpen(false)}
        />
      )}
    </div>
  );
}
