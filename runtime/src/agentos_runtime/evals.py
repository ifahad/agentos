"""Eval suites: YAML loading, agent scoring, and the /evals endpoints.

Suites live at ``runtime/evals/<suite>.yaml`` as
``cases: [{name, input, expect_substring, expect_tool?, judge?}]``. A case
passes when the run completes (a run stuck in ``pending_approval`` fails), the
expected substring appears case-insensitively in the final output, and — if
``expect_tool`` is set — that tool appears in the run's steps.

A case may additionally carry ``judge: {criteria, threshold}``: after the
substring/tool checks the judge model (AGENTOS_JUDGE_MODEL, routed through the
gateway) grades the agent's answer against the criteria as strict JSON
``{"score": 0..1, "justification": "..."}``; the case then also requires
``score >= threshold``. ``use_judge: false`` on POST /evals/run skips all
judge calls. A judge that fails or stays unparseable after one retry fails the
case with justification "judge unavailable" — never the whole suite.
"""

import json
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Annotated, Any

import yaml
from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel

from agentos_runtime.hitl import RunOutcome, run_until_settled
from agentos_runtime.messages import extract_output, extract_tool_calls, message_text
from agentos_runtime.store import ImprovementStore, get_store

_SOURCE_EVALS_DIR = Path(__file__).resolve().parents[2] / "evals"
# Fall back to <cwd>/evals when the package is installed outside the repo
# (e.g. non-editable in a container whose WORKDIR holds the evals/ copy).
EVALS_DIR = _SOURCE_EVALS_DIR if _SOURCE_EVALS_DIR.is_dir() else Path.cwd() / "evals"
SNIPPET_CHARS = 200

router = APIRouter()


@dataclass
class JudgeSpec:
    """LLM-judge grading spec: free-text criteria plus a pass threshold."""

    criteria: str
    threshold: float


@dataclass
class EvalCase:
    """One suite case: send ``input``, expect substring (and optionally a tool)."""

    name: str
    input: str
    expect_substring: str
    expect_tool: str | None = None
    judge: JudgeSpec | None = None


class EvalRunRequest(BaseModel):
    suite: str = "default"
    prompt_override: str | None = None
    use_judge: bool = True


def load_suite(suite: str, evals_dir: Path | None = None) -> list[EvalCase]:
    """Parse ``<evals_dir>/<suite>.yaml``; raises FileNotFoundError when absent."""
    path = (evals_dir or EVALS_DIR) / f"{suite}.yaml"
    if not path.is_file():
        raise FileNotFoundError(str(path))
    data = yaml.safe_load(path.read_text()) or {}
    cases: list[EvalCase] = []
    for case in data.get("cases", []):
        judge_block = case.get("judge")
        judge = (
            JudgeSpec(
                criteria=judge_block["criteria"],
                threshold=float(judge_block["threshold"]),
            )
            if judge_block
            else None
        )
        cases.append(
            EvalCase(
                name=case["name"],
                input=case["input"],
                expect_substring=case["expect_substring"],
                expect_tool=case.get("expect_tool"),
                judge=judge,
            )
        )
    return cases


JUDGE_TEMPLATE = """\
You are grading an AI agent's answer against evaluation criteria.

Question given to the agent:
{input}

Agent's answer:
{output}

Criteria:
{criteria}

Respond with ONLY a JSON object, no markdown fences, of the shape:
{{"score": <number between 0.0 and 1.0>, "justification": "<one short sentence>"}}
"""

JUDGE_UNAVAILABLE = "judge unavailable"


def parse_judge_reply(text: str) -> tuple[float, str] | None:
    """Extract (score, justification) from a judge reply; None when unusable.

    Tolerates surrounding prose or code fences by parsing the outermost
    ``{...}`` span. The score must be a number in [0, 1].
    """
    start = text.find("{")
    end = text.rfind("}")
    if start == -1 or end <= start:
        return None
    try:
        data = json.loads(text[start : end + 1])
    except json.JSONDecodeError:
        return None
    if not isinstance(data, dict):
        return None
    score = data.get("score")
    if isinstance(score, bool) or not isinstance(score, (int, float)):
        return None
    if not 0.0 <= score <= 1.0:
        return None
    justification = data.get("justification")
    return float(score), justification if isinstance(justification, str) else ""


async def judge_case(model: Any, case: EvalCase, output: str) -> tuple[float | None, str]:
    """Grade one output against the case's judge criteria (one retry).

    Plain ``ainvoke`` on the judge chat model — never the agent. A call that
    fails or stays unparseable after the retry returns (None, "judge
    unavailable") so a broken judge fails the case, not the suite.
    """
    assert case.judge is not None
    prompt = JUDGE_TEMPLATE.format(
        input=case.input, output=output, criteria=case.judge.criteria
    )
    for _ in range(2):
        try:
            response = await model.ainvoke(prompt)
        except Exception:  # noqa: BLE001 - judge failures degrade to a failed case
            continue
        parsed = parse_judge_reply(message_text(response))
        if parsed is not None:
            return parsed
    return None, JUDGE_UNAVAILABLE


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
    agent: Any,
    cases: list[EvalCase],
    approval_tools: list[str],
    judge_model: Any | None = None,
    use_judge: bool = True,
) -> dict[str, Any]:
    """Run every case on a fresh thread; a crashed case counts as failed.

    Cases with a ``judge`` block are additionally graded by ``judge_model``
    (skipped entirely when ``use_judge`` is false, in which case they score on
    substring/tool checks alone). ``judge_score``/``judge_justification`` are
    always present in each result, None when no judging happened.
    """
    results: list[dict[str, Any]] = []
    for case in cases:
        config = {"configurable": {"thread_id": f"eval-{uuid.uuid4().hex}"}}
        state = {"messages": [{"role": "user", "content": case.input}]}
        judge_score: float | None = None
        judge_justification: str | None = None
        try:
            outcome = await run_until_settled(agent, state, config, approval_tools)
        except Exception as exc:  # noqa: BLE001 - a crashed case is a failed case
            results.append(
                {
                    "name": case.name,
                    "passed": False,
                    "output_snippet": str(exc)[:SNIPPET_CHARS],
                    "judge_score": None,
                    "judge_justification": None,
                }
            )
            continue
        passed, snippet = score_case(case, outcome)
        if case.judge is not None and use_judge and outcome.status == "completed":
            if judge_model is None:
                judge_score, judge_justification = None, JUDGE_UNAVAILABLE
            else:
                output = extract_output(outcome.values.get("messages", []))
                judge_score, judge_justification = await judge_case(
                    judge_model, case, output
                )
            passed = passed and (
                judge_score is not None and judge_score >= case.judge.threshold
            )
        results.append(
            {
                "name": case.name,
                "passed": passed,
                "output_snippet": snippet,
                "judge_score": judge_score,
                "judge_justification": judge_justification,
            }
        )
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
    judge_model: Any | None = None,
    use_judge: bool = True,
) -> dict[str, Any]:
    """Run a suite and persist the result; returns the /evals/run body."""
    result = await run_suite(
        agent, cases, approval_tools, judge_model=judge_model, use_judge=use_judge
    )
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
    judge_model = getattr(http_request.app.state, "judge_model", None)
    return await run_and_record(
        store,
        agent,
        cases,
        request.suite,
        approval_tools,
        prompt_source,
        judge_model=judge_model,
        use_judge=request.use_judge,
    )


@router.get("/evals/runs")
async def evals_runs(
    store: Annotated[ImprovementStore, Depends(get_store)], limit: int = 20
) -> list[dict[str, Any]]:
    return await store.list_eval_runs(limit)
