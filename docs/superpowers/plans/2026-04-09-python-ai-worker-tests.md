# Python AI Worker Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Write missing pytest tests for the Python AI worker to reach 70% line coverage, covering gRPC service RPCs, document parsing/chunking, retrieval pipeline, voice agent lifecycle, EE connectors, and analytics.

**Architecture:** pytest with `asyncio_mode = "auto"`, `AsyncMock` from `unittest.mock`, factory functions returning protobuf objects (existing pattern in `tests/test_rag_service.py`). All LLM provider calls mocked with deterministic stubs. Tests live in `ai-worker/tests/`.

**Tech Stack:** Python 3.12, `pytest>=8.3.0`, `pytest-asyncio>=0.25.0`, `pytest-cov>=6.0.0`, `unittest.mock.AsyncMock`, `grpcio`, `asyncpg` (mocked)

---

## Pre-flight: Audit Existing Tests

- [ ] **Step 1: List existing test files and identify gaps**

```bash
find /Users/jobinlawrance/Project/raven/ai-worker/tests -name '*.py' | sort
```

Note which modules have zero or minimal tests. Do not rewrite passing tests.

- [ ] **Step 2: Run existing tests to establish baseline**

```bash
cd /Users/jobinlawrance/Project/raven/ai-worker && \
  python -m pytest tests/ -v --cov=raven_worker --cov-report=term-missing 2>&1 | tail -30
```

Record current coverage %. Target is 70%.

---

## Task 1: Shared Fixtures (`tests/conftest.py`)

**Files:**
- Modify: `ai-worker/tests/conftest.py`

- [ ] **Step 1: Audit existing conftest.py**

```bash
cat /Users/jobinlawrance/Project/raven/ai-worker/tests/conftest.py
```

Add the following fixtures if they don't exist:

```python
import pytest
from unittest.mock import AsyncMock, MagicMock
import raven_worker.generated.ai_worker_pb2 as pb2


@pytest.fixture
def mock_openai_provider():
    """Deterministic OpenAI provider stub returning fixed embeddings and completions."""
    provider = AsyncMock()
    provider.embed.return_value = [0.1] * 1536
    provider.complete.return_value = "Test completion response"
    provider.stream_complete = AsyncMock(return_value=_async_gen(["Test ", "response"]))
    provider.provider_name = "openai"
    return provider


@pytest.fixture
def mock_cohere_provider():
    provider = AsyncMock()
    provider.embed.return_value = [0.2] * 1024
    provider.complete.return_value = "Cohere response"
    provider.provider_name = "cohere"
    return provider


@pytest.fixture
def mock_anthropic_provider():
    provider = AsyncMock()
    provider.embed.return_value = [0.3] * 1536
    provider.complete.return_value = "Anthropic response"
    provider.provider_name = "anthropic"
    return provider


@pytest.fixture
def mock_db():
    """Async mock database connection."""
    db = AsyncMock()
    db.fetchrow.return_value = None
    db.fetch.return_value = []
    db.execute.return_value = "INSERT 0 1"
    return db


@pytest.fixture
def grpc_context():
    """Fake gRPC service context."""
    ctx = MagicMock()
    ctx.abort = MagicMock()
    ctx.is_active.return_value = True
    return ctx


async def _async_gen(items):
    for item in items:
        yield item


def make_parse_request(
    content: str = "Test document content",
    doc_id: str = "doc-test-1",
    org_id: str = "org-test",
    kb_id: str = "kb-test",
    chunk_size: int = 512,
    chunk_overlap: int = 50,
) -> pb2.ParseRequest:
    return pb2.ParseRequest(
        content=content,
        document_id=doc_id,
        org_id=org_id,
        kb_id=kb_id,
        chunk_size=chunk_size,
        chunk_overlap=chunk_overlap,
    )


def make_rag_request(
    query: str = "What is RAG?",
    org_id: str = "org-test",
    kb_ids: list[str] | None = None,
    model: str = "gpt-4o-mini",
    provider: str = "openai",
) -> pb2.RAGRequest:
    return pb2.RAGRequest(
        query=query,
        org_id=org_id,
        kb_ids=kb_ids or ["kb-test"],
        model=model,
        provider=provider,
    )


def make_embedding_request(
    text: str = "Hello world",
    provider: str = "openai",
    model: str = "text-embedding-3-small",
) -> pb2.EmbeddingRequest:
    return pb2.EmbeddingRequest(text=text, provider=provider, model=model)
```

