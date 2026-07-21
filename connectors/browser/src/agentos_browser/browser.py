"""Shared headless-chromium browser manager backing the MCP tools.

A single hardened browser context is launched lazily and reused across tool
calls. Hardening: downloads disabled, and a route handler aborts every request
whose hostname is not on the allowlist (so a page can never reach out to a host
the operator did not permit, even for sub-resources).
"""

from __future__ import annotations

import asyncio
from typing import Any

from playwright.async_api import Browser, BrowserContext, Page, Playwright, async_playwright

from .allowlist import host_allowed
from .config import Config


class BrowserManager:
    """Owns one lazily-launched headless chromium context and the tool logic."""

    def __init__(self, config: Config) -> None:
        self.config = config
        self._playwright: Playwright | None = None
        self._browser: Browser | None = None
        self._context: BrowserContext | None = None
        self._page: Page | None = None
        self._lock = asyncio.Lock()

    async def _ensure_page(self) -> Page:
        """Launch the browser/context/page on first use (mutex-guarded)."""
        async with self._lock:
            if self._page is not None:
                return self._page
            self._playwright = await async_playwright().start()
            self._browser = await self._playwright.chromium.launch(headless=True)
            self._context = await self._browser.new_context(
                accept_downloads=False,
                viewport={"width": 1280, "height": 800},
            )
            self._context.set_default_timeout(self.config.timeout_ms)
            self._context.set_default_navigation_timeout(self.config.timeout_ms)
            await self._context.route("**/*", self._route)
            self._page = await self._context.new_page()
            return self._page

    async def _route(self, route: Any) -> None:
        """Abort any request to a host that is not on the allowlist."""
        if host_allowed(route.request.url, self.config.allow_domains):
            await route.continue_()
        else:
            await route.abort()

    async def close(self) -> None:
        if self._context is not None:
            await self._context.close()
            self._context = None
        if self._browser is not None:
            await self._browser.close()
            self._browser = None
        if self._playwright is not None:
            await self._playwright.stop()
            self._playwright = None
        self._page = None

    # ---- tool implementations -------------------------------------------

    async def navigate(self, url: str) -> dict[str, Any]:
        if not host_allowed(url, self.config.allow_domains):
            return {"error": f"navigation to host of {url!r} is not on the allowlist"}
        page = await self._ensure_page()
        response = await page.goto(url, wait_until="load", timeout=self.config.timeout_ms)
        return {
            "final_url": page.url,
            "title": await page.title(),
            "status": response.status if response is not None else None,
        }

    async def get_text(self) -> dict[str, Any]:
        page = await self._ensure_page()
        text = await page.inner_text("body")
        capped = text[: self.config.max_text]
        return {"text": capped, "truncated": len(text) > self.config.max_text}

    async def find_links(self, query: str = "") -> dict[str, Any]:
        page = await self._ensure_page()
        raw: list[dict[str, str]] = await page.eval_on_selector_all(
            "a[href]",
            """els => els.map(a => ({
                text: (a.textContent || '').trim(),
                href: a.href,
            }))""",
        )
        q = query.strip().lower()
        if q:
            raw = [
                link
                for link in raw
                if q in link["text"].lower() or q in link["href"].lower()
            ]
        return {"links": raw}

    async def click(self, text: str) -> dict[str, Any]:
        page = await self._ensure_page()
        target = None
        # Exact trimmed text first, then substring ("contains").
        for locator in (page.get_by_text(text, exact=True), page.get_by_text(text)):
            count = await locator.count()
            for i in range(count):
                candidate = locator.nth(i)
                if await candidate.is_visible():
                    target = candidate
                    break
            if target is not None:
                break
        if target is None:
            return {"error": f"no visible element matching text {text!r}"}
        await target.click(timeout=self.config.timeout_ms)
        try:
            await page.wait_for_load_state("load", timeout=self.config.timeout_ms)
        except Exception:  # noqa: BLE001 - click may not trigger navigation
            pass
        return {"final_url": page.url, "title": await page.title()}
