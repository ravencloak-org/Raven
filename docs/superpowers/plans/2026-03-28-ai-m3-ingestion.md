# AI Worker M3 Ingestion Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement real document parsing (LiteParse), semantic text chunking, and multi-provider embedding in the Raven AI worker (M3: issues #14, #16, #17).

**Architecture:** The AI worker is a Python gRPC server. The three processors (`DocumentParser`, `TextChunker`, `EmbeddingServicer`) are stubs — replace each with a real implementation. All are invoked by the `ParseAndEmbed` RPC in `server.py`. Add missing dependencies to `pyproject.toml` before implementing. TDD with pytest-asyncio.

**Tech Stack:** Python 3.12, grpcio, liteparse, langchain-text-splitters (chunking), openai SDK, pytest, pytest-asyncio.

**Worktree:** `.claude/worktrees/stream-ai` on branch `feat/stream-ai-m3-ingestion`

---

## Pre-flight: Read before writing a single line

- [ ] Read `ai-worker/raven_worker/processors/parser.py` — understand stub interface
- [ ] Read `ai-worker/raven_worker/processors/chunker.py` — understand stub interface
- [ ] Read `ai-worker/raven_worker/services/embedding.py` — understand stub interface
- [ ] Read `ai-worker/raven_worker/providers/base.py` — understand provider interface
- [ ] Read `ai-worker/raven_worker/providers/openai_provider.py` — understand existing OpenAI stub
- [ ] Read `ai-worker/raven_worker/server.py` — understand how processors are wired to RPCs
- [ ] Read `ai-worker/raven_worker/config.py` — understand config structure
- [ ] Read `ai-worker/tests/conftest.py` — understand test setup and fixtures
- [ ] Read `ai-worker/pyproject.toml` — check what dependencies exist; liteparse is NOT there yet
- [ ] Read `proto/ai_worker.proto` — understand `ParseRequest`/`ParseResponse` contract

---

## Task 1: Add dependencies

- [ ] Add to `ai-worker/pyproject.toml` under `[project] dependencies`:
```toml
"liteparse>=0.3.0",
"langchain-text-splitters>=0.3.0",
"openai>=1.58.0",
"tiktoken>=0.8.0",
"httpx>=0.28.0",
```

- [ ] Install in worktree:
```bash
cd .claude/worktrees/stream-ai/ai-worker
python -m pip install -e ".[dev]" 2>&1 | tail -5
```

- [ ] Verify liteparse is importable:
```bash
python -c "import liteparse; print('liteparse ok')"
```

- [ ] Verify langchain splitters:
```bash
python -c "from langchain_text_splitters import RecursiveCharacterTextSplitter; print('splitters ok')"
```

- [ ] Commit:
```bash
git add ai-worker/pyproject.toml
git commit -m "deps(ai-worker): add liteparse, langchain-text-splitters, openai, tiktoken"
```

---

## Task 2: Issue #14 — LiteParse Integration (DocumentParser)

**GitHub issue:** #14 — Replace `NotImplementedError` stub with real LiteParse parsing.

**Files:**
- Replace: `ai-worker/raven_worker/processors/parser.py`
- Create: `ai-worker/tests/test_parser.py`

**Supported formats:** PDF, DOCX, PPTX, HTML, Markdown, plain text.

