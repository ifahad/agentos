import { describe, it, expect } from "vitest";
import { easeOutCubic, formatCount, countUpValue } from "./useCountUp";

describe("easeOutCubic", () => {
  it("is 0 at t=0 and 1 at t=1", () => {
    expect(easeOutCubic(0)).toBe(0);
    expect(easeOutCubic(1)).toBe(1);
  });

  it("eases out: progress ahead of linear early, behind late", () => {
    expect(easeOutCubic(0.25)).toBeGreaterThan(0.25);
    expect(easeOutCubic(0.75)).toBeGreaterThan(0.75);
  });

  it("is monotonically increasing on [0,1]", () => {
    let prev = -1;
    for (let t = 0; t <= 1.0001; t += 0.05) {
      const v = easeOutCubic(Math.min(1, t));
      expect(v).toBeGreaterThanOrEqual(prev);
      prev = v;
    }
  });
});

describe("formatCount", () => {
  it("formats integers with grouping", () => {
    expect(formatCount(12408)).toBe("12,408");
  });

  it("honors decimals, prefix and suffix", () => {
    expect(formatCount(41.2, { decimals: 2, prefix: "$" })).toBe("$41.20");
    expect(formatCount(3, { suffix: " runs" })).toBe("3 runs");
  });

  it("rounds mid-animation values to the requested decimals", () => {
    expect(formatCount(6204.321, { decimals: 0 })).toBe("6,204");
  });
});

describe("countUpValue", () => {
  it("clamps below 0 and above duration", () => {
    expect(countUpValue(100, -50, 700)).toBe(0);
    expect(countUpValue(100, 800, 700)).toBe(100);
  });

  it("returns target immediately when duration is 0 (reduced motion path)", () => {
    expect(countUpValue(12408, 0, 0)).toBe(12408);
  });

  it("midpoint is past half the target (ease-out)", () => {
    const mid = countUpValue(100, 350, 700);
    expect(mid).toBeGreaterThan(50);
    expect(mid).toBeLessThan(100);
  });
});
