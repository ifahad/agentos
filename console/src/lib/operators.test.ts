import { describe, expect, it } from "vitest";
import {
  createOperatorRequest,
  deleteOperatorRequest,
  getOperatorRequest,
  listOperatorsRequest,
  operatorEta,
  runOperatorRequest,
  runStatusTone,
  setEnabledRequest,
  triggerSummary,
} from "./operators";

describe("operator request builders", () => {
  it("targets the runtime proxy", () => {
    expect(listOperatorsRequest().url).toBe("/api/runtime/operators");
  });

  it("posts a create body", () => {
    const spec = createOperatorRequest({
      name: "nightly",
      goal: "summarise",
      trigger: { type: "interval", interval_s: 60 },
    });
    expect(spec.url).toBe("/api/runtime/operators");
    expect(spec.init.method).toBe("POST");
    expect(JSON.parse(String(spec.init.body)).trigger.interval_s).toBe(60);
  });

  it("encodes ids and picks the right verbs", () => {
    expect(getOperatorRequest("op/1").url).toBe("/api/runtime/operators/op%2F1");
    expect(runOperatorRequest("a").url).toBe("/api/runtime/operators/a/run");
    expect(runOperatorRequest("a").init.method).toBe("POST");
    expect(deleteOperatorRequest("a").init.method).toBe("DELETE");
  });

  it("patches enabled", () => {
    const spec = setEnabledRequest("a", false);
    expect(spec.init.method).toBe("PATCH");
    expect(JSON.parse(String(spec.init.body))).toEqual({ enabled: false });
  });
});

describe("operator helpers", () => {
  it("summarizes triggers", () => {
    expect(triggerSummary({ type: "interval", interval_s: 90 })).toBe("every 90s");
    expect(triggerSummary({ type: "cron", cron: "0 9 * * *" })).toBe("cron 0 9 * * *");
    expect(triggerSummary({ type: "webhook" })).toBe("webhook");
  });

  it("tones run statuses", () => {
    expect(runStatusTone("completed")).toBe("ok");
    expect(runStatusTone("needs_approval")).toBe("hold");
    expect(runStatusTone("error")).toBe("error");
    expect(runStatusTone("weird")).toBe("error");
  });
});

describe("operatorEta", () => {
  const now = Date.parse("2026-07-26T10:00:00Z");
  it("returns ms remaining for an interval operator that has fired", () => {
    const op = {
      trigger: { type: "interval" as const, interval_s: 300 },
      last_fired_at: "2026-07-26T09:56:00Z", // 240s ago → 60s left
    };
    expect(operatorEta(op, now)).toBe(60_000);
  });
  it("returns 0 when overdue", () => {
    const op = {
      trigger: { type: "interval" as const, interval_s: 60 },
      last_fired_at: "2026-07-26T09:56:00Z",
    };
    expect(operatorEta(op, now)).toBe(0);
  });
  it("returns null for cron/webhook or missing last_fired_at", () => {
    expect(operatorEta({ trigger: { type: "cron", cron: "* * * * *" }, last_fired_at: null }, now)).toBeNull();
    expect(operatorEta({ trigger: { type: "interval", interval_s: 60 }, last_fired_at: null }, now)).toBeNull();
  });
});
