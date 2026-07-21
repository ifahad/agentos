# Official Playwright image: chromium + all OS deps preinstalled, matching the
# pinned playwright==1.61.0 in pyproject.toml.
FROM mcr.microsoft.com/playwright/python:v1.61.0-noble

COPY --from=ghcr.io/astral-sh/uv:0.11.8 /uv /uvx /bin/

ENV UV_COMPILE_BYTECODE=1 UV_LINK_MODE=copy

WORKDIR /app

COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-install-project --no-dev

COPY src ./src
COPY README.md ./
RUN uv sync --frozen --no-dev

ENV PATH="/app/.venv/bin:$PATH"

# The base image already bundles the chromium build for this Playwright version,
# but re-declare the browsers path so our venv resolves it.
ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright

EXPOSE 8094

CMD ["python", "-m", "agentos_browser.main"]
