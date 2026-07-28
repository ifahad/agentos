import type { IconName } from "../../ui/icons";
import type { DiagramKey } from "./visuals/keys";

/** One renderable unit of documentation. */
export type DocBlock =
  | { kind: "prose"; text: string }
  | { kind: "list"; items: string[] }
  | { kind: "code"; code: string; lang?: string }
  | { kind: "keyvals"; caption?: string; rows: { k: string; v: string }[] }
  | { kind: "note"; text: string }
  | { kind: "diagram"; diagram: DiagramKey; caption?: string };

export interface DocSection {
  /** Stable slug — appears in the `?s=` deep link, so never rename one casually. */
  id: string;
  title: string;
  icon: IconName;
  /** One line under the section title. Also the rail's accessible description. */
  blurb: string;
  blocks: DocBlock[];
}

/**
 * Flatten a block to plain text. Drives the content invariants below and gives a
 * future in-page search something to index without touching the DOM.
 */
export function blockToText(block: DocBlock): string {
  switch (block.kind) {
    case "prose":
      return block.text;
    case "list":
      return block.items.join(" ");
    case "code":
      return block.code;
    case "keyvals":
      return [block.caption ?? "", ...block.rows.map((r) => `${r.k} ${r.v}`)].join(" ").trim();
    case "note":
      return block.text;
    case "diagram":
      return block.caption ?? "";
    default: {
      const exhaustive: never = block;
      return exhaustive;
    }
  }
}
