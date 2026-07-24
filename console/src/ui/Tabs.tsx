import { motion, useReducedMotion } from "framer-motion";
import { transitionFast } from "./motion";

export interface TabItem {
  id: string;
  label: string;
}

export interface TabsProps {
  tabs: TabItem[];
  active: string;
  onChange: (id: string) => void;
  /** layoutId namespace — must be unique per Tabs instance on the page. */
  id?: string;
}

/** Tab strip with a sliding active pill (same signature motion as the nav). */
export function Tabs({ tabs, active, onChange, id = "tabs" }: TabsProps) {
  const reduced = useReducedMotion();
  return (
    <div className="ui-tabs" role="tablist">
      {tabs.map((t) => {
        const isActive = t.id === active;
        return (
          <button
            key={t.id}
            role="tab"
            aria-selected={isActive}
            className={`ui-tab${isActive ? " active" : ""}`}
            onClick={() => onChange(t.id)}
            type="button"
          >
            {isActive && (
              <motion.span
                className="ui-tab-pill"
                layoutId={reduced ? undefined : `${id}-pill`}
                transition={transitionFast}
              />
            )}
            <span className="ui-tab-label">{t.label}</span>
          </button>
        );
      })}
    </div>
  );
}
