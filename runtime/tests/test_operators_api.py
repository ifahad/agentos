"""Operators HTTP API — mounted app with a FakeOperatorStore, no lifespan."""

from types import SimpleNamespace

import httpx
import pytest
from helpers import AUTH_HEADERS, FakeOperatorStore
from langchain_core.messages import AIMessage

from agentos_runtime.api import app
from agentos_runtime.operators.engine import OperatorDeps


class StubAgent:
    async def ainvoke(self, input_state, config=None):
        return {"messages": [AIMessage(content="ran", tool_calls=[])]}

    async def aget_state(self, config):
        return SimpleNamespace(next=(), values={"messages": []})


def mount_operators(store):
    deps = OperatorDeps(
        store=store, agent_builder=lambda prompt: StubAgent(), approval_tools=[],
    )
    app.state.operators = SimpleNamespace(store=store, deps=deps)


@pytest.fixture
async def store():
    return FakeOperatorStore()


@pytest.fixture
async def client(store):
    mount_operators(store)
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(
        transport=transport, base_url="http://test", headers=AUTH_HEADERS
    ) as c:
        yield c
    if hasattr(app.state, "operators"):
        del app.state.operators


@pytest.fixture
async def client_no_auth(store):
    mount_operators(store)
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as c:
        yield c
    if hasattr(app.state, "operators"):
        del app.state.operators


@pytest.fixture
async def client_no_store():
    app.state.operators = None
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(
        transport=transport, base_url="http://test", headers=AUTH_HEADERS
    ) as c:
        yield c


async def test_create_interval_operator(client):
    resp = await client.post(
        "/operators",
        json={"name": "nightly", "goal": "summarise invoices",
              "trigger": {"type": "interval", "interval_s": 60}},
    )
    assert resp.status_code == 201
    body = resp.json()
    assert body["name"] == "nightly"
    assert body["trigger"] == {"type": "interval", "interval_s": 60}


async def test_create_rejects_bad_trigger(client):
    resp = await client.post(
        "/operators",
        json={"name": "x", "goal": "g", "trigger": {"type": "interval", "interval_s": 1}},
    )
    assert resp.status_code == 400


async def test_create_webhook_returns_token_once_then_redacts(client):
    created = (await client.post(
        "/operators",
        json={"name": "hook", "goal": "g", "trigger": {"type": "webhook"}},
    )).json()
    token = created["trigger"]["webhook_token"]
    assert token and token.startswith("whk-")
    # The listing must not leak the token.
    listed = (await client.get("/operators")).json()["operators"]
    assert listed[0]["trigger"]["webhook_token"] is None


async def test_operators_require_auth(client_no_auth):
    for method, path in [("post", "/operators"), ("get", "/operators")]:
        m = getattr(client_no_auth, method)
        resp = await (m(path, json={}) if method == "post" else m(path))
        assert resp.status_code == 401, f"{method} {path} = {resp.status_code}"


async def test_run_now_fires_and_records(client, store):
    op = (await client.post(
        "/operators",
        json={"name": "x", "goal": "g", "trigger": {"type": "interval", "interval_s": 60}},
    )).json()
    resp = await client.post(f"/operators/{op['id']}/run")
    assert resp.status_code == 200
    run = resp.json()["run"]
    assert run["status"] == "completed"
    assert run["trigger_source"] == "manual"
    assert len(await store.list_runs(op["id"])) == 1


async def test_get_includes_recent_runs(client):
    op = (await client.post(
        "/operators",
        json={"name": "x", "goal": "g", "trigger": {"type": "interval", "interval_s": 60}},
    )).json()
    await client.post(f"/operators/{op['id']}/run")
    body = (await client.get(f"/operators/{op['id']}")).json()
    assert body["operator"]["id"] == op["id"]
    assert len(body["recent_runs"]) == 1


async def test_patch_pauses_and_resumes(client):
    op = (await client.post(
        "/operators",
        json={"name": "x", "goal": "g", "trigger": {"type": "interval", "interval_s": 60}},
    )).json()
    paused = (await client.patch(f"/operators/{op['id']}", json={"enabled": False})).json()
    assert paused["operator"]["enabled"] is False


async def test_delete_operator(client):
    op = (await client.post(
        "/operators",
        json={"name": "x", "goal": "g", "trigger": {"type": "interval", "interval_s": 60}},
    )).json()
    assert (await client.delete(f"/operators/{op['id']}")).status_code == 204
    assert (await client.get(f"/operators/{op['id']}")).status_code == 404


async def test_webhook_fires_by_token_without_bearer(client_no_auth, store):
    # Create needs auth, so seed the store directly.
    op = await store.create_operator(
        "hook", "react to orders", "webhook", {"webhook_token": "whk-live"}, True, 8
    )
    # No Authorization header on client_no_auth — the token is the credential.
    resp = await client_no_auth.post("/operators/webhooks/whk-live", json={"order_id": 7})
    assert resp.status_code == 200
    assert resp.json()["run"]["trigger_source"] == "webhook"
    assert len(await store.list_runs(op["id"])) == 1


async def test_unknown_webhook_is_404(client_no_auth):
    resp = await client_no_auth.post("/operators/webhooks/whk-nope", json={})
    assert resp.status_code == 404


async def test_operators_503_without_a_store(client_no_store):
    resp = await client_no_store.post(
        "/operators",
        json={"name": "x", "goal": "g", "trigger": {"type": "interval", "interval_s": 60}},
    )
    assert resp.status_code == 503
