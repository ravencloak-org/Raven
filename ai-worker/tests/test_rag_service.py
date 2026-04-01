"""Tests for the RAG query pipeline: RRF, vector search, BM25, and RAGServicer."""

from __future__ import annotations

from contextlib import asynccontextmanager
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from raven_worker.generated import ai_worker_pb2
from raven_worker.retrieval.rrf import reciprocal_rank_fusion
from raven_worker.services.rag import RAGServicer

# ---------------------------------------------------------------------------
# Helper: build a mock asyncpg pool whose acquire() acts as an async CM
# ---------------------------------------------------------------------------


def _make_mock_pool(mock_conn: AsyncMock) -> MagicMock:
    """Return a mock pool whose ``acquire()`` yields ``mock_conn``."""
    pool = MagicMock()

    @asynccontextmanager
    async def _acquire():
        yield mock_conn

    pool.acquire = _acquire
    return pool


# ---------------------------------------------------------------------------
# RRF unit tests (pure Python — no DB)
# ---------------------------------------------------------------------------


def test_rrf_basic() -> None:
    """RRF should merge two ranked lists and return correct top scores."""
    list_a = [("chunk-1", 0.9), ("chunk-2", 0.7), ("chunk-3", 0.5)]
    list_b = [("chunk-2", 0.95), ("chunk-1", 0.8), ("chunk-4", 0.6)]

    results = reciprocal_rank_fusion([list_a, list_b], k=60, top_n=4)

    # chunk-1 appears at rank 1 in list_a and rank 2 in list_b → score = 1/61 + 1/62
    # chunk-2 appears at rank 2 in list_a and rank 1 in list_b → score = 1/62 + 1/61
    # Both have the same score; they should both appear in top results.
    result_ids = [r[0] for r in results]
    assert "chunk-1" in result_ids
    assert "chunk-2" in result_ids
    # chunk-4 only appears in list_b at rank 3 → lower score
    assert "chunk-4" in result_ids
    # Verify scores are > 0
    for _, score in results:
        assert score > 0.0
    # Verify descending order
    scores = [s for _, s in results]
    assert scores == sorted(scores, reverse=True)


def test_rrf_basic_top_n_respected() -> None:
    """RRF should return at most top_n results."""
    list_a = [(f"chunk-{i}", float(i)) for i in range(20)]
    list_b = [(f"chunk-{i}", float(i)) for i in range(20)]

    results = reciprocal_rank_fusion([list_a, list_b], k=60, top_n=5)
    assert len(results) == 5


def test_rrf_empty_lists() -> None:
    """RRF with all-empty input lists should return an empty result."""
    results = reciprocal_rank_fusion([[], []], k=60, top_n=10)
    assert results == []


def test_rrf_one_empty_list() -> None:
    """RRF should work even when one list is empty."""
    list_a = [("chunk-1", 0.9), ("chunk-2", 0.7)]
    results = reciprocal_rank_fusion([list_a, []], k=60, top_n=5)
    result_ids = [r[0] for r in results]
    assert "chunk-1" in result_ids
    assert "chunk-2" in result_ids


def test_rrf_single_list() -> None:
    """RRF with a single list should preserve rank order."""
    list_a = [("a", 1.0), ("b", 0.5), ("c", 0.1)]
    results = reciprocal_rank_fusion([list_a], k=60, top_n=3)
    # "a" is rank 1 → highest RRF score
    assert results[0][0] == "a"
    assert results[1][0] == "b"
    assert results[2][0] == "c"


