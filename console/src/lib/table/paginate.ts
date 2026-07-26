export interface Page<T> {
  rows: T[];
  page: number;
  pageCount: number;
}

/** Slice one page. size <= 0 → a single page of everything. page clamped in range. */
export function paginate<T>(rows: readonly T[], page: number, size: number): Page<T> {
  if (size <= 0) return { rows: rows.slice(), page: 0, pageCount: 1 };
  const pageCount = Math.max(1, Math.ceil(rows.length / size));
  const p = Math.min(Math.max(0, Math.floor(page)), pageCount - 1);
  return { rows: rows.slice(p * size, p * size + size), page: p, pageCount };
}
