"""Basic tests for server startup and configuration."""

from raven_worker.config import Settings


class TestSettings:
    """Verify default configuration values."""

    def test_default_grpc_port(self, settings: Settings) -> None:
        assert settings.grpc_port == 50051

    def test_default_max_workers(self, settings: Settings) -> None:
        assert settings.grpc_max_workers == 10

    def test_default_database_url(self, settings: Settings) -> None:
        assert "postgresql" in settings.database_url

    def test_default_valkey_url(self, settings: Settings) -> None:
        assert settings.valkey_url.startswith("redis://")

    def test_default_log_level(self, settings: Settings) -> None:
        assert settings.log_level == "INFO"

    def test_otel_endpoint_defaults_to_none(self, settings: Settings) -> None:
        assert settings.otel_endpoint is None


class TestImports:
    """Verify that key modules can be imported without errors."""

    def test_import_server(self) -> None:
        from raven_worker import server  # noqa: F401

    def test_import_embedding_service(self) -> None:
        from raven_worker.services import embedding  # noqa: F401

    def test_import_rag_service(self) -> None:
        from raven_worker.services import rag  # noqa: F401

    def test_import_providers(self) -> None:
        from raven_worker.providers import base, openai_provider  # noqa: F401

    def test_import_processors(self) -> None:
        from raven_worker.processors import chunker, parser, scraper  # noqa: F401

    def test_version(self) -> None:
        from raven_worker import __version__

        assert __version__ == "0.1.0"
