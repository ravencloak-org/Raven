"""RAG service implementing the QueryRAG server-streaming RPC.

Full pipeline:
    1. Embed the query using the BYOK embedding provider.
    2. Run vector (cosine) search and BM25 full-text search in parallel.
    3. Merge ranked lists with Reciprocal Rank Fusion (RRF).
    4. Optionally re-rank top chunks with Cohere Rerank.
    5. Build a context string from the top chunks.
    6. Stream LLM completion tokens back as ``RAGChunk`` proto messages.
"""

from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator

import anthropic
import asyncpg
import grpc
import structlog
from openai import AsyncOpenAI

from raven_worker.config import settings
from raven_worker.crypto import decrypt_api_key
from raven_worker.generated import ai_worker_pb2
from raven_worker.providers.registry import get_provider_for_request
from raven_worker.retrieval.bm25_search import bm25_search
from raven_worker.retrieval.reranker import cohere_rerank
from raven_worker.retrieval.rrf import reciprocal_rank_fusion
from raven_worker.retrieval.vector_search import vector_search

logger = structlog.get_logger(__name__)

SYSTEM_PROMPT = """You are a helpful assistant. Answer the question based on the provided context.
If the answer is not in the context, say you don't know.

Context:
{context}
"""

_FETCH_CHUNKS_SQL = """
SELECT c.id::text, c.content, c.heading, c.document_id::text, d.title AS document_name
FROM chunks c
LEFT JOIN documents d ON d.id = c.document_id
WHERE c.id = ANY($1::uuid[]) AND c.org_id = $2::uuid
"""


