import type { ReactNode } from "react";
import { DIAGRAM_META } from "./keys";
import type { DiagramKey } from "./keys";

/**
 * Shared frame for every docs visual.
 *
 * The console's live instruments are evidence-driven — the chain in the topbar
 * is lit by recorded audit statuses, never by a timer. These are the opposite:
 * scripted pictures. The marker says so on the visual itself rather than in
 * surrounding prose, so the distinction survives a screenshot.
 */
export function Illustration({
  diagram,
  children,
}: {
  diagram: DiagramKey;
  children: ReactNode;
}) {
  const meta = DIAGRAM_META[diagram];
  return (
    <figure className="docs-fig">
      <figcaption className="docs-fig-head">
        <span className="docs-fig-title">{meta.title}</span>
        <span className="docs-fig-marker eyebrow">{meta.marker}</span>
      </figcaption>
      <div className="docs-fig-body">{children}</div>
    </figure>
  );
}
