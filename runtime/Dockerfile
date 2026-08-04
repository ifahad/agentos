# Build stage: resolve and compile the virtualenv as root, where uv wants to be.
FROM python:3.12-slim AS build

COPY --from=ghcr.io/astral-sh/uv:0.11.8 /uv /uvx /bin/

ENV UV_COMPILE_BYTECODE=1 UV_LINK_MODE=copy

WORKDIR /app

COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-install-project --no-dev

COPY src ./src
COPY evals ./evals
COPY council.yaml ./council.yaml
COPY skills ./skills
COPY README.md ./
RUN uv sync --frozen --no-dev

# Runtime stage. Two things this buys over building in place:
#
#   1. Ownership is set by COPY --chown rather than a `chown -R /app` after the
#      fact. A recursive chown rewrites every file's metadata, so Docker stores
#      a second copy of the whole venv in that layer — roughly 400 MB of image
#      for a permissions change.
#   2. uv and uvx stay in the build stage. Nothing at runtime invokes them, and
#      this service is where model-chosen tool calls execute, so a package
#      manager is exactly the kind of tool not to leave lying around in it.
FROM python:3.12-slim

WORKDIR /app

# The uid is fixed rather than assigned so Kubernetes runAsUser can match it.
RUN useradd --system --uid 10002 --user-group --home-dir /app --shell /usr/sbin/nologin agentos

# Drop to an unprivileged user. This service is where model-chosen tool calls
# actually execute, so it is the last container that should hold root: an escape
# from the agent loop should land somewhere with nothing worth having.
COPY --from=build --chown=10002:10002 /app /app

ENV PATH="/app/.venv/bin:$PATH"

USER 10002:10002

EXPOSE 8000

CMD ["python", "-m", "agentos_runtime.main"]
