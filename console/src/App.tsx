import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { useCallback, useEffect, useState } from "react";
import { Chain } from "./components/Chain";
import { SettingsModal } from "./components/SettingsModal";
import { Sidebar } from "./components/Sidebar";
import type { IconName } from "./ui/icons";
import { StateIcon } from "./ui/icons";
import { errorMessage } from "./components/common";
import { useConnectionState } from "./hooks/useLiveResource";
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
import { defaultRegistry, installVisibilityPause } from "./lib/live/registry";
import type { ConnectionState } from "./lib/live/status";
import type { AuthRole } from "./lib/rbac";
import { asAuthRole, can } from "./lib/rbac";
import { identityFromWhoAmI, parseAuthFragment } from "./lib/sso";
import type { WhoAmI } from "./lib/types";
import { ToastProvider, pageTransition } from "./ui";
import { Audit } from "./pages/Audit";
import { Documents } from "./pages/Documents";
import { Improve } from "./pages/Improve";
import { Multiverse } from "./pages/Multiverse";
import { Operators } from "./pages/Operators";
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
  navigate: (path: string) => void;
}

interface Route {
  path: string;
  label: string;
  icon: IconName;
  Component: (props: PageProps) => React.JSX.Element;
  // When set, the nav item is shown only if the predicate holds for the role.
  visible?: (role: AuthRole) => boolean;
}

const ROUTES: Route[] = [
  { path: "/", label: "Overview", icon: "overview", Component: Overview },
  { path: "/keys", label: "Keys", icon: "keys", Component: Keys },
  { path: "/audit", label: "Audit", icon: "audit", Component: Audit },
  { path: "/playground", label: "Playground", icon: "playground", Component: Playground },
  { path: "/documents", label: "Documents", icon: "documents", Component: Documents },
  { path: "/improve", label: "Improve", icon: "improve", Component: Improve },
  { path: "/multiverse", label: "Multiverse", icon: "multiverse", Component: Multiverse },
  { path: "/operators", label: "Operators", icon: "operators", Component: Operators },
  {
    path: "/orgs",
    label: "Orgs",
    icon: "orgs",
    Component: Orgs,
    visible: (r) => can(r, "org.view"),
  },
  {
    path: "/users",
    label: "Users",
    icon: "users",
    Component: Users,
    visible: (r) => can(r, "user.view"),
  },
  {
    path: "/secrets",
    label: "Secrets",
    icon: "secrets",
    Component: Secrets,
    visible: (r) => can(r, "secret.view"),
  },
  {
    path: "/provisioning",
    label: "Provisioning",
    icon: "provisioning",
    Component: Provisioning,
    visible: (r) => can(r, "provisioning.view"),
  },
];

/**
 * Connection state → topbar status glyph. `idle` (no key, or no traffic yet)
 * renders nothing: quiet must not look like a state, only a machine reading
 * (live/stale/offline) may speak.
 */
function connectionGlyph(conn: ConnectionState): { state: "live" | "hold" | "deny"; label: string } | null {
  switch (conn) {
    case "live":
      return { state: "live", label: "synced" };
    case "stale":
      return { state: "hold", label: "reconnecting" };
    case "offline":
      return { state: "deny", label: "offline" };
    case "idle":
      return null;
    default:
      return null;
  }
}

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
  const connection = useConnectionState();

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

  // Shell-wide: pause every registry poll while the tab is hidden, and refetch
  // on return, so a backgrounded tab does not keep hammering the gateway.
  useEffect(() => installVisibilityPause(defaultRegistry), []);

  const reducedMotion = useReducedMotion();
  const connGlyph = connectionGlyph(connection);
  const page = (
    <route.Component
      adminKey={adminKey}
      role={role}
      orgId={orgId}
      openSettings={openSettings}
      navigate={navigate}
    />
  );

  return (
    <ToastProvider>
      <div className="shell">
        {/* Keyboard users jump straight past the chrome to the panel. */}
        <a className="skip-link" href="#main-content">
          Skip to content
        </a>
        <Sidebar
          routes={visibleRoutes}
          activePath={route.path}
          onNavigate={navigate}
          adminKey={adminKey}
          role={role}
          openSettings={openSettings}
        />
        <main className="main" id="main-content">
          {/* The gauntlet every request runs, pinned above the content: it is the
              one thing true of the whole platform regardless of which page you
              are on. */}
          <header className="topbar">
            <span className="eyebrow topbar-legend">governance chain</span>
            {connGlyph && (
              <span className="topbar-status">
                <StateIcon state={connGlyph.state} size={12} />
                <span className="eyebrow">{connGlyph.label}</span>
              </span>
            )}
            <Chain adminKey={adminKey} />
          </header>
          {reducedMotion ? (
            <div className="page">{page}</div>
          ) : (
            <AnimatePresence mode="wait">
              <motion.div
                key={route.path}
                className="page"
                variants={pageTransition}
                initial="hidden"
                animate="show"
                exit="exit"
              >
                {page}
              </motion.div>
            </AnimatePresence>
          )}
        </main>
      </div>
      <SettingsModal
        open={settingsOpen}
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
    </ToastProvider>
  );
}
