"""Council fan-out: per-member agents, concurrency, failure isolation, quorum."""

import asyncio

from langchain_core.messages import AIMessage

from agentos_runtime.council.config import CouncilConfig, Member
from agentos_runtime.council.fanout import (
    MemberAnswer,
    fanout,
    member_thread_id,
    quorum_met,
)


def make_config(**overrides):
    base = dict(
        judge="ollama/judge",
        quorum=2,
        agreement_threshold=0.6,
        max_cycles=8,
        max_tool_steps=12,
        member_timeout_s=5,
        members=[
            Member(id="alpha", model="ollama/a", enabled=True),
            Member(id="beta", model="ollama/b", enabled=True),
            Member(id="gamma", model="ollama/c", enabled=True),
        ],
    )
    base.update(overrides)
    return CouncilConfig(**base)


class StubAgent:
    """Agent stand-in matching the run_until_settled contract.

    ainvoke returns graph state values ({"messages": [...]}); aget_state returns
    a settled snapshot so run_until_settled completes in one pass. Output is a
    real AIMessage because extract_output only reads AIMessage instances.
    """

    def __init__(self, answer="", raises=None, delay=0.0, tool_calls=None):
        self.answer = answer
        self.raises = raises
        self.delay = delay
        self.tool_calls = tool_calls or []

    async def ainvoke(self, input_state, config=None):
        if self.delay:
            await asyncio.sleep(self.delay)
        if self.raises:
            raise self.raises
        msg = AIMessage(content=self.answer, tool_calls=self.tool_calls)
        return {"messages": [msg]}

    async def aget_state(self, config):
        return _SettledSnapshot()


class _SettledSnapshot:
    next = ()
    values = {"messages": []}


def factory_for(agents):
    """agent_factory that returns a preconfigured StubAgent per member."""
    return lambda settings, member, tools, checkpointer, model=None: agents[member.id]


def test_member_thread_id_namespaces_by_objective_and_member():
    assert member_thread_id("obj-1", "alpha") == "obj-1:alpha"
    assert member_thread_id("obj-1", "beta") != member_thread_id("obj-1", "alpha")
    assert member_thread_id("obj-2", "alpha") != member_thread_id("obj-1", "alpha")


async def test_fanout_calls_every_enabled_member_and_returns_distinct_answers():
    config = make_config()
    agents = {
        "alpha": StubAgent("answer from alpha"),
        "beta": StubAgent("answer from beta"),
        "gamma": StubAgent("answer from gamma"),
    }
    answers = await fanout(
        config, settings=None, members=config.enabled_members, tools=[],
        checkpointer=None, objective_id="obj-1", cycle_no=1,
        input_text="what is the answer?",
        agent_factory=factory_for(agents),
    )
    assert len(answers) == 3
    assert {a.member_id for a in answers} == {"alpha", "beta", "gamma"}
    assert all(a.status == "answered" for a in answers)
    assert {a.output for a in answers} == {
        "answer from alpha", "answer from beta", "answer from gamma",
    }
    assert {a.model_used for a in answers} == {"ollama/a", "ollama/b", "ollama/c"}


async def test_fanout_records_tool_steps():
    config = make_config(quorum=1, members=[Member(id="alpha", model="ollama/a", enabled=True)])
    call = {"name": "query", "args": {"sql": "select 1"}, "id": "c1"}
    agent = StubAgent("done", tool_calls=[call])
    answers = await fanout(
        config, settings=None, members=config.enabled_members, tools=[],
        checkpointer=None, objective_id="obj-1", cycle_no=1, input_text="q",
        agent_factory=factory_for({"alpha": agent}),
    )
    assert answers[0].steps == [{"tool": "query", "input": {"sql": "select 1"}}]


async def test_fanout_isolates_a_failing_member():
    """One member exploding must not deny the others' answers."""
    config = make_config()
    agents = {
        "alpha": StubAgent("ok"),
        "beta": StubAgent(raises=RuntimeError("provider 502")),
        "gamma": StubAgent("ok too"),
    }
    answers = await fanout(
        config, settings=None, members=config.enabled_members, tools=[],
        checkpointer=None, objective_id="obj-1", cycle_no=1, input_text="q",
        agent_factory=factory_for(agents),
    )
    by_id = {a.member_id: a for a in answers}
    assert by_id["beta"].status == "failed"
    assert "provider 502" in by_id["beta"].error
    assert by_id["alpha"].status == "answered"
    assert by_id["gamma"].status == "answered"


async def test_fanout_times_out_a_hanging_member():
    config = make_config(member_timeout_s=1)
    agents = {
        "alpha": StubAgent("fast"),
        "beta": StubAgent("slow", delay=30),
        "gamma": StubAgent("fast too"),
    }
    answers = await fanout(
        config, settings=None, members=config.enabled_members, tools=[],
        checkpointer=None, objective_id="obj-1", cycle_no=1, input_text="q",
        agent_factory=factory_for(agents),
    )
    by_id = {a.member_id: a for a in answers}
    assert by_id["beta"].status == "timeout"
    assert by_id["alpha"].status == "answered"


async def test_fanout_runs_members_concurrently_not_serially():
    """Three 1s members must finish in about 1s, not 3s."""
    config = make_config(member_timeout_s=10)
    agents = {mid: StubAgent("ok", delay=1.0) for mid in ("alpha", "beta", "gamma")}
    loop = asyncio.get_running_loop()
    start = loop.time()
    await fanout(
        config, settings=None, members=config.enabled_members, tools=[],
        checkpointer=None, objective_id="obj-1", cycle_no=1, input_text="q",
        agent_factory=factory_for(agents),
    )
    assert loop.time() - start < 2.0


def test_quorum_met():
    answered = [MemberAnswer("a", "m", "t", "answered", "x", [], "")]
    failed = [MemberAnswer("b", "m", "t", "failed", "", [], "boom")]
    assert quorum_met(answered * 2, quorum=2)
    assert not quorum_met(answered + failed, quorum=2)
    assert not quorum_met(failed * 3, quorum=1)
