import { describe, expect, it } from "vitest";
import { filterByFacets } from "./filter";
import { paginate } from "./paginate";
type Row = { kind: string; status: number };
const rows: Row[] = [
  { kind: "chat", status: 200 },
  { kind: "chat", status: 500 },
  { kind: "embeddings", status: 200 },
];
describe("filterByFacets", () => {
  it("ignores facets with an empty value (all rows)", () => {
    expect(filterByFacets(rows, { kind: "" })).toEqual(rows);
    expect(filterByFacets(rows, {})).toEqual(rows);
  });
  it("keeps rows matching every active facet (string-equality)", () => {
    expect(filterByFacets(rows, { kind: "chat" })).toHaveLength(2);
    expect(filterByFacets(rows, { kind: "chat", status: "200" })).toHaveLength(1);
  });
});
describe("paginate", () => {
  it("returns one page of all rows when size <= 0", () => {
    const p = paginate(rows, 0, 0);
    expect(p.rows).toHaveLength(3);
    expect(p.pageCount).toBe(1);
  });
  it("slices the requested page and clamps out-of-range pages", () => {
    expect(paginate(rows, 0, 2).rows.map((r) => r.status)).toEqual([200, 500]);
    expect(paginate(rows, 1, 2).rows.map((r) => r.status)).toEqual([200]);
    expect(paginate(rows, 9, 2).page).toBe(1); // clamped to last page
  });
});
