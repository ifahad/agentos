"""Context-window management: keep a thread's message history bounded.

Every turn resends the whole conversation, so an agent that runs long enough
will eventually exceed its model's context window and fail — not gracefully, but
with a provider error mid-run, after the work is already paid for. Multi-cycle
autonomous runs reach that point routinely; a single chat rarely does.

This module trims the history immediately before each model call, via
LangGraph's ``pre_model_hook``. Two properties matter more than the trimming
itself:

* The **system prompt is always kept**. It carries the immutable safety frame
  (``SAFETY_PREAMBLE``), so dropping it to save tokens would quietly remove the
  agent's safety rules exactly when a run is at its longest and least
  supervised.
* The window **never starts mid tool-call**. Providers reject a history where an
  assistant tool-call has no matching tool result, or a tool result has no
  originating call. Trimming naively produces exactly that, turning a
  context-limit failure into a malformed-request failure.

Trimming is lossy by nature: dropped turns are gone from the model's view even
though they remain in the checkpoint. That is a deliberate trade — a bounded
agent that forgets early context beats an unbounded one that dies — and it is
why the limit is configurable rather than fixed.
"""

from collections.abc import Callable
from typing import Any

from langchain_core.messages import BaseMessage
from langchain_core.messages.utils import count_tokens_approximately, trim_messages

# Headroom left for the model's own reply. The limit counts what we send; the
# provider's limit covers request plus response, so trimming right up to the
# window would leave no room to answer.
RESPONSE_HEADROOM_TOKENS = 4096


def trim_history(
    messages: list[BaseMessage],
    max_tokens: int,
    token_counter: Callable[[list[BaseMessage]], int] = count_tokens_approximately,
) -> list[BaseMessage]:
    """Return the newest slice of ``messages`` that fits in ``max_tokens``.

    ``max_tokens <= 0`` disables trimming and returns the history unchanged.

    The system message is always retained, and the window is aligned to start on
    a human message so no orphaned tool call or tool result survives at the
    boundary.
    """
    if max_tokens <= 0 or not messages:
        return messages

    budget = max(max_tokens - RESPONSE_HEADROOM_TOKENS, 1)
    if token_counter(messages) <= budget:
        return messages

    trimmed = trim_messages(
        messages,
        max_tokens=budget,
        token_counter=token_counter,
        strategy="last",
        # Keep the safety frame no matter how tight the budget gets.
        include_system=True,
        # Align both edges to a complete exchange: starting on a human message
        # cannot orphan a tool result, and ending on one of these cannot leave a
        # tool call unanswered.
        start_on="human",
        end_on=("human", "tool"),
        allow_partial=False,
    )
    # trim_messages can return nothing when a single message exceeds the budget.
    # Sending an empty history guarantees a provider error, so fall back to the
    # system message plus the latest turn and let the provider judge it.
    if not trimmed:
        system = [m for m in messages[:1] if m.type == "system"]
        return system + messages[-1:]
    return trimmed


def build_trim_hook(max_tokens: int) -> Callable[[dict[str, Any]], dict[str, Any]] | None:
    """Build a LangGraph ``pre_model_hook`` that bounds the context window.

    Returns ``None`` when ``max_tokens <= 0`` so callers can pass the result
    straight through and leave the graph unchanged when trimming is off.

    The hook writes to ``llm_input_messages``, which supplies the model for this
    call only — the checkpointed history is left intact, so trimming never
    destroys the audit trail or a resumable thread.
    """
    if max_tokens <= 0:
        return None

    def hook(state: dict[str, Any]) -> dict[str, Any]:
        messages = state.get("messages", [])
        return {"llm_input_messages": trim_history(messages, max_tokens)}

    return hook