- [ ] **Step 2: Verify conftest is importable**

```bash
cd /Users/jobinlawrance/Project/raven/ai-worker && python -m pytest tests/ --collect-only 2>&1 | head -20
```

- [ ] **Step 3: Commit**

```bash
git add ai-worker/tests/conftest.py
git commit -m "test(python): add shared fixtures for gRPC, providers, DB"
```

---

## Task 2: gRPC Service Tests

**Files:**
- Modify/Create: `ai-worker/tests/test_grpc_server.py`

- [ ] **Step 1: Write failing ParseAndEmbed tests**

```python
import pytest
from unittest.mock import AsyncMock, patch
from tests.conftest import make_parse_request
import raven_worker.generated.ai_worker_pb2 as pb2


class TestParseAndEmbed:
    async def test_valid_document_returns_chunks(self, grpc_context, mock_openai_provider):
        with patch("raven_worker.server.get_provider", return_value=mock_openai_provider):
            from raven_worker.server import AIWorkerServicer
            servicer = AIWorkerServicer()
            req = make_parse_request(content="This is a test document with enough content to chunk.")
            resp = await servicer.ParseAndEmbed(req, grpc_context)
            assert resp.chunk_count > 0
            assert not grpc_context.abort.called

    async def test_empty_document_calls_abort(self, grpc_context):
        from raven_worker.server import AIWorkerServicer
        servicer = AIWorkerServicer()
        req = make_parse_request(content="")
        await servicer.ParseAndEmbed(req, grpc_context)
        grpc_context.abort.assert_called_once()

    async def test_large_document_chunked_correctly(self, grpc_context, mock_openai_provider):
        # 10000-char document with chunk_size=512 → at least 15 chunks
        with patch("raven_worker.server.get_provider", return_value=mock_openai_provider):
            from raven_worker.server import AIWorkerServicer
            servicer = AIWorkerServicer()
            req = make_parse_request(content="word " * 2000, chunk_size=512, chunk_overlap=50)
            resp = await servicer.ParseAndEmbed(req, grpc_context)
            assert resp.chunk_count >= 15
```

- [ ] **Step 2: Run to verify fails**

```bash
cd /Users/jobinlawrance/Project/raven/ai-worker && \
  python -m pytest tests/test_grpc_server.py::TestParseAndEmbed -v 2>&1 | tail -20
```

- [ ] **Step 3: Write QueryRAG streaming tests**

```python
class TestQueryRAG:
    async def test_query_returns_chunks(self, grpc_context, mock_openai_provider):
        # Mock vector search returning 2 chunks
        mock_chunks = [
            {"content": "RAG stands for Retrieval Augmented Generation", "score": 0.9, "doc_id": "d1"},
            {"content": "It combines retrieval with generation", "score": 0.8, "doc_id": "d2"},
        ]
        with patch("raven_worker.retrieval.search", return_value=mock_chunks), \
             patch("raven_worker.server.get_provider", return_value=mock_openai_provider):
            from raven_worker.server import AIWorkerServicer
            servicer = AIWorkerServicer()
            req = make_rag_request(query="What is RAG?")
            chunks = []
            async for chunk in servicer.QueryRAG(req, grpc_context):
                chunks.append(chunk)
            assert len(chunks) > 0
            assert any(c.text for c in chunks)

    async def test_empty_kb_returns_graceful_empty(self, grpc_context, mock_openai_provider):
        with patch("raven_worker.retrieval.search", return_value=[]), \
             patch("raven_worker.server.get_provider", return_value=mock_openai_provider):
            from raven_worker.server import AIWorkerServicer
            servicer = AIWorkerServicer()
            req = make_rag_request()
            chunks = [c async for c in servicer.QueryRAG(req, grpc_context)]
            # Empty KB: either empty chunks or a "no results" message chunk — no crash
            assert not grpc_context.abort.called

    async def test_sources_attributed_correctly(self, grpc_context, mock_openai_provider):
        mock_chunks = [{"content": "test", "score": 0.95, "doc_id": "doc-123", "source_url": "http://example.com"}]
        with patch("raven_worker.retrieval.search", return_value=mock_chunks), \
             patch("raven_worker.server.get_provider", return_value=mock_openai_provider):
            from raven_worker.server import AIWorkerServicer
            servicer = AIWorkerServicer()
            chunks = [c async for c in servicer.QueryRAG(make_rag_request(), grpc_context)]
            source_chunks = [c for c in chunks if c.HasField("source")]
            assert len(source_chunks) > 0
            assert source_chunks[0].source.document_id == "doc-123"
```

