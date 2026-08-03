import { describe, expect, it } from "vitest";
import { IDLE_CHAIN } from "../../../lib/chain";
import type { ChainState } from "../../../lib/chain";
import { CHAIN_STYLES } from "../../../lib/chainStyle";
import { FALLBACK_PALETTE } from "../palette";
import { createRenderer } from "./index";
import { frame, livePacket, stubCtx } from "./testHarness";

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

  it("draws no text in --text-faint, in any style or state", () => {
    // Design spec §6: "Stage labels remain readable at --text-dim (the
    // --text-faint contrast failure noted in Chain.css:86-90 must not be
    // reintroduced by the redesign)." --text-faint measures 2.65:1 on --bg,
    // below the 4.5:1 floor, and the stage labels are 8px uppercase mono — on
    // an idle chain every stage is unlit, so that was EVERY label in the
    // default style on every page. `faint` is for non-text furniture only.
    //
    // A sentinel value rather than the real token: this must catch the colour
    // whatever the theme resolves --text-faint to, including through alpha().
    const SENTINEL = "#ff00ff";
    const palette = { ...FALLBACK_PALETTE, faint: SENTINEL };
    const stopped: ChainState = { cleared: 1, stoppedAt: "rate", outcome: "deny", unproven: [] };
    const cleared: ChainState = {
      cleared: 6,
      stoppedAt: null,
      outcome: "pass",
      unproven: ["guardrail"],
    };
    const states: Array<[string, ChainState]> = [
      ["idle", IDLE_CHAIN],
      ["cleared", cleared],
      ["stopped", stopped],
    ];

    for (const style of CHAIN_STYLES) {
      if (style === "minimal") continue;
      for (const [name, state] of states) {
        for (const still of [false, true]) {
          const r = createRenderer(style)!;
          const { ctx, draws } = stubCtx();
          r.draw(frame({ ctx, palette, state, still, packets: [livePacket({ progress: 1 })] }));
          const text = draws.filter((d) => d.method === "fillText");
          expect(text.length, `${style}/${name} must draw some text`).toBeGreaterThan(0);
          const faintText = text.filter(
            (d) => String(d.fillStyle).includes("255,0,255") || String(d.fillStyle).includes(SENTINEL),
          );
          expect(faintText.map((d) => d.args[0]), `${style}/${name}/still=${still}`).toEqual([]);
        }
      }
    }
  });
});
