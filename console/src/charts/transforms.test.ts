import { describe, expect, it } from "vitest";
import {
  breakdownFromRows,
  normalizeSeries,
  seriesFromEvents,
} from "./transforms";
import type { BreakdownOptions } from "./transforms";

const DAY = 86_400_000;
const HOUR = 3_600_000;
// A UTC midnight so day-bucket assertions are timezone-independent.
const T0 = Date.UTC(2026, 0, 5);

describe("seriesFromEvents", () => {
  it("returns an empty series for empty input", () => {
    expect(seriesFromEvents([])).toEqual([]);
  });

  it("returns a single point for a single event", () => {
    const s = seriesFromEvents([{ timestamp: T0 + 1234 }]);
    expect(s).toEqual([{ t: T0, value: 1 }]);
  });

  it("sorts unsorted input ascending and zero-fills the gap", () => {
    const s = seriesFromEvents([
      { timestamp: T0 + 2 * DAY },
      { timestamp: T0 },
    ]);
    expect(s).toEqual([
      { t: T0, value: 1 },
      { t: T0 + DAY, value: 0 },
      { t: T0 + 2 * DAY, value: 1 },
    ]);
  });

  it("counts events in the same bucket and sums explicit values", () => {
    const s = seriesFromEvents([
      { timestamp: T0 + 10 },
      { timestamp: T0 + 20 },
      { timestamp: T0 + 30, value: 2.5 },
    ]);
    expect(s).toEqual([{ t: T0, value: 4.5 }]);
  });

  it("buckets by hour when asked", () => {
    const s = seriesFromEvents(
      [{ timestamp: T0 + HOUR + 1 }, { timestamp: T0 + HOUR + 2 }],
      { bucket: "hour" },
    );
    expect(s).toEqual([{ t: T0 + HOUR, value: 2 }]);
  });

  it("skips events with unparseable timestamps", () => {
    const s = seriesFromEvents([
      { timestamp: "not-a-date" },
      { timestamp: Number.NaN },
      { timestamp: T0 },
    ]);
    expect(s).toEqual([{ t: T0, value: 1 }]);
    expect(seriesFromEvents([{ timestamp: "garbage" }])).toEqual([]);
  });

  it("honors from/to, zero-filling even with no events in range", () => {
    const s = seriesFromEvents([], { from: T0, to: T0 + 2 * DAY });
    expect(s).toEqual([
      { t: T0, value: 0 },
      { t: T0 + DAY, value: 0 },
      { t: T0 + 2 * DAY, value: 0 },
    ]);
  });

  it("excludes events outside the from/to range", () => {
    const s = seriesFromEvents(
      [{ timestamp: T0 - 5 * DAY }, { timestamp: T0 + DAY }],
      { from: T0, to: T0 + DAY },
    );
    expect(s).toEqual([
      { t: T0, value: 0 },
      { t: T0 + DAY, value: 1 },
    ]);
  });

  it("returns an empty series when from is after to", () => {
    expect(seriesFromEvents([], { from: T0 + DAY, to: T0 })).toEqual([]);
  });

  it("accepts ISO string timestamps", () => {
    const s = seriesFromEvents([{ timestamp: new Date(T0).toISOString() }]);
    expect(s).toEqual([{ t: T0, value: 1 }]);
  });
});

describe("normalizeSeries", () => {
  it("left-pads a short series with zeros", () => {
    expect(normalizeSeries([3, 4], 4)).toEqual([0, 0, 3, 4]);
    expect(normalizeSeries([], 3)).toEqual([0, 0, 0]);
  });

  it("keeps the most recent n values of a long series", () => {
    expect(normalizeSeries([1, 2, 3, 4, 5], 3)).toEqual([3, 4, 5]);
  });

  it("passes through an exact-length series", () => {
    expect(normalizeSeries([1, 2], 2)).toEqual([1, 2]);
  });

  it("returns an empty array for non-positive n", () => {
    expect(normalizeSeries([1, 2], 0)).toEqual([]);
    expect(normalizeSeries([1, 2], -3)).toEqual([]);
  });

  it("maps non-finite values to zero", () => {
    expect(normalizeSeries([Number.NaN, Number.POSITIVE_INFINITY, 2], 3)).toEqual([0, 0, 2]);
  });
});

describe("breakdownFromRows", () => {
  const rows = [
    { model: "a", spend: 1 },
    { model: "b", spend: 5 },
    { model: "a", spend: 2 },
    { model: "c", spend: 5 },
  ];
  const by: BreakdownOptions<{ model: string; spend: number }> = {
    label: (r) => r.model,
    value: (r) => r.spend,
  };

  it("returns an empty array for empty input", () => {
    expect(breakdownFromRows([], by)).toEqual([]);
  });

  it("aggregates duplicate labels and sorts descending with alphabetical ties", () => {
    expect(breakdownFromRows(rows, by)).toEqual([
      { label: "b", value: 5 },
      { label: "c", value: 5 },
      { label: "a", value: 3 },
    ]);
  });

  it("folds rows beyond the limit into Other", () => {
    expect(breakdownFromRows(rows, { ...by, limit: 2 })).toEqual([
      { label: "b", value: 5 },
      { label: "c", value: 5 },
      { label: "Other", value: 3 },
    ]);
  });

  it("supports a custom Other label and treats limit 0 as empty", () => {
    expect(breakdownFromRows(rows, { ...by, limit: 1, otherLabel: "rest" })).toEqual([
      { label: "b", value: 5 },
      { label: "rest", value: 8 },
    ]);
    expect(breakdownFromRows(rows, { ...by, limit: 0 })).toEqual([]);
  });

  it("counts non-finite values as zero and labels blanks unknown", () => {
    const out = breakdownFromRows(
      [
        { model: "x", spend: Number.NaN },
        { model: "", spend: 1 },
      ],
      by,
    );
    expect(out).toEqual([
      { label: "unknown", value: 1 },
      { label: "x", value: 0 },
    ]);
  });
});