- [ ] **Step 4: Write GetEmbedding tests**

```python
class TestGetEmbedding:
    async def test_text_returns_correct_dimension(self, grpc_context, mock_openai_provider):
        with patch("raven_worker.server.get_provider", return_value=mock_openai_provider):
            from raven_worker.server import AIWorkerServicer
            servicer = AIWorkerServicer()
            req = make_embedding_request(text="Hello world")
            resp = await servicer.GetEmbedding(req, grpc_context)
            assert len(resp.embedding) == 1536
            assert resp.dimensions == 1536

    async def test_empty_string_calls_abort(self, grpc_context):
        from raven_worker.server import AIWorkerServicer
        servicer = AIWorkerServicer()
        req = make_embedding_request(text="")
        await servicer.GetEmbedding(req, grpc_context)
        grpc_context.abort.assert_called_once()
```

- [ ] **Step 5: Run all gRPC service tests**

```bash
cd /Users/jobinlawrance/Project/raven/ai-worker && \
  python -m pytest tests/test_grpc_server.py -v 2>&1 | tail -30
```

- [ ] **Step 6: Commit**

```bash
git add ai-worker/tests/test_grpc_server.py
git commit -m "test(python): gRPC service tests (ParseAndEmbed, QueryRAG, GetEmbedding)"
```

---

## Task 3: Document Processing Tests

**Files:**
- Modify/Create: `ai-worker/tests/processors/test_html_parser.py`
- Modify/Create: `ai-worker/tests/processors/test_text_splitter.py`
- Modify/Create: `ai-worker/tests/processors/test_chunk_metadata.py`

First, audit the actual processor module structure:

```bash
find /Users/jobinlawrance/Project/raven/ai-worker/raven_worker/processors -name '*.py' | sort
```

- [ ] **Step 1: HTML parser tests**

```python
# tests/processors/test_html_parser.py
import pytest
from raven_worker.processors.html_parser import HTMLParser  # adjust import


class TestHTMLParser:
    def test_strips_scripts(self):
        html = "<html><head><script>alert('xss')</script></head><body>Content</body></html>"
        parser = HTMLParser()
        result = parser.parse(html)
        assert "alert" not in result
        assert "Content" in result

    def test_strips_styles(self):
        html = "<html><body><style>.cls { color: red; }</style><p>Text</p></body></html>"
        result = HTMLParser().parse(html)
        assert "color: red" not in result
        assert "Text" in result

    def test_extracts_body_text(self):
        html = "<html><body><h1>Title</h1><p>Paragraph content.</p></body></html>"
        result = HTMLParser().parse(html)
        assert "Title" in result
        assert "Paragraph content." in result

    def test_handles_malformed_html(self):
        # BeautifulSoup4 is lenient — should not raise
        html = "<html><body><p>Unclosed tag<div>More"
        result = HTMLParser().parse(html)
        assert "Unclosed tag" in result
        assert "More" in result

    def test_empty_html_returns_empty_string(self):
        result = HTMLParser().parse("")
        assert result == ""
```

- [ ] **Step 2: Text splitter tests**

