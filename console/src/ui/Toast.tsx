import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { toastItem, toastItemReduced } from "./motion";

export type ToastVariant = "default" | "error" | "success";

export interface ToastOptions {
  variant?: ToastVariant;
  /** Auto-dismiss delay. Default 4000ms; 0 = sticky. */
  duration?: number;
}

interface ToastRecord {
  id: number;
  message: string;
  variant: ToastVariant;
  duration: number;
}

export interface ToastApi {
  toast: (message: string, opts?: ToastOptions) => void;
  success: (message: string, opts?: Omit<ToastOptions, "variant">) => void;
  error: (message: string, opts?: Omit<ToastOptions, "variant">) => void;
  dismiss: (id: number) => void;
}

const ToastContext = createContext<ToastApi | null>(null);

/** Access the toast stack. Must be inside <ToastProvider>. */
export function useToast(): ToastApi {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used within <ToastProvider>");
  return ctx;
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const reduced = useReducedMotion();
  const [toasts, setToasts] = useState<ToastRecord[]>([]);
  const nextId = useRef(1);
  const timers = useRef(new Map<number, ReturnType<typeof setTimeout>>());

  const dismiss = useCallback((id: number) => {
    const t = timers.current.get(id);
    if (t) clearTimeout(t);
    timers.current.delete(id);
    setToasts((prev) => prev.filter((x) => x.id !== id));
  }, []);

  const push = useCallback(
    (message: string, variant: ToastVariant, duration: number) => {
      const id = nextId.current++;
      setToasts((prev) => [...prev.slice(-4), { id, message, variant, duration }]);
      if (duration > 0) {
        timers.current.set(id, setTimeout(() => dismiss(id), duration));
      }
    },
    [dismiss],
  );

  const api = useMemo<ToastApi>(
    () => ({
      toast: (message, opts) => push(message, opts?.variant ?? "default", opts?.duration ?? 4000),
      success: (message, opts) => push(message, "success", opts?.duration ?? 4000),
      error: (message, opts) => push(message, "error", opts?.duration ?? 4000),
      dismiss,
    }),
    [push, dismiss],
  );

  return (
    <ToastContext.Provider value={api}>
      {children}
      {createPortal(
        <div className="ui-toast-stack" role="status" aria-live="polite">
          <AnimatePresence>
            {toasts.map((t) => (
              <motion.div
                key={t.id}
                className={`ui-toast ui-toast-${t.variant}`}
                variants={reduced ? toastItemReduced : toastItem}
                initial="hidden"
                animate="show"
                exit="exit"
                layout={!reduced}
                onClick={() => dismiss(t.id)}
              >
                <span className="ui-toast-dot" aria-hidden />
                <span>{t.message}</span>
              </motion.div>
            ))}
          </AnimatePresence>
        </div>,
        document.body,
      )}
    </ToastContext.Provider>
  );
}
