import { describe, expect, it } from "vitest";
import { ApiError } from "./api";
import {
  formatScore,
  isSelfImprovementDisabled,
  isUndecided,
  promptSourceLabel,
  proposalBadge,
  scoreDelta,
} from "./improve";

describe("formatScore", () => {
  it("renders a 0..1 fraction as a whole percentage", () => {
    expect(formatScore(0.75)).toBe("75%");
    expect(formatScore(1)).toBe("100%");
    expect(formatScore(0)).toBe("0%");
  });

  it("rounds to the nearest point", () => {
    expect(formatScore(2 / 3)).toBe("67%");
  });

  it("clamps out-of-range values", () => {
    expect(formatScore(1.2)).toBe("100%");
    expect(formatScore(-0.5)).toBe("0%");
  });

  it("renders a dash for non-finite input", () => {
    expect(formatScore(Number.NaN)).toBe("–");
  });
});

describe("scoreDelta", () => {
  it("shows gains in percentage points", () => {
    expect(scoreDelta(0.5, 0.75)).toBe("+25 pts");
  });

  it("shows losses with a minus sign", () => {
    expect(scoreDelta(0.75, 0.5)).toBe("−25 pts");
  });

  it("shows a tie as ±0", () => {
    expect(scoreDelta(0.75, 0.75)).toBe("±0 pts");
  });

  it("returns empty for non-finite input", () => {
    expect(scoreDelta(Number.NaN, 0.5)).toBe("");
  });
});

describe("proposalBadge", () => {
  it("maps each status to its badge class and label", () => {
    expect(proposalBadge("passed_evals")).toEqual({
      className: "badge passed_evals",
      label: "passed evals",
    });
    expect(proposalBadge("failed_evals")).toEqual({
      className: "badge failed_evals",
      label: "failed evals",
    });
    expect(proposalBadge("approved")).toEqual({ className: "badge approved", label: "approved" });
    expect(proposalBadge("denied")).toEqual({ className: "badge denied", label: "denied" });
  });

  it("falls back to a plain badge for unknown statuses", () => {
    expect(proposalBadge("weird")).toEqual({ className: "badge", label: "weird" });
  });
});

describe("isUndecided", () => {
  it("treats both eval outcomes as awaiting a decision", () => {
    expect(isUndecided("passed_evals")).toBe(true);
    expect(isUndecided("failed_evals")).toBe(true);
  });

  it("treats decided statuses as final", () => {
    expect(isUndecided("approved")).toBe(false);
    expect(isUndecided("denied")).toBe(false);
  });
});

describe("promptSourceLabel", () => {
  it("labels a proposal-sourced prompt with its id", () => {
    expect(promptSourceLabel("proposal", 3)).toBe("proposal #3");
  });

  it("labels the default prompt", () => {
    expect(promptSourceLabel("default", null)).toBe("default");
  });

  it("falls back to default when a proposal id is missing", () => {
    expect(promptSourceLabel("proposal", null)).toBe("default");
  });
});

describe("isSelfImprovementDisabled", () => {
  it("recognises a 503 ApiError", () => {
    expect(isSelfImprovementDisabled(new ApiError(503, "error", "disabled"))).toBe(true);
  });

  it("rejects other statuses and error kinds", () => {
    expect(isSelfImprovementDisabled(new ApiError(500, "error", "boom"))).toBe(false);
    expect(isSelfImprovementDisabled(new Error("boom"))).toBe(false);
  });
});
