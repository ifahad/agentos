FROM python:3.12-slim

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

ENV PATH="/app/.venv/bin:$PATH"

# Drop to an unprivileged user. This service is where model-chosen tool calls
# actually execute, so it is the last container that should hold root: an escape
# from the agent loop should land somewhere with nothing worth having. The uid
# is fixed rather than assigned so Kubernetes runAsUser can match it.
#
# Ownership is handed over after the venv is built, since uv writes it as root.
RUN useradd --system --uid 10002 --user-group --home-dir /app --shell /usr/sbin/nologin agentos \
    && chown -R agentos:agentos /app
USER 10002:10002

EXPOSE 8000

CMD ["python", "-m", "agentos_runtime.main"]
