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
  /** Right-aligned slot (actions, badges, filters). */
  actions?: ReactNode;
}

export function PanelHead({ title, actions }: PanelHeadProps) {
  return (
    <div className="panel-head">
      <h2>{title}</h2>
      {actions}
    </div>
  );
}
