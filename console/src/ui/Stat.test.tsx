// @vitest-environment jsdom
//
// The first component test in the console. Stat encodes the platform's central
// UI rule — never display a reading you cannot prove — and until the DOM
// harness landed that rule was enforced only by whoever last read the file.
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Stat } from "./Stat";

describe("Stat", () => {
  it("renders an em dash, not a zero, when the reading is unavailable", () => {
    render(<Stat label="Spend" value={0} unavailable prefix="$" />);
    // A 0 here would report a measurement nobody took, and would read
    // identically to a genuinely idle system. That ambiguity is the whole
    // reason the prop exists.
    expect(screen.getByText("—")).toBeTruthy();
    expect(screen.queryByText("$0")).toBeNull();
    expect(screen.queryByText("0")).toBeNull();
  });

  it("names the unavailable state for screen readers instead of reading punctuation", () => {
    render(<Stat label="Spend" value={0} unavailable />);
    // Without the aria-label a screen reader announces "em dash", which tells
    // the listener nothing about why the number is missing.
    expect(screen.getByLabelText("Not available")).toBeTruthy();
  });

  it("carries no unavailable labelling when a real reading is present", () => {
    render(<Stat label="Keys" value={12} />);
    expect(screen.queryByLabelText("Not available")).toBeNull();
    expect(screen.queryByText("—")).toBeNull();
  });

  it("shows the label in both states", () => {
    const { rerender } = render(<Stat label="Documents" value={3} />);
    expect(screen.getByText("Documents")).toBeTruthy();
    rerender(<Stat label="Documents" value={0} unavailable />);
    expect(screen.getByText("Documents")).toBeTruthy();
  });
});
