"""LiteParse integration stub for document parsing."""

import structlog

logger = structlog.get_logger(__name__)


class DocumentParser:
    """Parse documents into plain text using LiteParse.

    This is a stub that will be replaced with actual LiteParse integration.
    Supported formats will include PDF, DOCX, PPTX, HTML, and Markdown.
    """

    async def parse(self, content: bytes, mime_type: str, file_name: str) -> str:
        """Extract text content from a document.

        Args:
            content: Raw document bytes.
            mime_type: MIME type of the document.
            file_name: Original file name for format detection fallback.

        Returns:
            Extracted plain text content.

        Raises:
            NotImplementedError: Always, until LiteParse is integrated.
        """
        logger.info("parse_document", mime_type=mime_type, file_name=file_name, size=len(content))
        raise NotImplementedError(
            f"Document parsing not yet implemented for mime_type={mime_type}. "
            "LiteParse integration is pending."
        )
