"""FastAPI service exposing the agent as the Runtime HTTP API."""

import json
import uuid
from contextlib import asynccontextmanager
from typing import Annotated, Any

from fastapi import Depends, FastAPI, HTTPException, Request
from fastapi.responses import JSONResponse, StreamingResponse
from langchain_core.messages import AIMessage, BaseMessage, HumanMessage, ToolMessage
from pydantic import BaseModel

from agentos_runtime.agent import build_agent, get_checkpointer, load_mcp_tools
from agentos_runtime.config import Settings
from agentos_runtime.context import build_context_engine, make_search_tool
from agentos_runtime.hitl import (
    PendingCall,
    RunOutcome,
    deny_pending,
    pending_tool_calls,
    run_until_settled,
)
from agentos_runtime.otel import record_tool_spans, run_span, setup_tracing


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
    tools = await load_mcp_tools(settings)
    if settings.context_engine_enabled and settings.checkpoint_database_url:
        app.state.context_engine = build_context_engine(settings)
        tools = [*tools, make_search_tool(app.state.context_engine)]
    app.state.approval_tools = settings.approval_tool_names
    app.state.tracer = setup_tracing(settings)
    async with get_checkpointer(settings) as checkpointer:
        app.state.agent = build_agent(settings, tools, checkpointer)
        yield


app = FastAPI(title="agentos-runtime", lifespan=lifespan)


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


def _message_text(message: BaseMessage) -> str:
    content = message.content
    if isinstance(content, str):
        return content
    parts: list[str] = []
    for block in content:
        if isinstance(block, str):
            parts.append(block)
        elif isinstance(block, dict) and block.get("type") == "text":
            parts.append(block.get("text", ""))
    return "".join(parts)


def _extract_output(messages: list[BaseMessage]) -> str:
    for message in reversed(messages):
        if isinstance(message, AIMessage):
            return _message_text(message)
    return ""


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
        output=_extract_output(messages),
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
            raise HTTPException(status_code=502, detail=str(exc)) from exc
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
            raise HTTPException(status_code=502, detail=str(exc)) from exc
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
                            output = _message_text(message) or output
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
        except Exception as exc:  # noqa: BLE001 - surface agent failure in-stream
            yield _sse({"event": "error", "message": str(exc)})
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
        entry: dict[str, Any] = {"role": role, "content": _message_text(message)}
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
        raise HTTPException(status_code=502, detail=str(exc)) from exc
    return {"name": request.name, "chunks": chunks}


@app.get("/documents")
async def list_documents(
    engine: Annotated[Any, Depends(get_context_engine)],
) -> list[dict[str, Any]]:
    try:
        return await engine.list_documents()
    except Exception as exc:  # noqa: BLE001 - store failures surface as 502
        raise HTTPException(status_code=502, detail=str(exc)) from exc
