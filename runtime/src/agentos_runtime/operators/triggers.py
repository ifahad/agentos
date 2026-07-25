"""Operator triggers: what makes a standing autonomous task fire.

An operator is a stored objective the runtime fires on its own. Three trigger
kinds: ``interval`` (every N seconds), ``cron`` (a 5-field schedule), and
``webhook`` (fired by an inbound POST bearing an opaque token). Every trigger is
validated at creation so a malformed schedule is rejected up front rather than
silently never firing.
"""

from __future__ import annotations

import secrets
from dataclasses import dataclass
from typing import Any

from croniter import croniter

# Interval floor. A tighter interval than this against a governed agent is
# almost always a mistake, and it protects the scheduler from a busy loop.
MIN_INTERVAL_S = 30

# Opaque webhook token prefix, mirroring the gateway's agos-/agu- convention.
WEBHOOK_PREFIX = "whk-"

TRIGGER_KINDS = frozenset({"interval", "cron", "webhook"})


class TriggerError(ValueError):
    """Raised when a trigger specification is invalid."""


@dataclass(frozen=True)
class Trigger:
    """A validated trigger. Exactly one of interval_s/cron/webhook_token is set
    for its kind."""

    kind: str
    interval_s: int | None = None
    cron: str | None = None
    webhook_token: str | None = None

    def summary(self) -> str:
        """One-line human description, for the console and logs."""
        if self.kind == "interval":
            return f"every {self.interval_s}s"
        if self.kind == "cron":
            return f"cron: {self.cron}"
        return "webhook"

    def to_config(self) -> dict[str, Any]:
        """Serialize to the trigger_config JSONB shape."""
        if self.kind == "interval":
            return {"interval_s": self.interval_s}
        if self.kind == "cron":
            return {"cron": self.cron}
        return {"webhook_token": self.webhook_token}


def parse_trigger(raw: Any) -> Trigger:
    """Validate a trigger dict into a Trigger, or raise TriggerError.

    A webhook trigger without a token gets a fresh one — the token is the secret
    that authorizes an inbound fire, so it is generated server-side, never taken
    from the client.
    """
    if not isinstance(raw, dict):
        raise TriggerError("trigger must be an object")
    kind = str(raw.get("type", "")).strip()
    if kind not in TRIGGER_KINDS:
        raise TriggerError(f"trigger type must be one of {sorted(TRIGGER_KINDS)}")

    if kind == "interval":
        try:
            interval_s = int(raw.get("interval_s"))
        except (TypeError, ValueError) as exc:
            raise TriggerError("interval trigger needs an integer interval_s") from exc
        if interval_s < MIN_INTERVAL_S:
            raise TriggerError(f"interval_s must be >= {MIN_INTERVAL_S}")
        return Trigger(kind="interval", interval_s=interval_s)

    if kind == "cron":
        expr = str(raw.get("cron", "")).strip()
        if not expr:
            raise TriggerError("cron trigger needs a cron expression")
        if not croniter.is_valid(expr):
            raise TriggerError(f"invalid cron expression: {expr!r}")
        return Trigger(kind="cron", cron=expr)

    # webhook: keep an existing token (round-trip from the store) or mint one.
    token = str(raw.get("webhook_token", "")).strip()
    if not token:
        token = WEBHOOK_PREFIX + secrets.token_urlsafe(24)
    return Trigger(kind="webhook", webhook_token=token)


def next_interval_fire(interval_s: int, last_fired: float, now: float) -> float:
    """Absolute time an interval trigger is next due."""
    return last_fired + interval_s if last_fired else now


def next_cron_fire(cron: str, after: float) -> float:
    """Absolute time a cron trigger is next due, strictly after ``after``."""
    return croniter(cron, after).get_next(float)
