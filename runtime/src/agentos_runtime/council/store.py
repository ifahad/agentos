"""Persistence for the council: objectives, cycles, member runs, proposals.

Follows the ImprovementStore pattern (see ``agentos_runtime/store.py``): all SQL
behind one class so tests can substitute an in-memory fake, one short-lived
psycopg connection per operation, and lazy idempotent DDL on first use.
"""

from __future__ import annotations

import json
import uuid
from typing import Any

COUNCIL_DDL = """
CREATE TABLE IF NOT EXISTS council_objectives (
    id          text PRIMARY KEY,
    input       text NOT NULL,
    status      text NOT NULL,
    stop_reason text,
    cycles_run  integer NOT NULL DEFAULT 0,
    spend_usd   double precision NOT NULL DEFAULT 0,
    max_cycles  integer,
    budget_usd  double precision NOT NULL DEFAULT 5,
    created_at  timestamptz NOT NULL DEFAULT now(),
    claimed_at  timestamptz,
    claimed_by  text
);
CREATE TABLE IF NOT EXISTS council_cycles (
    id           text PRIMARY KEY,
    objective_id text NOT NULL REFERENCES council_objectives(id) ON DELETE CASCADE,
    cycle_no     integer NOT NULL,
    verdict      jsonb NOT NULL,
    agreement    double precision NOT NULL,
    dissent      jsonb NOT NULL,
    started_at   timestamptz NOT NULL DEFAULT now(),
    finished_at  timestamptz
);
CREATE TABLE IF NOT EXISTS council_member_runs (
    id         text PRIMARY KEY,
    cycle_id   text NOT NULL REFERENCES council_cycles(id) ON DELETE CASCADE,
    member_id  text NOT NULL,
    model_used text NOT NULL,
    thread_id  text NOT NULL,
    status     text NOT NULL,
    output     text NOT NULL DEFAULT '',
    steps      jsonb NOT NULL DEFAULT '[]'::jsonb,
    cost_usd   double precision NOT NULL DEFAULT 0,
    error      text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS council_proposals (
    id           text PRIMARY KEY,
    objective_id text NOT NULL REFERENCES council_objectives(id) ON DELETE CASCADE,
    member_id    text NOT NULL,
    tool         text NOT NULL,
    arguments    jsonb NOT NULL,
    status       text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    decided_at   timestamptz
);
CREATE TABLE IF NOT EXISTS council_state (
    id     integer PRIMARY KEY,
    paused boolean NOT NULL DEFAULT false
);
"""

_OBJECTIVE_COLUMNS = (
    "id, input, status, stop_reason, cycles_run, spend_usd, "
    "max_cycles, budget_usd, created_at, claimed_by"
)
_CYCLE_COLUMNS = "id, objective_id, cycle_no, verdict, agreement, dissent, started_at"
_PROPOSAL_COLUMNS = (
    "id, objective_id, member_id, tool, arguments, status, created_at, decided_at"
)


def _iso(value: Any) -> Any:
    return value.isoformat() if hasattr(value, "isoformat") else value


def _objective_row(row: tuple) -> dict[str, Any]:
    return {
        "id": row[0],
        "input": row[1],
        "status": row[2],
        "stop_reason": row[3],
        "cycles_run": row[4],
        "spend_usd": row[5],
        "max_cycles": row[6],
        "budget_usd": row[7],
        "created_at": _iso(row[8]),
        "claimed_by": row[9],
    }


def _cycle_row(row: tuple) -> dict[str, Any]:
    return {
        "id": row[0],
        "objective_id": row[1],
        "cycle_no": row[2],
        "verdict": row[3],
        "agreement": row[4],
        "dissent": row[5],
        "started_at": _iso(row[6]),
    }


def _proposal_row(row: tuple) -> dict[str, Any]:
    return {
        "id": row[0],
        "objective_id": row[1],
        "member_id": row[2],
        "tool": row[3],
        "arguments": row[4],
        "status": row[5],
        "created_at": _iso(row[6]),
        "decided_at": _iso(row[7]),
    }


