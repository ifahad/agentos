import { describe, expect, it } from "vitest";
import type { Action, AuthRole } from "./rbac";
import { ALL_ACTIONS, ROLES, asAuthRole, can, isRole, roleLabel } from "./rbac";
import type { Role } from "./types";

// The authoritative expectation of the capability map, mirroring the contract.
// Each role lists exactly the actions it is allowed; everything else is denied.
const EXPECTED_ALLOWED: Record<Role, Action[]> = {
  owner: ["user.invite", "user.remove", "user.view", "key.create", "key.view", "usage.view", "agent.run"],
  admin: ["user.invite", "user.remove", "user.view", "key.create", "key.view", "usage.view", "agent.run"],
  member: ["user.view", "key.create", "key.view", "usage.view", "agent.run"],
  viewer: ["key.view", "usage.view"],
};

describe("can — per-role allow/deny matrix", () => {
  for (const role of ROLES) {
    const allowed = new Set(EXPECTED_ALLOWED[role]);
    for (const action of ALL_ACTIONS) {
      const expected = allowed.has(action);
      it(`${role} ${expected ? "may" : "may not"} ${action}`, () => {
        expect(can(role, action)).toBe(expected);
      });
    }
  }

  it("root is a superuser — allowed every action", () => {
    for (const action of ALL_ACTIONS) {
      expect(can("root", action)).toBe(true);
    }
  });
});

describe("can — root-exclusive actions", () => {
  const rootOnly: Action[] = ["org.create", "org.view", "secret.view"];
  it("no org role may create/view orgs or view secrets", () => {
    for (const role of ROLES) {
      for (const action of rootOnly) {
        expect(can(role, action)).toBe(false);
      }
    }
  });
});

describe("can — owner and admin manage users; member and viewer do not", () => {
  it("owner/admin may invite and remove", () => {
    for (const role of ["owner", "admin"] as AuthRole[]) {
      expect(can(role, "user.invite")).toBe(true);
      expect(can(role, "user.remove")).toBe(true);
    }
  });

  it("member and viewer may not invite or remove", () => {
    for (const role of ["member", "viewer"] as AuthRole[]) {
      expect(can(role, "user.invite")).toBe(false);
      expect(can(role, "user.remove")).toBe(false);
    }
  });

  it("viewer is read-only — no key.create, no user.view, no agent.run", () => {
    expect(can("viewer", "key.create")).toBe(false);
    expect(can("viewer", "user.view")).toBe(false);
    expect(can("viewer", "agent.run")).toBe(false);
    expect(can("viewer", "key.view")).toBe(true);
    expect(can("viewer", "usage.view")).toBe(true);
  });
});

describe("isRole", () => {
  it("accepts the four org roles", () => {
    for (const role of ROLES) expect(isRole(role)).toBe(true);
  });

  it("rejects root, unknown strings, and non-strings", () => {
    expect(isRole("root")).toBe(false);
    expect(isRole("superadmin")).toBe(false);
    expect(isRole("")).toBe(false);
    expect(isRole(undefined)).toBe(false);
    expect(isRole(42)).toBe(false);
  });
});

describe("asAuthRole", () => {
  it("passes through valid roles and root", () => {
    expect(asAuthRole("root")).toBe("root");
    expect(asAuthRole("owner")).toBe("owner");
    expect(asAuthRole("viewer")).toBe("viewer");
  });

  it("falls back to root for unknown/empty values (back-compat)", () => {
    expect(asAuthRole("")).toBe("root");
    expect(asAuthRole("nonsense")).toBe("root");
    expect(asAuthRole(null)).toBe("root");
  });
});

describe("roleLabel", () => {
  it("capitalizes org roles", () => {
    expect(roleLabel("owner")).toBe("Owner");
    expect(roleLabel("member")).toBe("Member");
  });

  it("names the root superuser", () => {
    expect(roleLabel("root")).toBe("Root admin");
  });
});
