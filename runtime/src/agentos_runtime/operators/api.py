"""HTTP API for operators. Mounted on the runtime app behind require_auth.

Every route resolves operator state via get_operators, which 503s when autonomy
is not configured (no checkpoint database), so the endpoints degrade rather than
crash. The webhook fire endpoint is the ONE exception to the app-wide auth: its
token IS the credential, so it authenticates by the path, not the bearer.
"""

from __future__ import annotations

from typing import Annotated, Any

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel, Field

from agentos_runtime.operators.engine import SOURCE_MANUAL, SOURCE_WEBHOOK, fire_operator
from agentos_runtime.operators.triggers import TriggerError, parse_trigger

router = APIRouter(prefix="/operators")

OPERATORS_DISABLED = "operators require a checkpoint database (AGENTOS_CHECKPOINT_DATABASE_URL)"


class OperatorRequest(BaseModel):
    name: str = Field(min_length=1)
    goal: str = Field(min_length=1)
    trigger: dict[str, Any]
    max_cycles: int | None = Field(default=None, ge=1)
    enabled: bool = True


class OperatorPatch(BaseModel):
    enabled: bool | None = None
    goal: str | None = Field(default=None, min_length=1)
    max_cycles: int | None = Field(default=None, ge=1)
    trigger: dict[str, Any] | None = None


def get_operators(request: Request) -> Any:
    state = getattr(request.app.state, "operators", None)
    if state is None:
        raise HTTPException(status_code=503, detail=OPERATORS_DISABLED)
    return state


OpsDep = Annotated[Any, Depends(get_operators)]


@router.post("", status_code=201)
async def create_operator(body: OperatorRequest, ops: OpsDep) -> dict:
    """Create a standing operator. The webhook token is returned ONLY here."""
    try:
        trigger = parse_trigger(body.trigger)
    except TriggerError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return await ops.store.create_operator(
        name=body.name,
        goal=body.goal,
        trigger_type=trigger.kind,
        trigger_config=trigger.to_config(),
        enabled=body.enabled,
        max_cycles=body.max_cycles or ops.deps.max_cycles_default,
    )


@router.get("")
async def list_operators(ops: OpsDep) -> dict:
    return {"operators": await ops.store.list_operators()}


@router.get("/skills")
async def list_skills(request: Request) -> dict:
    """The loaded SKILL.md skills — names, descriptions, and provenance hashes.

    Declared before /{operator_id} so the static path wins over the parameter.
    Reads app.state.skills directly (skills load unconditionally, independent of
    the operator store), so this works even when autonomy has no checkpoint DB.
    Never returns a skill body — that is fetched by the agent via use_skill.
    """
    skills = getattr(request.app.state, "skills", None) or {}
    return {
        "skills": [
            {
                "name": s.name,
                "description": s.description,
                "when_to_use": s.when_to_use,
                "sha256": s.sha256,
            }
            for s in skills.values()
        ]
    }


@router.get("/{operator_id}")
async def get_operator(operator_id: str, ops: OpsDep) -> dict:
    operator = await ops.store.get_operator(operator_id)
    if operator is None:
        raise HTTPException(status_code=404, detail="operator not found")
    return {"operator": operator, "recent_runs": await ops.store.list_runs(operator_id, limit=10)}


@router.patch("/{operator_id}")
async def patch_operator(operator_id: str, body: OperatorPatch, ops: OpsDep) -> dict:
    existing = await ops.store.get_operator(operator_id)
    if existing is None:
        raise HTTPException(status_code=404, detail="operator not found")
    trigger_type = None
    trigger_config = None
    if body.trigger is not None:
        try:
            trigger = parse_trigger(body.trigger)
        except TriggerError as exc:
            raise HTTPException(status_code=400, detail=str(exc)) from exc
        trigger_type, trigger_config = trigger.kind, trigger.to_config()
    updated = await ops.store.update_operator(
        operator_id,
        enabled=body.enabled,
        goal=body.goal,
        max_cycles=body.max_cycles,
        trigger_type=trigger_type,
        trigger_config=trigger_config,
    )
    return {"operator": updated}


@router.delete("/{operator_id}", status_code=204)
async def delete_operator(operator_id: str, ops: OpsDep) -> None:
    if not await ops.store.delete_operator(operator_id):
        raise HTTPException(status_code=404, detail="operator not found")


@router.post("/{operator_id}/run")
async def run_operator_now(operator_id: str, ops: OpsDep) -> dict:
    """Fire an operator once, right now (manual trigger)."""
    operator = await ops.store.get_operator(operator_id)
    if operator is None:
        raise HTTPException(status_code=404, detail="operator not found")
    run = await fire_operator(ops.deps, operator, SOURCE_MANUAL)
    return {"run": run}


@router.get("/{operator_id}/runs")
async def list_runs(operator_id: str, ops: OpsDep, limit: int = 20) -> dict:
    return {"runs": await ops.store.list_runs(operator_id, limit=limit)}


@router.post("/webhooks/{token}")
async def fire_webhook(token: str, request: Request, ops: OpsDep) -> dict:
    """Fire a webhook operator by its token.

    The token is the secret — an unknown or disabled token is a 404, never a
    hint that some token exists. The request body is passed to the run as
    untrusted trigger context.
    """
    operator = await ops.store.find_by_webhook(token)
    if operator is None:
        raise HTTPException(status_code=404, detail="unknown webhook")
    try:
        payload = await request.json()
    except Exception:  # noqa: BLE001 - an empty or non-JSON body is fine
        payload = {}
    run = await fire_operator(ops.deps, operator, SOURCE_WEBHOOK, context=payload)
    return {"run": run}
