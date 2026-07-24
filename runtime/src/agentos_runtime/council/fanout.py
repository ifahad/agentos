"""Council fan-out: build one agent per member and run them concurrently.

Each member is an independent deep agent bound to its own model, with its own
checkpoint thread. Members never see each other's answers — only the judge does.
A member that errors, times out, or is cancelled is recorded as a failure and
the cycle proceeds if quorum still holds; one bad vendor must not deny the
council its verdict.
"""

from __future__ import annotations

import asyncio
import logging
from collections.abc import Sequence
from dataclasses import dataclass, field
from typing import Any

from agentos_runtime.agent import build_chat_model
from agentos_runtime.council.config import CouncilConfig, Member
from agentos_runtime.hitl import run_until_settled
from agentos_runtime.messages import extract_output, extract_tool_calls

logger = logging.getLogger(__name__)

STATUS_ANSWERED = "answered"
STATUS_FAILED = "failed"
STATUS_TIMEOUT = "timeout"


@dataclass
class MemberAnswer:
    """One member's contribution to a cycle."""

    member_id: str
    model_used: str
    thread_id: str
    status: str
    output: str
    steps: list[dict[str, Any]] = field(default_factory=list)
    error: str = ""


def member_thread_id(objective_id: str, member_id: str) -> str:
    """Checkpoint thread for one member on one objective.

    Namespacing by objective AND member keeps each member's history isolated:
    members must reason independently, so they must never share a thread. Member
    ids are validated to contain no ':' so this stays unambiguous.
    """
    return f"{objective_id}:{member_id}"


def build_member_agent(
    settings: Any,
    member: Member,
    tools: Sequence[Any],
    checkpointer: Any,
    model: str | None = None,
) -> Any:
    """Build one member's agent: its own model, profile, persona, and tools.

    The persona refines the system prompt; build_agent still prepends the
    immutable SAFETY_PREAMBLE, so no member configuration can weaken the safety
    frame. Imported lazily so tests that inject an agent_factory need not import
    the whole agent stack.
    """
    from agentos_runtime.agent import build_agent

    chat_model = build_chat_model(settings, model=model or member.model)
    member_tools = _select_tools(tools, member.tools)
    profile_settings = _with_profile(settings, member.profile)
    return build_agent(
        profile_settings,
        tools=member_tools,
        checkpointer=checkpointer,
        model=chat_model,
        prompt=member.persona or None,
    )


def _select_tools(tools: Sequence[Any], names: Sequence[str]) -> list[Any]:
    """Restrict a member to its configured tools (empty list = all loaded tools).

    A member can only narrow what the runtime already loaded; it can never name
    a tool into existence.
    """
    if not names:
        return list(tools)
    wanted = set(names)
    return [t for t in tools if getattr(t, "name", "") in wanted]


def _with_profile(settings: Any, profile: str) -> Any:
    """A settings copy whose agent_profile is the member's.

    build_agent reads agent_profile off settings; members choose react or deep
    independently, so each needs its own view.
    """
    return settings.model_copy(update={"agent_profile": profile})


async def fanout(
    config: CouncilConfig,
    settings: Any,
    members: Sequence[Member],
    tools: Sequence[Any],
    checkpointer: Any,
    objective_id: str,
    cycle_no: int,
    input_text: str,
    agent_factory=build_member_agent,
) -> list[MemberAnswer]:
    """Run every member on the same input concurrently.

    Returns one MemberAnswer per member, in the order given. Never raises for a
    member failure: exceptions and timeouts become failed answers so the caller
    can apply quorum.
    """
    tasks = [
        _run_member(
            config, settings, member, tools, checkpointer,
            objective_id, cycle_no, input_text, agent_factory,
        )
        for member in members
    ]
    return list(await asyncio.gather(*tasks))


async def _run_member(
    config: CouncilConfig,
    settings: Any,
    member: Member,
    tools: Sequence[Any],
    checkpointer: Any,
    objective_id: str,
    cycle_no: int,
    input_text: str,
    agent_factory,
) -> MemberAnswer:
    """Run one member to an answer, converting every failure into a status."""
    thread_id = member_thread_id(objective_id, member.id)
    model_used = member.model
    try:
        return await asyncio.wait_for(
            _invoke_member(
                config, settings, member, tools, checkpointer,
                thread_id, cycle_no, input_text, agent_factory, model_used,
            ),
            timeout=config.member_timeout_s,
        )
    except TimeoutError:
        logger.warning(
            "council member %s timed out after %ss on %s",
            member.id, config.member_timeout_s, objective_id,
        )
        return MemberAnswer(
            member_id=member.id, model_used=model_used, thread_id=thread_id,
            status=STATUS_TIMEOUT, output="",
            error=f"timed out after {config.member_timeout_s}s",
        )
    except Exception as exc:  # noqa: BLE001 - a member failure must not abort the cycle
        logger.warning("council member %s failed on %s: %s", member.id, objective_id, exc)
        return MemberAnswer(
            member_id=member.id, model_used=model_used, thread_id=thread_id,
            status=STATUS_FAILED, output="", error=str(exc),
        )


async def _invoke_member(
    config: CouncilConfig,
    settings: Any,
    member: Member,
    tools: Sequence[Any],
    checkpointer: Any,
    thread_id: str,
    cycle_no: int,
    input_text: str,
    agent_factory,
    model_used: str,
) -> MemberAnswer:
    """Invoke a member's agent, falling back to its fallback_model once."""
    try:
        agent = agent_factory(settings, member, tools, checkpointer)
        outcome = await _drive(agent, thread_id, cycle_no, input_text, config)
    except Exception:
        if not member.fallback_model:
            raise
        logger.warning(
            "council member %s falling back to %s", member.id, member.fallback_model
        )
        model_used = member.fallback_model
        agent = agent_factory(settings, member, tools, checkpointer, model=model_used)
        outcome = await _drive(agent, thread_id, cycle_no, input_text, config)

    messages = outcome.values.get("messages", [])
    return MemberAnswer(
        member_id=member.id,
        model_used=model_used,
        thread_id=thread_id,
        status=STATUS_ANSWERED,
        output=extract_output(messages),
        steps=[
            {"tool": name, "input": args}
            for name, args in extract_tool_calls(messages)
        ],
    )


async def _drive(agent, thread_id, cycle_no, input_text, config):
    """Drive one member's graph under the council's per-run step cap.

    recursion_limit is the cycle cap the runtime previously lacked: without it a
    member could loop until LangGraph's default 25, unbounded by council config.
    """
    run_config = {
        "configurable": {"thread_id": f"{thread_id}#{cycle_no}"},
        "recursion_limit": config.max_tool_steps,
    }
    return await run_until_settled(
        agent, {"messages": [("user", input_text)]}, run_config, approval_tools=[]
    )


def quorum_met(answers: Sequence[MemberAnswer], quorum: int) -> bool:
    """Whether enough members answered for the verdict to be valid."""
    return sum(1 for a in answers if a.status == STATUS_ANSWERED) >= quorum
