"""Prompt store endpoints: active prompt, proposal listing, approve/deny.

Approving a proposal is the ONLY path that activates a prompt change: it
writes ``active_prompt`` and hot-swaps the live agent (the graph is rebuilt
with the new prompt, keeping tools and checkpointer via the mounted
``agent_builder``). Approving a ``failed_evals`` proposal is allowed as a
human override but flagged with a warning.
"""

from typing import Annotated, Any

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel

from agentos_runtime.agent import SYSTEM_PROMPT
from agentos_runtime.evals import get_agent_builder
from agentos_runtime.store import ImprovementStore, get_store

DECIDABLE_STATUSES = ("passed_evals", "failed_evals")
BELOW_BASELINE_WARNING = "candidate scored below baseline"

router = APIRouter()


class ProposalDecision(BaseModel):
    approve: bool


@router.get("/prompts/active")
async def active_prompt(
    store: Annotated[ImprovementStore, Depends(get_store)],
) -> dict[str, Any]:
    row = await store.get_active_prompt()
    if row is None:
        return {"source": "default", "prompt": SYSTEM_PROMPT, "proposal_id": None}
    return {
        "source": "proposal",
        "prompt": row["prompt_text"],
        "proposal_id": row["proposal_id"],
    }


@router.get("/proposals")
async def list_proposals(
    store: Annotated[ImprovementStore, Depends(get_store)], limit: int = 20
) -> list[dict[str, Any]]:
    return await store.list_proposals(limit)


@router.post("/proposals/{proposal_id}/approve")
async def decide_proposal(
    proposal_id: str,
    decision: ProposalDecision,
    store: Annotated[ImprovementStore, Depends(get_store)],
    builder: Annotated[Any, Depends(get_agent_builder)],
    http_request: Request,
) -> dict[str, Any]:
    proposal = await store.get_proposal(proposal_id)
    if proposal is None:
        raise HTTPException(status_code=404, detail="unknown proposal")
    if proposal["status"] not in DECIDABLE_STATUSES:
        raise HTTPException(status_code=409, detail="proposal already decided")

    if not decision.approve:
        await store.update_proposal_status(proposal_id, "denied")
        return {**proposal, "status": "denied"}

    await store.update_proposal_status(proposal_id, "approved")
    await store.set_active_prompt(proposal["prompt_text"], proposal_id)
    state = http_request.app.state
    state.agent = builder(proposal["prompt_text"])  # hot-swap the live graph
    state.current_prompt = proposal["prompt_text"]
    body = {**proposal, "status": "approved"}
    if proposal["status"] == "failed_evals":
        body["warning"] = BELOW_BASELINE_WARNING
    return body
