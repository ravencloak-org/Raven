"""Tests for embedding generation via provider stubs.

Tests use the mock provider fixtures from conftest.py to verify
the embedding interface contract without making real API calls.
"""

from __future__ import annotations

import pytest


class TestEmbeddingGeneration:
    async def test_openai_returns_correct_dimension(self, mock_openai_provider):
        """OpenAI provider stub must return 1536-dimension embedding."""
        vec = await mock_openai_provider.embed("test text")
        assert len(vec) == 1536
        assert all(isinstance(v, float) for v in vec)

    async def test_cohere_returns_correct_dimension(self, mock_cohere_provider):
        """Cohere provider stub must return 1024-dimension embedding."""
        vec = await mock_cohere_provider.embed("test text")
        assert len(vec) == 1024

    async def test_anthropic_embed_raises_not_implemented(self):
        """Anthropic does not have a public embedding endpoint.

        NOTE: Anthropic does not expose a public embedding API.
        The AnthropicEmbeddingProvider.embed() raises NotImplementedError
        by design — this is intentional and expected.
        """
        from raven_worker.providers.anthropic_provider import AnthropicEmbeddingProvider

        provider = AnthropicEmbeddingProvider(api_key="sk-ant-test")
        with pytest.raises(NotImplementedError, match="Anthropic embedding not yet available"):
            await provider.embed("hello")

    async def test_openai_embed_batch_returns_list_of_vectors(self, mock_openai_provider):
        """embed_batch must return a list of vectors, one per input text."""
        texts = ["text one", "text two", "text three"]
        mock_openai_provider.embed_batch.return_value = [[0.1] * 1536] * len(texts)
        results = await mock_openai_provider.embed_batch(texts)
        assert len(results) == len(texts)
        for vec in results:
            assert len(vec) == 1536

    async def test_cohere_embed_batch_returns_list_of_vectors(self, mock_cohere_provider):
        """Cohere embed_batch must return a list of 1024-dimension vectors."""
        texts = ["a", "b"]
        mock_cohere_provider.embed_batch.return_value = [[0.2] * 1024] * len(texts)
        results = await mock_cohere_provider.embed_batch(texts)
        assert len(results) == len(texts)
        for vec in results:
            assert len(vec) == 1024

    def test_openai_provider_dimensions_property(self, mock_openai_provider):
        """Provider dimensions property must return 1536 for OpenAI."""
        assert mock_openai_provider.dimensions == 1536

    def test_cohere_provider_dimensions_property(self, mock_cohere_provider):
        """Provider dimensions property must return 1024 for Cohere."""
        assert mock_cohere_provider.dimensions == 1024

    async def test_empty_text_handling_openai(self, mock_openai_provider):
        """Provider.embed called with empty string should still return a vector from stub."""
        # The real provider would raise; stub just returns the mocked value
        vec = await mock_openai_provider.embed("")
        assert len(vec) == 1536

    async def test_real_anthropic_provider_embed_raises(self):
        """Real AnthropicEmbeddingProvider must raise NotImplementedError for embed_batch too."""
        from raven_worker.providers.anthropic_provider import AnthropicEmbeddingProvider

        provider = AnthropicEmbeddingProvider(api_key="sk-ant-test")
        with pytest.raises(NotImplementedError, match="Anthropic embedding not yet available"):
            await provider.embed_batch(["text1", "text2"])
