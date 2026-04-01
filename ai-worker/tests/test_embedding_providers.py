"""Tests for the embedding provider implementations."""

from __future__ import annotations

import base64
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from raven_worker.crypto import decrypt_api_key
from raven_worker.providers.anthropic_provider import AnthropicEmbeddingProvider
from raven_worker.providers.cohere_provider import CohereEmbeddingProvider
from raven_worker.providers.openai_provider import OpenAIEmbeddingProvider

# ---------------------------------------------------------------------------
# crypto tests
# ---------------------------------------------------------------------------


def test_decrypt_api_key() -> None:
    """Encrypt then decrypt a known string and verify round-trip correctness."""
    import os

    from cryptography.hazmat.primitives.ciphers.aead import AESGCM

    raw_key = os.urandom(32)
    key_b64 = base64.b64encode(raw_key).decode()
    iv = os.urandom(12)
    plaintext = b"sk-test-secret-api-key"

    aesgcm = AESGCM(raw_key)
    ciphertext_with_tag = aesgcm.encrypt(iv, plaintext, None)  # positional: nonce, data, aad

    result = decrypt_api_key(encrypted=ciphertext_with_tag, iv=iv, key_b64=key_b64)
    assert result == "sk-test-secret-api-key"


def test_decrypt_api_key_wrong_key_raises() -> None:
    """Decryption with the wrong key should raise an InvalidTag error."""
    import os

    from cryptography.exceptions import InvalidTag
    from cryptography.hazmat.primitives.ciphers.aead import AESGCM

    raw_key = os.urandom(32)
    wrong_key = base64.b64encode(os.urandom(32)).decode()
    iv = os.urandom(12)

    aesgcm = AESGCM(raw_key)
    ciphertext_with_tag = aesgcm.encrypt(iv, b"secret", None)  # positional: nonce, data, aad

    with pytest.raises(InvalidTag):
        decrypt_api_key(encrypted=ciphertext_with_tag, iv=iv, key_b64=wrong_key)


def test_decrypt_api_key_bad_key_length_raises() -> None:
    """A base64-encoded key that is not 32 bytes should raise ValueError."""
    short_key = base64.b64encode(b"tooshort").decode()
    with pytest.raises(ValueError, match="32 bytes"):
        decrypt_api_key(encrypted=b"\x00" * 32, iv=b"\x00" * 12, key_b64=short_key)


# ---------------------------------------------------------------------------
# OpenAI provider tests
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_openai_provider_embed() -> None:
    """embed() should call AsyncOpenAI.embeddings.create and return the vector."""
    fake_embedding = [0.1, 0.2, 0.3]

    mock_create = AsyncMock()
    mock_data_item = MagicMock()
    mock_data_item.embedding = fake_embedding
    mock_data_item.index = 0
    mock_create.return_value = MagicMock(data=[mock_data_item])

    with patch("raven_worker.providers.openai_provider.AsyncOpenAI") as mock_client_cls:
        mock_client_cls.return_value.embeddings.create = mock_create
        provider = OpenAIEmbeddingProvider(
            api_key="sk-test", model="text-embedding-3-small", dimensions=3
        )
        result = await provider.embed("hello world")

    assert result == fake_embedding
    mock_create.assert_awaited_once()


@pytest.mark.asyncio
async def test_openai_provider_embed_batch() -> None:
    """embed_batch() should batch-call the OpenAI API and return ordered vectors."""
    vectors = [[0.1, 0.2], [0.3, 0.4], [0.5, 0.6]]

    mock_create = AsyncMock()
    mock_data = [MagicMock(embedding=vectors[i], index=i) for i in range(len(vectors))]
    mock_create.return_value = MagicMock(data=mock_data)

    with patch("raven_worker.providers.openai_provider.AsyncOpenAI") as mock_client_cls:
        mock_client_cls.return_value.embeddings.create = mock_create
        provider = OpenAIEmbeddingProvider(
            api_key="sk-test", model="text-embedding-3-small", dimensions=2
        )
        results = await provider.embed_batch(["text1", "text2", "text3"])

    assert len(results) == 3
    assert results[0] == vectors[0]
    assert results[1] == vectors[1]
    assert results[2] == vectors[2]
    mock_create.assert_awaited_once()
    # Verify batch input was passed as a list
    call_kwargs = mock_create.call_args.kwargs
    assert isinstance(call_kwargs.get("input"), list)


