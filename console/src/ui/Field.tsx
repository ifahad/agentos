import type { InputHTMLAttributes, ReactNode, SelectHTMLAttributes, TextareaHTMLAttributes } from "react";

/**
 * Form controls. The animated accent focus ring (border + 3px accent-dim
 * ring, 120ms) is global in styles.css; these wrappers add an optional
 * label so pages stop hand-rolling `label.field` markup.
 */

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: ReactNode;
}

export function Input({ label, id, className = "", type = "text", ...rest }: InputProps) {
  // type defaults to "text" because styles.css keys form styling on
  // input[type="text"], input[type="number"], … — a bare <input> matches none
  // of it and falls back to user-agent styling. Since color-scheme is bound to
  // the theme, that meant UA dark fields with a 2.43:1 placeholder.
  const input = <input id={id} className={className} type={type} {...rest} />;
  if (label == null) return input;
  return (
    <label className="field" htmlFor={id}>
      <span>{label}</span>
      {input}
    </label>
  );
}

export interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label?: ReactNode;
}

export function Select({ label, id, className = "", children, ...rest }: SelectProps) {
  const select = (
    <select id={id} className={className} {...rest}>
      {children}
    </select>
  );
  if (label == null) return select;
  return (
    <label className="field" htmlFor={id}>
      <span>{label}</span>
      {select}
    </label>
  );
}

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: ReactNode;
}

export function Textarea({ label, id, className = "", ...rest }: TextareaProps) {
  const area = <textarea id={id} className={className} {...rest} />;
  if (label == null) return area;
  return (
    <label className="field" htmlFor={id}>
      <span>{label}</span>
      {area}
    </label>
  );
}
