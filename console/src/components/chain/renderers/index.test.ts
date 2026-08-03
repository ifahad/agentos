import { describe, expect, it } from "vitest";
import { CHAIN_STYLES } from "../../../lib/chainStyle";
import { createRenderer } from "./index";
import { frame } from "./testHarness";

describe("createRenderer", () => {
  it("returns a drawable renderer for every animated style", () => {
    for (const style of CHAIN_STYLES) {
      if (style === "minimal") continue;
      const r = createRenderer(style);
      expect(r, `${style} must have a renderer`).not.toBeNull();
      expect(() => r!.draw(frame())).not.toThrow();
    }
  });

  it("returns null for minimal — it is DOM, not canvas", () => {
    expect(createRenderer("minimal")).toBeNull();
  });
});
