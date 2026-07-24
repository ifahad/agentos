import { motion, useReducedMotion } from "framer-motion";
import type { HTMLAttributes, ReactNode, TableHTMLAttributes, TdHTMLAttributes, ThHTMLAttributes } from "react";
import { staggerContainer, staggerItem } from "./motion";

/**
 * Table primitives with staggered row entrance + hover lift.
 *
 * Usage:
 *   <Table>
 *     <thead><tr><th>…</th></tr></thead>
 *     <Tbody staggerKey={rows.length}>
 *       {rows.map(r => <Tr key={r.id}><td>…</td></Tr>)}
 *     </Tbody>
 *   </Table>
 *
 * `staggerKey` — change it when the data reloads to replay the stagger.
 */

export function Table({ children, className = "", ...rest }: TableHTMLAttributes<HTMLTableElement>) {
  return (
    <div className="table-wrap">
      <table className={className} {...rest}>
        {children}
      </table>
    </div>
  );
}

export interface TbodyProps {
  children: ReactNode;
  /** Replay the entrance stagger when this value changes. */
  staggerKey?: unknown;
}

export function Tbody({ children, staggerKey }: TbodyProps) {
  const reduced = useReducedMotion();
  if (reduced) {
    return <tbody>{children}</tbody>;
  }
  return (
    <motion.tbody
      key={String(staggerKey ?? "static")}
      variants={staggerContainer}
      initial="hidden"
      animate="show"
    >
      {children}
    </motion.tbody>
  );
}

export interface TrProps extends HTMLAttributes<HTMLTableRowElement> {
  children: ReactNode;
  /** Set false for rows that should not participate in the stagger. */
  animate?: boolean;
}

export function Tr({ children, animate = true, className = "", ...rest }: TrProps) {
  const reduced = useReducedMotion();
  if (!animate || reduced) {
    return (
      <tr className={className} {...rest}>
        {children}
      </tr>
    );
  }
  return (
    <motion.tr className={className} variants={staggerItem} {...(rest as object)}>
      {children}
    </motion.tr>
  );
}

// Re-export plain cell types for convenience so pages can type helpers.
export type { TdHTMLAttributes, ThHTMLAttributes };
