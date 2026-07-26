import type { TransportArgs, TransportFactory } from "./transport";

/**
 * Chained-setTimeout poller: fetch immediately, then schedule the next fetch
 * only after the current one settles, so a slow response never stacks requests.
 */
export const pollTransport: TransportFactory = <T>({
  fetcher,
  cadence,
  onData,
  onError,
}: TransportArgs<T>) => {
  let stopped = false;
  let timer: ReturnType<typeof setTimeout> | null = null;

  const tick = () => {
    fetcher()
      .then((d) => {
        if (!stopped) onData(d);
      })
      .catch((e) => {
        if (!stopped) onError(e);
      })
      .finally(() => {
        if (!stopped) timer = setTimeout(tick, cadence);
      });
  };

  tick();

  return {
    stop() {
      stopped = true;
      if (timer) clearTimeout(timer);
    },
  };
};