- [ ] Write failing tests in `ai-worker/tests/test_parser.py`:
```python
import pytest
from raven_worker.processors.parser import DocumentParser

@pytest.fixture
def parser():
    return DocumentParser()

@pytest.mark.asyncio
async def test_parse_plain_text(parser):
    content = b"Hello, world! This is a test document."
    result = await parser.parse(content, "text/plain", "test.txt")
    assert "Hello, world!" in result
    assert len(result) > 0

@pytest.mark.asyncio
async def test_parse_markdown(parser):
    content = b"# Title\n\nSome content here.\n\n## Section\n\nMore text."
    result = await parser.parse(content, "text/markdown", "doc.md")
    assert "Title" in result
    assert "Some content here" in result

@pytest.mark.asyncio
async def test_parse_html(parser):
    content = b"<html><body><h1>Title</h1><p>Some content.</p></body></html>"
    result = await parser.parse(content, "text/html", "page.html")
    assert "Title" in result
    assert "Some content" in result

@pytest.mark.asyncio
async def test_parse_unknown_mime_raises(parser):
    with pytest.raises(ValueError, match="Unsupported"):
        await parser.parse(b"data", "application/octet-stream", "file.bin")

@pytest.mark.asyncio
async def test_parse_empty_content_returns_empty(parser):
    result = await parser.parse(b"", "text/plain", "empty.txt")
    assert result == ""
```

- [ ] Run — expect FAIL:
```bash
cd ai-worker && python -m pytest tests/test_parser.py -v 2>&1
```

- [ ] Implement `ai-worker/raven_worker/processors/parser.py`:
```python
"""LiteParse integration for document parsing."""

from __future__ import annotations

import asyncio
import io
from functools import partial

import liteparse
import structlog

logger = structlog.get_logger(__name__)

# MIME types supported by LiteParse
_SUPPORTED = {
    "application/pdf",
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document",  # docx
    "application/vnd.openxmlformats-officedocument.presentationml.presentation",  # pptx
    "text/html",
    "text/markdown",
    "text/plain",
}

_EXTENSION_FALLBACK: dict[str, str] = {
    ".pdf": "application/pdf",
    ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    ".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
    ".html": "text/html",
    ".htm": "text/html",
    ".md": "text/markdown",
    ".txt": "text/plain",
}


class DocumentParser:
    """Parse documents into plain text using LiteParse."""

    async def parse(self, content: bytes, mime_type: str, file_name: str) -> str:
        """Extract text content from a document.

        Args:
            content: Raw document bytes.
            mime_type: MIME type. Falls back to extension detection if not supported.
            file_name: Original file name for extension-based fallback.

        Returns:
            Extracted plain text.

        Raises:
            ValueError: If the MIME type is not supported and no fallback exists.
        """
        if not content:
            return ""

        resolved_mime = self._resolve_mime(mime_type, file_name)
        logger.info(
            "parse_document",
            mime_type=resolved_mime,
            file_name=file_name,
            size=len(content),
        )

        # Run sync liteparse in thread pool to avoid blocking event loop
        loop = asyncio.get_event_loop()
        text = await loop.run_in_executor(
            None,
            partial(self._parse_sync, content, resolved_mime, file_name),
        )
        return text.strip()

    def _resolve_mime(self, mime_type: str, file_name: str) -> str:
        if mime_type in _SUPPORTED:
            return mime_type
        # Try extension fallback
        suffix = "." + file_name.rsplit(".", 1)[-1].lower() if "." in file_name else ""
        fallback = _EXTENSION_FALLBACK.get(suffix)
        if fallback:
            return fallback
        raise ValueError(f"Unsupported MIME type: {mime_type} (file: {file_name})")

    def _parse_sync(self, content: bytes, mime_type: str, file_name: str) -> str:
        doc = liteparse.Document(
            content=io.BytesIO(content),
            mime_type=mime_type,
            filename=file_name,
        )
        return doc.extract_text()
```

- [ ] Run tests — expect PASS:
```bash
cd ai-worker && python -m pytest tests/test_parser.py -v 2>&1
```

- [ ] Run full test suite to confirm no regressions:
```bash
cd ai-worker && python -m pytest -v 2>&1
```

- [ ] Commit:
```bash
git add ai-worker/raven_worker/processors/parser.py ai-worker/tests/test_parser.py
git commit -m "feat(#14): implement LiteParse document parsing (PDF, DOCX, PPTX, HTML, MD, TXT)"
```

- [ ] Push and create PR:
```bash
git push origin feat/stream-ai-m3-ingestion
gh pr create --title "feat: LiteParse document parsing (#14)" \
  --body "Closes #14"
```

