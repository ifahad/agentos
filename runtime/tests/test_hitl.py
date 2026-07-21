"""HITL flow tests: interrupt -> 202 -> approve/deny, auto-resume, /threads."""

import httpx
import pytest
from helpers import QUERY_RESULT, lookup, make_settings, query, query_then_answer
from langgraph.checkpoint.memory import InMemorySaver

from agentos_runtime.agent import build_agent
from agentos_runtime.api import app


def mount_agent(approval_tools: list[str], model=None):
    settings = make_settings(approval_tools=",".join(approval_tools))
    agent = build_agent(
        settings, [query, lookup], InMemorySaver(), model=model or query_then_answer()
    )
    app.state.agent = agent
    app.state.approval_tools = settings.approval_tool_names
    return agent


@pytest.fixture
async def client():
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as c:
        yield c
    for attr in ("agent", "approval_tools"):
        if hasattr(app.state, attr):
            delattr(app.state, attr)


async def test_approval_tool_interrupts_with_202(client):
    mount_agent(["query"])
    response = await client.post("/runs", json={"input": "count rows", "thread_id": "t-1"})
    assert response.status_code == 202
    assert response.json() == {
        "status": "pending_approval",
        "thread_id": "t-1",
        "pending": [{"tool": "query", "input": {"sql": "SELECT 1"}}],
    }


async def test_approve_true_resumes_to_completion(client):
    mount_agent(["query"])
    await client.post("/runs", json={"input": "count rows", "thread_id": "t-2"})
    response = await client.post("/runs/t-2/approve", json={"approve": True})
    assert response.status_code == 200
    body = response.json()
    assert body["status"] == "completed"
    assert body["thread_id"] == "t-2"
    assert body["output"] == "answer"
    assert body["steps"] == [{"tool": "query", "input": {"sql": "SELECT 1"}}]
    # the tool actually executed
    thread = (await client.get("/threads/t-2")).json()
    tool_messages = [m for m in thread["messages"] if m["role"] == "tool"]
    assert len(tool_messages) == 1
    assert QUERY_RESULT in tool_messages[0]["content"]


async def test_approve_false_injects_denial(client):
    mount_agent(["query"])
    await client.post("/runs", json={"input": "count rows", "thread_id": "t-3"})
    response = await client.post("/runs/t-3/approve", json={"approve": False})
    assert response.status_code == 200
    assert response.json()["status"] == "completed"
    thread = (await client.get("/threads/t-3")).json()
    tool_messages = [m for m in thread["messages"] if m["role"] == "tool"]
    assert tool_messages == [{"role": "tool", "content": "Denied by human reviewer."}]


async def test_non_approval_tool_auto_resumes(client):
    # HITL enabled (interrupt_before compiled in), but the called tool is not
    # in the approval list -> the run loop auto-resumes to completion.
    mount_agent(["dangerous_tool"])
    response = await client.post("/runs", json={"input": "count rows", "thread_id": "t-4"})
    assert response.status_code == 200
    body = response.json()
    assert body["status"] == "completed"
    assert body["output"] == "answer"
    assert body["steps"] == [{"tool": "query", "input": {"sql": "SELECT 1"}}]


async def test_approve_unknown_thread_404(client):
    mount_agent(["query"])
    response = await client.post("/runs/nope/approve", json={"approve": True})
    assert response.status_code == 404


async def test_approve_completed_thread_404(client):
    mount_agent(["dangerous_tool"])
    await client.post("/runs", json={"input": "count rows", "thread_id": "t-5"})
    response = await client.post("/runs/t-5/approve", json={"approve": True})
    assert response.status_code == 404


async def test_threads_shape_and_404(client):
    mount_agent([])
    await client.post("/runs", json={"input": "count rows", "thread_id": "t-6"})
    response = await client.get("/threads/t-6")
    assert response.status_code == 200
    body = response.json()
    assert body["thread_id"] == "t-6"
    roles = [m["role"] for m in body["messages"]]
    assert roles == ["user", "assistant", "tool", "assistant"]
    assert body["messages"][0]["content"] == "count rows"
    assert body["messages"][1]["tool_calls"] == [
        {"tool": "query", "input": {"sql": "SELECT 1"}}
    ]
    assert "tool_calls" not in body["messages"][3]
    assert body["messages"][3]["content"] == "answer"
    assert (await client.get("/threads/unknown")).status_code == 404
