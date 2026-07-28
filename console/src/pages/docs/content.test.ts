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

describe("taxonomy", () => {
  it("matches the canonical section order shared with the GitHub docs", () => {
    expect(DOC_SECTIONS.map((s) => s.id)).toEqual([
      "overview",
      "concepts",
      "architecture",
      "gateway",
      "runtime",
      "sandbox",
      "connectors",
      "console",
      "quickstart",
      "configuration",
      "deploy",
      "security",
    ]);
  });

  it("titles match the canonical taxonomy verbatim", () => {
    expect(DOC_SECTIONS.map((s) => s.title)).toEqual([
      "Overview",
      "Concepts & Glossary",
      "Architecture",
      "Gateway",
      "Runtime",
      "Sandbox",
      "Connectors",
      "Console",
      "Quickstart",
      "Configuration",
      "Deploy",
      "Security",
    ]);
  });
});

describe("visual placement", () => {
  const diagramsIn = (id: string) =>
    (DOC_SECTIONS.find((s) => s.id === id)?.blocks ?? [])
      .filter((b) => b.kind === "diagram")
      .map((b) => (b.kind === "diagram" ? b.diagram : ""));

  it("binds each visual to its section", () => {
    expect(diagramsIn("overview")).toContain("requestLifecycle");
    expect(diagramsIn("architecture")).toContain("architecture");
    expect(diagramsIn("gateway")).toContain("governanceChain");
    expect(diagramsIn("runtime")).toContain("councilFanout");
  });

  it("uses every declared diagram at least once", () => {
    const used = new Set(
      DOC_SECTIONS.flatMap((s) => s.blocks)
        .filter((b) => b.kind === "diagram")
        .map((b) => (b.kind === "diagram" ? b.diagram : "")),
    );
    for (const key of DIAGRAM_KEYS) expect(used).toContain(key);
  });
});

describe("content accuracy guards", () => {
  const all = DOC_SECTIONS.flatMap((s) => s.blocks).map(blockToText).join(" ");

  it("never claims skills are sha256-pinned", () => {
    expect(all).not.toMatch(/sha256-pinned/i);
  });

  it("never claims every outcome or every denial is audited", () => {
    expect(all).not.toMatch(/every outcome/i);
    expect(all).not.toMatch(/including denials/i);
  });

  it("spells the guardrail variable with the plural GUARDRAILS", () => {
    expect(all).not.toMatch(/AGENTOS_GUARDRAIL_MODE/);
  });

  it("never groups scim or oidc routes as role-checked", () => {
    expect(all).not.toMatch(/RBAC[^.]*\/scim/i);
  });
});
