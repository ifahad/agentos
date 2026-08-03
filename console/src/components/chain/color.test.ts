import { describe, expect, it } from "vitest";
import { alpha, stopColor } from "./color";
import { FALLBACK_PALETTE } from "./palette";

describe("alpha", () => {
  it("converts 6-digit hex", () => {
    expect(alpha("#5ad1c4", 0.5)).toBe("rgba(90,209,196,0.5)");
  });

  it("expands 3-digit hex", () => {
    expect(alpha("#abc", 1)).toBe("rgba(170,187,204,1)");
  });

  it("rewrites the alpha of an existing rgb()/rgba()", () => {
    expect(alpha("rgb(10, 20, 30)", 0.25)).toBe("rgba(10,20,30,0.25)");
    expect(alpha("rgba(10, 20, 30, 0.8)", 0.25)).toBe("rgba(10,20,30,0.25)");
  });

  it("trims the whitespace getComputedStyle leaves behind", () => {
    expect(alpha("  #5ad1c4  ", 1)).toBe("rgba(90,209,196,1)");
  });

  it("returns the input unchanged when it cannot be parsed", () => {
    // A named colour or an unsupported space must not become "rgba(NaN,...)".
    expect(alpha("rebeccapurple", 0.5)).toBe("rebeccapurple");
    expect(alpha("#zzz", 0.5)).toBe("#zzz");
    expect(alpha("", 0.5)).toBe("");
  });
});

describe("stopColor", () => {
  it("paints a governance denial in the deny hue", () => {
    expect(stopColor("deny", FALLBACK_PALETTE)).toBe(FALLBACK_PALETTE.deny);
  });

  it("paints a provider failure in the hold hue, NOT the denial hue", () => {
    // The distinction lib/chain.ts:20-29 exists to protect: a `chat` row with
    // a 5xx cleared every governance stage and broke at the provider. Drawing
    // it in denial red tells the operator AgentOS refused a request it
    // actually allowed.
    expect(stopColor("fail", FALLBACK_PALETTE)).toBe(FALLBACK_PALETTE.hold);
    expect(stopColor("fail", FALLBACK_PALETTE)).not.toBe(FALLBACK_PALETTE.deny);
  });

  it("fails closed on a pass, which can never reach a stopped mark", () => {
    // `stoppedAt` is null on a pass, so no renderer has a stop position to
    // draw. If one somehow asks, a visible red mark is how the bug surfaces —
    // resolving to something inert would let it read as a clean pass.
    expect(stopColor("pass", FALLBACK_PALETTE)).toBe(FALLBACK_PALETTE.deny);
  });

  it("reads from the injected palette, never a literal", () => {
    const themed = { ...FALLBACK_PALETTE, deny: "#111111", hold: "#222222" };
    expect(stopColor("deny", themed)).toBe("#111111");
    expect(stopColor("fail", themed)).toBe("#222222");
  });
});
