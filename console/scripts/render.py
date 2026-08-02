"""Render every console route in both themes and audit text contrast.

Run against a dev server:  npm run dev -- --port 5199
Then:  connectors/browser/.venv/bin/python console/scripts/render.py http://localhost:5199 out/

getComputedStyle reports `color` ignoring `opacity`, so a ratio computed from
it alone overstates contrast on anything faded. Every measurement here
composites opacity against the resolved background first.
"""
import sys, pathlib
from playwright.sync_api import sync_playwright, TimeoutError as PWTimeout

ROUTES = ["/", "/keys", "/audit", "/playground", "/documents", "/improve",
          "/multiverse", "/operators", "/docs", "/orgs", "/users", "/secrets",
          "/provisioning"]

# What each route must LEAD with — the text of its first .panel-head h2.
# This is the assertion that makes "what does this page lead with" mechanical
# rather than a matter of opinion. A page that gets reinverted (creation panel
# moved back above the reading panel) fails here. Only the six routes that
# this cycle inverted from create-first to read-first are asserted: the other
# seven have no create/read inversion to protect, and pinning their headings
# would fail the run on ordinary copy edits.
EXPECTED_LEAD = {
    "/keys": "Existing keys",
    "/documents": "Ingested documents",
    "/operators": "Operators",
    "/orgs": "Organizations",
    "/users": "Members",
    "/multiverse": "Council",
}

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
  // Whether the route's own content is present, checked in the SAME
  // execution context and at the SAME instant as the measurement above —
  // not in an earlier, separate wait_for_selector call. A selector check
  // made even a moment before this evaluate() runs leaves a gap: the app
  // can mount, get confirmed present, and then unmount or remount (a
  // client-side redirect, a dev-server HMR full-reload of the React tree)
  // before the audit actually runs. That gap is exactly how one run of
  // this sweep once measured 11 elements on /audit — the 11 injected test
  // badges (appended straight to <body>, outside React's tree, so they
  // survive a React remount) with zero of the ~28 real elements the route
  // always has, and 11 cleared the bare "did anything render" floor. Only
  // a check made atomically alongside the count closes that gap.
  return {failures: out, measured, mounted: document.querySelector('MOUNT_SELECTOR_PLACEHOLDER') !== null};
}
"""

# Every console route renders the sidebar unconditionally — App.tsx falls
# back to ROUTES[0] for an unknown path rather than rendering nothing, and
# the shell (Sidebar + governance chain + heading) is the one thing every
# route has regardless of auth state. `networkidle` alone isn't enough: it's
# satisfied the instant the network goes quiet, which is also true mid-flight
# during a client-side redirect, and — as an actual run of this sweep proved
# during hardening — even a `wait_for_selector` made before the audit isn't
# enough, because the app can mount, get confirmed present by that earlier
# check, and then unmount or remount (an HMR full-reload of the React tree,
# observed live on /audit: the page went from 39 measured elements to 11 —
# exactly the 11 test badges appended straight to <body>, outside React's
# tree, with every one of the ~28 real elements gone) before the audit
# actually runs. So MOUNT_SELECTOR is checked twice: once here, before doing
# any work, as a cheap early-out; and again inside AUDIT_JS itself, in the
# same evaluate() call as the element count, which is the check that
# actually matters because nothing can race between a check and a
# measurement that happen atomically together.
MOUNT_SELECTOR = ".sidebar"
AUDIT_JS = AUDIT_JS.replace("MOUNT_SELECTOR_PLACEHOLDER", MOUNT_SELECTOR)

# Real routes in this app measure 28-69 elements (see the sweep in the Task
# 6 report). 10 is comfortably below every legitimate count and comfortably
# above zero, so it catches "this route did not render" without being
# fragile to ordinary content variation. It is the secondary signal, though:
# `mounted` (see above) is what actually caught the one real anomaly found
# while hardening this, because that anomaly's count (11) cleared 10.
MIN_MEASURED = 10

# The text of the first .panel-head h2 on the page — what the route LEADS
# with. Read in the same evaluate() call site as everything else in this
# module reads DOM state atomically alongside its check (see the mounted
# comment above): a separate, earlier query_selector could observe a heading
# that a client-side remount has since replaced.
LEAD_JS = """
() => {
  const h = document.querySelector('.panel-head h2');
  return h ? h.textContent.trim() : null;
}
"""


def is_valid(result):
    return result["measured"] >= MIN_MEASURED and result.get("mounted", True)


def render_and_audit(browser, base, outdir, theme, route):
    """Load one route in one theme, wait for it to actually mount, screenshot
    and audit it. Returns the audit result dict.

    Nothing in here is allowed to raise past this function: a route that
    fails to mount, a navigation that errors outright, or a client-side
    redirect that destroys the execution context mid-evaluate (Playwright
    raises "Execution context was destroyed, most likely because of a
    navigation" for exactly this race — the failure mode this hardening
    exists for) must all come back as a low/zero `measured` count for the
    caller's floor check to report, not as an uncaught traceback that kills
    the sweep on whichever route happens to hit it."""
    pg = browser.new_page(viewport={"width": 1440, "height": 900})
    try:
        try:
            pg.goto(base + route, wait_until="networkidle")
            pg.evaluate(f"document.documentElement.setAttribute('data-theme','{theme}')")
            try:
                pg.wait_for_selector(MOUNT_SELECTOR, state="attached", timeout=8000)
            except PWTimeout:
                pass  # let the measured-count floor below report this, clearly labelled
            pg.wait_for_timeout(600)
            if route == BADGE_ROUTE:
                pg.evaluate(INJECT_BADGES_JS)
                badge_count = pg.evaluate("document.querySelectorAll('#__badge-audit-probe .badge').length")
                print(f"INJECTED {theme}{route}: {badge_count} badge variants "
                      f"({'OK' if badge_count == len(BADGE_VARIANTS) else 'EXPECTED ' + str(len(BADGE_VARIANTS)) + ' — CHECK INJECTION'})")
            name = (route.strip("/") or "overview").replace("/", "-")
            pg.screenshot(path=str(outdir / f"{theme}-{name}.png"), full_page=True)
            result = pg.evaluate(AUDIT_JS)
            result["lead"] = pg.evaluate(LEAD_JS)
            return result
        except Exception as e:
            print(f"WARN {theme}{route}: render/audit step raised {type(e).__name__}: "
                  f"{str(e).splitlines()[0]!r} — treating as an unmounted page")
            return {"measured": 0, "failures": [], "mounted": False, "lead": None}
    finally:
        pg.close()


def main():
    base, outdir = sys.argv[1], pathlib.Path(sys.argv[2])
    outdir.mkdir(parents=True, exist_ok=True)
    failures = 0
    harness_errors = 0
    lead_failures = 0
    coverage = {}  # (theme, route) -> measured count actually used
    valid = {}     # (theme, route) -> is_valid(result), for the summary flag
    with sync_playwright() as p:
        b = p.chromium.launch()
        for theme in ("dark", "light"):
            for route in ROUTES:
                result = render_and_audit(b, base, outdir, theme, route)
                attempts = 1
                if not is_valid(result):
                    why = "MOUNT_SELECTOR absent at audit time" if not result.get("mounted", True) \
                          else f"only {result['measured']} elements (floor {MIN_MEASURED})"
                    print(f"WARN {theme}{route}: {why} on attempt 1 — retrying once before failing the run")
                    result = render_and_audit(b, base, outdir, theme, route)
                    attempts = 2
                coverage[(theme, route)] = result["measured"]
                valid[(theme, route)] = is_valid(result)
                if not is_valid(result):
                    harness_errors += 1
                    why = "the mount selector was gone by the time the audit ran" if not result.get("mounted", True) \
                          else f"only {result['measured']} elements measured (floor {MIN_MEASURED})"
                    print(f"HARNESS FAILURE {theme}{route}: {why}, after {attempts} attempt(s). This route "
                          f"did not render — it is a harness failure, NOT a passing (contrast-clean) page.")
                else:
                    suffix = f" (needed a retry: attempt {attempts})" if attempts > 1 else ""
                    print(f"MEASURED {theme}{route}: {result['measured']} elements{suffix}")
                for bad in result["failures"]:
                    failures += 1
                    print(f"FAIL {theme}{route}: {bad['ratio']}:1 < {bad['floor']} "
                          f"{bad['tag']}.{bad['cls']} {bad['size']}px {bad['text']!r}")
                # A route that didn't mount is already counted as a harness
                # failure above; checking its lead too would just relabel the
                # same defect under a second, less accurate category, so this
                # only runs once the route is confirmed to have rendered.
                if route in EXPECTED_LEAD and is_valid(result):
                    expected = EXPECTED_LEAD[route]
                    if result.get("lead") != expected:
                        lead_failures += 1
                        print(f"WRONG LEAD {theme}{route}: expected first .panel-head h2 "
                              f"{expected!r}, got {result.get('lead')!r}")
        b.close()

    print("\n=== coverage summary (measured elements per route x theme) ===")
    for route in ROUTES:
        dark_n = coverage.get(("dark", route), "?")
        light_n = coverage.get(("light", route), "?")
        ok = valid.get(("dark", route), False) and valid.get(("light", route), False)
        flag = "" if ok else " <-- HARNESS FAILURE"
        print(f"  {route:<16} dark={dark_n:<4} light={light_n:<4}{flag}")

    if harness_errors:
        print(f"\n{harness_errors} route(s) did not render (harness failure — the run below is "
              f"NOT a certified 0-contrast-failures result, regardless of the count)")
    print(f"{failures} contrast failures")
    print(f"{lead_failures} wrong-lead failures")
    sys.exit(1 if (failures or harness_errors or lead_failures) else 0)

main()
