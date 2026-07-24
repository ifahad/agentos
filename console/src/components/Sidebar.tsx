import { motion, useReducedMotion } from "framer-motion";
import { useEffect, useRef, useState } from "react";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
import type { AuthRole } from "../lib/rbac";
import { roleLabel } from "../lib/rbac";
import type { KeyUsage } from "../lib/types";
import { transition } from "../ui";
import "./Sidebar.css";

export interface NavRoute {
  path: string;
  label: string;
}

interface SidebarProps {
  routes: NavRoute[];
  activePath: string;
  onNavigate: (path: string) => void;
  adminKey: string;
  role: AuthRole;
  openSettings: () => void;
}

/** Poll cadence for the brand live-dot — the same /admin/usage feed Overview reads. */
const POLL_MS = 4000;
/** How long the dot stays lit after the request count last moved. */
const LIVE_LINGER_MS = 8000;

/**
 * Cheap activity signal for the brand live-dot: poll the same usage endpoint
 * Overview renders and treat any growth in the total request count as "an
 * agent run is in flight". No new endpoints — presentation only.
 */
function useRunsInFlight(adminKey: string): boolean {
  const [live, setLive] = useState(false);
  const prevTotal = useRef<number | null>(null);
  const liveUntil = useRef(0);

  useEffect(() => {
    if (!adminKey) {
      setLive(false);
      prevTotal.current = null;
      return;
    }
    let cancelled = false;

    const poll = async () => {
      try {
        const usage = await apiFetch<KeyUsage[]>(gatewayAdminRequest("/admin/usage", adminKey));
        if (cancelled) return;
        const total = usage.reduce((acc, u) => acc + u.requests, 0);
        if (prevTotal.current !== null && total > prevTotal.current) {
          liveUntil.current = Date.now() + LIVE_LINGER_MS;
        }
        prevTotal.current = total;
        setLive(Date.now() < liveUntil.current);
      } catch {
        // The dot is decorative — poll failures never surface in the UI.
      }
    };

    void poll();
    const timer = setInterval(() => void poll(), POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [adminKey]);

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
              <span className="nav-item-label">{r.label}</span>
            </button>
          );
        })}
      </nav>
      <div className="sidebar-footer">
        <button className="nav-item" onClick={openSettings}>
          <span className="nav-item-label">
            Settings
            {adminKey ? (
              <span className="dim"> · {roleLabel(role).toLowerCase()}</span>
            ) : (
              <span style={{ color: "var(--amber)" }}> · no key</span>
            )}
          </span>
        </button>
      </div>
    </aside>
  );
}
