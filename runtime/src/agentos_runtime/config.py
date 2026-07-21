"""Runtime configuration loaded from AGENTOS_-prefixed environment variables."""

from typing import Literal

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Service settings.

    Environment variables:
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
    """

    model_config = SettingsConfigDict(env_prefix="AGENTOS_", extra="ignore")

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
