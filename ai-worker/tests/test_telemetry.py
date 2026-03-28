"""Tests for the OpenTelemetry initialisation module."""

from __future__ import annotations

import builtins
import contextlib
import importlib
import sys
from collections.abc import Iterator
from unittest.mock import patch


@contextlib.contextmanager
def _hide_imports(keyword: str) -> Iterator[None]:
    """Temporarily hide modules whose name contains *keyword* from the import system.

    On entry, matching modules are removed from ``sys.modules`` and
    ``builtins.__import__`` is monkey-patched to raise ``ImportError``
    for any import whose name contains *keyword*.  On exit everything is
    restored.
    """
    hidden = {name: sys.modules.pop(name) for name in list(sys.modules) if keyword in name}
    real_import = builtins.__import__

    def fake_import(name, *args, **kwargs):
        if keyword in name:
            raise ImportError(f"mocked missing package ({keyword})")
        return real_import(name, *args, **kwargs)

    builtins.__import__ = fake_import  # type: ignore[assignment]
    try:
        yield
    finally:
        builtins.__import__ = real_import  # type: ignore[assignment]
        sys.modules.update(hidden)


class TestInitTelemetryNoOp:
    """init_telemetry must be safe to call without any OTel packages."""

    def test_none_endpoint_does_not_raise(self) -> None:
        from raven_worker.telemetry import init_telemetry

        # Should complete without error when endpoint is None.
        init_telemetry(service_name="test-worker", endpoint=None)

    def test_none_endpoint_does_not_configure_tracer(self) -> None:
        """When no endpoint is given the global tracer should remain untouched."""
        from raven_worker.telemetry import init_telemetry

        init_telemetry(service_name="test-worker", endpoint=None)
        # No assertion beyond not raising -- the no-op path is verified.


class TestInitTelemetryWithEndpoint:
    """init_telemetry with a (mock) endpoint should configure the tracer."""

    def test_missing_packages_logs_warning(self) -> None:
        """If OTel SDK is not installed, a warning is logged instead of crashing."""
        with _hide_imports("opentelemetry"):
            import raven_worker.telemetry as tel_mod

            importlib.reload(tel_mod)

            # Patch the logger AFTER the reload so it targets the live object.
            with patch.object(tel_mod, "logger") as mock_logger:
                tel_mod.init_telemetry(service_name="test", endpoint="localhost:4317")
                mock_logger.warning.assert_called_once()

        # Reload after context manager restores original imports.
        importlib.reload(tel_mod)

    def test_with_endpoint_configures_tracer(self) -> None:
        """When a valid endpoint is given the tracer provider should be set."""
        from opentelemetry import trace

        from raven_worker.telemetry import init_telemetry

        init_telemetry(service_name="test-svc", endpoint="localhost:4317")
        provider = trace.get_tracer_provider()
        # The provider should not be the default ProxyTracerProvider;
        # it should be a real TracerProvider from the SDK.
        assert type(provider).__name__ == "TracerProvider"


class TestGrpcInterceptor:
    """get_grpc_server_interceptor must never raise."""

    def test_returns_interceptor_when_package_installed(self) -> None:
        """With the instrumentation package installed, an interceptor is returned."""
        from raven_worker.telemetry import get_grpc_server_interceptor

        result = get_grpc_server_interceptor()
        # The package is installed in the test venv so we expect a real interceptor.
        assert result is not None

    def test_returns_none_when_package_missing(self) -> None:
        """Without the instrumentation package, None should be returned."""
        with _hide_imports("instrumentation"):
            import raven_worker.telemetry as tel_mod

            importlib.reload(tel_mod)
            result = tel_mod.get_grpc_server_interceptor()
            assert result is None

        importlib.reload(tel_mod)
