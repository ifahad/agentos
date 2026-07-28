import { DIAGRAM_REGISTRY } from "./visuals/registry";
import type { DocBlock } from "./types";

function Block({ block }: { block: DocBlock }) {
  switch (block.kind) {
    case "prose":
      return <p className="docs-prose">{block.text}</p>;

    case "list":
      return (
        <ul className="docs-list">
          {block.items.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      );

    case "code":
      return (
        <div className="docs-code">
          {block.lang && <span className="docs-code-lang eyebrow">{block.lang}</span>}
          {/* prompt-text is the only container-safe block style in this app:
              .panel is overflow:hidden, so a bare <pre> is silently clipped. */}
          <pre className="prompt-text mono">{block.code}</pre>
        </div>
      );

    case "keyvals":
      return (
        <div className="docs-keyvals">
          {block.caption && <span className="docs-keyvals-cap eyebrow">{block.caption}</span>}
          <dl>
            {block.rows.map((row) => (
              <div key={row.k} className="docs-kv">
                <dt className="mono">{row.k}</dt>
                <dd>{row.v}</dd>
              </div>
            ))}
          </dl>
        </div>
      );

    case "note":
      // Monochrome by default: .warn and .error carry hue, and hue is reserved
      // for machine state.
      return <div className="notice docs-note">{block.text}</div>;

    case "diagram": {
      const Visual = DIAGRAM_REGISTRY[block.diagram];
      return (
        <>
          <Visual />
          {block.caption && <p className="docs-fig-caption">{block.caption}</p>}
        </>
      );
    }

    default: {
      const exhaustive: never = block;
      return exhaustive;
    }
  }
}

export function DocBlocks({ blocks }: { blocks: DocBlock[] }) {
  return (
    <>
      {blocks.map((block, i) => (
        <Block key={`${block.kind}-${i}`} block={block} />
      ))}
    </>
  );
}
