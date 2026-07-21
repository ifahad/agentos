"""Shared test fakes: tool-calling model, settings factory, query tool."""

from typing import Any

from langchain_core.language_models import BaseChatModel
from langchain_core.messages import AIMessage
from langchain_core.outputs import ChatGeneration, ChatResult
from langchain_core.tools import tool

from agentos_runtime.config import Settings

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
