export type SortDir = "asc" | "desc";
export interface SortSpec<T> {
  key: keyof T;
  dir: SortDir;
  /** Compare as numbers rather than by locale string order. */
  numeric?: boolean;
}

/** Stable sort by one column. `null` → a shallow copy (original order). */
export function sortRows<T>(rows: readonly T[], spec: SortSpec<T> | null): T[] {
  const out = rows.slice();
  if (!spec) return out;
  const sign = spec.dir === "desc" ? -1 : 1;
  out.sort((a, b) => {
    const av = a[spec.key];
    const bv = b[spec.key];
    let cmp: number;
    if (spec.numeric) {
      cmp = (Number(av) || 0) - (Number(bv) || 0);
    } else {
      cmp = String(av ?? "").localeCompare(String(bv ?? ""));
    }
    return cmp * sign;
  });
  return out;
}
