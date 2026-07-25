"""Operator engine: firing, HITL needs_approval, failure isolation, scheduling."""

from helpers import FakeOperatorStore
from langchain_core.messages import AIMessage

from agentos_runtime.operators.engine import (
    STATUS_COMPLETED,
    STATUS_ERROR,
    STATUS_NEEDS_APPROVAL,
    OperatorDeps,
    Scheduler,
    fire_operator,
)


class SettledSnapshot:
    next = ()
    values = {"messages": []}


class InterruptSnapshot:
    next = ("tools",)

    def __init__(self, messages):
        self.values = {"messages": messages}


class StubAgent:
    """Agent stand-in for run_until_settled: answers, or pauses for approval."""

    def __init__(self, answer="done", raises=None, pending_tool=None, tool_calls=None):
        self.answer = answer
        self.raises = raises
        self.pending_tool = pending_tool
        self.tool_calls = tool_calls or []

    async def ainvoke(self, input_state, config=None):
        if self.raises:
            raise self.raises
        return {"messages": [AIMessage(content=self.answer, tool_calls=self.tool_calls)]}

    async def aget_state(self, config):
        if self.pending_tool:
            msg = AIMessage(
                content="",
                tool_calls=[{"name": self.pending_tool, "args": {}, "id": "c1"}],
            )
            return InterruptSnapshot([msg])
        return SettledSnapshot()


def deps_with(agent, store=None, approval_tools=None, notify=None):
    store = store or FakeOperatorStore()
    return OperatorDeps(
        store=store,
        agent_builder=lambda prompt: agent,
        approval_tools=approval_tools or [],
        notify=notify,
    )


async def make_operator(store, **over):
    base = dict(name="op", goal="summarise invoices", trigger_type="interval",
                trigger_config={"interval_s": 60}, enabled=True, max_cycles=8)
    base.update(over)
    return await store.create_operator(**base)


async def test_fire_records_a_completed_run():
    store = FakeOperatorStore()
    op = await make_operator(store)
    deps = deps_with(StubAgent(answer="42 invoices open"), store)
    run = await fire_operator(deps, op, "manual")
    assert run["status"] == STATUS_COMPLETED
    assert run["output"] == "42 invoices open"
    assert run["trigger_source"] == "manual"
    assert store.operators[op["id"]]["last_fired_at"] is not None


async def test_fire_records_tool_steps_and_cycle_count():
    store = FakeOperatorStore()
    op = await make_operator(store)
    agent = StubAgent(tool_calls=[{"name": "query", "args": {"sql": "select 1"}, "id": "c1"}])
    run = await fire_operator(deps_with(agent, store), op, "manual")
    assert run["steps"] == [{"tool": "query", "input": {"sql": "select 1"}}]
    assert run["cycles"] == 1


async def test_hitl_interrupt_is_recorded_needs_approval_not_auto_resumed():
    store = FakeOperatorStore()
    op = await make_operator(store)
    # The agent wants to call an approval-gated tool; run_until_settled returns
    # pending_approval, and the engine must NOT resume it.
    agent = StubAgent(pending_tool="sql_write")
    deps = deps_with(agent, store, approval_tools=["sql_write"])
    run = await fire_operator(deps, op, "interval")
    assert run["status"] == STATUS_NEEDS_APPROVAL
    assert "approval" in run["error"]


async def test_a_failing_run_is_recorded_error_never_raises():
    store = FakeOperatorStore()
    op = await make_operator(store)
    deps = deps_with(StubAgent(raises=RuntimeError("gateway 502")), store)
    run = await fire_operator(deps, op, "cron")
    assert run["status"] == STATUS_ERROR
    assert "gateway 502" in run["error"]


async def test_webhook_payload_is_appended_as_untrusted_data():
    store = FakeOperatorStore()
    op = await make_operator(store, trigger_type="webhook",
                             trigger_config={"webhook_token": "whk-1"})
    captured = {}

    class CapturingAgent(StubAgent):
        async def ainvoke(self, input_state, config=None):
            captured["goal"] = input_state["messages"][0][1]
            return await super().ainvoke(input_state, config)

    await fire_operator(deps_with(CapturingAgent(), store), op, "webhook",
                        context={"order_id": 7})
    assert "trigger_payload" in captured["goal"]
    assert "order_id" in captured["goal"]


async def test_notify_is_called_but_its_failure_does_not_fail_the_run():
    store = FakeOperatorStore()
    op = await make_operator(store)
    calls = []

    async def notify(operator, run):
        calls.append(run["id"])
        raise RuntimeError("notify endpoint down")

    run = await fire_operator(deps_with(StubAgent(), store, notify=notify), op, "manual")
    assert run["status"] == STATUS_COMPLETED  # notify failure did not fail the run
    assert len(calls) == 1


# --- scheduler ---


async def test_scheduler_fires_a_due_interval_operator():
    store = FakeOperatorStore()
    await make_operator(store, trigger_config={"interval_s": 60})
    sched = Scheduler(deps_with(StubAgent(), store), tick_s=15)

    # First sight: fires immediately.
    fired = await sched.tick(now=1000.0, wall_now=1000.0)
    assert len(fired) == 1
    # 30s later: not yet due (interval 60).
    assert await sched.tick(now=1030.0, wall_now=1030.0) == []
    # 60s after the fire: due again.
    assert len(await sched.tick(now=1060.0, wall_now=1060.0)) == 1


async def test_scheduler_ignores_disabled_and_webhook_operators():
    store = FakeOperatorStore()
    off = await make_operator(store)
    await store.update_operator(off["id"], enabled=False)
    await make_operator(store, trigger_type="webhook",
                        trigger_config={"webhook_token": "whk-1"})
    sched = Scheduler(deps_with(StubAgent(), store))
    assert await sched.tick(now=1000.0, wall_now=1000.0) == []


async def test_scheduler_seeds_cron_without_firing_immediately():
    store = FakeOperatorStore()
    await make_operator(store, trigger_type="cron", trigger_config={"cron": "0 9 * * *"})
    sched = Scheduler(deps_with(StubAgent(), store))
    # First tick seeds the cron baseline; it must not fire out of schedule.
    assert await sched.tick(now=1000.0, wall_now=1_700_000_000.0) == []
