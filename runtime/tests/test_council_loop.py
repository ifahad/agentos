"""Council loop: cycle execution and every stop condition."""

from helpers import FakeCouncilStore

from agentos_runtime.council.config import CouncilConfig, Member
from agentos_runtime.council.fanout import MemberAnswer
from agentos_runtime.council.judge import JUDGE_UNAVAILABLE, Verdict
from agentos_runtime.council.loop import (
    STOP_BUDGET,
    STOP_CONVERGED,
    STOP_MAX_CYCLES,
    STOP_NEEDS_REVIEW,
    STOP_PAUSED,
    CouncilDeps,
    decide_stop,
    run_objective,
)


def config(**overrides):
    base = dict(
        judge="ollama/judge", quorum=2, agreement_threshold=0.6,
        max_cycles=3, max_tool_steps=12, member_timeout_s=5,
        members=[
            Member(id="alpha", model="ollama/a", enabled=True),
            Member(id="beta", model="ollama/b", enabled=True),
        ],
    )
    base.update(overrides)
    return CouncilConfig(**base)


def verdict(agreement=1.0, done=False, status="ok"):
    return Verdict(answer="a", agreement=agreement, dissent=[],
                   cited_members=["alpha"], done=done, status=status)


def test_decide_stop_converges_on_threshold():
    assert decide_stop(verdict(agreement=0.8), 1, 3, 0.0, 5.0, 0.6, True) == STOP_CONVERGED


def test_decide_stop_converges_when_judge_says_done():
    v = verdict(agreement=0.1, done=True)
    assert decide_stop(v, 1, 3, 0.0, 5.0, 0.6, True) == STOP_CONVERGED


def test_decide_stop_continues_below_threshold_with_cycles_left():
    assert decide_stop(verdict(agreement=0.5), 1, 3, 0.0, 5.0, 0.6, True) is None


def test_decide_stop_max_cycles_on_final_cycle_below_threshold():
    """Out of cycles without agreement stops as max_cycles (a review reason)."""
    assert decide_stop(verdict(agreement=0.5), 3, 3, 0.0, 5.0, 0.6, True) == STOP_MAX_CYCLES


def test_decide_stop_needs_review_when_quorum_failed():
    assert decide_stop(verdict(), 1, 3, 0.0, 5.0, 0.6, False) == STOP_NEEDS_REVIEW


def test_decide_stop_needs_review_when_judge_unavailable():
    v = verdict(status=JUDGE_UNAVAILABLE)
    assert decide_stop(v, 1, 3, 0.0, 5.0, 0.6, True) == STOP_NEEDS_REVIEW


def test_decide_stop_budget_beats_continuing():
    assert decide_stop(verdict(agreement=0.1), 1, 3, 5.5, 5.0, 0.6, True) == STOP_BUDGET


async def test_run_objective_stops_at_max_cycles():
    store = FakeCouncilStore()
    obj = await store.create_objective("q", budget_usd=100.0)
    calls = []

    async def fake_fanout(**kwargs):
        calls.append(kwargs["cycle_no"])
        return [
            MemberAnswer("alpha", "ollama/a", "t", "answered", "x", [], ""),
            MemberAnswer("beta", "ollama/b", "t", "answered", "y", [], ""),
        ]

    async def fake_synthesize(*args, **kwargs):
        return verdict(agreement=0.1)  # never converges

    deps = CouncilDeps(
        config=config(), settings=None, tools=[], checkpointer=None, store=store,
        judge_model=object(), fanout_fn=fake_fanout, synthesize_fn=fake_synthesize,
    )
    reason = await run_objective(deps, obj)
    assert reason == STOP_MAX_CYCLES
    assert calls == [1, 2, 3]
    assert (await store.get_objective(obj["id"]))["cycles_run"] == 3


async def test_run_objective_stops_when_paused():
    store = FakeCouncilStore()
    obj = await store.create_objective("q")
    await store.set_paused(True)

    async def fake_fanout(**kwargs):
        raise AssertionError("must not fan out while paused")

    deps = CouncilDeps(
        config=config(), settings=None, tools=[], checkpointer=None, store=store,
        judge_model=object(), fanout_fn=fake_fanout,
        synthesize_fn=lambda *a, **k: verdict(),
    )
    assert await run_objective(deps, obj) == STOP_PAUSED


async def test_run_objective_stops_on_quorum_failure():
    store = FakeCouncilStore()
    obj = await store.create_objective("q")

    async def fake_fanout(**kwargs):
        return [
            MemberAnswer("alpha", "ollama/a", "t", "answered", "x", [], ""),
            MemberAnswer("beta", "ollama/b", "t", "failed", "", [], "502"),
        ]

    async def fake_synthesize(*args, **kwargs):
        raise AssertionError("must not judge below quorum")

    deps = CouncilDeps(
        config=config(quorum=2), settings=None, tools=[], checkpointer=None,
        store=store, judge_model=object(), fanout_fn=fake_fanout,
        synthesize_fn=fake_synthesize,
    )
    assert await run_objective(deps, obj) == STOP_NEEDS_REVIEW


async def test_run_objective_persists_cycles_and_member_runs():
    store = FakeCouncilStore()
    obj = await store.create_objective("q")

    async def fake_fanout(**kwargs):
        return [
            MemberAnswer(
                "alpha", "ollama/a", "t1", "answered", "x",
                [{"tool": "query", "input": {}}], "",
            ),
            MemberAnswer("beta", "ollama/b", "t2", "answered", "y", [], ""),
        ]

    async def fake_synthesize(*args, **kwargs):
        return verdict(agreement=0.9)

    deps = CouncilDeps(
        config=config(), settings=None, tools=[], checkpointer=None, store=store,
        judge_model=object(), fanout_fn=fake_fanout, synthesize_fn=fake_synthesize,
    )
    assert await run_objective(deps, obj) == STOP_CONVERGED
    assert len(store.cycles) == 1
    assert len(store.member_runs) == 2
    assert {r["member_id"] for r in store.member_runs} == {"alpha", "beta"}
    final = await store.get_objective(obj["id"])
    assert final["status"] == "completed"
    assert final["stop_reason"] == STOP_CONVERGED