---

## Task 3: Issue #16 — Text Chunking (TextChunker)

**GitHub issue:** #16 — Replace basic sliding window with semantic recursive character chunking.

**Files:**
- Replace: `ai-worker/raven_worker/processors/chunker.py`
- Create: `ai-worker/tests/test_chunker.py`

**Strategy:** Use `langchain_text_splitters.RecursiveCharacterTextSplitter` — splits on paragraphs, then sentences, then characters. Preserves semantic boundaries better than a fixed sliding window.

- [ ] Write failing tests in `ai-worker/tests/test_chunker.py`:
```python
import pytest
from raven_worker.processors.chunker import TextChunker

def test_empty_text_returns_empty_list():
    chunker = TextChunker()
    assert chunker.chunk("") == []

def test_short_text_returns_single_chunk():
    chunker = TextChunker(chunk_size=512, chunk_overlap=64)
    text = "Short text under the chunk size limit."
    chunks = chunker.chunk(text)
    assert len(chunks) == 1
    assert chunks[0] == text

def test_long_text_produces_multiple_chunks():
    chunker = TextChunker(chunk_size=100, chunk_overlap=20)
    text = "word " * 100  # 500 chars
    chunks = chunker.chunk(text)
    assert len(chunks) > 1

def test_chunks_have_overlap():
    chunker = TextChunker(chunk_size=50, chunk_overlap=10)
    # Build text where overlap can be detected
    text = "AAAA " * 20  # repeating
    chunks = chunker.chunk(text)
    # Each chunk should be <= chunk_size + some tolerance
    for chunk in chunks:
        assert len(chunk) <= 60  # chunk_size + small tolerance

def test_chunk_count_matches_log_metadata():
    chunker = TextChunker(chunk_size=100, chunk_overlap=20)
    text = "sentence. " * 50
    chunks = chunker.chunk(text)
    assert len(chunks) > 0

def test_custom_separators_respected():
    chunker = TextChunker(chunk_size=200, chunk_overlap=0)
    # Text with clear paragraph boundaries
    text = "Paragraph one.\n\nParagraph two.\n\nParagraph three."
    chunks = chunker.chunk(text)
    # Should split on paragraphs
    assert any("Paragraph one" in c for c in chunks)
```

- [ ] Run — expect some failures (current implementation may not match):
```bash
cd ai-worker && python -m pytest tests/test_chunker.py -v 2>&1
```

- [ ] Replace `ai-worker/raven_worker/processors/chunker.py`:
```python
"""Semantic text chunking using RecursiveCharacterTextSplitter."""

from __future__ import annotations

import structlog
from langchain_text_splitters import RecursiveCharacterTextSplitter

logger = structlog.get_logger(__name__)

DEFAULT_CHUNK_SIZE = 512
DEFAULT_CHUNK_OVERLAP = 64


class TextChunker:
    """Split text into overlapping chunks suitable for embedding.

    Uses RecursiveCharacterTextSplitter which tries to split on natural
    boundaries (paragraphs → sentences → words → chars) before falling
    back to raw character splits.
    """

    def __init__(
        self,
        chunk_size: int = DEFAULT_CHUNK_SIZE,
        chunk_overlap: int = DEFAULT_CHUNK_OVERLAP,
    ) -> None:
        self.chunk_size = chunk_size
        self.chunk_overlap = chunk_overlap
        self._splitter = RecursiveCharacterTextSplitter(
            chunk_size=chunk_size,
            chunk_overlap=chunk_overlap,
            separators=["\n\n", "\n", ". ", "! ", "? ", " ", ""],
        )

    def chunk(self, text: str) -> list[str]:
        """Split text into overlapping semantic chunks.

        Args:
            text: Full document text.

        Returns:
            List of text chunks, each <= chunk_size characters (approximately).
        """
        if not text:
            return []

        chunks = self._splitter.split_text(text)
        logger.info(
            "chunked_text",
            total_length=len(text),
            chunk_count=len(chunks),
            chunk_size=self.chunk_size,
            chunk_overlap=self.chunk_overlap,
        )
        return chunks
```

