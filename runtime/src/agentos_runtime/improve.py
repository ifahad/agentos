"""Self-improvement loop: POST /improve proposes an eval-gated prompt change.

Flow (never free-running; activation always requires human approval via
/proposals/{id}/approve): take the latest eval run as baseline (auto-running
the default suite if none exists), ask the model — plain ``ainvoke`` on the
gateway-backed chat model, not the agent — to reflect on the failed cases and
propose a new system prompt, auto-eval the candidate via prompt_override, and
persist the proposal. Status is ``passed_evals`` iff the candidate scored at
least the baseline.
"""

import json
from typing import Annotated, Any

from fastapi import APIRouter, Depends, HTTPException, Request

from agentos_runtime.agent import SYSTEM_PROMPT
from agentos_runtime.evals import (
    EvalCase,
    get_agent_builder,
    load_suite_or_404,
    run_and_record,
)
from agentos_runtime.messages import message_text
from agentos_runtime.store import ImprovementStore, get_store

DEFAULT_SUITE = "default"

REFLECTION_TEMPLATE = """\
You are improving the system prompt of an enterprise data-analyst agent.

Current system prompt:
---
{current_prompt}
---

The agent was evaluated on a fixed suite. Failed cases:
{failures}

Write an improved system prompt that fixes the failures while keeping the
agent's guarantees (read-only tools, no fabricated values, cite sources).
Respond with ONLY a JSON object, no markdown fences, of the shape:
{{"prompt": "<the full new system prompt>", "rationale": "<why this should score better>"}}
"""


def format_failures(
    failed_cases: list[dict[str, Any]], suite_cases: list[EvalCase]
) -> str:
    """Human-readable failed-case details, joining stored results with the suite spec."""
    if not failed_cases:
        return "(none — all cases passed)"
    by_name = {case.name: case for case in suite_cases}
    lines: list[str] = []
    for result in failed_cases:
        case = by_name.get(result.get("name", ""))
        lines.append(f"- case: {result.get('name')}")
        if case is not None:
            lines.append(f"  input: {case.input}")
            lines.append(f"  expected substring: {case.expect_substring}")
            if case.expect_tool:
                lines.append(f"  expected tool: {case.expect_tool}")
        lines.append(f"  observed output: {result.get('output_snippet', '')!r}")
    return "\n".join(lines)


def parse_reflection(text: str) -> tuple[str, str] | None:
    """Extract ("prompt", "rationale") from a model reply; None when unusable.

    Tolerates surrounding prose or code fences by parsing the outermost
    ``{...}`` span.
    """
    start = text.find("{")
    end = text.rfind("}")
    if start == -1 or end <= start:
        return None
    try:
        data = json.loads(text[start : end + 1])
    except json.JSONDecodeError:
        return None
    prompt = data.get("prompt") if isinstance(data, dict) else None
    rationale = data.get("rationale") if isinstance(data, dict) else None
    if isinstance(prompt, str) and prompt.strip() and isinstance(rationale, str):
        return prompt, rationale
    return None


async def reflect(model: Any, reflection_prompt: str) -> tuple[str, str]:
    """Call the chat model (one retry) and parse its proposal; 502 on failure."""
    detail = "no reflection response"
    for _ in range(2):
        try:
            response = await model.ainvoke(reflection_prompt)
        except Exception as exc:  # noqa: BLE001 - model failures surface as 502
            detail = str(exc)
            continue
        parsed = parse_reflection(message_text(response))
        if parsed is not None:
            return parsed
        detail = "unparseable reflection response"
    raise HTTPException(status_code=502, detail=detail)


def get_reflection_model(request: Request) -> Any:
    model = getattr(request.app.state, "reflection_model", None)
    if model is None:
        raise HTTPException(status_code=503, detail="reflection model not initialized")
    return model


router = APIRouter()


@router.post("/improve")
async def improve(
    store: Annotated[ImprovementStore, Depends(get_store)],
    builder: Annotated[Any, Depends(get_agent_builder)],
    model: Annotated[Any, Depends(get_reflection_model)],
    http_request: Request,
) -> dict[str, Any]:
    state = http_request.app.state
    approval_tools = getattr(state, "approval_tools", []) or []
    suite_cases = load_suite_or_404(DEFAULT_SUITE)

    baseline = await store.latest_eval_run()
    if baseline is None:
        agent = getattr(state, "agent", None)
        if agent is None:
            raise HTTPException(status_code=503, detail="agent not initialized")
        baseline = await run_and_record(
            store, agent, suite_cases, DEFAULT_SUITE, approval_tools, "active"
        )
    baseline_score = baseline["score"]
    failed_cases = [case for case in baseline.get("cases", []) if not case.get("passed")]

    current_prompt = getattr(state, "current_prompt", SYSTEM_PROMPT)
    reflection_prompt = REFLECTION_TEMPLATE.format(
        current_prompt=current_prompt,
        failures=format_failures(failed_cases, suite_cases),
    )
    candidate_prompt, rationale = await reflect(model, reflection_prompt)

    candidate_run = await run_and_record(
        store, builder(candidate_prompt), suite_cases, DEFAULT_SUITE, approval_tools, "candidate"
    )
    candidate_score = candidate_run["score"]

    status = "passed_evals" if candidate_score >= baseline_score else "failed_evals"
    return await store.insert_proposal(
        prompt_text=candidate_prompt,
        rationale=rationale,
        baseline_score=baseline_score,
        candidate_score=candidate_score,
        status=status,
    )
