"""gRPC server setup with reflection, health checking, and graceful shutdown."""

import asyncio
import logging
import signal

import grpc
import structlog
from grpc_health.v1 import health, health_pb2, health_pb2_grpc
from grpc_reflection.v1alpha import reflection

from raven_worker.config import settings
from raven_worker.generated import ai_worker_pb2, ai_worker_pb2_grpc
from raven_worker.services.embedding import EmbeddingServicer
from raven_worker.services.rag import RAGServicer
from raven_worker.telemetry import get_grpc_server_interceptor, init_telemetry

logger = structlog.get_logger(__name__)


class AIWorkerServicer(
    ai_worker_pb2_grpc.AIWorkerServicer,
):
    """Combines all AI Worker RPC implementations into a single servicer."""

    def __init__(self) -> None:
        self._embedding = EmbeddingServicer()
        self._rag = RAGServicer()

    async def ParseAndEmbed(self, request, context):
        """Parse a document and generate embeddings for its chunks."""
        logger.info(
            "parse_and_embed_request",
            document_id=request.document_id,
            org_id=request.org_id,
            mime_type=request.mime_type,
        )
        await context.abort(
            grpc.StatusCode.UNIMPLEMENTED,
            "ParseAndEmbed is not yet implemented. "
            "This will integrate LiteParse for document parsing and chunk embedding.",
        )

    async def QueryRAG(self, request, context):
        """Stream RAG results for a query."""
        async for chunk in self._rag.query(request, context):
            yield chunk

    async def GetEmbedding(self, request, context):
        """Generate an embedding for a single text input."""
        return await self._embedding.get_embedding(request, context)


def _configure_logging() -> None:
    """Set up structured logging with structlog."""
    structlog.configure(
        processors=[
            structlog.contextvars.merge_contextvars,
            structlog.processors.add_log_level,
            structlog.processors.StackInfoRenderer(),
            structlog.dev.set_exc_info,
            structlog.processors.TimeStamper(fmt="iso"),
            structlog.dev.ConsoleRenderer(),
        ],
        wrapper_class=structlog.make_filtering_bound_logger(
            logging.getLevelName(settings.log_level.upper()),
        ),
        context_class=dict,
        logger_factory=structlog.PrintLoggerFactory(),
        cache_logger_on_first_use=True,
    )


async def serve() -> None:
    """Start the gRPC server and block until shutdown is requested."""
    _configure_logging()

    # Initialise OpenTelemetry (no-op when disabled or no endpoint).
    otel_endpoint = settings.otel_endpoint if settings.otel_enabled else None
    init_telemetry(service_name="raven-ai-worker", endpoint=otel_endpoint)

    # Build gRPC interceptors list.
    interceptors: list = [i for i in [get_grpc_server_interceptor()] if i is not None]

    server = grpc.aio.server(interceptors=interceptors or None)

    # Register AI Worker servicer
    ai_worker_pb2_grpc.add_AIWorkerServicer_to_server(AIWorkerServicer(), server)

    # Health checking
    health_servicer = health.aio.HealthServicer()
    health_pb2_grpc.add_HealthServicer_to_server(health_servicer, server)
    await health_servicer.set(
        "raven.ai.v1.AIWorker",
        health_pb2.HealthCheckResponse.SERVING,
    )

    # Server reflection for development/debugging
    service_names = (
        ai_worker_pb2.DESCRIPTOR.services_by_name["AIWorker"].full_name,
        health_pb2.DESCRIPTOR.services_by_name["Health"].full_name,
        reflection.SERVICE_NAME,
    )
    reflection.enable_server_reflection(service_names, server)

    listen_addr = f"[::]:{settings.grpc_port}"
    server.add_insecure_port(listen_addr)

    logger.info("starting_server", address=listen_addr, max_workers=settings.grpc_max_workers)
    await server.start()

    # Graceful shutdown on SIGINT / SIGTERM
    loop = asyncio.get_running_loop()
    shutdown_event = asyncio.Event()

    def _signal_handler() -> None:
        logger.info("shutdown_signal_received")
        shutdown_event.set()

    for sig in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(sig, _signal_handler)

    await shutdown_event.wait()

    logger.info("shutting_down", grace_period_seconds=5)
    await server.stop(grace=5)
    logger.info("server_stopped")
