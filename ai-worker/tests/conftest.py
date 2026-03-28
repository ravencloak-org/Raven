"""Shared pytest fixtures for the Raven AI Worker test suite."""

from unittest.mock import AsyncMock

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
