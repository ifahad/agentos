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
