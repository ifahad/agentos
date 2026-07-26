// console/src/lib/live/registry.test.ts
import { describe, expect, it, vi } from "vitest";
import { createRegistry } from "./registry";
import type { Transport, TransportArgs } from "./transport";

/** A hand-driven transport so tests can push data/errors deterministically. */
function fakeFactory() {
  const created: Array<TransportArgs<unknown> & Transport & { stopped: boolean }> = [];
  const factory = <T,>(args: TransportArgs<T>): Transport => {
    const inst = {
      ...(args as TransportArgs<unknown>),
      stopped: false,
      stop() {
        this.stopped = true;
      },
    };
    created.push(inst);
    return inst;
  };
  return { factory, created };
}

describe("createRegistry", () => {
  it("shares one transport across subscribers to the same key and stops on last unsubscribe", () => {
    const { factory, created } = fakeFactory();
    const reg = createRegistry({ transportFactory: factory, now: () => 1000 });
    const a = vi.fn();
    const b = vi.fn();
    const unsubA = reg.subscribe("k", async () => 1, 4000, a);
    const unsubB = reg.subscribe("k", async () => 1, 4000, b);
    expect(created).toHaveLength(1);
    created[0].onData({ x: 1 });
    expect(a).toHaveBeenCalled();
    expect(b).toHaveBeenCalled();
    expect(reg.getSnapshot<{ x: number }>("k").data).toEqual({ x: 1 });
    unsubA();
    expect(created[0].stopped).toBe(false);
    unsubB();
    expect(created[0].stopped).toBe(true);
  });

  it("holds last-known data on error and stamps timestamps from now()", () => {
    let clock = 1000;
    const { factory, created } = fakeFactory();
    const reg = createRegistry({ transportFactory: factory, now: () => clock });
    reg.subscribe("k", async () => 1, 4000, () => {});
    created[0].onData({ x: 1 });
    clock = 5000;
    created[0].onError(new Error("boom"));
    const s = reg.getSnapshot<{ x: number }>("k");
    expect(s.data).toEqual({ x: 1 }); // held
    expect(s.error).toContain("boom");
    expect(s.updatedAt).toBe(1000); // unchanged by the error
    expect(s.lastErrorAt).toBe(5000);
  });

  it("returns a stable snapshot reference until something changes", () => {
    const { factory, created } = fakeFactory();
    const reg = createRegistry({ transportFactory: factory, now: () => 1 });
    reg.subscribe("k", async () => 1, 4000, () => {});
    const s1 = reg.getSnapshot("k");
    expect(reg.getSnapshot("k")).toBe(s1);
    created[0].onData({ x: 2 });
    expect(reg.getSnapshot("k")).not.toBe(s1);
  });

  it("derives connection state across keys", () => {
    let clock = 1000;
    const { factory, created } = fakeFactory();
    const reg = createRegistry({ transportFactory: factory, now: () => clock });
    reg.subscribe("k", async () => 1, 4000, () => {});
    created[0].onData({ x: 1 });
    expect(reg.connectionState()).toBe("live");
    clock = 1000 + 4000 * 1.5 + 1;
    expect(reg.connectionState()).toBe("stale");
  });

  it("pauseAll stops transports; resumeAll rebuilds them for live keys", () => {
    const { factory, created } = fakeFactory();
    const reg = createRegistry({ transportFactory: factory, now: () => 1 });
    reg.subscribe("k", async () => 1, 4000, () => {});
    reg.pauseAll();
    expect(created[0].stopped).toBe(true);
    reg.resumeAll();
    expect(created).toHaveLength(2);
    expect(created[1].stopped).toBe(false);
  });

  it("reload stops the old transport and builds a fresh one", () => {
    const { factory, created } = fakeFactory();
    const reg = createRegistry({ transportFactory: factory, now: () => 1 });
    reg.subscribe("k", async () => 1, 4000, () => {});
    reg.reload("k");
    expect(created[0].stopped).toBe(true);
    expect(created).toHaveLength(2);
    expect(created[1].stopped).toBe(false);
  });
});
