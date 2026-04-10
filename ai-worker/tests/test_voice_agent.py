"""Tests for the voice agent LiveKit worker module.

The agent.py module is a LiveKit-based voice worker. It uses mocked LiveKit
input since the full LiveKit SDK requires network connections.

Since the agent module relies on livekit-agents (optional dep), we use
module-level mocks to test the configuration and worker options path.
"""

from __future__ import annotations

import sys
from types import ModuleType
from unittest.mock import MagicMock, patch

import pytest

# ---------------------------------------------------------------------------
# Inject livekit stubs so the module can be imported without the real SDK
# ---------------------------------------------------------------------------


def _make_livekit_stubs() -> None:
    """Inject minimal livekit stubs into sys.modules for the agent tests."""
    if "livekit" not in sys.modules:
        lk = ModuleType("livekit")
        lk.agents = ModuleType("livekit.agents")
        lk.agents.AutoSubscribe = MagicMock()
        lk.agents.WorkerOptions = MagicMock()
        lk.agents.cli = ModuleType("livekit.agents.cli")
        lk.agents.cli.run_app = MagicMock()
        lk.agents.voice = ModuleType("livekit.agents.voice")
        lk.agents.voice.AgentSession = MagicMock()

        sys.modules["livekit"] = lk
        sys.modules["livekit.agents"] = lk.agents
        sys.modules["livekit.agents.cli"] = lk.agents.cli
        sys.modules["livekit.agents.voice"] = lk.agents.voice


_make_livekit_stubs()


class TestVoiceAgentWorkerOptions:
    """Test that _create_worker_options builds the correct WorkerOptions."""

    def test_worker_options_created_with_livekit_credentials(self):
        """_create_worker_options must use livekit_url, api_key, api_secret from settings."""
        from livekit.agents import WorkerOptions

        mock_options = MagicMock()
        WorkerOptions.return_value = mock_options

        with patch("raven_worker.agent.settings") as mock_settings:
            mock_settings.livekit_url = "wss://test.livekit.cloud"
            mock_settings.livekit_api_key = "test-api-key"
            mock_settings.livekit_api_secret = "test-api-secret"

            from raven_worker.agent import _create_worker_options

            _create_worker_options()

        WorkerOptions.assert_called_once()
        call_kwargs = WorkerOptions.call_args.kwargs
        assert call_kwargs.get("ws_url") == "wss://test.livekit.cloud"
        assert call_kwargs.get("api_key") == "test-api-key"
        assert call_kwargs.get("api_secret") == "test-api-secret"

    def test_missing_credentials_raises_system_exit(self):
        """_create_worker_options must exit when credentials are missing."""
        with patch("raven_worker.agent.settings") as mock_settings:
            mock_settings.livekit_api_key = ""
            mock_settings.livekit_api_secret = ""
            mock_settings.livekit_url = "wss://test.livekit.cloud"

            from raven_worker.agent import _create_worker_options

            with pytest.raises(SystemExit):
                _create_worker_options()

    def test_entrypoint_function_registered(self):
        """WorkerOptions must receive an entrypoint_fnc callable."""
        from livekit.agents import WorkerOptions

        with patch("raven_worker.agent.settings") as mock_settings:
            mock_settings.livekit_url = "wss://test.livekit.cloud"
            mock_settings.livekit_api_key = "key"
            mock_settings.livekit_api_secret = "secret"

            from raven_worker.agent import _create_worker_options

            _create_worker_options()

        call_kwargs = WorkerOptions.call_args.kwargs
        assert callable(call_kwargs.get("entrypoint_fnc"))

    def test_auto_subscribe_audio_only(self):
        """The worker should subscribe to AUDIO_ONLY tracks."""
        from livekit.agents import AutoSubscribe, WorkerOptions

        with patch("raven_worker.agent.settings") as mock_settings:
            mock_settings.livekit_url = "wss://test.livekit.cloud"
            mock_settings.livekit_api_key = "key"
            mock_settings.livekit_api_secret = "secret"

            from raven_worker.agent import _create_worker_options

            _create_worker_options()

        call_kwargs = WorkerOptions.call_args.kwargs
        assert call_kwargs.get("auto_subscribe") == AutoSubscribe.AUDIO_ONLY


class TestVoiceAgentServe:
    """Test the serve() entry point for the voice agent."""

    def test_serve_calls_configure_logging(self):
        """serve() must configure structured logging before starting the worker."""
        mock_options = MagicMock()
        with (
            patch("raven_worker.agent._configure_logging") as mock_log,
            patch("raven_worker.agent._create_worker_options", return_value=mock_options),
            patch("raven_worker.agent.settings"),
        ):
            from livekit.agents import cli

            with patch.object(cli, "run_app"):
                from raven_worker.agent import serve

                serve()

        mock_log.assert_called_once()

    def test_serve_invokes_run_app(self):
        """serve() must call livekit.agents.cli.run_app with worker options."""
        mock_options = MagicMock()

        with (
            patch("raven_worker.agent._configure_logging"),
            patch("raven_worker.agent._create_worker_options", return_value=mock_options),
            patch("raven_worker.agent.settings"),
        ):
            from livekit.agents import cli

            with patch.object(cli, "run_app") as mock_run:
                from raven_worker.agent import serve

                serve()

            mock_run.assert_called_once_with(mock_options)


class TestVoiceAgentLogging:
    """Test the logging configuration helper."""

    def test_configure_logging_runs_without_error(self):
        """_configure_logging should run without raising any exception."""
        from raven_worker.agent import _configure_logging

        # Should not raise
        _configure_logging()


class TestVoiceAgentMain:
    """Test the main entry point."""

    def test_main_calls_serve(self):
        """main() must delegate to serve()."""
        with patch("raven_worker.agent.serve") as mock_serve:
            from raven_worker.agent import main

            main()

        mock_serve.assert_called_once()
