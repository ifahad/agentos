"""Shared test fakes: tool-calling model, settings factory, query tool,
scripted eval agent, in-memory improvement store, reflection chat model."""

from types import SimpleNamespace
from typing import Any

from langchain_core.language_models import BaseChatModel
from langchain_core.messages import AIMessage, HumanMessage, ToolMessage
from langchain_core.outputs import ChatGeneration, ChatResult
from langchain_core.tools import tool

from agentos_runtime.config import Settings

# Shared runtime auth token for tests. conftest.py exports it into the
# environment so the app-wide auth dependency accepts it; AUTH_HEADERS is what
# every test client sends. See tests/conftest.py.
RUNTIME_AUTH_TOKEN = "test-runtime-token"
AUTH_HEADERS = {"Authorization": f"Bearer {RUNTIME_AUTH_TOKEN}"}

QUERY_RESULT = '{"columns": ["n"], "rows": [[1]], "row_count": 1, "truncated": false}'


@tool
def query(sql: str) -> str:
    """Run a read-only SQL query against the legacy database."""
    return QUERY_RESULT


@tool
def lookup(key: str) -> str:
    """Look up a value in the reference data."""
    return "value-for-" + key


class FakeToolCallingModel(BaseChatModel):
    """Returns a scripted sequence of AIMessages; bind_tools is a no-op."""

    responses: list[AIMessage]
    call_index: int = 0

    @property
    def _llm_type(self) -> str:
        return "fake-tool-calling-model"

    def bind_tools(self, tools: Any, **kwargs: Any) -> "FakeToolCallingModel":
        return self

    def _generate(self, messages, stop=None, run_manager=None, **kwargs) -> ChatResult:
        message = self.responses[min(self.call_index, len(self.responses) - 1)]
        self.call_index += 1
        return ChatResult(generations=[ChatGeneration(message=message)])


def make_settings(**overrides: Any) -> Settings:
    values: dict[str, Any] = {
        "gateway_url": "http://gateway:8080",
        "gateway_key": "agos-test",
    }
    values.update(overrides)
    return Settings(_env_file=None, **values)


def tool_call(name: str, args: dict[str, Any], call_id: str = "call_1") -> dict[str, Any]:
    return {"name": name, "args": args, "id": call_id, "type": "tool_call"}


def make_model(*responses: AIMessage) -> FakeToolCallingModel:
    return FakeToolCallingModel(responses=list(responses))


def query_then_answer(answer: str = "answer") -> FakeToolCallingModel:
    return make_model(
        AIMessage(content="", tool_calls=[tool_call("query", {"sql": "SELECT 1"})]),
        AIMessage(content=answer),
    )


class ScriptedAgent:
    """Eval-test agent: maps input substrings to (final output, tools used).

    Keys are matched case-insensitively in script order, so a catch-all ``""``
    key must come last. ``stuck=True`` simulates a thread parked at an approval
    interrupt: the state snapshot reports a pending ``tools`` node and the run
    never produces a final answer.
    """

    def __init__(
        self, script: dict[str, tuple[str, list[str]]], stuck: bool = False
    ) -> None:
        self.script = script
        self.stuck = stuck
        self.threads: dict[str, dict[str, Any]] = {}

    def _lookup(self, text: str) -> tuple[str, list[str]]:
        for key, (output, tools) in self.script.items():
            if key.lower() in text.lower():
                return output, tools
        return "", []

    async def ainvoke(self, state: dict[str, Any], config: dict[str, Any] | None = None):
        thread_id = (config or {})["configurable"]["thread_id"]
        text = state["messages"][0]["content"]
        output, tools = self._lookup(text)
        messages: list[Any] = [HumanMessage(content=text)]
        if tools:
            calls = [tool_call(name, {"arg": "x"}, f"call_{i}") for i, name in enumerate(tools)]
            messages.append(AIMessage(content="", tool_calls=calls))
            if not self.stuck:
                messages.extend(
                    ToolMessage(content="tool-result", tool_call_id=c["id"], name=c["name"])
                    for c in calls
                )
        if not self.stuck:
            messages.append(AIMessage(content=output))
        values = {"messages": messages}
        self.threads[thread_id] = values
        return values

    async def aget_state(self, config: dict[str, Any]):
        values = self.threads.get(config["configurable"]["thread_id"], {"messages": []})
        return SimpleNamespace(values=values, next=("tools",) if self.stuck else ())


class FakeChatModel:
    """Reflection-model fake: pops one scripted reply (or exception) per ainvoke."""

    def __init__(self, replies: list[str | Exception]) -> None:
        self.replies = list(replies)
        self.prompts: list[str] = []

    async def ainvoke(self, prompt: str) -> AIMessage:
        self.prompts.append(prompt)
        reply = self.replies.pop(0)
        if isinstance(reply, Exception):
            raise reply
        return AIMessage(content=reply)


JUDGE_PASS_REPLY = '{"score": 0.9, "justification": "meets the criteria"}'


class FakeJudge:
    """Judge-model fake: replies in order, last entry repeats forever.

    Entries may be reply strings (returned as AIMessages) or exceptions
    (raised), so both the retry and the judge-unavailable paths are scriptable.
    """

    def __init__(self, *replies: str | Exception) -> None:
        self.replies = list(replies)
        self.prompts: list[str] = []

    async def ainvoke(self, prompt: str) -> AIMessage:
        self.prompts.append(prompt)
        reply = self.replies[min(len(self.prompts) - 1, len(self.replies) - 1)]
        if isinstance(reply, Exception):
            raise reply
        return AIMessage(content=reply)


