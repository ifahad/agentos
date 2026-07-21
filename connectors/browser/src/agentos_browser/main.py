"""Entrypoint: run the browser connector MCP server on :8094."""

from __future__ import annotations

from .server import run


def main() -> None:
    run()


if __name__ == "__main__":
    main()
