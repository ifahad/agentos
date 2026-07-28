import type { DocSection } from "./types";

export const DOC_SECTIONS: DocSection[] = [
  {
    id: "overview",
    title: "Overview",
    icon: "overview",
    blurb: "What AgentOS is, and the one guarantee everything else serves.",
    blocks: [
      {
        kind: "prose",
        text: "AgentOS is a self-hostable agentic operating layer: any LLM provider in, any legacy system out, with governed autonomous agents in between. Every model call flows through one gateway that holds the credentials, the budget, and the audit log — the agent runtime never holds a provider key.",
      },
      { kind: "diagram", diagram: "requestLifecycle" },
    ],
  },
  {
    id: "architecture",
    title: "Architecture",
    icon: "orgs",
    blurb: "Four planes, and the wiring that makes the guarantee hold.",
    blocks: [{ kind: "diagram", diagram: "architecture" }],
  },
];
