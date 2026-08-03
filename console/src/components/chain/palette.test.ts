import { describe, expect, it } from "vitest";
import { FALLBACK_PALETTE, PALETTE_TOKENS, paletteFrom } from "./palette";

describe("paletteFrom", () => {
  it("reads every channel from its CSS custom property", () => {
    const p = paletteFrom((name) => `<${name}>`);
    expect(p.bg).toBe("<--bg>");
    expect(p.ink).toBe("<--text-dim>");
    expect(p.faint).toBe("<--text-faint>");
    expect(p.edge).toBe("<--border-strong>");
    expect(p.live).toBe("<--live>");
    expect(p.ok).toBe("<--ok>");
    expect(p.hold).toBe("<--hold>");
    expect(p.deny).toBe("<--deny>");
  });

  it("falls back per-channel when a token resolves empty", () => {
    const p = paletteFrom((name) => (name === "--live" ? "" : "#123456"));
    expect(p.live).toBe(FALLBACK_PALETTE.live);
    expect(p.bg).toBe("#123456");
  });

  it("trims whitespace getComputedStyle leaves on custom properties", () => {
    expect(paletteFrom(() => "  #abcdef  ").bg).toBe("#abcdef");
  });

  it("sources no brand-violet token", () => {
    // The chroma partition guard (styles.partition.test.ts) polices CSS files
    // but cannot see canvas drawing. This is the canvas-side equivalent:
    // an instrument surface is graphite plus signal, never brand chroma.
    for (const token of PALETTE_TOKENS) {
      expect(token).not.toMatch(/^--(v|v2|v3|accent)/);
    }
  });
});
