import { describe, expect, it } from "vitest";
import { activeBadge, formatExternalId } from "./provisioning";

describe("activeBadge", () => {
  it("marks active users with a green pass badge", () => {
    expect(activeBadge(true)).toEqual({ label: "active", className: "badge pass" });
  });

  it("marks inactive users with a dim inactive badge", () => {
    expect(activeBadge(false)).toEqual({ label: "inactive", className: "badge inactive" });
  });

  it("always uses the shared .badge base class", () => {
    for (const active of [true, false]) {
      expect(activeBadge(active).className.startsWith("badge")).toBe(true);
    }
  });
});

describe("formatExternalId", () => {
  it("renders IdP-managed when a SCIM external id is present", () => {
    expect(formatExternalId("okta-abc-123")).toBe("IdP-managed");
  });

  it("renders an em dash when there is no external id", () => {
    expect(formatExternalId(undefined)).toBe("—");
    expect(formatExternalId(null)).toBe("—");
    expect(formatExternalId("")).toBe("—");
  });

  it("treats a blank (whitespace-only) external id as unmanaged", () => {
    expect(formatExternalId("   ")).toBe("—");
  });
});
