"""The operator engine: fire one operator, and the scheduler that fires them.

An operator run drives the runtime's single governed agent toward the operator's
goal, bounded by max_cycles and going through the gateway like every other run —
so budgets, rate limits, guardrails, and the audit trail all apply unchanged. A
run that hits a human-approval interrupt is recorded ``needs_approval`` and is
NOT auto-resumed: autonomy never approves its own writes.

The scheduler is opt-in (AGENTOS_AUTONOMY_ENABLED) and fires due interval/cron
operators sequentially, never overlapping a single operator with itself.
"""

from __future__ import annotations

import asyncio
import logging
import time
import uuid
from dataclasses import dataclass, field
from typing import Any

from agentos_runtime.hitl import run_until_settled
from agentos_runtime.messages import extract_output, extract_tool_calls
from agentos_runtime.operators.triggers import next_cron_fire

logger = logging.getLogger(__name__)

STATUS_COMPLETED = "completed"
STATUS_NEEDS_APPROVAL = "needs_approval"
STATUS_ERROR = "error"

# trigger_source values recorded on a run.
SOURCE_MANUAL = "manual"
SOURCE_INTERVAL = "interval"
SOURCE_CRON = "cron"
SOURCE_WEBHOOK = "webhook"


@dataclass
class OperatorDeps:
    """What the engine needs. Injected so the loop is testable without a model."""

    store: Any
    agent_builder: Any  # (prompt: str | None) -> agent
    approval_tools: list[str]
    max_cycles_default: int = 8
    notify: Any = None  # optional async (operator, run) -> None; SSRF-screened
    # A monotonic clock, injected so tests drive time deterministically.
    clock: Any = time.monotonic


async def fire_operator(
    deps: OperatorDeps,
    operator: dict[str, Any],
    trigger_source: str,
    context: dict[str, Any] | None = None,
) -> dict[str, Any]:
    """Run one operator once and persist the outcome. Never raises.

    context (a webhook body, say) is appended to the goal as untrusted data, so
    an operator can react to an inbound payload without that payload being able
    to rewrite its instructions.
    """
    max_cycles = int(operator.get("max_cycles") or deps.max_cycles_default)
    thread_id = f"op:{operator['id']}:{uuid.uuid4().hex[:8]}"
    goal = operator["goal"]
    if context:
        goal = f"{goal}\n\n<trigger_payload>\n{context}\n</trigger_payload>"

    try:
        agent = deps.agent_builder(None)
        run_config = {
            "configurable": {"thread_id": thread_id},
            # 2*max_cycles+1 leaves room for the model turn between each tool
            # batch; the recorded cycle count is the actual tool calls made.
            "recursion_limit": 2 * max_cycles + 1,
        }
        outcome = await run_until_settled(
            agent, {"messages": [("user", goal)]}, run_config, deps.approval_tools
        )
        messages = outcome.values.get("messages", [])
        steps = [{"tool": name, "input": args} for name, args in extract_tool_calls(messages)]

        if outcome.status == "pending_approval":
            run = await deps.store.insert_run(
                operator_id=operator["id"], thread_id=thread_id,
                status=STATUS_NEEDS_APPROVAL, output="", steps=steps,
                cycles=len(steps), trigger_source=trigger_source,
                error="awaiting human approval",
            )
        else:
            run = await deps.store.insert_run(
                operator_id=operator["id"], thread_id=thread_id,
                status=STATUS_COMPLETED, output=extract_output(messages), steps=steps,
                cycles=len(steps), trigger_source=trigger_source,
            )
    except Exception as exc:  # noqa: BLE001 - a bad run must never crash the scheduler
        logger.warning("operator %s run failed: %s", operator["id"], exc)
        run = await deps.store.insert_run(
            operator_id=operator["id"], thread_id=thread_id,
            status=STATUS_ERROR, output="", steps=[], cycles=0,
            trigger_source=trigger_source, error=str(exc),
        )

    await deps.store.mark_fired(operator["id"])
    if deps.notify is not None:
        try:
            await deps.notify(operator, run)
        except Exception:  # noqa: BLE001 - a notification failure must not fail the run
            logger.warning("operator %s notification failed", operator["id"], exc_info=True)
    return run


def _due(
    operator: dict[str, Any], last_monotonic: float | None, now: float, wall_now: float
) -> bool:
    """Whether an interval/cron operator is due to fire.

    interval uses the injected monotonic clock (robust to wall-clock jumps);
    cron uses wall time via croniter. last_monotonic is None until the first
    fire in this process — an interval operator fires promptly on first sight,
    a cron operator waits for its next scheduled instant.
    """
    trigger = operator["trigger"]
    if trigger["type"] == "interval":
        if last_monotonic is None:
            return True
        return now - last_monotonic >= trigger["interval_s"]
    if trigger["type"] == "cron":
        if last_monotonic is None:
            # Seed: fire at the next cron instant, not immediately, so enabling a
            # cron operator does not fire it out of schedule.
            return False
        return wall_now >= next_cron_fire(trigger["cron"], last_monotonic)
    return False


@dataclass
class Scheduler:
    """Background scheduler firing due interval/cron operators."""

    deps: OperatorDeps
    tick_s: int = 15
    # operator id -> monotonic time last fired (or the cron seed instant).
    _last: dict[str, float] = field(default_factory=dict)

    async def tick(self, now: float, wall_now: float) -> list[str]:
        """One scheduler pass. Returns the ids fired, for tests and logs."""
        fired: list[str] = []
        for operator in await self.deps.store.enabled_scheduled():
            oid = operator["id"]
            last = self._last.get(oid)
            if operator["trigger"]["type"] == "cron" and last is None:
                # Seed cron with the current wall time so the first fire is the
                # next scheduled instant.
                self._last[oid] = wall_now
                continue
            if _due(operator, last, now, wall_now):
                self._last[oid] = now if operator["trigger"]["type"] == "interval" else wall_now
                await fire_operator(self.deps, operator, operator["trigger"]["type"])
                fired.append(oid)
        return fired

    async def run(self, stop_event: asyncio.Event) -> None:
        """Loop until stopped, ticking every tick_s. Survives a bad tick."""
        logger.info("operator scheduler started (tick=%ss)", self.tick_s)
        while not stop_event.is_set():
            try:
                await self.tick(self.deps.clock(), time.time())
            except asyncio.CancelledError:
                raise
            except Exception:  # noqa: BLE001 - the scheduler must outlive one bad tick
                logger.exception("operator scheduler tick failed")
            try:
                await asyncio.wait_for(stop_event.wait(), timeout=self.tick_s)
            except TimeoutError:
                pass
        logger.info("operator scheduler stopped")
