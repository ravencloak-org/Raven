"""Application settings loaded from environment variables."""

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Raven AI Worker configuration.

    All settings can be overridden via environment variables
    prefixed with ``RAVEN_``. For example, ``RAVEN_GRPC_PORT=50052``.
    """

    grpc_port: int = 50051
    grpc_max_workers: int = 10
    database_url: str = "postgresql://raven:raven@localhost:5432/raven"
    valkey_url: str = "redis://localhost:6379/0"
    otel_endpoint: str | None = None
    otel_enabled: bool = False
    log_level: str = "INFO"
    liteparse_path: str = "liteparse"

    model_config = SettingsConfigDict(env_prefix="RAVEN_")


settings = Settings()
