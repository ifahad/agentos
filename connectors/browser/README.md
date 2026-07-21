# agentos-browser connector

MCP server (`agentos-browser`, StreamableHTTP `/mcp` on `:8094`) that exposes a
headless-chromium browser (Playwright, async) as agent tools. One shared,
hardened browser context is launched lazily and reused across calls.

This connector is **opt-in**: it is not part of the default `compose up` path.
Enable it only when an agent genuinely needs browser/computer-use, then add
`http://<host>:8094/mcp` to the runtime's `AGENTOS_MCP_SERVERS`
(comma-separated `streamable_http` MCP URLs).

## Environment

| Variable | Default | Meaning |
| --- | --- | --- |
| `AGENTOS_BROWSER_ALLOW_DOMAINS` | — (deny all) | Comma-separated hostname allowlist, e.g. `example.com,docs.python.org`. **Empty = every navigation denied** and a startup WARNING is logged. |
| `AGENTOS_BROWSER_TIMEOUT_S` | `20` | Navigation / action / load timeout in seconds. |
| `AGENTOS_BROWSER_MAX_TEXT` | `8000` | `get_text` output cap (characters); overflow is truncated and flagged. |

The MCP server binds `0.0.0.0:8094` at path `/mcp` (streamable HTTP).

## Tools

All tools enforce the domain allowlist and return a JSON string of a structured
dict.

- `navigate(url)` → `{"final_url", "title", "status"}`. A non-allowlisted host
  is **never** navigated to: it returns `{"error": ...}` instead.
- `get_text()` → `{"text", "truncated"}` — visible text of the current page,
  capped at `AGENTOS_BROWSER_MAX_TEXT`.
- `find_links(query="")` → `{"links": [{"text", "href"}]}` — all page anchors,
  optionally filtered by a case-insensitive substring on the link text or href.
- `click(text)` → `{"final_url", "title"}` — clicks the first **visible**
  element whose trimmed text matches (exact match first, then substring); if no
  visible element matches, returns `{"error": ...}`.

## Allowlist safety model

The allowlist is the connector's whole safety boundary and it is enforced in two
places:

1. **Navigation gate** — `navigate` rejects any URL whose hostname is not on the
   allowlist before the browser touches the network.
2. **Request gate** — a Playwright route handler intercepts *every* request the
   page makes (including sub-resources, redirects, and script-initiated fetches)
   and **aborts** any whose hostname is not on the allowlist. A page can never
   reach out to a host the operator did not permit.

Matching is exact-host or subdomain (`a.example.com` matches `example.com`);
invalid or hostless URLs are denied. An **empty allowlist denies everything** —
the connector runs but no page will ever load, which is the safe default.

Further hardening: downloads are disabled (`accept_downloads=False`), a fixed
reasonable viewport is used, and only headless chromium is launched. This is a
navigation/read tool, not a general automation sandbox — keep the allowlist
narrow.

## Development

```sh
uv sync
uv run playwright install chromium   # once, to enable the integration test
uv run ruff check .
uv run pytest
```

The `host_allowed` unit tests always run. The Playwright integration test serves
a localhost HTTP fixture (no real internet) and is `skipif`-ed when chromium is
not installed.

## Docker

The image is based on the official Playwright Python image
(`mcr.microsoft.com/playwright/python:v1.61.0-noble`), so chromium and all OS
dependencies are preinstalled and matched to the pinned `playwright==1.61.0`.

```sh
docker build -t agentos-browser .
docker run --rm -p 8094:8094 \
  -e AGENTOS_BROWSER_ALLOW_DOMAINS=example.com \
  agentos-browser
```
