import { motion, useReducedMotion } from "framer-motion";
import { useEffect, useRef, useState } from "react";
import { useLiveResource } from "../hooks/useLiveResource";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
import type { AuthRole } from "../lib/rbac";
import { roleLabel } from "../lib/rbac";
import type { KeyUsage } from "../lib/types";
import { transition } from "../ui";
import type { IconName } from "../ui/icons";
import { Icon } from "../ui/icons";
import "./Sidebar.css";

export interface NavRoute {
  path: string;
  label: string;
  icon: IconName;
}

interface SidebarProps {
  routes: NavRoute[];
  activePath: string;
  onNavigate: (path: string) => void;
  adminKey: string;
  role: AuthRole;
  openSettings: () => void;
}

/** How long the dot stays lit after the request count last moved. */
const LIVE_LINGER_MS = 8000;

/**
 * Cheap activity signal for the brand live-dot: read the same usage resource
 * Overview subscribes to (identical key + cadence, so the registry dedupes
 * the two onto a single shared poll) and treat any growth in the total
 * request count as "an agent run is in flight". No new endpoints —
 * presentation only.
 */
function useRunsInFlight(adminKey: string): boolean {
  const usage = useLiveResource<KeyUsage[]>(
    `admin/usage#${adminKey}`,
    () => apiFetch<KeyUsage[]>(gatewayAdminRequest("/admin/usage", adminKey)),
    { enabled: Boolean(adminKey), cadence: 5000 },
  );

  const [live, setLive] = useState(false);
  const prevTotal = useRef<number | null>(null);
  const liveUntil = useRef(0);

  useEffect(() => {
    if (!adminKey) {
      setLive(false);
      prevTotal.current = null;
      liveUntil.current = 0;
      return;
    }
    const data = usage.data;
    // The dot is decorative — poll failures never surface in the UI; hold
    // the last known reading rather than flipping it off.
    if (!data) return;
    const total = data.reduce((acc, u) => acc + u.requests, 0);
    if (prevTotal.current !== null && total > prevTotal.current) {
      liveUntil.current = Date.now() + LIVE_LINGER_MS;
    }
    prevTotal.current = total;
    setLive(Date.now() < liveUntil.current);
  }, [adminKey, usage.data]);

  return live;
}

/**
 * App shell sidebar: brand mark with live dot, nav with the sliding
 * active-pill (framer-motion layoutId), and the settings footer.
 * Look and motion follow console/design/shell/navigation.html.
 */
export function Sidebar({
  routes,
  activePath,
  onNavigate,
  adminKey,
  role,
  openSettings,
}: SidebarProps) {
  const reduced = useReducedMotion();
  const live = useRunsInFlight(adminKey);

  return (
    <aside className="sidebar">
      <div className="brand">
        <span className="brand-name">AgentOS</span>
        {live && <span className="live" title="run in flight" aria-label="agent run in flight" />}
      </div>
      <nav className="nav">
        {routes.map((r) => {
          const active = r.path === activePath;
          return (
            <button
              key={r.path}
              className={`nav-item${active ? " active" : ""}`}
              aria-current={active ? "page" : undefined}
              onClick={() => onNavigate(r.path)}
            >
              {active &&
                (reduced ? (
                  <span className="nav-pill" aria-hidden />
                ) : (
                  <motion.span
                    className="nav-pill"
                    aria-hidden
                    layoutId="nav-pill"
                    transition={transition}
                  />
                ))}
              <Icon name={r.icon} size={15} className="nav-icon" />
              <span className="nav-item-label">{r.label}</span>
            </button>
          );
        })}
      </nav>
      <div className="sidebar-footer">
        <button className="nav-item" onClick={openSettings}>
          <Icon name="settings" size={15} className="nav-icon" />
          <span className="nav-item-label">
            Settings
            {adminKey ? (
              <span className="dim"> · {roleLabel(role).toLowerCase()}</span>
            ) : (
              /* No key is a held state, not an error: the console is waiting on
                 you, exactly like a tool call awaiting approval. */
              <span className="nav-note-hold"> · no key</span>
            )}
          </span>
        </button>
      </div>
    </aside>
  );
}
