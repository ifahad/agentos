/**
 * Canvas colour, sourced from the same CSS custom properties the rest of the
 * console uses.
 *
 * Two reasons this exists rather than hex literals in the renderers. Themes:
 * --bg is #07080b dark and #eceae4 light, so baked values would be wrong half
 * the time. And the chroma partition: styles.partition.test.ts bans brand
 * violet on instrument surfaces, but it reads CSS files and canvas drawing is
 * invisible to it. Routing every colour through this one module keeps that
 * guarantee enforceable — see the token test in palette.test.ts.
 */

export interface ChainPalette {
  bg: string;
  ink: string;
  faint: string;
  edge: string;
  live: string;
  ok: string;
  hold: string;
  deny: string;
}

/** Channel -> CSS custom property. The single place canvas colour is decided. */
const TOKEN_OF: Record<keyof ChainPalette, string> = {
  bg: "--bg",
  ink: "--text-dim",
  faint: "--text-faint",
  edge: "--border-strong",
  live: "--live",
  ok: "--ok",
  hold: "--hold",
  deny: "--deny",
};

/** Every token this module reads. Asserted brand-violet-free by its test. */
export const PALETTE_TOKENS: readonly string[] = Object.values(TOKEN_OF);

/** Used when a token resolves empty — matches the dark theme's declared values. */
export const FALLBACK_PALETTE: ChainPalette = {
  bg: "#07080b",
  ink: "#98a1a8",
  faint: "#7e878e",
  edge: "rgba(255,255,255,0.13)",
  live: "#5ad1c4",
  ok: "#6cc48f",
  hold: "#e3a851",
  deny: "#e2685f",
};

/** Build a palette from a token resolver. Pure — the DOM lives in readPalette. */
export function paletteFrom(get: (token: string) => string): ChainPalette {
  const out = {} as ChainPalette;
  for (const channel of Object.keys(TOKEN_OF) as Array<keyof ChainPalette>) {
    const value = (get(TOKEN_OF[channel]) ?? "").trim();
    out[channel] = value || FALLBACK_PALETTE[channel];
  }
  return out;
}

/** Resolve the palette against a live element. Call on mount and theme change. */
export function readPalette(el: Element): ChainPalette {
  const cs = getComputedStyle(el);
  return paletteFrom((token) => cs.getPropertyValue(token));
}
