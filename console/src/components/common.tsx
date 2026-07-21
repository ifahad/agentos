import { useEffect, useState } from "react";
import { ApiError } from "../lib/api";

/** Standard page header. */
export function PageHead({ title, subtitle }: { title: string; subtitle?: string }) {
  return (
    <header className="page-head">
      <h1>{title}</h1>
      {subtitle && <p>{subtitle}</p>}
    </header>
  );
}

/** Inline error notice; renders nothing without a message. */
export function ErrorNotice({ error }: { error: string | null }) {
  if (!error) return null;
  return <div className="notice error">{error}</div>;
}

/** Shown on admin pages when no admin key is configured yet. */
export function NeedsKey({ openSettings }: { openSettings: () => void }) {
  return (
    <div className="notice warn">
      No admin key configured.{" "}
      <a
        href="#settings"
        onClick={(e) => {
          e.preventDefault();
          openSettings();
        }}
      >
        Open settings
      </a>{" "}
      and paste the gateway admin key to use this page.
    </div>
  );
}

/** Friendly inline notice for a role that lacks a capability (403 or UI-gated). */
export function ForbiddenNotice({ message }: { message?: string }) {
  return (
    <div className="notice warn">
      {message ??
        "You don't have permission to perform this action. Sign in with a higher-privileged token in Settings."}
    </div>
  );
}

export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 401) return "Unauthorized — set your credentials in Settings.";
    if (err.status === 403) return "You don't have permission to perform this action.";
    return `${err.message} (${err.type}, HTTP ${err.status})`;
  }
  return err instanceof Error ? err.message : String(err);
}

/**
 * Load data once per dependency change; exposes {data, error, loading, reload}.
 */
export function useLoad<T>(loader: () => Promise<T>, deps: unknown[]) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    loader()
      .then((d) => {
        if (!cancelled) setData(d);
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(errorMessage(err));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, tick]);

  return { data, error, loading, reload: () => setTick((t) => t + 1) };
}

/** Copy-to-clipboard button with transient confirmation. */
export function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <button
      className="btn small"
      onClick={() => {
        void navigator.clipboard?.writeText(text).then(() => {
          setCopied(true);
          setTimeout(() => setCopied(false), 1500);
        });
      }}
    >
      {copied ? "Copied" : "Copy"}
    </button>
  );
}
