"""Unit tests for the semantic-cache repository (issue #256 — M9).

The repo wraps asyncpg but doesn't do anything clever with connection state,
so we swap the pool for an in-memory fake that records every executed SQL
statement and returns canned rows. This lets the same tests run in CI under
a few hundred milliseconds without spinning up a Postgres container.

A separate integration test (see tests/retrieval/test_cache_repo_integration.py,
filed as a follow-up sub-issue) will run the same scenarios against a real
pgvector container via testcontainers-python.
"""

from __future__ import annotations

import asyncio
from dataclasses import dataclass
from typing import Any

import pytest

from raven_worker.retrieval.cache_repo import (
    DEFAULT_SIMILARITY_THRESHOLD,
    CacheRepository,
    _vector_literal,
)

# ---------------------------------------------------------------------------
# Fake asyncpg doubles
# ---------------------------------------------------------------------------


@dataclass
class _Call:
    """Record of a single SQL call (method + sql fragment + args)."""

    method: str
    sql: str
    args: tuple[Any, ...]


class _FakeConnection:
    """Minimal asyncpg.Connection stand-in used in unit tests.

    Each method can optionally raise a configured exception so tests can
    exercise the swallow-and-degrade error paths in CacheRepository.
    """

    def __init__(
        self,
        *,
        fetchrow_result: dict | None,
        execute_result: str = "DELETE 0",
        fetchrow_exc: Exception | None = None,
        execute_exc: Exception | None = None,
        fetchval_exc: Exception | None = None,
    ):
        self.calls: list[_Call] = []
        self._fetchrow_result = fetchrow_result
        self._execute_result = execute_result
        self._fetchrow_exc = fetchrow_exc
        self._execute_exc = execute_exc
        self._fetchval_exc = fetchval_exc

    async def execute(self, sql: str, *args: Any) -> str:
        self.calls.append(_Call("execute", sql, args))
        if self._execute_exc is not None:
            raise self._execute_exc
        return self._execute_result

    async def fetchrow(self, sql: str, *args: Any) -> dict | None:
        self.calls.append(_Call("fetchrow", sql, args))
        if self._fetchrow_exc is not None:
            raise self._fetchrow_exc
        return self._fetchrow_result

    async def fetchval(self, sql: str, *args: Any) -> Any:
        self.calls.append(_Call("fetchval", sql, args))
        if self._fetchval_exc is not None:
            raise self._fetchval_exc
        return "cache-id-1"

    def transaction(self):
        conn = self

        class _Tx:
            async def __aenter__(self):
                return conn

            async def __aexit__(self, *exc):
                return False

        return _Tx()


class _FakePool:
    def __init__(self, conn: _FakeConnection):
        self._conn = conn

    def acquire(self):
        conn = self._conn

        class _Acq:
            async def __aenter__(self_inner):  # noqa: N805
                return conn

            async def __aexit__(self_inner, *exc):  # noqa: N805
                return False

        return _Acq()


# ---------------------------------------------------------------------------
# _vector_literal
# ---------------------------------------------------------------------------


def test_vector_literal_formats_brackets_and_precision():
    out = _vector_literal([0.1, 0.25, -0.5])
    assert out.startswith("[") and out.endswith("]")
    # One float per comma-separated slot, 3 floats here.
    assert len(out.strip("[]").split(",")) == 3


def test_vector_literal_empty_raises():
    with pytest.raises(ValueError):
        _vector_literal([])


