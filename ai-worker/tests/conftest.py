"""Shared pytest fixtures for the Raven AI Worker test suite."""

from unittest.mock import AsyncMock, patch

import pytest

from raven_worker.config import Settings


@pytest.fixture
def settings() -> Settings:
    """Return a default Settings instance for testing."""
    return Settings()


@pytest.fixture
def grpc_context():
    """Return a mock gRPC context for servicer tests."""
    ctx = AsyncMock()
    ctx.abort = AsyncMock()
    return ctx


@pytest.fixture(autouse=True)
def _disable_rag_cache():
    """Disable the RAG response cache in all tests by default.

    Tests that specifically exercise cache behaviour should override
    ``_check_cache`` / ``_store_cache`` in their own patches.
    """
    with (
        patch(
            "raven_worker.services.rag.RAGServicer._check_cache",
            new_callable=AsyncMock,
            return_value=None,
        ),
        patch(
            "raven_worker.services.rag.RAGServicer._store_cache",
            new_callable=AsyncMock,
        ),
    ):
        yield
