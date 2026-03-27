"""Abstract base for embedding providers."""

from typing import Protocol


class EmbeddingProvider(Protocol):
    """Protocol that all embedding providers must implement.

    Concrete implementations (OpenAI, local models, etc.) must provide
    the ``embed`` coroutine and the ``dimensions`` / ``model_name`` properties.
    """

    async def embed(self, text: str) -> list[float]:
        """Generate an embedding vector for the given text.

        Args:
            text: Input text to embed.

        Returns:
            A list of floats representing the embedding vector.
        """
        ...

    @property
    def dimensions(self) -> int:
        """Return the dimensionality of vectors produced by this provider."""
        ...

    @property
    def model_name(self) -> str:
        """Return the model identifier used by this provider."""
        ...
