"""Vector similarity search using pgvector cosine distance."""

from __future__ import annotations

import asyncpg
import structlog

logger = structlog.get_logger(__name__)

_VECTOR_SEARCH_SQL = """
SELECT c.id::text, 1 - (e.embedding <=> $1::vector) AS score
FROM chunks c
JOIN embeddings e ON e.chunk_id = c.id
WHERE c.org_id = $2::uuid
  AND c.knowledge_base_id = ANY($3::uuid[])
ORDER BY e.embedding <=> $1::vector
LIMIT $4
"""


async def vector_search(
    conn: asyncpg.Connection,
    query_embedding: list[float],
    org_id: str,
    kb_ids: list[str],
    limit: int = 20,
) -> list[tuple[str, float]]:
    """Run cosine-similarity vector search against the embeddings table.

    Args:
        conn: An active asyncpg connection (with RLS GUC already set).
        query_embedding: The query embedding as a list of floats.
        org_id: UUID string of the requesting organisation.
        kb_ids: List of knowledge-base UUID strings to search within.
        limit: Maximum number of results to return.

    Returns:
        List of ``(chunk_id, cosine_similarity_score)`` tuples ordered by
        score descending.
    """
    # Build the pgvector-compatible string representation, e.g. "[0.1,0.2,...]"
    embedding_str = "[" + ",".join(str(v) for v in query_embedding) + "]"

    logger.debug(
        "vector_search_start",
        org_id=org_id,
        kb_ids=kb_ids,
        limit=limit,
        embedding_dims=len(query_embedding),
    )

    rows = await conn.fetch(
        _VECTOR_SEARCH_SQL,
        embedding_str,
        org_id,
        kb_ids,
        limit,
    )

    results = [(row["id"], float(row["score"])) for row in rows]
    logger.debug("vector_search_done", result_count=len(results))
    return results
