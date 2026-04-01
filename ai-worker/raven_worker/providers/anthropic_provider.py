"""Anthropic embedding provider stub.

NOTE: Anthropic does not offer a standalone embedding API as of the current
knowledge cutoff.  This stub satisfies the EmbeddingProvider protocol but
raises ``NotImplementedError`` on every call.  It exists so that the provider
registry can return a consistent error instead of an unknown-provider crash
when an org has an Anthropic provider config and requests embeddings.
"""

from __future__ import annotations

import structlog

from raven_worker.providers.base import EmbeddingProvider

logger = structlog.get_logger(__name__)


class AnthropicEmbeddingProvider:
    """Placeholder for future Anthropic embedding support.

    Anthropic does not yet expose a public embedding endpoint.
    All method calls raise ``NotImplementedError``.

    Implements the :class:`~raven_worker.providers.base.EmbeddingProvider` protocol.
    """

    def __init__(
        self,
        api_key: str,  # noqa: ARG002  — kept for protocol parity
        model: str = "anthropic-embed-placeholder",
        dimensions: int = 0,
    ) -> None:
        self._model = model
        self._dimensions = dimensions
        logger.warning(
            "anthropic_embedding_unavailable",
            message="Anthropic does not yet provide a public embedding API.",
        )

    async def embed(self, text: str) -> list[float]:  # noqa: ARG002
        """Not implemented — Anthropic has no embedding API yet.

        Raises:
            NotImplementedError: Always.
        """
        raise NotImplementedError("Anthropic embedding not yet available")

    async def embed_batch(self, texts: list[str]) -> list[list[float]]:  # noqa: ARG002
        """Not implemented — Anthropic has no embedding API yet.

        Raises:
            NotImplementedError: Always.
        """
        raise NotImplementedError("Anthropic embedding not yet available")

    @property
    def dimensions(self) -> int:
        return self._dimensions

    @property
    def model_name(self) -> str:
        return self._model


def _verify_protocol_compliance() -> None:
    """Static assertion that AnthropicEmbeddingProvider satisfies EmbeddingProvider."""
    _: EmbeddingProvider = AnthropicEmbeddingProvider(api_key="dummy")  # noqa: F841
