import { describe, expect, it } from "vitest";
import { filterBySearch } from "./search";
type Row = { name: string; model: string; n: number };
const rows: Row[] = [
  { name: "prod", model: "gpt-4o", n: 3 },
  { name: "dev", model: "claude", n: 10 },
];
describe("filterBySearch", () => {
  it("returns all rows (copied) for an empty/whitespace query", () => {
    expect(filterBySearch(rows, ["name"], "")).toEqual(rows);
    expect(filterBySearch(rows, ["name"], "  ")).toEqual(rows);
    expect(filterBySearch(rows, ["name"], "")).not.toBe(rows);
  });
  it("matches a case-insensitive substring in any chosen field", () => {
    expect(filterBySearch(rows, ["name", "model"], "CLA").map((r) => r.name)).toEqual(["dev"]);
    expect(filterBySearch(rows, ["model"], "gpt").map((r) => r.name)).toEqual(["prod"]);
  });
  it("coerces non-string fields to string for matching", () => {
    expect(filterBySearch(rows, ["n"], "10").map((r) => r.name)).toEqual(["dev"]);
  });
  it("returns [] when nothing matches", () => {
    expect(filterBySearch(rows, ["name"], "zzz")).toEqual([]);
  });
});
