"""FastAPI service exposing the agent as the Runtime HTTP API."""

import asyncio
import json
import logging
import os
import secrets
import uuid
from contextlib import asynccontextmanager, suppress
from types import SimpleNamespace
from typing import Annotated, Any

from fastapi import Depends, FastAPI, HTTPException, Request
from fastapi.responses import JSONResponse, StreamingResponse
from langchain_core.messages import AIMessage, BaseMessage, HumanMessage, ToolMessage
from pydantic import BaseModel

from agentos_runtime import evals, improve, prompts
from agentos_runtime.agent import (
    SYSTEM_PROMPT,
    build_agent,
    build_chat_model,
    get_checkpointer,
    load_mcp_tools,
)
from agentos_runtime.config import Settings
from agentos_runtime.context import build_context_engine, make_search_tool
from agentos_runtime.council import api as council_api
from agentos_runtime.hitl import (
    PendingCall,
    RunOutcome,
    deny_pending,
    pending_tool_calls,
    run_until_settled,
)
from agentos_runtime.messages import extract_output, message_text
from agentos_runtime.operators import api as operators_api
from agentos_runtime.otel import record_tool_spans, run_span, setup_tracing
from agentos_runtime.sandbox import sandbox_tools
from agentos_runtime.store import ImprovementStore

logger = logging.getLogger(__name__)

INTERNAL_ERROR = "internal error"
INVALID_TOKEN = "invalid runtime token"
# GET /healthz stays open so container/orchestrator probes need no credential.
OPEN_PATHS = frozenset({"/healthz"})
# Operator webhook fires authenticate by their opaque token in the path, not by
# the runtime bearer — an external system firing a webhook does not hold it. The
# token (whk-, 24 random bytes) is the credential, and an unknown token 404s.
OPEN_PREFIXES = ("/operators/webhooks/",)


def _expected_auth_token(request: Request) -> str:
    """The runtime token the caller must present.

    Prefers the value validated at startup (``app.state.runtime_auth_token``,
    set fail-closed in :func:`lifespan`) and falls back to the environment so
    the dependency works in tests that mount ``app`` without the lifespan.
    """
    state_token = getattr(request.app.state, "runtime_auth_token", "") or ""
    return state_token or os.environ.get("AGENTOS_RUNTIME_AUTH_TOKEN", "")


async def require_auth(request: Request) -> None:
    """App-wide dependency enforcing ``Authorization: Bearer <token>`` (finding C1).

    Applies to every route except ``GET /healthz``. Compares the presented
    token to the configured token with :func:`secrets.compare_digest`
    (constant-time). Any missing/malformed/mismatched token -> 401.
    """
    if request.url.path in OPEN_PATHS:
        return
    if any(request.url.path.startswith(prefix) for prefix in OPEN_PREFIXES):
        return
    expected = _expected_auth_token(request)
    header = request.headers.get("authorization", "")
    scheme, _, presented = header.partition(" ")
    if (
        not expected
        or scheme.lower() != "bearer"
        or not secrets.compare_digest(presented, expected)
    ):
        raise HTTPException(status_code=401, detail=INVALID_TOKEN)


class RunRequest(BaseModel):
    input: str
    thread_id: str | None = None


class Step(BaseModel):
    tool: str
    input: dict[str, Any]


class RunResponse(BaseModel):
    thread_id: str
    output: str
    steps: list[Step]
    status: str = "completed"


class ApproveRequest(BaseModel):
    approve: bool


class DocumentRequest(BaseModel):
    name: str
    text: str


