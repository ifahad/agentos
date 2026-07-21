import { describe, expect, it } from "vitest";
import { identityFromWhoAmI, parseAuthFragment } from "./sso";
import type { WhoAmI } from "./types";

describe("parseAuthFragment", () => {
  it("parses a full token/email/role fragment (with leading #)", () => {
    const f = parseAuthFragment("#token=agu-abc&email=me@acme.com&role=member");
    expect(f).toEqual({ token: "agu-abc", email: "me@acme.com", role: "member" });
  });

  it("parses a fragment without the leading #", () => {
    const f = parseAuthFragment("token=agu-xyz&role=owner");
    expect(f.token).toBe("agu-xyz");
    expect(f.role).toBe("owner");
    expect(f.email).toBeUndefined();
  });

  it("returns only the keys present (partial fragment)", () => {
    const f = parseAuthFragment("#token=agu-1");
    expect(f).toEqual({ token: "agu-1" });
  });

  it("URL-decodes values", () => {
    const f = parseAuthFragment("#token=agu-1&email=a%2Bb%40acme.com");
    expect(f.email).toBe("a+b@acme.com");
  });

  it("returns an empty object for an empty hash", () => {
    expect(parseAuthFragment("")).toEqual({});
    expect(parseAuthFragment("#")).toEqual({});
  });

  it("ignores unrelated fragments (no token/email/role)", () => {
    expect(parseAuthFragment("#section=keys&foo=bar")).toEqual({});
  });

  it("treats empty values as absent", () => {
    expect(parseAuthFragment("#token=&email=&role=")).toEqual({});
  });
});

describe("identityFromWhoAmI", () => {
  it("maps the root admin key to the root superuser (no org/email)", () => {
    const who: WhoAmI = { root: true };
    expect(identityFromWhoAmI(who)).toEqual({ role: "root", orgId: "", email: "" });
  });

  it("maps a user token to its role/org/email", () => {
    const who: WhoAmI = {
      root: false,
      user_id: "u_1",
      org_id: "org_acme",
      email: "me@acme.com",
      role: "admin",
    };
    expect(identityFromWhoAmI(who)).toEqual({
      role: "admin",
      orgId: "org_acme",
      email: "me@acme.com",
    });
  });
});
