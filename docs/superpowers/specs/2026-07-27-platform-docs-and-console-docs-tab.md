# Platform Documentation Overhaul + Visual In-Console Docs Handbook

- **Date:** 2026-07-27
- **Status:** Approved (design)
- **Repo:** `ifahad/agentos` (PRIVATE — local/not-pushed)
- **Supersedes/relates:** [[agentos-console-track-a-design]] (Track A shipped); reuses the "Instrument" design system and the landing-page animation patterns.

## 1. Problem

AgentOS is a large, four-plane platform (gateway / runtime / sandbox / connectors + console + landing + deploy), but nothing ties its capabilities together in one place:

- The **README** (331 lines) is a phase-by-phase changelog — accurate but hard to navigate, with no concepts/glossary, no real architecture section, a **stale repository-layout table** (lists 1 connector, not 5; says "Helm later" though Helm shipped), and private-repo CI badges that 404 for anyone else.
- `docs/project-context.md` (26 KB) is an excellent briefing but written internally (dev-host specifics, "the owner's words") and **factually stale in several sections** — §8 "Known open backlog" and §13 "Honest gaps" list as open a long list of controls that have since shipped, and §10 still calls Multiverse "designed, not implemented" (see §6 for the exact correction list).
- `connectors/ssh/README.md` documents host-key handling as warn-and-continue; the **code actually fails closed** (requires `AGENTOS_SSH_KNOWN_HOSTS` or an explicit `AGENTOS_SSH_INSECURE_HOST_KEY=true`). `deploy/.env.example:65` carries the same wrong claim.
- There is **no in-product documentation** at all — an operator in the running console cannot learn what the platform does without leaving it.

The landing page copy is the sharpest positioning in the repo and is a good source for messaging.

## 2. Goals

1. **Overhaul the GitHub docs** into a navigable, concept-first set (README + a full `docs/` set), correcting the known inaccuracies.
2. **Add a new top-level `Docs` tab** to the console: a **visual, animated, in-app handbook** covering every platform capability and "all related information", built entirely within the Instrument design system and the air-gap invariant.
3. Keep both aligned to **one canonical capability taxonomy** so a reader is never disoriented moving between them.

## 3. Non-goals

- No changes to platform behavior, endpoints, or dependencies. Docs-and-console only.
- No markdown-rendering dependency in the console (decided: typed content model + Instrument UI). No syntax-highlighting dependency either.
- No pushing to origin (repo stays private/local).
- Not building a docs search engine in v1 (but the content model must not preclude one — see §7).
- No routing-model change: in-page back/forward history within Docs sections is out of scope for v1 (§7.3).

## 4. Constraints & invariants (must hold)

- **Private/local:** everything merges to `main` locally; nothing is pushed.
- **Air-gap:** the console (and landing) make **no external requests** and gain **no new runtime dependencies**. Doc visuals are inline SVG/React, fonts self-hosted, images (if any) in-repo.
- **Chroma reserved for machine state:** the only hues are `--live` (in-flight) / `--ok` / `--hold` (awaiting human) / `--deny`; `--accent` resolves to ink. Doc **notes/callouts are monochrome** (neutral `.notice`). The animated governance visuals **may** use the four state hues — because they depict genuine machine state (a request in-flight vs approved vs held vs denied), which is exactly the sanctioned use.
- **Illustration provenance:** the console's live instruments are evidence-driven (`lib/chain.ts`: the console must never draw checks it cannot prove ran). Any docs visual that reproduces a live console instrument must **disclose that it is scripted** — a visible eyebrow/caption on the visual itself (e.g. `illustration · scripted sequence`), not only in surrounding prose. Hue semantics are unchanged; the disclosure is about provenance.
- **Motion discipline:** all animation comes from `src/ui/motion.ts` presets (or framer-motion timelines built on its `EASE`); **no `Math.random`** — visuals are driven by deterministic, scripted/seeded sequences; **reduced motion → a representative static still with no running intervals** (the pattern the landing page already established).
- **Design system:** compose existing primitives (`PageHead`, `Panel`/`PanelHead`, `Card`, `Badge`, `Table`, `.mono`, `.notice`, `Icon`/`StateIcon`); no new color tokens; mono face for machine-produced values, sans for prose.

