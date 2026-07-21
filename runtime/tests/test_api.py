"""HTTP API tests with a fake agent — no lifespan, no network, no real model."""

import httpx
import pytest
from langchain_core.messages import AIMessage, HumanMessage, ToolMessage

from agentos_runtime.api import app


class FakeAgent:
    def __init__(self, result=None, error=None):
        self.result = result
        self.error = error
        self.calls = []

    async def ainvoke(self, state, config=None):
        self.calls.append((state, config))
        if self.error is not None:
            raise self.error
        return self.result


def agent_result(output: str = "The answer is 1.") -> dict:
    return {
        "messages": [
            HumanMessage(content="how many rows?"),
            AIMessage(
                content="",
                tool_calls=[
                    {
                        "name": "query",
                        "args": {"sql": "SELECT count(*) FROM customers"},
                        "id": "call_1",
                        "type": "tool_call",
                    }
                ],
            ),
            ToolMessage(content='{"row_count": 1}', tool_call_id="call_1"),
            AIMessage(content=output),
        ]
    }


@pytest.fixture
def fake_agent():
    agent = FakeAgent(result=agent_result())
    app.state.agent = agent
    yield agent
    del app.state.agent


@pytest.fixture
async def client(fake_agent):
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as c:
        yield c


async def test_healthz(client):
    response = await client.get("/healthz")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


async def test_runs_generates_thread_id_and_returns_steps(client, fake_agent):
    response = await client.post("/runs", json={"input": "how many rows?"})
    assert response.status_code == 200
    body = response.json()
    assert set(body) == {"thread_id", "output", "steps", "status"}
    assert body["status"] == "completed"  # additive Phase 2 field
    assert len(body["thread_id"]) == 32
    int(body["thread_id"], 16)  # uuid4 hex
    assert body["output"] == "The answer is 1."
    assert body["steps"] == [
        {"tool": "query", "input": {"sql": "SELECT count(*) FROM customers"}}
    ]
    state, config = fake_agent.calls[0]
    assert state == {"messages": [{"role": "user", "content": "how many rows?"}]}
    assert config == {"configurable": {"thread_id": body["thread_id"]}}


async def test_runs_thread_id_passthrough(client, fake_agent):
    response = await client.post(
        "/runs", json={"input": "again", "thread_id": "my-thread"}
    )
    assert response.status_code == 200
    assert response.json()["thread_id"] == "my-thread"
    _, config = fake_agent.calls[0]
    assert config == {"configurable": {"thread_id": "my-thread"}}


async def test_runs_agent_error_returns_502(client, fake_agent):
    fake_agent.error = RuntimeError("provider exploded")
    response = await client.post("/runs", json={"input": "boom"})
    assert response.status_code == 502
    assert response.json() == {"detail": "provider exploded"}


async def test_runs_missing_input_is_422(client):
    response = await client.post("/runs", json={})
    assert response.status_code == 422


async def test_runs_without_agent_is_503():
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as c:
        response = await c.post("/runs", json={"input": "hi"})
    assert response.status_code == 503