class CouncilStore:
    """psycopg-backed store; one short-lived connection per operation."""

    def __init__(self, database_url: str) -> None:
        self._database_url = database_url
        self._ready = False

    async def _connect(self):
        import psycopg

        conn = await psycopg.AsyncConnection.connect(self._database_url)
        if not self._ready:
            await conn.execute(COUNCIL_DDL)
            await conn.commit()
            self._ready = True
        return conn

    # --- objectives -------------------------------------------------------

    async def create_objective(
        self, input_text: str, max_cycles: int | None = None, budget_usd: float = 5.0
    ) -> dict[str, Any]:
        objective_id = uuid.uuid4().hex
        async with await self._connect() as conn:
            cursor = await conn.execute(
                "INSERT INTO council_objectives (id, input, status, max_cycles, budget_usd) "
                "VALUES (%s, %s, 'pending', %s, %s) RETURNING " + _OBJECTIVE_COLUMNS,
                (objective_id, input_text, max_cycles, budget_usd),
            )
            row = await cursor.fetchone()
            await conn.commit()
        return _objective_row(row)

    async def get_objective(self, objective_id: str) -> dict[str, Any] | None:
        async with await self._connect() as conn:
            cursor = await conn.execute(
                f"SELECT {_OBJECTIVE_COLUMNS} FROM council_objectives WHERE id = %s",
                (objective_id,),
            )
            row = await cursor.fetchone()
        return _objective_row(row) if row else None

    async def list_objectives(self, limit: int = 20) -> list[dict[str, Any]]:
        async with await self._connect() as conn:
            cursor = await conn.execute(
                f"SELECT {_OBJECTIVE_COLUMNS} FROM council_objectives "
                "ORDER BY created_at DESC LIMIT %s",
                (limit,),
            )
            rows = await cursor.fetchall()
        return [_objective_row(row) for row in rows]

    async def claim_next_objective(self, worker: str) -> dict[str, Any] | None:
        """Atomically claim the oldest pending objective.

        FOR UPDATE SKIP LOCKED is what makes multiple runtime replicas safe:
        each transaction locks a distinct row, so two heartbeats never run the
        same objective.
        """
        conn = await self._connect()
        try:
            async with conn.transaction():
                cur = await conn.execute(
                    """
                    SELECT id FROM council_objectives
                     WHERE status = 'pending'
                     ORDER BY created_at
                     FOR UPDATE SKIP LOCKED
                     LIMIT 1
                    """
                )
                row = await cur.fetchone()
                if row is None:
                    return None
                await conn.execute(
                    """
                    UPDATE council_objectives
                       SET status = 'running', claimed_at = now(), claimed_by = %s
                     WHERE id = %s
                    """,
                    (worker, row[0]),
                )
            return await self.get_objective(row[0])
        finally:
            await conn.close()

    async def update_objective(
        self,
        objective_id: str,
        *,
        status: str | None = None,
        stop_reason: str | None = None,
        cycles_run: int | None = None,
        spend_usd: float | None = None,
    ) -> None:
        sets: list[str] = []
        args: list[Any] = []
        for column, value in (
            ("status", status),
            ("stop_reason", stop_reason),
            ("cycles_run", cycles_run),
            ("spend_usd", spend_usd),
        ):
            if value is not None:
                sets.append(f"{column} = %s")
                args.append(value)
        if not sets:
            return
        args.append(objective_id)
        async with await self._connect() as conn:
            await conn.execute(
                f"UPDATE council_objectives SET {', '.join(sets)} WHERE id = %s",
                tuple(args),
            )
            await conn.commit()

    # --- cycles and member runs -------------------------------------------

    async def insert_cycle(
        self,
        objective_id: str,
        cycle_no: int,
        verdict: dict[str, Any],
        agreement: float,
        dissent: list[dict[str, Any]],
    ) -> str:
        cycle_id = uuid.uuid4().hex
        async with await self._connect() as conn:
            await conn.execute(
                "INSERT INTO council_cycles "
                "(id, objective_id, cycle_no, verdict, agreement, dissent, finished_at) "
                "VALUES (%s, %s, %s, %s::jsonb, %s, %s::jsonb, now())",
                (
                    cycle_id,
                    objective_id,
                    cycle_no,
                    json.dumps(verdict),
                    agreement,
                    json.dumps(dissent),
                ),
            )
            await conn.commit()
        return cycle_id

    async def insert_member_run(
        self,
        cycle_id: str,
        member_id: str,
        model_used: str,
        thread_id: str,
        status: str,
        output: str,
        steps: list[dict[str, Any]],
        cost_usd: float,
        error: str,
    ) -> None:
        async with await self._connect() as conn:
            await conn.execute(
                "INSERT INTO council_member_runs "
                "(id, cycle_id, member_id, model_used, thread_id, status, output, "
                "steps, cost_usd, error) "
                "VALUES (%s, %s, %s, %s, %s, %s, %s, %s::jsonb, %s, %s)",
                (
                    uuid.uuid4().hex,
                    cycle_id,
                    member_id,
                    model_used,
                    thread_id,
                    status,
                    output,
                    json.dumps(steps),
                    cost_usd,
                    error,
                ),
            )
            await conn.commit()

    async def list_cycles(self, objective_id: str) -> list[dict[str, Any]]:
        async with await self._connect() as conn:
            cursor = await conn.execute(
                f"SELECT {_CYCLE_COLUMNS} FROM council_cycles "
                "WHERE objective_id = %s ORDER BY cycle_no",
                (objective_id,),
            )
            rows = await cursor.fetchall()
        return [_cycle_row(row) for row in rows]

    # --- proposals --------------------------------------------------------

    async def insert_proposal(
        self, objective_id: str, member_id: str, tool: str, arguments: dict[str, Any]
    ) -> str:
        proposal_id = uuid.uuid4().hex
        async with await self._connect() as conn:
            await conn.execute(
                "INSERT INTO council_proposals "
                "(id, objective_id, member_id, tool, arguments, status) "
                "VALUES (%s, %s, %s, %s, %s::jsonb, 'pending')",
                (proposal_id, objective_id, member_id, tool, json.dumps(arguments)),
            )
            await conn.commit()
        return proposal_id

    async def get_proposal(self, proposal_id: str) -> dict[str, Any] | None:
        async with await self._connect() as conn:
            cursor = await conn.execute(
                f"SELECT {_PROPOSAL_COLUMNS} FROM council_proposals WHERE id = %s",
                (proposal_id,),
            )
            row = await cursor.fetchone()
        return _proposal_row(row) if row else None

    async def list_proposals(
        self, status: str | None = None, limit: int = 50
    ) -> list[dict[str, Any]]:
        async with await self._connect() as conn:
            if status is None:
                cursor = await conn.execute(
                    f"SELECT {_PROPOSAL_COLUMNS} FROM council_proposals "
                    "ORDER BY created_at DESC LIMIT %s",
                    (limit,),
                )
            else:
                cursor = await conn.execute(
                    f"SELECT {_PROPOSAL_COLUMNS} FROM council_proposals "
                    "WHERE status = %s ORDER BY created_at DESC LIMIT %s",
                    (status, limit),
                )
            rows = await cursor.fetchall()
        return [_proposal_row(row) for row in rows]

    async def update_proposal_status(self, proposal_id: str, status: str) -> None:
        async with await self._connect() as conn:
            await conn.execute(
                "UPDATE council_proposals SET status = %s, decided_at = now() WHERE id = %s",
                (status, proposal_id),
            )
            await conn.commit()

    # --- kill switch ------------------------------------------------------

    async def is_paused(self) -> bool:
        """Whether the council kill switch is engaged (row 1 of council_state)."""
        async with await self._connect() as conn:
            cursor = await conn.execute("SELECT paused FROM council_state WHERE id = 1")
            row = await cursor.fetchone()
        return bool(row[0]) if row else False

    async def set_paused(self, paused: bool) -> None:
        async with await self._connect() as conn:
            await conn.execute(
                "INSERT INTO council_state (id, paused) VALUES (1, %s) "
                "ON CONFLICT (id) DO UPDATE SET paused = EXCLUDED.paused",
                (paused,),
            )
            await conn.commit()
