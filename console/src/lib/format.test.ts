import { describe, expect, it } from "vitest";
import {
  budgetFraction,
  compactJSON,
  formatInt,
  formatLatency,
  formatTimestamp,
  formatUSD,
} from "./format";

describe("formatUSD", () => {
  it("uses two decimals from a dollar upward", () => {
    expect(formatUSD(12.345)).toBe("$12.35");
    expect(formatUSD(1)).toBe("$1.00");
  });

  it("uses four decimals for sub-dollar amounts", () => {
    expect(formatUSD(0.00421)).toBe("$0.0042");
  });

  it("renders zero and non-finite values as $0.00", () => {
    expect(formatUSD(0)).toBe("$0.00");
    expect(formatUSD(Number.NaN)).toBe("$0.00");
  });
});

describe("formatInt", () => {
  it("adds thousands separators", () => {
    expect(formatInt(1234567)).toBe("1,234,567");
    expect(formatInt(999)).toBe("999");
  });

  it("guards non-finite input", () => {
    expect(formatInt(Number.POSITIVE_INFINITY)).toBe("0");
  });
});

describe("formatLatency", () => {
  it("keeps milliseconds under a second", () => {
    expect(formatLatency(834)).toBe("834 ms");
  });

  it("switches to seconds at 1000 ms", () => {
    expect(formatLatency(1240)).toBe("1.24 s");
  });

  it("renders a dash for invalid input", () => {
    expect(formatLatency(-1)).toBe("–");
  });
});

describe("formatTimestamp", () => {
  it("formats RFC3339 into a local date-time", () => {
    // Assert shape rather than exact value (test machine timezone varies).
    expect(formatTimestamp("2026-07-21T09:30:00Z")).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/);
  });

  it("returns unparsable input unchanged", () => {
    expect(formatTimestamp("not-a-date")).toBe("not-a-date");
  });
});

describe("compactJSON", () => {
  it("renders one-line JSON", () => {
    expect(compactJSON({ sql: "SELECT 1" })).toBe('{"sql":"SELECT 1"}');
  });

  it("truncates long payloads with an ellipsis", () => {
    const out = compactJSON({ text: "x".repeat(1000) }, 50);
    expect(out.length).toBe(50);
    expect(out.endsWith("…")).toBe(true);
  });
});

describe("budgetFraction", () => {
  it("computes the spent fraction", () => {
    expect(budgetFraction(5, 25)).toBeCloseTo(0.2);
  });

  it("clamps overspend to 1", () => {
    expect(budgetFraction(30, 25)).toBe(1);
  });

  it("returns 0 for zero or invalid budgets", () => {
    expect(budgetFraction(5, 0)).toBe(0);
    expect(budgetFraction(5, Number.NaN)).toBe(0);
  });
});
