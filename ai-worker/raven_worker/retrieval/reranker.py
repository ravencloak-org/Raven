"""Cohere Rerank API integration for optional re-ranking of retrieved chunks."""

from __future__ import annotations

import structlog

logger = structlog.get_logger(__name__)

_DEFAULT_RERANK_MODEL = "rerank-english-v3.0"


async def cohere_rerank(
    query: str,
    chunks: list[dict],
    api_key: str,
    model: str = _DEFAULT_RERANK_MODEL,
    top_n: int = 5,
) -> list[dict] | None:
    """Re-rank chunks using the Cohere Rerank API.

    Uses ``cohere.AsyncClientV2`` to call the Rerank endpoint.  If the
    ``cohere`` package is not installed or the API call fails, returns
    ``None`` for graceful degradation.

    Args:
        query: The original user query.
        chunks: List of chunk dicts, each containing at minimum:
            ``{"id": str, "content": str, "score": float}``.
        api_key: Cohere BYOK API key.
        model: Cohere rerank model identifier.
        top_n: Number of top results to return.

    Returns:
        Re-ranked list of chunk dicts with updated ``score`` values, or
        ``None`` if reranking is unavailable.
    """
    try:
        import cohere
    except ImportError:
        logger.warning("cohere_rerank_unavailable", reason="cohere package not installed")
        return None

    try:
        client = cohere.AsyncClientV2(api_key=api_key)
        documents = [c["content"] for c in chunks]

        logger.debug(
            "cohere_rerank_start",
            model=model,
            doc_count=len(documents),
            top_n=top_n,
        )

        resp = await client.rerank(
            model=model,
            query=query,
            documents=documents,
            top_n=top_n,
        )

        reranked: list[dict] = []
        for result in resp.results:
            original_chunk = chunks[result.index]
            reranked.append(
                {
                    **original_chunk,
                    "score": float(result.relevance_score),
                }
            )

        logger.debug("cohere_rerank_done", returned=len(reranked))
        return reranked

    except Exception as exc:
        logger.warning("cohere_rerank_failed", error=str(exc))
        return None
