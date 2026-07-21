"""Eval suites: YAML loading, agent scoring, and the /evals endpoints.

Suites live at ``runtime/evals/<suite>.yaml`` as
``cases: [{name, input, expect_substring, expect_tool?}]``. A case passes when
the run completes (a run stuck in ``pending_approval`` fails), the expected
substring appears case-insensitively in the final output, and — if
``expect_tool`` is set — that tool appears in the run's steps.
"""

import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Annotated, Any

import yaml
from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel

from agentos_runtime.hitl import RunOutcome, run_until_settled
from agentos_runtime.messages import extract_output, extract_tool_calls
from agentos_runtime.store import ImprovementStore, get_store

_SOURCE_EVALS_DIR = Path(__file__).resolve().parents[2] / "evals"
# Fall back to <cwd>/evals when the package is installed outside the repo
# (e.g. non-editable in a container whose WORKDIR holds the evals/ copy).
EVALS_DIR = _SOURCE_EVALS_DIR if _SOURCE_EVALS_DIR.is_dir() else Path.cwd() / "evals"
SNIPPET_CHARS = 200

router = APIRouter()


@dataclass
class EvalCase:
    """One suite case: send ``input``, expect substring (and optionally a tool)."""

    name: str
    input: str
    expect_substring: str
    expect_tool: str | None = None


class EvalRunRequest(BaseModel):
    suite: str = "default"
    prompt_override: str | None = None


def load_suite(suite: str, evals_dir: Path | None = None) -> list[EvalCase]:
    """Parse ``<evals_dir>/<suite>.yaml``; raises FileNotFoundError when absent."""
    path = (evals_dir or EVALS_DIR) / f"{suite}.yaml"
    if not path.is_file():
        raise FileNotFoundError(str(path))
    data = yaml.safe_load(path.read_text()) or {}
    return [
        EvalCase(
            name=case["name"],
            input=case["input"],
            expect_substring=case["expect_substring"],
            expect_tool=case.get("expect_tool"),
        )
        for case in data.get("cases", [])
    ]


def score_case(case: EvalCase, outcome: RunOutcome) -> tuple[bool, str]:
    """(passed, output_snippet) for one settled run against its case."""
    if outcome.status != "completed":
        return False, "run stuck in pending_approval"
    messages = outcome.values.get("messages", [])
    output = extract_output(messages)
    passed = case.expect_substring.lower() in output.lower()
    if case.expect_tool is not None:
        tools_used = [name for name, _ in extract_tool_calls(messages)]
        passed = passed and case.expect_tool in tools_used
    return passed, output[:SNIPPET_CHARS]


async def run_suite(
    agent: Any, cases: list[EvalCase], approval_tools: list[str]
) -> dict[str, Any]:
    """Run every case on a fresh thread; a crashed case counts as failed."""
    results: list[dict[str, Any]] = []
    for case in cases:
        config = {"configurable": {"thread_id": f"eval-{uuid.uuid4().hex}"}}
        state = {"messages": [{"role": "user", "content": case.input}]}
        try:
            outcome = await run_until_settled(agent, state, config, approval_tools)
        except Exception as exc:  # noqa: BLE001 - a crashed case is a failed case
            results.append(
                {"name": case.name, "passed": False, "output_snippet": str(exc)[:SNIPPET_CHARS]}
            )
            continue
        passed, snippet = score_case(case, outcome)
        results.append({"name": case.name, "passed": passed, "output_snippet": snippet})
    passed_count = sum(1 for r in results if r["passed"])
    return {
        "score": passed_count / len(results) if results else 0.0,
        "passed": passed_count,
        "failed": len(results) - passed_count,
        "cases": results,
    }


async def run_and_record(
    store: ImprovementStore,
    agent: Any,
    cases: list[EvalCase],
    suite: str,
    approval_tools: list[str],
    prompt_source: str,
) -> dict[str, Any]:
    """Run a suite and persist the result; returns the /evals/run body."""
    result = await run_suite(agent, cases, approval_tools)
    row = await store.insert_eval_run(
        suite=suite,
        score=result["score"],
        passed=result["passed"],
        failed=result["failed"],
        cases=result["cases"],
        prompt_source=prompt_source,
    )
    return {
        "run_id": row["run_id"],
        "suite": suite,
        "score": result["score"],
        "passed": result["passed"],
        "failed": result["failed"],
        "cases": result["cases"],
    }


def get_agent_builder(request: Request):
    """FastAPI dependency: prompt -> agent factory mounted by the lifespan."""
    builder = getattr(request.app.state, "agent_builder", None)
    if builder is None:
        raise HTTPException(status_code=503, detail="agent not initialized")
    return builder


def load_suite_or_404(suite: str) -> list[EvalCase]:
    try:
        return load_suite(suite)
    except FileNotFoundError as exc:
        raise HTTPException(status_code=404, detail=f"unknown suite: {suite}") from exc


@router.post("/evals/run")
async def evals_run(
    request: EvalRunRequest,
    store: Annotated[ImprovementStore, Depends(get_store)],
    builder: Annotated[Any, Depends(get_agent_builder)],
    http_request: Request,
) -> dict[str, Any]:
    cases = load_suite_or_404(request.suite)
    if request.prompt_override:
        agent = builder(request.prompt_override)
        prompt_source = "override"
    else:
        agent = getattr(http_request.app.state, "agent", None)
        if agent is None:
            raise HTTPException(status_code=503, detail="agent not initialized")
        prompt_source = "active"
    approval_tools = getattr(http_request.app.state, "approval_tools", []) or []
    return await run_and_record(store, agent, cases, request.suite, approval_tools, prompt_source)


@router.get("/evals/runs")
async def evals_runs(
    store: Annotated[ImprovementStore, Depends(get_store)], limit: int = 20
) -> list[dict[str, Any]]:
    return await store.list_eval_runs(limit)
