import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { pollTransport } from "./pollTransport";

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe("pollTransport", () => {
  it("fetches immediately, then once per cadence", async () => {
    const values: number[] = [];
    const fetcher = vi.fn<() => Promise<number>>().mockResolvedValue(42);
    const t = pollTransport({ fetcher, cadence: 1000, onData: (v) => values.push(v), onError: () => {} });
    await vi.advanceTimersByTimeAsync(0); // flush immediate fetch
    expect(values).toEqual([42]);
    await vi.advanceTimersByTimeAsync(1000);
    expect(values).toEqual([42, 42]);
    t.stop();
  });

  it("routes rejections to onError and keeps polling", async () => {
    const errs: unknown[] = [];
    const fetcher = vi.fn().mockRejectedValue(new Error("boom"));
    const t = pollTransport({ fetcher, cadence: 1000, onData: () => {}, onError: (e) => errs.push(e) });
    await vi.advanceTimersByTimeAsync(0);
    expect(errs).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(1000);
    expect(errs).toHaveLength(2);
    t.stop();
  });

  it("stop() prevents further callbacks", async () => {
    const values: number[] = [];
    const fetcher = vi.fn<() => Promise<number>>().mockResolvedValue(1);
    const t = pollTransport({ fetcher, cadence: 1000, onData: (v) => values.push(v), onError: () => {} });
    await vi.advanceTimersByTimeAsync(0);
    t.stop();
    await vi.advanceTimersByTimeAsync(5000);
    expect(values).toEqual([1]); // no growth after stop
  });
});
