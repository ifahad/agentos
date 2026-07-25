"""SKILL.md loading, the use_skill tool, and the prompt section."""

from pathlib import Path

from agentos_runtime.operators.skills import (
    load_skills,
    make_use_skill_tool,
    skills_prompt_section,
)

SKILL = """---
name: erp-analysis
description: How to read the ERP.
when_to_use: ERP questions.
---

# Body

Inspect schemas first.
"""


def write_skill(tmp_path, folder, text):
    d = tmp_path / folder
    d.mkdir()
    (d / "SKILL.md").write_text(text)
    return tmp_path


def test_loads_a_valid_skill(tmp_path):
    skills = load_skills(write_skill(tmp_path, "erp", SKILL))
    assert "erp-analysis" in skills
    s = skills["erp-analysis"]
    assert s.description == "How to read the ERP."
    assert s.when_to_use == "ERP questions."
    assert "Inspect schemas first." in s.body
    assert len(s.sha256) == 64


def test_missing_directory_is_empty_not_an_error(tmp_path):
    assert load_skills(tmp_path / "absent") == {}


def test_skill_without_frontmatter_is_skipped(tmp_path):
    assert load_skills(write_skill(tmp_path, "bad", "# no frontmatter here")) == {}


def test_skill_without_name_is_skipped(tmp_path):
    text = "---\ndescription: nameless\n---\nbody"
    assert load_skills(write_skill(tmp_path, "bad", text)) == {}


def test_unclosed_frontmatter_is_skipped(tmp_path):
    text = "---\nname: x\ndescription: y\nbody without closing fence"
    assert load_skills(write_skill(tmp_path, "bad", text)) == {}


def test_prompt_section_lists_names_and_descriptions(tmp_path):
    skills = load_skills(write_skill(tmp_path, "erp", SKILL))
    section = skills_prompt_section(skills)
    assert "erp-analysis" in section
    assert "How to read the ERP." in section
    # The full body must NOT be in the prompt section (it is fetched on demand).
    assert "Inspect schemas first." not in section


def test_prompt_section_empty_when_no_skills():
    assert skills_prompt_section({}) == ""


def test_use_skill_tool_returns_the_body(tmp_path):
    skills = load_skills(write_skill(tmp_path, "erp", SKILL))
    tool = make_use_skill_tool(skills)
    assert "Inspect schemas first." in tool.invoke({"name": "erp-analysis"})


def test_use_skill_unknown_name_lists_available(tmp_path):
    skills = load_skills(write_skill(tmp_path, "erp", SKILL))
    tool = make_use_skill_tool(skills)
    out = tool.invoke({"name": "nope"})
    assert "Unknown skill" in out
    assert "erp-analysis" in out


def test_shipped_example_skill_loads():
    """The repo's skills/ directory must always load — it is shipped."""
    root = Path(__file__).resolve().parents[1] / "skills"
    skills = load_skills(root)
    assert "erp-analysis" in skills, "the shipped example skill must load"
