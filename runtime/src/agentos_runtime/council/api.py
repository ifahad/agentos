"""HTTP API for the council. Mounted on the runtime app behind require_auth.

Every route resolves the council state via get_council, which returns 503 when
the council is not configured (no council.yaml, or no checkpoint database), so
the endpoints degrade rather than crash on a runtime that has the council off.
"""

from __future__ import annotations

from typing import Annotated, Any

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel, Field

from agentos_runtime.council.loop import STOP_CANCELLED, run_objective

router = APIRouter(prefix="/council")

COUNCIL_DISABLED = (
    "council is not configured (set AGENTOS_COUNCIL_CONFIG and a checkpoint database)"
)


class ObjectiveRequest(BaseModel):
    input: str = Field(min_length=1)
    max_cycles: int | None = Field(default=None, ge=1)
    budget_usd: float | None = Field(default=None, ge=0)


def get_council(request: Request) -> Any:
    """The council runtime state, or 503 when the council is not configured."""
    council = getattr(request.app.state, "council", None)
    if council is None:
        raise HTTPException(status_code=503, detail=COUNCIL_DISABLED)
    return council


CouncilDep = Annotated[Any, Depends(get_council)]


@router.post("/objectives", status_code=202)
async def create_objective(body: ObjectiveRequest, council: CouncilDep) -> dict:
    """Queue an objective. The heartbeat picks it up; POST /run forces it now."""
    objective = await council.store.create_objective(
        input_text=body.input,
        max_cycles=body.max_cycles,
        budget_usd=body.budget_usd
        if body.budget_usd is not None
        else council.default_budget_usd,
    )
    return {"id": objective["id"], "status": objective["status"]}


@router.get("/objectives")
async def list_objectives(council: CouncilDep) -> dict:
    return {"objectives": await council.store.list_objectives()}


@router.get("/objectives/{objective_id}")
async def get_objective(objective_id: str, council: CouncilDep) -> dict:
    """An objective plus its cycles, newest cycle last."""
    objective = await council.store.get_objective(objective_id)
    if objective is None:
        raise HTTPException(status_code=404, detail="objective not found")
    cycles = await council.store.list_cycles(objective_id)
    return {"objective": objective, "cycles": cycles}


@router.post("/objectives/{objective_id}/cancel")
async def cancel_objective(objective_id: str, council: CouncilDep) -> dict:
    """Request cancellation. The loop checks this before each cycle and stops."""
    objective = await council.store.get_objective(objective_id)
    if objective is None:
        raise HTTPException(status_code=404, detail="objective not found")
    await council.store.update_objective(
        objective_id, status=STOP_CANCELLED, stop_reason=STOP_CANCELLED
    )
    return {"id": objective_id, "status": STOP_CANCELLED}


@router.post("/objectives/{objective_id}/run")
async def run_objective_now(objective_id: str, council: CouncilDep) -> dict:
    """Run an objective synchronously and return its stop reason.

    Defaults to a single cycle unless the objective set its own max_cycles, so a
    manual run is a bounded, inspectable step rather than a full autonomous loop.
    """
    objective = await council.store.get_objective(objective_id)
    if objective is None:
        raise HTTPException(status_code=404, detail="objective not found")
    if objective.get("max_cycles") is None:
        objective = {**objective, "max_cycles": 1}
    reason = await run_objective(council.deps, objective)
    refreshed = await council.store.get_objective(objective_id)
    return {"id": objective_id, "stop_reason": reason, "objective": refreshed}


@router.post("/objectives/run")
async def create_and_run_objective(body: ObjectiveRequest, council: CouncilDep) -> dict:
    """Create an objective and run it to a verdict synchronously.

    This is what the gateway's council/* model calls: one full objective, not a
    queued one. It defaults to a single cycle — a synchronous OpenAI-style call
    must return in one round, not loop while an HTTP client waits — and the same
    caps still apply (budget ceiling, pause).
    """
    objective = await council.store.create_objective(
        input_text=body.input,
        max_cycles=body.max_cycles or 1,
        budget_usd=body.budget_usd
        if body.budget_usd is not None
        else council.default_budget_usd,
    )
    reason = await run_objective(council.deps, objective)
    cycles = await council.store.list_cycles(objective["id"])
    last = cycles[-1] if cycles else {}
    verdict = dict(last.get("verdict") or {})
    verdict["agreement"] = last.get("agreement", 0.0)
    verdict["dissent"] = last.get("dissent", [])
    final = await council.store.get_objective(objective["id"])
    return {
        "objective": {"id": objective["id"], "stop_reason": reason},
        "verdict": verdict,
        "spend_usd": float((final or {}).get("spend_usd") or 0.0),
    }


@router.post("/pause")
async def pause(council: CouncilDep) -> dict:
    await council.store.set_paused(True)
    return {"paused": True}


@router.post("/resume")
async def resume(council: CouncilDep) -> dict:
    await council.store.set_paused(False)
    return {"paused": False}


@router.get("/members")
async def list_members(council: CouncilDep) -> dict:
    """The configured members. Never returns a model's credential — only its id,
    model name, enabled flag, and profile."""
    return {
        "members": [
            {
                "id": m.id,
                "model": m.model,
                "enabled": m.enabled,
                "profile": m.profile,
            }
            for m in council.config.members
        ]
    }


@router.get("/proposals")
async def list_proposals(council: CouncilDep, status: str | None = None) -> dict:
    return {"proposals": await council.store.list_proposals(status=status)}


@router.post("/proposals/{proposal_id}/approve")
async def approve_proposal(proposal_id: str, council: CouncilDep) -> dict:
    """Approve a held write-class action.

    Approval records the human decision. It does NOT execute the tool: the
    member's graph already moved past the denied call, so acting on an approved
    proposal is a separate operator step. Recording the decision is what makes
    the action surface auditable.
    """
    proposal = await council.store.get_proposal(proposal_id)
    if proposal is None:
        raise HTTPException(status_code=404, detail="proposal not found")
    await council.store.update_proposal_status(proposal_id, "approved")
    return {"id": proposal_id, "status": "approved"}
