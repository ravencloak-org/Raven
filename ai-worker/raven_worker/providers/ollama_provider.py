"""Ollama embedding provider for Raven Local.

Talks to a local Ollama daemon's ``/api/embeddings`` endpoint. Used in
single-user (desktop) mode where embeddings stay on the host.

Implements the :class:`~raven_worker.providers.base.EmbeddingProvider` protocol.
"""

from __future__ import annotations

import httpx
import structlog

from raven_worker.providers.base import EmbeddingProvider

logger = structlog.get_logger(__name__)

# Default model and dimensions for nomic-embed-text — the bundled local
# embedding model spec'd for Raven Local.
_DEFAULT_MODEL = "nomic-embed-text"
_DEFAULT_DIMENSIONS = 768
_DEFAULT_BASE_URL = "http://ollama:11434"
_DEFAULT_TIMEOUT_S = 30.0


class ModelNotPulledError(RuntimeError):
    """Raised when Ollama returns 404 / 'model not found'.

    Callers can map this to a user-facing 'pull this model' affordance
    instead of a generic 5xx.
    """

    def __init__(self, model: str) -> None:
        super().__init__(f"ollama model not pulled: {model}")
        self.model = model


class OllamaEmbeddingProvider:
    """Generate embeddings using a locally-running Ollama daemon.

    No API key is required — connectivity is to a local sidecar within the
    Raven Local docker compose network.

    Implements the :class:`~raven_worker.providers.base.EmbeddingProvider` protocol.
    """

    def __init__(
        self,
        model: str = _DEFAULT_MODEL,
        dimensions: int = _DEFAULT_DIMENSIONS,
        base_url: str | None = None,
    ) -> None:
        self._model = model
        self._dimensions = dimensions
        self._base_url = (base_url or _DEFAULT_BASE_URL).rstrip("/")
        self._client = httpx.AsyncClient(timeout=_DEFAULT_TIMEOUT_S)

    @property
    def model_name(self) -> str:
        return self._model

    @property
    def dimensions(self) -> int:
        return self._dimensions

    async def embed(self, text: str) -> list[float]:
        """Generate an embedding for ``text`` via Ollama's REST API."""
        resp = await self._client.post(
            f"{self._base_url}/api/embeddings",
            json={"model": self._model, "prompt": text},
        )
        if resp.status_code == 404 and "not found" in resp.text.lower():
            logger.warning(
                "ollama_embed_model_not_pulled",
                model=self._model,
                base_url=self._base_url,
            )
            raise ModelNotPulledError(self._model)
        if resp.status_code >= 400:
            logger.error(
                "ollama_embed_failed",
                status=resp.status_code,
                model=self._model,
                base_url=self._base_url,
                body=resp.text[:300],
            )
            raise RuntimeError(
                f"ollama embed failed: status={resp.status_code} body={resp.text[:300]!r}"
            )
        return resp.json()["embedding"]


# Verify the protocol is satisfied at import time so type errors surface
# during test collection, not at first runtime call.
_: EmbeddingProvider = OllamaEmbeddingProvider()