@asynccontextmanager
async def lifespan(app: FastAPI):
    settings = Settings()
    # Fail closed: refuse to start without a runtime auth token (finding C1).
    app.state.runtime_auth_token = settings.require_runtime_auth_token()
    tools = await load_mcp_tools(settings)
    if settings.context_engine_enabled and settings.checkpoint_database_url:
        app.state.context_engine = build_context_engine(settings)
        tools = [*tools, make_search_tool(app.state.context_engine)]
    tools = [*tools, *sandbox_tools(settings)]
    app.state.approval_tools = settings.approval_tool_names
    app.state.tracer = setup_tracing(settings)
    async with get_checkpointer(settings) as checkpointer:
        active_prompt: str | None = None
        if settings.checkpoint_database_url:
            store = ImprovementStore(settings.checkpoint_database_url)
            app.state.improve_store = store
            active = await store.get_active_prompt()
            active_prompt = active["prompt_text"] if active else None
        else:
            app.state.improve_store = None

        # Skills: reviewed in-repo SKILL.md instructions the agent pulls on
        # demand. Loaded unconditionally (whether autonomy is on or not); a
        # missing directory simply yields no skills. The use_skill tool is added
        # to the toolset and the skill names are appended to the system prompt so
        # the model knows what exists without their full text bloating each turn.
        from agentos_runtime.operators.skills import (
            load_skills,
            make_use_skill_tool,
            skills_prompt_section,
        )

        skills_dir = settings.skills_dir or str(
            __import__("pathlib").Path(__file__).resolve().parents[2] / "skills"
        )
        skills = load_skills(skills_dir)
        agent_tools = list(tools)
        if skills:
            agent_tools.append(make_use_skill_tool(skills))
        skill_section = skills_prompt_section(skills)
        app.state.skills = skills

        def agent_builder(prompt: str | None):
            # Append the skills list AFTER the persona/default prompt, so it adds
            # to the instructions rather than replacing them. build_agent still
            # prepends the immutable SAFETY_PREAMBLE regardless.
            if skill_section:
                base = prompt if prompt is not None else SYSTEM_PROMPT
                effective: str | None = base + skill_section
            else:
                effective = prompt
            return build_agent(settings, agent_tools, checkpointer, prompt=effective)

        app.state.agent_builder = agent_builder
        app.state.reflection_model = build_chat_model(settings)
        app.state.judge_model = build_chat_model(settings, model=settings.judge_model)
        app.state.current_prompt = active_prompt or SYSTEM_PROMPT
        app.state.agent = agent_builder(active_prompt)

        # The council needs both a config file and a checkpoint database (its
        # store lives there). Missing either leaves app.state.council None, so
        # every /council route reports 503 rather than crashing.
        app.state.council = None
        app.state.council_task = None
        app.state.council_stop = None
        if settings.council_config and settings.checkpoint_database_url:
            from agentos_runtime.council.config import load_council_config
            from agentos_runtime.council.loop import CouncilDeps, heartbeat
            from agentos_runtime.council.store import CouncilStore

            council_config = load_council_config(settings.council_config)
            council_store = CouncilStore(settings.checkpoint_database_url)
            deps = CouncilDeps(
                config=council_config,
                settings=settings,
                tools=list(tools),
                checkpointer=checkpointer,
                store=council_store,
                judge_model=build_chat_model(settings, model=council_config.judge),
            )
            app.state.council = SimpleNamespace(
                config=council_config,
                store=council_store,
                deps=deps,
                default_budget_usd=settings.council_max_spend_usd,
            )
            # The autonomous loop is opt-in: it runs only with a positive
            # interval, so the default runtime serves the API but never acts on
            # its own.
            if settings.council_heartbeat_s > 0:
                stop_event = asyncio.Event()
                app.state.council_stop = stop_event
                app.state.council_task = asyncio.create_task(
                    heartbeat(deps, settings.council_heartbeat_s, stop_event)
                )
                logger.info("council heartbeat enabled (%ss)", settings.council_heartbeat_s)

        # Operators: the single-agent autonomy engine, parallel to the council.
        # Its store lives in the checkpoint DB, so it is available whenever that
        # is; the scheduler is separately opt-in via AGENTOS_AUTONOMY_ENABLED.
        app.state.operators = None
        app.state.operators_task = None
        app.state.operators_stop = None
        if settings.checkpoint_database_url:
            from agentos_runtime.operators.engine import OperatorDeps, Scheduler
            from agentos_runtime.operators.store import OperatorStore

            operator_store = OperatorStore(settings.checkpoint_database_url)
            operator_deps = OperatorDeps(
                store=operator_store,
                agent_builder=agent_builder,
                approval_tools=settings.approval_tool_names,
                max_cycles_default=settings.autonomy_max_cycles,
            )
            app.state.operators = SimpleNamespace(store=operator_store, deps=operator_deps)
            if settings.autonomy_enabled:
                ops_stop = asyncio.Event()
                app.state.operators_stop = ops_stop
                scheduler = Scheduler(operator_deps, tick_s=settings.autonomy_tick_s)
                app.state.operators_task = asyncio.create_task(scheduler.run(ops_stop))
                logger.info("operator scheduler enabled (tick=%ss)", settings.autonomy_tick_s)

        try:
            yield
        finally:
            for stop_attr, task_attr in (
                ("council_stop", "council_task"),
                ("operators_stop", "operators_task"),
            ):
                stop = getattr(app.state, stop_attr, None)
                task = getattr(app.state, task_attr, None)
                if stop is not None:
                    stop.set()
                if task is not None:
                    task.cancel()
                    with suppress(asyncio.CancelledError):
                        await task