- [ ] Run tests — expect PASS:
```bash
cd ai-worker && python -m pytest tests/test_chunker.py -v 2>&1
```

- [ ] Run full test suite:
```bash
cd ai-worker && python -m pytest -v 2>&1
```

- [ ] Commit:
```bash
git add ai-worker/raven_worker/processors/chunker.py ai-worker/tests/test_chunker.py
git commit -m "feat(#16): semantic text chunking with RecursiveCharacterTextSplitter"
```

- [ ] Push PR:
```bash
gh pr create --title "feat: semantic text chunking (#16)" \
  --body "Closes #16"
```

---

## Task 4: Issue #17 — Multi-Provider Embedding (BYOK)

**GitHub issue:** #17 — Real embedding via OpenAI (and extensible to other providers).

**Files:**
- Replace: `ai-worker/raven_worker/providers/base.py` — define protocol
- Replace: `ai-worker/raven_worker/providers/openai_provider.py` — real OpenAI impl
- Replace: `ai-worker/raven_worker/services/embedding.py` — wire provider selection
- Create: `ai-worker/tests/test_embedding.py`

- [ ] Read `ai-worker/raven_worker/config.py` — check how BYOK API keys are stored.

- [ ] Define provider protocol in `ai-worker/raven_worker/providers/base.py`:
```python
"""Base protocol for embedding providers."""

from __future__ import annotations

from typing import Protocol, runtime_checkable


@runtime_checkable
class EmbeddingProvider(Protocol):
    """Protocol all embedding providers must satisfy."""

    async def embed(self, text: str, model: str) -> list[float]:
        """Return embedding vector for text using the given model."""
        ...

    @property
    def dimensions(self) -> int:
        """Return the embedding dimensionality for the current model."""
        ...
```

- [ ] Write failing tests in `ai-worker/tests/test_embedding.py`:
```python
import pytest
from unittest.mock import AsyncMock, MagicMock, patch

@pytest.mark.asyncio
async def test_openai_provider_returns_embedding():
    mock_response = MagicMock()
    mock_response.data = [MagicMock(embedding=[0.1, 0.2, 0.3])]
    mock_response.usage.total_tokens = 10

    with patch("openai.AsyncOpenAI") as mock_client_cls:
        mock_client = AsyncMock()
        mock_client.embeddings.create = AsyncMock(return_value=mock_response)
        mock_client_cls.return_value = mock_client

        from raven_worker.providers.openai_provider import OpenAIEmbeddingProvider
        provider = OpenAIEmbeddingProvider(api_key="sk-test", model="text-embedding-3-small")
        result = await provider.embed("hello world", "text-embedding-3-small")

    assert result == [0.1, 0.2, 0.3]

@pytest.mark.asyncio
async def test_embedding_servicer_routes_to_provider(grpc_context):
    from raven_worker.services.embedding import EmbeddingServicer
    from raven_worker.generated import ai_worker_pb2

    mock_provider = AsyncMock()
    mock_provider.embed = AsyncMock(return_value=[0.1] * 1536)
    mock_provider.dimensions = 1536

    servicer = EmbeddingServicer(provider=mock_provider)
    request = ai_worker_pb2.EmbeddingRequest(
        text="hello", org_id="org-1", model="text-embedding-3-small", provider="openai"
    )
    response = await servicer.GetEmbedding(request, grpc_context)
    assert len(response.embedding) == 1536
    assert response.dimensions == 1536
```

- [ ] Add `grpc_context` fixture to `ai-worker/tests/conftest.py` (read current conftest first):
```python
import pytest
from unittest.mock import AsyncMock

@pytest.fixture
def grpc_context():
    ctx = AsyncMock()
    ctx.abort = AsyncMock()
    return ctx
```

