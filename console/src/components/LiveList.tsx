import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import type { ReactNode } from "react";
import { staggerItem, staggerItemReduced, transitionFast } from "../ui";

/**
 * A list whose rows animate in/out as items enter/leave — the streaming-feed
 * motion from Overview, reusable across tables and feeds. Motion is telemetry:
 * a new row appearing = a real new event. Reduced-motion collapses to instant.
 */
export function LiveList<T>({
  items,
  getKey,
  renderItem,
  as = "ul",
  className,
}: {
  items: readonly T[];
  getKey: (item: T) => string;
  renderItem: (item: T) => ReactNode;
  as?: "ul" | "ol";
  className?: string;
}) {
  const reduced = useReducedMotion();
  const List = as === "ol" ? motion.ol : motion.ul;
  return (
    <List className={className} layout={!reduced}>
      <AnimatePresence initial={false}>
        {items.map((item) => (
          <motion.li
            key={getKey(item)}
            layout={!reduced}
            variants={reduced ? staggerItemReduced : staggerItem}
            initial="hidden"
            animate="show"
            exit={{ opacity: 0, transition: transitionFast }}
          >
            {renderItem(item)}
          </motion.li>
        ))}
      </AnimatePresence>
    </List>
  );
}
