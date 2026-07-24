"""The council loop: one cycle, its stop conditions, and the heartbeat.

A cycle is fanout -> judge -> persist -> decide. An objective runs cycles until
a stop condition fires. Every stop is recorded as the objective's stop_reason,
so an operator can always answer "why did this stop?" from the record alone.
"""

from __future__ import annotations

import asyncio
import logging
import os
import socket
from dataclasses import dataclass
from typing import Any

from agentos_runtime.council.config import CouncilConfig
from agentos_runtime.council.fanout import STATUS_ANSWERED, fanout, quorum_met
from agentos_runtime.council.judge import JUDGE_UNAVAILABLE, Verdict, synthesize

logger = logging.getLogger(__name__)

STOP_CONVERGED = "converged"
STOP_MAX_CYCLES = "max_cycles"
STOP_BUDGET = "budget_exceeded"
STOP_PAUSED = "paused"
STOP_CANCELLED = "cancelled"
STOP_NEEDS_REVIEW = "needs_review"

# Stop reasons that mean "a human should look at this".
REVIEW_REASONS = frozenset({STOP_MAX_CYCLES, STOP_NEEDS_REVIEW, STOP_BUDGET})


@dataclass
class CouncilDeps:
    """Everything a cycle needs. Injected so the loop is testable without models."""

    config: CouncilConfig
    settings: Any
    tools: list[Any]
    checkpointer: Any
    store: Any
    judge_model: Any
    fanout_fn: Any = fanout
    synthesize_fn: Any = synthesize


def decide_stop(
    verdict: Verdict,
    cycle_no: int,
    max_cycles: int,
    spend_usd: float,
    budget_usd: float,
    threshold: float,
    quorum_ok: bool,
) -> str | None:
    """The stop reason for this cycle, or None to run another.

    Order matters: a failed cycle (no quorum, no judge) is a review case even if
    the budget also ran out, because the operator needs the more specific
    reason. Budget and cycle caps are checked before continuing so a
    non-converging objective can never run forever.
    """
    if not quorum_ok:
        return STOP_NEEDS_REVIEW
    if verdict.status == JUDGE_UNAVAILABLE:
        return STOP_NEEDS_REVIEW
    if verdict.done or verdict.agreement >= threshold:
        return STOP_CONVERGED
    if budget_usd > 0 and spend_usd >= budget_usd:
        return STOP_BUDGET
    if cycle_no >= max_cycles:
        return STOP_MAX_CYCLES
    return None


async def run_cycle(deps: CouncilDeps, objective: dict, cycle_no: int) -> tuple[Verdict, bool]:
    """Run one cycle: fan out, judge, persist. Returns (verdict, quorum_ok)."""
    config = deps.config
    answers = await deps.fanout_fn(
        config=config,
        settings=deps.settings,
        members=config.enabled_members,
        tools=deps.tools,
        checkpointer=deps.checkpointer,
        objective_id=objective["id"],
        cycle_no=cycle_no,
        input_text=objective["input"],
        store=deps.store,
    )
    quorum_ok = quorum_met(answers, config.quorum)
    if quorum_ok:
        verdict = await deps.synthesize_fn(deps.judge_model, objective["input"], answers)
    else:
        logger.warning(
            "council objective %s cycle %d: only %d/%d members answered",
            objective["id"], cycle_no,
            sum(1 for a in answers if a.status == STATUS_ANSWERED), config.quorum,
        )
        verdict = Verdict(answer="", agreement=0.0, status=JUDGE_UNAVAILABLE)

    cycle_id = await deps.store.insert_cycle(
        objective_id=objective["id"],
        cycle_no=cycle_no,
        verdict={
            "answer": verdict.answer,
            "cited_members": verdict.cited_members,
            "done": verdict.done,
            "status": verdict.status,
        },
        agreement=verdict.agreement,
        dissent=verdict.dissent,
    )
    for answer in answers:
        await deps.store.insert_member_run(
            cycle_id=cycle_id,
            member_id=answer.member_id,
            model_used=answer.model_used,
            thread_id=answer.thread_id,
            status=answer.status,
            output=answer.output,
            steps=answer.steps,
            cost_usd=0.0,
            error=answer.error,
        )
    return verdict, quorum_ok


async def run_objective(deps: CouncilDeps, objective: dict) -> str:
    """Run cycles until a stop condition fires. Returns the stop reason."""
    config = deps.config
    max_cycles = objective.get("max_cycles") or config.max_cycles
    budget_usd = float(objective.get("budget_usd") or 0.0)
    spend_usd = float(objective.get("spend_usd") or 0.0)
    reason = STOP_NEEDS_REVIEW

    for cycle_no in range(1, max_cycles + 1):
        # The kill switch is checked BEFORE each cycle, so pausing takes effect
        # within one cycle rather than at the end of the objective.
        if await deps.store.is_paused():
            reason = STOP_PAUSED
            break
        current = await deps.store.get_objective(objective["id"])
        if current and current.get("status") == STOP_CANCELLED:
            reason = STOP_CANCELLED
            break

        verdict, quorum_ok = await run_cycle(deps, objective, cycle_no)
        await deps.store.update_objective(objective["id"], cycles_run=cycle_no)

        stop = decide_stop(
            verdict, cycle_no, max_cycles, spend_usd, budget_usd,
            config.agreement_threshold, quorum_ok,
        )
        if stop:
            reason = stop
            break
    else:
        reason = STOP_MAX_CYCLES

    status = "needs_review" if reason in REVIEW_REASONS else "completed"
    if reason in (STOP_PAUSED, STOP_CANCELLED):
        status = reason
    await deps.store.update_objective(objective["id"], status=status, stop_reason=reason)
    logger.info("council objective %s stopped: %s", objective["id"], reason)
    return reason


def worker_id() -> str:
    """Identifies this runtime replica in claimed_by."""
    return f"{socket.gethostname()}:{os.getpid()}"


async def heartbeat(deps: CouncilDeps, interval_s: int, stop_event: asyncio.Event) -> None:
    """Claim and run pending objectives until stopped.

    Claiming uses FOR UPDATE SKIP LOCKED in the store, so several replicas can
    run this loop concurrently without ever double-running an objective.
    """
    worker = worker_id()
    logger.info("council heartbeat started (worker=%s interval=%ss)", worker, interval_s)
    while not stop_event.is_set():
        try:
            if not await deps.store.is_paused():
                objective = await deps.store.claim_next_objective(worker)
                if objective is not None:
                    await run_objective(deps, objective)
                    continue  # drain the queue before sleeping again
        except asyncio.CancelledError:
            raise
        except Exception:  # noqa: BLE001 - the heartbeat must survive one bad objective
            logger.exception("council heartbeat cycle failed")
        try:
            await asyncio.wait_for(stop_event.wait(), timeout=interval_s)
        except TimeoutError:
            pass
    logger.info("council heartbeat stopped")
