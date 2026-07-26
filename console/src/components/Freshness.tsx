import { formatAgo } from "../lib/live/relativeTime";
import { useNowTick } from "../hooks/useNowTick";

/**
 * "updated Ns ago" for a panel. Mono + faint, recomputed every second.
 * Renders nothing until the first successful load stamps `updatedAt`.
 */
export function Freshness({ updatedAt, className }: { updatedAt: number | null; className?: string }) {
  const now = useNowTick(1000);
  if (updatedAt === null) return null;
  return (
    <span className={`freshness mono${className ? ` ${className}` : ""}`} aria-live="off">
      updated {formatAgo(now - updatedAt)}
    </span>
  );
}
