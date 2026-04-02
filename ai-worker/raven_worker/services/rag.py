"""RAG service implementing the QueryRAG server-streaming RPC.

Full pipeline:
    1. Check exact-match response cache in Valkey.
    2. Embed the query using the BYOK embedding provider.
    3. Run vector (cosine) search and BM25 full-text search in parallel.
    4. Merge ranked lists with Reciprocal Rank Fusion (RRF).
    5. Optionally re-rank top chunks with Cohere Rerank.
    6. Build a context string from the top chunks.
    7. Stream LLM completion tokens back as ``RAGChunk`` proto messages.
    8. Cache the completed response for future exact-match hits.
"""

from __future__ import annotations

import asyncio
import hashlib
import json
from collections.abc import AsyncIterator

import anthropic
import asyncpg
import grpc
import redis.asyncio as aioredis
import structlog
from openai import AsyncOpenAI

from raven_worker.config import settings
from raven_worker.crypto import decrypt_api_key
from raven_worker.generated import ai_worker_pb2
from raven_worker.memory import MEMORY_TOOL, MemoryStore
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


_CACHE_KEY_PREFIX = "raven:cache:rag:"
_CACHE_TTL_SECONDS = 3600  # 1 hour

_MEMORY_SYSTEM_PROMPT_SUFFIX = (
    "\n\nIMPORTANT: Before answering, use the memory tool to check /memories "
    "for relevant context from previous conversations with this user."
)


def _cache_key(kb_id: str, query: str) -> str:
    """Build a deterministic cache key matching the Go-side format exactly."""
    normalized = query.lower().strip()
    digest = hashlib.sha256(f"{kb_id}:{normalized}".encode()).hexdigest()
    return f"{_CACHE_KEY_PREFIX}{digest}"


def _append_qa_to_memory(memory_store: MemoryStore, query: str, answer: str) -> None:
    """Append a Q&A pair to the session memory file after each response."""
    import datetime

    ts = datetime.datetime.now(datetime.UTC).strftime("%Y-%m-%d %H:%M")
    entry = f"\n## {ts}\n**Q:** {query}\n**A:** {answer[:800]}{'…' if len(answer) > 800 else ''}\n"

    existing = memory_store.handle("view", "session.md")
    if existing.startswith("Error") or "does not exist" in existing:
        memory_store.handle(
            "create",
            "session.md",
            file_text=f"# Session Memory\n{entry}",
        )
    else:
        # Append by replacing the end of the file
        current_text = memory_store.root.joinpath("session.md").read_text(encoding="utf-8")
        memory_store.handle(
            "str_replace",
            "session.md",
            old_str=current_text,
            new_str=current_text + entry,
        )