app = FastAPI(
    title="agentos-runtime",
    lifespan=lifespan,
    dependencies=[Depends(require_auth)],  # every route except GET /healthz
)
app.include_router(evals.router)
app.include_router(improve.router)
app.include_router(prompts.router)
app.include_router(council_api.router)
app.include_router(operators_api.router)


def get_agent(request: Request):
    agent = getattr(request.app.state, "agent", None)
    if agent is None:
        raise HTTPException(status_code=503, detail="agent not initialized")
    return agent


def get_approval_tools(request: Request) -> list[str]:
    return getattr(request.app.state, "approval_tools", []) or []


def get_context_engine(request: Request):
    engine = getattr(request.app.state, "context_engine", None)
    if engine is None:
        raise HTTPException(status_code=503, detail="context engine disabled")
    return engine


def _extract_steps(messages: list[BaseMessage]) -> list[Step]:
    steps: list[Step] = []
    for message in messages:
        if isinstance(message, AIMessage):
            for tool_call in message.tool_calls or []:
                steps.append(Step(tool=tool_call["name"], input=dict(tool_call["args"])))
    return steps


def _pending_body(thread_id: str, pending: list[PendingCall]) -> dict[str, Any]:
    return {
        "status": "pending_approval",
        "thread_id": thread_id,
        "pending": [{"tool": call.tool, "input": call.input} for call in pending],
    }


def _run_response(thread_id: str, outcome: RunOutcome, tracer: Any | None) -> RunResponse:
    messages = outcome.values.get("messages", [])
    steps = _extract_steps(messages)
    record_tool_spans(tracer, steps)
    return RunResponse(
        thread_id=thread_id,
        output=extract_output(messages),
        steps=steps,
        status="completed",
    )


@app.get("/healthz")
async def healthz() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/runs")
async def runs(
    request: RunRequest,
    agent: Annotated[Any, Depends(get_agent)],
    approval_tools: Annotated[list[str], Depends(get_approval_tools)],
    http_request: Request,
):
    thread_id = request.thread_id or uuid.uuid4().hex
    config = {"configurable": {"thread_id": thread_id}}
    input_state = {"messages": [{"role": "user", "content": request.input}]}
    tracer = getattr(http_request.app.state, "tracer", None)
    with run_span(tracer, thread_id):
        try:
            if approval_tools:
                outcome = await run_until_settled(agent, input_state, config, approval_tools)
            else:
                values = await agent.ainvoke(input_state, config=config)
                outcome = RunOutcome(status="completed", values=values, pending=[])
        except Exception as exc:  # noqa: BLE001 - agent failures surface as 502
            logger.exception("run failed for thread %s", thread_id)
            raise HTTPException(status_code=502, detail=INTERNAL_ERROR) from exc
        if outcome.status == "pending_approval":
            return JSONResponse(
                status_code=202, content=_pending_body(thread_id, outcome.pending)
            )
        return _run_response(thread_id, outcome, tracer)


@app.post("/runs/{thread_id}/approve")
async def approve(
    thread_id: str,
    request: ApproveRequest,
    agent: Annotated[Any, Depends(get_agent)],
    approval_tools: Annotated[list[str], Depends(get_approval_tools)],
    http_request: Request,
):
    config = {"configurable": {"thread_id": thread_id}}
    snapshot = await agent.aget_state(config)
    if not snapshot.values or not snapshot.next:
        raise HTTPException(status_code=404, detail="no pending approval for thread")
    pending = pending_tool_calls(snapshot)
    if not pending:
        raise HTTPException(status_code=404, detail="no pending approval for thread")
    tracer = getattr(http_request.app.state, "tracer", None)
    with run_span(tracer, thread_id):
        try:
            if not request.approve:
                await deny_pending(agent, config, pending)
            outcome = await run_until_settled(agent, None, config, approval_tools)
        except Exception as exc:  # noqa: BLE001 - agent failures surface as 502
            logger.exception("approve/resume failed for thread %s", thread_id)
            raise HTTPException(status_code=502, detail=INTERNAL_ERROR) from exc
        if outcome.status == "pending_approval":
            return JSONResponse(
                status_code=202, content=_pending_body(thread_id, outcome.pending)
            )
        return _run_response(thread_id, outcome, tracer)


