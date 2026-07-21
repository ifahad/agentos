"""POST /improve flow — fake reflection model, scripted agents, in-memory store."""

import httpx
import pytest
from helpers import (
    JUDGE_PASS_REPLY,
    FakeChatModel,
    FakeJudge,
    InMemoryImprovementStore,
    ScriptedAgent,
)

from agentos_runtime.agent import SYSTEM_PROMPT
from agentos_runtime.api import app
from agentos_runtime.improve import parse_reflection

STATE_ATTRS = (
    "improve_store",
    "agent_builder",
    "agent",
    "approval_tools",
    "reflection_model",
    "judge_model",
    "current_prompt",
)

GOOD_SCRIPT = {
    "highest total order": ("The top customer is Al-Faisal Trading Co.", ["query"]),
    "pending status": ("There are 4 pending orders.", ["query"]),
    "procurement policy": ("CFO Layla Al-Harbi signs off.", ["search_knowledge"]),
    "6 times 7": ("42", []),
    "most revenue": (
        "Al-Faisal Trading Co. generates the most revenue, per the orders "
        "and customers tables.",
        ["query"],
    ),
}
# Fails procurement-sign-off: no search_knowledge tool used.
BASELINE_SCRIPT = {**GOOD_SCRIPT, "procurement policy": ("I could not find that.", [])}
BAD_SCRIPT = {"": ("no idea", [])}

REFLECTION_JSON = '{"prompt": "You are an improved analyst.", "rationale": "use the KB"}'


@pytest.fixture
async def client():
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as c:
        yield c
    for attr in STATE_ATTRS:
        if hasattr(app.state, attr):
            delattr(app.state, attr)


def mount(agent, candidate_agent, model):
    store = InMemoryImprovementStore()
    app.state.improve_store = store
    app.state.agent = agent
    app.state.agent_builder = lambda prompt: candidate_agent
    app.state.reflection_model = model
    app.state.judge_model = FakeJudge(JUDGE_PASS_REPLY)
    app.state.approval_tools = []
    return store


def test_parse_reflection_tolerates_fences_and_prose():
    fenced = f"```json\n{REFLECTION_JSON}\n```"
    assert parse_reflection(fenced) == ("You are an improved analyst.", "use the KB")
    prose = f"Here is my proposal:\n{REFLECTION_JSON}\nHope that helps."
    assert parse_reflection(prose) == ("You are an improved analyst.", "use the KB")


def test_parse_reflection_rejects_garbage_and_missing_keys():
    assert parse_reflection("not json at all") is None
    assert parse_reflection('{"rationale": "no prompt key"}') is None
    assert parse_reflection('{"prompt": "", "rationale": "empty prompt"}') is None
    assert parse_reflection('{"prompt": ["not a string"], "rationale": "x"}') is None


async def test_improve_happy_path_passes_evals(client):
    model = FakeChatModel([REFLECTION_JSON])
    store = mount(ScriptedAgent(BASELINE_SCRIPT), ScriptedAgent(GOOD_SCRIPT), model)
    response = await client.post("/improve", json={})
    assert response.status_code == 200
    proposal = response.json()
    assert proposal["status"] == "passed_evals"
    assert proposal["prompt_text"] == "You are an improved analyst."
    assert proposal["rationale"] == "use the KB"
    assert proposal["baseline_score"] == 0.8
    assert proposal["candidate_score"] == 1.0
    # baseline auto-ran (no prior runs), then the candidate eval ran
    assert [run["prompt_source"] for run in store.eval_runs] == ["active", "candidate"]
    # reflection prompt carried the current system prompt + failed case details
    reflection_prompt = model.prompts[0]
    assert SYSTEM_PROMPT in reflection_prompt
    assert "procurement-sign-off" in reflection_prompt
    assert "al-harbi" in reflection_prompt
    assert "search_knowledge" in reflection_prompt
    # nothing activates without human approval
    assert store.active is None
    assert store.proposals[proposal["id"]]["status"] == "passed_evals"


async def test_improve_below_baseline_is_failed_evals(client):
    store = mount(
        ScriptedAgent(GOOD_SCRIPT), ScriptedAgent(BAD_SCRIPT), FakeChatModel([REFLECTION_JSON])
    )
    # pre-seeded baseline: latest run is used, no auto-run happens
    await store.insert_eval_run(
        "default",
        1.0,
        5,
        0,
        [{"name": "arithmetic-sanity", "passed": True, "output_snippet": "42"}],
        "active",
    )
    proposal = (await client.post("/improve", json={})).json()
    assert proposal["status"] == "failed_evals"
    assert proposal["baseline_score"] == 1.0
    assert proposal["candidate_score"] == 0.0
    assert [run["prompt_source"] for run in store.eval_runs] == ["active", "candidate"]


async def test_improve_retries_once_on_unparseable_reply(client):
    model = FakeChatModel(["complete garbage", f"```json\n{REFLECTION_JSON}\n```"])
    mount(ScriptedAgent(GOOD_SCRIPT), ScriptedAgent(GOOD_SCRIPT), model)
    response = await client.post("/improve", json={})
    assert response.status_code == 200
    assert response.json()["status"] == "passed_evals"
    assert len(model.prompts) == 2


async def test_improve_502_when_still_unparseable_after_retry(client):
    store = mount(
        ScriptedAgent(GOOD_SCRIPT), ScriptedAgent(GOOD_SCRIPT), FakeChatModel(["nope", "nope"])
    )
    response = await client.post("/improve", json={})
    assert response.status_code == 502
    assert response.json() == {"detail": "unparseable reflection response"}
    assert store.proposals == {}


async def test_improve_502_when_model_keeps_failing(client):
    model = FakeChatModel([RuntimeError("gateway down"), RuntimeError("gateway down")])
    mount(ScriptedAgent(GOOD_SCRIPT), ScriptedAgent(GOOD_SCRIPT), model)
    response = await client.post("/improve", json={})
    assert response.status_code == 502
    assert response.json() == {"detail": "gateway down"}
