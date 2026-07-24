import type { CSSProperties, ReactNode } from "react";

export interface SkeletonProps {
  width?: number | string;
  height?: number | string;
  /** Render as a circle (avatars, dots). */
  circle?: boolean;
  /** Number of stacked text lines (overrides width/height). */
  lines?: number;
  style?: CSSProperties;
}

/** Shimmer placeholder — replaces "loading…" text. */
export function Skeleton({ width, height = 14, circle, lines, style }: SkeletonProps) {
  if (lines && lines > 1) {
    return (
      <span className="ui-skeleton-lines" style={style}>
        {Array.from({ length: lines }, (_, i) => (
          <span
            key={i}
            className="ui-skeleton"
            style={{ width: i === lines - 1 ? "60%" : "100%", height }}
          />
        ))}
      </span>
    );
  }
  const size: CSSProperties = circle
    ? { width: width ?? height, height: width ?? height, borderRadius: "50%" }
    : { width: width ?? "100%", height };
  return <span className="ui-skeleton" style={{ ...size, ...style }} />;
}

export interface EmptyStateProps {
  title: ReactNode;
  description?: ReactNode;
  /** Optional call-to-action (usually a Button). */
  action?: ReactNode;
}

/** Centered, dim empty state with optional action. */
export function EmptyState({ title, description, action }: EmptyStateProps) {
  return (
    <div className="ui-empty">
      <div className="ui-empty-title">{title}</div>
      {description != null && <div className="ui-empty-desc">{description}</div>}
      {action != null && <div className="ui-empty-action">{action}</div>}
    </div>
  );
}
