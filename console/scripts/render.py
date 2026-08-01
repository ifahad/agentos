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

AUDIT_JS = r"""
() => {
  const lum = (r,g,b) => { const f = c => { c/=255; return c<=0.03928 ? c/12.92 : Math.pow((c+0.055)/1.055,2.4); };
    return 0.2126*f(r)+0.7152*f(g)+0.0722*f(b); };
  const parse = s => (s.match(/[\d.]+/g)||[]).map(Number);
  const over = (fg, bg, a) => fg.map((c,i) => c*a + bg[i]*(1-a));
  const bgOf = el => { let n = el; while (n) { const c = parse(getComputedStyle(n).backgroundColor);
      if (c.length >= 3 && (c[3] === undefined || c[3] > 0)) return c.slice(0,3); n = n.parentElement; }
    return [0,0,0]; };
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
