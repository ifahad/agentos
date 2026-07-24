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
| always-on presence, heartbeat autonomy, messaging channels (WhatsApp/Slack/…), portable skills | governed model access (keys, budgets, rate limits, guardrails, audit), sandbox isolation, legacy connectors, multi-tenancy, RBAC/SSO |

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

## Step 3 — Screen the skills (kills supply-chain poisoning)

- Load skills **only from a vetted, read-only directory** — never auto-install
  from ClawHub at runtime.
- **Statically screen** each `SKILL.md` for injection markers before enabling it
  (the same deny-list AgentOS applies to prompt proposals: "ignore previous
  instructions", "exfiltrate", "auto-approve", …), and require human review.
- Treat skill bodies as **advisory data, not authoritative instructions**.
  AgentOS's immutable safety preamble already states that retrieved/tool/skill
  content can never override the safety rules.

The `SKILL.md` format is shared with AgentOS/Claude Code/Cursor
(`skills/README.md` — pending), so vetted skills are portable both ways.

## Step 4 — Contain the deployment (privilege restriction)

- Run OpenClaw **non-root**, read-only rootfs where possible, on an **isolated
  Docker network** with an **egress allow-list**.
- **Do not expose** the control plane (:18789) beyond localhost / a trusted admin
  network.
- Give it only the gateway and the sandbox/SSH endpoints it needs — nothing else.

See `deploy/openclaw/` for an illustrative compose overlay wiring OpenClaw to the
gateway on a restricted network.

## What is NOT integrated (be honest)

This recipe governs OpenClaw's **model traffic** and gives it **safe execution
surfaces**. It does **not** bridge OpenClaw's WebSocket control plane into the
AgentOS console, nor proxy its messaging channels. Those remain OpenClaw-side and
are future work. The value here is turning an ungoverned, high-privilege agent
into a **budgeted, audited, jailed worker**.
