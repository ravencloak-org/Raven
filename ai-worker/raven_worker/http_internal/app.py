"""FastAPI application factory for the AI worker's internal HTTP endpoints."""

from __future__ import annotations

import os

import structlog
from fastapi import FastAPI

from raven_worker.http_internal.summarize import SummarizeRouter
from raven_worker.http_internal.summarize import build_router as build_summarize_router

logger = structlog.get_logger(__name__)


def build_app() -> FastAPI:
    """Return a FastAPI app with every internal route wired in.

    The Anthropic API key is read from the ``ANTHROPIC_API_KEY`` environment
    variable. When absent the summarize endpoint returns an informative 503
    rather than crashing at import time — the AI worker can still serve gRPC
    without it.
    """
    app = FastAPI(
        title="Raven AI Worker — internal",
        version="1.0.0",
        docs_url=None,  # never expose docs on the internal port
        redoc_url=None,
        openapi_url=None,
    )

    api_key = os.environ.get("ANTHROPIC_API_KEY", "")
    model = os.environ.get("RAVEN_SUMMARY_MODEL", "claude-haiku-4-5")

    app.include_router(build_summarize_router(api_key=api_key, model=model))

    @app.get("/internal/healthz")
    def healthz() -> dict[str, str]:
        return {"status": "ok"}

    return app


__all__ = ["build_app", "SummarizeRouter"]