## 5. Canonical capability taxonomy (shared spine)

Both work-streams organize around the **same ordered sections**:

`Overview → Concepts & Glossary → Architecture → Gateway (governed model access) → Runtime (agents) → Sandbox → Connectors → Console (operator surface) → Quickstart → Configuration → Deploy → Security`

Both surfaces carry **all sections, same names, same order**. They differ in medium and depth, not structure: GitHub renders them as Markdown (README headings plus `docs/*.md` and per-service READMEs for depth), the console as typed content + animated visuals, curating *depth per section* rather than dropping sections. `docs/api.md` and `CHANGELOG.md` are reference material outside the taxonomy.

### Capability content (source of truth, from the platform scan)

- **Gateway** (Go, `:8080`): OpenAI-compatible proxy (`/v1/chat/completions`, `/v1/embeddings`); virtual keys (`agos-`); atomic per-key + per-org **budget holds** — exhaustion is enforced with HTTP 402, **only a store/DB error fails open**, and — unlike the guardrail's classifier-error path — that admission is **not** audited; 15-min reservation TTL, `AGENTOS_BUDGET_RESERVE_USD`, default $0.05. **Rate limits** (in-memory per-replica or Postgres-distributed; the Postgres backend fails open on DB error, the memory backend has no such path); **guardrails** (`off|log|block|model`; fail-open on classifier error, audited); **RBAC** (owner/admin/member/viewer + root admin key) — an **admin-plane** control, see the pipeline note below; **secrets** (`env|file|age|vault` + `/admin/secrets/reload` rotation); **OIDC SSO** (sub-keyed, `email_verified` required); **SCIM 2.0** (token-gated); **audit log** (+ optional retention); **OTel**; **council routing** (`council/*` → runtime, recursion-guarded); **operator provider registry** (SSRF-screened); retries + SSE passthrough. Admin API under `/admin/*`; identity under `/auth/oidc/*` and `/scim/v2/*`.
  - **Actual `/v1/chat/completions` pipeline:** `Auth (agos- virtual key) → Rate limit → Budget hold → Guardrail (only when AGENTOS_GUARDRAIL_MODE != off) → Upstream provider or council → Audit`. `/v1/embeddings` runs the same chain minus the guardrail. Audit records outcomes as one of seven kinds (`chat`, `embeddings`, `guardrail_flag`, `guardrail_block`, `guardrail_error`, `rate_limited`, `secret_reload`); among denials, only rate-limit rejections and guardrail events are audited — a `401` auth failure and a `402` budget exhaustion are not. **RBAC is never evaluated on `/v1/*`** — it gates `/admin/*` only, via `agu-` user tokens and the root admin key. `/scim/v2/*` is gated separately by a static shared-secret bearer token, not a role; `/auth/oidc/*` carries no auth wrapper at all (the public login/callback flow). Org scoping on the proxy path comes from the virtual key's own org.
