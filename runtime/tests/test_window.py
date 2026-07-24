"""Context-window trimming: bounds, safety-frame retention, tool-call pairing."""

from langchain_core.messages import AIMessage, HumanMessage, SystemMessage, ToolMessage

from agentos_runtime.window import RESPONSE_HEADROOM_TOKENS, build_trim_hook, trim_history


def counter(messages) -> int:
    """Deterministic token counter: one 'token' per message, for exact assertions."""
    return len(messages)


def conversation(turns: int) -> list:
    """A system prompt, ``turns`` completed human/AI exchanges, then a pending question.

    The trailing human message matters: a ``pre_model_hook`` runs when the model
    is about to reply, so the history it sees always ends on a human or tool
    message and never on an AI one. Building it any other way would test a state
    the hook cannot encounter.
    """
    out: list = [SystemMessage(content="SAFETY RULES (IMMUTABLE)")]
    for i in range(turns):
        out.append(HumanMessage(content=f"question {i}"))
        out.append(AIMessage(content=f"answer {i}"))
    out.append(HumanMessage(content="pending question"))
    return out


def test_short_history_is_returned_unchanged():
    messages = conversation(2)
    assert trim_history(messages, max_tokens=10_000) == messages


def test_trimming_disabled_by_zero_and_negative():
    messages = conversation(50)
    assert trim_history(messages, max_tokens=0) is messages
    assert trim_history(messages, max_tokens=-1) is messages


def test_empty_history_is_safe():
    assert trim_history([], max_tokens=100) == []


def test_long_history_is_bounded():
    messages = conversation(200)
    budget = RESPONSE_HEADROOM_TOKENS + 10
    trimmed = trim_history(messages, max_tokens=budget, token_counter=counter)
    assert len(trimmed) < len(messages)
    assert counter(trimmed) <= budget - RESPONSE_HEADROOM_TOKENS


def test_system_message_survives_trimming():
    # The system prompt carries the immutable safety frame. Losing it to save
    # tokens would strip the agent's safety rules mid-run.
    messages = conversation(200)
    trimmed = trim_history(
        messages, max_tokens=RESPONSE_HEADROOM_TOKENS + 6, token_counter=counter
    )
    assert any(m.type == "system" for m in trimmed), "safety frame was dropped"
    assert trimmed[0].type == "system"


def test_most_recent_turn_is_kept():
    messages = conversation(200)
    trimmed = trim_history(
        messages, max_tokens=RESPONSE_HEADROOM_TOKENS + 8, token_counter=counter
    )
    assert trimmed[-1] is messages[-1], "trimming must keep the newest messages"


def test_trimming_never_orphans_a_tool_result():
    # A ToolMessage with no preceding AIMessage tool call is rejected by
    # providers, which would turn a context problem into a malformed request.
    messages: list = [SystemMessage(content="sys")]
    for i in range(60):
        messages.append(HumanMessage(content=f"q{i}"))
        messages.append(
            AIMessage(
                content="",
                tool_calls=[{"name": "query", "args": {}, "id": f"call{i}"}],
            )
        )
        messages.append(ToolMessage(content=f"result {i}", tool_call_id=f"call{i}"))
        messages.append(AIMessage(content=f"a{i}"))
    messages.append(HumanMessage(content="pending question"))

    trimmed = trim_history(
        messages, max_tokens=RESPONSE_HEADROOM_TOKENS + 12, token_counter=counter
    )

    seen_call_ids = set()
    for m in trimmed:
        if isinstance(m, AIMessage):
            for call in m.tool_calls or []:
                seen_call_ids.add(call["id"])
        if isinstance(m, ToolMessage):
            assert m.tool_call_id in seen_call_ids, (
                f"orphaned tool result {m.tool_call_id}: no matching call in the window"
            )


def test_oversized_single_message_still_yields_a_sendable_history():
    # One enormous message can exceed the budget on its own. Returning nothing
    # guarantees a provider error, so we fall back rather than send an empty list.
    messages = [SystemMessage(content="sys"), HumanMessage(content="x" * 10_000)]
    trimmed = trim_history(messages, max_tokens=RESPONSE_HEADROOM_TOKENS + 1, token_counter=counter)
    assert trimmed, "trimming must never produce an empty history"


