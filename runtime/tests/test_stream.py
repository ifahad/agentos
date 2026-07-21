"""SSE streaming tests for POST /runs/stream."""

import json

import httpx
import pytest
from helpers import AUTH_HEADERS, lookup, make_settings, query, query_then_answer
from langgraph.checkpoint.memory import InMemorySaver

from agentos_runtime.agent import build_agent
from agentos_runtime.api import app


def mount_agent(approval_tools: list[str], model=None):
    settings = make_settings(approval_tools=",".join(approval_tools))
    app.state.agent = build_agent(
        settings, [query, lookup], InMemorySaver(), model=model or query_then_answer()
    )
    app.state.approval_tools = settings.approval_tool_names


@pytest.fixture
async def client():
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(
        transport=transport, base_url="http://test", headers=AUTH_HEADERS
    ) as c:
        yield c
    for attr in ("agent", "approval_tools"):
        if hasattr(app.state, attr):
            delattr(app.state, attr)


def parse_sse(text: str) -> list[dict]:
    events = []
    for line in text.splitlines():
        if line.startswith("data: "):
            events.append(json.loads(line[len("data: ") :]))
    return events


async def test_stream_step_then_done(client):
    mount_agent([])
    response = await client.post(
        "/runs/stream", json={"input": "count rows", "thread_id": "s-1"}
    )
    assert response.status_code == 200
    assert response.headers["content-type"].startswith("text/event-stream")
    events = parse_sse(response.text)
    assert [e["event"] for e in events] == ["step", "done"]
    assert events[0] == {"event": "step", "tool": "query", "input": {"sql": "SELECT 1"}}
    assert events[1] == {
        "event": "done",
        "thread_id": "s-1",
        "output": "answer",
        "steps": [{"tool": "query", "input": {"sql": "SELECT 1"}}],
    }


async def test_stream_pending_approval_ends_stream(client):
    mount_agent(["query"])
    response = await client.post(
        "/runs/stream", json={"input": "count rows", "thread_id": "s-2"}
    )
    events = parse_sse(response.text)
    assert [e["event"] for e in events] == ["step", "pending_approval"]
    assert events[-1] == {
        "event": "pending_approval",
        "pending": [{"tool": "query", "input": {"sql": "SELECT 1"}}],
    }


async def test_stream_auto_resumes_non_approval_interrupt(client):
    mount_agent(["dangerous_tool"])
    response = await client.post(
        "/runs/stream", json={"input": "count rows", "thread_id": "s-3"}
    )
    events = parse_sse(response.text)
    assert [e["event"] for e in events] == ["step", "done"]
    assert events[-1]["output"] == "answer"
