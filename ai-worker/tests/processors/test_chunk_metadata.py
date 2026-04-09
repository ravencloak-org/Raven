"""Tests for chunk metadata propagation.

Uses TextChunker.chunk_with_metadata (the actual API) instead of a
non-existent process_document pipeline function.

make_parse_request is a plain importable function in conftest.py, not a fixture.
"""

from __future__ import annotations

from raven_worker.processors.chunker import TextChunker
from tests.conftest import make_parse_request


def test_chunk_metadata_includes_source_info():
    """Every chunk must carry doc_id, kb_id, chunk_index, and source_url."""
    req = make_parse_request(
        content=b"First sentence. Second sentence. Third sentence. Fourth sentence.",
        doc_id="doc-meta-test",
        org_id="org-1",
        kb_id="kb-meta-test",
    )

    # Build the source_url from the parse request (simulates what the pipeline does)
    source_url = "https://example.com/page"
    extra_metadata = {
        "document_id": req.document_id,
        "kb_id": req.kb_id,
        "org_id": req.org_id,
        "source_url": source_url,
    }

    splitter = TextChunker(chunk_size=100, chunk_overlap=0)
    chunks = splitter.chunk_with_metadata(
        req.content.decode("utf-8"),
        source=source_url,
        extra_metadata=extra_metadata,
    )

    assert len(chunks) > 0, "at least one chunk must be produced"
    for i, chunk in enumerate(chunks):
        assert chunk.metadata["document_id"] == "doc-meta-test", (
            f"chunk[{i}].document_id missing"
        )
        assert chunk.metadata["kb_id"] == "kb-meta-test", f"chunk[{i}].kb_id missing"
        assert chunk.metadata["source_url"] == source_url, f"chunk[{i}].source_url missing"
        assert chunk.metadata["chunk_index"] == i, (
            f"chunk[{i}].chunk_index must equal {i}"
        )


def test_chunk_indices_sequential():
    """Indices must be 0, 1, 2, 3... with no gaps or duplicates."""
    req = make_parse_request(
        content=("word " * 300).encode("utf-8"),
        doc_id="doc-seq-test",
        org_id="org-1",
        kb_id="kb-seq-test",
    )

    splitter = TextChunker(chunk_size=100, chunk_overlap=10)
    chunks = splitter.chunk_with_metadata(
        req.content.decode("utf-8"),
        source="https://example.com",
    )

    indices = [c.index for c in chunks]
    assert indices == list(range(len(chunks))), f"Indices not sequential: {indices}"


def test_chunk_metadata_chunk_total_correct():
    """chunk_total metadata field must equal the actual number of chunks."""
    text = "Sentence with some words. " * 20
    splitter = TextChunker(chunk_size=80, chunk_overlap=10)
    chunks = splitter.chunk_with_metadata(text, source="test.txt")

    total = len(chunks)
    assert total > 1, "Need multiple chunks for this test"
    for chunk in chunks:
        assert chunk.metadata["chunk_total"] == total, (
            f"Expected chunk_total={total}, got {chunk.metadata['chunk_total']}"
        )


def test_empty_content_produces_no_chunks():
    """Empty text must yield zero chunks, not raise an error."""
    req = make_parse_request(content=b"", doc_id="doc-empty")
    splitter = TextChunker(chunk_size=100, chunk_overlap=0)
    chunks = splitter.chunk_with_metadata(
        req.content.decode("utf-8"),
        source="empty.txt",
    )
    assert chunks == []


def test_chunk_text_non_empty():
    """Every chunk's text must be non-empty after splitting."""
    text = "The quick brown fox jumps over the lazy dog. " * 50
    splitter = TextChunker(chunk_size=100, chunk_overlap=20)
    chunks = splitter.chunk_with_metadata(text, source="fox.txt")
    for chunk in chunks:
        assert chunk.text.strip() != "", "Chunk text must be non-empty"
