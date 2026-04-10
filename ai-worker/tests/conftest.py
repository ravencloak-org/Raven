"""Shared pytest fixtures for the Raven AI Worker test suite."""

import sys
from contextlib import asynccontextmanager
from types import ModuleType
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from raven_worker.config import Settings
from raven_worker.generated import ai_worker_pb2  # noqa: E402

# ---------------------------------------------------------------------------
# Inject lightweight stub modules when heavy deps (asyncpg, anthropic, openai,
# cohere, redis) are not installed in the current Python environment.
# This allows all tests to be collected and run locally (e.g. Python 3.14)
# even when those packages are only available in CI (Python 3.12).
# ---------------------------------------------------------------------------


def _make_stub(name: str, **attrs) -> ModuleType:
    mod = ModuleType(name)
    for k, v in attrs.items():
        setattr(mod, k, v)
    return mod


def _inject_missing_stubs() -> None:
    """Inject stub modules into sys.modules for packages not yet installed."""
    stubs: dict[str, ModuleType] = {}

    if "asyncpg" not in sys.modules:
        pg = _make_stub(
            "asyncpg",
            connect=AsyncMock(),
            Connection=object,
            Pool=object,
            exceptions=_make_stub("asyncpg.exceptions"),
        )
        stubs["asyncpg"] = pg

    if "anthropic" not in sys.modules:
        ant = _make_stub(
            "anthropic",
            AsyncAnthropic=MagicMock(),
            APIError=Exception,
        )
        stubs["anthropic"] = ant

    if "openai" not in sys.modules:
        oai = _make_stub(
            "openai",
            AsyncOpenAI=MagicMock(),
            RateLimitError=Exception,
            APIError=Exception,
        )
        stubs["openai"] = oai

    if "cohere" not in sys.modules:
        coh = _make_stub(
            "cohere",
            AsyncClientV2=MagicMock(),
        )
        stubs["cohere"] = coh

    if "redis" not in sys.modules:
        redis_asyncio = _make_stub("redis.asyncio", Redis=MagicMock(), from_url=MagicMock())
        redis_mod = _make_stub("redis", asyncio=redis_asyncio)
        stubs["redis"] = redis_mod
        stubs["redis.asyncio"] = redis_asyncio

    for name, mod in stubs.items():
        sys.modules[name] = mod


_inject_missing_stubs()


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture
def settings() -> Settings:
    """Return a default Settings instance for testing."""
    return Settings()


@pytest.fixture
def grpc_context():
    """Return a mock gRPC context for servicer tests."""
    ctx = AsyncMock()
    ctx.abort = AsyncMock()
    ctx.is_active = MagicMock(return_value=True)
    return ctx


@pytest.fixture
def mock_openai_provider():
    """Deterministic OpenAI provider stub returning fixed embeddings and completions."""
    provider = AsyncMock()
    provider.embed = AsyncMock(return_value=[0.1] * 1536)
    provider.embed_batch = AsyncMock(return_value=[[0.1] * 1536])
    provider.complete = AsyncMock(return_value="Test completion response")
    provider.dimensions = 1536
    provider.model_name = "text-embedding-3-small"
    provider.provider_name = "openai"
    return provider


@pytest.fixture
def mock_cohere_provider():
    """Deterministic Cohere provider stub returning fixed embeddings."""
    provider = AsyncMock()
    provider.embed = AsyncMock(return_value=[0.2] * 1024)
    provider.embed_batch = AsyncMock(return_value=[[0.2] * 1024])
    provider.complete = AsyncMock(return_value="Cohere response")
    provider.dimensions = 1024
    provider.model_name = "embed-english-v3.0"
    provider.provider_name = "cohere"
    return provider


@pytest.fixture
def mock_anthropic_provider():
    """Deterministic Anthropic provider stub."""
    provider = AsyncMock()
    provider.embed = AsyncMock(return_value=[0.3] * 1536)
    provider.embed_batch = AsyncMock(return_value=[[0.3] * 1536])
    provider.complete = AsyncMock(return_value="Anthropic response")
    provider.dimensions = 1536
    provider.model_name = "claude-3-haiku-20240307"
    provider.provider_name = "anthropic"
    return provider


@pytest.fixture
def mock_db():
    """Async mock database connection."""
    db = AsyncMock()
    db.fetchrow = AsyncMock(return_value=None)
    db.fetch = AsyncMock(return_value=[])
    db.execute = AsyncMock(return_value="INSERT 0 1")

    @asynccontextmanager
    async def _acquire():
        yield db

    pool = MagicMock()
    pool.acquire = _acquire
    db._pool = pool
    return db


@pytest.fixture(autouse=True)
def _disable_rag_cache():
    """Disable the RAG response cache in all tests by default.

    Tests that specifically exercise cache behaviour should override
    ``_check_cache`` / ``_store_cache`` in their own patches.

    Skips gracefully if raven_worker.services.rag cannot be imported.
    """
    try:
        import raven_worker.services.rag  # noqa: F401
    except ImportError:
        yield
        return

    with (
        patch(
            "raven_worker.services.rag.RAGServicer._check_cache",
            new_callable=AsyncMock,
            return_value=None,
        ),
        patch(
            "raven_worker.services.rag.RAGServicer._store_cache",
            new_callable=AsyncMock,
        ),
    ):
        yield


# ---------------------------------------------------------------------------
# Plain factory functions (importable directly, NOT pytest fixtures)
# ---------------------------------------------------------------------------


def make_parse_request(
    content: bytes = b"Test document content",
    doc_id: str = "doc-test-1",
    org_id: str = "org-test",
    kb_id: str = "kb-test",
    mime_type: str = "text/plain",
    file_name: str = "test.txt",
) -> ai_worker_pb2.ParseRequest:
    """Build a ParseRequest protobuf for use in tests."""
    return ai_worker_pb2.ParseRequest(
        content=content,
        document_id=doc_id,
        org_id=org_id,
        kb_id=kb_id,
        mime_type=mime_type,
        file_name=file_name,
    )


def make_rag_request(
    query: str = "What is RAG?",
    org_id: str = "org-test",
    kb_ids: list[str] | None = None,
    model: str = "gpt-4o-mini",
    provider: str = "openai",
) -> ai_worker_pb2.RAGRequest:
    """Build a RAGRequest protobuf for use in tests."""
    return ai_worker_pb2.RAGRequest(
        query=query,
        org_id=org_id,
        kb_ids=kb_ids or ["kb-test"],
        model=model,
        provider=provider,
    )


def make_embedding_request(
    text: str = "Hello world",
    org_id: str = "org-test",
    provider: str = "openai",
    model: str = "text-embedding-3-small",
) -> ai_worker_pb2.EmbeddingRequest:
    """Build an EmbeddingRequest protobuf for use in tests."""
    return ai_worker_pb2.EmbeddingRequest(
        text=text,
        org_id=org_id,
        provider=provider,
        model=model,
    )
