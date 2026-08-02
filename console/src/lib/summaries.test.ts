import { describe, expect, it } from "vitest";
import type { BudgetMeter } from "../pages/overviewFeed";
import type {
  AuditEntry,
  CouncilObjective,
  CouncilProposal,
  DocumentInfo,
  KeyInfo,
  Operator,
  Org,
  User,
} from "./types";
import {
  auditEventsSummary,
  budgetsSummary,
  countLine,
  documentsSummary,
  keysSummary,
  objectivesSummary,
  operatorsSummary,
  orgsSummary,
  usersSummary,
} from "./summaries";

describe("countLine", () => {
  it("pluralises the noun", () => {
    expect(countLine(0, "key")).toBe("0 keys");
    expect(countLine(1, "key")).toBe("1 key");
    expect(countLine(12, "key")).toBe("12 keys");
  });

  it("appends an attention clause when it is non-zero", () => {
    expect(countLine(12, "key", { count: 2, label: "inactive" })).toBe("12 keys · 2 inactive");
  });

  it("omits the attention clause when it is zero", () => {
    // A quiet panel must read quiet. "· 0 inactive" invites a second look at
    // something that needs none.
    expect(countLine(12, "key", { count: 0, label: "inactive" })).toBe("12 keys");
  });

  it("omits the attention clause when it is absent", () => {
    expect(countLine(3, "document")).toBe("3 documents");
  });

  it("does not pluralise the attention label", () => {
    // The label is written already-plural by the caller ("inactive", "held"),
    // because these are adjectives, not nouns.
    expect(countLine(1, "operator", { count: 1, label: "held" })).toBe("1 operator · 1 held");
  });
});

// ---- Wrapper fixtures ----
// Minimal, real element shapes from lib/types.ts (and overviewFeed's
// BudgetMeter for the Overview budgets panel) — no invented fields.

function key(overrides: Partial<KeyInfo> = {}): KeyInfo {
  return { name: "k", monthly_budget_usd: 25, spend_usd: 0, ...overrides };
}

function doc(overrides: Partial<DocumentInfo> = {}): DocumentInfo {
  return { name: "d.pdf", chunks: 3, ...overrides };
}

function operator(overrides: Partial<Operator> = {}): Operator {
  return {
    id: "op1",
    name: "watcher",
    goal: "watch",
    trigger: { type: "interval", interval_s: 60 },
    enabled: true,
    max_cycles: 5,
    created_at: "2026-01-01T00:00:00Z",
    last_fired_at: null,
    ...overrides,
  };
}

function org(overrides: Partial<Org> = {}): Org {
  return {
    id: "org1",
    name: "Acme",
    monthly_budget_usd: 100,
    spend_usd: 0,
    created_at: "2026-01-01T00:00:00Z",
    rate_limit_rpm: 0,
    ...overrides,
  };
}

function user(overrides: Partial<User> = {}): User {
  return {
    id: "u1",
    org_id: "org1",
    email: "a@example.com",
    role: "member",
    created_at: "2026-01-01T00:00:00Z",
    active: true,
    ...overrides,
  };
}

function objective(overrides: Partial<CouncilObjective> = {}): CouncilObjective {
  return {
    id: "obj1",
    input: "do the thing",
    status: "running",
    stop_reason: null,
    cycles_run: 1,
    spend_usd: 0,
    max_cycles: null,
    budget_usd: 10,
    created_at: "2026-01-01T00:00:00Z",
    claimed_by: null,
    ...overrides,
  };
}

function auditEntry(overrides: Partial<AuditEntry> = {}): AuditEntry {
  return {
    ts: "2026-01-01T00:00:00Z",
    key_name: "k",
    model: "m",
    input_tokens: 1,
    output_tokens: 1,
    cost_usd: 0,
    latency_ms: 10,
    status: 200,
    kind: "chat",
    ...overrides,
  };
}

