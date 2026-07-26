import { describe, expect, it } from "vitest";
import { formatAgo } from "./relativeTime";

describe("formatAgo", () => {
  it("reads 'just now' under 3s and for clock skew", () => {
    expect(formatAgo(0)).toBe("just now");
    expect(formatAgo(2_999)).toBe("just now");
    expect(formatAgo(-500)).toBe("just now");
  });
  it("counts seconds, minutes, hours, days", () => {
    expect(formatAgo(3_000)).toBe("3s ago");
    expect(formatAgo(59_000)).toBe("59s ago");
    expect(formatAgo(60_000)).toBe("1m ago");
    expect(formatAgo(59 * 60_000)).toBe("59m ago");
    expect(formatAgo(60 * 60_000)).toBe("1h ago");
    expect(formatAgo(23 * 3_600_000)).toBe("23h ago");
    expect(formatAgo(24 * 3_600_000)).toBe("1d ago");
  });
});
