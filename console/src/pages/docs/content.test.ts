import { describe, expect, it } from "vitest";
import { DOC_SECTIONS } from "./content";
import { blockToText } from "./types";
import type { DocBlock } from "./types";
import { DIAGRAM_KEYS, DIAGRAM_META } from "./visuals/keys";

describe("DOC_SECTIONS", () => {
  it("has unique ids", () => {
    const ids = DOC_SECTIONS.map((s) => s.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it("uses url-safe slugs, because ids are deep-link query values", () => {
    for (const s of DOC_SECTIONS) expect(s.id).toMatch(/^[a-z0-9-]+$/);
  });

  it("gives every section a title, a blurb, and at least one block", () => {
    for (const s of DOC_SECTIONS) {
      expect(s.title.length).toBeGreaterThan(0);
      expect(s.blurb.length).toBeGreaterThan(0);
      expect(s.blocks.length).toBeGreaterThan(0);
    }
  });

  it("references only real diagram keys", () => {
    for (const s of DOC_SECTIONS) {
      for (const b of s.blocks) {
        if (b.kind === "diagram") expect(DIAGRAM_KEYS).toContain(b.diagram);
      }
    }
  });

  it("has no empty block anywhere", () => {
    for (const s of DOC_SECTIONS) {
      for (const b of s.blocks) {
        if (b.kind === "diagram") continue;
        expect(blockToText(b).trim().length).toBeGreaterThan(0);
      }
    }
  });
});

describe("DIAGRAM_META", () => {
  it("covers every diagram key", () => {
    expect(Object.keys(DIAGRAM_META).sort()).toEqual([...DIAGRAM_KEYS].sort());
  });

  it("gives every visual a non-empty illustration marker", () => {
    for (const key of DIAGRAM_KEYS) {
      expect(DIAGRAM_META[key].marker.trim().length).toBeGreaterThan(0);
    }
  });
});

describe("blockToText", () => {
  const cases: DocBlock[] = [
    { kind: "prose", text: "a" },
    { kind: "list", items: ["a", "b"] },
    { kind: "code", code: "make up", lang: "bash" },
    { kind: "keyvals", caption: "cap", rows: [{ k: "k", v: "v" }] },
    { kind: "note", text: "n" },
    { kind: "diagram", diagram: "architecture", caption: "c" },
  ];

  it("covers every block kind", () => {
    const kinds = new Set(cases.map((c) => c.kind));
    expect(kinds.size).toBe(6);
  });

  it("returns text for each kind", () => {
    expect(cases.map(blockToText)).toEqual(["a", "a b", "make up", "cap k v", "n", "c"]);
  });

  it("returns empty string for a diagram with no caption", () => {
    expect(blockToText({ kind: "diagram", diagram: "architecture" })).toBe("");
  });
});
