export type FacetSelection<T> = Partial<Record<keyof T, string>>;

/** Keep rows matching every facet with a non-empty value (string-equality). */
export function filterByFacets<T>(rows: readonly T[], facets: FacetSelection<T>): T[] {
  const active = (Object.entries(facets) as [keyof T, string | undefined][]).filter(
    ([, v]) => v !== undefined && v !== "",
  );
  if (active.length === 0) return rows.slice();
  return rows.filter((row) => active.every(([k, v]) => String(row[k] ?? "") === v));
}
