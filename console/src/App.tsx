import { useCallback, useEffect, useState } from "react";
import { SettingsModal } from "./components/SettingsModal";
import { getAdminKey, getStoredOrgId, getStoredRole } from "./lib/api";
import type { AuthRole } from "./lib/rbac";
import { asAuthRole, can, roleLabel } from "./lib/rbac";
import { Audit } from "./pages/Audit";
import { Documents } from "./pages/Documents";
import { Improve } from "./pages/Improve";
import { Keys } from "./pages/Keys";
import { Orgs } from "./pages/Orgs";
import { Overview } from "./pages/Overview";
import { Playground } from "./pages/Playground";
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
  const [role, setRole] = useState<AuthRole>(() => asAuthRole(getStoredRole()));
  const [orgId, setOrgId] = useState(getStoredOrgId);
  const [settingsOpen, setSettingsOpen] = useState(false);

  const route = ROUTES.find((r) => r.path === path) ?? ROUTES[0];
  const openSettings = useCallback(() => setSettingsOpen(true), []);
  const visibleRoutes = ROUTES.filter((r) => !r.visible || r.visible(role));

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
          onSave={(k, r, o) => {
            setAdminKey(k);
            setRole(r);
            setOrgId(o);
            setSettingsOpen(false);
          }}
          onClose={() => setSettingsOpen(false)}
        />
      )}
    </div>
  );
}
