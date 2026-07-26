import type { ButtonHTMLAttributes, ReactNode } from "react";
import { Icon } from "./icons";
import type { IconName } from "./icons";

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  /** default = ghost (inset surface, hairline border) */
  variant?: "primary" | "ghost" | "danger";
  small?: boolean;
  /** Optional leading glyph. Decorative — the button's text (or aria-label) names the action. */
  icon?: IconName;
  /** No visible label. REQUIRES an aria-label (or title) on the button for accessibility. */
  iconOnly?: boolean;
  children?: ReactNode;
}

/**
 * Token-driven button. Press-scale micro-interaction (90ms) lives in
 * styles.css on `.btn` so legacy markup gets it too.
 */
export function Button({
  variant = "ghost",
  small,
  icon,
  iconOnly,
  className = "",
  children,
  ...rest
}: ButtonProps) {
  const cls = [
    "btn",
    variant === "primary" ? "primary" : "",
    variant === "danger" ? "danger" : "",
    small ? "small" : "",
    iconOnly ? "icon-only" : "",
    className,
  ]
    .filter(Boolean)
    .join(" ");
  return (
    <button className={cls} {...rest}>
      {icon ? <Icon name={icon} size={small ? 12 : 13} className="btn-icon" /> : null}
      {children}
    </button>
  );
}
