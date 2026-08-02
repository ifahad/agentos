import { useEffect, useRef } from "react";
import type { ReactNode } from "react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { DUR_MED, EASE } from "./motion";

export interface DisclosureProps {
  /** Whether the body is shown. Owned by the page. */
  open: boolean;
  /** Called with the next state. The page also renders the trigger. */
  onOpenChange: (next: boolean) => void;
  /** Must match the trigger's aria-controls. */
  id: string;
  children: ReactNode;
}

/**
 * Collapsible body for a create form.
 *
 * The trigger lives in the panel head, not here — the design puts it beside
 * the panel title, and rendering it here would make Disclosure know about
 * PanelHead's layout. The page owns `open` and renders its own button with
 * aria-expanded and aria-controls={id}.
 */
export function Disclosure({ open, onOpenChange, id, children }: DisclosureProps) {
  const reduced = useReducedMotion();
  const bodyRef = useRef<HTMLDivElement>(null);

  // Escape closes an open form, matching the modal's affordance. The page
  // returns focus to its own trigger, which it owns.
  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onOpenChange(false);
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onOpenChange]);

  return (
    <AnimatePresence initial={false}>
      {open && (
        <motion.div
          id={id}
          ref={bodyRef}
          className="ui-disclosure"
          initial={reduced ? { opacity: 1 } : { opacity: 0, height: 0 }}
          animate={reduced ? { opacity: 1 } : { opacity: 1, height: "auto" }}
          exit={reduced ? { opacity: 1 } : { opacity: 0, height: 0 }}
          transition={{ duration: DUR_MED, ease: EASE }}
        >
          {children}
        </motion.div>
      )}
    </AnimatePresence>
  );
}
