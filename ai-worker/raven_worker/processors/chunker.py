"""Text chunking using langchain RecursiveCharacterTextSplitter.

Splits documents into overlapping chunks suitable for embedding, preserving
metadata such as source document, chunk index, and heading context.
"""

from __future__ import annotations

from dataclasses import dataclass, field

import structlog
from langchain_text_splitters import RecursiveCharacterTextSplitter

logger = structlog.get_logger(__name__)

# Defaults: 512 tokens target, 50 token overlap.
# Approximation: 1 token ~ 4 characters for English text.
_CHARS_PER_TOKEN = 4
DEFAULT_CHUNK_SIZE_TOKENS = 512
DEFAULT_CHUNK_OVERLAP_TOKENS = 50
DEFAULT_CHUNK_SIZE = DEFAULT_CHUNK_SIZE_TOKENS * _CHARS_PER_TOKEN  # 2048
DEFAULT_CHUNK_OVERLAP = DEFAULT_CHUNK_OVERLAP_TOKENS * _CHARS_PER_TOKEN  # 200

# Separators ordered from coarsest to finest granularity
_DEFAULT_SEPARATORS = [
    "\n\n",  # paragraph
    "\n",  # line
    ". ",  # sentence
    ", ",  # clause
    " ",  # word
    "",  # character
]


@dataclass
class Chunk:
    """A single text chunk with associated metadata."""

    text: str
    index: int
    metadata: dict[str, str | int] = field(default_factory=dict)


class TextChunker:
    """Split text into overlapping chunks suitable for embedding.

    Uses langchain's RecursiveCharacterTextSplitter under the hood with
    sensible defaults for RAG workloads: 512 tokens chunk size, 50 tokens
    overlap.
    """

    def __init__(
        self,
        chunk_size: int = DEFAULT_CHUNK_SIZE,
        chunk_overlap: int = DEFAULT_CHUNK_OVERLAP,
        separators: list[str] | None = None,
    ) -> None:
        if chunk_size <= 0:
            raise ValueError("chunk_size must be > 0")
        if chunk_overlap < 0:
            raise ValueError("chunk_overlap must be >= 0")
        if chunk_overlap >= chunk_size:
            raise ValueError("chunk_overlap must be < chunk_size")
        self._chunk_size = chunk_size
        self._chunk_overlap = chunk_overlap
        self._splitter = RecursiveCharacterTextSplitter(
            chunk_size=chunk_size,
            chunk_overlap=chunk_overlap,
            separators=separators or _DEFAULT_SEPARATORS,
            strip_whitespace=True,
        )

    @property
    def chunk_size(self) -> int:
        return self._chunk_size

    @property
    def chunk_overlap(self) -> int:
        return self._chunk_overlap

    def chunk(self, text: str, source: str = "") -> list[str]:
        """Split text into overlapping chunks.

        Args:
            text: The full document text to chunk.
            source: Optional source identifier for logging.

        Returns:
            A list of text chunks (plain strings).
        """
        if not text:
            return []

        chunks = self._splitter.split_text(text)

        logger.info(
            "chunked_text",
            source=source,
            total_length=len(text),
            chunk_count=len(chunks),
            chunk_size=self._chunk_size,
            chunk_overlap=self._chunk_overlap,
        )
        return chunks

    def chunk_with_metadata(
        self,
        text: str,
        source: str = "",
        extra_metadata: dict[str, str | int] | None = None,
    ) -> list[Chunk]:
        """Split text into chunks with metadata attached to each chunk.

        Each chunk carries:
        - ``source``: the source document identifier
        - ``chunk_index``: zero-based position in the document
        - ``heading``: extracted heading context (first line if it looks like one)
        - Any additional key-value pairs from ``extra_metadata``

        Args:
            text: The full document text to chunk.
            source: Source document identifier (filename, URL, etc.).
            extra_metadata: Additional metadata to attach to every chunk.

        Returns:
            A list of Chunk objects with text and metadata.
        """
        if not text:
            return []

        raw_chunks = self._splitter.split_text(text)
        result: list[Chunk] = []

        for idx, chunk_text in enumerate(raw_chunks):
            metadata: dict[str, str | int] = {
                "source": source,
                "chunk_index": idx,
                "chunk_total": len(raw_chunks),
            }

            heading = self._extract_heading(chunk_text)
            if heading:
                metadata["heading"] = heading

            if extra_metadata:
                metadata.update(extra_metadata)

            result.append(Chunk(text=chunk_text, index=idx, metadata=metadata))

        logger.info(
            "chunked_text_with_metadata",
            source=source,
            total_length=len(text),
            chunk_count=len(result),
        )
        return result

    @staticmethod
    def _extract_heading(text: str) -> str:
        """Extract a heading from the start of a chunk, if present.

        Looks for Markdown-style headings (lines starting with #) or
        lines that are short and end without punctuation (likely titles).
        """
        first_line = text.split("\n", maxsplit=1)[0].strip()

        # Markdown heading
        if first_line.startswith("#"):
            return first_line.lstrip("#").strip()

        # Short line without terminal punctuation (likely a heading)
        if len(first_line) < 100 and first_line and first_line[-1] not in ".!?:;,":
            return first_line

        return ""
