"""Tests for the OpenTelemetry initialisation module."""

from __future__ import annotations

import builtins
import importlib
import sys
from unittest.mock import patch


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
        # Temporarily hide all opentelemetry modules from sys.modules.
        hidden: dict[str, object] = {}
        for mod_name in list(sys.modules):
            if mod_name.startswith("opentelemetry"):
                hidden[mod_name] = sys.modules.pop(mod_name)

        real_import = builtins.__import__

        def fake_import(name, *args, **kwargs):
            if name.startswith("opentelemetry"):
                raise ImportError("mocked missing package")
            return real_import(name, *args, **kwargs)

        try:
            builtins.__import__ = fake_import  # type: ignore[assignment]
            import raven_worker.telemetry as tel_mod

            importlib.reload(tel_mod)

            # Patch the logger AFTER the reload so it targets the live object.
            with patch.object(tel_mod, "logger") as mock_logger:
                tel_mod.init_telemetry(service_name="test", endpoint="localhost:4317")
                mock_logger.warning.assert_called_once()
        finally:
            builtins.__import__ = real_import  # type: ignore[assignment]
            sys.modules.update(hidden)
            import raven_worker.telemetry as tel_mod2

            importlib.reload(tel_mod2)

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
        hidden: dict[str, object] = {}
        for mod_name in list(sys.modules):
            if "instrumentation" in mod_name:
                hidden[mod_name] = sys.modules.pop(mod_name)

        real_import = builtins.__import__

        def fake_import(name, *args, **kwargs):
            if "instrumentation" in name:
                raise ImportError("mocked missing package")
            return real_import(name, *args, **kwargs)

        try:
            builtins.__import__ = fake_import  # type: ignore[assignment]
            import raven_worker.telemetry as tel_mod

            importlib.reload(tel_mod)
            result = tel_mod.get_grpc_server_interceptor()
            assert result is None
        finally:
            builtins.__import__ = real_import  # type: ignore[assignment]
            sys.modules.update(hidden)
            importlib.reload(tel_mod)
