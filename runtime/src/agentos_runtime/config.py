"""Runtime configuration loaded from AGENTOS_-prefixed environment variables."""

from typing import Literal

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Service settings.

    Environment variables:
        AGENTOS_RUNTIME_AUTH_TOKEN: Shared bearer token every caller must present
            on every route except ``GET /healthz`` and the operator-webhook prefix
            ``/operators/webhooks/{token}`` (there the ``whk-`` path segment is the
            credential). Unset/empty -> the app refuses to start (fail-closed).
        AGENTOS_GATEWAY_URL: Base URL of the LLM gateway (e.g. http://gateway:8080).
        AGENTOS_GATEWAY_KEY: Gateway API key (``agos-...``).
        AGENTOS_MODEL: Provider-prefixed model name.
        AGENTOS_MCP_SERVERS: Comma-separated streamable-http MCP URLs (may be empty).
        AGENTOS_CHECKPOINT_DATABASE_URL: Postgres URL for checkpoints (empty -> in-memory).
        AGENTOS_APPROVAL_TOOLS: Comma-separated tool names needing human approval
            (empty -> HITL off).
        AGENTOS_EMBED_MODEL: Provider-prefixed embedding model routed via the gateway.
        AGENTOS_CONTEXT_ENGINE: ``on``/``off``; empty -> on iff checkpoint DB set.
        AGENTOS_AGENT_PROFILE: ``react`` (default) or ``deep`` (deepagents package).
        AGENTOS_OTEL_ENDPOINT: OTLP/HTTP base URL; empty -> tracing disabled.
        AGENTOS_SANDBOX_URL: Sandbox service base URL; empty -> run_python tool off.
        AGENTOS_JUDGE_MODEL: Provider-prefixed model used to judge eval cases
            that carry a ``judge`` block (routed via the gateway).
        AGENTOS_MAX_CONTEXT_TOKENS: Cap on the history sent to the model on each
            turn; older messages are dropped first (0 -> no trimming). Applies
            to the ``react`` profile: the ``deep`` profile bounds its own
            context via deepagents' built-in summarization middleware.
    """

    model_config = SettingsConfigDict(env_prefix="AGENTOS_", extra="ignore")

    runtime_auth_token: str = ""
    gateway_url: str
    gateway_key: str
    model: str = "anthropic/claude-sonnet-5"
    mcp_servers: str = ""
    checkpoint_database_url: str | None = None
    approval_tools: str = ""
    embed_model: str = "ollama/bge-m3"
    context_engine: Literal["on", "off", ""] = ""
    agent_profile: Literal["react", "deep"] = "react"
    otel_endpoint: str | None = None
    sandbox_url: str = ""
    judge_model: str = "anthropic/claude-haiku-4-5"
    # Every turn resends the whole thread, so a long-running react agent
    # eventually exceeds its model's window and fails mid-run. Trimming keeps
    # the newest messages plus the system prompt. Default 0 (off) preserves
    # existing behaviour exactly; autonomous multi-cycle runs should set it.
    # The deep profile ignores this: deepagents summarises on its own.
    max_context_tokens: int = 0
    # Multiverse council. council_config points at council.yaml (empty disables
    # the council entirely); the heartbeat runs autonomous objectives only when
    # its interval is > 0 (off by default — a conservative, opt-in stance);
    # council_max_spend_usd is the default per-objective spend ceiling.
    council_config: str = ""
    council_heartbeat_s: int = 0
    council_max_spend_usd: float = 5.0
    # Operators: governed always-on autonomy for the single agent. The scheduler
    # runs only when autonomy_enabled is true (opt-in — the runtime serves the
    # operators API but never fires on its own by default). tick_s is how often
    # the scheduler checks for due operators; max_cycles is an operator's default
    # tool-iteration cap; skills_dir is the in-repo SKILL.md directory.
    autonomy_enabled: bool = False
    autonomy_tick_s: int = 15
    autonomy_max_cycles: int = 8
    skills_dir: str = ""

    def require_runtime_auth_token(self) -> str:
        """Return the configured runtime auth token or fail closed.

        Called at startup so the service refuses to boot when
        AGENTOS_RUNTIME_AUTH_TOKEN is unset/empty, rather than serving an
        unauthenticated API (finding C1).
        """
        token = self.runtime_auth_token.strip()
        if not token:
            raise RuntimeError("AGENTOS_RUNTIME_AUTH_TOKEN must be set")
        return token

    @property
    def mcp_server_urls(self) -> list[str]:
        """AGENTOS_MCP_SERVERS parsed into a list of non-empty URLs."""
        return [url.strip() for url in self.mcp_servers.split(",") if url.strip()]

    @property
    def approval_tool_names(self) -> list[str]:
        """AGENTOS_APPROVAL_TOOLS parsed into a list of non-empty tool names."""
        return [name.strip() for name in self.approval_tools.split(",") if name.strip()]

    @property
    def context_engine_enabled(self) -> bool:
        """Whether the context engine is on (default: on iff checkpoint DB is set)."""
        if self.context_engine == "on":
            return True
        if self.context_engine == "off":
            return False
        return bool(self.checkpoint_database_url)
