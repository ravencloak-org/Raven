"""Embedding service stub implementing the GetEmbedding RPC."""

import grpc
import structlog

from raven_worker.generated import ai_worker_pb2

logger = structlog.get_logger(__name__)


class EmbeddingServicer:
    """Handles GetEmbedding RPC calls."""

    async def get_embedding(self, request, context) -> ai_worker_pb2.EmbeddingResponse:
        """Generate an embedding vector for the given text.

        Currently returns UNIMPLEMENTED. Will be wired to an EmbeddingProvider
        (OpenAI, local model, etc.) once provider configuration is in place.
        """
        logger.info(
            "get_embedding_request",
            org_id=request.org_id,
            model=request.model,
            provider=request.provider,
            text_length=len(request.text),
        )
        await context.abort(
            grpc.StatusCode.UNIMPLEMENTED,
            "GetEmbedding is not yet implemented. "
            "Configure an embedding provider (e.g. OpenAI) to enable this RPC.",
        )
        # Unreachable, but keeps the return type correct for static analysis
        return ai_worker_pb2.EmbeddingResponse()  # pragma: no cover
