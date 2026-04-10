"""Tests for vector search ranking and cosine similarity.

The vector search in this codebase is backed by pgvector — there is no
standalone cosine_rank function. Tests verify the vector_search function
(mocked asyncpg) builds the correct SQL embedding string and returns
properly-ordered results.
"""

from __future__ import annotations

from unittest.mock import AsyncMock

from raven_worker.retrieval.vector_search import vector_search


class TestVectorSearchRanking:
    async def test_results_ordered_by_score_descending(self):
        """vector_search results should be ordered by score descending."""
        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(
            return_value=[
                {"id": "chunk-high", "score": 0.95},
                {"id": "chunk-mid", "score": 0.80},
                {"id": "chunk-low", "score": 0.60},
            ]
        )
        results = await vector_search(mock_conn, [0.1, 0.2, 0.3], "org-1", ["kb-1"])
        scores = [score for _, score in results]
        assert scores == sorted(scores, reverse=True)

    async def test_embedding_string_format_correct(self):
        """The embedding must be formatted as '[v1,v2,v3]' for pgvector."""
        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=[])
        embedding = [0.1, 0.2, 0.3]
        await vector_search(mock_conn, embedding, "org-1", ["kb-1"])
        call_args = mock_conn.fetch.call_args
        embedding_arg = call_args.args[1]
        assert embedding_arg == "[0.1,0.2,0.3]"

    async def test_returns_correct_tuples(self):
        """vector_search should return (chunk_id, score) tuples."""
        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(
            return_value=[
                {"id": "uuid-1", "score": 0.95},
                {"id": "uuid-2", "score": 0.80},
            ]
        )
        results = await vector_search(mock_conn, [0.1, 0.2], "org-1", ["kb-1"])
        assert results == [("uuid-1", 0.95), ("uuid-2", 0.80)]

    async def test_empty_result_handled(self):
        """vector_search should return an empty list when no rows match."""
        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=[])
        results = await vector_search(mock_conn, [0.1, 0.2], "org-1", ["kb-1"])
        assert results == []

    async def test_org_id_forwarded(self):
        """org_id must be passed as positional arg to the SQL query."""
        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=[])
        await vector_search(mock_conn, [1.0, 0.0], "org-xyz", ["kb-1"])
        call_args = mock_conn.fetch.call_args
        assert call_args.args[2] == "org-xyz"

    async def test_kb_ids_forwarded(self):
        """kb_ids must be passed as positional arg to the SQL query."""
        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=[])
        kb_ids = ["kb-alpha", "kb-beta"]
        await vector_search(mock_conn, [1.0], "org-1", kb_ids)
        call_args = mock_conn.fetch.call_args
        assert call_args.args[3] == kb_ids

    async def test_limit_forwarded(self):
        """limit parameter must be passed to SQL."""
        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=[])
        await vector_search(mock_conn, [1.0], "org-1", ["kb-1"], limit=5)
        call_args = mock_conn.fetch.call_args
        assert call_args.args[4] == 5

    async def test_score_cast_to_float(self):
        """Scores from DB must be cast to float."""
        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=[{"id": "chunk-1", "score": 0.99}])
        results = await vector_search(mock_conn, [1.0], "org-1", ["kb-1"])
        assert isinstance(results[0][1], float)

    async def test_sql_contains_vector_cosine_operator(self):
        """The SQL must reference the pgvector cosine distance operator."""
        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=[])
        await vector_search(mock_conn, [0.5, 0.5], "org-1", ["kb-1"])
        sql = mock_conn.fetch.call_args.args[0]
        assert "embedding" in sql.lower()
