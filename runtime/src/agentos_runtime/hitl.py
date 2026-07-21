"""Human-in-the-loop run loop: interrupt inspection, auto-resume, approve/deny.

The agent graph is compiled with ``interrupt_before=["tools"]`` whenever
AGENTOS_APPROVAL_TOOLS is non-empty, so every tool batch pauses. This module
decides what to do at each pause: auto-resume when no pending call needs
approval, or surface the pending calls to the caller for a human decision.
"""

from dataclasses import dataclass
from typing import Any

from langchain_core.messages import AIMessage, ToolMessage


@dataclass
class PendingCall:
    """One tool call awaiting execution at an interrupt."""

    tool: str
    input: dict[str, Any]
    tool_call_id: str


@dataclass
class RunOutcome:
    """Result of driving the graph until completion or an approval interrupt."""

    status: str  # "completed" | "pending_approval"
    values: dict[str, Any]  # graph state values (has "messages")
    pending: list[PendingCall]


def pending_tool_calls(snapshot: Any) -> list[PendingCall]:
    """Tool calls that will execute next, from an interrupted state snapshot.

    Empty when the snapshot is not interrupted before the tools node.
    """
    if "tools" not in (snapshot.next or ()):
        return []
    messages = snapshot.values.get("messages", [])
    for message in reversed(messages):
        if isinstance(message, AIMessage):
            return [
                PendingCall(
                    tool=tc["name"],
                    input=dict(tc["args"]),
                    tool_call_id=tc["id"],
                )
                for tc in message.tool_calls or []
            ]
    return []


async def run_until_settled(
    agent: Any,
    input_state: dict[str, Any] | None,
    config: dict[str, Any],
    approval_tools: list[str],
) -> RunOutcome:
    """Invoke the graph, auto-resuming interrupts that need no approval.

    Returns once the run completes or an interrupt has at least one pending
    tool call in ``approval_tools``. ``input_state=None`` resumes an already
    interrupted thread.
    """
    values = await agent.ainvoke(input_state, config=config)
    while True:
        snapshot = await agent.aget_state(config)
        if not snapshot.next:
            return RunOutcome(status="completed", values=values, pending=[])
        pending = pending_tool_calls(snapshot)
        if any(call.tool in approval_tools for call in pending):
            return RunOutcome(
                status="pending_approval", values=snapshot.values, pending=pending
            )
        values = await agent.ainvoke(None, config=config)


async def deny_pending(agent: Any, config: dict[str, Any], pending: list[PendingCall]) -> None:
    """Inject a denial ToolMessage for each pending call, as the tools node.

    The graph then resumes past the tools node as if the tools had run and
    returned the denial text.
    """
    denials = [
        ToolMessage(
            content="Denied by human reviewer.",
            tool_call_id=call.tool_call_id,
            name=call.tool,
        )
        for call in pending
    ]
    await agent.aupdate_state(config, {"messages": denials}, as_node="tools")