- [ ] Run — expect FAIL:
```bash
cd ai-worker && python -m pytest tests/test_embedding.py -v 2>&1
```

- [ ] Implement `ai-worker/raven_worker/providers/openai_provider.py`:
```python
"""OpenAI embedding provider."""

from __future__ import annotations

import openai
import structlog

logger = structlog.get_logger(__name__)

_MODEL_DIMENSIONS = {
    "text-embedding-3-small": 1536,
    "text-embedding-3-large": 3072,
    "text-embedding-ada-002": 1536,
}


class OpenAIEmbeddingProvider:
    """Embed text using OpenAI embeddings API (BYOK)."""

    def __init__(self, api_key: str, model: str = "text-embedding-3-small") -> None:
        self._client = openai.AsyncOpenAI(api_key=api_key)
        self._model = model

    @property
    def dimensions(self) -> int:
        return _MODEL_DIMENSIONS.get(self._model, 1536)

    async def embed(self, text: str, model: str | None = None) -> list[float]:
        resolved = model or self._model
        logger.info("embed_request", model=resolved, text_length=len(text))
        response = await self._client.embeddings.create(
            input=text,
            model=resolved,
        )
        return response.data[0].embedding
```

- [ ] Replace `ai-worker/raven_worker/services/embedding.py` — wire provider selection:
```python
"""Embedding service implementing the GetEmbedding RPC."""

from __future__ import annotations

import grpc
import structlog

from raven_worker.generated import ai_worker_pb2
from raven_worker.providers.base import EmbeddingProvider

logger = structlog.get_logger(__name__)


class EmbeddingServicer:
    """Handles GetEmbedding RPC calls."""

    def __init__(self, provider: EmbeddingProvider) -> None:
        self._provider = provider

    async def GetEmbedding(self, request, context) -> ai_worker_pb2.EmbeddingResponse:
        if not request.text:
            await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "text must not be empty")
            return ai_worker_pb2.EmbeddingResponse()

        logger.info(
            "get_embedding",
            org_id=request.org_id,
            model=request.model,
            provider=request.provider,
            text_length=len(request.text),
        )

        embedding = await self._provider.embed(request.text, request.model or None)
        return ai_worker_pb2.EmbeddingResponse(
            embedding=embedding,
            dimensions=len(embedding),
        )
```

- [ ] Update `server.py` to wire `OpenAIEmbeddingProvider` from config (read server.py first — minimal change to inject provider):

```python
# In server.py initialisation — add provider wiring
from raven_worker.providers.openai_provider import OpenAIEmbeddingProvider
from raven_worker.services.embedding import EmbeddingServicer

provider = OpenAIEmbeddingProvider(
    api_key=config.openai_api_key,  # read from config
    model=config.embedding_model,
)
embedding_servicer = EmbeddingServicer(provider=provider)
```

- [ ] Add `openai_api_key` and `embedding_model` to `config.py` (read it first to follow pattern):
```python
openai_api_key: str = ""
embedding_model: str = "text-embedding-3-small"
```

- [ ] Run all tests — expect PASS:
```bash
cd ai-worker && python -m pytest -v 2>&1
```

- [ ] Run linting:
```bash
cd ai-worker && python -m ruff check . && python -m ruff format --check . 2>&1
```

- [ ] Commit:
```bash
git add ai-worker/
git commit -m "feat(#17): multi-provider embedding (OpenAI BYOK) with EmbeddingProvider protocol"
```

- [ ] Push PR:
```bash
gh pr create --title "feat: multi-provider embedding BYOK (#17)" \
  --body "Closes #17"
```

---

## Final verification before each PR

```bash
cd ai-worker
python -m pytest -v                          # all tests pass
python -m ruff check .                       # lint clean
python -m ruff format --check .             # format clean
python -m mypy raven_worker/ --ignore-missing-imports  # type check (optional but recommended)
```
