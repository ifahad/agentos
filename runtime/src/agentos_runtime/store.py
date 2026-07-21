"""Persistence for the self-improvement loop: eval runs, proposals, active prompt.

All SQL sits behind ``ImprovementStore`` so tests can substitute an in-memory
fake with the same method surface. The live store uses psycopg against
AGENTOS_CHECKPOINT_DATABASE_URL and creates its tables lazily
(CREATE TABLE IF NOT EXISTS) on first use. When no checkpoint database is
configured the API mounts no store and every self-improvement endpoint
returns 503 via :func:`get_store`.
"""

import json
import uuid
from typing import Any

from fastapi import HTTPException, Request

DDL = """
CREATE TABLE IF NOT EXISTS eval_runs (
    id            text PRIMARY KEY,
    suite         text NOT NULL,
    score         double precision NOT NULL,
    passed        integer NOT NULL,
    failed        integer NOT NULL,
    cases         jsonb NOT NULL,
    prompt_source text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS prompt_proposals (
    id              text PRIMARY KEY,
    prompt_text     text NOT NULL,
    rationale       text NOT NULL,
    baseline_score  double precision NOT NULL,
    candidate_score double precision NOT NULL,
    status          text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS active_prompt (
    id          integer PRIMARY KEY,
    prompt_text text NOT NULL,
    proposal_id text,
    updated_at  timestamptz NOT NULL DEFAULT now()
);
"""

EVAL_RUN_COLUMNS = "id, suite, score, passed, failed, cases, prompt_source, created_at"
PROPOSAL_COLUMNS = (
    "id, prompt_text, rationale, baseline_score, candidate_score, status, created_at"
)


def _iso(value: Any) -> Any:
    return value.isoformat() if hasattr(value, "isoformat") else value


def _eval_run_row(row: tuple, with_cases: bool) -> dict[str, Any]:
    run = {
        "run_id": row[0],
        "suite": row[1],
        "score": row[2],
        "passed": row[3],
        "failed": row[4],
        "cases": row[5],
        "prompt_source": row[6],
        "created_at": _iso(row[7]),
    }
    if not with_cases:
        del run["cases"]
    return run


def _proposal_row(row: tuple) -> dict[str, Any]:
    return {
        "id": row[0],
        "prompt_text": row[1],
        "rationale": row[2],
        "baseline_score": row[3],
        "candidate_score": row[4],
        "status": row[5],
        "created_at": _iso(row[6]),
    }


class ImprovementStore:
    """psycopg-backed store; one short-lived connection per operation."""

    def __init__(self, database_url: str) -> None:
        self._database_url = database_url
        self._ready = False

    async def _connect(self):
        import psycopg

        conn = await psycopg.AsyncConnection.connect(self._database_url)
        if not self._ready:
            await conn.execute(DDL)
            await conn.commit()
            self._ready = True
        return conn

    async def insert_eval_run(
        self,
        suite: str,
        score: float,
        passed: int,
        failed: int,
        cases: list[dict[str, Any]],
        prompt_source: str,
    ) -> dict[str, Any]:
        run_id = uuid.uuid4().hex
        async with await self._connect() as conn:
            cursor = await conn.execute(
                "INSERT INTO eval_runs (id, suite, score, passed, failed, cases, prompt_source) "
                "VALUES (%s, %s, %s, %s, %s, %s::jsonb, %s) RETURNING "
                + EVAL_RUN_COLUMNS,
                (run_id, suite, score, passed, failed, json.dumps(cases), prompt_source),
            )
            row = await cursor.fetchone()
            await conn.commit()
        return _eval_run_row(row, with_cases=True)

    async def list_eval_runs(self, limit: int = 20) -> list[dict[str, Any]]:
        async with await self._connect() as conn:
            cursor = await conn.execute(
                f"SELECT {EVAL_RUN_COLUMNS} FROM eval_runs ORDER BY created_at DESC LIMIT %s",
                (limit,),
            )
            rows = await cursor.fetchall()
        return [_eval_run_row(row, with_cases=False) for row in rows]

    async def latest_eval_run(self) -> dict[str, Any] | None:
        async with await self._connect() as conn:
            cursor = await conn.execute(
                f"SELECT {EVAL_RUN_COLUMNS} FROM eval_runs ORDER BY created_at DESC LIMIT 1"
            )
            row = await cursor.fetchone()
        return _eval_run_row(row, with_cases=True) if row else None

    async def insert_proposal(
        self,
        prompt_text: str,
        rationale: str,
        baseline_score: float,
        candidate_score: float,
        status: str,
    ) -> dict[str, Any]:
        proposal_id = uuid.uuid4().hex
        async with await self._connect() as conn:
            cursor = await conn.execute(
                "INSERT INTO prompt_proposals "
                "(id, prompt_text, rationale, baseline_score, candidate_score, status) "
                "VALUES (%s, %s, %s, %s, %s, %s) RETURNING " + PROPOSAL_COLUMNS,
                (proposal_id, prompt_text, rationale, baseline_score, candidate_score, status),
            )
            row = await cursor.fetchone()
            await conn.commit()
        return _proposal_row(row)

    async def get_proposal(self, proposal_id: str) -> dict[str, Any] | None:
        async with await self._connect() as conn:
            cursor = await conn.execute(
                f"SELECT {PROPOSAL_COLUMNS} FROM prompt_proposals WHERE id = %s",
                (proposal_id,),
            )
            row = await cursor.fetchone()
        return _proposal_row(row) if row else None

    async def list_proposals(self, limit: int = 20) -> list[dict[str, Any]]:
        async with await self._connect() as conn:
            cursor = await conn.execute(
                f"SELECT {PROPOSAL_COLUMNS} FROM prompt_proposals "
                "ORDER BY created_at DESC LIMIT %s",
                (limit,),
            )
            rows = await cursor.fetchall()
        return [_proposal_row(row) for row in rows]

    async def update_proposal_status(self, proposal_id: str, status: str) -> None:
        async with await self._connect() as conn:
            await conn.execute(
                "UPDATE prompt_proposals SET status = %s WHERE id = %s",
                (status, proposal_id),
            )
            await conn.commit()

    async def get_active_prompt(self) -> dict[str, Any] | None:
        async with await self._connect() as conn:
            cursor = await conn.execute(
                "SELECT prompt_text, proposal_id, updated_at FROM active_prompt WHERE id = 1"
            )
            row = await cursor.fetchone()
        if row is None:
            return None
        return {"prompt_text": row[0], "proposal_id": row[1], "updated_at": _iso(row[2])}

    async def set_active_prompt(self, prompt_text: str, proposal_id: str) -> None:
        async with await self._connect() as conn:
            await conn.execute(
                "INSERT INTO active_prompt (id, prompt_text, proposal_id, updated_at) "
                "VALUES (1, %s, %s, now()) "
                "ON CONFLICT (id) DO UPDATE SET prompt_text = EXCLUDED.prompt_text, "
                "proposal_id = EXCLUDED.proposal_id, updated_at = now()",
                (prompt_text, proposal_id),
            )
            await conn.commit()


def get_store(request: Request) -> ImprovementStore:
    """FastAPI dependency: the mounted store, or 503 when no checkpoint DB."""
    store = getattr(request.app.state, "improve_store", None)
    if store is None:
        raise HTTPException(
            status_code=503,
            detail="self-improvement requires AGENTOS_CHECKPOINT_DATABASE_URL",
        )
    return store
