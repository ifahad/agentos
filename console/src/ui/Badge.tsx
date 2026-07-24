import type { ReactNode } from "react";

/** Same semantic variants as today's `.badge` classes. */
export type BadgeVariant =
  | "chat"
  | "embeddings"
  | "guardrail_flag"
  | "guardrail_block"
  | "pass"
  | "fail"
  | "passed_evals"
  | "failed_evals"
  | "approved"
  | "denied"
  | "inactive";

export interface BadgeProps {
  variant?: BadgeVariant;
  children: ReactNode;
}

/** Refined pill; styling lives in styles.css on `.badge`. */
export function Badge({ variant, children }: BadgeProps) {
  return <span className={variant ? `badge ${variant}` : "badge"}>{children}</span>;
}
