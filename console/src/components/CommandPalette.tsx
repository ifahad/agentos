import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { filterBySearch } from "../lib/table/search";
import { Input, modalBackdrop, modalPanel, modalPanelReduced } from "../ui";
import type { IconName } from "../ui/icons";
import { Icon } from "../ui/icons";

/** One entry in the palette: a label to match on and an action to run. */
export interface Command {
  id: string;
  label: string;
  icon?: IconName;
  run: () => void;
}

export interface CommandPaletteProps {
  open: boolean;
  onClose: () => void;
  commands: Command[];
}

/**
 * ⌘K / Ctrl-K command palette.
 *
 * Not built on top of `Modal`: that primitive is shaped for a static
 * title/body/actions dialog, while this needs an autofocused search input,
 * a filtered listbox, and its own arrow-key/Enter navigation on top. It is a
 * lean portal of its own, but borrows Modal's backdrop/panel motion variants
 * (`modalBackdrop` / `modalPanel` / `modalPanelReduced`) and its monochrome
 * surface tokens, so it still reads as the same chrome.
 */
export function CommandPalette({ open, onClose, commands }: CommandPaletteProps) {
  const reduced = useReducedMotion();
  const [query, setQuery] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);

  // Fresh every time the palette opens: empty query, top row highlighted.
  useEffect(() => {
    if (open) {
      setQuery("");
      setActiveIndex(0);
    }
  }, [open]);

  // Typing changes the candidate set — always re-highlight the top match.
  useEffect(() => {
    setActiveIndex(0);
  }, [query]);

  // Same window-keydown Escape pattern as Modal, so focus doesn't have to be
  // on the input for Esc to close the palette.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  const filtered = filterBySearch(commands, ["label"], query);
  const activeIdx = filtered.length === 0 ? -1 : Math.min(activeIndex, filtered.length - 1);

  const runAndClose = (cmd: Command) => {
    cmd.run();
    onClose();
  };

  return createPortal(
    <AnimatePresence>
      {open && (
        <motion.div
          className="cmdk-backdrop"
          variants={modalBackdrop}
          initial="hidden"
          animate="show"
          exit="exit"
          onMouseDown={(e) => {
            if (e.target === e.currentTarget) onClose();
          }}
        >
          <motion.div
            className="cmdk-panel"
            role="dialog"
            aria-modal="true"
            aria-label="Command palette"
            variants={reduced ? modalPanelReduced : modalPanel}
            initial="hidden"
            animate="show"
            exit="exit"
          >
            <div className="cmdk-search">
              <Icon name="search" size={14} className="cmdk-search-icon" />
              <Input
                aria-label="Command palette"
                placeholder="Type a command…"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "ArrowDown") {
                    e.preventDefault();
                    if (filtered.length > 0) setActiveIndex((activeIdx + 1) % filtered.length);
                  } else if (e.key === "ArrowUp") {
                    e.preventDefault();
                    if (filtered.length > 0) {
                      setActiveIndex((activeIdx - 1 + filtered.length) % filtered.length);
                    }
                  } else if (e.key === "Enter") {
                    e.preventDefault();
                    const cmd = filtered[activeIdx];
                    if (cmd) runAndClose(cmd);
                  } else if (e.key === "Escape") {
                    onClose();
                  }
                }}
                autoFocus
              />
            </div>
            <div className="cmdk-list" role="listbox" aria-label="Commands">
              {filtered.length === 0 ? (
                <div className="empty">No matching commands</div>
              ) : (
                filtered.map((cmd, i) => (
                  <button
                    key={cmd.id}
                    type="button"
                    role="option"
                    aria-selected={i === activeIdx}
                    className={`cmdk-item${i === activeIdx ? " active" : ""}`}
                    onMouseEnter={() => setActiveIndex(i)}
                    onClick={() => runAndClose(cmd)}
                  >
                    {cmd.icon && <Icon name={cmd.icon} size={14} className="cmdk-item-icon" />}
                    <span>{cmd.label}</span>
                  </button>
                ))
              )}
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>,
    document.body,
  );
}
