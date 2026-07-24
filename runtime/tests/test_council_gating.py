"""Write-class gating: reads execute, writes become proposals, unknown is write."""

from pathlib import Path

from helpers import FakeCouncilStore

from agentos_runtime.council.config import load_council_config
from agentos_runtime.council.gating import (
    READ_SAFE_TOOLS,
    hold_writes_as_proposals,
    is_read_safe,
    write_class_calls,
)
from agentos_runtime.hitl import PendingCall


def call(tool, **args):
    return PendingCall(tool=tool, input=args, tool_call_id=f"tc-{tool}")


def test_known_read_tools_are_read_safe():
    for tool in ("query", "list_tables", "describe_table", "search_knowledge", "run_python"):
        assert is_read_safe(tool), tool
        assert tool in READ_SAFE_TOOLS


def test_unknown_tools_are_write_class():
    """Fail closed: a tool nobody classified must not run unattended."""
    for tool in ("delete_customer", "post_invoice", "ssh_exec", "brand_new_tool", ""):
        assert not is_read_safe(tool), tool


def test_write_class_calls_partitions():
    calls = [call("query", sql="select 1"), call("delete_customer", id=7)]
    held = write_class_calls(calls)
    assert [c.tool for c in held] == ["delete_customer"]


async def test_hold_writes_records_proposals_and_denies_the_calls():
    store = FakeCouncilStore()
    obj = await store.create_objective("do the thing")
    calls = [call("delete_customer", id=7), call("query", sql="select 1")]

    denied = []

    class StubAgent:
        async def aupdate_state(self, config, values, as_node=None):
            denied.append((values, as_node))

    ids = await hold_writes_as_proposals(
        StubAgent(), {"configurable": {"thread_id": "t"}}, calls,
        store, obj["id"], "alpha",
    )

    assert len(ids) == 1
    proposals = await store.list_proposals()
    assert len(proposals) == 1
    assert proposals[0]["tool"] == "delete_customer"
    assert proposals[0]["arguments"] == {"id": 7}
    assert proposals[0]["status"] == "pending"
    assert proposals[0]["member_id"] == "alpha"
    # The write must be denied in-graph so the member sees it did not run.
    assert denied, "write-class calls must be denied at the tools node"


async def test_hold_writes_is_a_noop_when_all_calls_are_reads():
    store = FakeCouncilStore()
    obj = await store.create_objective("read only")

    class StubAgent:
        async def aupdate_state(self, config, values, as_node=None):
            raise AssertionError("must not deny a read-only batch")

    ids = await hold_writes_as_proposals(
        StubAgent(), {"configurable": {"thread_id": "t"}},
        [call("query", sql="select 1")], store, obj["id"], "alpha",
    )
    assert ids == []
    assert await store.list_proposals() == []


def test_shipped_config_gives_deep_members_only_read_safe_tools():
    """Deep members cannot be gated in-graph, so their tool list must be read-only."""
    cfg = load_council_config(Path(__file__).resolve().parents[1] / "council.yaml")
    for member in cfg.members:
        if member.profile != "deep":
            continue
        assert member.tools, f"deep member {member.id} must list its tools explicitly"
        for tool in member.tools:
            assert is_read_safe(tool), f"deep member {member.id} has write-class tool {tool}"
