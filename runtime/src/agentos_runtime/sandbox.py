"""run_python tool backed by the Rust sandbox service (AGENTOS_SANDBOX_URL)."""

import httpx
from langchain_core.tools import BaseTool, tool

from agentos_runtime.config import Settings

SANDBOX_TIMEOUT_S = 10


def make_run_python_tool(sandbox_url: str) -> BaseTool:
    """Build the ``run_python`` tool bound to ``sandbox_url``."""
    execute_url = sandbox_url.rstrip("/") + "/execute"

    @tool
    async def run_python(code: str) -> str:
        """Execute Python code in an isolated sandbox; returns the exit code,
        stdout, and stderr of the run."""
        async with httpx.AsyncClient() as client:
            response = await client.post(
                execute_url,
                json={"language": "python", "code": code, "timeout_s": SANDBOX_TIMEOUT_S},
                timeout=SANDBOX_TIMEOUT_S + 5,
            )
        response.raise_for_status()
        result = response.json()
        return (
            f"exit {result['exit_code']}\n"
            f"stdout: {result['stdout']}\n"
            f"stderr: {result['stderr']}"
        )

    return run_python


def sandbox_tools(settings: Settings) -> list[BaseTool]:
    """[run_python] when AGENTOS_SANDBOX_URL is set, else no tools."""
    if not settings.sandbox_url:
        return []
    return [make_run_python_tool(settings.sandbox_url)]