- **Runtime** (Python, `:8000`): LangGraph ReAct agent or **deep-agent** profile; talks to models **only through the gateway** (holds no provider keys); **immutable SAFETY_PREAMBLE**; **skills** (`SKILL.md`, in-repo/image-baked, never fetched at runtime; each loaded skill records a sha256 **for provenance** — there is no comparison against a pinned expected digest); **HITL** tool approval; **RAG/knowledge** (pgvector, untrusted-data fenced); **evals** + **eval-gate**; eval-gated **self-improvement** (human-approved, override-marker rejection); **Operators** (interval/cron/webhook, off by default); **Multiverse council** (N models → judged verdict + dissent, with **fail-closed action gating** — reads run, every other tool call is held as a human-approved proposal — enforced **in-graph for `react`-profile members**; `deep`-profile members are constrained instead by an explicit read-only `tools:` allowlist in `council.yaml`, because deepagents exposes no `interrupt_before`). Endpoints: `/runs*`, `/threads*`, `/documents*`, `/evals*`, `/improve`, `/proposals*`, `/prompts*`, `/operators*`, `/council/*`.
- **Sandbox** (Rust, `:8070`): per-process isolation (cleared env, rlimits CPU/AS/NPROC/FSIZE, process-group SIGKILL); container hardening (read-only rootfs, `cap_drop: ALL`, non-root); **default-deny egress** (internal-only Docker network, verified); Python only; `POST /execute`; surfaced as `run_python`.
- **Connectors** (MCP streamable-http): **SQL** (Postgres, read-only via validator + `READ ONLY` txn + read-only role, `:8090`); **REST** (OpenAPI, GET-only unless `ALLOW_MUTATIONS`, SSRF-screened, `:8091`); **SSH** (allowlisted commands, **fail-closed host keys**, `:8092`); **SOAP** (WSDL 1.1, operation allowlist, SSRF-screened, `:8093`); **browser** (Playwright, domain allowlist, read/navigate only, `:8094`). Secrets via env.
  - **Deployment tier per connector (a gotcha, not a detail):** Compose wires **SQL + REST** into `AGENTOS_MCP_SERVERS` by default (a hardcoded literal — no `${}` override); **SOAP + browser** start only under `--profile connectors` and must be appended to `AGENTOS_MCP_SERVERS` **by hand**; **SSH has no Compose or Helm service** at all (run it standalone). Helm composes the list from its templates and supports `extraMcpServers`. Call this out explicitly in `docs/configuration.md` and `docs/deployment.md`.
- **Console** (TypeScript/React SPA, `:3000`, nginx-served): 12 RBAC-gated pages — Overview, Keys, Audit, Playground, Documents, Improve, Multiverse, Operators, Orgs, Users, Secrets, Provisioning (the last four gated by `can(role, …)`); admin-key or OIDC-SSO sign-in with `whoami`-driven role-aware UI; same-origin `/api/gateway/*` + `/api/runtime/*` only — the runtime auth token is injected **server-side by nginx** and never reaches the browser; live/poll freshness indicators; ⌘K palette; CSV/JSON export.
- **The load-bearing invariant:** the runtime never holds credentials; every action is authorized, attributed, and recorded; all external integrations are opt-in and off by default. **Configured for local Ollama** (`AGENTOS_MODEL`, `AGENTOS_EMBED_MODEL`, `AGENTOS_JUDGE_MODEL` + `AGENTOS_OLLAMA_BASE_URL`), the stack makes no third-party calls and the shipped `council.yaml` runs a **five-model council at $0**. Note the shipped defaults point at Anthropic, the frontier council members ship `enabled: false`, and `AGENTOS_COUNCIL_HEARTBEAT_S=0` — so the council serves its API but never acts on its own, and a literal `docker compose up` is *not* the $0 path without those overrides.

## 6. Work-stream 1 — GitHub doc overhaul (content)

### README.md (rewritten concept-first)

