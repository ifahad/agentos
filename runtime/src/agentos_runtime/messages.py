"""Shared helpers for extracting text and tool calls from graph messages.

Used by both the HTTP API response builders and the eval scorer, so it lives
outside ``api`` to avoid an import cycle (``api`` includes the eval router).
"""

from typing import Any

from langchain_core.messages import AIMessage, BaseMessage


def message_text(message: BaseMessage) -> str:
    """Plain text of a message, flattening content blocks."""
    content = message.content
    if isinstance(content, str):
        return content
    parts: list[str] = []
    for block in content:
        if isinstance(block, str):
            parts.append(block)
        elif isinstance(block, dict) and block.get("type") == "text":
            parts.append(block.get("text", ""))
    return "".join(parts)


def extract_output(messages: list[BaseMessage]) -> str:
    """Text of the last AI message (the agent's final answer)."""
    for message in reversed(messages):
        if isinstance(message, AIMessage):
            return message_text(message)
    return ""


def extract_tool_calls(messages: list[BaseMessage]) -> list[tuple[str, dict[str, Any]]]:
    """All (tool name, args) pairs requested across the run, in order."""
    calls: list[tuple[str, dict[str, Any]]] = []
    for message in messages:
        if isinstance(message, AIMessage):
            for tool_call in message.tool_calls or []:
                calls.append((tool_call["name"], dict(tool_call["args"])))
    return calls