# ---------------------------------------------------------------------------
# lookup()
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_lookup_returns_cached_answer_on_hit():
    row = {
        "id": "11111111-1111-1111-1111-111111111111",
        "query_text": "what is RAG?",
        "response_text": "Retrieval-augmented generation...",
        "metadata": {"model": "gpt-4o-mini"},
        "similarity": 0.97,
        "hit_count": 3,
        "created_at": "2026-01-01T00:00:00Z",
        "expires_at": "2026-01-08T00:00:00Z",
    }
    conn = _FakeConnection(fetchrow_result=row)
    repo = CacheRepository(_FakePool(conn))  # type: ignore[arg-type]

    got = await repo.lookup(
        org_id="00000000-0000-0000-0000-000000000001",
        kb_id="00000000-0000-0000-0000-000000000002",
        embedding=[0.1, 0.2, 0.3],
        threshold=0.92,
    )
    assert got is not None
    assert got.id == row["id"]
    assert got.answer_text == row["response_text"]
    assert got.similarity == pytest.approx(0.97)
    assert got.metadata == {"model": "gpt-4o-mini"}
    # Lookup returns immediately after SELECT — the hit_count UPDATE is a
    # detached task so we drain the loop before asserting on the call trace.
    # Give the create_task() coroutine a chance to run (set_config + UPDATE).
    for _ in range(4):
        await asyncio.sleep(0)
    methods = [c.method for c in conn.calls]
    # 1st set_config + SELECT inside the lookup transaction,
    # then the detached bump acquires another connection and runs
    # set_config + UPDATE on the fake pool (same fake _conn).
    assert methods == ["execute", "fetchrow", "execute", "execute"]
    assert "set_config" in conn.calls[0].sql
    assert "SELECT" in conn.calls[1].sql.upper()
    assert "set_config" in conn.calls[2].sql
    assert "UPDATE response_cache SET hit_count" in conn.calls[3].sql


@pytest.mark.asyncio
async def test_lookup_returns_none_on_miss():
    conn = _FakeConnection(fetchrow_result=None)
    repo = CacheRepository(_FakePool(conn))  # type: ignore[arg-type]
    got = await repo.lookup(
        org_id="00000000-0000-0000-0000-000000000001",
        kb_id="00000000-0000-0000-0000-000000000002",
        embedding=[0.1, 0.2, 0.3],
    )
    assert got is None
    # No hit_count UPDATE issued on miss.
    methods = [c.method for c in conn.calls]
    assert methods == ["execute", "fetchrow"]


@pytest.mark.asyncio
async def test_lookup_rejects_empty_embedding():
    conn = _FakeConnection(fetchrow_result=None)
    repo = CacheRepository(_FakePool(conn))  # type: ignore[arg-type]
    assert await repo.lookup(org_id="x", kb_id="y", embedding=[]) is None
    assert conn.calls == []


@pytest.mark.asyncio
@pytest.mark.parametrize("bad_threshold", [-0.1, -1e-9, 1.0 + 1e-9, 1.5])
async def test_lookup_rejects_out_of_range_threshold(bad_threshold: float):
    conn = _FakeConnection(fetchrow_result=None)
    repo = CacheRepository(_FakePool(conn))  # type: ignore[arg-type]
    got = await repo.lookup(org_id="x", kb_id="y", embedding=[0.1], threshold=bad_threshold)
    assert got is None
    # Out-of-range threshold must short-circuit before we hit the DB.
    assert conn.calls == []


def test_default_similarity_threshold_is_in_documented_range():
    """The module-level default must stay inside the documented 0.80-0.99 band."""
    assert 0.80 <= DEFAULT_SIMILARITY_THRESHOLD <= 0.99


@pytest.mark.asyncio
async def test_lookup_uses_default_threshold_when_unset():
    """When no threshold is passed, lookup() must bind DEFAULT_SIMILARITY_THRESHOLD."""
    conn = _FakeConnection(fetchrow_result=None)
    repo = CacheRepository(_FakePool(conn))  # type: ignore[arg-type]
    await repo.lookup(
        org_id="00000000-0000-0000-0000-000000000001",
        kb_id="00000000-0000-0000-0000-000000000002",
        embedding=[0.1, 0.2, 0.3],
    )
    # The SELECT is the 2nd call (0 = set_config). Third positional arg is the
    # threshold that gets bound to $3 in _LOOKUP_SQL.
    select_call = next(c for c in conn.calls if c.method == "fetchrow")
    assert select_call.args[2] == DEFAULT_SIMILARITY_THRESHOLD


@pytest.mark.asyncio
async def test_lookup_degrades_to_miss_on_db_error():
    """A DB error during SELECT must return None — cache failures cannot break RAG."""
    conn = _FakeConnection(
        fetchrow_result=None,
        fetchrow_exc=RuntimeError("simulated pool/network failure"),
    )
    repo = CacheRepository(_FakePool(conn))  # type: ignore[arg-type]
    got = await repo.lookup(
        org_id="00000000-0000-0000-0000-000000000001",
        kb_id="00000000-0000-0000-0000-000000000002",
        embedding=[0.1, 0.2, 0.3],
    )
    assert got is None
    # We still attempted the transaction — we just swallowed the error.
    assert any(c.method == "fetchrow" for c in conn.calls)


