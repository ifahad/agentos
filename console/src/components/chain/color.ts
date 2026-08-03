import type { ChainOutcome } from "../../lib/chain";
import type { ChainPalette } from "./palette";

/**
 * Alpha compositing for canvas colour.
 *
 * Renderers draw the same token at many opacities — a gate at 16% at rest and
 * 71% mid-flare — but a CSS custom property resolves to whatever the theme
 * declared: `#07080b`, `#abc`, or `rgba(255,255,255,0.13)`. This normalises
 * all of them to an rgba() string at the requested alpha.
 *
 * Anything it cannot parse is returned unchanged rather than coerced. A named
 * colour or an unsupported colour space must degrade to a visible wrong-alpha
 * mark, never to `rgba(NaN,NaN,NaN,a)`, which paints nothing at all and would
 * silently blank a stage the operator is relying on.
 */
export function alpha(color: string, a: number): string {
  const c = color.trim();

  if (c.startsWith("#")) {
    const hex = c.slice(1);
    if (!/^[0-9a-fA-F]{3}$|^[0-9a-fA-F]{6}$/.test(hex)) return color.trim();
    const full =
      hex.length === 3
        ? hex
            .split("")
            .map((ch) => ch + ch)
            .join("")
        : hex;
    const n = parseInt(full, 16);
    return `rgba(${(n >> 16) & 255},${(n >> 8) & 255},${n & 255},${a})`;
  }

  const nums = c.match(/[\d.]+/g);
  if (c.startsWith("rgb") && nums && nums.length >= 3) {
    return `rgba(${nums[0]},${nums[1]},${nums[2]},${a})`;
  }

  return c;
}

/**
 * Colour for any mark that says "the request stopped here."
 *
 * This encodes the one distinction lib/chain.ts:20-29 spends its longest
 * comment forbidding the console to blur. `deny` is AgentOS refusing the
 * request — a governance stage said no. `fail` is AgentOS clearing the whole
 * gauntlet and the *provider* then breaking, which `chainStateFromEntry`
 * reports as `outcome: "fail"`, `stoppedAt: "upstream"`. Painting that in the
 * governance-denial hue tells the operator the gateway rejected a request it
 * actually allowed, and blames the wrong side of the proxy.
 *
 * Every renderer needs the same answer for gate flares, sparks, burn marks,
 * particle trails and stage labels, so it lives here once rather than as four
 * hand-copied ternaries — one of which (corridor's) had already drifted.
 *
 * `pass` cannot reach a stopped mark: `stoppedAt` is null on a pass, so no
 * renderer has a stop position to draw. It resolves to the denial hue anyway
 * rather than to something inert, on the same fail-closed principle
 * `chainStateFromEntry`'s default branch uses — a caller that asks for the
 * stop colour of a passing request has a bug, and a visible red mark is how
 * it gets found instead of silently reading as a clean pass.
 */
export function stopColor(outcome: ChainOutcome, palette: ChainPalette): string {
  return outcome === "fail" ? palette.hold : palette.deny;
}
