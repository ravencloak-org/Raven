"""Shared pytest fixtures for the Raven AI Worker test suite."""

import pytest

from raven_worker.config import Settings


@pytest.fixture
def settings() -> Settings:
    """Return a default Settings instance for testing."""
    return Settings()
