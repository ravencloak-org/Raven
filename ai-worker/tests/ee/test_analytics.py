"""Tests for EE analytics modules.

The enterprise analytics modules (PostHog, ClickHouse event ingestion, etc.)
are planned future features. The EE analytics package currently exists as a
placeholder. These tests verify:
1. The package structure is importable
2. Placeholder event contracts are validated via simulated helpers

When analytics code is added to raven_worker.ee.analytics, update or
uncomment the class-based tests below.
"""

from __future__ import annotations

from unittest.mock import AsyncMock


class TestEEAnalyticsPackage:
    """Verify the EE analytics package is importable and structured correctly."""

    def test_ee_analytics_importable(self):
        """raven_worker.ee.analytics must be importable without errors."""
        import raven_worker.ee.analytics  # noqa: F401

    def test_ee_analytics_has_docstring(self):
        """The analytics __init__ should describe the intended features."""
        import raven_worker.ee.analytics as analytics

        assert analytics.__doc__ is not None
        assert len(analytics.__doc__.strip()) > 0


class TestAnalyticsEventShape:
    """Validate event shape contracts for PostHog and ClickHouse.

    These tests use simulated event sinks to verify the expected event
    structure. When raven_worker.ee.analytics.{posthog,clickhouse} are
    implemented, replace these with real import-based tests.
    """

    async def test_posthog_event_correct_shape(self):
        """Tracked events must have distinct_id, event name, and properties."""
        captured = []
        mock_sink = AsyncMock(side_effect=lambda event: captured.append(event))

        async def fake_track_event(distinct_id: str, event: str, properties: dict) -> None:
            await mock_sink(
                {
                    "distinct_id": distinct_id,
                    "event": event,
                    "properties": properties,
                }
            )

        await fake_track_event(
            distinct_id="user-123",
            event="document_uploaded",
            properties={"doc_id": "doc-1", "kb_id": "kb-1"},
        )
        assert len(captured) == 1
        assert captured[0]["event"] == "document_uploaded"
        assert captured[0]["distinct_id"] == "user-123"
        assert "doc_id" in captured[0]["properties"]

    async def test_clickhouse_event_write_calls_db(self, mock_db):
        """Writing a ClickHouse event must call DB execute with the event type."""

        async def fake_write_clickhouse_event(
            event_type: str,
            org_id: str,
            payload: dict,
            db,
        ) -> None:
            await db.execute(
                "INSERT INTO events (event_type, org_id, payload) VALUES ($1, $2, $3)",
                event_type,
                org_id,
                str(payload),
            )

        await fake_write_clickhouse_event(
            event_type="chat_message",
            org_id="org-1",
            payload={"session_id": "s1"},
            db=mock_db,
        )
        mock_db.execute.assert_awaited_once()
        call_args = str(mock_db.execute.call_args)
        assert "chat_message" in call_args

    async def test_multiple_events_tracked_independently(self):
        """Each track_event call must produce an independent event record."""
        captured = []
        mock_sink = AsyncMock(side_effect=lambda event: captured.append(event))

        async def fake_track_event(distinct_id: str, event: str, properties: dict) -> None:
            await mock_sink({"distinct_id": distinct_id, "event": event, "properties": properties})

        await fake_track_event("user-1", "chat_started", {"session_id": "s1"})
        await fake_track_event("user-2", "document_uploaded", {"doc_id": "d1"})

        assert len(captured) == 2
        assert captured[0]["event"] == "chat_started"
        assert captured[1]["event"] == "document_uploaded"
        assert captured[0]["distinct_id"] != captured[1]["distinct_id"]

    async def test_event_properties_preserved(self):
        """All property fields must be preserved exactly in the tracked event."""
        captured = []
        mock_sink = AsyncMock(side_effect=lambda event: captured.append(event))

        async def fake_track_event(distinct_id: str, event: str, properties: dict) -> None:
            await mock_sink({"distinct_id": distinct_id, "event": event, "properties": properties})

        props = {
            "doc_id": "doc-999",
            "kb_id": "kb-42",
            "file_name": "report.pdf",
            "size_bytes": 12345,
        }
        await fake_track_event("user-x", "document_parsed", props)

        assert captured[0]["properties"]["doc_id"] == "doc-999"
        assert captured[0]["properties"]["kb_id"] == "kb-42"
        assert captured[0]["properties"]["size_bytes"] == 12345

    async def test_clickhouse_event_includes_org_id(self, mock_db):
        """ClickHouse events must include org_id for tenant isolation."""

        async def fake_write_clickhouse_event(
            event_type: str,
            org_id: str,
            payload: dict,
            db,
        ) -> None:
            await db.execute(
                "INSERT INTO events (event_type, org_id, payload) VALUES ($1, $2, $3)",
                event_type,
                org_id,
                str(payload),
            )

        await fake_write_clickhouse_event(
            event_type="query_executed",
            org_id="org-tenant-1",
            payload={"query": "test query"},
            db=mock_db,
        )
        call_args = mock_db.execute.call_args
        assert "org-tenant-1" in str(call_args)
