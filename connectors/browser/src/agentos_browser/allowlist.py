"""Domain allowlist helpers.

The allowlist is the connector's whole safety model: every navigation and every
sub-resource request is checked against it. ``host_allowed`` is a pure function
so it can be unit-tested exhaustively without a browser.
"""

from __future__ import annotations

from urllib.parse import urlsplit


def parse_allowlist(raw: str | None) -> list[str]:
    """Parse a comma-separated hostname allowlist into normalized entries.

    Empty/blank entries are dropped; hosts are lowercased and stripped. An empty
    or unset string yields an empty list (which denies all navigation).
    """
    if not raw:
        return []
    return [h.strip().lower() for h in raw.split(",") if h.strip()]


def host_allowed(url: str, allowlist: list[str]) -> bool:
    """Return True iff ``url``'s hostname is permitted by ``allowlist``.

    A host matches an allowlist entry on an exact match or as a subdomain of it
    (``a.example.com`` matches ``example.com``). An empty allowlist denies
    everything. Invalid URLs, or URLs without a hostname, return False.
    """
    if not allowlist:
        return False
    try:
        host = urlsplit(url).hostname
    except ValueError:
        return False
    if not host:
        return False
    host = host.lower()
    for allowed in allowlist:
        if not allowed:
            continue
        if host == allowed or host.endswith("." + allowed):
            return True
    return False
