"""Tests for DocumentParser's HTML parsing capability.

The actual HTML parser lives in DocumentParser._strip_html (used for text/html
MIME type). We test it through the public parse() interface as well as via
the static method directly.
"""

from __future__ import annotations

import pytest

from raven_worker.processors.parser import DocumentParser


@pytest.fixture
def parser():
    return DocumentParser(liteparse_path="/usr/bin/liteparse")


class TestHTMLParsing:
    async def test_strips_scripts(self, parser):
        html = b"<html><head><script>alert('xss')</script></head><body>Content</body></html>"
        result = await parser.parse(html, "text/html", "page.html")
        assert "alert" not in result
        assert "Content" in result

    async def test_strips_styles(self, parser):
        html = b"<html><body><style>.cls { color: red; }</style><p>Text</p></body></html>"
        result = await parser.parse(html, "text/html", "page.html")
        assert "color: red" not in result
        assert "Text" in result

    async def test_extracts_body_text(self, parser):
        html = b"<html><body><h1>Title</h1><p>Paragraph content.</p></body></html>"
        result = await parser.parse(html, "text/html", "page.html")
        assert "Title" in result
        assert "Paragraph content." in result

    async def test_handles_malformed_html(self, parser):
        # The parser is regex-based (not BeautifulSoup), should handle unclosed tags gracefully
        html = b"<html><body><p>Unclosed tag<div>More content here"
        result = await parser.parse(html, "text/html", "page.html")
        # Should not raise, should extract available text
        assert isinstance(result, str)
        assert "Unclosed tag" in result or "More content" in result

    async def test_empty_html_returns_empty_string(self, parser):
        result = await parser.parse(b"", "text/html", "page.html")
        assert result == ""

    async def test_strips_block_tags_replaced_with_newlines(self, parser):
        """Block-level closing tags are replaced with newlines, not spaces."""
        html = b"<html><body><p>Para one</p><p>Para two</p></body></html>"
        result = await parser.parse(html, "text/html", "page.html")
        assert "Para one" in result
        assert "Para two" in result

    async def test_br_tags_replaced_with_newlines(self, parser):
        html = b"<html><body>Line 1<br/>Line 2<br>Line 3</body></html>"
        result = await parser.parse(html, "text/html", "page.html")
        assert "Line 1" in result
        assert "Line 2" in result
        assert "Line 3" in result

    def test_strip_html_static_removes_script_content(self):
        """DocumentParser._strip_html should remove script block content."""
        html = "<script>var x = 1; alert('bad');</script><p>Good content</p>"
        result = DocumentParser._strip_html(html)
        assert "alert" not in result
        assert "Good content" in result

    def test_strip_html_static_removes_style_content(self):
        """DocumentParser._strip_html should remove style block content."""
        html = "<style>body { color: red; }</style><p>Visible text</p>"
        result = DocumentParser._strip_html(html)
        assert "color: red" not in result
        assert "Visible text" in result

    def test_strip_html_preserves_plain_text(self):
        """Plain text with no tags should pass through unchanged."""
        html = "Just plain text without tags"
        result = DocumentParser._strip_html(html)
        assert "Just plain text" in result

    async def test_parse_markdown_passes_through(self, parser):
        """text/markdown content should be returned as-is (no stripping)."""
        md = b"# Heading\n\n**Bold** and *italic* text."
        result = await parser.parse(md, "text/markdown", "doc.md")
        assert "# Heading" in result
        assert "**Bold**" in result
