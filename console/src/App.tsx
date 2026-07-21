import { useCallback, useEffect, useState } from "react";
import { SettingsModal } from "./components/SettingsModal";
import { getAdminKey } from "./lib/api";
import { Audit } from "./pages/Audit";
import { Documents } from "./pages/Documents";
import { Improve } from "./pages/Improve";
import { Keys } from "./pages/Keys";
import { Overview } from "./pages/Overview";
import { Playground } from "./pages/Playground";

export interface PageProps {
  adminKey: string;
  openSettings: () => void;
}

interface Route {
  path: string;
  label: string;
  Component: (props: PageProps) => React.JSX.Element;
}

const ROUTES: Route[] = [
  { path: "/", label: "Overview", Component: Overview },
  { path: "/keys", label: "Keys", Component: Keys },
  { path: "/audit", label: "Audit", Component: Audit },
  { path: "/playground", label: "Playground", Component: Playground },
  { path: "/documents", label: "Documents", Component: Documents },
  { path: "/improve", label: "Improve", Component: Improve },
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
  const [settingsOpen, setSettingsOpen] = useState(false);

  const route = ROUTES.find((r) => r.path === path) ?? ROUTES[0];
  const openSettings = useCallback(() => setSettingsOpen(true), []);

  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-name">AgentOS</span>
          <span className="brand-tag">console</span>
        </div>
        <nav className="nav">
          {ROUTES.map((r) => (
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
            {!adminKey && <span style={{ color: "var(--amber)" }}> · no key</span>}
          </button>
        </div>
      </aside>
      <main className="main">
        <div className="page">
          <route.Component adminKey={adminKey} openSettings={openSettings} />
        </div>
      </main>
      {settingsOpen && (
        <SettingsModal
          adminKey={adminKey}
          onSave={(k) => {
            setAdminKey(k);
            setSettingsOpen(false);
          }}
          onClose={() => setSettingsOpen(false)}
        />
      )}
    </div>
  );
}
