import type { DocSection } from "./types";

/**
 * The in-console handbook. Twelve sections in the canonical taxonomy order the
 * GitHub docs share — a contradiction between the two surfaces is the failure
 * this content exists to prevent, so every claim here is sourced from
 * `README.md`, `docs/*.md`, or `SECURITY.md`.
 *
 * The console is air-gapped: there are no outbound links anywhere in this file.
 * Where depth belongs in the repository docs, the prose names the file.
 */
export const DOC_SECTIONS: DocSection[] = [
  {
    id: "overview",
    title: "Overview",
    icon: "overview",
    blurb: "What AgentOS is, and the one guarantee everything else serves.",
    blocks: [
      {
        kind: "prose",
        text: "AgentOS is a self-hostable agentic operating layer: any LLM provider in, any legacy system out, with governed autonomous agents in between. Agents are useful exactly to the degree you can let them touch real systems, and this is the layer that makes that defensible: nothing runs unauthorized, unattributed, or unrecorded. It is for teams who want autonomous agents against systems of record and have to answer, afterwards: who authorized this, what did it cost, and what did it touch.",
      },
      {
        kind: "prose",
        text: "One rule holds all of it together, stated once: the runtime never holds a provider credential. Every model call — from a human in this console, an OpenAI-compatible client, an agent, or the eval judge — crosses the gateway on a virtual key, so an agent's spend is budgeted and audited exactly like a human's.",
      },
      {
        kind: "diagram",
        diagram: "requestLifecycle",
        caption: "One governed request, hop by hop, and what each hop checks before a provider is reached.",
      },
      {
        kind: "list",
        items: [
          "Governed model access — a Go gateway authenticates a virtual key, enforces per-key and per-org budgets and rate limits, screens for prompt injection, and writes an audit record.",
          "Durable agents — a Python/LangGraph runtime with Postgres checkpointing: resumable threads, reviewed in-repo skills, standing operators, an eval-gated self-improvement loop, and a council that reports its dissent instead of averaging it away.",
          "Legacy systems as tools — SQL, REST, SSH, SOAP, and browser connectors expose old systems as MCP tools, with each safety constraint enforced inside the connector rather than asked of the model.",
          "Untrusted code, contained — a Rust sandbox executes agent-written Python in an isolated process group with no network egress and no published host port.",
        ],
      },
      {
        kind: "prose",
        text: "Nothing runs unauthorized, unattributed, or unrecorded is a claim about the actions the platform executes: each one is authorized against a credential, attributed to a caller and an org, and written to the audit log. Coverage on the refusal side is deliberately partial, and the Security section names exactly which denials are recorded and which are not.",
      },
    ],
  },
  {
    id: "concepts",
    title: "Concepts & Glossary",
    icon: "docs",
    blurb: "The small, specific vocabulary the rest of this handbook assumes.",
    blocks: [
      {
        kind: "prose",
        text: "Each term below is defined once, grouped by the part of the system it belongs to. Everything else in this handbook uses these words in exactly this sense; the docs/concepts.md file in the repository is the same glossary at full length.",
      },
      {
        kind: "keyvals",
        caption: "Planes",
        rows: [
          { k: "Gateway", v: "The Go service every model call flows through, and the only component that ever holds a provider key." },
          { k: "Runtime", v: "The Python service that runs agents: the agent loop, durable threads, skills, operators, and the council. It holds no provider credentials." },
          { k: "Sandbox", v: "The Rust service that executes untrusted Python with per-run isolation and no network egress." },
          { k: "Connector", v: "A service that exposes one legacy system as MCP tools, with the safety constraint enforced inside the connector." },
          { k: "Console", v: "The operator surface you are reading this in." },
        ],
      },
      {
        kind: "keyvals",
        caption: "Identity and tokens",
        rows: [
          { k: "agos-…", v: "Virtual key. A gateway-issued credential for model access, carrying its own org, budget, and rate limit. The only credential accepted on /v1/*." },
          { k: "agu-…", v: "User token. An identity credential for the admin plane, bound to a user, org, and role. Minted by OIDC sign-in or created directly." },
          { k: "AGENTOS_ADMIN_KEY", v: "Root admin key — global superuser for the admin API." },
          { k: "AGENTOS_SCIM_TOKEN", v: "Static shared secret that gates /scim/v2/*. Unset leaves those routes unregistered, so they answer 404." },
          { k: "whk-…", v: "Webhook token. The credential is the URL path segment itself, on the operator webhook route — one of the two runtime paths that take no bearer." },
          { k: "Org / Role", v: "The tenancy boundary, and one of owner, admin, member, or viewer. Roles are evaluated on the admin plane only." },
        ],
      },
      {
        kind: "keyvals",
        caption: "Governance",
        rows: [
          { k: "Budget hold", v: "A reservation taken before an upstream call and settled after, so concurrent calls cannot overspend a cap." },
          { k: "Rate limit", v: "A per-org token bucket (rate_limit_rpm, 0 = unlimited). Over-limit calls return 429 with Retry-After." },
          { k: "Guardrail", v: "Prompt-injection screening at the gateway, in mode off, log, block, or model." },
          { k: "Audit log", v: "The gateway's record of outcomes, as one of seven kinds. Among denials, only rate-limit and guardrail events are recorded." },
          { k: "Role checks", v: "Role-based access control on the admin plane, /admin/* only. Never evaluated on /v1/*, where scoping comes from the virtual key's own org." },
          { k: "HITL", v: "Human-in-the-loop: configured tools pause a run for approval. Off by default." },
          { k: "Secret backend", v: "Where provider keys resolve from: env, file, age, or vault." },
        ],
      },
      {
        kind: "keyvals",
        caption: "Agents",
        rows: [
          { k: "Run / thread", v: "One agent invocation, and its durable, resumable conversation state." },
          { k: "Profile", v: "react (a LangGraph ReAct loop) or deep (deepagents)." },
          { k: "Skill", v: "A reviewed SKILL.md instruction sheet the agent pulls on demand via use_skill." },
          { k: "Operator", v: "A standing objective the runtime pursues on its own, fired by an interval, a cron schedule, or a webhook. Off by default." },
          { k: "Council", v: "N model-bound members answer one objective in parallel; a judge synthesizes one verdict plus an explicit dissent report." },
          { k: "Proposal", v: "A change — a write-class tool call, or a new system prompt — that only a human can activate." },
          { k: "Eval gate", v: "A scored suite, and the CI check that blocks a change scoring below the threshold." },
        ],
      },
      {
        kind: "prose",
        text: "Two integration terms complete the set. MCP is the streamable-HTTP protocol connectors speak, and the runtime discovers tools from the servers listed in AGENTOS_MCP_SERVERS. The provider registry, deploy/providers.json, is config-driven vendor routing whose per-1M-token prices drive budget enforcement — the shipped file is a placeholder whose own comment flags every price as unverified.",
      },
    ],
  },
  {
    id: "architecture",
    title: "Architecture",
    icon: "orgs",
    blurb: "Four planes, and the wiring that makes the guarantee hold.",
    blocks: [
      {
        kind: "prose",
        text: "Four planes, wired together by the credential invariant: the gateway is the only component that ever holds a provider key; the runtime runs agents and borrows model access from the gateway on a virtual key; the sandbox executes untrusted code with no egress; and connectors front one legacy system each as MCP tools.",
      },
      {
        kind: "diagram",
        diagram: "architecture",
        caption: "The four planes and the console, with the wiring that keeps provider credentials on one side of it.",
      },
      {
        kind: "list",
        items: [
          "Gateway — authenticates virtual keys, enforces budgets and rate limits, runs the guardrail, routes to a provider or the council, and writes the audit log. It also carries the admin plane. It does not run agents and does not execute code.",
          "Runtime — the agent loop, durable threads, skills, operators, the council, retrieval, evals, and self-improvement. It reaches legacy systems only through connectors and the sandbox.",
          "Sandbox — one isolated process group per run. It holds no state between runs and makes no decision about whether the code is safe; it only isolates and kills it.",
          "Connectors — one legacy system each, each enforcing its own constraint in code rather than asking the model to behave. None of them holds application state.",
        ],
      },
      {
        kind: "keyvals",
        caption: "Services and ports",
        rows: [
          { k: "postgres  5432", v: "The only stateful store: gateway state, agent checkpoints, and pgvector embeddings." },
          { k: "gateway  8080", v: "OpenAI-compatible edge plus the admin plane." },
          { k: "runtime  8000", v: "Agents, operators, council, documents, evals." },
          { k: "sandbox  8070", v: "No Compose host port at all — reachable only from the runtime, over an internal network." },
          { k: "sql-connector  8090", v: "On by default, and wired into the agent's tool list." },
          { k: "rest-connector  8091", v: "On by default in Compose; its Helm template ships disabled." },
          { k: "ssh-connector  8092", v: "Hardcoded listen port. No Compose service and no Helm template — run it standalone." },
          { k: "soap-connector  8093", v: "Starts only under a Compose profile. No Helm template." },
          { k: "browser-connector  8094", v: "Starts only under a Compose profile. No Helm template." },
          { k: "console  3000", v: "Host port. The container itself listens on 8080." },
        ],
      },
      {
        kind: "prose",
        text: "Postgres is the only stateful store in the platform. The sandbox is stateless between runs, and connectors hold no application state of their own. A trap in the naming: AGENTOS_SSH_PORT is the remote SSH target port the connector dials, default 22 — not the connector's own listen port, which is the hardcoded 8092 above.",
      },
      {
        kind: "prose",
        text: "The Gateway section below walks the request flow stage by stage. The docs/architecture.md file in the repository carries the same material at reference depth, including the full services-and-ports table with each port's environment variable.",
      },
    ],
  },
  {
    id: "gateway",
    title: "Gateway",
    icon: "keys",
    blurb: "Governed model access: one chain, one order, every call.",
    blocks: [
      {
        kind: "prose",
        text: "The gateway is the OpenAI-compatible edge that fronts all model traffic plus the admin plane, and the only part of the platform that ever holds a provider credential. Virtual keys carry their own org, budget, and rate limit; budget holds are reserved before the upstream call so concurrent requests cannot overspend a cap.",
      },
      {
        kind: "diagram",
        diagram: "governanceChain",
        caption: "The fixed order every /v1/chat/completions request passes through.",
      },
      {
        kind: "keyvals",
        caption: "The /v1/chat/completions pipeline, in this fixed order",
        rows: [
          { k: "1. Auth", v: "The request must carry a valid agos- virtual key; otherwise 401." },
          { k: "2. Rate limit", v: "The key's org must have room in its requests-per-minute bucket; otherwise 429 with Retry-After." },
          { k: "3. Budget hold", v: "A reservation against both the key's and the org's monthly cap; exhaustion is enforced with 402, not advisory." },
          { k: "4. Guardrail", v: "Runs only when AGENTOS_GUARDRAILS_MODE is not off. A flagged prompt is blocked with 400 in block and model mode, or logged and forwarded in log mode." },
          { k: "5. Upstream", v: "Routed by model prefix to a provider, or, for a council/… model, to the runtime's council." },
          { k: "6. Audit", v: "The outcome is written to the audit log." },
          { k: "/v1/embeddings", v: "Runs the identical chain minus the guardrail step." },
        ],
      },
      {
        kind: "keyvals",
        caption: "Audit kinds — the log records outcomes as one of exactly seven",
        rows: [
          { k: "chat", v: "A served /v1/chat/completions." },
          { k: "embeddings", v: "A served /v1/embeddings." },
          { k: "guardrail_flag", v: "The classifier flagged a prompt and the request was forwarded rather than blocked." },
          { k: "guardrail_block", v: "The classifier flagged a prompt and the request was blocked." },
          { k: "guardrail_error", v: "A classifier outage: screening could not complete, and the request was admitted anyway." },
          { k: "rate_limited", v: "A 429 rejection." },
          { k: "secret_reload", v: "A secret backend reload." },
        ],
      },
      {
        kind: "prose",
        text: "Read the coverage precisely, because it is asymmetric. A served request writes both a usage record and an audit entry. Among refusals, only the rate-limit rejection and the guardrail outcomes are recorded — a 401 auth failure, a 400 malformed request, and a 402 budget exhaustion write nothing at all. If you need a complete record of rejected traffic, take it from your ingress logs rather than the audit table.",
      },
      {
        kind: "keyvals",
        caption: "Fail-open vs fail-closed",
        rows: [
          { k: "Budget", v: "Fail closed on a verdict: HTTP 402. Only a store or database error admits the request, and — unlike the guardrail below — that admission is not audited; it reads as an ordinary success." },
          { k: "Rate limit", v: "Fail closed on a verdict: 429 with Retry-After. The Postgres backend fails open on a database error; the memory backend has no such path." },
          { k: "Guardrail", v: "Fail closed on a verdict: the request is blocked and audited. A classifier outage fails open, and leaves its own guardrail_error entry." },
        ],
      },
      {
        kind: "prose",
        text: "The admin plane is not gated the way the proxy path is. Role checks apply to /admin/* only, via the root admin key or an agu- user token that is role-checked per action; POST /admin/secrets/reload is root-only and rejects even the most privileged user token. /scim/v2/* is gated separately, by a static shared-secret bearer compared on every request, with no role evaluation at all. /auth/oidc/* carries no auth wrapper — it is the public login and callback flow a browser follows. Role checks are never evaluated on /v1/*: org scoping there comes entirely from the virtual key's own org.",
      },
    ],
  },
  {
    id: "runtime",
    title: "Runtime",
    icon: "playground",
    blurb: "Agents that survive a restart, and never hold a provider key.",
    blocks: [
      {
        kind: "prose",
        text: "The runtime runs the agent loop over durable, resumable threads checkpointed in Postgres, so a run survives a restart. Every model call it makes — including the eval judge and the guardrail classifier — goes back through the gateway on a virtual key. It holds no provider credentials of its own, and it reaches legacy systems only through connectors and the sandbox.",
      },
      {
        kind: "keyvals",
        caption: "Agent profiles (AGENTOS_AGENT_PROFILE)",
        rows: [
          { k: "react", v: "The default: a LangGraph ReAct loop. The only profile where human-in-the-loop tool approval works — a call to a tool named in AGENTOS_APPROVAL_TOOLS pauses the graph, the run returns pending_approval, and a human resumes or denies it." },
          { k: "deep", v: "The deepagents planner. It compiles its own graph and exposes no interrupt_before, so tool approval and in-graph council write gating do not apply to it." },
        ],
      },
      {
        kind: "keyvals",
        caption: "What the runtime runs",
        rows: [
          { k: "Skills", v: "SKILL.md instruction sheets pulled on demand via use_skill. Loaded only from the image-baked skills directory and never fetched at runtime; each load records a sha256 for provenance, which is logged rather than compared against an expected value. The integrity guarantee comes from the skills being in the image." },
          { k: "Operators", v: "Standing objectives fired by an interval, a cron schedule, or a webhook. The scheduler is off unless AGENTOS_AUTONOMY_ENABLED is true; webhook operators still fire on their endpoint." },
          { k: "Self-improvement", v: "Eval-gated prompt proposals. No proposal activates without a human, and an immutable safety preamble is always prepended to whatever prompt wins." },
          { k: "Council", v: "N model-bound members answer one objective in parallel; a judge synthesizes one verdict plus an explicit dissent report." },
        ],
      },
      {
        kind: "diagram",
        diagram: "councilFanout",
        caption: "One objective, N model-bound members, one judged verdict — and the dissent reported rather than averaged away.",
      },
      {
        kind: "note",
        text: "Council write gating is fail-closed for react-profile members: anything outside a small read-safe set is held as a proposal for a human instead of being executed. A deep-profile member gets no in-graph gate, because deepagents exposes no interrupt_before — it is bounded solely by the hand-written read-only tools: allowlist on its entry in council.yaml. That list is its entire action surface. The five frontier members, all of them deep-profile, ship disabled: their endpoints, model ids, and prices in deploy/providers.json are unverified placeholders by that file's own admission, and those prices drive budget enforcement, so confirm each one before enabling it.",
      },
      {
        kind: "keyvals",
        caption: "Runtime API — every route requires the AGENTOS_RUNTIME_AUTH_TOKEN bearer except the last row",
        rows: [
          { k: "POST /runs, /runs/stream", v: "Run the agent to completion, or stream it as Server-Sent Events." },
          { k: "POST /runs/{thread_id}/approve", v: "Approve or deny a paused tool call and resume the run." },
          { k: "GET /threads/{thread_id}", v: "A thread's message history." },
          { k: "GET, POST /documents", v: "List and ingest context-engine documents." },
          { k: "/evals/*, /improve, /proposals/*", v: "Eval runs and the self-improvement flow; approving a prompt proposal hot-swaps the live agent." },
          { k: "/council/*", v: "Objectives and their cycles, members, pause and resume, and approval of held write-class calls." },
          { k: "/operators/*", v: "Standing operators, their recent runs, and the loaded skill list with each skill's provenance digest." },
          { k: "GET /healthz, POST /operators/webhooks/{token}", v: "The only two paths exempt from the bearer. On the webhook, the opaque whk- token in the path is itself the credential, and an unknown token answers 404 rather than 401." },
        ],
      },
      {
        kind: "prose",
        text: "Autonomy is off by default throughout: operators do not fire, HITL is not configured, and the council serves its API without acting on its own until AGENTOS_COUNCIL_HEARTBEAT_S is set. The docs/api.md file in the repository is the full endpoint and tool reference.",
      },
    ],
  },
  {
    id: "sandbox",
    title: "Sandbox",
    icon: "secrets",
    blurb: "Untrusted code, contained: no egress, no published host port.",
    blocks: [
      {
        kind: "prose",
        text: "The sandbox is the Rust service backing the agent's run_python tool. It executes agent-written Python in an isolated child process, holds no state between runs, and makes no decision about whether the code is safe to run — it only isolates and kills it. Its request and response shapes are a frozen contract.",
      },
      {
        kind: "keyvals",
        caption: "Per-run isolation",
        rows: [
          { k: "Fresh workdir", v: "Code is written to a temporary directory that is removed when the run ends." },
          { k: "Cleared environment", v: "The child sees a restricted PATH and nothing else." },
          { k: "New process group", v: "The group is SIGKILLed on timeout, so anything the code spawned dies with it." },
          { k: "rlimits", v: "CPU time, address space, process count, and file size are all capped." },
          { k: "Wall-clock timeout", v: "Requested timeout_s is clamped to AGENTOS_SANDBOX_MAX_TIMEOUT_S (default 30); stdout and stderr are each capped at AGENTOS_SANDBOX_MAX_OUTPUT_BYTES (default 65536)." },
        ],
      },
      {
        kind: "keyvals",
        caption: "Per-container hardening",
        rows: [
          { k: "Filesystem", v: "Read-only root filesystem with a tmpfs for /tmp, because per-run workdirs live there." },
          { k: "Privileges", v: "All capabilities dropped, no-new-privileges, non-root user." },
          { k: "Resources", v: "Memory, pid, and CPU caps at the container level." },
          { k: "Network", v: "An internal-only network with no outbound route, and no published host port." },
        ],
      },
      {
        kind: "prose",
        text: "Egress is closed at the network level, not merely mitigated. Under Compose the sandbox sits alone on an internal Docker network with no ports mapping — Docker forbids publishing a host port from an internal network — so the runtime, attached to both networks, is the only way in. Helm ships a matching NetworkPolicy restricting the sandbox pod to ingress from the runtime pod and egress to kube-dns only, and the repository records a verification run proving a container on that network can neither resolve nor reach the internet.",
      },
      {
        kind: "keyvals",
        caption: "Contract",
        rows: [
          { k: "POST /execute", v: "Request {language, code, timeout_s, stdin} → response {exit_code, stdout, stderr, duration_ms, timed_out, truncated}." },
          { k: "language", v: "Accepts only \"python\"." },
          { k: "AGENTOS_SANDBOX_URL", v: "Runtime-side setting; empty disables the run_python tool entirely." },
        ],
      },
      {
        kind: "note",
        text: "These layers bound what a run can do to the host and to the network. They do not judge whether the Python being executed is well-intentioned — that is a residual risk, not a covered one.",
      },
    ],
  },
  {
    id: "connectors",
    title: "Connectors",
    icon: "external-link",
    blurb: "Five legacy systems as MCP tools, each constraint enforced in code.",
    blocks: [
      {
        kind: "prose",
        text: "A connector fronts one legacy system and exposes it as MCP tools. Every safety constraint lives inside the connector, in code, rather than being asked of the model — an agent does not open sockets, it calls tools, and the tool refuses before the call leaves the connector.",
      },
      {
        kind: "keyvals",
        caption: "The five connectors",
        rows: [
          { k: "sql  8090", v: "Read-only: a statement validator, a READ ONLY transaction, a read-only database role, a row cap (AGENTOS_CONNECTOR_MAX_ROWS, default 200), and a statement_timeout." },
          { k: "rest  8091", v: "GET-only unless AGENTOS_REST_ALLOW_MUTATIONS is set; SSRF-screened, with the auth header stripped across redirects and a response-body cap." },
          { k: "ssh  8092", v: "A command basename allowlist, a deny-list of exec-capable binaries, and rejection of chaining characters. Host-key verification is fail-closed." },
          { k: "soap  8093", v: "WSDL 1.1, an explicit operation allowlist (empty by default, so nothing is allowed), SSRF-screened." },
          { k: "browser  8094", v: "A domain allowlist (empty by default), read and navigate only, with no form-submission surface." },
        ],
      },
      {
        kind: "keyvals",
        caption: "Tool catalog — what an agent can actually call",
        rows: [
          { k: "query, list_tables, describe_table", v: "sql-connector. Read-only query and metadata." },
          { k: "list_operations, plus generated tools", v: "rest-connector and soap-connector. One tool per OpenAPI or WSDL operation." },
          { k: "run_command, list_allowed", v: "ssh-connector. Runs an allowlisted command and reports the allowlist." },
          { k: "navigate, get_text, find_links, click", v: "browser-connector. Allowlisted navigation and reading." },
          { k: "run_python", v: "The sandbox, via the runtime. Enabled only when AGENTOS_SANDBOX_URL is set." },
          { k: "use_skill", v: "The runtime itself. Loads an in-repo SKILL.md instruction sheet." },
        ],
      },
      {
        kind: "note",
        text: "A running connector is not a reachable one, and this is the single thing that trips people up. A tool is callable only if its server's URL is present in AGENTOS_MCP_SERVERS — which, in the shipped deploy/compose.yaml, is a hardcoded literal wiring in SQL and REST with no substitution, so it cannot be overridden from .env. Starting SOAP and browser with docker compose --profile connectors starts their containers but does not expose their tools to the agent; their URLs have to be appended by editing compose.yaml directly.",
      },
      {
        kind: "prose",
        text: "The deployment asymmetry is deliberate. SQL and REST are wired in by default and both have Helm templates, though the REST template ships disabled. SOAP and browser are Compose-profile-only with no Helm template. SSH has neither a Compose service nor a Helm template and must be run and wired up by hand. Under Helm, runtime.extraMcpServers is the supported way to append a server the chart has no template for.",
      },
    ],
  },
  {
    id: "console",
    title: "Console",
    icon: "provisioning",
    blurb: "The operator surface — and where its authority actually comes from.",
    blocks: [
      {
        kind: "prose",
        text: "The console is a single-page app served by nginx: keys, orgs, users, secrets, and provisioning on the administration side, and running, observing, and approving agent work on the other. It never asks you to declare your own role or org — it calls GET /admin/whoami with whatever token it holds, and the gateway's answer drives which pages appear, whose data loads, and what actions are offered.",
      },
      {
        kind: "keyvals",
        caption: "The twelve operator pages",
        rows: [
          { k: "/", v: "Overview — usage, spend, and recent audit events for the org's keys." },
          { k: "/keys", v: "Keys — virtual keys and their budgets; create one, with the secret shown exactly once." },
          { k: "/audit", v: "Audit — recent gateway events: chat, embeddings, guardrail flags and blocks." },
          { k: "/playground", v: "Playground — run an agent turn, watch tool calls stream in as a timeline, approve or deny a paused call." },
          { k: "/documents", v: "Documents — list and ingest documents into the context engine." },
          { k: "/improve", v: "Improve — run the eval suite, review a proposed system prompt, approve or deny it." },
          { k: "/multiverse", v: "Multiverse — council objectives, member proposals, and the write-class calls they held back." },
          { k: "/operators", v: "Operators — create, enable, fire on demand, and inspect recent runs." },
          { k: "/orgs", v: "Orgs — create orgs and adjust per-org rate limits. Requires org.view." },
          { k: "/users", v: "Users — invite and remove users within an org. Requires user.view." },
          { k: "/secrets", v: "Secrets — which secrets are configured, status only and never values; trigger a reload. Requires secret.view." },
          { k: "/provisioning", v: "Provisioning — SCIM status for org and user sync from an identity provider. Requires provisioning.view." },
        ],
      },
      {
        kind: "prose",
        text: "The four gated pages each require the named permission; everything else renders for every role, including viewer. This handbook is the one page that fetches nothing of its own: it reads no credential, and the only traffic while it is open is the shell's governance-chain and liveness polling, which runs on every route. Hiding a page is a UI affordance and never the security boundary: the gateway re-checks the same permission on every /admin/* call using the token's real role, so a scripted client calling a gated endpoint directly gets exactly the decision a role-appropriate session would have gotten.",
      },
      {
        kind: "prose",
        text: "The runtime side is different, and it is the sharpest thing to know about this console. nginx injects the runtime bearer on the /api/runtime/ location unconditionally, from the container's own environment, so the browser never holds the token and nothing it runs can read it. But the runtime has no role concept to check a caller against, which means the console's server-side proxy carries full runtime authority: reaching the console's port is equivalent to holding the runtime token. Confidentiality and access control are separate properties here — the token stays server-side, and the port is the boundary.",
      },
      {
        kind: "note",
        text: "Anyone who can reach the console's port can drive the full runtime API — create or delete operators, ingest documents, approve a held council write, or hot-swap the live system prompt. There is no second check behind it. Bound that port at the network layer, because nothing in the application layer bounds it.",
      },
      {
        kind: "prose",
        text: "The browser makes same-origin requests only, to /api/gateway/* and /api/runtime/*; there is no third-party traffic and no CDN dependency. Sign in with the root admin key or an agu- user token, or with SSO when the gateway has AGENTOS_OIDC_ISSUER configured. Most pages poll their endpoint on their own cadence, pause while the tab is hidden, and report synced, reconnecting, or offline in the topbar, so a stale screen never looks live. Every route is also reachable from the ⌘K command palette, filtered by the same visibility rules the sidebar uses. The Improve page needs AGENTOS_CHECKPOINT_DATABASE_URL on the runtime; without it, self-improvement, council, and operator state do not persist and the page shows a disabled empty state.",
      },
    ],
  },
  {
    id: "quickstart",
    title: "Quickstart",
    icon: "run",
    blurb: "Two paths to a running stack: a provider key, or a local Ollama at $0.",
    blocks: [
      {
        kind: "prose",
        text: "Prerequisites are Docker with Compose, and either a provider API key or a local Ollama. Everything runs on one host; the console comes up on port 3000 and you sign in with the admin key from deploy/.env.",
      },
      {
        kind: "note",
        text: "Two variables are mandatory before anything else. The gateway refuses to start without AGENTOS_ADMIN_KEY, and the runtime refuses to start without AGENTOS_RUNTIME_AUTH_TOKEN — the shared bearer every runtime route requires except GET /healthz and the operator webhook path. deploy/.env.example ships placeholder values for both so a local bring-up works; change them for anything that is not your laptop.",
      },
      {
        kind: "code",
        lang: "bash",
        code: `cd deploy
cp .env.example .env        # set AGENTOS_ANTHROPIC_API_KEY (or AGENTOS_OPENAI_API_KEY)
cd ..
make up                     # builds and starts the default services
make smoke                  # end-to-end: the agent answers from the seeded legacy ERP database`,
      },
      {
        kind: "prose",
        text: "The demo ships a seeded legacy ERP Postgres database — customers, orders, invoices — that the agent can reach only through the SQL connector's read-only tools, the same path a real system of record would take. Ask it something from the Playground page, then watch the governance side of the same request on Overview and Audit.",
      },
      {
        kind: "keyvals",
        caption: "The local-Ollama $0 path — set all four in deploy/.env before starting",
        rows: [
          { k: "AGENTOS_MODEL", v: "ollama/qwen3.6:latest" },
          { k: "AGENTOS_EMBED_MODEL", v: "ollama/bge-m3" },
          { k: "AGENTOS_JUDGE_MODEL", v: "ollama/qwen3.6:latest" },
          { k: "AGENTOS_OLLAMA_BASE_URL", v: "http://host.docker.internal:11434" },
        ],
      },
      {
        kind: "prose",
        text: "A bare make up is not the $0 path: the shipped defaults point AGENTOS_MODEL and AGENTOS_JUDGE_MODEL at Anthropic, and even the already-local embedding default cannot resolve without an Ollama base URL. With all four set, make up and every make smoke target run without a single third-party call, and make smoke9 runs a real five-model council against the seeded ERP at $0. Those local members run the react profile, not deep — on deep, local models exhaust the recursion limit inside deepagents' own graph.",
      },
    ],
  },
  {
    id: "configuration",
    title: "Configuration",
    icon: "settings",
    blurb: "How the settings are grouped, and the few that decide whether anything starts.",
    blocks: [
      {
        kind: "prose",
        text: "Every AGENTOS_* environment variable the platform reads is catalogued in the docs/configuration.md file in the repository, grouped by subsystem, with defaults read from the Go, Python, and Rust sources rather than from comments in the example env file. This section names the groups and the handful of settings that decide whether the platform does anything at all.",
      },
      {
        kind: "keyvals",
        caption: "Required — these have no fallback",
        rows: [
          { k: "AGENTOS_ADMIN_KEY", v: "Root key for the gateway admin API. The gateway refuses to start without it." },
          { k: "AGENTOS_RUNTIME_AUTH_TOKEN", v: "Shared bearer for the runtime API. The runtime refuses to start without it." },
          { k: "AGENTOS_GATEWAY_URL", v: "Where the runtime sends every model call." },
          { k: "AGENTOS_GATEWAY_KEY", v: "The agos- virtual key the runtime authenticates to the gateway with." },
        ],
      },
      {
        kind: "keyvals",
        caption: "The groups",
        rows: [
          { k: "Core", v: "Admin key, runtime token, database URL, and the bootstrap org and keys." },
          { k: "Models & providers", v: "Agent, embedding, judge, and guardrail models; provider credentials; the provider registry file." },
          { k: "Governance", v: "Guardrail mode and budget, approval tools, rate limits and their backend, budget reserve, body cap, audit retention, CORS." },
          { k: "Identity & secrets", v: "Secret backend and its file or Vault settings, OIDC SSO, SCIM." },
          { k: "Runtime & agents", v: "Agent profile, MCP servers, context engine, checkpoint database, sandbox URL, skills directory." },
          { k: "Council", v: "Runtime URL, config file, heartbeat, and the per-objective spend ceiling." },
          { k: "Operators & skills", v: "Autonomy toggle, tick interval, per-run cycle cap, skills directory." },
          { k: "Sandbox, Connectors, Console & networking, Observability, Ports", v: "Per-service tuning, the OTLP endpoint, and the host-side port mappings. The console itself takes no configuration beyond the runtime token and its own port." },
        ],
      },
      {
        kind: "keyvals",
        caption: "The four people get wrong",
        rows: [
          { k: "AGENTOS_GUARDRAILS_MODE", v: "Plural GUARDRAILS. It reads like a typo and is not. Values are off, log, block, or model; the default is off, so no screening runs until you set it." },
          { k: "AGENTOS_MCP_SERVERS", v: "A hardcoded literal in the shipped compose.yaml with no substitution, so setting it in .env changes nothing. Edit compose.yaml, or use Helm." },
          { k: "AGENTOS_SSH_PORT", v: "The remote SSH target port, default 22 — not the connector's own listen port, which is a hardcoded 8092." },
          { k: "AGENTOS_RATE_LIMIT_RPM", v: "0 means unlimited, not blocked — and 0 is the shipped default, so no rate limit applies to an org until one is set." },
        ],
      },
      {
        kind: "prose",
        text: "Two defaults are worth knowing before you rely on them. Secret rotation is not automatic: AGENTOS_SECRETS_REFRESH_S is 0, so a rotated key takes effect only through POST /admin/secrets/reload or a restart. And audit retention is off by default, because deciding to discard audit history is a decision an operator should make deliberately.",
      },
    ],
  },
  {
    id: "deploy",
    title: "Deploy",
    icon: "export",
    blurb: "Compose on a laptop, Helm on a cluster, CI with an eval gate.",
    blocks: [
      {
        kind: "prose",
        text: "Docker Compose runs the whole platform on a laptop or a single host, with optional overlays layered on top. For Kubernetes, the repository ships a Helm 3 chart with per-service toggles, a bundled or external Postgres, a hardened sandbox pod with its NetworkPolicy, and an optional console ingress.",
      },
      {
        kind: "code",
        lang: "bash",
        code: `make up                      # build and start the default services
make down                    # stop the stack and remove the Postgres volume, so the next up reseeds the demo data
make logs                    # follow the stack's logs
deploy/helm/test-render.sh   # helm lint --strict plus helm template under several value combinations`,
      },
      {
        kind: "keyvals",
        caption: "Overlays — each is one extra Compose file",
        rows: [
          { k: "compose.hitl.yaml", v: "Human-in-the-loop governance: requires approval for the query tool and switches the guardrail to block." },
          { k: "compose.otel.yaml", v: "An OpenTelemetry collector; gateway and runtime emit a span per request, model call, and tool call." },
          { k: "compose.langfuse.yaml", v: "A bundled Langfuse UI, layered on top of the OTel overlay." },
          { k: "compose.auth-mocks.yaml", v: "A mock OIDC provider and a mock Vault, so SSO and the Vault backend can be exercised offline." },
          { k: "openclaw/compose.openclaw.yaml", v: "An OpenClaw worker running as a budgeted, audited client of the gateway." },
        ],
      },
      {
        kind: "keyvals",
        caption: "Connector deployment tiers",
        rows: [
          { k: "SQL + REST", v: "Wired into AGENTOS_MCP_SERVERS by default, as a hardcoded literal with no substitution, so it cannot be overridden from .env. Both have Helm templates; restConnector.enabled defaults to false." },
          { k: "SOAP + browser", v: "Start only under docker compose --profile connectors (or the narrower per-connector profiles). Starting them does not make their tools reachable — append their URLs by hand. Neither has a Helm template." },
          { k: "SSH", v: "No Compose service and no Helm template at all. Run it standalone and add its URL yourself." },
          { k: "Helm", v: "Composes the MCP list from whichever connectors are enabled, and appends anything in runtime.extraMcpServers." },
        ],
      },
      {
        kind: "keyvals",
        caption: "Helm enabled defaults",
        rows: [
          { k: "on", v: "postgres, gateway, runtime, sqlConnector, sandbox, console — plus sandbox.networkPolicy, which restricts the sandbox pod to ingress from the runtime and egress to kube-dns." },
          { k: "off", v: "restConnector, demoCrm, ingress. Ingress exposes only the console; the gateway and runtime APIs are reached through the console's own proxy paths." },
        ],
      },
      {
        kind: "prose",
        text: "CI runs five independent jobs on every push — Go (the gateway and four Go connectors), Python, Rust, console, and a Helm render check — so a failure in one does not mask the others. The eval gate is a separate, opt-in workflow: it boots the full stack against a deterministic offline mock model and fails the build if the suite scores below the threshold.",
      },
      {
        kind: "note",
        text: "Two gaps to know before you trust a green run. The Python browser connector is in no CI job and no make target — it runs nowhere but by hand. And make test covers Go, Python, and the console but excludes the Rust sandbox suite, which has to be run separately.",
      },
    ],
  },
  {
    id: "security",
    title: "Security",
    icon: "audit",
    blurb: "What the platform enforces, where it degrades on purpose, and what it does not cover.",
    blocks: [
      {
        kind: "prose",
        text: "The posture is stated as what the platform actually enforces, where it deliberately degrades rather than fails, and what it does not cover. Controls described here are present in source; anything not described should be assumed absent. The SECURITY.md file in the repository is the current statement in full, and it names the source location behind each claim.",
      },
      {
        kind: "keyvals",
        caption: "Trust boundaries",
        rows: [
          { k: "Provider egress", v: "The gateway is the only path to a model provider. Nothing else is configured with a provider key." },
          { k: "Sandbox", v: "No egress at all and no published host port; only the runtime can reach it." },
          { k: "Legacy systems", v: "Connectors are the only path. Agents do not open sockets; they call tools that refuse before the call leaves." },
          { k: "Console token", v: "The browser bundle never sees the runtime bearer — nginx injects it server-side." },
          { k: "Console port", v: "That injection is unconditional and the runtime has no roles, so the proxy is an unauthenticated path to the full runtime API." },
          { k: "Open runtime paths", v: "GET /healthz, so orchestrator probes need no credential; and the operator webhook path, where the opaque token in the URL is the only credential. Treat a webhook URL as a secret." },
        ],
      },
      {
        kind: "keyvals",
        caption: "Fail-open vs fail-closed — a backend outage must not take traffic down; a policy verdict must not be bypassable",
        rows: [
          { k: "Budget", v: "Fail closed on a verdict — HTTP 402. Fails open only on a store error, and that admission is not audited, which makes it the one blind spot in this table that is not self-reporting." },
          { k: "Rate limit", v: "Fail closed — 429 with Retry-After. The Postgres backend fails open on a database error; the memory backend has no such path." },
          { k: "Guardrail", v: "Fail closed on a verdict — blocked and audited. Fails open on a classifier outage, with its own guardrail_error entry." },
          { k: "OIDC email_verified", v: "Fail closed." },
          { k: "SSH host keys", v: "Fail closed — the connector refuses to start without a known_hosts file, absent an explicit, loudly-warned development opt-out." },
          { k: "Runtime auth token", v: "Fail closed — the runtime refuses to start without it." },
          { k: "Council write gating", v: "Fail closed, treating an unclassified tool as write-class — for react-profile members only." },
        ],
      },
      {
        kind: "prose",
        text: "Two coverage facts follow from the audit design and should shape how you use it. A served request writes a usage record and an audit entry together, in one transaction. Among refusals, only rate-limit rejections and guardrail events are recorded — a 401 auth failure, a 400 malformed request, and a 402 budget exhaustion write nothing — so a complete record of rejected traffic has to come from your ingress logs.",
      },
      {
        kind: "prose",
        text: "The residual risks, stated plainly so nothing above reads as more than it is:",
      },
      {
        kind: "list",
        items: [
          "A model can act badly within its permissions. Every control here bounds what an agent may reach; none makes its judgment sound.",
          "Prompt injection via content the agent reads. Retrieved documents, web pages, and API responses are untrusted input that reaches the model; the safety preamble and untrusted-content delimiters raise the cost of an attack without closing the class.",
          "The browser connector has no IP-level backstop. Its allowlist matches on hostname only and never resolves the host, so unlike REST and SOAP it has no private-address refusal. Keep that list narrow and hold it to names you control.",
          "Budget enforcement is untested end to end against paid models. The admission race is covered by a test; the full path against a real metered provider is not.",
          "The console's port is the runtime's authorization boundary, and there is no second check behind it.",
          "The deep agent profile has no human-in-the-loop, and the council's in-graph write gating is limited the same way.",
          "Human-in-the-loop is off by default: tools run unattended unless you opt in.",
        ],
      },
      {
        kind: "prose",
        text: "The record of how the platform got here is kept deliberately. A four-audit security assessment run before any autonomy work found a real unauthenticated remote-code-execution chain and other issues; all were fixed and verified first — runtime auth added and fail-closed at startup, the SSH allowlist backed by an exec-capable deny-list and required host keys, an immutable safety preamble prepended to every prompt, redirect and private-address screening on the REST and SOAP connectors, verified emails and stable subject matching on OIDC, and a cross-org usage leak closed by scoping on the key's own hash and org.",
      },
    ],
  },
];
