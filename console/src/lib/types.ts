// Wire types for the AgentOS gateway and runtime APIs (frozen contracts,
// Phase 1 + Phase 2 plans).

// ---- Gateway admin ----

export interface KeyUsage {
  name: string;
  requests: number;
  input_tokens: number;
  output_tokens: number;
  spend_usd: number;
}

export interface KeyInfo {
  name: string;
  monthly_budget_usd: number;
  spend_usd: number;
}

export interface CreatedKey {
  key: string;
  name: string;
  monthly_budget_usd: number;
}

export type AuditKind = "chat" | "embeddings" | "guardrail_flag" | "guardrail_block";

export interface AuditEntry {
  ts: string;
  key_name: string;
  model: string;
  input_tokens: number;
  output_tokens: number;
  cost_usd: number;
  latency_ms: number;
  status: number;
  kind: AuditKind;
}

// ---- Runtime ----

export interface Step {
  tool: string;
  input: Record<string, unknown>;
}

export interface RunResponse {
  thread_id: string;
  output: string;
  steps: Step[];
  status?: "completed";
}

export interface PendingTool {
  tool: string;
  input: Record<string, unknown>;
}

export interface PendingApprovalResponse {
  status: "pending_approval";
  thread_id: string;
  pending: PendingTool[];
}

export interface DocumentInfo {
  name: string;
  chunks: number;
}

// SSE events emitted by POST /runs/stream.
export type StreamEvent =
  | { event: "step"; tool: string; input: Record<string, unknown> }
  | { event: "pending_approval"; pending: PendingTool[]; thread_id?: string }
  | { event: "done"; thread_id: string; output: string; steps: Step[] };

// ---- Runtime self-improvement (Phase 3) ----

// Per-case result inside a POST /evals/run response.
export interface EvalCaseResult {
  name: string;
  passed: boolean;
  output_snippet: string;
}

// POST /evals/run response (score is a 0..1 fraction).
export interface EvalRunResult {
  run_id: number;
  suite: string;
  score: number;
  passed: number;
  failed: number;
  cases: EvalCaseResult[];
}

// GET /evals/runs row — recent runs, no per-case detail.
export interface EvalRunSummary {
  id: number;
  suite: string;
  score: number;
  passed: number;
  failed: number;
  prompt_source: string;
  created_at: string;
}

export type ProposalStatus = "passed_evals" | "failed_evals" | "approved" | "denied";

// GET /proposals row; POST /improve returns the same shape.
export interface Proposal {
  id: number;
  prompt_text: string;
  rationale: string;
  baseline_score: number;
  candidate_score: number;
  status: ProposalStatus;
  created_at: string;
}

// POST /proposals/{id}/approve response; warning set when a below-baseline
// proposal is approved (human override).
export interface ApproveResponse {
  id: number;
  status: ProposalStatus;
  warning?: string;
}

// GET /prompts/active response.
export interface ActivePrompt {
  source: "default" | "proposal";
  prompt: string;
  proposal_id: number | null;
}
