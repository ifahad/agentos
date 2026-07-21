"""Eval suite loading, scoring, and /evals endpoints — fake agent + in-memory store."""

import httpx
import pytest
from helpers import JUDGE_PASS_REPLY, FakeJudge, InMemoryImprovementStore, ScriptedAgent
from langchain_core.messages import AIMessage, HumanMessage

from agentos_runtime.api import app
from agentos_runtime.evals import (
    EvalCase,
    JudgeSpec,
    load_suite,
    parse_judge_reply,
    score_case,
)
from agentos_runtime.hitl import RunOutcome

STATE_ATTRS = (
    "improve_store",
    "agent_builder",
    "agent",
    "approval_tools",
    "reflection_model",
    "judge_model",
    "current_prompt",
)

# Answers the shipped default suite perfectly.
PERFECT_SCRIPT = {
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

JUDGED_CASE = "top-customer-cited-judged"

CASE_KEYS = {"name", "passed", "output_snippet", "judge_score", "judge_justification"}


@pytest.fixture
async def client():
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as c:
        yield c
    for attr in STATE_ATTRS:
        if hasattr(app.state, attr):
            delattr(app.state, attr)


def mount(agent=None, builder=None, approval_tools: list[str] | None = None, judge=None):
    store = InMemoryImprovementStore()
    app.state.improve_store = store
    app.state.agent = agent or ScriptedAgent(PERFECT_SCRIPT)
    app.state.agent_builder = builder or (lambda prompt: ScriptedAgent(PERFECT_SCRIPT))
    app.state.approval_tools = approval_tools or []
    app.state.judge_model = judge or FakeJudge(JUDGE_PASS_REPLY)
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


def test_load_default_suite_ships_five_cases():
    cases = load_suite("default")
    assert [case.name for case in cases] == [
        "top-customer-by-order-value",
        "pending-orders-count",
        "procurement-sign-off",
        "arithmetic-sanity",
        JUDGED_CASE,
    ]
    assert [case.expect_tool for case in cases] == [
        "query",
        None,
        "search_knowledge",
        None,
        "query",
    ]
    assert [case.expect_substring for case in cases] == [
        "al-faisal",
        "4",
        "al-harbi",
        "42",
        "al-faisal",
    ]
    assert all(case.input for case in cases)
    # exactly one judged case; the judge block round-trips from YAML
    assert [case.judge for case in cases[:4]] == [None, None, None, None]
    judged = cases[4].judge
    assert judged is not None
    assert judged.threshold == 0.7
    assert "Al-Faisal" in judged.criteria
    assert "table" in judged.criteria


def test_load_suite_judge_block_round_trips(tmp_path):
    (tmp_path / "custom.yaml").write_text(
        "cases:\n"
        "  - name: judged\n"
        "    input: q\n"
        '    expect_substring: "x"\n'
        "    judge:\n"
        "      criteria: cites the source table\n"
        "      threshold: 0.5\n"
    )
    cases = load_suite("custom", evals_dir=tmp_path)
    assert cases == [
        EvalCase(
            name="judged",
            input="q",
            expect_substring="x",
            expect_tool=None,
            judge=JudgeSpec(criteria="cites the source table", threshold=0.5),
        )
    ]


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
    assert (body["passed"], body["failed"]) == (5, 0)
    assert all(set(case) == CASE_KEYS for case in body["cases"])
    assert len(store.eval_runs) == 1
    assert store.eval_runs[0]["prompt_source"] == "active"
    assert store.eval_runs[0]["run_id"] == body["run_id"]


async def test_evals_run_scores_partial_failure(client):
    script = dict(PERFECT_SCRIPT)
    script["procurement policy"] = ("Al-Harbi signs off.", [])  # right text, no tool
    mount(agent=ScriptedAgent(script))
    body = (await client.post("/evals/run", json={"suite": "default"})).json()
    assert body["score"] == 0.8
    assert (body["passed"], body["failed"]) == (4, 1)
    failed = [case for case in body["cases"] if not case["passed"]]
    assert [case["name"] for case in failed] == ["procurement-sign-off"]


async def test_evals_run_stuck_pending_approval_counts_as_failed(client):
    stuck = ScriptedAgent({"": ("", ["query"])}, stuck=True)
    judge = FakeJudge(RuntimeError("judge must not run on stuck cases"))
    mount(agent=stuck, approval_tools=["query"], judge=judge)
    body = (await client.post("/evals/run", json={"suite": "default"})).json()
    assert body["score"] == 0.0
    assert body["failed"] == 5
    assert all("pending_approval" in case["output_snippet"] for case in body["cases"])
    assert judge.prompts == []


def judged_result(body: dict) -> dict:
    return next(case for case in body["cases"] if case["name"] == JUDGED_CASE)


def test_parse_judge_reply_tolerates_fences_and_prose():
    verdict = '{"score": 0.85, "justification": "cites the orders table"}'
    assert parse_judge_reply(verdict) == (0.85, "cites the orders table")
    assert parse_judge_reply(f"```json\n{verdict}\n```") == (0.85, "cites the orders table")
    prose = f"Here is my grading:\n{verdict}\nHope that helps."
    assert parse_judge_reply(prose) == (0.85, "cites the orders table")
    # integer scores and a missing justification are tolerated
    assert parse_judge_reply('{"score": 1}') == (1.0, "")


def test_parse_judge_reply_rejects_garbage_and_bad_scores():
    assert parse_judge_reply("not json at all") is None
    assert parse_judge_reply('{"justification": "no score"}') is None
    assert parse_judge_reply('{"score": "high"}') is None
    assert parse_judge_reply('{"score": true}') is None
    assert parse_judge_reply('{"score": 1.5}') is None
    assert parse_judge_reply('{"score": -0.1}') is None


async def test_evals_run_judged_case_passes_at_threshold(client):
    judge = FakeJudge('{"score": 0.7, "justification": "names customer and table"}')
    mount(judge=judge)
    body = (await client.post("/evals/run", json={"suite": "default"})).json()
    assert body["score"] == 1.0
    judged = judged_result(body)
    assert judged["passed"]
    assert judged["judge_score"] == 0.7
    assert judged["judge_justification"] == "names customer and table"
    # only the judged case consulted the judge, with input+output+criteria
    assert len(judge.prompts) == 1
    assert "most revenue" in judge.prompts[0]
    assert "orders" in judge.prompts[0]
    assert "Al-Faisal Trading Co." in judge.prompts[0]
    # unjudged cases carry null judge fields
    others = [case for case in body["cases"] if case["name"] != JUDGED_CASE]
    assert all(case["judge_score"] is None for case in others)
    assert all(case["judge_justification"] is None for case in others)


async def test_evals_run_judged_case_fails_below_threshold(client):
    judge = FakeJudge('{"score": 0.2, "justification": "does not cite a table"}')
    mount(judge=judge)
    body = (await client.post("/evals/run", json={"suite": "default"})).json()
    assert body["score"] == 0.8
    assert (body["passed"], body["failed"]) == (4, 1)
    judged = judged_result(body)
    assert not judged["passed"]
    assert judged["judge_score"] == 0.2
    assert judged["judge_justification"] == "does not cite a table"


async def test_evals_run_use_judge_false_bypasses_judge(client):
    judge = FakeJudge(RuntimeError("judge must not be called"))
    mount(judge=judge)
    body = (
        await client.post("/evals/run", json={"suite": "default", "use_judge": False})
    ).json()
    assert body["score"] == 1.0
    judged = judged_result(body)
    assert judged["passed"]  # substring/tool checks only
    assert judged["judge_score"] is None
    assert judged["judge_justification"] is None
    assert judge.prompts == []


async def test_evals_run_judge_failure_fails_case_not_suite(client):
    judge = FakeJudge(RuntimeError("gateway down"))  # both attempts fail
    mount(judge=judge)
    response = await client.post("/evals/run", json={"suite": "default"})
    assert response.status_code == 200  # the suite still completes
    body = response.json()
    assert body["score"] == 0.8
    judged = judged_result(body)
    assert not judged["passed"]
    assert judged["judge_score"] is None
    assert judged["judge_justification"] == "judge unavailable"
    assert len(judge.prompts) == 2  # one retry


async def test_evals_run_judge_retries_once_on_unparseable_reply(client):
    judge = FakeJudge("complete garbage", f"```json\n{JUDGE_PASS_REPLY}\n```")
    mount(judge=judge)
    body = (await client.post("/evals/run", json={"suite": "default"})).json()
    assert body["score"] == 1.0
    assert judged_result(body)["judge_score"] == 0.9
    assert len(judge.prompts) == 2


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
