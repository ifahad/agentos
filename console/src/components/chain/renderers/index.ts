/**
 * Style -> renderer. Adding a style is one file plus one line here.
 *
 * `minimal` deliberately has no renderer: it is the original DOM chain, kept
 * for low-power machines and anyone who does not want an animated canvas above
 * their content. Chain.tsx branches on the null.
 */

import type { ChainStyle } from "../../../lib/chainStyle";
import { createCorridorRenderer } from "./corridor";
import { createFlowRenderer } from "./flow";
import { createPhosphorRenderer } from "./phosphor";
import type { ChainRenderer } from "./types";

export function createRenderer(style: ChainStyle): ChainRenderer | null {
  switch (style) {
    case "phosphor":
      return createPhosphorRenderer();
    case "corridor":
      return createCorridorRenderer();
    case "flow":
      return createFlowRenderer();
    case "minimal":
      return null;
  }
}

export type { ChainRenderer, RenderFrame, LivePacket } from "./types";
