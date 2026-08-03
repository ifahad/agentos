import { describe, expect, it } from "vitest";
import { alpha } from "./color";

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
