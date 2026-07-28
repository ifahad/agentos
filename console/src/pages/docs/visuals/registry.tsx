import type { JSX } from "react";
import { ArchitectureVisual } from "./ArchitectureVisual";
import { CouncilFanoutVisual } from "./CouncilFanoutVisual";
import { GovernanceChainVisual } from "./GovernanceChainVisual";
import { RequestLifecycleVisual } from "./RequestLifecycleVisual";
import type { DiagramKey } from "./keys";

/**
 * Every diagram a content block may name, bound to the component that draws it.
 *
 * Completeness is compile-enforced: the type is Record<DiagramKey, …>, so adding
 * a key to DIAGRAM_KEYS without a component here is a tsc error, and a `diagram`
 * block can never name a visual that does not exist.
 */
export const DIAGRAM_REGISTRY: Record<DiagramKey, () => JSX.Element> = {
  architecture: ArchitectureVisual,
  governanceChain: GovernanceChainVisual,
  requestLifecycle: RequestLifecycleVisual,
  councilFanout: CouncilFanoutVisual,
};
