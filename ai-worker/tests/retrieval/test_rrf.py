"""Tests for Reciprocal Rank Fusion (RRF).

The actual API is reciprocal_rank_fusion(ranked_lists, k, top_n) where
ranked_lists is a list of lists of (item_id, score) tuples.
"""

from __future__ import annotations

from raven_worker.retrieval.rrf import reciprocal_rank_fusion


class TestRRFFusion:
    def test_combined_score_higher_for_item_in_both_lists(self):
        """Items appearing in both ranked lists get a higher RRF score."""
        list_a = [("doc-1", 0.9), ("doc-2", 0.7)]
        list_b = [("doc-1", 0.8), ("doc-3", 0.6)]
        fused = reciprocal_rank_fusion([list_a, list_b])
        # doc-1 appears in both lists — should rank highest
        assert fused[0][0] == "doc-1"

    def test_empty_second_list_returns_first_list_items(self):
        """When second list is empty, items from the first list are still returned."""
        list_a = [("doc-a", 0.9), ("doc-b", 0.7)]
        fused = reciprocal_rank_fusion([list_a, []])
        result_ids = [item[0] for item in fused]
        assert "doc-a" in result_ids
        assert "doc-b" in result_ids

    def test_both_empty_returns_empty(self):
        """Both empty lists should yield an empty result."""
        result = reciprocal_rank_fusion([[], []])
        assert result == []

    def test_top_n_limits_result_count(self):
        """top_n should limit the number of returned results."""
        list_a = [(f"doc-{i}", float(10 - i)) for i in range(10)]
        list_b = [(f"doc-{i}", float(10 - i)) for i in range(10)]
        fused = reciprocal_rank_fusion([list_a, list_b], top_n=3)
        assert len(fused) == 3

    def test_scores_are_positive(self):
        """All RRF scores must be strictly positive."""
        list_a = [("a", 0.9), ("b", 0.5)]
        list_b = [("b", 0.8), ("c", 0.3)]
        fused = reciprocal_rank_fusion([list_a, list_b])
        for _, score in fused:
            assert score > 0.0

    def test_results_ordered_by_score_descending(self):
        """RRF results must be sorted by score descending."""
        list_a = [("x", 0.9), ("y", 0.5), ("z", 0.1)]
        list_b = [("y", 0.9), ("x", 0.5), ("z", 0.1)]
        fused = reciprocal_rank_fusion([list_a, list_b])
        scores = [score for _, score in fused]
        assert scores == sorted(scores, reverse=True)

    def test_all_items_included(self):
        """Every item from either list should appear in results (up to top_n)."""
        list_a = [("p", 0.9)]
        list_b = [("q", 0.8)]
        fused = reciprocal_rank_fusion([list_a, list_b], top_n=10)
        result_ids = {item[0] for item in fused}
        assert "p" in result_ids
        assert "q" in result_ids

    def test_k_parameter_affects_scores(self):
        """Different k values should produce different RRF scores."""
        list_a = [("doc-1", 0.9)]
        score_low_k = reciprocal_rank_fusion([list_a], k=1)[0][1]
        score_high_k = reciprocal_rank_fusion([list_a], k=100)[0][1]
        # Lower k → higher score for rank 1 (1/(k+1))
        assert score_low_k > score_high_k

    def test_single_item_list(self):
        """Single-item list should return that one item with a valid RRF score."""
        list_a = [("solo", 1.0)]
        fused = reciprocal_rank_fusion([list_a])
        assert len(fused) == 1
        assert fused[0][0] == "solo"
        assert fused[0][1] > 0.0

    def test_rrf_formula_correctness(self):
        """Verify RRF score formula: sum(1 / (k + rank)) for each list."""
        k = 60
        # doc-1 is rank 1 in list_a and rank 2 in list_b
        list_a = [("doc-1", 0.9), ("doc-2", 0.5)]
        list_b = [("doc-2", 0.9), ("doc-1", 0.5)]
        fused = reciprocal_rank_fusion([list_a, list_b], k=k, top_n=2)
        # Both doc-1 and doc-2 have the same score: 1/(k+1) + 1/(k+2) = 1/(k+2) + 1/(k+1)
        result_dict = dict(fused)
        expected_score = 1.0 / (k + 1) + 1.0 / (k + 2)
        assert abs(result_dict["doc-1"] - expected_score) < 1e-9
        assert abs(result_dict["doc-2"] - expected_score) < 1e-9
