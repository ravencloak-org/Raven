"""Semantic response cache backed by pgvector cosine similarity.

This module provides a second-layer cache that sits behind the exact-match
Valkey cache.  When a query does not match any cached response exactly, we
embed the query and search the ``response_cache`` table for semantically
similar entries using pgvector's cosine distance operator (``<=>``)
with a configurable similarity threshold (default 0.95).

The high threshold is deliberate — it ensures only near-identical queries
return cached responses, avoiding the risk of serving wrong answers.
"""

from __future__ import annotations

import json
import uuid

import asyncpg
import structlog

from raven_worker.providers.base import EmbeddingProvider

logger = structlog.get_logger(__name__)

# Expected embedding dimensionality — must match the vector(1536) column in the migration.
EXPECTED_DIMS = 1536

_LOOKUP_SQL = """
SELECT id, response_text, sources, model_name,
       1 - (query_embedding <=> $1::vector) AS sim
FROM response_cache
WHERE org_id = $2::uuid AND kb_id = $3::uuid
  AND 1 - (query_embedding <=> $1::vector) > $4
  AND expires_at > NOW()
ORDER BY sim DESC
LIMIT 1
"""

_INCREMENT_HIT_SQL = """
UPDATE response_cache SET hit_count = hit_count + 1 WHERE id = $1
"""

_STORE_SQL = """
INSERT INTO response_cache
    (id, org_id, kb_id, query_text, query_embedding, response_text, sources, model_name)
VALUES ($1, $2::uuid, $3::uuid, $4, $5::vector, $6, $7::jsonb, $8)
"""

_INVALIDATE_KB_SQL = """
DELETE FROM response_cache WHERE org_id = $1::uuid AND kb_id = $2::uuid
"""


class SemanticCache:
    """Semantic response cache using pgvector cosine similarity.

    Attributes:
        _pool: Shared ``asyncpg`` connection pool.
        _embedder: Embedding provider used to vectorise queries.
        _threshold: Minimum cosine similarity for a cache hit (default 0.95).
    """

    def __init__(
        self,
        pool: asyncpg.Pool,
        embedding_provider: EmbeddingProvider,
        similarity_threshold: float = 0.95,
    ) -> None:
        self._pool = pool
        self._embedder = embedding_provider
        self._threshold = similarity_threshold

    async def lookup(self, org_id: str, kb_id: str, query: str) -> dict | None:
        """Search for a semantically similar cached response.

        Args:
            org_id: UUID string of the requesting organisation.
            kb_id: UUID string of the knowledge base.
            query: The user query text.

        Returns:
            A dict with ``response_text``, ``sources``, and ``model_name``
            if a cache hit is found, otherwise ``None``.
        """
        try:
            query_embedding = await self._embedder.embed(query)
            embedding_str = "[" + ",".join(str(v) for v in query_embedding) + "]"

            async with self._pool.acquire() as conn, conn.transaction():
                await conn.execute("SELECT set_config('app.current_org_id', $1, true)", org_id)
                # nosemgrep: python.lang.security.audit.sqli.asyncpg-sqli -- hard-coded SQL, $N placeholders  # noqa: E501
                row = await conn.fetchrow(
                    _LOOKUP_SQL, embedding_str, org_id, kb_id, self._threshold
                )
                if row is not None:
                    await conn.execute(_INCREMENT_HIT_SQL, row["id"])

            if row is None:
                logger.debug(
                    "semantic_cache_miss",
                    org_id=org_id,
                    kb_id=kb_id,
                )
                return None

            sources = row["sources"] if row["sources"] else []
            if isinstance(sources, str):
                sources = json.loads(sources)

            logger.info(
                "semantic_cache_hit",
                org_id=org_id,
                kb_id=kb_id,
                similarity=float(row["sim"]),
                cache_id=str(row["id"]),
            )

            return {
                "response_text": row["response_text"],
                "sources": sources,
                "model_name": row["model_name"],
            }

        except Exception:
            logger.error("semantic_cache_lookup_error", exc_info=True)
            return None

    async def store(
        self,
        org_id: str,
        kb_id: str,
        query: str,
        query_embedding: list[float],
        response_text: str,
        sources: list[dict],
        model: str | None,
    ) -> None:
        """Store a RAG response in the semantic cache.

        Args:
            org_id: UUID string of the requesting organisation.
            kb_id: UUID string of the knowledge base.
            query: The original query text.
            query_embedding: Pre-computed embedding vector for the query.
            response_text: The full LLM response text.
            sources: List of source document references.
            model: LLM model name used for generation.
        """
        if len(query_embedding) != EXPECTED_DIMS:
            logger.error(
                "semantic_cache_store_dimension_mismatch",
                expected=EXPECTED_DIMS,
                got=len(query_embedding),
            )
            return

        try:
            cache_id = str(uuid.uuid4())
            embedding_str = "[" + ",".join(str(v) for v in query_embedding) + "]"
            sources_json = json.dumps(sources)

            async with self._pool.acquire() as conn, conn.transaction():
                await conn.execute("SELECT set_config('app.current_org_id', $1, true)", org_id)
                # nosemgrep: python.lang.security.audit.sqli.asyncpg-sqli -- hard-coded SQL, $N placeholders  # noqa: E501
                await conn.execute(
                    _STORE_SQL,
                    cache_id,
                    org_id,
                    kb_id,
                    query,
                    embedding_str,
                    response_text,
                    sources_json,
                    model,
                )

            logger.info(
                "semantic_cache_stored",
                org_id=org_id,
                kb_id=kb_id,
                cache_id=cache_id,
            )
        except Exception:
            logger.error("semantic_cache_store_error", exc_info=True)

    async def invalidate_kb(self, org_id: str, kb_id: str) -> None:
        """Delete all cached responses for a knowledge base.

        Called when a KB is updated (e.g. new documents ingested) to ensure
        stale answers are not served.

        Args:
            org_id: UUID string of the requesting organisation.
            kb_id: UUID string of the knowledge base.
        """
        try:
            async with self._pool.acquire() as conn, conn.transaction():
                await conn.execute("SELECT set_config('app.current_org_id', $1, true)", org_id)
                result = await conn.execute(_INVALIDATE_KB_SQL, org_id, kb_id)

            logger.info(
                "semantic_cache_invalidated",
                org_id=org_id,
                kb_id=kb_id,
                result=result,
            )
        except Exception:
            logger.error("semantic_cache_invalidate_error", exc_info=True)
