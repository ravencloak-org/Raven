"""Semantic response cache repository (Issue #256 — M9).

Runs cosine-similarity lookups against the ``response_cache`` table using the
pgvector HNSW index created in migration 00027 and extended in migration
00036. All queries are tenant-scoped via ``app.current_org_id`` set inside
the same transaction so RLS policies enforce org isolation automatically.

The hot path is lookup() — it must stay under ~10 ms p99 to satisfy the
M9 acceptance criterion. To achieve that:

* the HNSW index on ``query_embedding`` handles the ANN search in sub-ms,
* we execute exactly one prepared SELECT per lookup,
* expired rows are filtered by a composite ``(kb_id, expires_at)`` index
  added in 00036,
* the hit_count UPDATE is fire-and-forget after we've returned the answer.

Embedding dimensionality is 1536 (OpenAI text-embedding-3-small). Any other
provider must project into 1536 dims before calling this repo.
"""

from __future__ import annotations

import asyncio
import json
import time
from dataclasses import dataclass, field
from typing import Any

import asyncpg
import structlog

logger = structlog.get_logger(__name__)

# Default cosine-similarity threshold for a HIT. Matches the knowledge_bases
# column default in migration 00036. Kept in Python too for tests / callers
# that don't read the KB row.
DEFAULT_SIMILARITY_THRESHOLD = 0.92

# Lookup SQL. The `<=>` operator is pgvector's cosine distance (0..2);
# cosine similarity is `1 - (<=>)`. Filter expired rows first so the HNSW
# index doesn't return ghosts.
_LOOKUP_SQL = """
SELECT
  id::text,
  query_text,
  response_text,
  metadata,
  1 - (query_embedding <=> $1::vector) AS similarity,
  hit_count,
  created_at,
  expires_at
FROM response_cache
WHERE org_id = $4::uuid
  AND kb_id = $2::uuid
  AND expires_at > NOW()
  AND 1 - (query_embedding <=> $1::vector) >= $3
ORDER BY query_embedding <=> $1::vector
LIMIT 1
"""

_STORE_SQL = """
INSERT INTO response_cache
  (org_id, kb_id, query_text, query_embedding, response_text, metadata)
VALUES
  ($1::uuid, $2::uuid, $3, $4::vector, $5, $6::jsonb)
RETURNING id::text
"""

_INVALIDATE_SQL = """
DELETE FROM response_cache WHERE kb_id = $1::uuid
"""

_BUMP_HIT_SQL = "UPDATE response_cache SET hit_count = hit_count + 1 WHERE id = $1::uuid"


@dataclass(slots=True)
class CachedAnswer:
    """A single semantic-cache hit returned by :meth:`CacheRepository.lookup`."""

    id: str
    query_text: str
    answer_text: str
    similarity: float
    hit_count: int
    metadata: dict[str, Any] = field(default_factory=dict)
    latency_ms: float = 0.0  # measured lookup time, for observability


def _vector_literal(embedding: list[float]) -> str:
    """Render an embedding as the pgvector text representation.

    asyncpg does not natively understand pgvector's binary protocol, so we
    pass the literal string and let the server cast it to ``vector`` via
    ``$1::vector``. The `.7g` precision is plenty for 1536-dim cosine
    similarity and keeps the statement small.
    """
    if not embedding:
        raise ValueError("embedding cannot be empty")
    return "[" + ",".join(f"{float(v):.7g}" for v in embedding) + "]"


