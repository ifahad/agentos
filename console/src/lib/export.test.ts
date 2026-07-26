import { describe, expect, it } from "vitest";
import { toCSV, toJSON } from "./export";

type Row = { name: string; note: string; n: number };
const cols = [
  { header: "Name", value: (r: Row) => r.name },
  { header: "Note", value: (r: Row) => r.note },
  { header: "Count", value: (r: Row) => r.n },
];

describe("toCSV", () => {
  it("writes a header row then one row per record (CRLF-joined)", () => {
    const csv = toCSV([{ name: "prod", note: "ok", n: 3 }], cols);
    expect(csv).toBe("Name,Note,Count\r\nprod,ok,3");
  });
  it("quotes cells with commas, quotes, or newlines and doubles internal quotes", () => {
    const csv = toCSV([{ name: 'a,b', note: 'he said "hi"', n: 1 }], cols);
    expect(csv).toBe('Name,Note,Count\r\n"a,b","he said ""hi""",1');
  });
  it("handles empty rows (header only)", () => {
    expect(toCSV([], cols)).toBe("Name,Note,Count");
  });
});

describe("toJSON", () => {
  it("maps each row to a header-keyed object", () => {
    expect(JSON.parse(toJSON([{ name: "p", note: "n", n: 2 }], cols))).toEqual([
      { Name: "p", Note: "n", Count: 2 },
    ]);
  });
});