1. **Hero** — one-line pitch + "nothing runs unauthorized, unattributed, or unrecorded"; a real architecture diagram (ASCII or linked image); links to the landing page and the in-console Docs tab. **Remove the two shields.io CI badges** — they cannot resolve while the repo is private and unpushed; note re-adding them as a roadmap item.
2. **What it is / who it's for** — 3–4 value props; the credential invariant stated once.
3. **Concepts & glossary** — short, links `docs/concepts.md`.
4. **Quickstart** — Compose path **and** the local-Ollama $0/air-gapped path, **spelling out the required overrides** (`AGENTOS_MODEL`, `AGENTOS_EMBED_MODEL`, `AGENTOS_JUDGE_MODEL`, `AGENTOS_OLLAMA_BASE_URL`) since the shipped defaults point at Anthropic; the `AGENTOS_RUNTIME_AUTH_TOKEN` gotcha; console at `:3000`; `make smoke`.
5. **Architecture** — links `docs/architecture.md`.
6. **Capability tour** — follows the **§5 taxonomy** (not the landing grid, whose legends are Engine / Observability / Evaluation / Deployment / Sandbox / Fleet and do not map onto it). Each group names an **exact link target**; where a group has no dedicated `docs/` file, point at a heading anchor in an existing one (`docs/architecture.md`, `docs/api.md`, `docs/console.md`, `connectors/*/README.md`, `deploy/helm/agentos/README.md`). **Phase framing removed.**
7. **Console** — short section on the operator surface, linking `docs/console.md`.
8. **Deploy** — Compose → Helm → CI/eval-gate; links `docs/deployment.md`, `docs/operations.md` and `deploy/helm/agentos/README.md`.
9. **Security** — short posture; links `SECURITY.md`.
10. **Repository layout** — **rewritten**: 5 connectors, Helm present, add `landing/`, `scripts/`, `docs/`.
11. **Development / testing / contributing** — `make` targets, TDD/smoke conventions; state explicitly that `make test` = go + python + console and **excludes `make test-rust`**.
12. **Roadmap** (forward-looking only) + **Docs index** + license.

### New `docs/` files

- `docs/concepts.md` — glossary: gateway/runtime/connectors/sandbox/console; token types (`agos-` virtual key, `agu-` user token, `whk-` webhook token, SCIM token, root admin key); org/role/RBAC; budget/hold; MCP; skill; operator; council; proposal; eval/eval-gate; guardrail; secret backend.
- `docs/architecture.md` — de-internalized from `project-context.md §2–3`: diagram, services+ports table, request/data flow (the real `/v1/*` pipeline per §5, with RBAC shown as the admin plane), persistence, the credential invariant.
- `docs/configuration.md` — prose config reference sourced from `deploy/.env.example` **plus** `deploy/compose.yaml`, `deploy/helm/agentos/values.yaml`, `gateway/cmd/gateway/main.go`, `runtime/src/agentos_runtime/config.py`, `sandbox/src/`, and each `connectors/*/README.md` (≈30 live `AGENTOS_*` vars are absent from `.env.example`, including `AGENTOS_MCP_SERVERS`, `AGENTOS_COUNCIL_*`, `AGENTOS_SANDBOX_*`, `AGENTOS_CORS_ORIGINS`, `AGENTOS_OTEL_ENDPOINT`, and all per-connector tuning). Grouped by subsystem, phase tags dropped. Carries the SSH host-key correction and the connector-wiring gotcha (§5).
- `docs/api.md` — consolidated gateway + runtime endpoint reference (method, path, purpose, auth) **plus a tool catalog**: tool name, owning connector/service, port, arguments, safety constraint (read-only / allowlist / domain-gate) — covering `query` / `list_tables` / `describe_table`, REST + SOAP `list_operations` and per-operation tools, `run_command` / `list_allowed`, the browser tools, `run_python`, `use_skill` — plus the sandbox `POST /execute` contract. ("What can an agent actually do" currently has no doc home.)
- `docs/console.md` — page-by-page purpose and backing endpoint, role gating, the two sign-in paths, the server-side-only runtime-token boundary, and the same-origin/air-gap property.
- `docs/deployment.md` — Compose (laptop) → Helm (cluster) → CI + offline eval-gate; overlays (HITL/OTel/Langfuse/auth-mocks/openclaw); the connector deployment tiers from §5.
- `docs/operations.md` — verification & demo fixtures: a table of every `make smoke` … `smoke9` target and what it proves; the demo fixtures (`deploy/initdb/01-legacy-erp.sql`, `deploy/demo-crm/`) and which connector each backs; the CI mocks (`deploy/ci/mock-model.py`, `mock-oidc.py`, `mock-vault.py` + `compose.auth-mocks.yaml`); `deploy/helm/test-render.sh`; a link to `deploy/SANDBOX_EGRESS_VERIFY.md`. Linked from the Deploy section of the taxonomy.
- `SECURITY.md` (repo root, GitHub convention) — **living** posture: credential invariant, the fail-open/fail-closed matrix stated **per failure mode** (budget / rate-limit / guardrail fail open on backend error and fail closed on verdict; OIDC `email_verified`, SSH host keys, runtime auth token, council write-gating fail closed), hardening summary, how to report; links the dated assessment.
- `CHANGELOG.md` (repo root) — the phase-completion log moved out of README.
- Keep `docs/interop/openclaw.md` (publishable as-is).

