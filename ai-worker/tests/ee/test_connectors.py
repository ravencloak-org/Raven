"""Tests for EE connector modules.

The enterprise connector modules (Airbyte, etc.) are planned future features.
The EE connector package currently exists as a placeholder. These tests verify:
1. The package structure is importable
2. The module docstring describes the intended connector features
3. Future connector implementations can be tested here via mocked HTTP

When Airbyte connector code is added to raven_worker.ee.connectors.airbyte,
update or uncomment the class-based tests below.
"""

from __future__ import annotations

from unittest.mock import AsyncMock

import pytest


class TestEEConnectorsPackage:
    """Verify the EE connectors package is importable and structured correctly."""

    def test_ee_connectors_importable(self):
        """raven_worker.ee.connectors must be importable without errors."""
        import raven_worker.ee.connectors  # noqa: F401

    def test_ee_connectors_has_docstring(self):
        """The connectors __init__ should describe the intended features."""
        import raven_worker.ee.connectors as connectors

        assert connectors.__doc__ is not None
        assert len(connectors.__doc__.strip()) > 0

    def test_ee_package_importable(self):
        """raven_worker.ee must be importable."""
        import raven_worker.ee  # noqa: F401

    def test_ee_has_connectors_subpackage(self):
        """raven_worker.ee.connectors must be a package (not just a module)."""
        import raven_worker.ee.connectors as connectors

        assert hasattr(connectors, "__path__")


class TestAirbyteConnectorPlaceholder:
    """Placeholder tests for Airbyte connector (EE feature, not yet implemented).

    These tests use mocked HTTP helpers. When raven_worker.ee.connectors.airbyte
    is implemented, replace these stubs with real import-based tests.
    """

    async def test_trigger_sync_returns_job_id(self):
        """A triggered Airbyte sync should return a job ID from the API."""
        # Simulate what a real trigger_sync would do
        mock_http_post = AsyncMock(return_value={"job_id": "job-123", "status": "running"})

        async def fake_trigger_sync(connection_id: str, org_id: str) -> dict:
            return await mock_http_post(
                f"https://api.airbyte.com/v1/connections/{connection_id}/sync",
                headers={"X-Org-Id": org_id},
            )

        result = await fake_trigger_sync("conn-abc", "org-1")
        assert result["job_id"] == "job-123"
        assert result["status"] == "running"

    async def test_poll_sync_succeeded_updates_kb(self, mock_db):
        """A succeeded sync job should trigger a DB update for the KB."""
        mock_http_get = AsyncMock(return_value={"status": "SUCCEEDED"})

        async def fake_poll(job_id: str, org_id: str, db) -> str:
            result = await mock_http_get(
                f"https://api.airbyte.com/v1/jobs/{job_id}",
            )
            if result["status"] == "SUCCEEDED":
                await db.execute("UPDATE knowledge_bases SET status='ready' WHERE id=$1", job_id)
            return result["status"]

        status = await fake_poll("job-123", "org-1", mock_db)
        assert status == "SUCCEEDED"
        mock_db.execute.assert_awaited_once()

    async def test_poll_sync_failed_raises(self):
        """A FAILED sync status should surface an error."""
        mock_http_get = AsyncMock(return_value={"status": "FAILED", "error": "Connection refused"})

        async def fake_poll(job_id: str) -> str:
            result = await mock_http_get(f"https://api.airbyte.com/v1/jobs/{job_id}")
            if result["status"] == "FAILED":
                raise RuntimeError(f"Sync FAILED: {result.get('error')}")
            return result["status"]

        with pytest.raises(RuntimeError, match="FAILED"):
            await fake_poll("job-fail")

    async def test_connection_id_forwarded_in_request(self):
        """The connection_id must be included in the HTTP request path."""
        calls = []

        async def fake_http_post(url: str, **kwargs) -> dict:
            calls.append(url)
            return {"job_id": "job-456", "status": "running"}

        async def fake_trigger(connection_id: str, org_id: str) -> dict:
            return await fake_http_post(
                f"https://api.airbyte.com/v1/connections/{connection_id}/sync",
                headers={"X-Org-Id": org_id},
            )

        await fake_trigger("conn-specific", "org-1")
        assert any("conn-specific" in url for url in calls)
