"""Tests for BM25 full-text search (PostgreSQL tsvector-based).

The BM25 search in this codebase is backed by PostgreSQL ts_rank_cd —
there is no standalone in-memory BM25Scorer class. Tests use a mocked
asyncpg connection to verify the correct SQL and parameters are used.
"""

from __future__ import annotations

from unittest.mock import AsyncMock

from raven_worker.retrieval.bm25_search import bm25_search


class TestBM25Search:
    async def test_returns_correct_result_tuples(self):
        """bm25_search should return (chunk_id, score) tuples from the DB."""
        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(
            return_value=[
                {"id": "chunk-a", "score": 0.75},
                {"id": "chunk-b", "score": 0.50},
            ]
        )
        results = await bm25_search(mock_conn, "machine learning", "org-1", ["kb-1"])
        assert results == [("chunk-a", 0.75), ("chunk-b", 0.50)]

    async def test_sql_uses_ts_rank_cd(self):
        """The SQL query must use PostgreSQL ts_rank_cd for BM25 scoring."""
        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=[{"id": "x", "score": 0.1}])
        await bm25_search(mock_conn, "dog", "org-1", ["kb-1"])
        call_args = mock_conn.fetch.call_args
        sql = call_args.args[0]
        assert "ts_rank_cd" in sql.lower()
        assert "plainto_tsquery" in sql.lower()

    async def test_query_text_passed_as_first_arg(self):
        """The query text must be the first positional SQL arg."""
        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=[])
        await bm25_search(mock_conn, "neural network", "org-2", ["kb-2"])
        call_args = mock_conn.fetch.call_args
        assert call_args.args[1] == "neural network"

    async def test_org_id_passed_correctly(self):
        """The org_id must be forwarded to the SQL as a UUID parameter."""
        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=[])
        await bm25_search(mock_conn, "query", "org-uuid-123", ["kb-1"])
        call_args = mock_conn.fetch.call_args
        assert call_args.args[2] == "org-uuid-123"

    async def test_kb_ids_passed_correctly(self):
        """The kb_ids list must be forwarded to the SQL for filtering."""
        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=[])
        kb_ids = ["kb-a", "kb-b"]
        await bm25_search(mock_conn, "query", "org-1", kb_ids)
        call_args = mock_conn.fetch.call_args
        assert call_args.args[3] == kb_ids

    async def test_limit_respected(self):
        """The limit parameter must be forwarded to SQL."""
        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=[])
        await bm25_search(mock_conn, "query", "org-1", ["kb-1"], limit=7)
        call_args = mock_conn.fetch.call_args
        assert call_args.args[4] == 7

    async def test_empty_result_returns_empty_list(self):
        """bm25_search should return an empty list when no rows match."""
        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=[])
        results = await bm25_search(mock_conn, "mango", "org-1", ["kb-1"])
        assert results == []

    async def test_score_cast_to_float(self):
        """Scores returned from DB must be cast to float."""
        mock_conn = AsyncMock()
        # Simulate DB returning Decimal or other numeric type
        mock_conn.fetch = AsyncMock(return_value=[{"id": "chunk-1", "score": 0.99}])
        results = await bm25_search(mock_conn, "query", "org-1", ["kb-1"])
        assert isinstance(results[0][1], float)

    async def test_multiple_kb_ids_forwarded(self):
        """Multi-KB queries must pass all KB IDs to SQL."""
        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=[])
        kb_ids = ["kb-1", "kb-2", "kb-3"]
        await bm25_search(mock_conn, "query", "org-1", kb_ids)
        call_args = mock_conn.fetch.call_args
        assert call_args.args[3] == kb_ids
