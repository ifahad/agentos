/** A running data source for one resource. Stopping it releases its timer/stream. */
export interface Transport {
  stop(): void;
}

export interface TransportArgs<T> {
  fetcher: () => Promise<T>;
  cadence: number; // ms
  onData: (data: T) => void;
  onError: (err: unknown) => void;
}

/** Builds a Transport. `pollTransport` now; an `sseTransport` can implement this later. */
export type TransportFactory = <T>(args: TransportArgs<T>) => Transport;

export type TransportKind = "poll" | "sse";
