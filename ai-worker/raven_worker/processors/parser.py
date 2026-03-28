"""LiteParse integration for document parsing (PDF, DOCX, images with OCR).

Invokes the LiteParse CLI as a subprocess and returns structured JSON output.
Falls back to direct text decoding for plain-text formats (text/plain,
text/markdown, text/html).
"""

from __future__ import annotations

import asyncio
import json
import tempfile
from dataclasses import dataclass, field

import structlog

from raven_worker.config import settings

logger = structlog.get_logger(__name__)

# MIME types that LiteParse handles via CLI
_LITEPARSE_MIME_TYPES: set[str] = {
    "application/pdf",
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document",  # DOCX
    "application/vnd.openxmlformats-officedocument.presentationml.presentation",  # PPTX
    "image/png",
    "image/jpeg",
    "image/tiff",
    "image/webp",
}

# MIME types we handle directly without LiteParse
_DIRECT_MIME_TYPES: set[str] = {
    "text/plain",
    "text/markdown",
    "text/html",
}

# Map MIME types to common file extensions for temp files
_MIME_TO_EXT: dict[str, str] = {
    "application/pdf": ".pdf",
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
    "application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
    "image/png": ".png",
    "image/jpeg": ".jpg",
    "image/tiff": ".tiff",
    "image/webp": ".webp",
    "text/plain": ".txt",
    "text/markdown": ".md",
    "text/html": ".html",
}


@dataclass
class ParseResult:
    """Structured output from document parsing."""

    text: str
    metadata: dict[str, str | int | float] = field(default_factory=dict)
    pages: list[str] = field(default_factory=list)


class LiteParseError(Exception):
    """Raised when the LiteParse CLI fails."""


class DocumentParser:
    """Parse documents into structured text using LiteParse CLI.

    Supported formats:
    - PDF, DOCX, PPTX (via LiteParse CLI subprocess)
    - Images with OCR: PNG, JPEG, TIFF, WebP (via LiteParse CLI)
    - Plain text, Markdown, HTML (handled directly)
    """

    def __init__(self, liteparse_path: str | None = None) -> None:
        self._liteparse_path = liteparse_path or settings.liteparse_path

    async def parse(self, content: bytes, mime_type: str, file_name: str) -> str:
        """Extract text content from a document.

        Args:
            content: Raw document bytes.
            mime_type: MIME type of the document.
            file_name: Original file name for format detection fallback.

        Returns:
            Extracted plain text content.

        Raises:
            ValueError: If the MIME type is not supported.
            LiteParseError: If the LiteParse CLI invocation fails.
        """
        if not content:
            return ""

        logger.info(
            "parse_document",
            mime_type=mime_type,
            file_name=file_name,
            size=len(content),
        )

        if mime_type in _DIRECT_MIME_TYPES:
            return self._parse_direct(content, mime_type)

        if mime_type in _LITEPARSE_MIME_TYPES:
            return await self._parse_with_liteparse(content, mime_type, file_name)

        raise ValueError(
            f"Unsupported MIME type: {mime_type}. "
            f"Supported types: {sorted(_LITEPARSE_MIME_TYPES | _DIRECT_MIME_TYPES)}"
        )

    async def parse_structured(self, content: bytes, mime_type: str, file_name: str) -> ParseResult:
        """Parse a document and return structured output with metadata.

        Args:
            content: Raw document bytes.
            mime_type: MIME type of the document.
            file_name: Original file name.

        Returns:
            A ParseResult with text, metadata, and per-page content.
        """
        if not content:
            return ParseResult(text="", metadata={"source": file_name})

        if mime_type in _DIRECT_MIME_TYPES:
            text = self._parse_direct(content, mime_type)
            return ParseResult(
                text=text,
                metadata={"source": file_name, "mime_type": mime_type},
            )

        if mime_type in _LITEPARSE_MIME_TYPES:
            return await self._parse_structured_liteparse(content, mime_type, file_name)

        raise ValueError(f"Unsupported MIME type: {mime_type}")

    def _parse_direct(self, content: bytes, mime_type: str) -> str:
        """Decode text-based formats directly."""
        text = content.decode("utf-8", errors="replace")

        if mime_type == "text/html":
            return self._strip_html(text)

        return text

    @staticmethod
    def _strip_html(html: str) -> str:
        """Minimal HTML tag stripping for direct HTML parsing."""
        import re

        # Remove script and style blocks
        html = re.sub(r"<(script|style)[^>]*>.*?</\1>", "", html, flags=re.DOTALL | re.IGNORECASE)
        # Replace block-level tags with newlines
        html = re.sub(r"</(p|div|h[1-6]|li|tr|br)>", "\n", html, flags=re.IGNORECASE)
        html = re.sub(r"<br\s*/?>", "\n", html, flags=re.IGNORECASE)
        # Strip remaining tags
        html = re.sub(r"<[^>]+>", "", html)
        # Collapse whitespace
        lines = [line.strip() for line in html.splitlines()]
        return "\n".join(line for line in lines if line)

    async def _parse_with_liteparse(self, content: bytes, mime_type: str, file_name: str) -> str:
        """Invoke LiteParse CLI and return plain text."""
        result = await self._parse_structured_liteparse(content, mime_type, file_name)
        return result.text

    async def _parse_structured_liteparse(
        self, content: bytes, mime_type: str, file_name: str
    ) -> ParseResult:
        """Invoke LiteParse CLI and return structured output."""
        ext = _MIME_TO_EXT.get(mime_type, "")

        with tempfile.NamedTemporaryFile(suffix=ext, delete=True) as tmp:
            tmp.write(content)
            tmp.flush()
            tmp_path = tmp.name

            raw_output = await self._run_liteparse(tmp_path)

        return self._parse_liteparse_output(raw_output, file_name, mime_type)

    async def _run_liteparse(self, file_path: str) -> str:
        """Execute the LiteParse CLI and return its stdout.

        Args:
            file_path: Path to the temporary file to parse.

        Returns:
            Raw stdout from the LiteParse CLI.

        Raises:
            LiteParseError: If the process exits with a non-zero code.
        """
        cmd = [self._liteparse_path, "--output-format", "json", file_path]

        logger.debug("liteparse_invoke", cmd=cmd)

        process = await asyncio.create_subprocess_exec(
            *cmd,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        stdout, stderr = await process.communicate()

        if process.returncode != 0:
            error_msg = stderr.decode("utf-8", errors="replace").strip()
            logger.error(
                "liteparse_failed",
                returncode=process.returncode,
                stderr=error_msg,
            )
            raise LiteParseError(f"LiteParse exited with code {process.returncode}: {error_msg}")

        return stdout.decode("utf-8", errors="replace")

    @staticmethod
    def _parse_liteparse_output(raw_output: str, file_name: str, mime_type: str) -> ParseResult:
        """Parse the JSON output from LiteParse CLI.

        Expected JSON structure::

            {
                "text": "full document text",
                "pages": ["page 1 text", "page 2 text", ...],
                "metadata": { ... }
            }

        Falls back to treating the entire output as plain text if JSON
        parsing fails.
        """
        try:
            data = json.loads(raw_output)
        except (json.JSONDecodeError, ValueError):
            logger.warning("liteparse_json_fallback", file_name=file_name)
            return ParseResult(
                text=raw_output.strip(),
                metadata={"source": file_name, "mime_type": mime_type},
            )

        text = data.get("text", "")
        pages = data.get("pages", [])
        metadata = data.get("metadata", {})
        metadata.setdefault("source", file_name)
        metadata.setdefault("mime_type", mime_type)

        return ParseResult(text=text, metadata=metadata, pages=pages)
