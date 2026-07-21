"""Pure, always-run tests for the domain allowlist (no browser required)."""

from __future__ import annotations

import pytest

from agentos_browser.allowlist import host_allowed, parse_allowlist

ALLOW = ["example.com", "localhost", "127.0.0.1"]


@pytest.mark.parametrize(
    ("url", "allowlist", "expected"),
    [
        # exact host match
        ("https://example.com/path", ALLOW, True),
        ("http://localhost:8000/x", ALLOW, True),
        ("http://127.0.0.1:9000/", ALLOW, True),
        # subdomain match
        ("https://www.example.com", ALLOW, True),
        ("https://a.b.example.com/deep", ALLOW, True),
        # denied host (not on list, and not a subdomain)
        ("https://evil.com", ALLOW, False),
        ("https://notexample.com", ALLOW, False),  # suffix but not a subdomain
        ("https://example.com.evil.com", ALLOW, False),
        # empty allowlist denies everything
        ("https://example.com", [], False),
        # invalid / hostless URLs
        ("not a url", ALLOW, False),
        ("", ALLOW, False),
        ("file:///etc/passwd", ALLOW, False),
        ("mailto:foo@example.com", ALLOW, False),
    ],
)
def test_host_allowed(url: str, allowlist: list[str], expected: bool) -> None:
    assert host_allowed(url, allowlist) is expected


def test_parse_allowlist() -> None:
    assert parse_allowlist("a.com, B.com ,,c.com") == ["a.com", "b.com", "c.com"]
    assert parse_allowlist("") == []
    assert parse_allowlist(None) == []