@pytest.mark.asyncio
async def test_lookup_decodes_json_string_metadata():
    """asyncpg returns jsonb as a string unless a codec is registered; we must decode it."""
    row = {
        "id": "11111111-1111-1111-1111-111111111111",
        "query_text": "q",
        "response_text": "a",
        "metadata": '{"model": "gpt-4o-mini", "provider": "openai"}',
        "similarity": 0.95,
        "hit_count": 1,
        "created_at": "2026-01-01T00:00:00Z",
        "expires_at": "2026-01-08T00:00:00Z",
    }
    conn = _FakeConnection(fetchrow_result=row)
    repo = CacheRepository(_FakePool(conn))  # type: ignore[arg-type]
    got = await repo.lookup(
        org_id="00000000-0000-0000-0000-000000000001",
        kb_id="00000000-0000-0000-0000-000000000002",
        embedding=[0.1, 0.2, 0.3],
    )
    assert got is not None
    assert got.metadata == {"model": "gpt-4o-mini", "provider": "openai"}


@pytest.mark.asyncio
async def test_lookup_falls_back_to_empty_dict_on_malformed_metadata():
    """Malformed JSON in the metadata column must not crash the hit path."""
    row = {
        "id": "11111111-1111-1111-1111-111111111111",
        "query_text": "q",
        "response_text": "a",
        "metadata": "not-json",
        "similarity": 0.95,
        "hit_count": 1,
        "created_at": "2026-01-01T00:00:00Z",
        "expires_at": "2026-01-08T00:00:00Z",
    }
    conn = _FakeConnection(fetchrow_result=row)
    repo = CacheRepository(_FakePool(conn))  # type: ignore[arg-type]
    got = await repo.lookup(
        org_id="00000000-0000-0000-0000-000000000001",
        kb_id="00000000-0000-0000-0000-000000000002",
        embedding=[0.1, 0.2, 0.3],
    )
    assert got is not None
    assert got.metadata == {}


# ---------------------------------------------------------------------------
# store()
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_store_inserts_row_and_returns_id():
    conn = _FakeConnection(fetchrow_result=None)
    repo = CacheRepository(_FakePool(conn))  # type: ignore[arg-type]
    row_id = await repo.store(
        org_id="00000000-0000-0000-0000-000000000001",
        kb_id="00000000-0000-0000-0000-000000000002",
        query_text="hello",
        embedding=[0.1, 0.2],
        answer_text="hi there",
        metadata={"model": "m"},
    )
    assert row_id == "cache-id-1"
    assert any("INSERT INTO response_cache" in c.sql for c in conn.calls)


@pytest.mark.asyncio
async def test_store_noops_on_empty_answer_or_embedding():
    conn = _FakeConnection(fetchrow_result=None)
    repo = CacheRepository(_FakePool(conn))  # type: ignore[arg-type]
    assert (
        await repo.store(org_id="x", kb_id="y", query_text="q", embedding=[], answer_text="a")
        is None
    )
    assert (
        await repo.store(org_id="x", kb_id="y", query_text="q", embedding=[0.1], answer_text="")
        is None
    )
    assert conn.calls == []


# ---------------------------------------------------------------------------
# invalidate_by_kb()
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_invalidate_by_kb_parses_rowcount():
    conn = _FakeConnection(fetchrow_result=None, execute_result="DELETE 5")
    repo = CacheRepository(_FakePool(conn))  # type: ignore[arg-type]
    n = await repo.invalidate_by_kb(
        org_id="00000000-0000-0000-0000-000000000001",
        kb_id="00000000-0000-0000-0000-000000000002",
    )
    assert n == 5
    assert any("DELETE FROM response_cache" in c.sql for c in conn.calls)


@pytest.mark.asyncio
async def test_invalidate_by_kb_tolerates_malformed_tag():
    conn = _FakeConnection(fetchrow_result=None, execute_result="WHATEVER")
    repo = CacheRepository(_FakePool(conn))  # type: ignore[arg-type]
    n = await repo.invalidate_by_kb(org_id="o", kb_id="k")
    assert n == 0
