import { describe, expect, it } from "vitest";
import {
  ApiError,
  buildRequest,
  formatRetryAfter,
  gatewayAdminRequest,
  oidcStatusRequest,
  parseErrorBody,
  parseRetryAfter,
  runtimeRequest,
  whoamiRequest,
} from "./api";

function headersOf(init: RequestInit): Record<string, string> {
  return init.headers as Record<string, string>;
}

describe("buildRequest", () => {
  it("defaults to GET with no body and no auth", () => {
    const { url, init } = buildRequest("/api/gateway", "/admin/keys");
    expect(url).toBe("/api/gateway/admin/keys");
    expect(init.method).toBe("GET");
    expect(init.body).toBeUndefined();
    expect(headersOf(init)["Authorization"]).toBeUndefined();
    expect(headersOf(init)["Content-Type"]).toBeUndefined();
  });

  it("switches to POST with a JSON body and content type", () => {
    const { init } = buildRequest("/api/runtime", "/runs", { body: { input: "hi" } });
    expect(init.method).toBe("POST");
    expect(init.body).toBe('{"input":"hi"}');
    expect(headersOf(init)["Content-Type"]).toBe("application/json");
  });

  it("normalizes a missing leading slash on the path", () => {
    const { url } = buildRequest("/api/gateway", "admin/usage");
    expect(url).toBe("/api/gateway/admin/usage");
  });

  it("honors an explicit method override", () => {
    const { init } = buildRequest("/api/runtime", "/runs", { method: "PUT", body: {} });
    expect(init.method).toBe("PUT");
  });
});

describe("gatewayAdminRequest", () => {
  it("targets /api/gateway and carries the admin key as Bearer", () => {
    const { url, init } = gatewayAdminRequest("/admin/usage", "sekret");
    expect(url).toBe("/api/gateway/admin/usage");
    expect(init.method).toBe("GET");
    expect(headersOf(init)["Authorization"]).toBe("Bearer sekret");
  });

  it("builds the create-key POST", () => {
    const { url, init } = gatewayAdminRequest("/admin/keys", "sekret", {
      name: "team-a",
      monthly_budget_usd: 25,
    });
    expect(url).toBe("/api/gateway/admin/keys");
    expect(init.method).toBe("POST");
    expect(init.body).toBe('{"name":"team-a","monthly_budget_usd":25}');
    expect(headersOf(init)["Authorization"]).toBe("Bearer sekret");
    expect(headersOf(init)["Content-Type"]).toBe("application/json");
  });

  it("builds the audit listing URL with a query string", () => {
    const { url } = gatewayAdminRequest("/admin/audit?limit=100", "k");
    expect(url).toBe("/api/gateway/admin/audit?limit=100");
  });
});

describe("runtimeRequest", () => {
  it("targets /api/runtime without auth", () => {
    const { url, init } = runtimeRequest("/runs", { input: "q", thread_id: "t1" });
    expect(url).toBe("/api/runtime/runs");
    expect(init.method).toBe("POST");
    expect(init.body).toBe('{"input":"q","thread_id":"t1"}');
    expect(headersOf(init)["Authorization"]).toBeUndefined();
  });

  it("builds the approve URL from a thread id", () => {
    const { url, init } = runtimeRequest("/runs/abc123/approve", { approve: false });
    expect(url).toBe("/api/runtime/runs/abc123/approve");
    expect(init.body).toBe('{"approve":false}');
  });
});

describe("parseErrorBody", () => {
  it("reads the gateway error envelope", () => {
    const err = parseErrorBody(402, {
      error: { type: "budget_exceeded", message: "monthly budget exhausted" },
    });
    expect(err).toBeInstanceOf(ApiError);
    expect(err.status).toBe(402);
    expect(err.type).toBe("budget_exceeded");
    expect(err.message).toBe("monthly budget exhausted");
  });

  it("reads the FastAPI detail string", () => {
    const err = parseErrorBody(503, { detail: "agent not initialized" });
    expect(err.message).toBe("agent not initialized");
  });

  it("stringifies structured FastAPI detail", () => {
    const err = parseErrorBody(422, { detail: [{ loc: ["body", "input"] }] });
    expect(err.message).toContain("input");
  });

  it("falls back to the status code for unknown bodies", () => {
    expect(parseErrorBody(500, undefined).message).toBe("HTTP 500");
    expect(parseErrorBody(404, "nope").message).toBe("HTTP 404");
  });

  it("carries retryAfter through onto the ApiError when supplied", () => {
    const err = parseErrorBody(429, { error: { type: "rate_limited" } }, 5);
    expect(err.status).toBe(429);
    expect(err.type).toBe("rate_limited");
    expect(err.retryAfter).toBe(5);
  });

  it("leaves retryAfter undefined when not supplied", () => {
    expect(parseErrorBody(402, { error: { type: "budget_exceeded" } }).retryAfter).toBeUndefined();
  });
});

describe("parseRetryAfter", () => {
  it("parses a delta-seconds header", () => {
    expect(parseRetryAfter("5")).toBe(5);
    expect(parseRetryAfter(" 30 ")).toBe(30);
    expect(parseRetryAfter("0")).toBe(0);
  });

  it("returns undefined for missing/empty/garbage headers", () => {
    expect(parseRetryAfter(null)).toBeUndefined();
    expect(parseRetryAfter(undefined)).toBeUndefined();
    expect(parseRetryAfter("")).toBeUndefined();
    expect(parseRetryAfter("   ")).toBeUndefined();
    expect(parseRetryAfter("soon")).toBeUndefined();
  });

  it("parses an HTTP-date into a non-negative seconds delta", () => {
    const future = new Date(Date.now() + 10_000).toUTCString();
    const secs = parseRetryAfter(future);
    expect(secs).toBeGreaterThanOrEqual(8);
    expect(secs).toBeLessThanOrEqual(11);
    const past = new Date(Date.now() - 10_000).toUTCString();
    expect(parseRetryAfter(past)).toBe(0);
  });
});

describe("formatRetryAfter", () => {
  it("formats positive seconds, rounding up", () => {
    expect(formatRetryAfter(3)).toBe("3s");
    expect(formatRetryAfter(2.1)).toBe("3s");
  });

  it("falls back to 'a moment' for unknown/non-positive input", () => {
    expect(formatRetryAfter(undefined)).toBe("a moment");
    expect(formatRetryAfter(0)).toBe("a moment");
    expect(formatRetryAfter(-4)).toBe("a moment");
    expect(formatRetryAfter(Number.NaN)).toBe("a moment");
  });
});

describe("whoamiRequest", () => {
  it("GETs /admin/whoami with the token as Bearer", () => {
    const { url, init } = whoamiRequest("agu-abc");
    expect(url).toBe("/api/gateway/admin/whoami");
    expect(init.method).toBe("GET");
    expect(headersOf(init)["Authorization"]).toBe("Bearer agu-abc");
  });
});

describe("oidcStatusRequest", () => {
  it("GETs the public /auth/oidc/status without auth", () => {
    const { url, init } = oidcStatusRequest();
    expect(url).toBe("/api/gateway/auth/oidc/status");
    expect(init.method).toBe("GET");
    expect(headersOf(init)["Authorization"]).toBeUndefined();
  });
});
