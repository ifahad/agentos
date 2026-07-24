"""Council configuration loading and validation."""

import pytest
import yaml

from agentos_runtime.council.config import (
    CouncilConfigError,
    load_council_config,
)

VALID = {
    "judge": "ollama/qwen3.6:latest",
    "quorum": 2,
    "agreement_threshold": 0.6,
    "max_cycles": 8,
    "max_tool_steps": 12,
    "member_timeout_s": 300,
    "members": [
        {"id": "alpha", "model": "ollama/qwen3.6:latest", "enabled": True},
        {"id": "beta", "model": "ollama/gemma4:31b", "enabled": True},
        {"id": "gamma", "model": "moonshot/kimi-k3", "enabled": False},
    ],
}


def write(tmp_path, data):
    path = tmp_path / "council.yaml"
    path.write_text(yaml.safe_dump(data))
    return path


def test_loads_valid_config(tmp_path):
    cfg = load_council_config(write(tmp_path, VALID))
    assert cfg.judge == "ollama/qwen3.6:latest"
    assert cfg.quorum == 2
    assert cfg.max_tool_steps == 12
    assert [m.id for m in cfg.members] == ["alpha", "beta", "gamma"]
    assert [m.id for m in cfg.enabled_members] == ["alpha", "beta"]


def test_member_defaults(tmp_path):
    cfg = load_council_config(write(tmp_path, VALID))
    alpha = cfg.members[0]
    assert alpha.profile == "deep"
    assert alpha.tools == []
    assert alpha.persona == ""
    assert alpha.fallback_model == ""
    assert alpha.budget_usd_per_cycle == 0.0


def test_rejects_council_prefixed_member_model(tmp_path):
    """A member on council/* would recurse into the council: a spend bomb."""
    data = {**VALID, "members": [{"id": "alpha", "model": "council/multiverse", "enabled": True}]}
    with pytest.raises(CouncilConfigError, match="council/"):
        load_council_config(write(tmp_path, data))


def test_rejects_council_prefixed_judge(tmp_path):
    with pytest.raises(CouncilConfigError, match="council/"):
        load_council_config(write(tmp_path, {**VALID, "judge": "council/multiverse"}))


def test_rejects_duplicate_member_ids(tmp_path):
    data = {**VALID, "members": [
        {"id": "alpha", "model": "ollama/a", "enabled": True},
        {"id": "alpha", "model": "ollama/b", "enabled": True},
    ]}
    with pytest.raises(CouncilConfigError, match="duplicate"):
        load_council_config(write(tmp_path, data))


def test_rejects_member_id_with_colon(tmp_path):
    """Thread ids are '{objective}:{member}' — a colon would break namespacing."""
    data = {**VALID, "members": [{"id": "a:b", "model": "ollama/a", "enabled": True}]}
    with pytest.raises(CouncilConfigError, match="id"):
        load_council_config(write(tmp_path, data))


def test_rejects_quorum_above_enabled_member_count(tmp_path):
    """quorum=3 with 2 enabled members can never be satisfied."""
    with pytest.raises(CouncilConfigError, match="quorum"):
        load_council_config(write(tmp_path, {**VALID, "quorum": 3}))


@pytest.mark.parametrize("field,value", [
    ("quorum", 0),
    ("max_cycles", 0),
    ("max_tool_steps", 0),
    ("member_timeout_s", 0),
    ("agreement_threshold", 1.5),
    ("agreement_threshold", -0.1),
])
def test_rejects_out_of_range(tmp_path, field, value):
    with pytest.raises(CouncilConfigError):
        load_council_config(write(tmp_path, {**VALID, field: value}))


def test_rejects_invalid_profile(tmp_path):
    member = {"id": "a", "model": "ollama/a", "enabled": True, "profile": "wat"}
    data = {**VALID, "members": [member]}
    with pytest.raises(CouncilConfigError, match="profile"):
        load_council_config(write(tmp_path, data))


def test_missing_file_raises(tmp_path):
    with pytest.raises(CouncilConfigError, match="not found"):
        load_council_config(tmp_path / "absent.yaml")


def test_shipped_config_is_valid():
    """runtime/council.yaml must always load — it is the shipped default."""
    from pathlib import Path

    path = Path(__file__).resolve().parents[1] / "council.yaml"
    cfg = load_council_config(path)
    assert cfg.members, "shipped config must define members"
