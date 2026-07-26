export interface Column<T> {
  header: string;
  value: (row: T) => string | number;
}

function csvCell(raw: string | number): string {
  const s = String(raw);
  return /[",\r\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

/** RFC-4180-ish CSV: header + rows, CRLF-joined, minimal quoting. */
export function toCSV<T>(rows: readonly T[], columns: readonly Column<T>[]): string {
  const head = columns.map((c) => csvCell(c.header)).join(",");
  const body = rows.map((row) => columns.map((c) => csvCell(c.value(row))).join(","));
  return [head, ...body].join("\r\n");
}

/** Pretty JSON array of header-keyed objects. */
export function toJSON<T>(rows: readonly T[], columns: readonly Column<T>[]): string {
  return JSON.stringify(
    rows.map((row) => Object.fromEntries(columns.map((c) => [c.header, c.value(row)]))),
    null,
    2,
  );
}

/** Trigger a client-side download. Browser-only; no-op where document is absent. */
export function downloadBlob(filename: string, mime: string, text: string): void {
  if (typeof document === "undefined") return;
  const url = URL.createObjectURL(new Blob([text], { type: mime }));
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}
