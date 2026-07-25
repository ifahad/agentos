import { describe, expect, it } from "vitest";
import {
  createOperatorRequest,
  deleteOperatorRequest,
  getOperatorRequest,
  listOperatorsRequest,
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
