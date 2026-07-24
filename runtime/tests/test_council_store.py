"""Council store contract: DDL shape and the fake used across council tests."""

from helpers import FakeCouncilStore

from agentos_runtime.council.store import COUNCIL_DDL


def test_ddl_creates_all_four_tables():
    for table in (
        "council_objectives",
        "council_cycles",
        "council_member_runs",
        "council_proposals",
    ):
        assert f"CREATE TABLE IF NOT EXISTS {table}" in COUNCIL_DDL


def test_ddl_is_idempotent_by_construction():
    """Every statement must be IF NOT EXISTS: the store applies DDL on each boot."""
    statements = [s.strip() for s in COUNCIL_DDL.split(";") if s.strip()]
    assert statements
    for statement in statements:
        assert "IF NOT EXISTS" in statement, statement


def test_objectives_table_supports_skip_locked_claiming():
    """Claiming needs a status to filter on and a worker/claimed_at to stamp."""
    assert "status" in COUNCIL_DDL
    assert "claimed_at" in COUNCIL_DDL
    assert "claimed_by" in COUNCIL_DDL


def test_member_runs_record_the_model_actually_used():
    """Fallback means the configured model and the used model can differ."""
    assert "model_used" in COUNCIL_DDL


# --- fake-store contract: the surface every council test relies on ---


async def test_fake_create_and_claim_objective():
    store = FakeCouncilStore()
    obj = await store.create_objective("summarise Q3", budget_usd=5.0)
    assert obj["status"] == "pending"

    claimed = await store.claim_next_objective("worker-1")
    assert claimed["id"] == obj["id"]
    assert claimed["claimed_by"] == "worker-1"
    # A second claim finds nothing pending.
    assert await store.claim_next_objective("worker-2") is None


async def test_fake_update_objective_ignores_none():
    store = FakeCouncilStore()
    obj = await store.create_objective("x")
    await store.update_objective(obj["id"], status="done", stop_reason=None)
    refreshed = await store.get_objective(obj["id"])
    assert refreshed["status"] == "done"
    assert refreshed["stop_reason"] is None


async def test_fake_cycles_and_member_runs():
    store = FakeCouncilStore()
    obj = await store.create_objective("x")
    cid = await store.insert_cycle(obj["id"], 1, {"answer": "a"}, 0.8, [])
    await store.insert_member_run(
        cid, "alpha", "ollama/qwen3.6:latest", "obj:alpha", "completed",
        "a", [], 0.0, "",
    )
    cycles = await store.list_cycles(obj["id"])
    assert len(cycles) == 1 and cycles[0]["agreement"] == 0.8
    assert store.member_runs[0]["model_used"] == "ollama/qwen3.6:latest"


async def test_fake_proposals_and_pause():
    store = FakeCouncilStore()
    obj = await store.create_objective("x")
    pid = await store.insert_proposal(obj["id"], "alpha", "sql.write", {"q": "..."})
    assert (await store.get_proposal(pid))["status"] == "pending"
    assert len(await store.list_proposals(status="pending")) == 1
    await store.update_proposal_status(pid, "approved")
    assert len(await store.list_proposals(status="pending")) == 0

    assert await store.is_paused() is False
    await store.set_paused(True)
    assert await store.is_paused() is True
