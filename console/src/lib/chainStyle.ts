/**
 * Which governance-chain renderer the operator has chosen.
 *
 * Persisted per browser, following the `agentos-theme` precedent in
 * SettingsModal. Storage is injected so this stays testable under vitest's
 * node environment, and every access is guarded: a browser in privacy mode
 * throws on localStorage rather than returning null.
 */

export const CHAIN_STYLES = ["phosphor", "corridor", "flow", "minimal"] as const;

export type ChainStyle = (typeof CHAIN_STYLES)[number];

/**
 * Phosphor by default: it is the only renderer that reads as deliberate at
 * zero traffic, and the chain is unlit far more often than it is busy.
 */
export const DEFAULT_CHAIN_STYLE: ChainStyle = "phosphor";

export const CHAIN_STYLE_KEY = "agentos-chain-style";

/** One-line descriptions for the settings picker. */
export const CHAIN_STYLE_LABELS: Record<ChainStyle, string> = {
  phosphor: "Phosphor — an instrument trace with persistence",
  corridor: "Corridor — luminous gates a request punches through",
  flow: "Flow — every request a particle in the stream",
  minimal: "Minimal — the plain rule, no canvas",
};

/** A stored value, or anything else, narrowed to a style. Never throws. */
export function parseChainStyle(raw: string | null | undefined): ChainStyle {
  return (CHAIN_STYLES as readonly string[]).includes(raw ?? "")
    ? (raw as ChainStyle)
    : DEFAULT_CHAIN_STYLE;
}

/** The subset of Storage this module needs — lets tests pass a plain object. */
export interface StyleStore {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

function defaultStore(): StyleStore | null {
  try {
    return typeof localStorage === "undefined" ? null : localStorage;
  } catch {
    return null;
  }
}

export function readChainStyle(store: StyleStore | null = defaultStore()): ChainStyle {
  if (!store) return DEFAULT_CHAIN_STYLE;
  try {
    return parseChainStyle(store.getItem(CHAIN_STYLE_KEY));
  } catch {
    return DEFAULT_CHAIN_STYLE;
  }
}

export function writeChainStyle(
  style: ChainStyle,
  store: StyleStore | null = defaultStore(),
): void {
  if (!store) return;
  try {
    store.setItem(CHAIN_STYLE_KEY, style);
  } catch {
    // Privacy mode: the choice applies for this session only.
  }
}