# ---------------------------------------------------------------------------
# vector_search unit tests (mocked asyncpg)
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_vector_search_query() -> None:
    """vector_search should call asyncpg.fetch with the right parameters."""
    from raven_worker.retrieval.vector_search import vector_search

    mock_row_1 = {"id": "uuid-1", "score": 0.95}
    mock_row_2 = {"id": "uuid-2", "score": 0.80}

    mock_conn = AsyncMock()
    mock_conn.fetch = AsyncMock(return_value=[mock_row_1, mock_row_2])

    embedding = [0.1, 0.2, 0.3]
    results = await vector_search(mock_conn, embedding, "org-1", ["kb-1"], limit=10)

    assert len(results) == 2
    assert results[0] == ("uuid-1", 0.95)
    assert results[1] == ("uuid-2", 0.80)

    mock_conn.fetch.assert_awaited_once()
    call_args = mock_conn.fetch.call_args
    sql = call_args.args[0]
    assert "embedding" in sql.lower()
    # Verify embedding string representation was passed
    embedding_arg = call_args.args[1]
    assert embedding_arg == "[0.1,0.2,0.3]"
    # org_id and kb_ids are positional args 2 and 3
    assert call_args.args[2] == "org-1"
    assert call_args.args[3] == ["kb-1"]


@pytest.mark.asyncio
async def test_vector_search_empty_result() -> None:
    """vector_search should return an empty list when no rows match."""
    from raven_worker.retrieval.vector_search import vector_search

    mock_conn = AsyncMock()
    mock_conn.fetch = AsyncMock(return_value=[])

    results = await vector_search(mock_conn, [0.1, 0.2], "org-1", ["kb-1"])
    assert results == []


# ---------------------------------------------------------------------------
# bm25_search unit tests (mocked asyncpg)
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_bm25_search_query() -> None:
    """bm25_search should call asyncpg.fetch with the correct SQL parameters."""
    from raven_worker.retrieval.bm25_search import bm25_search

    mock_row = {"id": "uuid-3", "score": 0.42}

    mock_conn = AsyncMock()
    mock_conn.fetch = AsyncMock(return_value=[mock_row])

    results = await bm25_search(mock_conn, "machine learning", "org-2", ["kb-2"], limit=5)

    assert len(results) == 1
    assert results[0] == ("uuid-3", 0.42)

    mock_conn.fetch.assert_awaited_once()
    call_args = mock_conn.fetch.call_args
    sql = call_args.args[0]
    assert "ts_rank_cd" in sql.lower()
    assert "plainto_tsquery" in sql.lower()
    # query text is the first positional arg
    assert call_args.args[1] == "machine learning"
    assert call_args.args[2] == "org-2"
    assert call_args.args[3] == ["kb-2"]
    assert call_args.args[4] == 5


@pytest.mark.asyncio
async def test_bm25_search_empty_result() -> None:
    """bm25_search should return an empty list when no rows match."""
    from raven_worker.retrieval.bm25_search import bm25_search

    mock_conn = AsyncMock()
    mock_conn.fetch = AsyncMock(return_value=[])

    results = await bm25_search(mock_conn, "some query", "org-1", ["kb-1"])
    assert results == []


# ---------------------------------------------------------------------------
# RAGServicer integration tests (fully mocked)
# ---------------------------------------------------------------------------


def _make_rag_request(
    query: str = "What is RAG?",
    org_id: str = "org-abc",
    kb_ids: list[str] | None = None,
    model: str = "gpt-4o-mini",
    provider: str = "openai",
    filters: dict[str, str] | None = None,
) -> ai_worker_pb2.RAGRequest:
    return ai_worker_pb2.RAGRequest(
        query=query,
        org_id=org_id,
        kb_ids=kb_ids or ["kb-1"],
        session_id="sess-1",
        model=model,
        provider=provider,
        filters=filters or {},
    )


def _make_db_row(
    chunk_id: str = "chunk-1",
    content: str = "Some chunk content",
    heading: str = "Section 1",
    document_id: str = "doc-1",
    document_name: str = "My Document",
) -> MagicMock:
    row = MagicMock()
    row.__getitem__ = lambda self, key: {
        "id": chunk_id,
        "content": content,
        "heading": heading,
        "document_id": document_id,
        "document_name": document_name,
    }[key]
    return row


