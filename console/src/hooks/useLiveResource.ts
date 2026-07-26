import { useCallback, useSyncExternalStore } from "react";
import { defaultRegistry } from "../lib/live/registry";
import type { Registry, Snapshot } from "../lib/live/registry";
import { deriveStatus } from "../lib/live/status";
import type { ConnectionState, ResourceStatus } from "../lib/live/status";
import { useNowTick } from "./useNowTick";

/** Default poll cadence — matches the legacy Overview/Chain intervals. */
export const DEFAULT_CADENCE = 4000;

const IDLE_SNAPSHOT: Snapshot<unknown> = {
  data: null,
  error: null,
  updatedAt: null,
  lastErrorAt: null,
  transport: "poll",
};

export interface LiveResource<T> {
  data: T | null;
  error: string | null;
  status: ResourceStatus;
  updatedAt: number | null;
  transport: "poll" | "sse";
  reload: () => void;
}

export function useLiveResource<T>(
  key: string,
  fetcher: () => Promise<T>,
  opts: { cadence?: number; enabled?: boolean; registry?: Registry } = {},
): LiveResource<T> {
  const cadence = opts.cadence ?? DEFAULT_CADENCE;
  const enabled = opts.enabled ?? true;
  const reg = opts.registry ?? defaultRegistry;

  // Subscribe is stable per (key, cadence, enabled). The registry keys by
  // `key`, so `key` MUST encode everything that changes the fetch (endpoint +
  // auth token); the fetcher captured at first-subscribe is reused thereafter.
  const subscribe = useCallback(
    (onChange: () => void) => {
      if (!enabled) return () => {};
      return reg.subscribe(key, fetcher, cadence, onChange);
      // eslint-disable-next-line react-hooks/exhaustive-deps
    },
    [key, cadence, enabled, reg],
  );

  const snapshot = useSyncExternalStore<Snapshot<T>>(
    subscribe,
    () => (enabled ? reg.getSnapshot<T>(key) : (IDLE_SNAPSHOT as Snapshot<T>)),
    () => IDLE_SNAPSHOT as Snapshot<T>,
  );

  const now = useNowTick(1000);
  const status = enabled
    ? deriveStatus({
        updatedAt: snapshot.updatedAt,
        lastErrorAt: snapshot.lastErrorAt,
        now,
        cadence,
      })
    : "idle";

  return {
    data: snapshot.data,
    error: snapshot.error,
    status,
    updatedAt: snapshot.updatedAt,
    transport: snapshot.transport,
    reload: () => reg.reload(key),
  };
}

/** Shell-wide live/stale/offline signal, recomputed on a 1s tick. */
export function useConnectionState(registry: Registry = defaultRegistry): ConnectionState {
  useNowTick(1000);
  return registry.connectionState();
}
