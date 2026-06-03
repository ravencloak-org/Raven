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
_SUPPORTED_PROVIDERS = {"openai", "cohere", "anthropic", "ollama"}

# Providers that don't require an API key — the local Ollama sidecar in
# Raven AI has no auth surface. The registry skips ``decrypt_api_key``
# when the provider is in this set so an empty encrypted blob is fine.
_KEYLESS_PROVIDERS = {"ollama"}


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

    if provider_lower in _KEYLESS_PROVIDERS:
        api_key = ""
    else:
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


# Embedding-model defaults per provider. The `model` argument the registry
# receives is the *chat* model selected for completion (qwen2.5:7b, gpt-4o,
# claude-3-5-sonnet, etc.) — wrong shape for an embedding call. Each
# embedding API needs its own dedicated model. When the caller doesn't pin
# one explicitly, fall back here so callers don't have to know each
# vendor's embedding-model id.
_DEFAULT_EMBEDDING_MODELS = {
    "openai": "text-embedding-3-small",
    "cohere": "embed-english-v3.0",
    "anthropic": "anthropic-embed-placeholder",  # Anthropic has no native embedding API yet
    "ollama": "nomic-embed-text",
}


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
        model: Embedding model name. Empty / chat-model values are replaced
            with the provider-appropriate embedding default — see
            ``_DEFAULT_EMBEDDING_MODELS``.
        base_url: Optional custom base URL (used for OpenAI-compatible proxies).

    Returns:
        A concrete :class:`EmbeddingProvider` instance.

    Raises:
        ValueError: If ``provider_name`` is not recognised.
    """
    # The model the caller hands us is the chat model picked by the user
    # for completion (e.g. qwen2.5:7b, gpt-4o). Embedding APIs need a
    # separate model — Ollama rejects an empty string with HTTP 400
    # `{"error":"model is required"}`, OpenAI/Cohere quietly succeed on
    # text-embedding-3-small/embed-english-v3.0 but produce wrong-dim
    # vectors. Substitute the embedding default here unless the caller
    # has explicitly pinned a known embedding model.
    if not model or _looks_like_chat_model(provider_name, model):
        model = _DEFAULT_EMBEDDING_MODELS.get(provider_name, model)

    if provider_name == "openai":
        from raven_worker.providers.openai_provider import OpenAIEmbeddingProvider

        return OpenAIEmbeddingProvider(api_key=api_key, model=model, base_url=base_url)

    if provider_name == "cohere":
        from raven_worker.providers.cohere_provider import CohereEmbeddingProvider

        return CohereEmbeddingProvider(api_key=api_key, model=model)

    if provider_name == "anthropic":
        from raven_worker.providers.anthropic_provider import AnthropicEmbeddingProvider

        return AnthropicEmbeddingProvider(api_key=api_key, model=model)

    if provider_name == "ollama":
        from raven_worker.providers.ollama_provider import OllamaEmbeddingProvider

        # Ollama doesn't take an api_key; it talks to a local sidecar.
        return OllamaEmbeddingProvider(model=model, base_url=base_url)

    raise ValueError(f"Unsupported provider: '{provider_name}'")


# Per-provider known chat-model prefixes — anything matching these is
# almost certainly a chat model, not an embedding model. Used to catch
# the "caller forwarded the chat model into the embedding code path"
# bug class without false-positiving on legitimate embedding model
# names like 'text-embedding-3-small' or 'nomic-embed-text'.
_CHAT_MODEL_PREFIXES = {
    "openai": ("gpt-", "o1-", "o3-", "chatgpt-"),
    "anthropic": ("claude-",),
    "ollama": (
        "llama",
        "qwen",
        "mistral",
        "mixtral",
        "phi",
        "gemma",
        "deepseek",
        "codellama",
    ),
    # Cohere chat models follow "command-*"; "chat-" reserved for any
    # future chat-named variants. Embedding models are "embed-*" so
    # they never trip these prefixes.
    "cohere": ("command-", "chat-"),
}


def _looks_like_chat_model(provider_name: str, model: str) -> bool:
    """True when `model` matches a known chat-model naming pattern.

    The signal isn't perfect, but the false-positive cost is low (we just
    override with the embedding default — same behaviour as model="") and
    the false-negative cost would be a noisy 400 from the embedding API.
    """
    prefixes = _CHAT_MODEL_PREFIXES.get(provider_name, ())
    return any(model.startswith(p) for p in prefixes)


def clear_cache() -> None:
    """Clear the in-process provider cache.

    Useful in tests to isolate state between test cases.
    """
    _provider_cache.clear()
