import { describe, expect, it } from "vitest";
import { sortRows } from "./sort";
type Row = { name: string; n: number };
const rows: Row[] = [
  { name: "b", n: 2 },
  { name: "a", n: 10 },
  { name: "c", n: 2 },
];
describe("sortRows", () => {
  it("returns a shallow copy for a null spec", () => {
    const out = sortRows(rows, null);
    expect(out).toEqual(rows);
    expect(out).not.toBe(rows);
  });
  it("sorts strings with localeCompare, both directions", () => {
    expect(sortRows(rows, { key: "name", dir: "asc" }).map((r) => r.name)).toEqual(["a", "b", "c"]);
    expect(sortRows(rows, { key: "name", dir: "desc" }).map((r) => r.name)).toEqual(["c", "b", "a"]);
  });
  it("sorts numerically when numeric is set (not lexically)", () => {
    expect(sortRows(rows, { key: "n", dir: "asc", numeric: true }).map((r) => r.n)).toEqual([2, 2, 10]);
    expect(sortRows(rows, { key: "n", dir: "desc", numeric: true }).map((r) => r.n)).toEqual([10, 2, 2]);
  });
  it("is stable for equal keys (preserves input order)", () => {
    const asc = sortRows(rows, { key: "n", dir: "asc", numeric: true });
    expect(asc.filter((r) => r.n === 2).map((r) => r.name)).toEqual(["b", "c"]);
  });
});
