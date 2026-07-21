"""Agent profile selection and opt-in OTel tracing tests."""

import pytest
from helpers import make_settings, query, query_then_answer
from langchain_core.messages import AIMessage
from langgraph.checkpoint.memory import InMemorySaver
from pydantic import ValidationError

from agentos_runtime.agent import build_agent
from agentos_runtime.api import Step
from agentos_runtime.otel import record_tool_spans, run_span, setup_tracing


def test_react_profile_is_default_graph():
    agent = build_agent(make_settings(), [query], InMemorySaver(), model=query_then_answer())
    assert "agent" in agent.nodes  # create_react_agent model node


def test_deep_profile_builds_deepagents_graph():
    settings = make_settings(agent_profile="deep")
    agent = build_agent(settings, [query], InMemorySaver(), model=query_then_answer())
    assert "model" in agent.nodes  # deepagents/langchain create_agent model node
    assert "agent" not in agent.nodes
    assert "tools" in agent.nodes


async def test_deep_profile_answers_with_fake_model():
    settings = make_settings(agent_profile="deep")
    agent = build_agent(
        settings,
        [query],
        InMemorySaver(),
        model=query_then_answer(answer="deep answer"),
    )
    result = await agent.ainvoke(
        {"messages": [{"role": "user", "content": "count rows"}]},
        config={"configurable": {"thread_id": "deep-1"}},
    )
    final = [m for m in result["messages"] if isinstance(m, AIMessage)][-1]
    assert final.content == "deep answer"


def test_invalid_profile_rejected():
    with pytest.raises(ValidationError):
        make_settings(agent_profile="galaxy-brain")


def test_otel_disabled_returns_none_and_noops():
    assert setup_tracing(make_settings()) is None
    with run_span(None, "t-1") as span:
        assert span is None
    record_tool_spans(None, [Step(tool="query", input={"sql": "SELECT 1"})])


def test_otel_enabled_creates_tracer_and_spans(monkeypatch):
    from opentelemetry import trace
    from opentelemetry.exporter.otlp.proto.http import trace_exporter
    from opentelemetry.sdk.trace.export import SpanExportResult

    exported = []
    monkeypatch.setattr(
        trace_exporter.OTLPSpanExporter,
        "export",
        lambda self, spans: exported.extend(spans) or SpanExportResult.SUCCESS,
    )
    settings = make_settings(otel_endpoint="http://otel-collector:4318")
    tracer = setup_tracing(settings)
    assert tracer is not None
    with run_span(tracer, "t-2") as span:
        assert span is not None
        record_tool_spans(tracer, [Step(tool="query", input={"sql": "SELECT 1"})])
    trace.get_tracer_provider().shutdown()  # flush through the patched exporter
    names = sorted(s.name for s in exported)
    assert names == ["agent.run", "agent.tool"]
    run = next(s for s in exported if s.name == "agent.run")
    assert run.attributes["agentos.thread_id"] == "t-2"
    tool = next(s for s in exported if s.name == "agent.tool")
    assert tool.attributes["agentos.tool"] == "query"
