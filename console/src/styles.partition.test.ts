import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = join(process.cwd(), "src");

/** Every .css file under src/, as [relative path, contents]. */
function cssFiles(): Array<[string, string]> {
  const out: Array<[string, string]> = [];
  (function walk(dir: string) {
    for (const name of readdirSync(dir)) {
      const full = join(dir, name);
      if (statSync(full).isDirectory()) walk(full);
      else if (name.endsWith(".css")) out.push([full.slice(SRC.length + 1), readFileSync(full, "utf8")]);
    }
  })(SRC);
  return out;
}

/** --v, --v2 or --v3, but not --vertical and not --v-something. */
const VIOLET = /--v[23]?\b/;

/** Surfaces that depict machine outcomes. Chroma there means state, not brand. */
const INSTRUMENT_FILES = ["components/Chain.css", "pages/Audit.css", "charts/charts.css"];

describe("chroma partition", () => {
  it("finds the instrument files it is meant to guard", () => {
    const present = cssFiles().map(([p]) => p);
    for (const f of INSTRUMENT_FILES) expect(present).toContain(f);
  });

  it("keeps violet off instrument surfaces", () => {
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
    const css = readFileSync(join(SRC, "styles.css"), "utf8");
    // Strip comments first so a comment's text (which may itself say ".badge"
    // or contain brace-like characters) can never be mistaken for a selector
    // or split the block stream. What's left is pure rule syntax, so a naive
    // split on "}" then "{" is safe: CSS rules do not nest, so each segment
    // between two "}" boundaries holds exactly one selector and one body.
    const withoutComments = css.replace(/\/\*[\s\S]*?\*\//g, "");
    for (const block of withoutComments.split("}")) {
      const [selector, body = ""] = block.split("{");
      if (!selector.includes(".badge")) continue;
      expect(VIOLET.test(body), `badge rule "${selector.trim()}" must stay monochrome or signal`).toBe(false);
    }
  });

  it("never uses --v as a text colour", () => {
    // --v is 4.43:1 on --bg, below the 4.5:1 floor. It is a fill.
    const COLOR_DECL = /(?<![-\w])color\s*:\s*[^;{}]*var\(\s*--v\s*\)/;
    const offenders: string[] = [];
    for (const [path, css] of cssFiles()) {
      css.split("\n").forEach((line, i) => {
        if (COLOR_DECL.test(line)) offenders.push(`${path}:${i + 1} ${line.trim()}`);
      });
    }
    expect(offenders, "--v is fill-only; use --v2 for text").toEqual([]);
  });
});
