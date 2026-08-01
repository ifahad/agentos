"""Render every console route in both themes and audit text contrast.

Run against a dev server:  npm run dev -- --port 5199
Then:  connectors/browser/.venv/bin/python console/scripts/render.py http://localhost:5199 out/

getComputedStyle reports `color` ignoring `opacity`, so a ratio computed from
it alone overstates contrast on anything faded. Every measurement here
composites opacity against the resolved background first.
"""
import sys, pathlib
from playwright.sync_api import sync_playwright

ROUTES = ["/", "/keys", "/audit", "/playground", "/documents", "/improve",
          "/multiverse", "/operators", "/docs", "/orgs", "/users", "/secrets",
          "/provisioning"]

# State badges only exist in the DOM once real gateway/audit data renders one
# — every route in this sweep runs against an unconfigured gateway, so no
# route ever puts a `.badge` on screen and 734 measured elements across 26
# renders included zero of them. That let a real defect (--hold/--ok/--deny
# fail 4.5:1 as 10px badge text on the light ground) through a green run.
# Rather than depend on seeded backend data, inject one of each variant
# directly — same markup ui/Badge.tsx renders — into the route that is
# thematically theirs, so the audit that already runs on every route sees
# them too instead of needing a parallel code path.
#
# The wrapper is given `class="panel"` rather than reused from whatever the
# route happens to already have on screen: in the unconfigured-gateway state
# this sweep runs in, /audit renders no `.panel` at all (just the notice),
# so an earlier version of this injection fell back to `document.body` —
# `--bg`, not the `--raised` background a real badge always sits on inside a
# populated audit table — and reported guardrail_flag/guardrail_block/fail/
# failed_evals failing at 3.8-3.83:1. That was a false positive from the
# injection site, not the badge: production badges never render directly on
# `--bg`, only inside a `.panel`. Taking `.panel`'s own background from
# styles.css, rather than hardcoding --raised here, keeps this honest if
# that token ever moves.
BADGE_ROUTE = "/audit"
BADGE_VARIANTS = ["chat", "embeddings", "guardrail_flag", "guardrail_block",
                   "pass", "fail", "passed_evals", "failed_evals", "approved",
                   "denied", "inactive"]

INJECT_BADGES_JS = f"""
() => {{
  const wrap = document.createElement('div');
  wrap.id = '__badge-audit-probe';
  wrap.className = 'panel';
  wrap.style.cssText = 'display:flex;flex-wrap:wrap;gap:8px;padding:16px;';
  for (const v of {BADGE_VARIANTS!r}) {{
    const b = document.createElement('span');
    b.className = 'badge ' + v;
    b.textContent = v;
    wrap.appendChild(b);
  }}
  document.body.prepend(wrap);
}}
"""