**Canonical ownership** (the overhaul creates three duplicate pairs — name the owner in a one-line "canonical source:" header on each duplicated file):

- architecture ⇒ `docs/architecture.md`; `project-context.md` keeps only internal/dev-host material and links out.
- observability/Langfuse ⇒ `docs/deployment.md`; `deploy/LANGFUSE_DOCS_SNIPPET.md` and the README section reduce to pointers.
- OpenClaw ⇒ `docs/interop/openclaw.md`; `deploy/openclaw/README.md` reduces to the overlay command.

### Accuracy fixes (must land)

- Rewrite the README repository-layout table.
- Correct `README.md:220` — `--profile connectors` does **not** make SOAP/browser reachable to the agent (see the §5 connector tiers).
- Correct `docs/project-context.md` in place and mark it an internal briefing (the public architecture doc is `docs/architecture.md`):
  - **§8 "Known open backlog"** and **§13 "Honest gaps"** — re-verify every listed item against current source; most have shipped (verified shipped: atomic budget hold, `http.MaxBytesReader`, SQL `statement_timeout`, HTTP server timeouts + graceful shutdown, non-root runtime image, audit retention/prune, gateway provider retry, per-run cycle cap `AGENTOS_AUTONOMY_MAX_CYCLES`, context-trimming hook, always-on autonomy via Operators). Keep only what re-verification still confirms open.
  - **§10** — Multiverse shipped; retitle away from "designed, not implemented". Note the file never mentions Operators at all; the stale autonomy claim is §13's "No always-on autonomy", not §10.
  - **§9 Delivery history** — extend with the Multiverse/Operators entry (`make smoke8` / `smoke9`).
  - **Constraint:** **no claim from §8 or §13 may be carried into the new root `SECURITY.md` or any new `docs/` file without independent re-verification against current source.** (Item-by-item checklist belongs in Plan W1.)
- Correct `connectors/ssh/README.md` host-key section to match the fail-closed code, and the matching stale comment at `deploy/.env.example:65` (`# unset = insecure host-key (warns at startup)`).
- Rewrite `console/README.md` (currently documents 5 of 12 pages, titled "Phase 2").
- Drop phase labels from `sandbox/README.md` and `connectors/rest/README.md`.

### W1 verification

No automated tests (prose). Tickable checklist:

