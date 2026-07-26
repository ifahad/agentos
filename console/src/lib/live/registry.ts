import { errorMessage } from "../../components/common";
import { deriveStatus, reduceConnection } from "./status";
import type { ConnectionState } from "./status";
import { pollTransport } from "./pollTransport";
import type { Transport, TransportFactory, TransportKind } from "./transport";

export interface Snapshot<T> {
  data: T | null;
  error: string | null;
  updatedAt: number | null;
  lastErrorAt: number | null;
  transport: TransportKind;
}

interface Entry {
  fetcher: () => Promise<unknown>;
  cadence: number;
  // Refcount of active subscriptions for this key. Relies on callback-identity
  // uniqueness: React's `useSyncExternalStore` hands each subscription a
  // distinct callback, so a `Set` correctly tracks add/remove without
  // collisions between subscribers.
  subscribers: Set<() => void>;
  transport: Transport | null;
  data: unknown;
  error: string | null;
  updatedAt: number | null;
  lastErrorAt: number | null;
  kind: TransportKind;
  snapshot: Snapshot<unknown>;
}

export interface Registry {
  subscribe<T>(key: string, fetcher: () => Promise<T>, cadence: number, onChange: () => void): () => void;
  getSnapshot<T>(key: string): Snapshot<T>;
  reload(key: string): void;
  pauseAll(): void;
  resumeAll(): void;
  connectionState(): ConnectionState;
}

const IDLE: Snapshot<unknown> = {
  data: null,
  error: null,
  updatedAt: null,
  lastErrorAt: null,
  transport: "poll",
};

export function createRegistry(
  opts: { transportFactory?: TransportFactory; now?: () => number } = {},
): Registry {
  const makeTransport = opts.transportFactory ?? pollTransport;
  const now = opts.now ?? (() => Date.now());
  const entries = new Map<string, Entry>();

  function rebuildSnapshot(e: Entry) {
    e.snapshot = {
      data: e.data,
      error: e.error,
      updatedAt: e.updatedAt,
      lastErrorAt: e.lastErrorAt,
      transport: e.kind,
    };
  }

  function notify(e: Entry) {
    for (const cb of e.subscribers) cb();
  }

  function start(e: Entry) {
    e.transport = makeTransport({
      fetcher: e.fetcher,
      cadence: e.cadence,
      onData: (d) => {
        e.data = d;
        e.error = null;
        e.updatedAt = now();
        rebuildSnapshot(e);
        notify(e);
      },
      onError: (err) => {
        e.error = errorMessage(err);
        e.lastErrorAt = now();
        rebuildSnapshot(e); // data held; only error/lastErrorAt change
        notify(e);
      },
    });
  }

  return {
    // The FIRST subscriber for a given `key` wins: its `fetcher` and `cadence`
    // are captured on the entry and reused by every later subscriber to the
    // same key, regardless of what they pass. Callers MUST therefore encode
    // everything that changes the fetch (endpoint + auth token) into `key`
    // itself — a shared key with divergent fetchers makes behavior depend on
    // subscription order, not on which caller you think you're looking at.
    subscribe(key, fetcher, cadence, onChange) {
      let e = entries.get(key);
      if (!e) {
        e = {
          fetcher: fetcher as () => Promise<unknown>,
          cadence,
          subscribers: new Set(),
          transport: null,
          data: null,
          error: null,
          updatedAt: null,
          lastErrorAt: null,
          kind: "poll",
          snapshot: { ...IDLE },
        };
        entries.set(key, e);
        start(e);
      }
      e.subscribers.add(onChange);
      return () => {
        const entry = entries.get(key);
        if (!entry) return;
        entry.subscribers.delete(onChange);
        if (entry.subscribers.size === 0) {
          entry.transport?.stop();
          entries.delete(key);
        }
      };
    },

    getSnapshot<T>(key: string): Snapshot<T> {
      const e = entries.get(key);
      return (e ? e.snapshot : IDLE) as Snapshot<T>;
    },

    reload(key) {
      const e = entries.get(key);
      if (!e) return;
      e.transport?.stop();
      start(e); // immediate refetch
    },

    pauseAll() {
      for (const e of entries.values()) {
        e.transport?.stop();
        e.transport = null;
      }
    },

    resumeAll() {
      for (const e of entries.values()) {
        if (!e.transport) start(e);
      }
    },

    connectionState() {
      const t = now();
      const statuses = [...entries.values()].map((e) =>
        deriveStatus({ updatedAt: e.updatedAt, lastErrorAt: e.lastErrorAt, now: t, cadence: e.cadence }),
      );
      return reduceConnection(statuses);
    },
  };
}

/** The console's single shared registry. */
export const defaultRegistry: Registry = createRegistry();

/**
 * Pause polling while the tab is hidden; refetch on return. Browser-only;
 * returns a teardown. No-op where `document` is absent (node tests never call it).
 */
export function installVisibilityPause(reg: Registry): () => void {
  if (typeof document === "undefined") return () => {};
  const onChange = () => (document.hidden ? reg.pauseAll() : reg.resumeAll());
  document.addEventListener("visibilitychange", onChange);
  return () => document.removeEventListener("visibilitychange", onChange);
}
