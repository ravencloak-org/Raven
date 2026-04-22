"""Internal HTTP (FastAPI) endpoints served alongside the gRPC server.

These endpoints are intended for the Go API/worker only — they are *not*
exposed to the public internet. Network-level access control is enforced
by the ingress/gateway layer; the service does not re-authenticate.

Routes:
    POST /internal/summarize   (#257 post-session email summaries)
    GET  /internal/healthz
"""

from __future__ import annotations

from raven_worker.http_internal.app import build_app

__all__ = ["build_app"]
