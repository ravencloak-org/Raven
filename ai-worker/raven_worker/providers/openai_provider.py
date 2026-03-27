"""OpenAI embedding provider stub."""

import structlog

from raven_worker.providers.base import EmbeddingProvider

logger = structlog.get_logger(__name__)

# Default model and dimensions for OpenAI text-embedding-3-small
_DEFAULT_MODEL = "text-embedding-3-small"
_DEFAULT_DIMENSIONS = 1536


class OpenAIEmbeddingProvider:
    """Generate embeddings using the OpenAI API.

    This is a stub implementation. To make it functional, install the
    ``openai`` package and provide an API key via environment variable.

    Implements the :class:`~raven_worker.providers.base.EmbeddingProvider` protocol.
    """

    def __init__(
        self,
        model: str = _DEFAULT_MODEL,
        dimensions: int = _DEFAULT_DIMENSIONS,
    ) -> None:
        self._model = model
        self._dimensions = dimensions

    async def embed(self, text: str) -> list[float]:
        """Generate an embedding for the given text via OpenAI API.

        Args:
            text: Input text to embed.

        Returns:
            Embedding vector as a list of floats.

        Raises:
            NotImplementedError: Always, until the OpenAI client is wired up.
        """
        logger.info("openai_embed", model=self._model, text_length=len(text))
        raise NotImplementedError(
            "OpenAI embedding provider is not yet wired up. "
            "Install the openai package and configure OPENAI_API_KEY."
        )

    @property
    def dimensions(self) -> int:
        return self._dimensions

    @property
    def model_name(self) -> str:
        return self._model


def _verify_protocol_compliance() -> None:
    """Static assertion that OpenAIEmbeddingProvider satisfies EmbeddingProvider."""
    _: EmbeddingProvider = OpenAIEmbeddingProvider()  # noqa: F841