```python
# tests/processors/test_text_splitter.py
from raven_worker.processors.splitter import TextSplitter  # adjust import


class TestTextSplitter:
    def test_chunk_size_respected(self):
        text = "word " * 200  # 1000 chars
        splitter = TextSplitter(chunk_size=100, chunk_overlap=10)
        chunks = splitter.split(text)
        for chunk in chunks:
            assert len(chunk) <= 120  # allow small overage for word boundaries

    def test_overlap_present(self):
        text = "alpha beta gamma delta epsilon zeta eta theta iota kappa " * 20
        splitter = TextSplitter(chunk_size=50, chunk_overlap=20)
        chunks = splitter.split(text)
        assert len(chunks) > 1
        # Consecutive chunks should share tokens
        first_words = set(chunks[0].split())
        second_words = set(chunks[1].split())
        assert len(first_words & second_words) > 0  # overlap exists

    def test_no_orphan_tokens(self):
        text = "Complete sentence. Another sentence. Third sentence."
        chunks = TextSplitter(chunk_size=30, chunk_overlap=5).split(text)
        # Every chunk should be non-empty and well-formed
        for chunk in chunks:
            assert chunk.strip() != ""

    def test_single_chunk_when_fits(self):
        text = "Short text"
        chunks = TextSplitter(chunk_size=1000, chunk_overlap=0).split(text)
        assert len(chunks) == 1
        assert chunks[0] == "Short text"
```

- [ ] **Step 3: Chunk metadata propagation tests**

```python
# tests/processors/test_chunk_metadata.py
# make_parse_request is a plain importable function in conftest.py, not a fixture.
from tests.conftest import make_parse_request
from raven_worker.processors.pipeline import process_document  # adjust to actual module path


async def test_chunk_metadata_includes_source_info():
    """Every chunk must carry doc_id, kb_id, chunk_index, and source_url."""
    req = make_parse_request(
        content="First sentence. Second sentence. Third sentence. Fourth sentence.",
        doc_id="doc-meta-test",
        org_id="org-1",
        kb_id="kb-meta-test",
    )
    chunks = await process_document(req, source_url="https://example.com/page")

    assert len(chunks) > 0, "at least one chunk must be produced"
    for i, chunk in enumerate(chunks):
        assert chunk.document_id == "doc-meta-test", f"chunk[{i}].document_id missing"
        assert chunk.kb_id == "kb-meta-test", f"chunk[{i}].kb_id missing"
        assert chunk.source_url == "https://example.com/page", f"chunk[{i}].source_url missing"
        assert chunk.chunk_index == i, f"chunk[{i}].chunk_index must equal {i}"


async def test_chunk_indices_sequential():
    """Indices must be 0, 1, 2, 3... with no gaps or duplicates."""
    req = make_parse_request(content="word " * 300, chunk_size=100, chunk_overlap=10)
    chunks = await process_document(req, source_url="https://example.com")

    indices = [c.chunk_index for c in chunks]
    assert indices == list(range(len(chunks))), f"indices not sequential: {indices}"
```

- [ ] **Step 4: Run processor tests**

```bash
cd /Users/jobinlawrance/Project/raven/ai-worker && \
  python -m pytest tests/processors/ -v 2>&1 | tail -20
```

- [ ] **Step 5: Commit**

```bash
git add ai-worker/tests/processors/
git commit -m "test(python): document processing (HTML parser, text splitter, chunk metadata)"
```

---

## Task 4: Retrieval Pipeline Tests

**Files:**
- Modify/Create: `ai-worker/tests/retrieval/test_embedding.py`
- Modify/Create: `ai-worker/tests/retrieval/test_ranking.py`
- Modify/Create: `ai-worker/tests/retrieval/test_rrf.py`

- [ ] **Step 1: Embedding generation tests**

```python
# tests/retrieval/test_embedding.py
class TestEmbeddingGeneration:
    async def test_openai_returns_correct_dimension(self, mock_openai_provider):
        from raven_worker.retrieval.embedding import generate_embedding
        vec = await generate_embedding("test text", provider=mock_openai_provider)
        assert len(vec) == 1536
        assert all(isinstance(v, float) for v in vec)

    async def test_cohere_returns_correct_dimension(self, mock_cohere_provider):
        from raven_worker.retrieval.embedding import generate_embedding
        vec = await generate_embedding("test text", provider=mock_cohere_provider)
        assert len(vec) == 1024

    async def test_empty_text_raises_value_error(self, mock_openai_provider):
        from raven_worker.retrieval.embedding import generate_embedding
        with pytest.raises(ValueError, match="empty"):
            await generate_embedding("", provider=mock_openai_provider)
```

- [ ] **Step 2: Cosine similarity ranking tests**

