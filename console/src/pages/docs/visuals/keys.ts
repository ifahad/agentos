/**
 * Diagram identity, kept in a pure module so the content model and the tests can
 * reference a visual without pulling React in. The registry in `registry.tsx` is
 * typed `Record<DiagramKey, …>`, so adding a key here without a component is a
 * compile error — that is the completeness guarantee.
 */
export const DIAGRAM_KEYS = [
  "architecture",
  "governanceChain",
  "requestLifecycle",
  "councilFanout",
] as const;

export type DiagramKey = (typeof DIAGRAM_KEYS)[number];

/**
 * Every visual on this page is an illustration, not an instrument. The console's
 * live chain is evidence-driven by contract; these are scripted. The marker is
 * rendered on the visual itself so the distinction survives a screenshot.
 */
export const DIAGRAM_META: Record<
  DiagramKey,
  { title: string; marker: string; usesStateHues: boolean }
> = {
  architecture: {
    title: "The four planes",
    marker: "illustration · not live data",
    usesStateHues: true,
  },
  governanceChain: {
    title: "What a model call clears",
    marker: "illustration · scripted sequence",
    usesStateHues: true,
  },
  requestLifecycle: {
    title: "One governed request, end to end",
    marker: "illustration · not live data",
    usesStateHues: false,
  },
  councilFanout: {
    title: "Many models, one verdict",
    marker: "illustration · not live data",
    usesStateHues: false,
  },
};
