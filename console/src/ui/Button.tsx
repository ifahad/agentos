import type { ButtonHTMLAttributes, ReactNode } from "react";

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  /** default = ghost (inset surface, hairline border) */
  variant?: "primary" | "ghost" | "danger";
  small?: boolean;
  children: ReactNode;
}

/**
 * Token-driven button. Press-scale micro-interaction (90ms) lives in
 * styles.css on `.btn` so legacy markup gets it too.
 */
export function Button({ variant = "ghost", small, className = "", children, ...rest }: ButtonProps) {
  const cls = [
    "btn",
    variant === "primary" ? "primary" : "",
    variant === "danger" ? "danger" : "",
    small ? "small" : "",
    className,
  ]
    .filter(Boolean)
    .join(" ");
  return (
    <button className={cls} {...rest}>
      {children}
    </button>
  );
}
