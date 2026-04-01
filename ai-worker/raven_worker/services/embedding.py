"""Embedding service implementing the GetEmbedding RPC."""

from __future__ import annotations

import grpc
import structlog

from raven_worker.generated import ai_worker_pb2
from raven_worker.providers.registry import get_provider_for_request

logger = structlog.get_logger(__name__)


class EmbeddingServicer:
    """Handles GetEmbedding RPC calls."""

    async def get_embedding(self, request, context) -> ai_worker_pb2.EmbeddingResponse:
        """Generate an embedding vector for the given text.

        Looks up the BYOK provider configuration for the requesting organisation
        from the database, decrypts the API key, calls the provider, and returns
        the embedding vector.

        Args:
            request: :class:`ai_worker_pb2.EmbeddingRequest` with fields
                ``text``, ``org_id``, ``model``, ``provider``.
            context: gRPC ``ServicerContext`` for aborting with status codes.

        Returns:
            :class:`ai_worker_pb2.EmbeddingResponse` containing the vector and
            its dimensionality.
        """
        logger.info(
            "get_embedding_request",
            org_id=request.org_id,
            model=request.model,
            provider=request.provider,
            text_length=len(request.text),
        )
        try:
            provider = await get_provider_for_request(
                request.org_id,
                request.provider,
                request.model,
            )
            embedding = await provider.embed(request.text)
            logger.info(
                "get_embedding_success",
                org_id=request.org_id,
                provider=request.provider,
                model=request.model,
                dimensions=len(embedding),
            )
            return ai_worker_pb2.EmbeddingResponse(
                embedding=embedding,
                dimensions=len(embedding),
            )
        except ValueError as exc:
            logger.warning("get_embedding_bad_request", error=str(exc))
            await context.abort(grpc.StatusCode.NOT_FOUND, str(exc))
            return ai_worker_pb2.EmbeddingResponse()  # pragma: no cover
        except Exception as exc:
            logger.exception("get_embedding_error", error=str(exc))
            await context.abort(grpc.StatusCode.INTERNAL, str(exc))
            return ai_worker_pb2.EmbeddingResponse()  # pragma: no cover