```python
# tests/retrieval/test_ranking.py
from raven_worker.retrieval.ranking import cosine_rank  # adjust import


class TestCosineSimilarityRanking:
    def test_higher_similarity_ranks_higher(self):
        query_vec = [1.0, 0.0, 0.0]
        candidates = [
            {"id": "a", "vector": [0.9, 0.1, 0.0]},  # high similarity
            {"id": "b", "vector": [0.1, 0.9, 0.0]},  # low similarity
        ]
        ranked = cosine_rank(query_vec, candidates)
        assert ranked[0]["id"] == "a"

    def test_tied_scores_handled(self):
        query_vec = [1.0, 0.0]
        candidates = [
            {"id": "a", "vector": [1.0, 0.0]},
            {"id": "b", "vector": [1.0, 0.0]},
        ]
        ranked = cosine_rank(query_vec, candidates)
        assert len(ranked) == 2  # both present, no crash
```

- [ ] **Step 3: BM25 scoring tests** (`tests/retrieval/test_bm25.py`)

```python
# tests/retrieval/test_bm25.py
from raven_worker.retrieval.bm25 import BM25Scorer  # adjust import


class TestBM25Scoring:
    def test_term_frequency_increases_score(self):
        """Documents with more occurrences of the query term score higher."""
        scorer = BM25Scorer()
        corpus = [
            "the cat sat on the mat",          # 0 occurrences of "dog"
            "the dog barked at the dog fence",  # 2 occurrences of "dog"
            "a dog walked by",                  # 1 occurrence of "dog"
        ]
        scorer.fit(corpus)
        scores = scorer.score("dog", corpus)
        # doc[1] (2 occurrences) should beat doc[2] (1 occurrence) should beat doc[0] (0)
        assert scores[1] > scores[2] > scores[0]

    def test_idf_downweights_common_terms(self):
        """Very common terms (appear in all docs) get low IDF and low overall score."""
        scorer = BM25Scorer()
        corpus = ["the quick fox", "the lazy dog", "the brown bear"]
        scorer.fit(corpus)
        scores = scorer.score("the", corpus)
        # All docs have "the" — IDF is near 0, all scores should be very low
        for score in scores:
            assert score < 0.5, f"IDF should downweight 'the'; got score {score}"

    def test_zero_occurrences_score_is_zero(self):
        scorer = BM25Scorer()
        corpus = ["apple banana cherry"]
        scorer.fit(corpus)
        scores = scorer.score("mango", corpus)
        assert scores[0] == 0.0
```

- [ ] **Step 4: Reciprocal Rank Fusion tests**

```python
# tests/retrieval/test_rrf.py
from raven_worker.retrieval.fusion import reciprocal_rank_fusion


class TestRRFFusion:
    def test_combined_score_higher_than_individual(self):
        vector_results = [{"id": "doc-1", "rank": 1}, {"id": "doc-2", "rank": 2}]
        bm25_results   = [{"id": "doc-1", "rank": 1}, {"id": "doc-3", "rank": 2}]
        fused = reciprocal_rank_fusion(vector_results, bm25_results)
        # doc-1 appears in both lists — should rank highest
        assert fused[0]["id"] == "doc-1"

    def test_empty_one_side_returns_other(self):
        vector_results = [{"id": "doc-a", "rank": 1}]
        fused = reciprocal_rank_fusion(vector_results, [])
        assert len(fused) == 1
        assert fused[0]["id"] == "doc-a"

    def test_both_empty_returns_empty(self):
        assert reciprocal_rank_fusion([], []) == []
```

- [ ] **Step 4: Run retrieval tests**

```bash
cd /Users/jobinlawrance/Project/raven/ai-worker && \
  python -m pytest tests/retrieval/ -v 2>&1 | tail -20
```

- [ ] **Step 5: Commit**

```bash
git add ai-worker/tests/retrieval/
git commit -m "test(python): retrieval pipeline (embedding, cosine ranking, RRF fusion)"
```

---

## Task 5: Voice Agent Tests

**Files:**
- Modify/Create: `ai-worker/tests/test_voice_agent.py`

- [ ] **Step 1: Write voice agent lifecycle tests**

