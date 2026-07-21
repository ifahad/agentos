"""Configuration parsed from the environment (per the frozen Phase 5 contract)."""

from __future__ import annotations

import os
from dataclasses import dataclass, field

from .allowlist import parse_allowlist

DEFAULT_TIMEOUT_S = 20.0
DEFAULT_MAX_TEXT = 8000
LISTEN_HOST = "0.0.0.0"  # noqa: S104 - container service, bound intentionally
LISTEN_PORT = 8094
MCP_PATH = "/mcp"


@dataclass
class Config:
    """Runtime configuration for the browser connector."""

    allow_domains: list[str] = field(default_factory=list)
    timeout_s: float = DEFAULT_TIMEOUT_S
    max_text: int = DEFAULT_MAX_TEXT

    @property
    def timeout_ms(self) -> float:
        return self.timeout_s * 1000.0

    @classmethod
    def from_env(cls, env: dict[str, str] | None = None) -> Config:
        e = os.environ if env is None else env
        return cls(
            allow_domains=parse_allowlist(e.get("AGENTOS_BROWSER_ALLOW_DOMAINS")),
            timeout_s=_float(e.get("AGENTOS_BROWSER_TIMEOUT_S"), DEFAULT_TIMEOUT_S),
            max_text=_int(e.get("AGENTOS_BROWSER_MAX_TEXT"), DEFAULT_MAX_TEXT),
        )


def _float(raw: str | None, default: float) -> float:
    if not raw:
        return default
    try:
        return float(raw)
    except ValueError:
        return default


def _int(raw: str | None, default: int) -> int:
    if not raw:
        return default
    try:
        return int(raw)
    except ValueError:
        return default
