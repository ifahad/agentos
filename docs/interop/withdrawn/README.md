# Withdrawn: OpenClaw interop

**Status: withdrawn 2026-07-24. Do not implement. Do not run the compose file here.**

AgentOS will not integrate OpenClaw. The material in this directory is kept for
its risk analysis and as the record of the decision, not as guidance to follow.

## Why it was withdrawn

The integration was designed defensively — model traffic through the gateway,
execution jailed to the sandbox or hardened SSH, skills screened from a
read-only vetted directory, non-root and unexposed. That recipe was sound as far
as it went. It was withdrawn anyway, because the most serious risks are
*structural* and cannot be mitigated by configuration:

- **Supply chain.** OpenClaw's value depends on its skill ecosystem, and that
  ecosystem is compromised: roughly 36% of ClawHub skills were found carrying
  prompt injection, with 341+ outright malicious skills. Our mitigation was
  "never fetch at runtime, vet by hand" — which discards the reason to adopt it
  and leaves a standing obligation to re-vet forever.
- **Governance by configuration, not construction.** OpenClaw reaches the
  gateway because it is *pointed* at it. A changed setting, an upgrade default,
  or an operator error routes model traffic straight to a provider, and the
  audit log simply shows nothing. Nothing in the design makes that impossible.
- **A second control plane.** Its WebSocket control port is authority over the
  agent that lives outside our RBAC, our audit trail, and our approval gate.
  Not exposing it is a deployment convention, and conventions decay.

## What replaces it

The same capabilities, built first-party — see the Operators work in
`docs/superpowers/plans/`. The distinction that matters is structural: the
AgentOS runtime holds **no provider credentials at all**, so a native autonomy
engine cannot route around the gateway even if misconfigured. Governance stops
being a setting and becomes a property of the architecture.

| Risk | Under OpenClaw | Under Operators |
|------|----------------|-----------------|
| Skill supply chain | mitigated by policy (vetted dir, no auto-fetch) | eliminated — no registry exists; a skill is a reviewed commit |
| Bypassing the gateway | mitigated by configuration | impossible — the runtime has no provider credentials |
| Foreign control plane | mitigated by not exposing a port | eliminated — no second control plane |
| Credential leak | mitigated by a scoped key | eliminated — no credentials to leak |
| Unsandboxed execution | routed to our sandbox | already ours |
| Autonomous misoperation | bounded by max_cycles + HITL | bounded identically |

Four items move from "mitigated" to "does not exist". The last row is the
irreducible risk of autonomy itself, and it is bounded the same way either way.

## Honest note

Withdrawing this removes specific, named risk classes. It does not make
autonomous operation risk-free, and nothing in the replacement should be read as
claiming otherwise. A model acting within its permissions can still act badly;
that is what the governance chain, the approval gate, and cycle caps are for.