@pytest.mark.asyncio
async def test_rag_servicer_no_chunks(grpc_context) -> None:
    """RAGServicer should yield a 'no relevant info' chunk when search returns nothing."""
    mock_conn = AsyncMock()
    mock_conn.execute = AsyncMock()
    # vector search returns [], bm25 returns []
    mock_conn.fetch = AsyncMock(return_value=[])

    mock_pool = _make_mock_pool(mock_conn)

    mock_provider = AsyncMock()
    mock_provider.embed = AsyncMock(return_value=[0.1, 0.2, 0.3])

    servicer = RAGServicer(pool=mock_pool)

    with patch(
        "raven_worker.services.rag.get_provider_for_request",
        AsyncMock(return_value=mock_provider),
    ):
        chunks = []
        async for chunk in servicer.query(_make_rag_request(), grpc_context):
            chunks.append(chunk)

    assert len(chunks) == 1
    assert chunks[0].is_final is True
    assert "No relevant information found" in chunks[0].text
    grpc_context.abort.assert_not_awaited()


@pytest.mark.asyncio
async def test_rag_servicer_streams_tokens(grpc_context) -> None:
    """RAGServicer should stream tokens and then emit a final chunk."""
    mock_conn = AsyncMock()
    mock_conn.execute = AsyncMock()
    # vector search (1 hit), bm25 search (1 hit), chunk content fetch (1 row)
    mock_conn.fetch = AsyncMock(
        side_effect=[
            [{"id": "chunk-1", "score": 0.9}],  # vector search
            [{"id": "chunk-1", "score": 0.4}],  # bm25 search
            [_make_db_row()],  # fetch chunk content
        ]
    )

    mock_pool = _make_mock_pool(mock_conn)

    mock_provider = AsyncMock()
    mock_provider.embed = AsyncMock(return_value=[0.1] * 1536)

    # Mock OpenAI streaming
    async def _fake_openai_stream():
        for token in ["Hello", " world", "!"]:
            event = MagicMock()
            event.choices = [MagicMock()]
            event.choices[0].delta.content = token
            yield event

    mock_openai_client = MagicMock()
    mock_openai_client.chat.completions.create = AsyncMock(return_value=_fake_openai_stream())

    servicer = RAGServicer(pool=mock_pool)

    _patch_provider = patch(
        "raven_worker.services.rag.get_provider_for_request",
        AsyncMock(return_value=mock_provider),
    )
    _patch_key = patch(
        "raven_worker.services.rag.RAGServicer._get_llm_api_key",
        AsyncMock(return_value="sk-test"),
    )
    _patch_openai = patch(
        "raven_worker.services.rag.AsyncOpenAI",
        return_value=mock_openai_client,
    )
    with _patch_provider, _patch_key, _patch_openai:
        chunks = []
        async for chunk in servicer.query(_make_rag_request(), grpc_context):
            chunks.append(chunk)

    # Should have 3 token chunks + 1 final chunk
    assert len(chunks) == 4
    token_chunks = [c for c in chunks if not c.is_final]
    final_chunks = [c for c in chunks if c.is_final]

    assert len(token_chunks) == 3
    assert token_chunks[0].text == "Hello"
    assert token_chunks[1].text == " world"
    assert token_chunks[2].text == "!"

    assert len(final_chunks) == 1
    assert final_chunks[0].text == ""
    grpc_context.abort.assert_not_awaited()


