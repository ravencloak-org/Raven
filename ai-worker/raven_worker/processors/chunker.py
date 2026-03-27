"""Text chunking stub for splitting documents into embeddable segments."""

import structlog

logger = structlog.get_logger(__name__)

# Sensible defaults for chunk sizes (in characters)
DEFAULT_CHUNK_SIZE = 512
DEFAULT_CHUNK_OVERLAP = 64


class TextChunker:
    """Split text into overlapping chunks suitable for embedding.

    This is a stub that will be replaced with a proper chunking strategy
    (e.g. recursive character splitting, semantic paragraph detection).
    """

    def __init__(
        self,
        chunk_size: int = DEFAULT_CHUNK_SIZE,
        chunk_overlap: int = DEFAULT_CHUNK_OVERLAP,
    ) -> None:
        self.chunk_size = chunk_size
        self.chunk_overlap = chunk_overlap

    def chunk(self, text: str) -> list[str]:
        """Split text into overlapping chunks.

        Args:
            text: The full document text to chunk.

        Returns:
            A list of text chunks.
        """
        if not text:
            return []

        chunks: list[str] = []
        start = 0
        while start < len(text):
            end = start + self.chunk_size
            chunks.append(text[start:end])
            start += self.chunk_size - self.chunk_overlap

        logger.info("chunked_text", total_length=len(text), chunk_count=len(chunks))
        return chunks
