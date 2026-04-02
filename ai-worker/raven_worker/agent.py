"""LiveKit Agents voice worker.

Runs as a standalone process alongside the gRPC server.
Joins LiveKit rooms and handles bidirectional audio with an AI assistant.

Usage:
    python -m raven_worker.agent
"""

import asyncio

import structlog

from raven_worker.config import settings

logger = structlog.get_logger(__name__)


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
            structlog.get_level_from_name(settings.log_level),
        ),
        context_class=dict,
        logger_factory=structlog.PrintLoggerFactory(),
        cache_logger_on_first_use=True,
    )


async def _create_agent_worker():
    """Create and configure the LiveKit agent worker."""
    try:
        from livekit.agents import AutoSubscribe, WorkerOptions
        from livekit.agents.voice import AgentSession
    except ImportError:
        logger.error(
            "livekit_agents_not_installed",
            hint="Install with: pip install 'livekit-agents>=1.0.0'",
        )
        raise SystemExit(1) from None

    async def _entrypoint(ctx):
        """Called when the agent joins a room."""
        logger.info(
            "agent_joined_room",
            room=ctx.room.name,
            participant=ctx.room.local_participant.identity,
        )

        # Create a voice agent session
        # STT and TTS plugins will be added in issues #59 and #60
        session = AgentSession()

        # TODO (#59): Add STT plugin (Deepgram or faster-whisper)
        # TODO (#60): Add TTS plugin (Cartesia or Piper)
        # TODO: Connect to RAG pipeline via gRPC for knowledge retrieval
        #   channel = grpc.aio.insecure_channel(f"localhost:{settings.grpc_port}")
        #   stub = ai_worker_pb2_grpc.AIWorkerStub(channel)

        await session.start(room=ctx.room, participant=ctx.room.remote_participants.get(0))

        logger.info("agent_session_started", room=ctx.room.name)

    return WorkerOptions(
        entrypoint_fnc=_entrypoint,
        ws_url=settings.livekit_url,
        api_key=settings.livekit_api_key,
        api_secret=settings.livekit_api_secret,
        auto_subscribe=AutoSubscribe.AUDIO_ONLY,
    )


async def serve() -> None:
    """Start the LiveKit agent worker."""
    _configure_logging()

    logger.info(
        "starting_agent_worker",
        livekit_url=settings.livekit_url,
    )

    worker_options = await _create_agent_worker()

    from livekit.agents import cli

    # Use LiveKit's CLI runner which handles graceful shutdown
    cli.run_app(worker_options)


def main() -> None:
    """Entry point for the agent worker."""
    asyncio.run(serve())


if __name__ == "__main__":
    main()
