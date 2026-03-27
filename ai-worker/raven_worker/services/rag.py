"""RAG service stub implementing the QueryRAG streaming RPC."""

from collections.abc import AsyncIterator

import structlog

from raven_worker.generated import ai_worker_pb2

logger = structlog.get_logger(__name__)


class RAGServicer:
    """Handles QueryRAG streaming RPC calls."""

    async def query(self, request, context) -> AsyncIterator[ai_worker_pb2.RAGChunk]:
        """Stream RAG response chunks for the given query.

        Currently yields a single stub chunk indicating the feature is not yet
        implemented. Will be wired to the retrieval pipeline and LLM provider
        once those components are built.
        """
        logger.info(
            "query_rag_request",
            org_id=request.org_id,
            kb_ids=list(request.kb_ids),
            session_id=request.session_id,
            model=request.model,
            provider=request.provider,
            query_length=len(request.query),
        )
        yield ai_worker_pb2.RAGChunk(
            text="Not implemented yet",
            is_final=True,
            sources=[],
        )
