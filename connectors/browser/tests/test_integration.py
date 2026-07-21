"""Integration test exercising navigate/get_text/find_links/click against a
tiny in-process localhost HTTP fixture. Skipped when chromium is not installed.

No real internet is used: the fixture is served on 127.0.0.1 and the test
allowlist only permits localhost/127.0.0.1.
"""

from __future__ import annotations

import threading
from collections.abc import Iterator
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import pytest

from agentos_browser.browser import BrowserManager
from agentos_browser.config import Config

from .chromium import chromium_installed

pytestmark = pytest.mark.skipif(
    not chromium_installed(), reason="Playwright chromium is not installed"
)

INDEX = b"""<!doctype html><html><head><title>Fixture Home</title></head>
<body>
  <h1>Welcome Heading</h1>
  <p>Some visible paragraph text for extraction.</p>
  <a href="/page2">Go to page two</a>
  <a href="https://example.com/docs">External Docs</a>
</body></html>"""

PAGE2 = b"""<!doctype html><html><head><title>Page Two</title></head>
<body><h1>Second Page</h1><p>Arrived on page two.</p></body></html>"""


class _Handler(BaseHTTPRequestHandler):
    def do_GET(self) -> None:  # noqa: N802 - stdlib API name
        body = PAGE2 if self.path.startswith("/page2") else INDEX
        self.send_response(200)
        self.send_header("Content-Type", "text/html")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args: object) -> None:  # silence test output
        pass


@pytest.fixture
def fixture_server() -> Iterator[str]:
    server = ThreadingHTTPServer(("127.0.0.1", 0), _Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        host, port = server.server_address
        yield f"http://{host}:{port}"
    finally:
        server.shutdown()
        server.server_close()


@pytest.fixture
async def manager() -> Iterator[BrowserManager]:
    config = Config(allow_domains=["localhost", "127.0.0.1"], timeout_s=15)
    mgr = BrowserManager(config)
    try:
        yield mgr
    finally:
        await mgr.close()


async def test_navigate_get_text_find_links_click(
    fixture_server: str, manager: BrowserManager
) -> None:
    # navigate
    nav = await manager.navigate(f"{fixture_server}/")
    assert "error" not in nav
    assert nav["status"] == 200
    assert nav["title"] == "Fixture Home"
    assert nav["final_url"].rstrip("/") == fixture_server

    # get_text
    text = await manager.get_text()
    assert "Welcome Heading" in text["text"]
    assert "visible paragraph text" in text["text"]

    # find_links (unfiltered)
    links = (await manager.find_links())["links"]
    hrefs = {link["href"] for link in links}
    assert any(h.endswith("/page2") for h in hrefs)
    assert "https://example.com/docs" in hrefs

    # find_links filtered by substring on text
    filtered = (await manager.find_links("page two"))["links"]
    assert len(filtered) == 1
    assert filtered[0]["text"] == "Go to page two"

    # click navigates to page two (external request to example.com would be
    # blocked by the route handler, but we click the internal link)
    clicked = await manager.click("Go to page two")
    assert "error" not in clicked
    assert clicked["title"] == "Page Two"
    assert clicked["final_url"].endswith("/page2")


async def test_navigate_rejects_non_allowlisted_host(manager: BrowserManager) -> None:
    result = await manager.navigate("https://evil.example.org/")
    assert "error" in result


async def test_click_no_match_returns_error(
    fixture_server: str, manager: BrowserManager
) -> None:
    await manager.navigate(f"{fixture_server}/")
    result = await manager.click("nonexistent link text")
    assert "error" in result
