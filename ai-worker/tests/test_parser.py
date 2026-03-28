"""Tests for the DocumentParser (LiteParse integration)."""

import json
from unittest.mock import AsyncMock, patch

import pytest

from raven_worker.processors.parser import DocumentParser, LiteParseError, ParseResult


@pytest.fixture
def parser():
    return DocumentParser(liteparse_path="/usr/bin/liteparse")


# --- Direct parsing (text/plain, text/markdown, text/html) ---


@pytest.mark.asyncio
async def test_parse_plain_text(parser):
    content = b"Hello, world! This is a test document."
    result = await parser.parse(content, "text/plain", "test.txt")
    assert "Hello, world!" in result
    assert len(result) > 0


@pytest.mark.asyncio
async def test_parse_markdown(parser):
    content = b"# Title\n\nSome content here.\n\n## Section\n\nMore text."
    result = await parser.parse(content, "text/markdown", "doc.md")
    assert "Title" in result
    assert "Some content here" in result


@pytest.mark.asyncio
async def test_parse_html(parser):
    content = b"<html><body><h1>Title</h1><p>Some content.</p></body></html>"
    result = await parser.parse(content, "text/html", "page.html")
    assert "Title" in result
    assert "Some content" in result


@pytest.mark.asyncio
async def test_parse_html_strips_scripts(parser):
    content = b"<html><body><script>alert('xss')</script><p>Real content.</p></body></html>"
    result = await parser.parse(content, "text/html", "page.html")
    assert "alert" not in result
    assert "Real content" in result


@pytest.mark.asyncio
async def test_parse_unknown_mime_raises(parser):
    with pytest.raises(ValueError, match="Unsupported"):
        await parser.parse(b"data", "application/octet-stream", "file.bin")


@pytest.mark.asyncio
async def test_parse_empty_content_returns_empty(parser):
    result = await parser.parse(b"", "text/plain", "empty.txt")
    assert result == ""


# --- LiteParse CLI subprocess ---


@pytest.mark.asyncio
async def test_parse_pdf_invokes_liteparse(parser):
    """PDF parsing should invoke the LiteParse CLI and parse JSON output."""
    liteparse_output = json.dumps({
        "text": "Extracted PDF text content.",
        "pages": ["Page 1 text", "Page 2 text"],
        "metadata": {"page_count": 2},
    })

    mock_process = AsyncMock()
    mock_process.communicate = AsyncMock(
        return_value=(liteparse_output.encode(), b"")
    )
    mock_process.returncode = 0

    with patch("asyncio.create_subprocess_exec", return_value=mock_process) as mock_exec:
        result = await parser.parse(b"%PDF-fake", "application/pdf", "doc.pdf")

    assert result == "Extracted PDF text content."
    # Verify CLI was called with correct args
    call_args = mock_exec.call_args[0]
    assert call_args[0] == "/usr/bin/liteparse"
    assert "--output-format" in call_args
    assert "json" in call_args


@pytest.mark.asyncio
async def test_parse_docx_invokes_liteparse(parser):
    """DOCX parsing should invoke the LiteParse CLI."""
    liteparse_output = json.dumps({"text": "Word document text.", "pages": [], "metadata": {}})

    mock_process = AsyncMock()
    mock_process.communicate = AsyncMock(
        return_value=(liteparse_output.encode(), b"")
    )
    mock_process.returncode = 0

    with patch("asyncio.create_subprocess_exec", return_value=mock_process):
        mime = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
        result = await parser.parse(b"PK\x03\x04", mime, "doc.docx")

    assert result == "Word document text."


@pytest.mark.asyncio
async def test_parse_image_ocr(parser):
    """Image parsing should invoke LiteParse for OCR."""
    liteparse_output = json.dumps({"text": "OCR extracted text.", "pages": [], "metadata": {}})

    mock_process = AsyncMock()
    mock_process.communicate = AsyncMock(
        return_value=(liteparse_output.encode(), b"")
    )
    mock_process.returncode = 0

    with patch("asyncio.create_subprocess_exec", return_value=mock_process):
        result = await parser.parse(b"\x89PNG", "image/png", "scan.png")

    assert result == "OCR extracted text."


@pytest.mark.asyncio
async def test_liteparse_failure_raises_error(parser):
    """Non-zero exit code from LiteParse should raise LiteParseError."""
    mock_process = AsyncMock()
    mock_process.communicate = AsyncMock(
        return_value=(b"", b"Error: corrupt file")
    )
    mock_process.returncode = 1

    with (
        patch("asyncio.create_subprocess_exec", return_value=mock_process),
        pytest.raises(LiteParseError, match="corrupt file"),
    ):
        await parser.parse(b"%PDF-bad", "application/pdf", "corrupt.pdf")


@pytest.mark.asyncio
async def test_liteparse_non_json_fallback(parser):
    """If LiteParse returns non-JSON, fall back to raw text."""
    mock_process = AsyncMock()
    mock_process.communicate = AsyncMock(
        return_value=(b"Plain text fallback output", b"")
    )
    mock_process.returncode = 0

    with patch("asyncio.create_subprocess_exec", return_value=mock_process):
        result = await parser.parse(b"%PDF-ok", "application/pdf", "doc.pdf")

    assert result == "Plain text fallback output"


# --- Structured parsing ---


@pytest.mark.asyncio
async def test_parse_structured_returns_parse_result(parser):
    """parse_structured should return a ParseResult with metadata."""
    result = await parser.parse_structured(
        b"Hello, structured!", "text/plain", "test.txt"
    )
    assert isinstance(result, ParseResult)
    assert "Hello, structured!" in result.text
    assert result.metadata["source"] == "test.txt"
    assert result.metadata["mime_type"] == "text/plain"


@pytest.mark.asyncio
async def test_parse_structured_empty_content(parser):
    result = await parser.parse_structured(b"", "text/plain", "empty.txt")
    assert result.text == ""
    assert result.metadata["source"] == "empty.txt"
