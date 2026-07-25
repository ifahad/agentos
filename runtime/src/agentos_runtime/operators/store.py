"""Persistence for operators: stored objectives and their autonomous runs.

Follows the ImprovementStore/CouncilStore pattern: all SQL behind one class,
one short-lived psycopg connection per operation, lazy idempotent DDL, and a
method surface a fake mirrors so the engine and API tests never touch a
database.
"""

from __future__ import annotations

import json
import uuid
from typing import Any

OPERATORS_DDL = """
CREATE TABLE IF NOT EXISTS operators (
    id             text PRIMARY KEY,
    name           text NOT NULL,
    goal           text NOT NULL,
    trigger_type   text NOT NULL,
    trigger_config jsonb NOT NULL,
    enabled        boolean NOT NULL DEFAULT true,
    max_cycles     integer NOT NULL DEFAULT 8,
    created_at     timestamptz NOT NULL DEFAULT now(),
    last_fired_at  timestamptz
);
CREATE TABLE IF NOT EXISTS operator_runs (
    id             text PRIMARY KEY,
    operator_id    text NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
    thread_id      text NOT NULL,
    status         text NOT NULL,
    output         text NOT NULL DEFAULT '',
    steps          jsonb NOT NULL DEFAULT '[]'::jsonb,
    cycles         integer NOT NULL DEFAULT 0,
    trigger_source text NOT NULL,
    error          text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS operator_runs_operator_idx ON operator_runs (operator_id);
CREATE INDEX IF NOT EXISTS operators_webhook_idx
    ON operators ((trigger_config->>'webhook_token'));
"""

_OP_COLUMNS = (
    "id, name, goal, trigger_type, trigger_config, enabled, max_cycles, "
    "created_at, last_fired_at"
)
_RUN_COLUMNS = (
    "id, operator_id, thread_id, status, output, steps, cycles, "
    "trigger_source, error, created_at"
)


def _iso(value: Any) -> Any:
    return value.isoformat() if hasattr(value, "isoformat") else value


def _operator_row(row: tuple, *, redact_webhook: bool = True) -> dict[str, Any]:
    config = dict(row[4] or {})
    # The webhook token is a secret; it is returned only at creation, never in a
    # listing. Redact it from every read path.
    if redact_webhook and "webhook_token" in config:
        config = {**config, "webhook_token": None}
    return {
        "id": row[0],
        "name": row[1],
        "goal": row[2],
        "trigger": {"type": row[3], **config},
        "enabled": row[5],
        "max_cycles": row[6],
        "created_at": _iso(row[7]),
        "last_fired_at": _iso(row[8]),
    }


def _run_row(row: tuple) -> dict[str, Any]:
    return {
        "id": row[0],
        "operator_id": row[1],
        "thread_id": row[2],
        "status": row[3],
        "output": row[4],
        "steps": row[5],
        "cycles": row[6],
        "trigger_source": row[7],
        "error": row[8],
        "created_at": _iso(row[9]),
    }


