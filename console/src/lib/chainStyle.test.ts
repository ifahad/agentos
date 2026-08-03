import { describe, expect, it } from "vitest";
import {
  CHAIN_STYLES,
  CHAIN_STYLE_KEY,
  DEFAULT_CHAIN_STYLE,
  parseChainStyle,
  readChainStyle,
  writeChainStyle,
} from "./chainStyle";

/** Minimal in-memory Storage stand-in — vitest runs under environment: "node". */
function memStore(seed: Record<string, string> = {}) {
  const map = new Map(Object.entries(seed));
  return {
    getItem: (k: string) => map.get(k) ?? null,
    setItem: (k: string, v: string) => void map.set(k, v),
    removeItem: (k: string) => void map.delete(k),
  };
}

describe("parseChainStyle", () => {
  it("accepts every declared style", () => {
    for (const s of CHAIN_STYLES) expect(parseChainStyle(s)).toBe(s);
  });

  it("falls back to the default for unknown, empty or null input", () => {
    expect(parseChainStyle("hologram")).toBe(DEFAULT_CHAIN_STYLE);
    expect(parseChainStyle("")).toBe(DEFAULT_CHAIN_STYLE);
    expect(parseChainStyle(null)).toBe(DEFAULT_CHAIN_STYLE);
  });

  it("defaults to phosphor", () => {
    expect(DEFAULT_CHAIN_STYLE).toBe("phosphor");
  });
});

describe("read/write", () => {
  it("round-trips a stored choice", () => {
    const store = memStore();
    writeChainStyle("flow", store);
    expect(store.getItem(CHAIN_STYLE_KEY)).toBe("flow");
    expect(readChainStyle(store)).toBe("flow");
  });

  it("returns the default when nothing is stored", () => {
    expect(readChainStyle(memStore())).toBe(DEFAULT_CHAIN_STYLE);
  });

  it("returns the default when the stored value is corrupt", () => {
    expect(readChainStyle(memStore({ [CHAIN_STYLE_KEY]: "{}" }))).toBe(DEFAULT_CHAIN_STYLE);
  });

  it("survives a storage that throws (privacy mode)", () => {
    const hostile = {
      getItem: () => {
        throw new Error("denied");
      },
      setItem: () => {
        throw new Error("denied");
      },
      removeItem: () => {},
    };
    expect(readChainStyle(hostile)).toBe(DEFAULT_CHAIN_STYLE);
    expect(() => writeChainStyle("corridor", hostile)).not.toThrow();
  });
});
