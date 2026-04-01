"""OpenAI embedding provider (BYOK)."""

from __future__ import annotations

import structlog
from openai import AsyncOpenAI
from tenacity import retry, retry_if_exception, stop_after_attempt, wait_exponential

from raven_worker.providers.base import EmbeddingProvider

logger = structlog.get_logger(__name__)

# Default model and dimensions for OpenAI text-embedding-3-small
_DEFAULT_MODEL = "text-embedding-3-small"
_DEFAULT_DIMENSIONS = 1536

_OPENAI_RATE_LIMIT_STATUS = 429


def _is_rate_limit_error(exc: BaseException) -> bool:
    """Return True for OpenAI RateLimitError (HTTP 429)."""
    from openai import RateLimitError

    return isinstance(exc, RateLimitError)


class OpenAIEmbeddingProvider:
    """Generate embeddings using the OpenAI API (BYOK).

    Supports both single-text and batch embedding.  Rate-limit errors
    (HTTP 429) are retried automatically with exponential back-off via
    ``tenacity``.

    Implements the :class:`~raven_worker.providers.base.EmbeddingProvider` protocol.
    """

    def __init__(
        self,
        api_key: str,
        model: str = _DEFAULT_MODEL,
        dimensions: int = _DEFAULT_DIMENSIONS,
        base_url: str | None = None,
    ) -> None:
        self._model = model
        self._dimensions = dimensions
        self._client = AsyncOpenAI(
            api_key=api_key,
            base_url=base_url,
        )

    @retry(
        retry=retry_if_exception(_is_rate_limit_error),
        wait=wait_exponential(multiplier=1, min=1, max=60),
        stop=stop_after_attempt(5),
        reraise=True,
    )
    async def embed(self, text: str) -> list[float]:
        """Generate an embedding for a single text via the OpenAI API.

        Args:
            text: Input text to embed.

        Returns:
            Embedding vector as a list of floats.
        """
        logger.info("openai_embed", model=self._model, text_length=len(text))
        resp = await self._client.embeddings.create(model=self._model, input=text)
        return list(resp.data[0].embedding)

    @retry(
        retry=retry_if_exception(_is_rate_limit_error),
        wait=wait_exponential(multiplier=1, min=1, max=60),
        stop=stop_after_attempt(5),
        reraise=True,
    )
    async def embed_batch(self, texts: list[str]) -> list[list[float]]:
        """Generate embeddings for a batch of texts.

        OpenAI natively supports list input for embeddings, so a single
        API call handles the whole batch.

        Args:
            texts: List of input texts to embed.

        Returns:
            List of embedding vectors, one per input text.
        """
        logger.info("openai_embed_batch", model=self._model, batch_size=len(texts))
        resp = await self._client.embeddings.create(model=self._model, input=texts)
        # Results are returned in the same order as the input
        return [list(item.embedding) for item in sorted(resp.data, key=lambda d: d.index)]

    @property
    def dimensions(self) -> int:
        return self._dimensions

    @property
    def model_name(self) -> str:
        return self._model


def _verify_protocol_compliance() -> None:
    """Static assertion that OpenAIEmbeddingProvider satisfies EmbeddingProvider."""
    _: EmbeddingProvider = OpenAIEmbeddingProvider(api_key="dummy")  # noqa: F841
