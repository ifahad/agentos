// @vitest-environment jsdom
//
// The first hook test in the console. useLiveResource sits under every page's
// data loading, and until the DOM harness landed it could not be exercised at
// all — its behaviour was only ever observed by opening the app.
import { act, render, waitFor } from "@testing-library/react";
import { useEffect } from "react";
import { describe, expect, it, vi } from "vitest";
import { createRegistry } from "../lib/live/registry";
import { useLiveResource } from "./useLiveResource";

/** Renders the hook and records every value it returned, in order. */
function harness<T>(key: () => string, fetcher: () => Promise<T>, registry: ReturnType<typeof createRegistry>) {
  const seen: Array<ReturnType<typeof useLiveResource<T>>> = [];
  function Probe() {
    const r = useLiveResource<T>(key(), fetcher, { cadence: 50, registry });
    useEffect(() => {
      seen.push(r);
    });
    seen[seen.length] = r;
    return null;
  }
  return { seen, Probe };
}

describe("useLiveResource", () => {
  it("reports no data before the first fetch resolves", async () => {
    const registry = createRegistry();
    let resolve!: (v: string) => void;
    const fetcher = () => new Promise<string>((r) => (resolve = r));
    const { seen, Probe } = harness(() => "k1", fetcher, registry);

    render(<Probe />);
    // The console's rule is that it must never show a reading it cannot prove.
    // Before the fetch answers there is nothing to show, and null is how the
    // pages above distinguish "not loaded" from "loaded and empty".
    expect(seen[0].data).toBeNull();

    await act(async () => {
      resolve("value");
    });
    await waitFor(() => expect(seen[seen.length - 1].data).toBe("value"));
  });

  it("never shows one key's data under another key", async () => {
    const registry = createRegistry();
    const fetcher = vi.fn(async () => "alpha-data");
    let key = "alpha";
    const { seen, Probe } = harness(() => key, fetcher, registry);

    const { rerender } = render(<Probe />);
    await waitFor(() => expect(seen[seen.length - 1].data).toBe("alpha-data"));

    // Switching resource must not leave the previous resource's rows on screen
    // under the new one's heading — that is the same "reading it cannot prove"
    // failure the console forbids, and a subscription that re-keys without
    // clearing produces it.
    key = "beta";
    let resolveBeta!: (v: string) => void;
    const betaFetcher = () => new Promise<string>((r) => (resolveBeta = r));
    function Beta() {
      const r = useLiveResource<string>("beta", betaFetcher, { cadence: 50, registry });
      seen.push(r);
      return null;
    }
    rerender(<Beta />);
    expect(seen[seen.length - 1].data).not.toBe("alpha-data");

    await act(async () => {
      resolveBeta("beta-data");
    });
    await waitFor(() => expect(seen[seen.length - 1].data).toBe("beta-data"));
  });

  it("does not subscribe at all when disabled", async () => {
    const registry = createRegistry();
    const fetcher = vi.fn(async () => "data");
    function Probe() {
      useLiveResource("k", fetcher, { cadence: 50, enabled: false, registry });
      return null;
    }
    render(<Probe />);
    // A disabled resource must not poll. Pages disable when there is no admin
    // key, and a hook that fetched anyway would hammer an endpoint that can
    // only 401.
    await new Promise((r) => setTimeout(r, 120));
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("surfaces a fetch error instead of pretending there is no data", async () => {
    const registry = createRegistry();
    const fetcher = async () => {
      throw new Error("boom");
    };
    const { seen, Probe } = harness(() => "err", fetcher, registry);
    render(<Probe />);
    await waitFor(() => expect(seen[seen.length - 1].error).toBeTruthy());
    // Data stays null: reporting an empty result for a failed fetch is exactly
    // the "0 keys" lie the console's information design forbids.
    expect(seen[seen.length - 1].data).toBeNull();
  });
});