class CacheRepository:
    """Async pgvector-backed semantic cache.

    All public methods set the ``app.current_org_id`` RLS GUC before touching
    ``response_cache``. Connections come from an injected ``asyncpg.Pool``.
    """

    def __init__(self, pool: asyncpg.Pool):
        self._pool = pool
        # Strong references to in-flight detached tasks. Python's event loop
        # holds only weak references, so without this set the garbage
        # collector can cancel a task mid-execution. Tasks remove themselves
        # via add_done_callback once they finish.
        self._bg_tasks: set[asyncio.Task[Any]] = set()

    def _spawn(self, coro: Any) -> asyncio.Task[Any] | None:
        """Schedule a coroutine as a detached task, retaining a strong ref.

        Returns ``None`` when no running event loop is available (sync
        callers in tests). Errors in the coroutine itself are the task's
        own responsibility to log.
        """
        try:
            task = asyncio.create_task(coro)
        except RuntimeError:  # no running loop — sync context (tests)
            coro.close()
            return None
        self._bg_tasks.add(task)
        task.add_done_callback(self._bg_tasks.discard)
        return task

    async def lookup(
        self,
        *,
        org_id: str,
        kb_id: str,
        embedding: list[float],
        threshold: float = DEFAULT_SIMILARITY_THRESHOLD,
    ) -> CachedAnswer | None:
        """Return the most similar cached answer if cosine similarity ≥ threshold.

        Returns ``None`` on miss, on an empty embedding, or on any DB error.
        Errors are logged but NEVER raised — a cache miss must degrade to the
        full RAG pipeline, not break the request.
        """
        if not embedding:
            return None
        if threshold < 0 or threshold > 1:
            # Defensive: callers sometimes pass 0-1 scaled, sometimes 0-100.
            logger.warning("cache_lookup_bad_threshold", threshold=threshold)
            return None

        vec = _vector_literal(embedding)
        started = time.perf_counter()
        try:
            async with self._pool.acquire() as conn, conn.transaction():
                await conn.execute("SELECT set_config('app.current_org_id', $1, true)", org_id)
                row = await conn.fetchrow(_LOOKUP_SQL, vec, kb_id, threshold, org_id)
                if row is None:
                    return None
        except Exception as exc:  # pragma: no cover — logged-and-swallowed
            logger.warning(
                "cache_lookup_error",
                org_id=org_id,
                kb_id=kb_id,
                error=str(exc),
            )
            return None

        latency_ms = (time.perf_counter() - started) * 1000.0
        metadata = row["metadata"] or {}
        if isinstance(metadata, str):
            try:
                metadata = json.loads(metadata)
            except (TypeError, ValueError):
                metadata = {}
        # Defensive: jsonb column can legally decode to a non-object JSON
        # value (array, string, number). `dict(metadata)` on those would
        # raise outside the swallow-and-degrade handler above, breaking the
        # method contract that lookup() never raises.
        if not isinstance(metadata, dict):
            metadata = {}
        answer = CachedAnswer(
            id=row["id"],
            query_text=row["query_text"],
            answer_text=row["response_text"],
            similarity=float(row["similarity"]),
            hit_count=int(row["hit_count"]),
            metadata=dict(metadata),
            latency_ms=latency_ms,
        )
        # Fire-and-forget: bump hit_count on its own connection so the caller
        # never waits for UPDATE + commit. Strong task ref kept via _spawn so
        # the GC can't cancel it mid-flight.
        self._spawn(self._bump_hit_async(row["id"], org_id))
        return answer

    async def _bump_hit_async(self, row_id: str, org_id: str) -> None:
        """Best-effort hit_count UPDATE on a detached connection.

        Never raises — any exception is logged so the caller's response is not
        disturbed. Acquires its own connection from the pool and re-establishes
        the RLS GUC before issuing the UPDATE.
        """
        try:
            async with self._pool.acquire() as conn, conn.transaction():
                await conn.execute("SELECT set_config('app.current_org_id', $1, true)", org_id)
                await conn.execute(_BUMP_HIT_SQL, row_id)
        except Exception as exc:  # pragma: no cover — logged-and-swallowed
            logger.warning(
                "cache_bump_hit_error",
                row_id=row_id,
                org_id=org_id,
                error=str(exc),
            )

    async def store(
        self,
        *,
        org_id: str,
        kb_id: str,
        query_text: str,
        embedding: list[float],
        answer_text: str,
        metadata: dict[str, Any] | None = None,
    ) -> str | None:
        """Insert a new cache entry. Returns the row ID or None on failure.

        Intended to be called as ``asyncio.create_task(...)`` so an INSERT
        never blocks the streaming response.
        """
        if not embedding or not answer_text:
            return None
        vec = _vector_literal(embedding)
        payload = json.dumps(metadata or {})
        try:
            async with self._pool.acquire() as conn, conn.transaction():
                await conn.execute("SELECT set_config('app.current_org_id', $1, true)", org_id)
                row_id = await conn.fetchval(
                    _STORE_SQL, org_id, kb_id, query_text, vec, answer_text, payload
                )
                return row_id
        except Exception as exc:  # pragma: no cover — logged-and-swallowed
            logger.warning(
                "cache_store_error",
                org_id=org_id,
                kb_id=kb_id,
                error=str(exc),
            )
            return None

    async def invalidate_by_kb(self, *, org_id: str, kb_id: str) -> int:
        """Delete every cache entry belonging to kb_id. Returns row count."""
        try:
            async with self._pool.acquire() as conn, conn.transaction():
                await conn.execute("SELECT set_config('app.current_org_id', $1, true)", org_id)
                result = await conn.execute(_INVALIDATE_SQL, kb_id)
            # asyncpg returns "DELETE N"
            try:
                return int(result.split()[-1])
            except (ValueError, IndexError):
                return 0
        except Exception as exc:  # pragma: no cover — logged-and-swallowed
            logger.warning(
                "cache_invalidate_error",
                org_id=org_id,
                kb_id=kb_id,
                error=str(exc),
            )
            return 0

    async def store_async(
        self,
        *,
        org_id: str,
        kb_id: str,
        query_text: str,
        embedding: list[float],
        answer_text: str,
        metadata: dict[str, Any] | None = None,
    ) -> asyncio.Task[str | None] | None:
        """Schedule a store() in the background.

        Returns the scheduled task so callers that want to await completion
        can, but most callers fire-and-forget. A strong reference is kept
        internally so the GC cannot cancel the task mid-flight.
        """
        return self._spawn(
            self.store(
                org_id=org_id,
                kb_id=kb_id,
                query_text=query_text,
                embedding=embedding,
                answer_text=answer_text,
                metadata=metadata,
            )
        )