class InMemoryImprovementStore:
    """In-memory stand-in matching the ImprovementStore method surface."""

    def __init__(self) -> None:
        self.eval_runs: list[dict[str, Any]] = []
        self.proposals: dict[str, dict[str, Any]] = {}
        self.active: dict[str, Any] | None = None
        self._counter = 0

    def _next(self, prefix: str) -> str:
        self._counter += 1
        return f"{prefix}-{self._counter}"

    async def insert_eval_run(
        self,
        suite: str,
        score: float,
        passed: int,
        failed: int,
        cases: list[dict[str, Any]],
        prompt_source: str,
    ) -> dict[str, Any]:
        row = {
            "run_id": self._next("run"),
            "suite": suite,
            "score": score,
            "passed": passed,
            "failed": failed,
            "cases": cases,
            "prompt_source": prompt_source,
            "created_at": f"2026-07-21T00:00:{self._counter:02d}+00:00",
        }
        self.eval_runs.append(row)
        return dict(row)

    async def list_eval_runs(self, limit: int = 20) -> list[dict[str, Any]]:
        rows = [
            {k: v for k, v in row.items() if k != "cases"}
            for row in reversed(self.eval_runs)
        ]
        return rows[:limit]

    async def latest_eval_run(self) -> dict[str, Any] | None:
        return dict(self.eval_runs[-1]) if self.eval_runs else None

    async def insert_proposal(
        self,
        prompt_text: str,
        rationale: str,
        baseline_score: float,
        candidate_score: float,
        status: str,
    ) -> dict[str, Any]:
        row = {
            "id": self._next("prop"),
            "prompt_text": prompt_text,
            "rationale": rationale,
            "baseline_score": baseline_score,
            "candidate_score": candidate_score,
            "status": status,
            "created_at": f"2026-07-21T00:00:{self._counter + 1:02d}+00:00",
        }
        self.proposals[row["id"]] = row
        return dict(row)

    async def get_proposal(self, proposal_id: str) -> dict[str, Any] | None:
        row = self.proposals.get(proposal_id)
        return dict(row) if row else None

    async def list_proposals(self, limit: int = 20) -> list[dict[str, Any]]:
        return [dict(row) for row in reversed(list(self.proposals.values()))][:limit]

    async def update_proposal_status(self, proposal_id: str, status: str) -> None:
        self.proposals[proposal_id]["status"] = status

    async def get_active_prompt(self) -> dict[str, Any] | None:
        return dict(self.active) if self.active else None

    async def set_active_prompt(self, prompt_text: str, proposal_id: str) -> None:
        self.active = {
            "prompt_text": prompt_text,
            "proposal_id": proposal_id,
            "updated_at": "2026-07-21T00:00:00+00:00",
        }


class FakeCouncilStore:
    """In-memory CouncilStore with the same method surface as the real one."""

    def __init__(self) -> None:
        self.objectives: dict[str, dict] = {}
        self.cycles: list[dict] = []
        self.member_runs: list[dict] = []
        self.proposals: dict[str, dict] = {}
        self.paused = False
        self._seq = 0

    def _next_id(self, prefix: str) -> str:
        self._seq += 1
        return f"{prefix}-{self._seq}"

    async def create_objective(self, input_text, max_cycles=None, budget_usd=5.0):
        oid = self._next_id("obj")
        obj = {
            "id": oid, "input": input_text, "status": "pending", "stop_reason": None,
            "cycles_run": 0, "spend_usd": 0.0, "max_cycles": max_cycles,
            "budget_usd": budget_usd, "claimed_by": None,
        }
        self.objectives[oid] = obj
        return dict(obj)

    async def get_objective(self, objective_id):
        obj = self.objectives.get(objective_id)
        return dict(obj) if obj else None

    async def list_objectives(self, limit=20):
        return [dict(o) for o in list(self.objectives.values())[:limit]]

    async def claim_next_objective(self, worker):
        for obj in self.objectives.values():
            if obj["status"] == "pending":
                obj["status"] = "running"
                obj["claimed_by"] = worker
                return dict(obj)
        return None

    async def update_objective(self, objective_id, **fields):
        obj = self.objectives[objective_id]
        for key, value in fields.items():
            if value is not None:
                obj[key] = value

    async def insert_cycle(self, objective_id, cycle_no, verdict, agreement, dissent):
        cid = self._next_id("cycle")
        self.cycles.append({
            "id": cid, "objective_id": objective_id, "cycle_no": cycle_no,
            "verdict": verdict, "agreement": agreement, "dissent": dissent,
        })
        return cid

    async def insert_member_run(self, cycle_id, member_id, model_used, thread_id,
                                status, output, steps, cost_usd, error):
        self.member_runs.append({
            "cycle_id": cycle_id, "member_id": member_id, "model_used": model_used,
            "thread_id": thread_id, "status": status, "output": output,
            "steps": steps, "cost_usd": cost_usd, "error": error,
        })

    async def list_cycles(self, objective_id):
        return [dict(c) for c in self.cycles if c["objective_id"] == objective_id]

    async def insert_proposal(self, objective_id, member_id, tool, arguments):
        pid = self._next_id("prop")
        self.proposals[pid] = {
            "id": pid, "objective_id": objective_id, "member_id": member_id,
            "tool": tool, "arguments": arguments, "status": "pending",
        }
        return pid

    async def get_proposal(self, proposal_id):
        prop = self.proposals.get(proposal_id)
        return dict(prop) if prop else None

    async def list_proposals(self, status=None, limit=50):
        out = [dict(p) for p in self.proposals.values()
               if status is None or p["status"] == status]
        return out[:limit]

    async def update_proposal_status(self, proposal_id, status):
        self.proposals[proposal_id]["status"] = status

    async def set_paused(self, paused):
        self.paused = paused

    async def is_paused(self):
        return self.paused
