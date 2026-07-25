"""Operator store contract: DDL shape and the fake used across operator tests."""

from helpers import FakeOperatorStore

from agentos_runtime.operators.store import OPERATORS_DDL


def test_ddl_creates_both_tables():
    assert "CREATE TABLE IF NOT EXISTS operators" in OPERATORS_DDL
    assert "CREATE TABLE IF NOT EXISTS operator_runs" in OPERATORS_DDL


def test_ddl_indexes_the_webhook_token():
    # The webhook fire path resolves an operator by token; that lookup needs an
    # index or every inbound webhook scans the table.
    assert "webhook_token" in OPERATORS_DDL
    assert "operators_webhook_idx" in OPERATORS_DDL


async def test_fake_create_returns_webhook_token_once():
    store = FakeOperatorStore()
    created = await store.create_operator(
        "nightly", "summarise the day", "webhook",
        {"webhook_token": "whk-secret"}, True, 8,
    )
    # Creation returns the token...
    assert created["trigger"]["webhook_token"] == "whk-secret"
    # ...but a subsequent read redacts it.
    fetched = await store.get_operator(created["id"])
    assert fetched["trigger"]["webhook_token"] is None


async def test_fake_find_by_webhook_only_matches_enabled():
    store = FakeOperatorStore()
    op = await store.create_operator(
        "hook", "do", "webhook", {"webhook_token": "whk-1"}, True, 8
    )
    assert (await store.find_by_webhook("whk-1"))["id"] == op["id"]
    assert await store.find_by_webhook("whk-nope") is None

    await store.update_operator(op["id"], enabled=False)
    assert await store.find_by_webhook("whk-1") is None, "disabled webhook must not fire"


async def test_fake_enabled_scheduled_excludes_webhook_and_disabled():
    store = FakeOperatorStore()
    await store.create_operator("i", "g", "interval", {"interval_s": 60}, True, 8)
    await store.create_operator("c", "g", "cron", {"cron": "0 9 * * *"}, True, 8)
    await store.create_operator("w", "g", "webhook", {"webhook_token": "whk-x"}, True, 8)
    off = await store.create_operator("off", "g", "interval", {"interval_s": 60}, True, 8)
    await store.update_operator(off["id"], enabled=False)

    scheduled = await store.enabled_scheduled()
    kinds = sorted(op["trigger"]["type"] for op in scheduled)
    assert kinds == ["cron", "interval"], "only enabled interval/cron operators are scheduled"


async def test_fake_runs_are_scoped_and_newest_first():
    store = FakeOperatorStore()
    a = await store.create_operator("a", "g", "interval", {"interval_s": 60}, True, 8)
    b = await store.create_operator("b", "g", "interval", {"interval_s": 60}, True, 8)
    await store.insert_run(a["id"], "t1", "completed", "one", [], 1, "manual")
    await store.insert_run(a["id"], "t2", "error", "", [], 0, "interval", error="boom")
    await store.insert_run(b["id"], "t3", "completed", "b-run", [], 1, "manual")

    runs_a = await store.list_runs(a["id"])
    assert [r["status"] for r in runs_a] == ["error", "completed"]  # newest first
    assert len(await store.list_runs(b["id"])) == 1


async def test_fake_delete():
    store = FakeOperatorStore()
    op = await store.create_operator("x", "g", "interval", {"interval_s": 60}, True, 8)
    assert await store.delete_operator(op["id"]) is True
    assert await store.get_operator(op["id"]) is None
    assert await store.delete_operator(op["id"]) is False