class RAGServicer:
    """Handles QueryRAG streaming RPC calls.

    Attributes:
        _pool: Shared ``asyncpg`` connection pool.  Created lazily on the
            first RPC call if not injected via the constructor.
    """

    def __init__(self, pool: asyncpg.Pool | None = None) -> None:
        self._pool = pool

    async def _get_pool(self) -> asyncpg.Pool:
        """Return the shared connection pool, creating it if necessary."""
        if self._pool is None:
            self._pool = await asyncpg.create_pool(settings.database_url)
        return self._pool

    # ------------------------------------------------------------------
    # Public RPC handler
    # ------------------------------------------------------------------

    async def query(self, request, context) -> AsyncIterator[ai_worker_pb2.RAGChunk]:
        """Stream RAG response chunks for the given query.

        Args:
            request: :class:`ai_worker_pb2.RAGRequest` with fields
                ``query``, ``org_id``, ``kb_ids``, ``session_id``,
                ``filters``, ``model``, ``provider``.
            context: gRPC ``ServicerContext`` for aborting with status codes.

        Yields:
            :class:`ai_worker_pb2.RAGChunk` messages — non-final chunks
            carry LLM tokens; the final chunk has ``is_final=True`` and
            the ``sources`` list populated.
        """
        org_id = request.org_id
        kb_ids = list(request.kb_ids)
        query_text = request.query
        provider_name = request.provider
        model = request.model
        filters: dict[str, str] = dict(request.filters)

        log = logger.bind(
            org_id=org_id,
            kb_ids=kb_ids,
            session_id=request.session_id,
            model=model,
            provider=provider_name,
            query_length=len(query_text),
        )
        log.info("query_rag_request")

        try:
            async for chunk in self._run_pipeline(
                query_text, org_id, kb_ids, provider_name, model, filters, log
            ):
                yield chunk
        except Exception as exc:
            log.exception("query_rag_error", error=str(exc))
            await context.abort(grpc.StatusCode.INTERNAL, str(exc))

    # ------------------------------------------------------------------
    # Internal pipeline helpers
    # ------------------------------------------------------------------

    async def _run_pipeline(
        self,
        query_text: str,
        org_id: str,
        kb_ids: list[str],
        provider_name: str,
        model: str,
        filters: dict[str, str],
        log: structlog.BoundLogger,
    ) -> AsyncIterator[ai_worker_pb2.RAGChunk]:
        """Execute the full RAG pipeline and yield streaming chunks."""
        # 1. Embed the query
        embedding_provider = await get_provider_for_request(org_id, provider_name, model)
        query_embedding = await embedding_provider.embed(query_text)
        log.debug("query_embedded", dims=len(query_embedding))

        # 2. Hybrid search (vector + BM25) in parallel — share a single connection
        pool = await self._get_pool()
        async with pool.acquire() as conn:
            await conn.execute("SELECT set_config('app.current_org_id', $1, false)", org_id)
            vector_results, bm25_results = await asyncio.gather(
                vector_search(conn, query_embedding, org_id, kb_ids),
                bm25_search(conn, query_text, org_id, kb_ids),
            )

        log.debug(
            "hybrid_search_done",
            vector_hits=len(vector_results),
            bm25_hits=len(bm25_results),
        )

        # 3. RRF fusion
        fused = reciprocal_rank_fusion([vector_results, bm25_results], top_n=10)
        log.debug("rrf_done", fused_count=len(fused))

        if not fused:
            log.info("no_chunks_found")
            yield ai_worker_pb2.RAGChunk(
                text="No relevant information found.",
                is_final=True,
                sources=[],
            )
            return

        # 4. Fetch full chunk content for top RRF results
        chunk_ids = [cid for cid, _ in fused]
        rrf_score_map = {cid: score for cid, score in fused}

        async with pool.acquire() as conn:
            await conn.execute("SELECT set_config('app.current_org_id', $1, false)", org_id)
            rows = await conn.fetch(_FETCH_CHUNKS_SQL, chunk_ids, org_id)

        chunk_records: list[dict] = [
            {
                "id": row["id"],
                "content": row["content"] or "",
                "heading": row["heading"] or "",
                "document_id": row["document_id"] or "",
                "document_name": row["document_name"] or "",
                "score": rrf_score_map.get(row["id"], 0.0),
            }
            for row in rows
        ]
        # Preserve RRF order
        chunk_records.sort(key=lambda r: r["score"], reverse=True)
        log.debug("chunks_fetched", count=len(chunk_records))

        # 5. Optional Cohere reranking
        use_cohere_rerank = filters.get("rerank") == "cohere"
        if use_cohere_rerank and chunk_records:
            cohere_api_key = await self._get_llm_api_key(org_id, "cohere")
            if cohere_api_key:
                reranked = await cohere_rerank(
                    query=query_text,
                    chunks=chunk_records,
                    api_key=cohere_api_key,
                    top_n=5,
                )
                if reranked is not None:
                    chunk_records = reranked
                    log.debug("rerank_applied", count=len(chunk_records))

        # 6. Build context from top chunks
        context_parts = []
        for i, chunk in enumerate(chunk_records, start=1):
            heading = chunk["heading"]
            content = chunk["content"]
            part = f"[{i}] {heading}\n{content}" if heading else f"[{i}] {content}"
            context_parts.append(part)
        context_str = "\n\n".join(context_parts)

        # 7. Get LLM API key and stream completion
        llm_api_key = await self._get_llm_api_key(org_id, provider_name)
        if llm_api_key is None:
            raise ValueError(
                f"No active LLM provider config found for org '{org_id}' "
                f"and provider '{provider_name}'"
            )

        prompt = SYSTEM_PROMPT.format(context=context_str)
        messages = [{"role": "user", "content": query_text}]

        # Build sources for final chunk
        sources = [
            ai_worker_pb2.Source(
                document_id=chunk["document_id"],
                document_name=chunk["document_name"],
                chunk_text=chunk["content"][:500],
                score=chunk["score"],
            )
            for chunk in chunk_records
        ]

        # 8. Stream LLM tokens
        async for token_chunk in self._stream_llm(
            provider_name, llm_api_key, model, prompt, messages
        ):
            yield token_chunk

        # Final chunk with sources
        yield ai_worker_pb2.RAGChunk(text="", is_final=True, sources=sources)

    async def _get_llm_api_key(self, org_id: str, provider: str) -> str | None:
        """Look up and decrypt the LLM provider API key from the database.

        Args:
            org_id: UUID string of the requesting organisation.
            provider: Provider slug, e.g. ``"openai"`` or ``"anthropic"``.

        Returns:
            Decrypted API key string, or ``None`` if no active config found.
        """
        pool = await self._get_pool()
        async with pool.acquire() as conn:
            await conn.execute("SELECT set_config('app.current_org_id', $1, false)", org_id)
            row = await conn.fetchrow(
                """
                SELECT api_key_encrypted, api_key_iv, base_url
                FROM llm_provider_configs
                WHERE org_id = $1
                  AND provider = $2
                  AND status = 'active'
                  AND is_default = true
                """,
                org_id,
                provider,
            )

        if row is None:
            return None

        return decrypt_api_key(
            encrypted=bytes(row["api_key_encrypted"]),
            iv=bytes(row["api_key_iv"]),
            key_b64=settings.encryption_key,
        )

    async def _stream_llm(
        self,
        provider: str,
        api_key: str,
        model: str,
        system_prompt: str,
        messages: list[dict],
    ) -> AsyncIterator[ai_worker_pb2.RAGChunk]:
        """Stream LLM completion tokens for the supported providers.

        Args:
            provider: Provider slug (``"openai"`` or ``"anthropic"``).
            api_key: Decrypted provider API key.
            model: LLM model identifier.
            system_prompt: System-level prompt (with injected context).
            messages: User/assistant message history.

        Yields:
            Non-final :class:`ai_worker_pb2.RAGChunk` messages, one per token.
        """
        if provider == "openai":
            async for chunk in self._stream_openai(api_key, model, system_prompt, messages):
                yield chunk
        elif provider == "anthropic":
            async for chunk in self._stream_anthropic(api_key, model, system_prompt, messages):
                yield chunk
        else:
            raise ValueError(f"LLM streaming not supported for provider '{provider}'")

    async def _stream_openai(
        self,
        api_key: str,
        model: str,
        system_prompt: str,
        messages: list[dict],
    ) -> AsyncIterator[ai_worker_pb2.RAGChunk]:
        """Stream tokens from the OpenAI Chat Completions API."""
        client = AsyncOpenAI(api_key=api_key)
        full_messages = [{"role": "system", "content": system_prompt}, *messages]

        stream = await client.chat.completions.create(
            model=model,
            messages=full_messages,  # type: ignore[arg-type]
            stream=True,
        )
        async for event in stream:  # type: ignore[union-attr]
            delta = event.choices[0].delta.content if event.choices else None
            if delta:
                yield ai_worker_pb2.RAGChunk(text=delta, is_final=False, sources=[])

    async def _stream_anthropic(
        self,
        api_key: str,
        model: str,
        system_prompt: str,
        messages: list[dict],
    ) -> AsyncIterator[ai_worker_pb2.RAGChunk]:
        """Stream tokens from the Anthropic Messages API."""
        client = anthropic.AsyncAnthropic(api_key=api_key)
        async with client.messages.stream(
            model=model,
            system=system_prompt,
            messages=messages,  # type: ignore[arg-type]
            max_tokens=2048,
        ) as stream:
            async for text in stream.text_stream:
                yield ai_worker_pb2.RAGChunk(text=text, is_final=False, sources=[])
