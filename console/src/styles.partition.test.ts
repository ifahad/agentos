import { describe, expect, it } from "vitest";

/**
 * Every .css file under src/, contents inlined at collect time via Vite's
 * `import.meta.glob`. This is a Vite project — `vite/client` types are
 * already in `tsconfig.json`'s `types` array — so reading CSS this way
 * needs no Node APIs at all. `node:fs`/`node:path`/`process.cwd()` have no
 * type declarations under this tsconfig (`@types/node` is not installed),
 * so a Node-API version of this file type-checks fine under `vitest run`
 * (which never runs `tsc`) while failing `tsc` / `npm run build` outright —
 * exactly the gap this test exists to prevent, just one layer up the
 * toolchain. `query: "?raw", import: "default"` with `eager: true` yields
 * plain file contents (not a CSS module, not a loader function).
 */
const rawCssModules = import.meta.glob("./**/*.css", { query: "?raw", import: "default", eager: true }) as Record<
  string,
  string
>;

/**
 * Removes /* ... *\/ comments but keeps every newline inside them, so line
 * numbers in any later per-line scan stay aligned with the original file.
 * Without this, a comment merely mentioning a token name (e.g. "never use
 * --v2 here") reads as a real use of that token.
 */
function stripComments(css: string): string {
  return css.replace(/\/\*[\s\S]*?\*\//g, (comment) => comment.replace(/[^\n]/g, ""));
}

/** Every .css file under src/, as [path relative to src/, comment-stripped contents]. */
function cssFiles(): Array<[string, string]> {
  // Glob keys are relative to this file, e.g. "./components/Chain.css";
  // strip the leading "./" so they compare equal to INSTRUMENT_FILES entries
  // like "components/Chain.css".
  return Object.entries(rawCssModules).map(([path, css]) => [path.replace(/^\.\//, ""), stripComments(css)]);
}

function stylesCss(): string {
  const found = cssFiles().find(([path]) => path === "styles.css");
  if (!found) throw new Error("styles.css not found under src/");
  return found[1];
}

/** Surfaces that depict machine outcomes. Chroma there means state, not brand. */
const INSTRUMENT_FILES = ["components/Chain.css", "pages/Audit.css", "charts/charts.css"];

/**
 * How strictly a custom property has to carry a colour before it counts as
 * "being" that colour.
 *
 * - `"alias"`: only an *exact* `var(--name)` value is followed.
 *   `color-mix(in srgb, var(--v) 16%, transparent)` is NOT followed, because
 *   a 16% tint is not the same contrast risk as the source token. This is
 *   the right test for the fill-only `color:` ban, whose whole argument is
 *   about the source token's own measured ratio (--v is 4.43:1 on --bg).
 *
 * - `"mention"`: a value counts if it MENTIONS a carrier anywhere, including
 *   inside a `color-mix()`, a gradient, or a shadow. This is the right test
 *   for the two SURFACE bans, which say "no violet on this surface at any
 *   strength" — a 16% violet wash on a chain node is still violet on an
 *   instrument surface, and the reviewer proved the alias-only rule could
 *   not see it: `.probe-mix { background: var(--accent-dim); }` in
 *   Chain.css left the guard at 4 passed / 0 failed.
 */
type Carrier = "alias" | "mention";

/**
 * Custom properties declared anywhere in `css` (already comment-stripped)
 * that carry `--${root}` under the given `mode`. Always includes `root`
 * itself.
 *
 * Either way the set is DISCOVERED by walking the `--name: value;` graph,
 * never by string-matching known names: that is what lets `--accent`
 * (`--accent: var(--v);`) and `--accent-dim` (a color-mix of `--v`) get
 * caught by the same rules as `--v`, and what makes an alias added tomorrow
 * picked up for free.
 *
 * `"mention"` iterates to a fixpoint rather than recursing, because a
 * mention edge can appear through a property that only became a carrier on a
 * previous pass (`--a: color-mix(… var(--v) …)`, `--b: color-mix(… var(--a)
 * …)`). Each pass can only add names, and there are finitely many, so it
 * terminates; re-testing only not-yet-found names keeps it cheap and makes
 * cycles harmless.
 */
function resolvesTo(css: string, root: string, mode: Carrier = "alias"): Set<string> {
  const declRe = /--([\w-]+)\s*:\s*([^;]+);/g;
  const valueOf = new Map<string, string>();
  let m: RegExpExecArray | null;
  while ((m = declRe.exec(css))) valueOf.set(m[1], m[2].trim());

  const found = new Set<string>([root]);

  if (mode === "mention") {
    for (let grew = true; grew; ) {
      grew = false;
      // Rebuilt each pass so newly found carriers are themselves matchable.
      // wholeTokenPattern is used rather than a plain substring test so
      // `--v-offset` is not read as a mention of `--v`.
      const carriers = wholeTokenPattern(found);
      for (const [name, value] of valueOf) {
        if (found.has(name)) continue;
        if (carriers.test(value)) {
          found.add(name);
          grew = true;
        }
      }
    }
    return found;
  }

  const aliasOf = /^var\(\s*--([\w-]+)\s*\)$/;
  function chases(name: string, seen: Set<string>): boolean {
    if (name === root) return true;
    if (seen.has(name)) return false; // cycle guard
    seen.add(name);
    const match = aliasOf.exec(valueOf.get(name) ?? "");
    return match ? chases(match[1], seen) : false;
  }

  for (const name of valueOf.keys()) {
    if (chases(name, new Set())) found.add(name);
  }
  return found;
}

/** Escape a literal string for use inside a RegExp character class/alternation. */
function reEscape(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/**
 * Matches any of `names` as a whole custom-property identifier — never as a
 * prefix of a longer one. `\b` alone is not enough: CSS custom-property
 * names may continue past a short name with a hyphen (e.g. `--v-offset`
 * continues past `--v`), and `\b` treats the letter/hyphen boundary as a
 * word boundary too, so `/--v\b/` wrongly matches inside `--v-offset`. The
 * negative lookahead requires the next character to be neither a word
 * character nor a hyphen, i.e. that the matched name is not itself a prefix
 * of a longer, unrelated identifier.
 */
function wholeTokenPattern(names: Iterable<string>): RegExp {
  const alternation = [...names].map(reEscape).join("|");
  return new RegExp(`--(?:${alternation})(?![\\w-])`);
}

/**
 * A pattern matching any custom property that carries violet — `--v`, `--v2`,
 * `--v3`, or anything reaching them under `mode`. The two surface bans pass
 * `"mention"`; the fill-only `color:` ban keeps `"alias"`.
 */
function violetPattern(css: string, mode: Carrier): RegExp {
  return wholeTokenPattern(
    new Set([...resolvesTo(css, "v", mode), ...resolvesTo(css, "v2", mode), ...resolvesTo(css, "v3", mode)]),
  );
}

describe("chroma partition", () => {
  it("finds the instrument files it is meant to guard", () => {
    const present = cssFiles().map(([p]) => p);
    for (const f of INSTRUMENT_FILES) expect(present).toContain(f);
  });

  it("keeps violet off instrument surfaces", () => {
    // Every custom property that IS violet or CARRIES violet (--v, --v2,
    // --v3, an alias like --accent, or a mix like --accent-dim) is banned
    // here, not just the three literal token names — an instrument surface
    // reached through an alias is still an instrument surface reached by
    // violet, and so is one reached through a 16% wash. "This surface is
    // graphite plus signal" admits no strength of brand chroma, so this ban
    // uses "mention", not "alias".
    const VIOLET = violetPattern(stylesCss(), "mention");

    const offenders: string[] = [];
    for (const [path, css] of cssFiles()) {
      if (!INSTRUMENT_FILES.includes(path)) continue;
      css.split("\n").forEach((line, i) => {
        if (VIOLET.test(line)) offenders.push(`${path}:${i + 1} ${line.trim()}`);
      });
    }
    expect(offenders, "these files depict machine state; violet is banned there").toEqual([]);
  });

  it("keeps violet out of badge rules", () => {
    // Same surface ban, same "mention" strictness as the instrument files: a
    // badge names a machine state, so a violet TINT behind one is the same
    // category error as a violet fill.
    const styles = stylesCss();
    const VIOLET = violetPattern(styles, "mention");

    // Comments are already stripped by stylesCss(), so a naive split on "}"
    // then "{" is safe: CSS rules do not nest, so each segment between two
    // "}" boundaries holds exactly one selector and one body, and neither
    // can contain leftover comment text.
    for (const block of styles.split("}")) {
      const [selector, body = ""] = block.split("{");
      if (!selector.includes(".badge")) continue;
      expect(VIOLET.test(body), `badge rule "${selector.trim()}" must stay monochrome or signal`).toBe(false);
    }
  });

  it("never uses --v, or anything that resolves to it, as a text colour", () => {
    // --v is 4.43:1 on --bg, below the 4.5:1 floor. It is a fill. --accent
    // is a bare alias for --v (styles.css: `--accent: var(--v);`), so a
    // `color:` on --accent ships the exact same failing contrast — the
    // token name differs, the pixels don't. Anything else that resolves to
    // --v inherits the same ban. --v2 and --v3 are excluded on purpose:
    // they are the text-safe rungs of the ladder.
    //
    // This one keeps "alias" while the two surface bans above use "mention",
    // and the asymmetry is deliberate rather than an oversight: this ban's
    // entire justification is --v's own measured ratio, and a 16% tint of it
    // is a different colour with a different ratio. Widening this to
    // "mention" would ban `color: var(--accent-dim)` on the strength of an
    // argument that does not apply to it.
    const violetFillNames = resolvesTo(stylesCss(), "v");
    const names = [...violetFillNames].map(reEscape).join("|");
    const COLOR_DECL = new RegExp(`(?<![-\\w])color\\s*:\\s*[^;{}]*var\\(\\s*--(?:${names})\\s*\\)`);

    const offenders: string[] = [];
    for (const [path, css] of cssFiles()) {
      css.split("\n").forEach((line, i) => {
        if (COLOR_DECL.test(line)) offenders.push(`${path}:${i + 1} ${line.trim()}`);
      });
    }
    expect(offenders, "--v (and its aliases) are fill-only; use --v2 for text").toEqual([]);
  });
});
