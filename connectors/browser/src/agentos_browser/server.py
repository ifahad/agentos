"""FastMCP server exposing the browser tools over streamable HTTP at /mcp.

The runtime adds ``http://<host>:8094/mcp`` to ``AGENTOS_MCP_SERVERS`` (a
``streamable_http`` transport URL) and the four tools below become agent tools.
Every tool returns a JSON string of a structured dict.
"""

from __future__ import annotations

import json
import logging

from mcp.server.fastmcp import FastMCP

from .browser import BrowserManager
from .config import LISTEN_HOST, LISTEN_PORT, MCP_PATH, Config

logger = logging.getLogger(__name__)

SERVER_NAME = "agentos-browser"


def build_server(config: Config, manager: BrowserManager | None = None) -> FastMCP:
    """Construct the FastMCP server with the four browser tools registered."""
    manager = manager or BrowserManager(config)

    mcp = FastMCP(
        SERVER_NAME,
        host=LISTEN_HOST,
        port=LISTEN_PORT,
        streamable_http_path=MCP_PATH,
        stateless_http=True,
    )

    @mcp.tool()
    async def navigate(url: str) -> str:
        """Navigate to an allowlisted URL. Returns {final_url, title, status}."""
        return json.dumps(await manager.navigate(url))

    @mcp.tool()
    async def get_text() -> str:
        """Return the visible text of the current page (capped at MAX_TEXT)."""
        return json.dumps(await manager.get_text())

    @mcp.tool()
    async def find_links(query: str = "") -> str:
        """List page links [{text, href}], optionally filtered by a substring."""
        return json.dumps(await manager.find_links(query))

    @mcp.tool()
    async def click(text: str) -> str:
        """Click the first visible element matching text; return {final_url, title}."""
        return json.dumps(await manager.click(text))

    return mcp


def run(config: Config | None = None) -> None:
    config = config or Config.from_env()
    logging.basicConfig(level=logging.INFO)
    if not config.allow_domains:
        logger.warning(
            "AGENTOS_BROWSER_ALLOW_DOMAINS is empty: all navigation is DENIED. "
            "Set it to a comma-separated hostname allowlist to enable browsing."
        )
    else:
        logger.info("browser allowlist: %s", ", ".join(config.allow_domains))
    mcp = build_server(config)
    logger.info("agentos-browser MCP on http://%s:%d%s", LISTEN_HOST, LISTEN_PORT, MCP_PATH)
    mcp.run(transport="streamable-http")
