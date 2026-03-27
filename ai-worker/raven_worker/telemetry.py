"""OpenTelemetry initialisation for the Raven AI Worker.

When no endpoint is supplied the module configures no-op providers so the
rest of the application works without any observable overhead or log noise.
"""

from __future__ import annotations

import structlog

logger = structlog.get_logger(__name__)


def init_telemetry(service_name: str, endpoint: str | None) -> None:
    """Initialise OpenTelemetry tracing for the worker.

    Args:
        service_name: The ``service.name`` resource attribute.
        endpoint: OTLP gRPC collector endpoint (e.g. ``localhost:4317``).
            When *None* a no-op tracer is used.
    """
    if endpoint is None:
        logger.info("otel_disabled", reason="no endpoint configured")
        return

    try:
        from opentelemetry import trace
        from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
        from opentelemetry.sdk.resources import Resource
        from opentelemetry.sdk.trace import TracerProvider
        from opentelemetry.sdk.trace.export import BatchSpanProcessor
    except ImportError:
        logger.warning(
            "otel_packages_missing",
            hint="install opentelemetry-sdk and opentelemetry-exporter-otlp to enable tracing",
        )
        return

    resource = Resource.create({"service.name": service_name})
    provider = TracerProvider(resource=resource)

    exporter = OTLPSpanExporter(endpoint=endpoint, insecure=True)
    provider.add_span_processor(BatchSpanProcessor(exporter))

    trace.set_tracer_provider(provider)
    logger.info("otel_initialised", endpoint=endpoint, service=service_name)


def get_grpc_server_interceptor():
    """Return an OpenTelemetry gRPC server interceptor, or *None*.

    Returns *None* when the instrumentation package is not installed so
    callers can simply skip adding the interceptor.
    """
    try:
        from opentelemetry.instrumentation.grpc import aio_server_interceptor

        return aio_server_interceptor()
    except ImportError:
        logger.debug("otel_grpc_interceptor_unavailable")
        return None
    except Exception:
        logger.debug("otel_grpc_interceptor_error", exc_info=True)
        return None
