"""Council configuration: member registry loading and validation.

The council is configured entirely by ``council.yaml`` (AGENTOS_COUNCIL_CONFIG)
so adding, removing, or re-modeling a member never requires a code change. Every
invalid configuration is rejected at load with a specific message rather than
failing mid-cycle against a paid API.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path

import yaml

VALID_PROFILES = frozenset({"react", "deep"})

# A member or judge on a council/* model would re-enter the council: unbounded
# recursion and unbounded spend. Rejected at load; the gateway enforces the same
# rule independently at request time (defense in depth).
COUNCIL_PREFIX = "council/"


class CouncilConfigError(ValueError):
    """Raised when council.yaml is missing, unparseable, or invalid."""


@dataclass(frozen=True)
class Member:
    """One council member: a deep agent bound to one model."""

    id: str
    model: str
    enabled: bool = False
    profile: str = "deep"
    persona: str = ""
    tools: list[str] = field(default_factory=list)
    budget_usd_per_cycle: float = 0.0
    fallback_model: str = ""


@dataclass(frozen=True)
class CouncilConfig:
    """The full council registry."""

    judge: str
    quorum: int
    agreement_threshold: float
    max_cycles: int
    max_tool_steps: int
    member_timeout_s: int
    members: list[Member]

    @property
    def enabled_members(self) -> list[Member]:
        """Members that will actually be called."""
        return [m for m in self.members if m.enabled]


def load_council_config(path: str | Path) -> CouncilConfig:
    """Load and validate council.yaml.

    Raises CouncilConfigError with a specific reason for any problem.
    """
    path = Path(path)
    if not path.is_file():
        raise CouncilConfigError(f"council config not found: {path}")
    try:
        raw = yaml.safe_load(path.read_text()) or {}
    except yaml.YAMLError as exc:
        raise CouncilConfigError(f"council config is not valid YAML: {exc}") from exc
    if not isinstance(raw, dict):
        raise CouncilConfigError("council config must be a mapping")

    judge = str(raw.get("judge", "")).strip()
    if not judge:
        raise CouncilConfigError("judge is required")
    _reject_council_model(judge, "judge")

    quorum = _positive_int(raw, "quorum", default=1)
    max_cycles = _positive_int(raw, "max_cycles", default=8)
    max_tool_steps = _positive_int(raw, "max_tool_steps", default=12)
    member_timeout_s = _positive_int(raw, "member_timeout_s", default=300)

    threshold = raw.get("agreement_threshold", 0.6)
    try:
        threshold = float(threshold)
    except (TypeError, ValueError) as exc:
        raise CouncilConfigError("agreement_threshold must be a number") from exc
    if not 0.0 <= threshold <= 1.0:
        raise CouncilConfigError("agreement_threshold must be between 0 and 1")

    members = _parse_members(raw.get("members") or [])
    enabled = [m for m in members if m.enabled]
    if quorum > len(enabled):
        raise CouncilConfigError(
            f"quorum {quorum} exceeds the {len(enabled)} enabled member(s); "
            "it could never be satisfied"
        )
    return CouncilConfig(
        judge=judge,
        quorum=quorum,
        agreement_threshold=threshold,
        max_cycles=max_cycles,
        max_tool_steps=max_tool_steps,
        member_timeout_s=member_timeout_s,
        members=members,
    )


def _parse_members(raw_members: object) -> list[Member]:
    if not isinstance(raw_members, list):
        raise CouncilConfigError("members must be a list")
    members: list[Member] = []
    seen: set[str] = set()
    for entry in raw_members:
        if not isinstance(entry, dict):
            raise CouncilConfigError("each member must be a mapping")
        member_id = str(entry.get("id", "")).strip()
        if not member_id:
            raise CouncilConfigError("member id is required")
        if ":" in member_id or "/" in member_id or any(c.isspace() for c in member_id):
            raise CouncilConfigError(
                f"member id {member_id!r} must not contain ':', '/', or whitespace "
                "(ids namespace checkpoint thread ids)"
            )
        if member_id in seen:
            raise CouncilConfigError(f"duplicate member id {member_id!r}")
        seen.add(member_id)

        model = str(entry.get("model", "")).strip()
        if not model:
            raise CouncilConfigError(f"member {member_id!r}: model is required")
        _reject_council_model(model, f"member {member_id!r}")

        fallback = str(entry.get("fallback_model", "")).strip()
        if fallback:
            _reject_council_model(fallback, f"member {member_id!r} fallback_model")

        profile = str(entry.get("profile", "deep")).strip()
        if profile not in VALID_PROFILES:
            raise CouncilConfigError(
                f"member {member_id!r}: profile must be one of {sorted(VALID_PROFILES)}"
            )

        tools = entry.get("tools") or []
        if not isinstance(tools, list) or not all(isinstance(t, str) for t in tools):
            raise CouncilConfigError(f"member {member_id!r}: tools must be a list of strings")

        try:
            budget = float(entry.get("budget_usd_per_cycle", 0.0))
        except (TypeError, ValueError) as exc:
            raise CouncilConfigError(
                f"member {member_id!r}: budget_usd_per_cycle must be a number"
            ) from exc
        if budget < 0:
            raise CouncilConfigError(f"member {member_id!r}: budget_usd_per_cycle must be >= 0")

        members.append(
            Member(
                id=member_id,
                model=model,
                enabled=bool(entry.get("enabled", False)),
                profile=profile,
                persona=str(entry.get("persona", "")),
                tools=list(tools),
                budget_usd_per_cycle=budget,
                fallback_model=fallback,
            )
        )
    return members


def _reject_council_model(model: str, where: str) -> None:
    if model.startswith(COUNCIL_PREFIX):
        raise CouncilConfigError(
            f"{where}: model {model!r} uses the reserved 'council/' prefix, which "
            "would make the council call itself (unbounded recursion and spend)"
        )


def _positive_int(raw: dict, key: str, default: int) -> int:
    value = raw.get(key, default)
    try:
        value = int(value)
    except (TypeError, ValueError) as exc:
        raise CouncilConfigError(f"{key} must be an integer") from exc
    if value < 1:
        raise CouncilConfigError(f"{key} must be >= 1")
    return value
