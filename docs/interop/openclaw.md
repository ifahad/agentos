# Running OpenClaw securely under AgentOS

[OpenClaw](https://openclaw.ai) is an open-source, always-on autonomous agent
(heartbeat loop, portable `SKILL.md` skills, host shell/file/browser/Docker
access, a WebSocket control plane on :18789, and the ClawHub skill marketplace).
It's powerful, but its **default trust model is dangerous**. This guide shows how
to operate it as a **governed, jailed worker under AgentOS** — the "secure shell"
around OpenClaw.

> This is an operator recipe, not an OpenClaw distribution. The `openclaw.json`
> and compose snippets are **illustrative** — check your OpenClaw version's own
> provider/config docs and adapt. Nothing here is auto-installed by AgentOS.

## Why you should not run OpenClaw ungoverned

Documented, cited problems:

- **Poisoned skill supply chain.** Snyk found **~36% of ClawHub skills contain
  prompt injection**; researchers found **341+ skills stealing API keys /
  installing malware** ([The Hacker News, Feb 2026](https://thehackernews.com/2026/02/researchers-find-341-malicious-clawhub.html)),
  with **>1,184 malicious skills** and roughly **1 in 12 packages** malicious as
  the registry passed ~13,700 skills
  ([Unit 42](https://unit42.paloaltonetworks.com/openclaw-ai-supply-chain-risk/),
  [HKCERT](https://www.hkcert.org/blog/openclaw-s-rapid-adoption-exposes-skills-supply-chain-and-fake-installer-risks-in-a-high-privilege-ai-agent-platform)).
- **Unsandboxed execution** with broad system permissions — a compromised skill
  inherits shell/file/network access.
- **Prompt injection & data leakage**, **autonomous misoperation**, **credential
  leakage**, and **token-billing explosion**
  ([Giskard](https://www.giskard.ai/knowledge/openclaw-security-vulnerabilities-include-data-leakage-and-prompt-injection-risks),
  [Nebius hardening guide](https://nebius.com/blog/posts/openclaw-security)).

The community's own defense list — *privilege restriction, supply-chain
screening, sandbox isolation, confirmation gating, credential protection, cost
capping, upgrade verification* — is exactly the AgentOS control set.

## The division of labor

| OpenClaw provides | AgentOS provides |
|---|---|
| always-on presence, heartbeat autonomy | governed model access (keys, budgets, rate limits, guardrails, audit), sandbox isolation, legacy connectors, multi-tenancy, RBAC/SSO, **and the network that makes all of it unavoidable** |

Two OpenClaw features are deliberately **not** taken: **skills** (Step 3 — the
registry is the single largest attack surface) and **messaging channels** (an
inbound instruction path into an autonomous agent). What remains is the part
worth having: an always-on loop, running under someone else's governance.

Point OpenClaw's **model traffic** at the AgentOS gateway and route its
**execution** through AgentOS isolation, and OpenClaw's biggest risk classes are
covered by controls it doesn't have on its own.

## Step 1 — Governed model access (kills credential-leak + token-blowup)

The AgentOS gateway is a governed OpenAI-compatible endpoint. Give the fleet an
org with a **budget and rate limit**, then a scoped virtual key:

```bash
G=http://localhost:8080; ADMIN="Authorization: Bearer $AGENTOS_ADMIN_KEY"

# an org for the fleet, capped at $50/mo and 60 rpm
OID=$(curl -fsS -X POST $G/admin/orgs -H "$ADMIN" -H 'Content-Type: application/json' \
  -d '{"name":"openclaw-fleet","monthly_budget_usd":50,"rate_limit_rpm":60}' \
  | python3 -c "import json,sys;print(json.load(sys.stdin)['id'])")

# a per-worker key inside that org
curl -fsS -X POST $G/admin/keys -H "$ADMIN" -H 'Content-Type: application/json' \
  -d "{\"name\":\"openclaw-worker-1\",\"monthly_budget_usd\":10,\"org_id\":\"$OID\"}"
# -> {"key":"agos-…"}   (shown once)
```

Point OpenClaw at the gateway as its OpenAI-compatible provider (illustrative):

```jsonc
// openclaw.json (illustrative — match your OpenClaw version's provider schema)
{
  "provider": {
    "type": "openai-compatible",
    "base_url": "http://gateway:8080/v1",
    "api_key": "agos-…",              // the scoped virtual key above
    "model": "anthropic/claude-sonnet-5" // or openai/… or ollama/…
  }
}
```

**Verified** — the exact call OpenClaw makes is now budgeted, rate-limited,
guardrail-screened, and audited, attributed to the key/org:

```
$ curl -s -X POST $G/v1/chat/completions -H "Authorization: Bearer agos-…" \
    -d '{"model":"ollama/qwen3.6:latest","messages":[{"role":"user","content":"Reply: governed by AgentOS"}]}'
reply: governed by AgentOS
usage: {"prompt_tokens":18,"completion_tokens":223,"total_tokens":241}

$ curl -s "$G/admin/usage" -H "$ADMIN"      # attribution
{"name":"openclaw-worker-1","requests":1,"input_tokens":18,"output_tokens":223,"spend_usd":0}
```

Turn on `AGENTOS_GUARDRAILS_MODE=block` (or `model`) and every OpenClaw prompt is
injection-screened at the gateway before it reaches a provider. When the org's
budget or rpm is exceeded, the gateway returns `402`/`429` — OpenClaw cannot run
away with your spend.

Point `OPENCLAW_MODEL` at `council/multiverse` and the OpenClaw worker thinks
through the whole council — several models, one synthesized verdict with
dissent — with no OpenClaw-side change. The same budget, rate limit, and audit
apply, and the recursion guard prevents the council calling itself.

## Step 2 — Jail the execution (kills unsandboxed RCE)

OpenClaw's shell/skill execution must **never touch the host**. Two safe surfaces:

- **Generated code → the egress-less Rust sandbox.** It runs read-only, with all
  capabilities dropped, no outbound network route, and CPU/memory/file rlimits
  (see `sandbox/README.md`). Route OpenClaw's code execution here instead of a
  host shell.
- **Commands on a legacy box → the hardened SSH connector.** It denies
  exec-capable binaries (even if allow-listed), blocks shell chaining/redirection,
  and requires `known_hosts` (see `connectors/ssh/`). Never give OpenClaw a raw
  host shell.

Gate high-risk actions behind **human confirmation** — OpenClaw's own exec-approval
policy, and AgentOS's HITL approval on the agent side.

## Step 3 — Turn skills off (kills supply-chain poisoning outright)

**Run with skills disabled.** Not "vetted", not "screened" — off:

```
OPENCLAW_SKILLS_DIR=""
OPENCLAW_DISABLE_SKILLS=true
OPENCLAW_DISABLE_REGISTRY=true
```

The earlier version of this guide recommended a vetted read-only directory plus
static injection screening. That is a reasonable control and it is not the one we
use, for two reasons. It relies on a human performing an unbounded review
obligation correctly, forever, against an adversary that only has to win once.
And screening for injection markers is a deny-list on natural language, which
cannot be made complete — a skill that passes the check is not thereby safe.
Against a registry where roughly **1 in 12 packages is malicious**, the only
control with a provable outcome is having nothing to load.

Disabling skills costs the portable-skill capability. If you later need it, treat
re-enabling as its own reviewed change with its own threat model, not as flipping
a flag — and note that AgentOS's own tools (SQL, RAG, sandbox, connectors) reach
the agent through the gateway and MCP already, without touching ClawHub.

If a skill body ever does reach the model, AgentOS's immutable safety preamble
already frames retrieved/tool/skill content as **untrusted data that can never
override the safety rules**. That is a backstop, not a reason to re-enable.

## Step 4 — Contain the deployment (this is the load-bearing control)

Put the worker on an **`internal: true` Docker network**. This is the control
that makes everything above hold, because it is the difference between OpenClaw
being *configured* to use the gateway and OpenClaw being *unable to reach
anything else*:

```yaml
networks:
  governed-net:
    internal: true      # no route off-host, for anything attached
```

Attach the gateway to both `default` and `governed-net`; attach OpenClaw to
`governed-net` (plus `sandbox-net` for execution) and **never to `default`**. The
gateway becomes the sole crossing point between the worker and the outside world.

**Verified, not assumed** — a container on an internal network cannot resolve or
reach a provider at all:

```
$ docker run --rm --network <internal-net> alpine \
    sh -c 'wget -T5 -O- https://api.openai.com'
wget: bad address 'api.openai.com'

$ ... nslookup api.openai.com
** server can't find api.openai.com: SERVFAIL
```

Set `OPENAI_BASE_URL` to a provider by mistake, or ship a compromised build that
tries to phone home, and the connection simply fails. Governance stops depending
on configuration being right.

The rest of the containment:

- **Non-root, read-only rootfs, all caps dropped, `no-new-privileges`**, with
  memory and pid limits.
- **The control plane (:18789) is never published.** On an internal network
  Docker refuses host port publishing outright, so this is enforced rather than
  merely omitted from the compose file.
- Pin the image **by digest**, not by tag — an unverified upgrade is a supply
  chain of its own.

See `deploy/openclaw/compose.openclaw.yaml` for the wired overlay.

## Residual risk (what this does not fix)

Be clear about what is left after all four steps:

- **A model acting badly within its permissions.** The worker can still issue
  legitimate-looking tool calls that are wrong or harmful. Budgets, rate limits,
  `max_cycles`, and the HITL approval gate bound the blast radius; they do not
  make the agent correct.
- **Prompt injection via content it reads.** Screening at the gateway
  (`AGENTOS_GUARDRAILS_MODE=block`) and the untrusted-data framing reduce this;
  neither eliminates it.
- **OpenClaw's own code.** Pinning by digest fixes *which* build you run, not
  whether that build is sound. It remains third-party software with host-agent
  ambitions, contained rather than trusted.
- **Everything inside `governed-net`.** The isolation stops egress to the
  internet, not lateral movement to the gateway and sandbox — which is precisely
  why both of those are hardened and audited.

## What is NOT integrated (be honest)

This recipe governs OpenClaw's **model traffic** and gives it **safe execution
surfaces**. It does **not** bridge OpenClaw's WebSocket control plane into the
AgentOS console, nor proxy its messaging channels.

Messaging in particular is deferred **on purpose**, not merely unfinished: an
inbound channel is a path for anyone who can post in it to instruct an autonomous
agent. If notifications are needed, prefer **outbound-only** delivery through the
SSRF-screened egress — you get told what happened, and nobody gets to issue
instructions over Slack.

The value here is turning an ungoverned, high-privilege agent into a **budgeted,
audited, jailed worker that cannot reach the internet**.