function meter(overrides: Partial<BudgetMeter> = {}): BudgetMeter {
  return { id: "m1", name: "k", spend: 0, budget: 100, fraction: 0, ...overrides };
}

function proposal(overrides: Partial<CouncilProposal> = {}): CouncilProposal {
  return {
    id: "p1",
    objective_id: "obj1",
    member_id: "m1",
    tool: "write_file",
    arguments: {},
    status: "pending",
    created_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("keysSummary", () => {
  it("counts zero keys", () => {
    expect(keysSummary([])).toBe("0 keys");
  });

  it("singularises one key under budget, no attention clause", () => {
    expect(keysSummary([key({ name: "solo", monthly_budget_usd: 25, spend_usd: 5 })])).toBe("1 key");
  });

  it("flags a key exactly at its budget as over — the >= boundary", () => {
    expect(keysSummary([key({ monthly_budget_usd: 25, spend_usd: 25 })])).toBe("1 key · 1 over budget");
  });

  it("counts many keys and only those over budget", () => {
    const keys = [
      key({ monthly_budget_usd: 25, spend_usd: 5 }),
      key({ monthly_budget_usd: 25, spend_usd: 25 }),
      key({ monthly_budget_usd: 25, spend_usd: 30 }),
    ];
    expect(keysSummary(keys)).toBe("3 keys · 2 over budget");
  });

  it("treats a zero budget as unlimited, not over", () => {
    expect(keysSummary([key({ monthly_budget_usd: 0, spend_usd: 500 })])).toBe("1 key");
  });
});

describe("documentsSummary", () => {
  it("counts zero documents", () => {
    expect(documentsSummary([])).toBe("0 documents");
  });

  it("singularises one document", () => {
    expect(documentsSummary([doc()])).toBe("1 document");
  });

  it("is a plain count for many documents", () => {
    expect(documentsSummary([doc(), doc(), doc()])).toBe("3 documents");
  });
});

describe("operatorsSummary", () => {
  it("is a plain count for no operators — Operator has no held state", () => {
    expect(operatorsSummary([])).toBe("0 operators");
  });

  it("singularises one operator", () => {
    expect(operatorsSummary([operator()])).toBe("1 operator");
  });

  it("stays a plain count for many operators, no attention clause", () => {
    expect(operatorsSummary([operator(), operator({ enabled: false }), operator()])).toBe("3 operators");
  });
});

describe("orgsSummary", () => {
  it("counts zero orgs", () => {
    expect(orgsSummary([])).toBe("0 orgs");
  });

  it("singularises one org over budget", () => {
    expect(orgsSummary([org({ monthly_budget_usd: 50, spend_usd: 75 })])).toBe("1 org · 1 over budget");
  });

  it("flags an org exactly at its budget as over — the >= boundary", () => {
    expect(orgsSummary([org({ monthly_budget_usd: 100, spend_usd: 100 })])).toBe("1 org · 1 over budget");
  });

  it("counts many orgs and only those over budget", () => {
    const orgs = [
      org({ monthly_budget_usd: 100, spend_usd: 50 }),
      org({ monthly_budget_usd: 100, spend_usd: 150 }),
      org({ monthly_budget_usd: 100, spend_usd: 200 }),
    ];
    expect(orgsSummary(orgs)).toBe("3 orgs · 2 over budget");
  });

  it("treats a zero budget as unlimited, not over", () => {
    expect(orgsSummary([org({ monthly_budget_usd: 0, spend_usd: 500 })])).toBe("1 org");
  });
});

describe("usersSummary", () => {
  it("counts zero members", () => {
    expect(usersSummary([])).toBe("0 members");
  });

  it("singularises one member", () => {
    expect(usersSummary([user()])).toBe("1 member");
  });

  it("is a plain count for many members, no attention clause", () => {
    expect(usersSummary([user(), user({ active: false }), user()])).toBe("3 members");
  });
});

describe("objectivesSummary", () => {
  it("counts zero objectives with no proposals", () => {
    expect(objectivesSummary([], [])).toBe("0 objectives");
  });

  it("has no attention clause when there are no pending proposals", () => {
    expect(objectivesSummary([objective({ id: "obj1" })], [])).toBe("1 objective");
  });

  it("singularises one objective with one pending proposal against it", () => {
    expect(
      objectivesSummary([objective({ id: "obj1" })], [proposal({ objective_id: "obj1", status: "pending" })]),
    ).toBe("1 objective · 1 with pending writes");
  });

  it("counts an objective once even with several pending proposals against it", () => {
    // This is the case most likely to be wrong: counting by proposal instead
    // of by distinct objective would report 3, not 1.
    const proposals = [
      proposal({ id: "p1", objective_id: "obj1", status: "pending" }),
      proposal({ id: "p2", objective_id: "obj1", status: "pending" }),
      proposal({ id: "p3", objective_id: "obj1", status: "pending" }),
    ];
    expect(objectivesSummary([objective({ id: "obj1" })], proposals)).toBe(
      "1 objective · 1 with pending writes",
    );
  });

  it("does not count a pending proposal against an objective not in the list", () => {
    const proposals = [proposal({ objective_id: "ghost-objective", status: "pending" })];
    expect(objectivesSummary([objective({ id: "obj1" })], proposals)).toBe("1 objective");
  });

  it("does not count approved or denied proposals as pending", () => {
    const proposals = [
      proposal({ objective_id: "obj1", status: "approved" }),
      proposal({ objective_id: "obj1", status: "denied" }),
    ];
    expect(objectivesSummary([objective({ id: "obj1" })], proposals)).toBe("1 objective");
  });

  it("counts many objectives and only those with a pending proposal", () => {
    const objectives = [objective({ id: "obj1" }), objective({ id: "obj2" }), objective({ id: "obj3" })];
    const proposals = [
      proposal({ objective_id: "obj1", status: "pending" }),
      proposal({ objective_id: "obj2", status: "approved" }),
    ];
    expect(objectivesSummary(objectives, proposals)).toBe("3 objectives · 1 with pending writes");
  });
});

describe("auditEventsSummary", () => {
  it("counts zero events", () => {
    expect(auditEventsSummary([])).toBe("0 events");
  });

  it("singularises one denied event", () => {
    expect(auditEventsSummary([auditEntry({ kind: "guardrail_block", status: 403 })])).toBe(
      "1 event · 1 denied",
    );
  });

  it("counts many events and only guardrail_block as denied", () => {
    const events = [
      auditEntry({ kind: "chat" }),
      auditEntry({ kind: "guardrail_flag" }),
      auditEntry({ kind: "guardrail_block", status: 403 }),
      auditEntry({ kind: "guardrail_block", status: 403 }),
    ];
    expect(auditEventsSummary(events)).toBe("4 events · 2 denied");
  });
});

describe("budgetsSummary", () => {
  it("counts zero budgets", () => {
    expect(budgetsSummary([])).toBe("0 budgets");
  });

  it("singularises one budget over its limit", () => {
    expect(budgetsSummary([meter({ spend: 120, budget: 100 })])).toBe("1 budget · 1 over limit");
  });

  it("flags a budget exactly at its limit as over — the >= boundary", () => {
    // Same boundary as keysSummary and orgsSummary: a key sitting exactly at
    // its budget must not be flagged on the Keys page and silent here.
    expect(budgetsSummary([meter({ spend: 100, budget: 100 })])).toBe("1 budget · 1 over limit");
  });

  it("counts many budgets and only those at or over limit", () => {
    const meters = [
      meter({ spend: 10, budget: 100 }),
      meter({ spend: 100, budget: 100 }),
      meter({ spend: 150, budget: 100 }),
    ];
    expect(budgetsSummary(meters)).toBe("3 budgets · 2 over limit");
  });
});
