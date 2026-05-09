"""Tests for the Ollama embedding provider used in Raven Local."""

from __future__ import annotations

import pytest
import respx
from httpx import Response

from raven_worker.providers.ollama_provider import (
    ModelNotPulledError,
    OllamaEmbeddingProvider,
)


@pytest.fixture
def provider() -> OllamaEmbeddingProvider:
    return OllamaEmbeddingProvider(
        base_url="http://ollama:11434",
        model="nomic-embed-text",
    )


@pytest.mark.asyncio
@respx.mock
async def test_embed_returns_vector(provider: OllamaEmbeddingProvider) -> None:
    route = respx.post("http://ollama:11434/api/embeddings").mock(
        return_value=Response(200, json={"embedding": [0.1, 0.2, 0.3]})
    )
    vec = await provider.embed("hello world")
    assert vec == [0.1, 0.2, 0.3]
    assert route.called
    body = route.calls.last.request.read().decode()
    assert '"model":"nomic-embed-text"' in body
    assert '"prompt":"hello world"' in body


@pytest.mark.asyncio
@respx.mock
async def test_embed_raises_typed_error_when_model_missing(
    provider: OllamaEmbeddingProvider,
) -> None:
    """Ollama returns 404 with a 'model not found' body when the model
    hasn't been pulled. The provider raises ModelNotPulledError so callers
    can offer a 'pull this model' affordance instead of a generic 5xx."""
    respx.post("http://ollama:11434/api/embeddings").mock(
        return_value=Response(
            404,
            json={"error": "model 'nomic-embed-text' not found, try pulling it first"},
        )
    )
    with pytest.raises(ModelNotPulledError) as exc:
        await provider.embed("x")
    assert exc.value.model == "nomic-embed-text"


@pytest.mark.asyncio
@respx.mock
async def test_embed_raises_on_other_http_error(
    provider: OllamaEmbeddingProvider,
) -> None:
    respx.post("http://ollama:11434/api/embeddings").mock(
        return_value=Response(503, text="overloaded")
    )
    with pytest.raises(RuntimeError, match="ollama embed failed"):
        await provider.embed("x")


def test_provider_advertises_known_dimension(
    provider: OllamaEmbeddingProvider,
) -> None:
    assert provider.dimensions == 768
    assert provider.model_name == "nomic-embed-text"


def test_provider_strips_trailing_slash_in_base_url() -> None:
    p = OllamaEmbeddingProvider(base_url="http://ollama:11434/")
    # _base_url is private but observable via the request URL in respx tests.
    assert p._base_url == "http://ollama:11434"


def test_provider_uses_default_base_url_when_omitted() -> None:
    p = OllamaEmbeddingProvider()
    assert p._base_url == "http://ollama:11434"
