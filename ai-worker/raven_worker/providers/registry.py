"""Provider registry: look up BYOK keys from the DB and return a ready provider.

Providers are cached in a module-level dict to avoid re-creating async clients
on every RPC call.  The cache key is ``(org_id, provider_name, model)``.
"""

from __future__ import annotations

import asyncpg
import structlog

from raven_worker.config import settings
from raven_worker.crypto import decrypt_api_key
from raven_worker.providers.base import EmbeddingProvider

logger = structlog.get_logger(__name__)

# Module-level provider cache: (org_id, provider_name, model) -> EmbeddingProvider
_provider_cache: dict[tuple[str, str, str], EmbeddingProvider] = {}

# Supported provider names (must match the ``llm_provider`` enum in Postgres)
_SUPPORTED_PROVIDERS = {"openai", "cohere", "anthropic"}


async def get_provider_for_request(
    org_id: str,
    provider_name: str,
    model: str,
) -> EmbeddingProvider:
    """Return an :class:`EmbeddingProvider` instance for the given request parameters.

    The provider configuration (including the encrypted API key) is loaded from
    the ``llm_provider_configs`` table.  Results are cached so that repeated
    requests for the same ``(org_id, provider, model)`` combination reuse the
    same client object.

    Args:
        org_id: UUID string of the requesting organisation.
        provider_name: Provider slug, e.g. ``"openai"``, ``"cohere"``.
        model: Embedding model name, e.g. ``"text-embedding-3-small"``.

    Returns:
        A fully configured :class:`EmbeddingProvider` instance.

    Raises:
        ValueError: If the provider is not supported or no active config is found.
    """
    provider_lower = provider_name.lower()
    if provider_lower not in _SUPPORTED_PROVIDERS:
        raise ValueError(
            f"Unsupported embedding provider: '{provider_name}'. "
            f"Supported providers: {sorted(_SUPPORTED_PROVIDERS)}"
        )

    cache_key = (org_id, provider_lower, model)
    if cache_key in _provider_cache:
        logger.debug(
            "provider_cache_hit",
            org_id=org_id,
            provider=provider_lower,
            model=model,
        )
        return _provider_cache[cache_key]

    logger.info(
        "provider_cache_miss_loading_from_db",
        org_id=org_id,
        provider=provider_lower,
        model=model,
    )

    conn = await asyncpg.connect(settings.database_url)
    try:
        # Set the RLS GUC so row-level security policies apply
        await conn.execute(
            "SELECT set_config('app.current_org_id', $1, false)",
            org_id,
        )

        row = await conn.fetchrow(
            """
            SELECT api_key_encrypted, api_key_iv, base_url, config
            FROM llm_provider_configs
            WHERE org_id = $1
              AND provider = $2
              AND status = 'active'
            ORDER BY is_default DESC
            LIMIT 1
            """,
            org_id,
            provider_lower,
        )
    finally:
        await conn.close()

    if row is None:
        raise ValueError(f"No active '{provider_lower}' provider config found for org '{org_id}'")

    api_key = decrypt_api_key(
        encrypted=bytes(row["api_key_encrypted"]),
        iv=bytes(row["api_key_iv"]),
        key_b64=settings.encryption_key,
    )
    base_url: str | None = row["base_url"]

    provider = _build_provider(provider_lower, api_key, model, base_url)
    _provider_cache[cache_key] = provider

    logger.info(
        "provider_created",
        org_id=org_id,
        provider=provider_lower,
        model=model,
    )
    return provider


def _build_provider(
    provider_name: str,
    api_key: str,
    model: str,
    base_url: str | None,
) -> EmbeddingProvider:
    """Instantiate the correct provider class.

    Args:
        provider_name: Lowercase provider slug.
        api_key: Decrypted BYOK API key.
        model: Embedding model name.
        base_url: Optional custom base URL (used for OpenAI-compatible proxies).

    Returns:
        A concrete :class:`EmbeddingProvider` instance.

    Raises:
        ValueError: If ``provider_name`` is not recognised.
    """
    if provider_name == "openai":
        from raven_worker.providers.openai_provider import OpenAIEmbeddingProvider

        return OpenAIEmbeddingProvider(api_key=api_key, model=model, base_url=base_url)

    if provider_name == "cohere":
        from raven_worker.providers.cohere_provider import CohereEmbeddingProvider

        return CohereEmbeddingProvider(api_key=api_key, model=model)

    if provider_name == "anthropic":
        from raven_worker.providers.anthropic_provider import AnthropicEmbeddingProvider

        return AnthropicEmbeddingProvider(api_key=api_key, model=model)

    raise ValueError(f"Unsupported provider: '{provider_name}'")


def clear_cache() -> None:
    """Clear the in-process provider cache.

    Useful in tests to isolate state between test cases.
    """
    _provider_cache.clear()