```python
# tests/test_voice_agent.py
import pytest
from unittest.mock import AsyncMock, patch, MagicMock


class TestVoiceAgentSTT:
    async def test_audio_input_produces_transcript(self):
        mock_stt = AsyncMock(return_value="Hello, how can I help you?")
        with patch("raven_worker.agent.stt_provider", mock_stt):
            from raven_worker.agent import process_audio
            transcript = await process_audio(audio_bytes=b"\x00" * 1024)
            assert transcript == "Hello, how can I help you?"
            mock_stt.assert_called_once()


class TestVoiceAgentLLM:
    async def test_transcript_produces_response(self, mock_openai_provider):
        with patch("raven_worker.agent.get_provider", return_value=mock_openai_provider):
            from raven_worker.agent import generate_response
            response = await generate_response(
                transcript="What is the capital of France?",
                session_id="sess-1",
                kb_id="kb-1",
            )
            assert response == "Test completion response"


class TestVoiceAgentTTS:
    async def test_response_text_produces_audio(self):
        mock_tts = AsyncMock(return_value=b"\xff\xfb" * 100)  # fake MP3 bytes
        with patch("raven_worker.agent.tts_provider", mock_tts):
            from raven_worker.agent import synthesize_speech
            audio = await synthesize_speech("Hello there")
            assert len(audio) > 0


class TestVoiceSessionLifecycle:
    async def test_join_active_disconnect_cleanup(self):
        from raven_worker.agent import VoiceSession
        session = VoiceSession(session_id="sess-lifecycle", kb_id="kb-1")

        # Join
        await session.join()
        assert session.status == "active"

        # Disconnect
        await session.disconnect()
        assert session.status == "disconnected"

        # Cleanup — no resources leaked
        assert session.audio_buffer is None
```

Adjust import paths to match actual `raven_worker/agent.py` structure.

- [ ] **Step 2: Run voice tests**

```bash
cd /Users/jobinlawrance/Project/raven/ai-worker && \
  python -m pytest tests/test_voice_agent.py -v 2>&1 | tail -20
```

- [ ] **Step 3: Commit**

```bash
git add ai-worker/tests/test_voice_agent.py
git commit -m "test(python): voice agent STT/LLM/TTS pipeline and session lifecycle"
```

---

## Task 6: EE Connector & Analytics Tests

**Files:**
- Modify/Create: `ai-worker/tests/ee/test_connectors.py`
- Modify/Create: `ai-worker/tests/ee/test_analytics.py`

- [ ] **Step 1: Audit EE Python modules**

```bash
find /Users/jobinlawrance/Project/raven/ai-worker/raven_worker/ee -name '*.py' | sort
```

- [ ] **Step 2: Write Airbyte connector tests** (`tests/ee/test_connectors.py`)

```python
class TestAirbyteConnector:
    async def test_sync_trigger_creates_job(self):
        mock_http = AsyncMock(return_value={"job_id": "job-123", "status": "running"})
        with patch("raven_worker.ee.connectors.airbyte.http_post", mock_http):
            from raven_worker.ee.connectors.airbyte import trigger_sync
            result = await trigger_sync(connection_id="conn-abc", org_id="org-1")
            assert result["job_id"] == "job-123"

    async def test_status_polling_succeeded_updates_kb(self, mock_db):
        mock_http = AsyncMock(return_value={"status": "SUCCEEDED"})
        with patch("raven_worker.ee.connectors.airbyte.http_get", mock_http):
            from raven_worker.ee.connectors.airbyte import poll_sync_status
            status = await poll_sync_status(job_id="job-123", org_id="org-1", db=mock_db)
            assert status == "SUCCEEDED"
            mock_db.execute.assert_called()  # KB updated

    async def test_sync_failure_surfaces_error(self):
        mock_http = AsyncMock(return_value={"status": "FAILED", "error": "Connection refused"})
        with patch("raven_worker.ee.connectors.airbyte.http_get", mock_http):
            from raven_worker.ee.connectors.airbyte import poll_sync_status
            with pytest.raises(Exception, match="FAILED"):
                await poll_sync_status(job_id="job-fail", org_id="org-1", db=AsyncMock())
```

- [ ] **Step 3: Write analytics event tests** (`tests/ee/test_analytics.py`)