@pytest.mark.asyncio
async def test_rag_servicer_with_sources(grpc_context) -> None:
    """Final RAGChunk should have sources populated from retrieved chunks."""
    mock_conn = AsyncMock()
    mock_conn.execute = AsyncMock()
    mock_conn.fetch = AsyncMock(
        side_effect=[
            [{"id": "chunk-1", "score": 0.9}],
            [{"id": "chunk-1", "score": 0.4}],
            [
                _make_db_row(
                    chunk_id="chunk-1",
                    content="Detailed content here",
                    heading="Introduction",
                    document_id="doc-42",
                    document_name="Research Paper",
                )
            ],
        ]
    )

    mock_pool = _make_mock_pool(mock_conn)

    mock_provider = AsyncMock()
    mock_provider.embed = AsyncMock(return_value=[0.1] * 1536)

    async def _fake_openai_stream():
        event = MagicMock()
        event.choices = [MagicMock()]
        event.choices[0].delta.content = "The answer is 42."
        yield event

    mock_openai_client = MagicMock()
    mock_openai_client.chat.completions.create = AsyncMock(return_value=_fake_openai_stream())

    servicer = RAGServicer(pool=mock_pool)

    _patch_provider = patch(
        "raven_worker.services.rag.get_provider_for_request",
        AsyncMock(return_value=mock_provider),
    )
    _patch_key = patch(
        "raven_worker.services.rag.RAGServicer._get_llm_api_key",
        AsyncMock(return_value="sk-test"),
    )
    _patch_openai = patch(
        "raven_worker.services.rag.AsyncOpenAI",
        return_value=mock_openai_client,
    )
    with _patch_provider, _patch_key, _patch_openai:
        chunks = []
        async for chunk in servicer.query(_make_rag_request(), grpc_context):
            chunks.append(chunk)

    final = next(c for c in chunks if c.is_final)
    assert len(final.sources) == 1
    src = final.sources[0]
    assert src.document_id == "doc-42"
    assert src.document_name == "Research Paper"
    assert "Detailed content here" in src.chunk_text
    assert src.score > 0.0


@pytest.mark.asyncio
async def test_rag_servicer_anthropic_streams_tokens(grpc_context) -> None:
    """RAGServicer should stream Anthropic tokens correctly."""
    mock_conn = AsyncMock()
    mock_conn.execute = AsyncMock()
    mock_conn.fetch = AsyncMock(
        side_effect=[
            [{"id": "chunk-1", "score": 0.8}],
            [{"id": "chunk-1", "score": 0.3}],
            [_make_db_row()],
        ]
    )

    mock_pool = _make_mock_pool(mock_conn)

    mock_provider = AsyncMock()
    mock_provider.embed = AsyncMock(return_value=[0.1] * 1536)

    # Anthropic streaming context manager
    async def _text_stream():
        for token in ["Claude", " here"]:
            yield token

    mock_stream_ctx = MagicMock()

    @asynccontextmanager
    async def _stream_as_cm(*_args, **_kwargs):
        mock_stream_ctx.text_stream = _text_stream()
        yield mock_stream_ctx

    mock_anthropic_client = MagicMock()
    mock_anthropic_client.messages.stream = _stream_as_cm

    servicer = RAGServicer(pool=mock_pool)

    _patch_provider = patch(
        "raven_worker.services.rag.get_provider_for_request",
        AsyncMock(return_value=mock_provider),
    )
    _patch_key = patch(
        "raven_worker.services.rag.RAGServicer._get_llm_api_key",
        AsyncMock(return_value="sk-ant-test"),
    )
    _patch_anthropic = patch("raven_worker.services.rag.anthropic")
    with _patch_provider, _patch_key, _patch_anthropic as mock_anthropic_module:
        mock_anthropic_module.AsyncAnthropic.return_value = mock_anthropic_client
        chunks = []
        async for chunk in servicer.query(
            _make_rag_request(provider="anthropic", model="claude-3-5-haiku-latest"),
            grpc_context,
        ):
            chunks.append(chunk)

    token_chunks = [c for c in chunks if not c.is_final]
    assert len(token_chunks) == 2
    assert token_chunks[0].text == "Claude"
    assert token_chunks[1].text == " here"
    assert chunks[-1].is_final is True


@pytest.mark.asyncio
async def test_rag_servicer_internal_error_aborts(grpc_context) -> None:
    """An unexpected exception in the pipeline should abort with INTERNAL status."""
    servicer = RAGServicer()

    with patch(
        "raven_worker.services.rag.get_provider_for_request",
        AsyncMock(side_effect=RuntimeError("DB connection refused")),
    ):
        chunks = []
        async for chunk in servicer.query(_make_rag_request(), grpc_context):
            chunks.append(chunk)

    grpc_context.abort.assert_awaited_once()
    args = grpc_context.abort.call_args.args
    assert args[0].value[0] == 13  # grpc.StatusCode.INTERNAL value
    assert "DB connection refused" in args[1]
