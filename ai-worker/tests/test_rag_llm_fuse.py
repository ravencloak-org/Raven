"""Integration shape test for the LLM $-fuse wiring in services.rag.

We don't actually drive a real LLM call from here — that would require
a real provider client and a real Redis instance. Instead we exercise
:func:`raven_worker.services.rag._get_llm_fuse` against fakeredis and
the cap-related branches in isolation.

The realistic end-to-end behaviour (FuseTripped surfacing as
``grpc.StatusCode.RESOURCE_EXHAUSTED``) is covered by the QueryRAG
handler in :mod:`raven_worker.server` and verified during the manual
smoke pass described in
``docs/runbooks/demo-app-features.md``.
"""

from __future__ import annotations

import fakeredis
import pytest

import raven_worker.services.rag as rag_module
from raven_worker.config import settings
from raven_worker.llm_fuse import FuseTripped


@pytest.fixture
def reset_fuse_singleton(monkeypatch: pytest.MonkeyPatch):
    """Null out the lazy fuse singletons so each test sees a clean construct path.

    Previously this fixture also called ``importlib.reload(rag_module)``, but
    that permanently swapped the module identity — any later test that had
    imported ``services.rag`` at module load time saw stale references and the
    AsyncMock patches in test_rag_service.py ended up wired to the old module.
    The monkeypatched ``setattr`` lines below already reset the singletons that
    the fuse construction reads, so the reload was redundant and harmful.
    """
    monkeypatch.setattr(rag_module, "_llm_fuse_sync_client", None)
    monkeypatch.setattr(rag_module, "_llm_fuse_singleton", None)
    yield rag_module


def test_get_llm_fuse_returns_none_when_cap_zero(
    monkeypatch: pytest.MonkeyPatch,
    reset_fuse_singleton,
) -> None:
    monkeypatch.setattr(settings, "llm_daily_usd_cap", 0.0)
    assert reset_fuse_singleton._get_llm_fuse() is None


def test_get_llm_fuse_constructs_when_cap_positive(
    monkeypatch: pytest.MonkeyPatch,
    reset_fuse_singleton,
) -> None:
    monkeypatch.setattr(settings, "llm_daily_usd_cap", 5.0)
    # Swap in fakeredis before the first call so the constructor uses it.
    fake = fakeredis.FakeRedis()
    monkeypatch.setattr(
        "raven_worker.services.rag.syncredis.from_url",
        lambda _url: fake,
    )

    fuse = reset_fuse_singleton._get_llm_fuse()
    assert fuse is not None
    # Second call returns the same instance (singleton behaviour).
    assert reset_fuse_singleton._get_llm_fuse() is fuse


def test_fuse_trips_at_module_constant(
    monkeypatch: pytest.MonkeyPatch,
    reset_fuse_singleton,
) -> None:
    """A cap just below 1× ``_LLM_REQUEST_COST_USD`` trips on the first guard."""
    monkeypatch.setattr(settings, "llm_daily_usd_cap", 0.01)  # < 0.02 module const
    fake = fakeredis.FakeRedis()
    monkeypatch.setattr(
        "raven_worker.services.rag.syncredis.from_url",
        lambda _url: fake,
    )

    fuse = reset_fuse_singleton._get_llm_fuse()
    assert fuse is not None
    with pytest.raises(FuseTripped):
        fuse.guard(reset_fuse_singleton._LLM_REQUEST_COST_USD)