def test_build_trim_hook_returns_none_when_disabled():
    assert build_trim_hook(0) is None
    assert build_trim_hook(-5) is None


def test_hook_supplies_model_input_without_mutating_history():
    # llm_input_messages feeds this call only; the checkpointed history must
    # stay whole so threads remain resumable and auditable.
    hook = build_trim_hook(RESPONSE_HEADROOM_TOKENS + 8)
    assert hook is not None
    messages = conversation(200)
    out = hook({"messages": messages})
    assert "llm_input_messages" in out
    assert "messages" not in out, "the hook must not overwrite checkpointed history"
    assert len(out["llm_input_messages"]) <= len(messages)


def test_hook_handles_missing_messages_key():
    hook = build_trim_hook(1000)
    assert hook is not None
    assert hook({})["llm_input_messages"] == []


# --- Integration: the hook must actually be applied by the compiled graph ---


def test_graph_applies_the_trim_hook(monkeypatch):
    """The model must receive the trimmed window, not the full history.

    Unit-testing trim_history proves the trimming logic; it does not prove
    LangGraph calls it. This asserts the wiring end to end, which is the part
    that silently breaks when the graph API changes.
    """
    import asyncio

    from helpers import FakeToolCallingModel
    from langchain_core.messages import AIMessage, HumanMessage
    from langgraph.checkpoint.memory import InMemorySaver

    from agentos_runtime.agent import build_agent
    from agentos_runtime.config import Settings

    seen: dict[str, int] = {}
    model = FakeToolCallingModel(responses=[AIMessage(content="done")])
    original = model._generate

    def spy(messages, stop=None, run_manager=None, **kwargs):
        seen["count"] = len(messages)
        return original(messages, stop, run_manager, **kwargs)

    monkeypatch.setattr(model, "_generate", spy)

    settings = Settings(
        gateway_url="http://gateway",
        gateway_key="k",
        max_context_tokens=6000,
    )
    agent = build_agent(settings, [], InMemorySaver(), model=model)

    history: list = []
    for _ in range(200):
        history.append(HumanMessage(content="q" * 200))
        history.append(AIMessage(content="a" * 200))
    history.append(HumanMessage(content="final question"))

    asyncio.run(
        agent.ainvoke({"messages": history}, config={"configurable": {"thread_id": "t"}})
    )

    assert "count" in seen, "the model was never called"
    assert seen["count"] < len(history), (
        f"model received {seen['count']} of {len(history)} messages — "
        "the trim hook is not being applied by the graph"
    )


def test_deep_profile_builds_with_context_bounding():
    """The deep profile must construct with trimming enabled.

    deepagents takes no pre_model_hook, so this branch bounds context with
    SummarizationMiddleware instead — configured differently enough that a
    mistake here raises at build time and takes the whole service down at
    start-up. Every other test in this file exercises the react profile, so
    without this the deep branch is only ever tested by deploying it.
    """
    from helpers import FakeToolCallingModel
    from langchain_core.messages import AIMessage
    from langgraph.checkpoint.memory import InMemorySaver

    from agentos_runtime.agent import build_agent
    from agentos_runtime.config import Settings

    settings = Settings(
        gateway_url="http://gateway",
        gateway_key="k",
        agent_profile="deep",
        max_context_tokens=6000,
    )
    agent = build_agent(
        settings,
        [],
        InMemorySaver(),
        model=FakeToolCallingModel(responses=[AIMessage(content="ok")]),
    )
    assert agent is not None


def test_deep_profile_builds_without_context_bounding():
    """The deep profile must also build when trimming is off (the default)."""
    from helpers import FakeToolCallingModel
    from langchain_core.messages import AIMessage
    from langgraph.checkpoint.memory import InMemorySaver

    from agentos_runtime.agent import build_agent
    from agentos_runtime.config import Settings

    settings = Settings(
        gateway_url="http://gateway",
        gateway_key="k",
        agent_profile="deep",
        max_context_tokens=0,
    )
    agent = build_agent(
        settings,
        [],
        InMemorySaver(),
        model=FakeToolCallingModel(responses=[AIMessage(content="ok")]),
    )
    assert agent is not None
