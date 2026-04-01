"""Reciprocal Rank Fusion for merging multiple ranked result lists."""

from __future__ import annotations

import structlog

logger = structlog.get_logger(__name__)


def reciprocal_rank_fusion(
    ranked_lists: list[list[tuple[str, float]]],
    k: int = 60,
    top_n: int = 10,
) -> list[tuple[str, float]]:
    """Merge multiple ranked lists using Reciprocal Rank Fusion (RRF).

    RRF formula: ``score(d) = Σ 1 / (k + rank(d))`` where rank is the
    1-based position of document ``d`` in each ranked list.

    Args:
        ranked_lists: A list of ranked result lists.  Each inner list is a
            sequence of ``(item_id, score)`` tuples already sorted by
            relevance descending.  The original scores are discarded; only
            the rank position matters for RRF.
        k: Smoothing constant (default 60, as recommended in the original
            paper by Cormack et al.).
        top_n: Number of top results to return after fusion.

    Returns:
        List of ``(item_id, rrf_score)`` tuples sorted by RRF score
        descending, truncated to ``top_n`` entries.
    """
    rrf_scores: dict[str, float] = {}

    for ranked in ranked_lists:
        for rank_idx, (item_id, _) in enumerate(ranked):
            # rank is 1-based
            rank = rank_idx + 1
            rrf_scores[item_id] = rrf_scores.get(item_id, 0.0) + 1.0 / (k + rank)

    sorted_results = sorted(rrf_scores.items(), key=lambda x: x[1], reverse=True)
    top_results = sorted_results[:top_n]

    logger.debug(
        "rrf_fusion_done",
        input_lists=len(ranked_lists),
        unique_items=len(rrf_scores),
        top_n=top_n,
        returned=len(top_results),
    )
    return top_results
