"""Prompt store endpoints: /prompts/active, /proposals, approve/deny + hot-swap."""

import httpx
import pytest
from helpers import FakeToolCallingModel, InMemoryImprovementStore, make_settings, query
from langchain_core.messages import AIMessage
from langgraph.checkpoint.memory import InMemorySaver
from pydantic import Field

from agentos_runtime.agent import SYSTEM_PROMPT, build_agent
from agentos_runtime.api import app
from agentos_runtime.prompts import BELOW_BASELINE_WARNING

STATE_ATTRS = (
    "improve_store",
    "agent_builder",
    "agent",
    "approval_tools",
    "reflection_model",
    "current_prompt",
)

NEW_PROMPT = "You are the improved analyst prompt."


class RecordingModel(FakeToolCallingModel):
    """Fake model that records the messages each generate call received."""

    seen: list = Field(default_factory=list)

    def _generate(self, messages, stop=None, run_manager=None, **kwargs):
        self.seen.append(list(messages))
        return super()._generate(messages, stop, run_manager, **kwargs)


@pytest.fixture
async def client():
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as c:
        yield c
    for attr in STATE_ATTRS:
        if hasattr(app.state, attr):
            delattr(app.state, attr)


def mount():
    """Store + a real agent_builder whose fake model records its system prompt."""
    store = InMemoryImprovementStore()
    models: list[RecordingModel] = []

    def builder(prompt):
        model = RecordingModel(responses=[AIMessage(content="ok")])
        models.append(model)
        return build_agent(make_settings(), [query], InMemorySaver(), model=model, prompt=prompt)

    app.state.improve_store = store
    app.state.agent_builder = builder
    app.state.agent = builder(None)
    app.state.approval_tools = []
    return store, models


async def seed_proposal(store, status="passed_evals", prompt_text=NEW_PROMPT):
    return await store.insert_proposal(
        prompt_text=prompt_text,
        rationale="better",
        baseline_score=0.75,
        candidate_score=1.0 if status == "passed_evals" else 0.5,
        status=status,
    )


async def test_active_prompt_defaults_to_system_prompt(client):
    mount()
    response = await client.get("/prompts/active")
    assert response.status_code == 200
    assert response.json() == {
        "source": "default",
        "prompt": SYSTEM_PROMPT,
        "proposal_id": None,
    }


async def test_approve_activates_prompt_and_hot_swaps_agent(client):
    store, models = mount()
    proposal = await seed_proposal(store)
    old_agent = app.state.agent

    response = await client.post(f"/proposals/{proposal['id']}/approve", json={"approve": True})
    assert response.status_code == 200
    body = response.json()
    assert body["status"] == "approved"
    assert "warning" not in body

    # /prompts/active flips to the proposal
    active = (await client.get("/prompts/active")).json()
    assert active == {
        "source": "proposal",
        "prompt": NEW_PROMPT,
        "proposal_id": proposal["id"],
    }
    assert store.proposals[proposal["id"]]["status"] == "approved"
    assert app.state.current_prompt == NEW_PROMPT

    # the live agent was rebuilt: a run through it now carries the new prompt
    assert app.state.agent is not old_agent
    await client.post("/runs", json={"input": "hi", "thread_id": "t-swap"})
    system_message = models[-1].seen[0][0]
    assert system_message.content == NEW_PROMPT


async def test_deny_sets_status_without_activation(client):
    store, _ = mount()
    proposal = await seed_proposal(store)
    old_agent = app.state.agent
    response = await client.post(f"/proposals/{proposal['id']}/approve", json={"approve": False})
    assert response.status_code == 200
    assert response.json()["status"] == "denied"
    assert store.proposals[proposal["id"]]["status"] == "denied"
    assert store.active is None
    assert app.state.agent is old_agent
    assert (await client.get("/prompts/active")).json()["source"] == "default"


async def test_approve_unknown_proposal_404(client):
    mount()
    response = await client.post("/proposals/missing/approve", json={"approve": True})
    assert response.status_code == 404


async def test_approve_already_decided_409(client):
    store, _ = mount()
    approved = await seed_proposal(store)
    await client.post(f"/proposals/{approved['id']}/approve", json={"approve": True})
    response = await client.post(f"/proposals/{approved['id']}/approve", json={"approve": True})
    assert response.status_code == 409

    denied = await seed_proposal(store)
    await client.post(f"/proposals/{denied['id']}/approve", json={"approve": False})
    response = await client.post(f"/proposals/{denied['id']}/approve", json={"approve": False})
    assert response.status_code == 409


async def test_approving_failed_evals_warns_but_activates(client):
    store, _ = mount()
    proposal = await seed_proposal(store, status="failed_evals")
    response = await client.post(f"/proposals/{proposal['id']}/approve", json={"approve": True})
    assert response.status_code == 200
    body = response.json()
    assert body["status"] == "approved"
    assert body["warning"] == BELOW_BASELINE_WARNING
    assert (await client.get("/prompts/active")).json()["prompt"] == NEW_PROMPT


async def test_proposals_listing_newest_first_with_limit(client):
    store, _ = mount()
    first = await seed_proposal(store, prompt_text="first")
    second = await seed_proposal(store, prompt_text="second")
    listed = (await client.get("/proposals")).json()
    assert [p["id"] for p in listed] == [second["id"], first["id"]]
    assert set(listed[0]) == {
        "id",
        "prompt_text",
        "rationale",
        "baseline_score",
        "candidate_score",
        "status",
        "created_at",
    }
    assert [p["id"] for p in (await client.get("/proposals?limit=1")).json()] == [second["id"]]
