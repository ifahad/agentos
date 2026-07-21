"""Uvicorn entrypoint: ``python -m agentos_runtime.main``."""

import uvicorn


def main() -> None:
    uvicorn.run("agentos_runtime.api:app", host="0.0.0.0", port=8000)  # noqa: S104


if __name__ == "__main__":
    main()
