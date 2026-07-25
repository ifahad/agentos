import { describe, expect, it } from "vitest";
import {
  agreementLabel,
  approveProposalRequest,
  createObjectiveRequest,
  getObjectiveRequest,
  listMembersRequest,
  memberStatusTone,
  pauseRequest,
  summarizeMembers,
} from "./council";

describe("council request builders", () => {
  it("targets the runtime through the same-origin proxy", () => {
    expect(listMembersRequest().url).toBe("/api/runtime/council/members");
  });

  it("posts an objective body", () => {
    const spec = createObjectiveRequest("audit invoices");
    expect(spec.url).toBe("/api/runtime/council/objectives");
    expect(spec.init.method).toBe("POST");
    expect(JSON.parse(String(spec.init.body))).toEqual({ input: "audit invoices" });
  });

  it("encodes ids into paths", () => {
    expect(getObjectiveRequest("obj/1").url).toBe("/api/runtime/council/objectives/obj%2F1");
    expect(approveProposalRequest("p 1").url).toBe("/api/runtime/council/proposals/p%201/approve");
  });

  it("maps pause and resume onto distinct endpoints", () => {
    expect(pauseRequest(true).url).toBe("/api/runtime/council/pause");
    expect(pauseRequest(false).url).toBe("/api/runtime/council/resume");
  });
});

describe("verdict helpers", () => {
  it("labels agreement bands", () => {
    expect(agreementLabel(1)).toBe("unanimous");
    expect(agreementLabel(0.8)).toBe("strong");
    expect(agreementLabel(0.6)).toBe("majority");
    expect(agreementLabel(0.2)).toBe("split");
  });

  it("clamps out-of-range agreement", () => {
    expect(agreementLabel(4)).toBe("unanimous");
    expect(agreementLabel(-1)).toBe("split");
  });

  it("tones member statuses", () => {
    expect(memberStatusTone("answered")).toBe("ok");
    expect(memberStatusTone("timeout")).toBe("warn");
    expect(memberStatusTone("failed")).toBe("error");
    expect(memberStatusTone("anything-else")).toBe("warn");
  });

  it("summarizes a cycle's member runs", () => {
    const runs = [
      { member_id: "a", status: "answered" },
      { member_id: "b", status: "failed" },
      { member_id: "c", status: "timeout" },
    ] as never[];
    expect(summarizeMembers(runs)).toEqual({ answered: 1, failed: 2 });
  });
});