class RAGServicer:
    """Handles QueryRAG streaming RPC calls.

    Attributes:
        _pool: Shared ``asyncpg`` connection pool.  Created lazily on the
            first RPC call if not injected via the constructor.
        _redis: Async Redis client for response caching. Created lazily from
            ``settings.valkey_url``. All cache operations are gracefully
            degraded — if Valkey is unavailable the full RAG pipeline runs
            without caching.
    """

    def __init__(
        self,
        pool: asyncpg.Pool | None = None,
        redis_client: aioredis.Redis | None = None,
    ) -> None:
        self._pool = pool
        self._redis: aioredis.Redis | None = redis_client

    async def _get_pool(self) -> asyncpg.Pool:
        """Return the shared connection pool, creating it if necessary."""
        if self._pool is None:
            self._pool = await asyncpg.create_pool(settings.database_url)
        return self._pool

    async def _get_redis(self) -> aioredis.Redis | None:
        """Return the Redis client, creating it lazily if necessary."""
        if self._redis is None:
            try:
                self._redis = aioredis.from_url(settings.valkey_url)
            except Exception:
                logger.warning("valkey_connection_failed", url=settings.valkey_url)
                return None
        return self._redis

    async def _check_cache(self, cache_key: str) -> dict | None:
        """Look up a cached response by key. Returns None on miss or error."""
        try:
            rds = await self._get_redis()
            if rds is None:
                return None
            data = await rds.get(cache_key)
            if data is None:
                return None
            return json.loads(data)
        except Exception:
            logger.debug("cache_check_error", cache_key=cache_key, exc_info=True)
            return None

    async def _store_cache(self, cache_key: str, payload: dict) -> None:
        """Store a response in cache. Failures are silently ignored."""
        try:
            rds = await self._get_redis()
            if rds is None:
                return
            await rds.setex(cache_key, _CACHE_TTL_SECONDS, json.dumps(payload))
        except Exception:
            logger.debug("cache_store_error", cache_key=cache_key, exc_info=True)

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

        # --- Exact-match cache check (per KB) ---
        # When the request targets a single KB we can perform an exact-match
        # cache lookup.  Multi-KB queries skip the cache for now.
        # The cache is also bypassed for Anthropic requests that use memory or
        # web search — those responses are session-aware and must not be shared.
        cache_key: str | None = None
        cache_safe = len(kb_ids) == 1 and not (
            provider_name == "anthropic"
            and ((settings.memory_dir and request.session_id) or settings.enable_web_search)
        )
        if cache_safe:
            cache_key = _cache_key(kb_ids[0], query_text)
            cached = await self._check_cache(cache_key)
            if cached is not None:
                log.info("cache_hit", cache_key=cache_key)
                sources = [
                    ai_worker_pb2.Source(
                        document_id=s.get("document_id", ""),
                        document_name=s.get("document_name", ""),
                        chunk_text=s.get("chunk_text", ""),
                        score=s.get("score", 0.0),
                    )
                    for s in cached.get("sources", [])
                ]
                yield ai_worker_pb2.RAGChunk(
                    text=cached.get("text", ""),
                    is_final=True,
                    sources=sources,
                )
                return

        try:
            collected_text: list[str] = []
            final_sources: list[dict] = []

            async for chunk in self._run_pipeline(
                query_text,
                org_id,
                kb_ids,
                provider_name,
                model,
                filters,
                request.session_id,
                log,
            ):
                if not chunk.is_final:
                    collected_text.append(chunk.text)
                else:
                    final_sources = [
                        {
                            "document_id": s.document_id,
                            "document_name": s.document_name,
                            "chunk_text": s.chunk_text,
                            "score": s.score,
                        }
                        for s in chunk.sources
                    ]
                yield chunk

            # --- Store in cache after successful pipeline completion ---
            if cache_key and collected_text:
                payload = {
                    "text": "".join(collected_text),
                    "sources": final_sources,
                    "model": model,
                }
                await self._store_cache(cache_key, payload)
                log.info("cache_stored", cache_key=cache_key)

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
        session_id: str,
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
            provider_name,
            llm_api_key,
            model,
            prompt,
            messages,
            session_id=session_id,
            org_id=org_id,
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
        session_id: str = "",
        org_id: str = "",
    ) -> AsyncIterator[ai_worker_pb2.RAGChunk]:
        """Stream LLM completion tokens for the supported providers.

        Args:
            provider: Provider slug (``"openai"`` or ``"anthropic"``).
            api_key: Decrypted provider API key.
            model: LLM model identifier.
            system_prompt: System-level prompt (with injected context).
            messages: User/assistant message history.
            session_id: Session identifier for per-session memory (Anthropic only).
            org_id: Org identifier for per-org memory scoping (Anthropic only).

        Yields:
            Non-final :class:`ai_worker_pb2.RAGChunk` messages, one per token.
        """
        if provider == "openai":
            async for chunk in self._stream_openai(api_key, model, system_prompt, messages):
                yield chunk
        elif provider == "anthropic":
            async for chunk in self._stream_anthropic(
                api_key,
                model,
                system_prompt,
                messages,
                session_id=session_id,
                org_id=org_id,
            ):
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
        session_id: str = "",
        org_id: str = "",
    ) -> AsyncIterator[ai_worker_pb2.RAGChunk]:
        """Stream tokens from the Anthropic Messages API.

        Integrates three Anthropic features on top of plain streaming:

        * **Prompt caching** — adds ``cache_control: ephemeral`` to the system
          prompt so repeated RAG context tokens are cached across calls, reducing
          input token costs by up to 90 % on cache hits.

        * **Memory tool** — when ``RAVEN_MEMORY_DIR`` is set, Claude runs a
          short non-streaming pre-flight to read per-session memory files, then
          appends that context before streaming the final answer.  After streaming
          completes, the Q&A pair is saved back to memory for future sessions.

        * **Web search** — when ``RAVEN_ENABLE_WEB_SEARCH=true``, the Anthropic
          server-side ``web_search_20260209`` tool is included so Claude can
          supplement knowledge-base context with live web results.
        """
        client = anthropic.AsyncAnthropic(api_key=api_key)

        # ── Prompt caching: wrap system prompt as a cacheable block ──────────
        # The cache_control marker tells Anthropic to cache this prefix across
        # calls. Cache TTL is 5 minutes; a hit costs ~10 % of normal input price.
        system_blocks: list[dict] = [
            {"type": "text", "text": system_prompt, "cache_control": {"type": "ephemeral"}},
        ]

        # ── Build tools list ──────────────────────────────────────────────────
        tools: list[dict] = []

        # Memory tool (client-side) — we execute file operations locally
        memory_store: MemoryStore | None = None
        if settings.memory_dir and session_id and org_id:
            memory_store = MemoryStore(settings.memory_dir, org_id, session_id)
            # cache_control on last tool definition caches the entire tools prefix
            tools.append({**MEMORY_TOOL, "cache_control": {"type": "ephemeral"}})

        # Web search (server-side) — Anthropic executes searches automatically
        if settings.enable_web_search:
            web_tool: dict = {"type": "web_search_20260209", "name": "web_search"}
            if tools:
                # Move cache_control to the last tool to extend the cached prefix
                tools[-1].pop("cache_control", None)
            tools.append({**web_tool, "cache_control": {"type": "ephemeral"}})

        # ── Memory pre-flight ─────────────────────────────────────────────────
        # Non-streaming agentic loop: Claude views/reads memory files, then we
        # inject what it learned into the conversation before streaming the answer.
        current_messages: list[dict] = list(messages)

        if memory_store:
            memory_system = (
                system_blocks[0]["text"]
                + "\n\nIMPORTANT: Before answering, use the memory tool to check "
                "/memories for any relevant context from previous conversations "
                "with this user. Read files that seem relevant to the current query."
            )
            preflight_system = [
                {"type": "text", "text": memory_system, "cache_control": {"type": "ephemeral"}}
            ]
            preflight_tools = [t for t in tools if t.get("name") == "memory"]

            try:
                response = await client.messages.create(
                    model=model,
                    system=preflight_system,  # type: ignore[arg-type]
                    messages=current_messages,  # type: ignore[arg-type]
                    tools=preflight_tools,  # type: ignore[arg-type]
                    max_tokens=1024,
                )

                # Execute memory tool calls until Claude stops requesting them
                while response.stop_reason == "tool_use":
                    tool_results = []
                    for block in response.content:
                        if block.type == "tool_use" and block.name == "memory":
                            result = memory_store.handle(
                                command=str(block.input.get("command", "view")),
                                path=str(block.input.get("path", "/memories")),
                                **{
                                    k: str(v)
                                    for k, v in block.input.items()
                                    if k not in ("command", "path")
                                },
                            )
                            logger.debug(
                                "memory_tool_call",
                                command=block.input.get("command"),
                                path=block.input.get("path"),
                            )
                            tool_results.append(
                                {
                                    "type": "tool_result",
                                    "tool_use_id": block.id,
                                    "content": result,
                                }
                            )

                    current_messages = [
                        *current_messages,
                        {"role": "assistant", "content": response.content},
                        {"role": "user", "content": tool_results},
                    ]
                    response = await client.messages.create(
                        model=model,
                        system=preflight_system,  # type: ignore[arg-type]
                        messages=current_messages,  # type: ignore[arg-type]
                        tools=preflight_tools,  # type: ignore[arg-type]
                        max_tokens=1024,
                    )

                # Append the completed pre-flight turn so the streaming call
                # has full context of what Claude found in memory
                current_messages = [
                    *current_messages,
                    {"role": "assistant", "content": response.content},
                ]

            except Exception:  # noqa: BLE001
                logger.warning("memory_preflight_error", exc_info=True)
                # Non-fatal: fall through and answer without memory context

        # ── Stream the final answer ───────────────────────────────────────────
        stream_kwargs: dict = {
            "model": model,
            "system": system_blocks,
            "messages": current_messages,
            "max_tokens": 2048,
        }
        if tools:
            stream_kwargs["tools"] = tools

        collected_tokens: list[str] = []
        async with client.messages.stream(**stream_kwargs) as stream:  # type: ignore[arg-type]
            async for text in stream.text_stream:
                collected_tokens.append(text)
                yield ai_worker_pb2.RAGChunk(text=text, is_final=False, sources=[])

        # ── Post-stream: save Q&A to memory ──────────────────────────────────
        if memory_store and collected_tokens:
            user_query = (
                messages[-1]["content"]
                if messages and isinstance(messages[-1].get("content"), str)
                else ""
            )
            answer = "".join(collected_tokens)
            _append_qa_to_memory(memory_store, user_query, answer)
