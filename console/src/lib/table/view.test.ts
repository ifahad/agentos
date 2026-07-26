import { describe, expect, it } from "vitest";
import { computeTableView } from "./view";
type Row = { name: string; kind: string; n: number };
const rows: Row[] = [
  { name: "prod", kind: "chat", n: 3 },
  { name: "dev", kind: "chat", n: 10 },
  { name: "stg", kind: "embeddings", n: 1 },
];
describe("computeTableView", () => {
  it("composes filter → search → sort → paginate and reports counts", () => {
    const v = computeTableView(rows, {
      facets: { kind: "chat" },
      searchFields: ["name"],
      query: "",
      sort: { key: "n", dir: "desc", numeric: true },
      page: 0,
      pageSize: 1,
    });
    expect(v.total).toBe(3);
    expect(v.filteredCount).toBe(2);         // kind=chat
    expect(v.filtered.map((r) => r.n)).toEqual([10, 3]); // sorted, pre-paginate
    expect(v.rows.map((r) => r.n)).toEqual([10]);        // page 0, size 1
    expect(v.pageCount).toBe(2);
  });
  it("no config → all rows, single page", () => {
    const v = computeTableView(rows, {});
    expect(v.rows).toHaveLength(3);
    expect(v.pageCount).toBe(1);
  });
});
