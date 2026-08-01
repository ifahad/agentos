import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = join(process.cwd(), "src");

/**
 * Removes /* ... *\/ comments but keeps every newline inside them, so line
 * numbers in any later per-line scan stay aligned with the original file.
 * Without this, a comment merely mentioning a token name (e.g. "never use
 * --v2 here") reads as a real use of that token.
 */
function stripComments(css: string): string {
  return css.replace(/\/\*[\s\S]*?\*\//g, (comment) => comment.replace(/[^\n]/g, ""));
}

/** Every .css file under src/, as [relative path, comment-stripped contents]. */
function cssFiles(): Array<[string, string]> {
  const out: Array<[string, string]> = [];
  (function walk(dir: string) {
    for (const name of readdirSync(dir)) {
      const full = join(dir, name);
      if (statSync(full).isDirectory()) walk(full);
      else if (name.endsWith(".css")) {
        out.push([full.slice(SRC.length + 1), stripComments(readFileSync(full, "utf8"))]);
      }
    }
  })(SRC);
  return out;
}

function stylesCss(): string {
  const found = cssFiles().find(([path]) => path === "styles.css");
  if (!found) throw new Error("styles.css not found under src/");
  return found[1];
}

/** Surfaces that depict machine outcomes. Chroma there means state, not brand. */
const INSTRUMENT_FILES = ["components/Chain.css", "pages/Audit.css", "charts/charts.css"];

/**
 * Custom properties declared anywhere in `css` (already comment-stripped)
 * whose value resolves — directly, or through a chain of `var(--x)` aliases
 * — to `--${root}`. Always includes `root` itself.
 *
 * A value only counts as an alias if it is *exactly* `var(--name)`.
 * Expressions like `color-mix(in srgb, var(--v) 16%, transparent)` are
 * deliberately not followed: they no longer carry the aliased colour
 * verbatim (a 16%-mixed tint is not the same fill or text risk as the
 * source token), so treating them as full aliases would be its own kind of
 * false positive.
 *
 * This is what lets `--accent` (`--accent: var(--v);`) get caught by the
 * same rule as `--v` without hardcoding the name "accent" anywhere: if a
 * new alias is added tomorrow, it is picked up automatically because it is
 * discovered by walking the `--name: value;` graph, not by string-matching
 * known names.
 */
function resolvesTo(css: string, root: string): Set<string> {
  const declRe = /--([\w-]+)\s*:\s*([^;]+);/g;
  const valueOf = new Map<string, string>();
  let m: RegExpExecArray | null;
  while ((m = declRe.exec(css))) valueOf.set(m[1], m[2].trim());

  const aliasOf = /^var\(\s*--([\w-]+)\s*\)$/;
  function chases(name: string, seen: Set<string>): boolean {
    if (name === root) return true;
    if (seen.has(name)) return false; // cycle guard
    seen.add(name);
    const match = aliasOf.exec(valueOf.get(name) ?? "");
    return match ? chases(match[1], seen) : false;
  }

  const found = new Set<string>([root]);
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

describe("chroma partition", () => {
  it("finds the instrument files it is meant to guard", () => {
    const present = cssFiles().map(([p]) => p);
    for (const f of INSTRUMENT_FILES) expect(present).toContain(f);
  });

  it("keeps violet off instrument surfaces", () => {
    // Every custom property that is violet or resolves to violet (--v, --v2,
    // --v3, or an alias like --accent) is banned here, not just the three
    // literal token names — an instrument surface reached through an alias
    // is still an instrument surface reached by violet.
    const styles = stylesCss();
    const violetNames = new Set([
      ...resolvesTo(styles, "v"),
      ...resolvesTo(styles, "v2"),
      ...resolvesTo(styles, "v3"),
    ]);
    const VIOLET = wholeTokenPattern(violetNames);

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
    const styles = stylesCss();
    const violetNames = new Set([
      ...resolvesTo(styles, "v"),
      ...resolvesTo(styles, "v2"),
      ...resolvesTo(styles, "v3"),
    ]);
    const VIOLET = wholeTokenPattern(violetNames);

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
