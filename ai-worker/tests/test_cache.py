"""Tests for the RAG response cache integration."""

from __future__ import annotations

import hashlib

from raven_worker.services.rag import _CACHE_KEY_PREFIX, _cache_key

# ---------------------------------------------------------------------------
# _cache_key unit tests (pure Python — no Redis)
# ---------------------------------------------------------------------------


def test_cache_key_deterministic() -> None:
    """Same inputs must always produce the same cache key."""
    key1 = _cache_key("kb-123", "What is RAG?")
    key2 = _cache_key("kb-123", "What is RAG?")
    assert key1 == key2


def test_cache_key_different_kb() -> None:
    """Different KB IDs must produce different keys."""
    key1 = _cache_key("kb-1", "query")
    key2 = _cache_key("kb-2", "query")
    assert key1 != key2


def test_cache_key_different_query() -> None:
    """Different queries must produce different keys."""
    key1 = _cache_key("kb-1", "What is RAG?")
    key2 = _cache_key("kb-1", "How does RAG work?")
    assert key1 != key2


def test_cache_key_normalizes_query() -> None:
    """Cache key should normalize whitespace and case."""
    key1 = _cache_key("kb-1", "Hello World")
    key2 = _cache_key("kb-1", "  hello world  ")
    key3 = _cache_key("kb-1", "HELLO WORLD")
    assert key1 == key2 == key3


def test_cache_key_prefix() -> None:
    """Cache key must start with the expected prefix."""
    key = _cache_key("kb-1", "test query")
    assert key.startswith(_CACHE_KEY_PREFIX)


def test_cache_key_matches_go_format() -> None:
    """Cache key must match the Go-side sha256(kb_id:normalised_query) format.

    The Go implementation computes:
        sha256("kb_id:" + strings.ToLower(strings.TrimSpace(query)))
    and prepends "raven:cache:rag:".
    """
    kb_id = "kb-abc"
    query = "  What is RAG?  "
    normalized = query.lower().strip()
    expected_hash = hashlib.sha256(f"{kb_id}:{normalized}".encode()).hexdigest()
    expected_key = f"{_CACHE_KEY_PREFIX}{expected_hash}"
    assert _cache_key(kb_id, query) == expected_key
