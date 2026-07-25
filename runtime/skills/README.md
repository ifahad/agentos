# AgentOS skills (`SKILL.md`)

A **skill** is a reviewed, in-repo instruction sheet the agent can pull on demand
via the `use_skill(name)` tool. The format is a single Markdown file with YAML
frontmatter:

```markdown
---
name: erp-analysis
description: How to answer questions about the legacy ERP database accurately.
when_to_use: Any question about customers, orders, invoices, or revenue.
---

# The instructions

Markdown body the agent follows when it calls use_skill("erp-analysis").
```

**Frontmatter fields**

| field | required | meaning |
|-------|----------|---------|
| `name` | yes | unique identifier; how the agent names the skill in `use_skill` |
| `description` | no | one line; shown in the agent's "Available skills" list |
| `when_to_use` | no | a hint about when the skill applies |

The body below the second `---` is free-form Markdown.

## How skills load — and why only from here

Skills are loaded **only** from a directory baked into the runtime image
(`AGENTOS_SKILLS_DIR`, default this `skills/` directory), discovered as
`*/SKILL.md`. They are **never fetched at runtime and never from a public
registry.**

This is deliberate. A skill is instructions an autonomous agent will follow, so
it is code, and it is treated like code: reviewed in a pull request, versioned in
the repo, and shipped read-only in the image. Each loaded skill records a sha256
for provenance (surfaced by `GET /skills`). This is the direct lesson of the
OpenClaw / ClawHub supply chain, where roughly a third of published skills
carried prompt injection — the only control with a provable outcome is having
nothing untrusted to load.

## Portability

The `SKILL.md` format is intentionally compatible with the convention used by
OpenClaw, Claude Code, and Cursor, so a skill vetted here is portable — but
portability is one-directional by policy: skills come **into** this repo through
review, never in from an external source at runtime.