AUDIT_JS = r"""
() => {
  const lum = (r,g,b) => { const f = c => { c/=255; return c<=0.03928 ? c/12.92 : Math.pow((c+0.055)/1.055,2.4); };
    return 0.2126*f(r)+0.7152*f(g)+0.0722*f(b); };
  // getComputedStyle serializes most colours as rgb()/rgba() on a 0-255
  // scale, but Chromium serializes color-mix() results that resolve to a
  // predefined colour space (e.g. "color-mix(in srgb, ...)" — the house
  // idiom this codebase uses for every translucent fill) as
  // `color(srgb R G B / A)` on a 0-1 scale instead. A bare digit-extracting
  // regex reads 0.556863 as if it were on the 0-255 scale, i.e. as
  // near-black, which is exactly backwards for a light tint. Badge text
  // measured against its own color-mix background hit this: the badge's
  // true background (its stated colour at low alpha, composited over the
  // panel under it) was being read as almost-black, corrupting every ratio
  // involving a color-mix background.
  const parse = s => {
    const cf = s.match(/^color\(srgb\s+([\d.]+)\s+([\d.]+)\s+([\d.]+)(?:\s*\/\s*([\d.]+))?\)/);
    if (cf) return [+cf[1]*255, +cf[2]*255, +cf[3]*255, cf[4] === undefined ? 1 : +cf[4]];
    const n = (s.match(/[\d.]+/g)||[]).map(Number);
    return [n[0]??0, n[1]??0, n[2]??0, n[3] === undefined ? 1 : n[3]];
  };
  const over = (fg, bg, a) => fg.map((c,i) => c*a + bg[i]*(1-a));
  // A translucent background (any alpha under 1 — a badge's colour-mix tint,
  // an overlay scrim) does not replace what is behind it, it blends with
  // it. Walk every ancestor, not just the first with a non-zero alpha, and
  // composite front-to-back onto an assumed-opaque canvas so a barely-
  // tinted layer reads as barely-tinted rather than as its own undiluted
  // pigment.
  const bgOf = el => {
    const layers = [];
    let n = el;
    while (n) {
      const [r,g,b,a] = parse(getComputedStyle(n).backgroundColor);
      if (a > 0) layers.push([r,g,b,a]);
      if (a >= 0.999) break;
      n = n.parentElement;
    }
    let result = [255,255,255];
    for (let i = layers.length - 1; i >= 0; i--) {
      const [r,g,b,a] = layers[i];
      result = [r*a+result[0]*(1-a), g*a+result[1]*(1-a), b*a+result[2]*(1-a)];
    }
    return result;
  };
  const out = [];
  let measured = 0;
  for (const el of document.querySelectorAll('*')) {
    // SVG <title>/<desc> hold accessible-name text that is never painted —
    // no browser puts pixels on screen for them, so a contrast ratio
    // computed against inherited color is meaningless.
    const tag = el.tagName.toLowerCase();
    if (tag === 'title' || tag === 'desc') continue;
    // A disabled control's label has no WCAG 1.4.3 contrast requirement
    // ("inactive user interface component" is explicitly exempt), and
    // fading it is the only visual cue that it is not actionable.
    if (el.matches(':disabled') || el.closest(':disabled')) continue;
    const t = [...el.childNodes].some(n => n.nodeType === 3 && n.textContent.trim());
    if (!t) continue;
    const cs = getComputedStyle(el);
    if (cs.visibility === 'hidden' || cs.display === 'none') continue;
    const a = parseFloat(cs.opacity);
    const bg = bgOf(el);
    const fg = over(parse(cs.color).slice(0,3), bg, isNaN(a) ? 1 : a);
    const L1 = lum(...fg), L2 = lum(...bg);
    const ratio = (Math.max(L1,L2)+0.05)/(Math.min(L1,L2)+0.05);
    const size = parseFloat(cs.fontSize), bold = parseInt(cs.fontWeight) >= 700;
    const floor = (size >= 24 || (size >= 18.66 && bold)) ? 3.0 : 4.5;
    measured++;
    if (ratio < floor) out.push({tag: el.tagName, cls: el.className, text: el.textContent.trim().slice(0,40),
                                 ratio: +ratio.toFixed(2), floor, size});
  }
  return {failures: out, measured};
}
"""

def main():
    base, outdir = sys.argv[1], pathlib.Path(sys.argv[2])
    outdir.mkdir(parents=True, exist_ok=True)
    failures = 0
    with sync_playwright() as p:
        b = p.chromium.launch()
        for theme in ("dark", "light"):
            for route in ROUTES:
                pg = b.new_page(viewport={"width": 1440, "height": 900})
                pg.goto(base + route, wait_until="networkidle")
                pg.evaluate(f"document.documentElement.setAttribute('data-theme','{theme}')")
                pg.wait_for_timeout(600)
                if route == BADGE_ROUTE:
                    pg.evaluate(INJECT_BADGES_JS)
                    badge_count = pg.evaluate("document.querySelectorAll('#__badge-audit-probe .badge').length")
                    print(f"INJECTED {theme}{route}: {badge_count} badge variants "
                          f"({'OK' if badge_count == len(BADGE_VARIANTS) else 'EXPECTED ' + str(len(BADGE_VARIANTS)) + ' — CHECK INJECTION'})")
                name = (route.strip("/") or "overview").replace("/", "-")
                pg.screenshot(path=str(outdir / f"{theme}-{name}.png"), full_page=True)
                result = pg.evaluate(AUDIT_JS)
                print(f"MEASURED {theme}{route}: {result['measured']} elements")
                for bad in result['failures']:
                    failures += 1
                    print(f"FAIL {theme}{route}: {bad['ratio']}:1 < {bad['floor']} "
                          f"{bad['tag']}.{bad['cls']} {bad['size']}px {bad['text']!r}")
                pg.close()
        b.close()
    print(f"\n{failures} contrast failures")
    sys.exit(1 if failures else 0)

main()
