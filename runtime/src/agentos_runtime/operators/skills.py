"""SKILL.md skills: reviewed, in-repo instructions the agent can pull on demand.

A skill is a Markdown file with YAML frontmatter (name, description, when_to_use)
and an instruction body. Skills are loaded ONLY from a directory baked into the
image — never fetched at runtime, never from a public registry. That is the
whole lesson of the OpenClaw supply-chain problem: a skill is code, so it is
reviewed in-repo, and each loaded skill records a sha256 for provenance.

The agent gets a ``use_skill(name)`` tool that returns a skill's body, and its
system prompt lists what exists — so it knows a skill is available without its
full text bloating every turn.
"""

from __future__ import annotations

import hashlib
import logging
from dataclasses import dataclass
from pathlib import Path

import yaml
from langchain_core.tools import BaseTool, tool

logger = logging.getLogger(__name__)

_FRONTMATTER = "---"


@dataclass(frozen=True)
class Skill:
    """One loaded skill."""

    name: str
    description: str
    when_to_use: str
    body: str
    sha256: str


def _parse_skill(path: Path) -> Skill | None:
    """Parse one SKILL.md, or None (logged) when it is malformed."""
    raw = path.read_text()
    digest = hashlib.sha256(raw.encode("utf-8")).hexdigest()
    if not raw.startswith(_FRONTMATTER):
        logger.warning("skill %s has no frontmatter; skipped", path)
        return None
    _, _, rest = raw.partition(_FRONTMATTER)
    front, sep, body = rest.partition(_FRONTMATTER)
    if not sep:
        logger.warning("skill %s frontmatter is not closed; skipped", path)
        return None
    try:
        meta = yaml.safe_load(front) or {}
    except yaml.YAMLError as exc:
        logger.warning("skill %s frontmatter is not valid YAML: %s; skipped", path, exc)
        return None
    name = str(meta.get("name", "")).strip()
    if not name:
        logger.warning("skill %s has no name; skipped", path)
        return None
    return Skill(
        name=name,
        description=str(meta.get("description", "")).strip(),
        when_to_use=str(meta.get("when_to_use", "")).strip(),
        body=body.strip(),
        sha256=digest,
    )


def load_skills(skills_dir: str | Path) -> dict[str, Skill]:
    """Load every ``*/SKILL.md`` under skills_dir, keyed by name.

    A missing directory yields no skills (not an error): skills are optional. A
    duplicate name keeps the first and logs the collision.
    """
    directory = Path(skills_dir)
    skills: dict[str, Skill] = {}
    if not directory.is_dir():
        return skills
    for path in sorted(directory.glob("*/SKILL.md")):
        skill = _parse_skill(path)
        if skill is None:
            continue
        if skill.name in skills:
            logger.warning("duplicate skill name %r (%s); keeping the first", skill.name, path)
            continue
        skills[skill.name] = skill
        logger.info("loaded skill %r (sha256=%s)", skill.name, skill.sha256[:12])
    return skills


def skills_prompt_section(skills: dict[str, Skill]) -> str:
    """The 'Available skills' block appended to the agent's system prompt.

    Names and one-line descriptions only — the body is fetched on demand via
    use_skill, so listing skills costs a line each, not their full text.
    """
    if not skills:
        return ""
    lines = ["", "AVAILABLE SKILLS (call use_skill(name) to read the full instructions):"]
    for skill in skills.values():
        hint = f" — {skill.description}" if skill.description else ""
        lines.append(f"- {skill.name}{hint}")
    return "\n".join(lines)


def make_use_skill_tool(skills: dict[str, Skill]) -> BaseTool:
    """A tool returning a skill's instruction body by name.

    An unknown name returns an error string listing what exists, rather than
    raising, so the model can recover by picking a real skill.
    """

    @tool
    def use_skill(name: str) -> str:
        """Read the full instructions for a named skill before doing a task."""
        skill = skills.get(name)
        if skill is None:
            available = ", ".join(skills) or "(none)"
            return f"Unknown skill {name!r}. Available skills: {available}."
        return skill.body

    return use_skill
