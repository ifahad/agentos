"""Runtime configuration loaded from AGENTOS_-prefixed environment variables."""

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Service settings.

    Environment variables:
        AGENTOS_GATEWAY_URL: Base URL of the LLM gateway (e.g. http://gateway:8080).
        AGENTOS_GATEWAY_KEY: Gateway API key (``agos-...``).
        AGENTOS_MODEL: Provider-prefixed model name.
        AGENTOS_MCP_SERVERS: Comma-separated streamable-http MCP URLs (may be empty).
        AGENTOS_CHECKPOINT_DATABASE_URL: Postgres URL for checkpoints (empty -> in-memory).
    """

    model_config = SettingsConfigDict(env_prefix="AGENTOS_", extra="ignore")

    gateway_url: str
    gateway_key: str
    model: str = "anthropic/claude-sonnet-5"
    mcp_servers: str = ""
    checkpoint_database_url: str | None = None

    @property
    def mcp_server_urls(self) -> list[str]:
        """AGENTOS_MCP_SERVERS parsed into a list of non-empty URLs."""
        return [url.strip() for url in self.mcp_servers.split(",") if url.strip()]
