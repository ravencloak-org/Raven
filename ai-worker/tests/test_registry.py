"""Tests for the provider registry (BYOK key lookup)."""

from __future__ import annotations

import base64
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from raven_worker.providers.registry import _build_provider, clear_cache, get_provider_for_request


@pytest.fixture(autouse=True)
def clear_provider_cache() -> None:
    """Ensure the module-level cache is empty before every test."""
    clear_cache()


# ---------------------------------------------------------------------------
# _build_provider unit tests (no DB)
# ---------------------------------------------------------------------------


def test_build_provider_openai() -> None:
    """_build_provider should return an OpenAIEmbeddingProvider for 'openai'."""
    from raven_worker.providers.openai_provider import OpenAIEmbeddingProvider

    with patch("raven_worker.providers.openai_provider.AsyncOpenAI"):
        provider = _build_provider("openai", "sk-test", "text-embedding-3-small", None)
    assert isinstance(provider, OpenAIEmbeddingProvider)


def test_build_provider_cohere() -> None:
    """_build_provider should return a CohereEmbeddingProvider for 'cohere'."""
    from raven_worker.providers.cohere_provider import CohereEmbeddingProvider

    with patch("raven_worker.providers.cohere_provider.cohere.AsyncClientV2"):
        provider = _build_provider("cohere", "co-test", "embed-english-v3.0", None)
    assert isinstance(provider, CohereEmbeddingProvider)


def test_build_provider_anthropic() -> None:
    """_build_provider should return an AnthropicEmbeddingProvider for 'anthropic'."""
    from raven_worker.providers.anthropic_provider import AnthropicEmbeddingProvider

    provider = _build_provider("anthropic", "sk-ant-test", "anthropic-embed", None)
    assert isinstance(provider, AnthropicEmbeddingProvider)


def test_build_provider_unknown_raises() -> None:
    """_build_provider should raise ValueError for an unsupported provider slug."""
    with pytest.raises(ValueError, match="Unsupported provider"):
        _build_provider("unknown_llm", "key", "model", None)


# ---------------------------------------------------------------------------
# get_provider_for_request — unsupported provider
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_provider_registry_unknown_provider() -> None:
    """get_provider_for_request raises ValueError for an unknown provider name."""
    with pytest.raises(ValueError, match="Unsupported embedding provider"):
        await get_provider_for_request("org-1", "unknownprovider", "some-model")


# ---------------------------------------------------------------------------
# get_provider_for_request — DB lookup and caching
# ---------------------------------------------------------------------------


def _make_fake_row(api_key_encrypted: bytes, api_key_iv: bytes) -> MagicMock:
    row = MagicMock()
    row.__getitem__ = lambda self, key: {
        "api_key_encrypted": api_key_encrypted,
        "api_key_iv": api_key_iv,
        "base_url": None,
        "config": {},
    }[key]
    return row


@pytest.mark.asyncio
async def test_get_provider_for_request_returns_openai_provider() -> None:
    """get_provider_for_request should return an OpenAIEmbeddingProvider when provider='openai'."""
    import os

    from cryptography.hazmat.primitives.ciphers.aead import AESGCM

    raw_key = os.urandom(32)
    key_b64 = base64.b64encode(raw_key).decode()
    iv = os.urandom(12)
    api_key_plain = b"sk-real-key"
    ciphertext = AESGCM(raw_key).encrypt(iv, api_key_plain, None)  # positional: nonce, data, aad

    fake_row = _make_fake_row(ciphertext, iv)

    mock_conn = AsyncMock()
    mock_conn.fetchrow = AsyncMock(return_value=fake_row)
    mock_conn.execute = AsyncMock()
    mock_conn.close = AsyncMock()

    with (
        patch("raven_worker.providers.registry.asyncpg.connect", AsyncMock(return_value=mock_conn)),
        patch("raven_worker.providers.registry.settings") as mock_settings,
        patch("raven_worker.providers.openai_provider.AsyncOpenAI"),
    ):
        mock_settings.database_url = "postgresql://localhost/raven"
        mock_settings.encryption_key = key_b64

        from raven_worker.providers.openai_provider import OpenAIEmbeddingProvider

        provider = await get_provider_for_request("org-abc", "openai", "text-embedding-3-small")

    assert isinstance(provider, OpenAIEmbeddingProvider)
    # GUC for RLS must have been set
    mock_conn.execute.assert_awaited()


@pytest.mark.asyncio
async def test_get_provider_for_request_caches_provider() -> None:
    """get_provider_for_request should return the same object on subsequent calls."""
    import os

    from cryptography.hazmat.primitives.ciphers.aead import AESGCM

    raw_key = os.urandom(32)
    key_b64 = base64.b64encode(raw_key).decode()
    iv = os.urandom(12)
    api_key_plain = b"sk-cached"
    ciphertext = AESGCM(raw_key).encrypt(iv, api_key_plain, None)  # positional: nonce, data, aad

    fake_row = _make_fake_row(ciphertext, iv)

    mock_conn = AsyncMock()
    mock_conn.fetchrow = AsyncMock(return_value=fake_row)
    mock_conn.execute = AsyncMock()
    mock_conn.close = AsyncMock()

    connect_mock = AsyncMock(return_value=mock_conn)

    with (
        patch("raven_worker.providers.registry.asyncpg.connect", connect_mock),
        patch("raven_worker.providers.registry.settings") as mock_settings,
        patch("raven_worker.providers.openai_provider.AsyncOpenAI"),
    ):
        mock_settings.database_url = "postgresql://localhost/raven"
        mock_settings.encryption_key = key_b64

        p1 = await get_provider_for_request("org-abc", "openai", "text-embedding-3-small")
        p2 = await get_provider_for_request("org-abc", "openai", "text-embedding-3-small")

    # DB should only have been hit once
    assert connect_mock.call_count == 1
    assert p1 is p2


@pytest.mark.asyncio
async def test_get_provider_for_request_no_config_raises() -> None:
    """get_provider_for_request raises ValueError when no active config row found."""
    mock_conn = AsyncMock()
    mock_conn.fetchrow = AsyncMock(return_value=None)  # no row
    mock_conn.execute = AsyncMock()
    mock_conn.close = AsyncMock()

    with (
        patch("raven_worker.providers.registry.asyncpg.connect", AsyncMock(return_value=mock_conn)),
        patch("raven_worker.providers.registry.settings") as mock_settings,
    ):
        mock_settings.database_url = "postgresql://localhost/raven"
        mock_settings.encryption_key = base64.b64encode(b"a" * 32).decode()

        with pytest.raises(ValueError, match="No active 'openai' provider config"):
            await get_provider_for_request("org-xyz", "openai", "text-embedding-3-small")
