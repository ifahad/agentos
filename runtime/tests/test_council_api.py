"""Council HTTP API — mounted app with a FakeCouncilStore, no lifespan."""

from types import SimpleNamespace

import httpx
import pytest
from helpers import AUTH_HEADERS, FakeCouncilStore

from agentos_runtime.api import app
from agentos_runtime.council.config import CouncilConfig, Member
from agentos_runtime.council.loop import CouncilDeps


def council_config():
    return CouncilConfig(
        judge="ollama/judge", quorum=1, agreement_threshold=0.6,
        max_cycles=3, max_tool_steps=12, member_timeout_s=5,
        members=[
            Member(id="alpha", model="ollama/a", enabled=True, profile="react"),
            Member(id="beta", model="moonshot/kimi-k3", enabled=False, profile="deep"),
        ],
    )


def mount_council(store):
    cfg = council_config()
    deps = CouncilDeps(
        config=cfg, settings=None, tools=[], checkpointer=None, store=store,
        judge_model=object(),
    )
    app.state.council = SimpleNamespace(
        config=cfg, store=store, deps=deps, default_budget_usd=5.0,
    )


@pytest.fixture
async def council_store():
    return FakeCouncilStore()


@pytest.fixture
async def council_client(council_store):
    mount_council(council_store)
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(
        transport=transport, base_url="http://test", headers=AUTH_HEADERS
    ) as c:
        yield c
    if hasattr(app.state, "council"):
        del app.state.council


@pytest.fixture
async def council_client_no_auth(council_store):
    mount_council(council_store)
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as c:
        yield c
    if hasattr(app.state, "council"):
        del app.state.council


@pytest.fixture
async def council_client_no_store():
    app.state.council = None
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(
        transport=transport, base_url="http://test", headers=AUTH_HEADERS
    ) as c:
        yield c


async def test_create_objective_returns_202_and_an_id(council_client):
    resp = await council_client.post("/council/objectives", json={"input": "audit invoices"})
    assert resp.status_code == 202
    body = resp.json()
    assert body["id"]
    assert body["status"] == "pending"


async def test_create_objective_requires_input(council_client):
    resp = await council_client.post("/council/objectives", json={})
    assert resp.status_code == 422


async def test_council_routes_require_auth(council_client_no_auth):
    for method, path in [
        ("post", "/council/objectives"),
        ("get", "/council/objectives"),
        ("get", "/council/members"),
        ("post", "/council/pause"),
        ("get", "/council/proposals"),
    ]:
        client_method = getattr(council_client_no_auth, method)
        resp = await (
            client_method(path, json={}) if method == "post" else client_method(path)
        )
        assert resp.status_code == 401, f"{method} {path} = {resp.status_code}"


async def test_get_objective_includes_cycles(council_client, council_store):
    created = (await council_client.post("/council/objectives", json={"input": "q"})).json()
    cycle_id = await council_store.insert_cycle(
        created["id"], 1, {"answer": "a"}, 0.8,
        [{"member": "beta", "claim": "c", "basis": "b"}],
    )
    await council_store.insert_member_run(
        cycle_id, "alpha", "ollama/a", "t", "answered", "a", [], 0.0, ""
    )
    body = (await council_client.get(f"/council/objectives/{created['id']}")).json()
    assert body["objective"]["id"] == created["id"]
    assert len(body["cycles"]) == 1
    assert body["cycles"][0]["agreement"] == 0.8
    assert body["cycles"][0]["dissent"][0]["member"] == "beta"


async def test_get_unknown_objective_404s(council_client):
    resp = await council_client.get("/council/objectives/nope")
    assert resp.status_code == 404


async def test_cancel_marks_the_objective_cancelled(council_client):
    created = (await council_client.post("/council/objectives", json={"input": "q"})).json()
    resp = await council_client.post(f"/council/objectives/{created['id']}/cancel")
    assert resp.status_code == 200
    body = (await council_client.get(f"/council/objectives/{created['id']}")).json()
    assert body["objective"]["status"] == "cancelled"


async def test_pause_and_resume_toggle_the_kill_switch(council_client):
    assert (await council_client.post("/council/pause")).json()["paused"] is True
    assert (await council_client.get("/council/members")).status_code == 200
    assert (await council_client.post("/council/resume")).json()["paused"] is False


async def test_members_lists_configured_members_without_secrets(council_client):
    body = (await council_client.get("/council/members")).json()
    assert body["members"]
    for member in body["members"]:
        assert {"id", "model", "enabled", "profile"} <= set(member)
        assert "api_key" not in member


async def test_approve_proposal_marks_it_approved(council_client, council_store):
    created = (await council_client.post("/council/objectives", json={"input": "q"})).json()
    pid = await council_store.insert_proposal(created["id"], "alpha", "delete_customer", {"id": 7})
    resp = await council_client.post(f"/council/proposals/{pid}/approve")
    assert resp.status_code == 200
    assert resp.json()["status"] == "approved"


async def test_approve_unknown_proposal_404s(council_client):
    resp = await council_client.post("/council/proposals/nope/approve")
    assert resp.status_code == 404


async def test_council_endpoints_503_without_a_store(council_client_no_store):
    """No council configured means every route reports 503, not a crash."""
    resp = await council_client_no_store.post("/council/objectives", json={"input": "q"})
    assert resp.status_code == 503
