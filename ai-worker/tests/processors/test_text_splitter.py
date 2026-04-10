"""Tests for TextChunker (RecursiveCharacterTextSplitter).

These tests supplement the existing test_chunker.py with additional coverage
focused on the splitter's core size/overlap/boundary behaviour.
"""

from __future__ import annotations

from raven_worker.processors.chunker import TextChunker


class TestTextSplitterSizeAndOverlap:
    def test_chunk_size_respected(self):
        """All chunks must be <= chunk_size + tolerance for word boundaries."""
        text = "word " * 200  # 1000 chars
        splitter = TextChunker(chunk_size=100, chunk_overlap=10)
        chunks = splitter.chunk(text)
        for chunk in chunks:
            # Allow small overage for word-boundary preservation
            assert len(chunk) <= 120, f"chunk too long: {len(chunk)}"

    def test_overlap_present_between_adjacent_chunks(self):
        """Adjacent chunks should share overlapping words."""
        text = "alpha beta gamma delta epsilon zeta eta theta iota kappa " * 20
        splitter = TextChunker(chunk_size=50, chunk_overlap=20)
        chunks = splitter.chunk(text)
        assert len(chunks) > 1
        # Consecutive chunks should share some tokens (overlap)
        first_words = set(chunks[0].split())
        second_words = set(chunks[1].split())
        assert len(first_words & second_words) > 0, "No overlap found between adjacent chunks"

    def test_no_empty_chunks_produced(self):
        """Every produced chunk must be non-empty."""
        text = "Complete sentence. Another sentence. Third sentence."
        chunks = TextChunker(chunk_size=30, chunk_overlap=5).chunk(text)
        for chunk in chunks:
            assert chunk.strip() != "", "Empty chunk produced"

    def test_single_chunk_when_text_fits(self):
        """Short text that fits in a single chunk should not be split."""
        text = "Short text"
        chunks = TextChunker(chunk_size=1000, chunk_overlap=0).chunk(text)
        assert len(chunks) == 1
        assert chunks[0] == "Short text"

    def test_zero_overlap_no_repeated_content(self):
        """With chunk_overlap=0, consecutive chunks should have minimal shared words."""
        # Use unique sequences so overlap is clearly detectable
        text = " ".join(f"uniqueword{i}" for i in range(100))
        splitter = TextChunker(chunk_size=50, chunk_overlap=0)
        chunks = splitter.chunk(text)
        if len(chunks) > 1:
            # With zero overlap the splitter should not duplicate words
            # (tolerance: at most 1 word boundary overlap)
            first_words = set(chunks[0].split())
            second_words = set(chunks[1].split())
            overlap_count = len(first_words & second_words)
            assert overlap_count <= 1, f"Too much overlap with chunk_overlap=0: {overlap_count}"

    def test_large_text_produces_many_chunks(self):
        """10000-char text with chunk_size=512 should produce multiple chunks."""
        text = "word " * 2000  # ~10000 chars
        splitter = TextChunker(chunk_size=512, chunk_overlap=50)
        chunks = splitter.chunk(text)
        assert len(chunks) > 3, f"Expected many chunks, got {len(chunks)}"


class TestTextSplitterMetadata:
    def test_chunk_with_metadata_indices_are_sequential(self):
        """chunk_with_metadata must return chunks with sequential indices 0,1,2,..."""
        text = "word " * 300
        splitter = TextChunker(chunk_size=100, chunk_overlap=10)
        chunks = splitter.chunk_with_metadata(text, source="test.txt")
        indices = [c.index for c in chunks]
        assert indices == list(range(len(chunks))), f"Non-sequential indices: {indices}"

    def test_chunk_with_metadata_source_propagated(self):
        """All chunks must carry the source identifier in their metadata."""
        text = "Some document content. " * 30
        chunks = TextChunker(chunk_size=100, chunk_overlap=20).chunk_with_metadata(
            text, source="myfile.pdf"
        )
        for chunk in chunks:
            assert chunk.metadata["source"] == "myfile.pdf"

    def test_chunk_with_metadata_extra_metadata_propagated(self):
        """Extra metadata dict must be present in every chunk."""
        text = "Some content. " * 30
        extra = {"org_id": "org-test", "kb_id": "kb-42", "doc_id": "doc-meta-test"}
        chunks = TextChunker(chunk_size=100, chunk_overlap=10).chunk_with_metadata(
            text, source="doc.pdf", extra_metadata=extra
        )
        for chunk in chunks:
            assert chunk.metadata["org_id"] == "org-test"
            assert chunk.metadata["kb_id"] == "kb-42"
            assert chunk.metadata["doc_id"] == "doc-meta-test"

    def test_chunk_total_consistent(self):
        """chunk_total in metadata must equal the total number of chunks."""
        text = "word " * 200
        chunks = TextChunker(chunk_size=100, chunk_overlap=10).chunk_with_metadata(
            text, source="x.txt"
        )
        total = len(chunks)
        for chunk in chunks:
            assert chunk.metadata["chunk_total"] == total

    def test_heading_extracted_from_markdown_heading(self):
        """A chunk starting with '# Heading' should have heading metadata."""
        text = "# My Document Title\n\nThis paragraph has enough text to fill a chunk adequately."
        chunks = TextChunker(chunk_size=200, chunk_overlap=0).chunk_with_metadata(
            text, source="doc.md"
        )
        assert len(chunks) >= 1
        assert chunks[0].metadata.get("heading") == "My Document Title"
