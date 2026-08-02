import { motion, useReducedMotion } from "framer-motion";
import type { HTMLAttributes, ReactNode } from "react";
import { fadeRise, fadeRiseReduced } from "./motion";

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
  /** Set false to render without the fade-rise entrance. */
  animate?: boolean;
}

/** Raised surface card with fade + 6px rise entrance. */
export function Card({ children, animate = true, className = "", ...rest }: CardProps) {
  const reduced = useReducedMotion();
  if (!animate) {
    return (
      <div className={`card ${className}`.trim()} {...rest}>
        {children}
      </div>
    );
  }
  return (
    <motion.div
      className={`card ${className}`.trim()}
      variants={reduced ? fadeRiseReduced : fadeRise}
      initial="hidden"
      animate="show"
      {...(rest as object)}
    >
      {children}
    </motion.div>
  );
}

/** Raised panel (header + body) with fade-rise entrance. */
export function Panel({ children, animate = true, className = "", ...rest }: CardProps) {
  const reduced = useReducedMotion();
  if (!animate) {
    return (
      <div className={`panel ${className}`.trim()} {...rest}>
        {children}
      </div>
    );
  }
  return (
    <motion.div
      className={`panel ${className}`.trim()}
      variants={reduced ? fadeRiseReduced : fadeRise}
      initial="hidden"
      animate="show"
      {...(rest as object)}
    >
      {children}
    </motion.div>
  );
}

export interface PanelHeadProps {
  title: ReactNode;
  /**
   * One line stating what this panel contains and whether anything needs
   * attention — "12 keys · 2 inactive". Derived from data the page already
   * has; never fetched. Omit the attention clause when the count is zero so a
   * quiet panel reads quiet.
   */
  summary?: ReactNode;
  /** Right-aligned slot (actions, badges, filters). */
  actions?: ReactNode;
}

export function PanelHead({ title, summary, actions }: PanelHeadProps) {
  return (
    <div className="panel-head">
      <div className="panel-head-text">
        <h2>{title}</h2>
        {summary != null && <div className="panel-head-summary">{summary}</div>}
      </div>
      {actions}
    </div>
  );
}
