"""Agent-loop tests using a fake tool-calling model — no network, no real LLM."""

from typing import Any

from langchain_core.language_models import BaseChatModel
from langchain_core.messages import AIMessage, ToolMessage
from langchain_core.outputs import ChatGeneration, ChatResult
from langchain_core.tools import tool
from langgraph.checkpoint.memory import InMemorySaver

from agentos_runtime.agent import SYSTEM_PROMPT, build_agent, get_checkpointer, load_mcp_tools
from agentos_runtime.config import Settings

QUERY_RESULT = '{"columns": ["n"], "rows": [[1]], "row_count": 1, "truncated": false}'


@tool
def query(sql: str) -> str:
    """Run a read-only SQL query against the legacy database."""
    return QUERY_RESULT


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


def make_fake_model() -> FakeToolCallingModel:
    return FakeToolCallingModel(
        responses=[
            AIMessage(
                content="",
                tool_calls=[
                    {
                        "name": "query",
                        "args": {"sql": "SELECT 1"},
                        "id": "call_1",
                        "type": "tool_call",
                    }
                ],
            ),
            AIMessage(content="answer"),
        ]
    )


async def test_build_agent_executes_tool_loop():
    agent = build_agent(
        make_settings(), [query], InMemorySaver(), model=make_fake_model()
    )
    result = await agent.ainvoke(
        {"messages": [{"role": "user", "content": "how many rows?"}]},
        config={"configurable": {"thread_id": "t-1"}},
    )
    messages = result["messages"]
    final = messages[-1]
    assert isinstance(final, AIMessage)
    assert final.content == "answer"
    tool_messages = [m for m in messages if isinstance(m, ToolMessage)]
    assert len(tool_messages) == 1
    assert QUERY_RESULT in tool_messages[0].content
    tool_calls = [tc for m in messages if isinstance(m, AIMessage) for tc in m.tool_calls]
    assert [(tc["name"], tc["args"]) for tc in tool_calls] == [("query", {"sql": "SELECT 1"})]


async def test_checkpointer_persists_thread_state():
    checkpointer = InMemorySaver()
    agent = build_agent(make_settings(), [query], checkpointer, model=make_fake_model())
    config = {"configurable": {"thread_id": "t-persist"}}
    await agent.ainvoke({"messages": [{"role": "user", "content": "hi"}]}, config=config)
    state = await agent.aget_state(config)
    assert len(state.values["messages"]) >= 3  # human + tool-call AI + tool + final AI


async def test_load_mcp_tools_empty_config_returns_no_tools():
    assert await load_mcp_tools(make_settings(mcp_servers="")) == []


async def test_get_checkpointer_defaults_to_in_memory():
    async with get_checkpointer(make_settings()) as checkpointer:
        assert isinstance(checkpointer, InMemorySaver)


def test_settings_defaults_and_mcp_parsing():
    settings = make_settings(
        mcp_servers="http://sql-connector:8090/mcp, http://other:8091/mcp"
    )
    assert settings.model == "anthropic/claude-sonnet-5"
    assert settings.checkpoint_database_url is None
    assert settings.mcp_server_urls == [
        "http://sql-connector:8090/mcp",
        "http://other:8091/mcp",
    ]
    assert make_settings().mcp_server_urls == []


def test_system_prompt_contract():
    text = SYSTEM_PROMPT.lower()
    for expectation in ("data analyst", "schema", "read", "fabricate", "cite"):
        assert expectation in text