```python
class TestAnalyticsEvents:
    async def test_posthog_event_correct_shape(self):
        captured = []
        mock_sink = AsyncMock(side_effect=lambda event: captured.append(event))
        with patch("raven_worker.ee.analytics.posthog_sink", mock_sink):
            from raven_worker.ee.analytics import track_event
            await track_event(
                distinct_id="user-123",
                event="document_uploaded",
                properties={"doc_id": "doc-1", "kb_id": "kb-1"},
            )
        assert len(captured) == 1
        assert captured[0]["event"] == "document_uploaded"
        assert captured[0]["distinct_id"] == "user-123"
        assert "doc_id" in captured[0]["properties"]

    async def test_clickhouse_event_write(self, mock_db):
        from raven_worker.ee.analytics import write_clickhouse_event
        await write_clickhouse_event(
            event_type="chat_message",
            org_id="org-1",
            payload={"session_id": "s1"},
            db=mock_db,
        )
        mock_db.execute.assert_called_once()
        call_args = str(mock_db.execute.call_args)
        assert "chat_message" in call_args
```

- [ ] **Step 4: Run EE tests**

```bash
cd /Users/jobinlawrance/Project/raven/ai-worker && \
  python -m pytest tests/ee/ -v 2>&1 | tail -20
```

- [ ] **Step 5: Commit**

```bash
git add ai-worker/tests/ee/
git commit -m "test(python): EE connector (Airbyte) and analytics (PostHog, ClickHouse)"
```

---

## Task 7: Coverage Gate & Smoke Test CI Job

- [ ] **Step 1: Run full suite with coverage**

```bash
cd /Users/jobinlawrance/Project/raven/ai-worker && \
  python -m pytest tests/ --cov=raven_worker --cov-report=term-missing --cov-fail-under=70 2>&1 | tail -20
```

Expected: all pass, coverage ≥ 70%

- [ ] **Step 2: If below 70%, check gaps**

```bash
python -m pytest tests/ --cov=raven_worker --cov-report=term-missing 2>&1 | grep -E "^raven_worker" | sort -k4 -n | head -20
```

Write tests for lowest-coverage modules.

- [ ] **Step 3: Add manual smoke test CI job config**

In `.github/workflows/python.yml`, add a new job (triggered only by `workflow_dispatch`):

```yaml
  smoke-tests:
    if: github.event_name == 'workflow_dispatch'
    runs-on: ubuntu-latest
    environment: smoke
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-python@v5
        with:
          python-version: '3.12'
      - run: pip install -e ".[dev]"
        working-directory: ai-worker
      - name: Run LLM provider smoke tests
        working-directory: ai-worker
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
          COHERE_API_KEY: ${{ secrets.COHERE_API_KEY }}
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
        run: python -m pytest tests/smoke/ -v --tb=short
```

Create `ai-worker/tests/smoke/test_real_providers.py`:

```python
"""
Real LLM provider smoke tests.
Only run via manual workflow_dispatch — never on PRs.
Requires env vars: OPENAI_API_KEY, COHERE_API_KEY, ANTHROPIC_API_KEY
"""
import os
import pytest


@pytest.mark.skipif(not os.getenv("OPENAI_API_KEY"), reason="OPENAI_API_KEY not set")
async def test_openai_embedding_real():
    from raven_worker.providers.openai import OpenAIProvider
    provider = OpenAIProvider(api_key=os.environ["OPENAI_API_KEY"])
    vec = await provider.embed("smoke test")
    assert len(vec) == 1536


@pytest.mark.skipif(not os.getenv("COHERE_API_KEY"), reason="COHERE_API_KEY not set")
async def test_cohere_embedding_real():
    from raven_worker.providers.cohere import CohereProvider
    provider = CohereProvider(api_key=os.environ["COHERE_API_KEY"])
    vec = await provider.embed("smoke test")
    assert len(vec) > 0


@pytest.mark.skipif(not os.getenv("ANTHROPIC_API_KEY"), reason="ANTHROPIC_API_KEY not set")
async def test_anthropic_completion_real():
    # NOTE: Anthropic does not expose a public embedding endpoint.
    # This smoke test verifies the completion API only, which is intentional.
    from raven_worker.providers.anthropic import AnthropicProvider
    provider = AnthropicProvider(api_key=os.environ["ANTHROPIC_API_KEY"])
    result = await provider.complete("Say 'pong'")
    assert "pong" in result.lower()
```

- [ ] **Step 4: Final commit**

```bash
git add ai-worker/tests/ .github/workflows/python.yml
git commit -m "test(python): 70% coverage gate + manual smoke test CI job for real providers"
```