def test_openai_provider_dimensions() -> None:
    """dimensions property should return the configured value."""
    with patch("raven_worker.providers.openai_provider.AsyncOpenAI"):
        provider = OpenAIEmbeddingProvider(api_key="sk-test", dimensions=768)
    assert provider.dimensions == 768


def test_openai_provider_model_name() -> None:
    """model_name property should return the configured model string."""
    with patch("raven_worker.providers.openai_provider.AsyncOpenAI"):
        provider = OpenAIEmbeddingProvider(api_key="sk-test", model="text-embedding-ada-002")
    assert provider.model_name == "text-embedding-ada-002"


# ---------------------------------------------------------------------------
# Cohere provider tests
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_cohere_provider_embed() -> None:
    """embed() should call AsyncClientV2.embed and return the first float vector."""
    fake_embedding = [0.7, 0.8, 0.9]

    mock_embed = AsyncMock()
    mock_embeddings_obj = MagicMock()
    mock_embeddings_obj.float_ = [fake_embedding]
    mock_embed.return_value = MagicMock(embeddings=mock_embeddings_obj)

    with patch("raven_worker.providers.cohere_provider.cohere.AsyncClientV2") as mock_cls:
        mock_cls.return_value.embed = mock_embed
        provider = CohereEmbeddingProvider(api_key="co-test", dimensions=3)
        result = await provider.embed("hello cohere")

    assert result == fake_embedding
    mock_embed.assert_awaited_once()


@pytest.mark.asyncio
async def test_cohere_provider_embed_batch() -> None:
    """embed_batch() should call AsyncClientV2.embed with all texts and return all vectors."""
    vectors = [[0.1, 0.2], [0.3, 0.4]]

    mock_embed = AsyncMock()
    mock_embeddings_obj = MagicMock()
    mock_embeddings_obj.float_ = vectors
    mock_embed.return_value = MagicMock(embeddings=mock_embeddings_obj)

    with patch("raven_worker.providers.cohere_provider.cohere.AsyncClientV2") as mock_cls:
        mock_cls.return_value.embed = mock_embed
        provider = CohereEmbeddingProvider(api_key="co-test", dimensions=2)
        results = await provider.embed_batch(["text1", "text2"])

    assert len(results) == 2
    assert results[0] == vectors[0]
    assert results[1] == vectors[1]
    call_kwargs = mock_embed.call_args.kwargs
    assert call_kwargs.get("texts") == ["text1", "text2"]


def test_cohere_provider_dimensions() -> None:
    """dimensions property returns configured value."""
    with patch("raven_worker.providers.cohere_provider.cohere.AsyncClientV2"):
        provider = CohereEmbeddingProvider(api_key="co-test", dimensions=1024)
    assert provider.dimensions == 1024


def test_cohere_provider_model_name() -> None:
    """model_name property returns configured model string."""
    with patch("raven_worker.providers.cohere_provider.cohere.AsyncClientV2"):
        provider = CohereEmbeddingProvider(api_key="co-test", model="embed-multilingual-v3.0")
    assert provider.model_name == "embed-multilingual-v3.0"


# ---------------------------------------------------------------------------
# Anthropic provider tests
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_anthropic_provider_embed_raises() -> None:
    """Anthropic embed() must raise NotImplementedError."""
    provider = AnthropicEmbeddingProvider(api_key="sk-ant-test")
    with pytest.raises(NotImplementedError, match="Anthropic embedding not yet available"):
        await provider.embed("hello anthropic")


@pytest.mark.asyncio
async def test_anthropic_provider_embed_batch_raises() -> None:
    """Anthropic embed_batch() must raise NotImplementedError."""
    provider = AnthropicEmbeddingProvider(api_key="sk-ant-test")
    with pytest.raises(NotImplementedError, match="Anthropic embedding not yet available"):
        await provider.embed_batch(["text1", "text2"])