def _sse(payload: dict[str, Any]) -> str:
    return f"data: {json.dumps(payload)}\n\n"


@app.post("/runs/stream")
async def runs_stream(
    request: RunRequest,
    agent: Annotated[Any, Depends(get_agent)],
    approval_tools: Annotated[list[str], Depends(get_approval_tools)],
):
    thread_id = request.thread_id or uuid.uuid4().hex
    config = {"configurable": {"thread_id": thread_id}}

    async def events():
        steps: list[dict[str, Any]] = []
        output = ""
        input_state: dict[str, Any] | None = {
            "messages": [{"role": "user", "content": request.input}]
        }
        try:
            while True:
                async for update in agent.astream(
                    input_state, config=config, stream_mode="updates"
                ):
                    if not isinstance(update, dict):
                        continue
                    for node_output in update.values():
                        if not isinstance(node_output, dict):
                            continue
                        for message in node_output.get("messages") or []:
                            if not isinstance(message, AIMessage):
                                continue
                            output = message_text(message) or output
                            for tool_call in message.tool_calls or []:
                                step = {
                                    "tool": tool_call["name"],
                                    "input": dict(tool_call["args"]),
                                }
                                steps.append(step)
                                yield _sse({"event": "step", **step})
                if not approval_tools:
                    break
                snapshot = await agent.aget_state(config)
                if not snapshot.next:
                    break
                pending = pending_tool_calls(snapshot)
                if any(call.tool in approval_tools for call in pending):
                    yield _sse(
                        {
                            "event": "pending_approval",
                            "pending": [
                                {"tool": call.tool, "input": call.input}
                                for call in pending
                            ],
                        }
                    )
                    return
                input_state = None  # auto-resume past non-approval interrupt
        except Exception:  # noqa: BLE001 - surface agent failure in-stream
            logger.exception("stream failed for thread %s", thread_id)
            yield _sse({"event": "error", "message": INTERNAL_ERROR})
            return
        yield _sse(
            {"event": "done", "thread_id": thread_id, "output": output, "steps": steps}
        )

    return StreamingResponse(events(), media_type="text/event-stream")


@app.get("/threads/{thread_id}")
async def get_thread(
    thread_id: str, agent: Annotated[Any, Depends(get_agent)]
) -> dict[str, Any]:
    snapshot = await agent.aget_state({"configurable": {"thread_id": thread_id}})
    if not snapshot.values:
        raise HTTPException(status_code=404, detail="unknown thread")
    messages: list[dict[str, Any]] = []
    for message in snapshot.values.get("messages", []):
        if isinstance(message, HumanMessage):
            role = "user"
        elif isinstance(message, ToolMessage):
            role = "tool"
        elif isinstance(message, AIMessage):
            role = "assistant"
        else:
            continue
        entry: dict[str, Any] = {"role": role, "content": message_text(message)}
        if isinstance(message, AIMessage) and message.tool_calls:
            entry["tool_calls"] = [
                {"tool": tc["name"], "input": dict(tc["args"])} for tc in message.tool_calls
            ]
        messages.append(entry)
    return {"thread_id": thread_id, "messages": messages}


@app.post("/documents")
async def add_document(
    request: DocumentRequest, engine: Annotated[Any, Depends(get_context_engine)]
) -> dict[str, Any]:
    try:
        chunks = await engine.add_document(request.name, request.text)
    except Exception as exc:  # noqa: BLE001 - embed/store failures surface as 502
        logger.exception("add_document failed for %s", request.name)
        raise HTTPException(status_code=502, detail=INTERNAL_ERROR) from exc
    return {"name": request.name, "chunks": chunks}


@app.get("/documents")
async def list_documents(
    engine: Annotated[Any, Depends(get_context_engine)],
) -> list[dict[str, Any]]:
    try:
        return await engine.list_documents()
    except Exception as exc:  # noqa: BLE001 - store failures surface as 502
        logger.exception("list_documents failed")
        raise HTTPException(status_code=502, detail=INTERNAL_ERROR) from exc