- Every relative link resolves.
- Every `make` target cited exists in the `Makefile`.
- Every `AGENTOS_*` literal found by grep over the sources named for `docs/configuration.md` appears in that file.
- Every env var cited anywhere in the docs exists in those sources.
- Section names **and order** match §5 verbatim — every §5 section name appears as a README heading or a `docs/` file title. (Checkable without W2; matching W1 is W2's job — see §7.6.)
- No remaining private-repo-badge breakage (badges removed).
- Manual read-through.

## 7. Work-stream 2 — In-console Docs tab (visual, animated handbook)

Data-driven, no new dependency, fully on the Instrument system. Adding a tab is a 3-file router touch + content + visuals + a page stylesheet (confirmed by the console scan: routes are one `ROUTES` array in `src/App.tsx`; Sidebar and ⌘K palette are driven off it; no per-route edits needed there).

Files: `src/pages/Docs.tsx`, `src/pages/docs/Docs.css`, `src/pages/docs/content.ts`, `src/pages/docs/DocBlocks.tsx`, `src/pages/docs/visuals/keys.ts`, `src/pages/docs/visuals/*.tsx` (+ registry), tests; edits to `src/ui/icons.tsx` and `src/App.tsx`.

### 7.1 Typed content model — `src/pages/docs/content.ts`

```ts
type DocBlock =
  | { kind: "prose"; text: string }
  | { kind: "list"; items: string[] }
  | { kind: "code"; code: string; lang?: string }
  | { kind: "keyvals"; caption?: string; rows: { k: string; v: string }[] } // config / endpoints (mono)
  | { kind: "note"; text: string }                                           // neutral .notice, MONOCHROME
  | { kind: "diagram"; diagram: DiagramKey; caption?: string };              // an animated visual

interface DocSection { id: string; title: string; icon: IconName; blocks: DocBlock[] }
export const DOC_SECTIONS: DocSection[]; // ordered per the taxonomy (§5)
```

`lang?` is **metadata only** — rendered as a mono label on the code block. No syntax highlighting, no new dependency (§3).

Sections mirror the full §5 taxonomy: Overview, Concepts & Glossary, Architecture, Gateway, Runtime, Sandbox, Connectors, Console, Quickstart, Configuration, Deploy, Security → "all related information." Content is authored from §5 and kept coherent with the GitHub docs. **Console** is a short orientation section (what each page is for, why some are hidden for your role, where freshness indicators and ⌘K live) — **no new visual**; the v1 visual set stays at four.

### 7.2 Visualizations (the "visualized and animated" requirement) — `src/pages/docs/visuals/`

Diagram keys are declared in a **pure** module `src/pages/docs/visuals/keys.ts`:

```ts
export const DIAGRAM_KEYS = ["architecture", "governanceChain", "requestLifecycle", "councilFanout"] as const;
export type DiagramKey = (typeof DIAGRAM_KEYS)[number];
```

The `.tsx` registry is typed `Record<DiagramKey, (props) => JSX.Element>` (same compile-enforced pattern as `GLYPHS`), so every `diagram` block references a real visual. v1 set:

- **`architecture`** → the **Architecture** section. The four planes (Gateway ⋄ Runtime ⋄ Sandbox ⋄ Connectors) plus the console, with the "models only via gateway / runtime holds no keys" wiring; **animated reveal** (staggered `fadeRise`), plus a looping request-pulse that traces client → gateway → runtime → connector/sandbox → back.
- **`governanceChain`** → the **Gateway** section. An **animated stage conveyor** over the real proxy pipeline: `Auth (agos- key) → Rate limit → Budget hold → Guardrail (when enabled) → Upstream/Council → Audit`. A React/`motion.ts` port of the landing conveyor's *motion and determinism pattern* — **not its stage list**, which is marketing shorthand (`Auth, RBAC, Budget, Rate, Audit`) and factually wrong for `/v1/*`. Tokens advance stage-by-stage on a **deterministic scripted sequence** (fixed verdict script — mostly `ok`, some `hold`, some `deny`; no `Math.random`), each stage lighting the appropriate **state hue**. This is the sanctioned chroma use. Because it duplicates the live `Chain` the shell pins to the topbar on **every** route including `/docs`, it carries the §4 **illustration marker** and must not be styled to read as the topbar instrument; its stage names and order must match `CHAIN_STAGES` in `src/lib/chain.ts` (mechanism — shared import or a covered test — is a Plan W2 decision). How the **admin plane** is depicted alongside it is also a Plan W2 decision — and it must depict the real split, not one RBAC-gated group: role checks (RBAC, via `agu-` tokens and the root admin key) gate `/admin/*` only; `/scim/v2/*` is gated separately by `scimAuth`, a static shared-secret bearer compare with no role evaluation (`gateway/internal/server/scim.go:19-28`); `/auth/oidc/*` is registered with no auth wrapper at all — it is the public login/callback flow (`gateway/internal/server/server.go:274-276`). None of this must appear as a stage of the model-call path.
- **`requestLifecycle`** → the **Overview** section. A **static annotated** client → gateway → runtime → tool → back path with callouts naming the checks each hop clears (distinct from `governanceChain`, which animates the gate stages themselves). Notes that the budget hold fails open only on a store/DB error and, unlike the guardrail's `guardrail_error` audit kind, that admission gets **no** audit entry; the guardrail stage exists only when `AGENTOS_GUARDRAIL_MODE != off`; audit records outcomes as one of seven kinds (`chat`, `embeddings`, `guardrail_flag`, `guardrail_block`, `guardrail_error`, `rate_limited`, `secret_reload`) and among denials only rate-limit and guardrail events are audited — a `401` auth failure, a `400` malformed request, and a `402` budget exhaustion are not; and `/v1/embeddings` runs the same chain minus the guardrail.
- **`councilFanout`** → the **Runtime** section. One objective → N model-bound members → judge → verdict + dissent (static-structured, gentle animated reveal).

Rules for every visual: pure inline SVG + framer-motion; **deterministic** (scripted timelines; no `Math.random`); **reduced motion → a static representative still, no intervals** (gate with `useReducedMotion()` and clear/never-start timers); air-gapped; hues limited to the four state tokens + graphite/ink. No PRNG module is required by the v1 set — `governanceChain` is a fixed script. If a later visual genuinely needs seeded variation, add a small pure `src/lib/prng.ts` (a fresh port of the landing's inline `mulberry32`, which is not importable) with a determinism test; it is **conditional, not a prerequisite**.

### 7.3 Page — `src/pages/Docs.tsx`

Two-pane: a sticky **section rail** (left; the `DOC_SECTIONS` list with icons and an active pill) + the rendered active section (right), composed from `PageHead` + `Panel`/`PanelHead`.

- **Rail pill:** uses its **own** `layoutId` (e.g. `docs-rail-pill`) and its own pill class in `Docs.css` — **never** the Sidebar's `layoutId="nav-pill"`, which is a global framer-motion namespace and would make the pill fly between sidebar and rail (both are mounted simultaneously; cf. `ui/Tabs.tsx`'s per-instance `${id}-pill`). It must replicate the Sidebar's reduced-motion branch (plain `<span>`, no `layoutId`).
- **Section selection:** local state, seeded once on mount from `window.location.search` (`?s=<id>`, falling back to the first section when absent/unknown). Section changes update the query with `window.history.replaceState(null, "", "/docs?s=" + id)` **directly — never via the router's `navigate`**, which stores its whole argument as the route key and is matched by exact equality (`App.tsx:150`, `Sidebar.tsx:96`), so a query-bearing path falls through to Overview and drops the nav pill. Inbound `/docs?s=…` links work unchanged because the router reads `window.location.pathname`. **No change to `usePath`**; do **not** adopt a `path.split("?")[0]` router change — it contradicts the 3-file router touch.
- **Network:** the page issues no fetches and reads no `adminKey`. Inherits page transition + reduced-motion + theming via `Panel`.

### 7.4 Block renderer — `src/pages/docs/DocBlocks.tsx`

A pure block→markup mapper using existing classes (`.panel-body`, `.mono`, `.notice`) and, for `diagram`, the visual registry. Exhaustive `switch` over `DocBlock["kind"]` (compile-checked via `never`).

`src/pages/docs/Docs.css` carries the two-pane/rail layout rules **and an explicit code-block treatment** — the console has no shared code styling (`code, pre, .mono` set only font-family/size), so a bare `<pre>` would be unstyled and overflow the panel. Mirror the existing `.prompt-text` pattern; existing tokens only, no new color tokens. Exact values are Plan W2's call.

### 7.5 Icon + route

- `src/ui/icons.tsx` — add `"docs"` to the `IconName` union + a new **house-grid** glyph (open-book / reference-sheet: 16×16 viewBox, artwork in 2..14, `strokeWidth 1.25`, square caps, mitre joins, `fill=none`/`stroke=currentColor`; visually distinct from the existing `documents` file-sheets). Also export a runtime `ICON_NAMES: readonly IconName[]` — `GLYPHS` is module-private, so the §7.6 icon test cannot be written without it.
- `src/pages/Docs.tsx` — export `function Docs(): React.JSX.Element` (**zero parameters**; a declared-and-unused `props` fails `tsc` under `noUnusedParameters` and breaks `npm run build`).
- `src/App.tsx` — import `Docs`; add `{ path: "/docs", label: "Docs", icon: "docs", Component: Docs }` to `ROUTES`, positioned **last among the always-visible routes, before the `visible`-gated admin block**, so sidebar and ⌘K ordering are deliberate. **No `visible` predicate** → available to every role; Sidebar + ⌘K palette pick it up automatically. No in-page guard needed.

### 7.6 W2 tests (node-env `.test.ts`) + verification

Vitest runs `environment: "node"` with no jsdom, and no existing test imports a `.tsx`. **No new `.test.ts` may import a `.tsx` module** — hence the pure `keys.ts` (§7.2) and the `ICON_NAMES` export (§7.5). These runtime tests are belt-and-braces over the compile-time `Record<…>` checks.

- Content-model invariants: unique section ids; section titles and order match §5 verbatim; every section `icon` ∈ `ICON_NAMES`; every `diagram` key ∈ `DIAGRAM_KEYS`; no empty blocks/sections.
- Visual placement: the Architecture, Gateway, Overview and Runtime sections each contain a `diagram` block (nothing else currently binds visuals to sections).
- Illustration marker: every `diagram` whose visual uses state hues carries a non-empty illustration marker.
- Pure `blockToText(block)` helper exercising **every** block kind (drives the invariant tests and gives a plain-text index for a future search).
- Page, visuals, and renderer verified by `tsc --noEmit` + full vitest suite + `vite build` + **manual in-console check**:
  - nav shows Docs; every section renders; animation runs; reduced-motion shows the static still with no running intervals; theming correct in both modes.
  - deep link — load `/docs?s=gateway` directly, then click through rail sections: the Docs pill stays active and the URL updates without a route change.
  - network — the Docs page issues no fetches and reads no `adminKey`; the only traffic while it is open is the **pre-existing shell polling** (topbar `Chain` → `/admin/audit`, sidebar live-dot → `/admin/usage`, both 5s); there are **no third-party/CDN/font/image requests**.

## 8. Decomposition & sequencing

**One spec, two plans**, each executed via subagent-driven development (like Track A):

- **Plan W1 — GitHub doc overhaul** (README + `docs/` set + accuracy fixes). Content; reviewed as prose against the §6 checklist.
- **Plan W2 — Console visual Docs tab** (diagram keys → content model → visuals → block renderer + CSS → page → icon/`ICON_NAMES` → route → tests). Feature; TDD where pure, `tsc` + build + manual for React/animation. `src/lib/prng.ts` only if a visual actually needs seeded variation (§7.2) — not a prerequisite task.

Build order: **W1 → W2** (W1 nails the canonical content W2 mirrors). Independent enough to swap if the tab is wanted first.

Each plan → its own SDD run → local merge to `main` (fast-forward, branch + workspace deleted) → **not pushed**.

## 9. Risks / open questions

- **Content drift** between GitHub docs and the console handbook. Mitigation: single taxonomy (§5) + author W2 content directly from the same capability facts; accept intentional duplication (different media/audiences). For the three in-repo duplicate pairs the overhaul itself creates, ownership is named in §6.
- **Stale-source contamination.** `project-context.md` §8/§13 are known-wrong; the §6 no-copy-without-re-verification constraint exists to stop that staleness becoming public in `SECURITY.md` or `docs/*`.
- **Animation scope creep.** Mitigation: fixed v1 visual set (§7.2, four visuals, each bound to one section); everything else is prose/keyvals. Reuse the landing's proven deterministic/reduced-motion pattern rather than inventing new machinery.
- **Chroma-invariant misreads.** Mitigation: spec states explicitly that only the four state hues appear in visuals (as machine state), doc notes are monochrome, and any visual mirroring a live instrument carries an illustration marker (§4) — call this out to reviewers.
