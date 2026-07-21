"""run_python sandbox tool: registration gate + request/result formatting."""

from helpers import make_settings

from agentos_runtime import sandbox
from agentos_runtime.config import Settings
from agentos_runtime.sandbox import make_run_python_tool, sandbox_tools


class FakeResponse:
    def __init__(self, payload):
        self._payload = payload

    def raise_for_status(self):
        return None

    def json(self):
        return self._payload


class FakeAsyncClient:
    """Records POSTs and returns a canned sandbox /execute payload."""

    requests: list[tuple[str, dict, float]] = []
    payload: dict = {}

    def __init__(self, *args, **kwargs):
        pass

    async def __aenter__(self):
        return self

    async def __aexit__(self, *exc):
        return False

    async def post(self, url, json=None, timeout=None):
        FakeAsyncClient.requests.append((url, json, timeout))
        return FakeResponse(FakeAsyncClient.payload)


def use_fake_client(monkeypatch, payload):
    FakeAsyncClient.requests = []
    FakeAsyncClient.payload = payload
    monkeypatch.setattr(sandbox.httpx, "AsyncClient", FakeAsyncClient)


def test_run_python_registered_iff_sandbox_url_set(monkeypatch):
    assert sandbox_tools(make_settings()) == []
    tools = sandbox_tools(make_settings(sandbox_url="http://sandbox:8070"))
    assert [tool.name for tool in tools] == ["run_python"]
    # the gate reads the AGENTOS_SANDBOX_URL env var
    monkeypatch.setenv("AGENTOS_SANDBOX_URL", "http://sandbox:8070")
    settings = Settings(_env_file=None, gateway_url="http://gw", gateway_key="agos-x")
    assert settings.sandbox_url == "http://sandbox:8070"
    assert [tool.name for tool in sandbox_tools(settings)] == ["run_python"]
    monkeypatch.delenv("AGENTOS_SANDBOX_URL")
    unset = Settings(_env_file=None, gateway_url="http://gw", gateway_key="agos-x")
    assert sandbox_tools(unset) == []


async def test_run_python_posts_execute_and_formats_result(monkeypatch):
    use_fake_client(monkeypatch, {"exit_code": 0, "stdout": "hello\n", "stderr": ""})
    tool = make_run_python_tool("http://sandbox:8070/")
    result = await tool.ainvoke({"code": "print('hello')"})
    assert result == "exit 0\nstdout: hello\n\nstderr: "
    url, body, timeout = FakeAsyncClient.requests[0]
    assert url == "http://sandbox:8070/execute"
    assert body == {"language": "python", "code": "print('hello')", "timeout_s": 10}
    assert timeout == 15


async def test_run_python_reports_nonzero_exit_and_stderr(monkeypatch):
    use_fake_client(
        monkeypatch, {"exit_code": 1, "stdout": "", "stderr": "Traceback: boom"}
    )
    tool = make_run_python_tool("http://sandbox:8070")
    result = await tool.ainvoke({"code": "raise SystemExit(1)"})
    assert result == "exit 1\nstdout: \nstderr: Traceback: boom"
