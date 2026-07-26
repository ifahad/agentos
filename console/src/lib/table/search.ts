/** Case-insensitive substring filter over chosen fields. Empty query → all rows (copied). */
export function filterBySearch<T>(rows: readonly T[], fields: readonly (keyof T)[], query: string): T[] {
  const q = query.trim().toLowerCase();
  if (q === "") return rows.slice();
  return rows.filter((row) =>
    fields.some((f) => String(row[f] ?? "").toLowerCase().includes(q)),
  );
}
