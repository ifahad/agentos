import { useEffect, useState } from "react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import type { PageProps } from "../App";
import { PageHead } from "../components/common";
import { Panel, PanelHead, fadeRise, fadeRiseReduced, transition } from "../ui";
import { Icon } from "../ui/icons";
import { DOC_SECTIONS } from "./docs/content";
import { DocBlocks } from "./docs/DocBlocks";
import "./Docs.css";

/** Read ?s=<id> once on mount. The router only ever looks at pathname, so an
 *  inbound /docs?s=gateway link lands here correctly. */
function initialSectionId(): string {
  if (typeof window === "undefined") return DOC_SECTIONS[0].id;
  const wanted = new URLSearchParams(window.location.search).get("s");
  return DOC_SECTIONS.some((s) => s.id === wanted) ? (wanted as string) : DOC_SECTIONS[0].id;
}

export function Docs(_props: PageProps) {
  const reduced = useReducedMotion();
  const [activeId, setActiveId] = useState(initialSectionId);

  const active = DOC_SECTIONS.find((s) => s.id === activeId) ?? DOC_SECTIONS[0];

  useEffect(() => {
    // replaceState, never the router's navigate(): navigate stores its whole
    // argument as the route key and matches by exact equality, so a query-bearing
    // path silently falls through to Overview.
    window.history.replaceState(null, "", `/docs?s=${active.id}`);
  }, [active.id]);

  return (
    <>
      <PageHead
        title="Docs"
        subtitle="What this platform does, how the governance works, and how to run it — without leaving the console."
      />

      <div className="docs-layout">
        <nav className="docs-rail" aria-label="Documentation sections">
          {DOC_SECTIONS.map((s) => {
            const isActive = s.id === active.id;
            return (
              <button
                key={s.id}
                type="button"
                className={`docs-rail-item${isActive ? " active" : ""}`}
                aria-current={isActive ? "true" : undefined}
                onClick={() => setActiveId(s.id)}
              >
                {isActive &&
                  (reduced ? (
                    <span className="docs-rail-pill" aria-hidden />
                  ) : (
                    <motion.span
                      className="docs-rail-pill"
                      aria-hidden
                      layoutId="docs-rail-pill"
                      transition={transition}
                    />
                  ))}
                <Icon name={s.icon} size={14} className="docs-rail-icon" />
                <span className="docs-rail-label">{s.title}</span>
              </button>
            );
          })}
        </nav>

        <div className="docs-body">
          <Panel>
            <PanelHead title={active.title} />
            <div className="panel-body">
              <p className="docs-blurb">{active.blurb}</p>
              <AnimatePresence mode="wait" initial={false}>
                <motion.div
                  key={active.id}
                  variants={reduced ? fadeRiseReduced : fadeRise}
                  initial="hidden"
                  animate="show"
                  exit="exit"
                >
                  <DocBlocks blocks={active.blocks} />
                </motion.div>
              </AnimatePresence>
            </div>
          </Panel>
        </div>
      </div>
    </>
  );
}
