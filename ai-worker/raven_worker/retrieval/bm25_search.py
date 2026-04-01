"""BM25-style full-text search using PostgreSQL tsvector and ts_rank_cd."""

from __future__ import annotations

import asyncpg
import structlog

logger = structlog.get_logger(__name__)

_BM25_SEARCH_SQL = """
SELECT c.id::text,
       ts_rank_cd(
           to_tsvector('english', coalesce(c.heading, '') || ' ' || c.content),
           q
       ) AS score
FROM chunks c,
     plainto_tsquery('english', $1) q
WHERE c.org_id = $2::uuid
  AND c.knowledge_base_id = ANY($3::uuid[])
  AND to_tsvector('english', coalesce(c.heading, '') || ' ' || c.content) @@ q
ORDER BY score DESC
LIMIT $4
"""


async def bm25_search(
    conn: asyncpg.Connection,
    query: str,
    org_id: str,
    kb_ids: list[str],
    limit: int = 20,
) -> list[tuple[str, float]]:
    """Run full-text BM25 search using PostgreSQL ``ts_rank_cd``.

    Args:
        conn: An active asyncpg connection (with RLS GUC already set).
        query: The natural-language query string.
        org_id: UUID string of the requesting organisation.
        kb_ids: List of knowledge-base UUID strings to search within.
        limit: Maximum number of results to return.

    Returns:
        List of ``(chunk_id, ts_rank_cd_score)`` tuples ordered by score
        descending.
    """
    logger.debug(
        "bm25_search_start",
        org_id=org_id,
        kb_ids=kb_ids,
        limit=limit,
        query_length=len(query),
    )

    rows = await conn.fetch(
        _BM25_SEARCH_SQL,
        query,
        org_id,
        kb_ids,
        limit,
    )

    results = [(row["id"], float(row["score"])) for row in rows]
    logger.debug("bm25_search_done", result_count=len(results))
    return results
