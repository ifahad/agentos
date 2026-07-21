"""FastAPI service exposing the agent as the Runtime HTTP API."""

import uuid
from contextlib import asynccontextmanager
from typing import Annotated, Any

from fastapi import Depends, FastAPI, HTTPException, Request
from langchain_core.messages import AIMessage, BaseMessage
from pydantic import BaseModel

from agentos_runtime.agent import build_agent, get_checkpointer, load_mcp_tools
from agentos_runtime.config import Settings


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


@asynccontextmanager
async def lifespan(app: FastAPI):
    settings = Settings()
    tools = await load_mcp_tools(settings)
    async with get_checkpointer(settings) as checkpointer:
        app.state.agent = build_agent(settings, tools, checkpointer)
        yield


app = FastAPI(title="agentos-runtime", lifespan=lifespan)


def get_agent(request: Request):
    agent = getattr(request.app.state, "agent", None)
    if agent is None:
        raise HTTPException(status_code=503, detail="agent not initialized")
    return agent


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


@app.get("/healthz")
async def healthz() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/runs")
async def runs(request: RunRequest, agent: Annotated[Any, Depends(get_agent)]) -> RunResponse:
    thread_id = request.thread_id or uuid.uuid4().hex
    try:
        result = await agent.ainvoke(
            {"messages": [{"role": "user", "content": request.input}]},
            config={"configurable": {"thread_id": thread_id}},
        )
    except Exception as exc:  # noqa: BLE001 - agent failures surface as 502
        raise HTTPException(status_code=502, detail=str(exc)) from exc
    messages = result.get("messages", [])
    return RunResponse(
        thread_id=thread_id,
        output=_extract_output(messages),
        steps=_extract_steps(messages),
    )
