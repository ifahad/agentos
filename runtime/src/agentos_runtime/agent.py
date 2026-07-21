"""Agent construction: model wiring, MCP tool loading, and checkpointing."""

from collections.abc import AsyncIterator, Sequence
from contextlib import asynccontextmanager

from langchain_core.language_models import BaseChatModel
from langchain_core.tools import BaseTool
from langchain_mcp_adapters.client import MultiServerMCPClient
from langchain_openai import ChatOpenAI
from langgraph.checkpoint.base import BaseCheckpointSaver
from langgraph.checkpoint.memory import InMemorySaver
from langgraph.checkpoint.postgres.aio import AsyncPostgresSaver
from langgraph.prebuilt import create_react_agent

from agentos_runtime.config import Settings

SYSTEM_PROMPT = (
    "You are a careful data analyst operating over legacy enterprise systems "
    "via approved tools. Always inspect the available tables and their schema "
    "before writing queries. Only read data; never attempt to modify it. "
    "Never fabricate values: every figure in your answer must come from tool "
    "results. When you answer, cite which tables the answer came from."
)


def build_agent(
    settings: Settings,
    tools: Sequence[BaseTool],
    checkpointer: BaseCheckpointSaver,
    model: BaseChatModel | None = None,
):
    """Build the agent graph for the configured profile.

    ``model`` overrides the gateway-backed ChatOpenAI (used by tests to inject
    a fake tool-calling model). When AGENTOS_APPROVAL_TOOLS is non-empty the
    react graph is compiled with ``interrupt_before=["tools"]`` so every tool
    batch pauses for the HITL run loop (non-approval tools auto-resume there).
    """
    if model is None:
        model = ChatOpenAI(
            base_url=settings.gateway_url.rstrip("/") + "/v1",
            api_key=settings.gateway_key,
            model=settings.model,
        )
    if settings.agent_profile == "deep":
        # deepagents compiles its own graph and exposes no interrupt_before
        # pass-through; HITL tool approvals therefore apply to the react
        # profile only (documented deviation).
        from deepagents import create_deep_agent

        return create_deep_agent(
            model=model,
            tools=list(tools),
            system_prompt=SYSTEM_PROMPT,
            checkpointer=checkpointer,
        )
    interrupt_before = ["tools"] if settings.approval_tool_names else None
    return create_react_agent(
        model,
        tools=list(tools),
        prompt=SYSTEM_PROMPT,
        checkpointer=checkpointer,
        interrupt_before=interrupt_before,
    )


async def load_mcp_tools(settings: Settings) -> list[BaseTool]:
    """Load tools from every MCP server in AGENTOS_MCP_SERVERS.

    Servers are named ``sql0``, ``sql1``, ... in listed order. Returns an
    empty list when no servers are configured.
    """
    urls = settings.mcp_server_urls
    if not urls:
        return []
    connections = {
        f"sql{i}": {"transport": "streamable_http", "url": url}
        for i, url in enumerate(urls)
    }
    client = MultiServerMCPClient(connections)
    return await client.get_tools()


@asynccontextmanager
async def get_checkpointer(settings: Settings) -> AsyncIterator[BaseCheckpointSaver]:
    """Yield the configured checkpointer.

    AsyncPostgresSaver (with ``.setup()`` run once) when
    AGENTOS_CHECKPOINT_DATABASE_URL is set, else an InMemorySaver. Exposed as
    an async context manager because the Postgres saver owns a connection
    whose lifetime must match the application's.
    """
    if settings.checkpoint_database_url:
        async with AsyncPostgresSaver.from_conn_string(
            settings.checkpoint_database_url
        ) as saver:
            await saver.setup()
            yield saver
    else:
        yield InMemorySaver()
