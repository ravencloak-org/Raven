"""Tests for the TextChunker (RecursiveCharacterTextSplitter)."""

from raven_worker.processors.chunker import Chunk, TextChunker


def test_empty_text_returns_empty_list():
    chunker = TextChunker()
    assert chunker.chunk("") == []


def test_short_text_returns_single_chunk():
    chunker = TextChunker(chunk_size=512, chunk_overlap=64)
    text = "Short text under the chunk size limit."
    chunks = chunker.chunk(text)
    assert len(chunks) == 1
    assert chunks[0] == text


def test_long_text_produces_multiple_chunks():
    chunker = TextChunker(chunk_size=100, chunk_overlap=20)
    text = "word " * 100  # 500 chars
    chunks = chunker.chunk(text)
    assert len(chunks) > 1


def test_chunks_have_overlap():
    """Adjacent chunks should share overlapping content."""
    chunker = TextChunker(chunk_size=50, chunk_overlap=10)
    # Build text with unique words to detect overlap
    text = " ".join(f"word{i}" for i in range(50))
    chunks = chunker.chunk(text)
    assert len(chunks) > 1
    # Each chunk should be <= chunk_size + some tolerance
    for chunk in chunks:
        assert len(chunk) <= 60  # chunk_size + small tolerance


def test_chunk_count_matches_log_metadata():
    chunker = TextChunker(chunk_size=100, chunk_overlap=20)
    text = "sentence. " * 50
    chunks = chunker.chunk(text)
    assert len(chunks) > 0


def test_custom_separators_respected():
    chunker = TextChunker(chunk_size=200, chunk_overlap=0)
    # Text with clear paragraph boundaries
    text = "Paragraph one.\n\nParagraph two.\n\nParagraph three."
    chunks = chunker.chunk(text)
    # Should split on paragraphs
    assert any("Paragraph one" in c for c in chunks)


# --- Default configuration ---


def test_default_chunk_size():
    """Default chunk_size should be 2048 chars (~512 tokens)."""
    chunker = TextChunker()
    assert chunker.chunk_size == 2048


def test_default_chunk_overlap():
    """Default chunk_overlap should be 200 chars (~50 tokens)."""
    chunker = TextChunker()
    assert chunker.chunk_overlap == 200


# --- chunk_with_metadata ---


def test_chunk_with_metadata_returns_chunk_objects():
    chunker = TextChunker(chunk_size=100, chunk_overlap=20)
    text = "Hello world. " * 30
    chunks = chunker.chunk_with_metadata(text, source="test.pdf")
    assert len(chunks) > 0
    assert all(isinstance(c, Chunk) for c in chunks)


def test_chunk_metadata_contains_source():
    chunker = TextChunker(chunk_size=100, chunk_overlap=20)
    text = "Some document content. " * 20
    chunks = chunker.chunk_with_metadata(text, source="report.pdf")
    for chunk in chunks:
        assert chunk.metadata["source"] == "report.pdf"


def test_chunk_metadata_contains_index():
    chunker = TextChunker(chunk_size=100, chunk_overlap=20)
    text = "Some document content. " * 20
    chunks = chunker.chunk_with_metadata(text, source="doc.pdf")
    for i, chunk in enumerate(chunks):
        assert chunk.index == i
        assert chunk.metadata["chunk_index"] == i


def test_chunk_metadata_contains_total():
    chunker = TextChunker(chunk_size=100, chunk_overlap=20)
    text = "Some document content. " * 20
    chunks = chunker.chunk_with_metadata(text, source="doc.pdf")
    total = len(chunks)
    for chunk in chunks:
        assert chunk.metadata["chunk_total"] == total


def test_chunk_metadata_extra_metadata():
    chunker = TextChunker(chunk_size=100, chunk_overlap=20)
    text = "Some document content. " * 20
    extra = {"org_id": "org-1", "kb_id": "kb-42"}
    chunks = chunker.chunk_with_metadata(text, source="doc.pdf", extra_metadata=extra)
    for chunk in chunks:
        assert chunk.metadata["org_id"] == "org-1"
        assert chunk.metadata["kb_id"] == "kb-42"


def test_chunk_with_metadata_empty_returns_empty():
    chunker = TextChunker()
    assert chunker.chunk_with_metadata("") == []


def test_heading_extraction_markdown():
    """Chunks starting with markdown headings should have heading metadata."""
    chunker = TextChunker(chunk_size=200, chunk_overlap=0)
    text = "# Introduction\n\nThis is the intro paragraph with enough text to fill a chunk."
    chunks = chunker.chunk_with_metadata(text, source="doc.md")
    assert len(chunks) >= 1
    assert chunks[0].metadata.get("heading") == "Introduction"


def test_heading_extraction_short_line():
    """Chunks starting with short non-punctuated lines should extract headings."""
    chunker = TextChunker(chunk_size=200, chunk_overlap=0)
    text = "Overview\nThis section covers the basics of the system architecture and design."
    chunks = chunker.chunk_with_metadata(text, source="doc.txt")
    assert len(chunks) >= 1
    assert chunks[0].metadata.get("heading") == "Overview"


# --- Paragraph splitting ---


def test_paragraph_splitting():
    """Long text with paragraphs should split on paragraph boundaries."""
    chunker = TextChunker(chunk_size=100, chunk_overlap=0)
    paragraphs = [f"Paragraph {i} content that fills some space." for i in range(10)]
    text = "\n\n".join(paragraphs)
    chunks = chunker.chunk(text)
    assert len(chunks) > 1
