import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { useEffect, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { modalBackdrop, modalPanel, modalPanelReduced } from "./motion";

export interface ModalProps {
  open: boolean;
  onClose: () => void;
  title?: ReactNode;
  children: ReactNode;
  /** Right-aligned action row (usually Buttons). */
  actions?: ReactNode;
  width?: number;
}

/**
 * Backdrop fade + panel scale/rise. ESC and backdrop click close.
 * Rendered in a portal; exit animations run via AnimatePresence.
 */
export function Modal({ open, onClose, title, children, actions, width = 440 }: ModalProps) {
  const reduced = useReducedMotion();

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  return createPortal(
    <AnimatePresence>
      {open && (
        <motion.div
          className="modal-backdrop"
          variants={modalBackdrop}
          initial="hidden"
          animate="show"
          exit="exit"
          onMouseDown={(e) => {
            if (e.target === e.currentTarget) onClose();
          }}
        >
          <motion.div
            className="modal"
            role="dialog"
            aria-modal="true"
            style={{ width }}
            variants={reduced ? modalPanelReduced : modalPanel}
            initial="hidden"
            animate="show"
            exit="exit"
          >
            {title != null && <h2>{title}</h2>}
            {children}
            {actions != null && <div className="modal-actions">{actions}</div>}
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>,
    document.body,
  );
}
