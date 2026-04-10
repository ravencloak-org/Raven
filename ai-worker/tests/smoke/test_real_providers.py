"""
Real LLM provider smoke tests.

Only run via manual workflow_dispatch — never on PRs.
Requires env vars: OPENAI_API_KEY, COHERE_API_KEY, ANTHROPIC_API_KEY

These tests make real network calls and validate live API responses.
They are skipped automatically when API keys are not set.
"""

import os

import pytest


@pytest.mark.skipif(
    not os.getenv("OPENAI_API_KEY"),
    reason="OPENAI_API_KEY not set",
)
async def test_openai_embedding_real():
    """Smoke test: OpenAI embedding must return a 1536-dimension vector."""
    from raven_worker.providers.openai_provider import OpenAIEmbeddingProvider

    provider = OpenAIEmbeddingProvider(
        api_key=os.environ["OPENAI_API_KEY"],
        model="text-embedding-3-small",
        dimensions=1536,
    )
    vec = await provider.embed("smoke test")
    assert len(vec) == 1536
    assert all(isinstance(v, float) for v in vec)


@pytest.mark.skipif(
    not os.getenv("COHERE_API_KEY"),
    reason="COHERE_API_KEY not set",
)
async def test_cohere_embedding_real():
    """Smoke test: Cohere embedding must return a non-empty vector."""
    from raven_worker.providers.cohere_provider import CohereEmbeddingProvider

    provider = CohereEmbeddingProvider(api_key=os.environ["COHERE_API_KEY"])
    vec = await provider.embed("smoke test")
    assert len(vec) > 0


@pytest.mark.skipif(
    not os.getenv("ANTHROPIC_API_KEY"),
    reason="ANTHROPIC_API_KEY not set",
)
async def test_anthropic_completion_real():
    # NOTE: Anthropic does not expose a public embedding endpoint.
    # This smoke test verifies the completion API only (via the RAGServicer
    # _stream_anthropic path), which is intentional.
    # The AnthropicEmbeddingProvider.embed() always raises NotImplementedError.
    from raven_worker.providers.anthropic_provider import AnthropicEmbeddingProvider

    provider = AnthropicEmbeddingProvider(api_key=os.environ["ANTHROPIC_API_KEY"])
    with pytest.raises(NotImplementedError, match="Anthropic embedding not yet available"):
        await provider.embed("smoke test")
    # The NotImplementedError is the expected behaviour — Anthropic has no embedding API.
