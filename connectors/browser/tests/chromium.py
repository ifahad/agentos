"""Helper to detect whether a launchable chromium is installed for Playwright."""

from __future__ import annotations

import os


def chromium_installed() -> bool:
    """Return True iff Playwright's chromium executable is present on disk.

    Used to ``skipif`` the integration test when the browser is absent (e.g. in
    a minimal environment where ``playwright install chromium`` was not run).
    """
    try:
        from playwright.sync_api import sync_playwright
    except Exception:  # noqa: BLE001 - playwright missing entirely
        return False
    try:
        with sync_playwright() as p:
            path = p.chromium.executable_path
    except Exception:  # noqa: BLE001 - no browser registered
        return False
    return bool(path) and os.path.exists(path)
