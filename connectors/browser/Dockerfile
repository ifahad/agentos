# Build stage: resolve the virtualenv as root, where uv wants to be.
# Official Playwright image: chromium + all OS deps preinstalled, matching the
# pinned playwright==1.61.0 in pyproject.toml.
FROM mcr.microsoft.com/playwright/python:v1.61.0-noble AS build

COPY --from=ghcr.io/astral-sh/uv:0.11.8 /uv /uvx /bin/

ENV UV_COMPILE_BYTECODE=1 UV_LINK_MODE=copy

WORKDIR /app

COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-install-project --no-dev

COPY src ./src
COPY README.md ./
RUN uv sync --frozen --no-dev

# Runtime stage on the same base, so chromium and its OS deps are still present.
# Two stages rather than one so uv does not ship in the final image: this
# container drives a real browser over attacker-influenced pages, which is the
# last place to leave a package manager lying around.
FROM mcr.microsoft.com/playwright/python:v1.61.0-noble

WORKDIR /app

COPY --from=build --chown=1001:1001 /app /app

ENV PATH="/app/.venv/bin:$PATH"

# The base image already bundles the chromium build for this Playwright version,
# but re-declare the browsers path so our venv resolves it.
ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright

# Drop to the base image's own non-root user. Every other AgentOS service runs
# unprivileged; without this line the connector that renders untrusted web
# content was the sole exception, running as root.
#
# pwuser is uid/gid 1001 (verified in the base image, NOT the 1000 the name
# suggests) and /ms-playwright is world-readable, so chromium still resolves.
# Kubernetes needs the numeric id: deploy/helm/.../browser-connector.yaml sets
# runAsUser: 1001 to match, and runAsNonRoot cannot resolve a USER name on its
# own.
USER 1001:1001

EXPOSE 8094

CMD ["python", "-m", "agentos_browser.main"]
