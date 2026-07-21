"""Eval suite loading, scoring, and /evals endpoints — fake agent + in-memory store."""

import httpx
import pytest
from helpers import InMemoryImprovementStore, ScriptedAgent
from langchain_core.messages import AIMessage, HumanMessage

from agentos_runtime.api import app
from agentos_runtime.evals import EvalCase, load_suite, score_case
from agentos_runtime.hitl import RunOutcome

STATE_ATTRS = (
    "improve_store",
    "agent_builder",
    "agent",
    "approval_tools",
    "reflection_model",
    "current_prompt",
)

# Answers the shipped default suite perfectly.
PERFECT_SCRIPT = {
    "highest total order": ("The top customer is Al-Faisal Trading Co.", ["query"]),
    "pending status": ("There are 4 pending orders.", ["query"]),
    "procurement policy": ("CFO Layla Al-Harbi signs off.", ["search_knowledge"]),
    "6 times 7": ("42", []),
}


@pytest.fixture
async def client():
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as c:
        yield c
    for attr in STATE_ATTRS:
        if hasattr(app.state, attr):
            delattr(app.state, attr)


def mount(agent=None, builder=None, approval_tools: list[str] | None = None):
    store = InMemoryImprovementStore()
    app.state.improve_store = store
    app.state.agent = agent or ScriptedAgent(PERFECT_SCRIPT)
    app.state.agent_builder = builder or (lambda prompt: ScriptedAgent(PERFECT_SCRIPT))
    app.state.approval_tools = approval_tools or []
    return store


def completed_outcome(output: str, tools: list[str] | None = None) -> RunOutcome:
    messages = [HumanMessage(content="q")]
    if tools:
        messages.append(
            AIMessage(
                content="",
                tool_calls=[
                    {"name": name, "args": {}, "id": f"c{i}", "type": "tool_call"}
                    for i, name in enumerate(tools)
                ],
            )
        )
    messages.append(AIMessage(content=output))
    return RunOutcome(status="completed", values={"messages": messages}, pending=[])


def test_load_default_suite_ships_four_cases():
    cases = load_suite("default")
    assert [case.name for case in cases] == [
        "top-customer-by-order-value",
        "pending-orders-count",
        "procurement-sign-off",
        "arithmetic-sanity",
    ]
    assert [case.expect_tool for case in cases] == ["query", None, "search_knowledge", None]
    assert [case.expect_substring for case in cases] == ["al-faisal", "4", "al-harbi", "42"]
    assert all(case.input for case in cases)


def test_load_unknown_suite_raises():
    with pytest.raises(FileNotFoundError):
        load_suite("no-such-suite")


def test_score_case_substring_is_case_insensitive():
    case = EvalCase(name="c", input="q", expect_substring="al-faisal")
    passed, snippet = score_case(case, completed_outcome("Top customer: AL-FAISAL Trading."))
    assert passed
    assert snippet == "Top customer: AL-FAISAL Trading."
    assert not score_case(case, completed_outcome("Top customer: Gulf Retail."))[0]


def test_score_case_requires_expected_tool():
    case = EvalCase(name="c", input="q", expect_substring="42", expect_tool="query")
    assert not score_case(case, completed_outcome("The answer is 42."))[0]
    assert score_case(case, completed_outcome("The answer is 42.", tools=["query"]))[0]


def test_score_case_pending_approval_fails():
    case = EvalCase(name="c", input="q", expect_substring="42")
    outcome = RunOutcome(status="pending_approval", values={"messages": []}, pending=[])
    passed, snippet = score_case(case, outcome)
    assert not passed
    assert "pending_approval" in snippet


async def test_evals_run_response_shape_and_persistence(client):
    store = mount()
    response = await client.post("/evals/run", json={"suite": "default"})
    assert response.status_code == 200
    body = response.json()
    assert set(body) == {"run_id", "suite", "score", "passed", "failed", "cases"}
    assert body["suite"] == "default"
    assert body["score"] == 1.0
    assert (body["passed"], body["failed"]) == (4, 0)
    assert all(set(case) == {"name", "passed", "output_snippet"} for case in body["cases"])
    assert len(store.eval_runs) == 1
    assert store.eval_runs[0]["prompt_source"] == "active"
    assert store.eval_runs[0]["run_id"] == body["run_id"]


async def test_evals_run_scores_partial_failure(client):
    script = dict(PERFECT_SCRIPT)
    script["procurement policy"] = ("Al-Harbi signs off.", [])  # right text, no tool
    mount(agent=ScriptedAgent(script))
    body = (await client.post("/evals/run", json={"suite": "default"})).json()
    assert body["score"] == 0.75
    assert (body["passed"], body["failed"]) == (3, 1)
    failed = [case for case in body["cases"] if not case["passed"]]
    assert [case["name"] for case in failed] == ["procurement-sign-off"]


async def test_evals_run_stuck_pending_approval_counts_as_failed(client):
    stuck = ScriptedAgent({"": ("", ["query"])}, stuck=True)
    mount(agent=stuck, approval_tools=["query"])
    body = (await client.post("/evals/run", json={"suite": "default"})).json()
    assert body["score"] == 0.0
    assert body["failed"] == 4
    assert all("pending_approval" in case["output_snippet"] for case in body["cases"])


async def test_evals_run_prompt_override_builds_temp_agent(client):
    built_prompts: list[str] = []

    def builder(prompt):
        built_prompts.append(prompt)
        return ScriptedAgent(PERFECT_SCRIPT)

    store = mount(agent=ScriptedAgent({"": ("never used", [])}), builder=builder)
    response = await client.post(
        "/evals/run", json={"suite": "default", "prompt_override": "candidate prompt"}
    )
    assert response.status_code == 200
    assert response.json()["score"] == 1.0
    assert built_prompts == ["candidate prompt"]
    assert store.eval_runs[0]["prompt_source"] == "override"


async def test_evals_run_unknown_suite_404(client):
    mount()
    response = await client.post("/evals/run", json={"suite": "nope"})
    assert response.status_code == 404


async def test_evals_runs_lists_recent_without_cases(client):
    store = mount()
    await store.insert_eval_run("default", 0.5, 2, 2, [{"name": "x"}], "active")
    await store.insert_eval_run("default", 1.0, 4, 0, [{"name": "y"}], "candidate")
    response = await client.get("/evals/runs")
    assert response.status_code == 200
    runs = response.json()
    assert [run["score"] for run in runs] == [1.0, 0.5]  # newest first
    assert all("cases" not in run for run in runs)
    assert (await client.get("/evals/runs?limit=1")).json() == runs[:1]


async def test_self_improvement_endpoints_503_without_checkpoint_db(client):
    # no improve_store mounted -> every self-improvement endpoint refuses
    assert (await client.post("/evals/run", json={})).status_code == 503
    assert (await client.get("/evals/runs")).status_code == 503
    assert (await client.post("/improve", json={})).status_code == 503
    assert (await client.get("/proposals")).status_code == 503
    assert (await client.post("/proposals/p1/approve", json={"approve": True})).status_code == 503
    assert (await client.get("/prompts/active")).status_code == 503
