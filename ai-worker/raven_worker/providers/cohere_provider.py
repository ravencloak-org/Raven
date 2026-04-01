"""Cohere embedding provider (BYOK)."""

from __future__ import annotations

import cohere
import structlog
from tenacity import retry, retry_if_exception, stop_after_attempt, wait_exponential

from raven_worker.providers.base import EmbeddingProvider

logger = structlog.get_logger(__name__)

# Default model and dimensions for Cohere embed-english-v3.0
_DEFAULT_MODEL = "embed-english-v3.0"
_DEFAULT_DIMENSIONS = 1024


def _is_cohere_rate_limit(exc: BaseException) -> bool:
    """Return True for Cohere TooManyRequestsError (HTTP 429)."""
    try:
        from cohere.errors import TooManyRequestsError  # type: ignore[import-untyped]

        return isinstance(exc, TooManyRequestsError)
    except ImportError:
        # Fallback: inspect exception class name or HTTP status attribute
        cls_name = type(exc).__name__
        return "TooManyRequests" in cls_name or "RateLimit" in cls_name


class CohereEmbeddingProvider:
    """Generate embeddings using the Cohere API (BYOK).

    Supports both single-text and batch embedding.  Rate-limit errors
    are retried automatically with exponential back-off via ``tenacity``.

    Implements the :class:`~raven_worker.providers.base.EmbeddingProvider` protocol.
    """

    def __init__(
        self,
        api_key: str,
        model: str = _DEFAULT_MODEL,
        dimensions: int = _DEFAULT_DIMENSIONS,
    ) -> None:
        self._model = model
        self._dimensions = dimensions
        self._client = cohere.AsyncClientV2(api_key=api_key)

    @retry(
        retry=retry_if_exception(_is_cohere_rate_limit),
        wait=wait_exponential(multiplier=1, min=1, max=60),
        stop=stop_after_attempt(5),
        reraise=True,
    )
    async def embed(self, text: str) -> list[float]:
        """Generate an embedding for a single text via the Cohere API.

        Args:
            text: Input text to embed.

        Returns:
            Embedding vector as a list of floats.
        """
        logger.info("cohere_embed", model=self._model, text_length=len(text))
        resp = await self._client.embed(
            texts=[text],
            model=self._model,
            input_type="search_document",
            embedding_types=["float"],
        )
        embeddings = resp.embeddings
        # Cohere SDK v2 uses float_ as the Python attribute (float is a keyword)
        if hasattr(embeddings, "float_") and embeddings.float_ is not None:
            return list(embeddings.float_[0])
        raise ValueError("Cohere embed response missing float embeddings")

    @retry(
        retry=retry_if_exception(_is_cohere_rate_limit),
        wait=wait_exponential(multiplier=1, min=1, max=60),
        stop=stop_after_attempt(5),
        reraise=True,
    )
    async def embed_batch(self, texts: list[str]) -> list[list[float]]:
        """Generate embeddings for a batch of texts.

        Cohere natively supports list input so a single API call covers
        the whole batch.

        Args:
            texts: List of input texts to embed.

        Returns:
            List of embedding vectors, one per input text.
        """
        logger.info("cohere_embed_batch", model=self._model, batch_size=len(texts))
        resp = await self._client.embed(
            texts=texts,
            model=self._model,
            input_type="search_document",
            embedding_types=["float"],
        )
        embeddings = resp.embeddings
        # Cohere SDK v2 uses float_ as the Python attribute (float is a keyword)
        if hasattr(embeddings, "float_") and embeddings.float_ is not None:
            return [list(v) for v in embeddings.float_]
        raise ValueError("Cohere embed response missing float embeddings")

    @property
    def dimensions(self) -> int:
        return self._dimensions

    @property
    def model_name(self) -> str:
        return self._model


def _verify_protocol_compliance() -> None:
    """Static assertion that CohereEmbeddingProvider satisfies EmbeddingProvider."""
    _: EmbeddingProvider = CohereEmbeddingProvider(api_key="dummy")  # noqa: F841