class OperatorStore:
    """psycopg-backed store; one short-lived connection per operation."""

    def __init__(self, database_url: str) -> None:
        self._database_url = database_url
        self._ready = False

    async def _connect(self):
        import psycopg

        conn = await psycopg.AsyncConnection.connect(self._database_url)
        if not self._ready:
            await conn.execute(OPERATORS_DDL)
            await conn.commit()
            self._ready = True
        return conn

    async def create_operator(
        self,
        name: str,
        goal: str,
        trigger_type: str,
        trigger_config: dict[str, Any],
        enabled: bool,
        max_cycles: int,
    ) -> dict[str, Any]:
        operator_id = uuid.uuid4().hex
        async with await self._connect() as conn:
            cursor = await conn.execute(
                "INSERT INTO operators "
                "(id, name, goal, trigger_type, trigger_config, enabled, max_cycles) "
                "VALUES (%s, %s, %s, %s, %s::jsonb, %s, %s) RETURNING " + _OP_COLUMNS,
                (operator_id, name, goal, trigger_type, json.dumps(trigger_config),
                 enabled, max_cycles),
            )
            row = await cursor.fetchone()
            await conn.commit()
        # Creation is the ONE place the webhook token is returned.
        return _operator_row(row, redact_webhook=False)

    async def get_operator(self, operator_id: str) -> dict[str, Any] | None:
        async with await self._connect() as conn:
            cursor = await conn.execute(
                f"SELECT {_OP_COLUMNS} FROM operators WHERE id = %s", (operator_id,)
            )
            row = await cursor.fetchone()
        return _operator_row(row) if row else None

    async def list_operators(self) -> list[dict[str, Any]]:
        async with await self._connect() as conn:
            cursor = await conn.execute(
                f"SELECT {_OP_COLUMNS} FROM operators ORDER BY created_at DESC"
            )
            rows = await cursor.fetchall()
        return [_operator_row(row) for row in rows]

    async def find_by_webhook(self, token: str) -> dict[str, Any] | None:
        """Resolve an enabled webhook operator by its token. Returns the row
        WITH the token intact (the caller already holds it)."""
        async with await self._connect() as conn:
            cursor = await conn.execute(
                f"SELECT {_OP_COLUMNS} FROM operators "
                "WHERE trigger_type = 'webhook' AND enabled = true "
                "AND trigger_config->>'webhook_token' = %s",
                (token,),
            )
            row = await cursor.fetchone()
        return _operator_row(row, redact_webhook=False) if row else None

    async def enabled_scheduled(self) -> list[dict[str, Any]]:
        """Enabled interval/cron operators the scheduler must consider."""
        async with await self._connect() as conn:
            cursor = await conn.execute(
                f"SELECT {_OP_COLUMNS} FROM operators "
                "WHERE enabled = true AND trigger_type IN ('interval', 'cron')"
            )
            rows = await cursor.fetchall()
        return [_operator_row(row, redact_webhook=False) for row in rows]

    async def update_operator(
        self,
        operator_id: str,
        *,
        enabled: bool | None = None,
        goal: str | None = None,
        max_cycles: int | None = None,
        trigger_type: str | None = None,
        trigger_config: dict[str, Any] | None = None,
    ) -> dict[str, Any] | None:
        sets: list[str] = []
        args: list[Any] = []
        for column, value in (
            ("enabled", enabled),
            ("goal", goal),
            ("max_cycles", max_cycles),
            ("trigger_type", trigger_type),
        ):
            if value is not None:
                sets.append(f"{column} = %s")
                args.append(value)
        if trigger_config is not None:
            sets.append("trigger_config = %s::jsonb")
            args.append(json.dumps(trigger_config))
        if not sets:
            return await self.get_operator(operator_id)
        args.append(operator_id)
        async with await self._connect() as conn:
            cursor = await conn.execute(
                f"UPDATE operators SET {', '.join(sets)} WHERE id = %s RETURNING " + _OP_COLUMNS,
                tuple(args),
            )
            row = await cursor.fetchone()
            await conn.commit()
        return _operator_row(row) if row else None

    async def mark_fired(self, operator_id: str) -> None:
        async with await self._connect() as conn:
            await conn.execute(
                "UPDATE operators SET last_fired_at = now() WHERE id = %s", (operator_id,)
            )
            await conn.commit()

    async def delete_operator(self, operator_id: str) -> bool:
        async with await self._connect() as conn:
            cursor = await conn.execute(
                "DELETE FROM operators WHERE id = %s", (operator_id,)
            )
            await conn.commit()
            return cursor.rowcount > 0

    async def insert_run(
        self,
        operator_id: str,
        thread_id: str,
        status: str,
        output: str,
        steps: list[dict[str, Any]],
        cycles: int,
        trigger_source: str,
        error: str = "",
    ) -> dict[str, Any]:
        run_id = uuid.uuid4().hex
        async with await self._connect() as conn:
            cursor = await conn.execute(
                "INSERT INTO operator_runs "
                "(id, operator_id, thread_id, status, output, steps, cycles, "
                "trigger_source, error) "
                "VALUES (%s, %s, %s, %s, %s, %s::jsonb, %s, %s, %s) RETURNING " + _RUN_COLUMNS,
                (run_id, operator_id, thread_id, status, output, json.dumps(steps),
                 cycles, trigger_source, error),
            )
            row = await cursor.fetchone()
            await conn.commit()
        return _run_row(row)

    async def list_runs(self, operator_id: str, limit: int = 20) -> list[dict[str, Any]]:
        async with await self._connect() as conn:
            cursor = await conn.execute(
                f"SELECT {_RUN_COLUMNS} FROM operator_runs "
                "WHERE operator_id = %s ORDER BY created_at DESC LIMIT %s",
                (operator_id, limit),
            )
            rows = await cursor.fetchall()
        return [_run_row(row) for row in rows]
