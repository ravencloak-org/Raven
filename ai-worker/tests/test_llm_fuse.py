"""Tests for the LLMSpendFuse daily $-cap circuit breaker."""

from __future__ import annotations

import fakeredis
import pytest

from raven_worker.llm_fuse import FuseTripped, LLMSpendFuse


@pytest.fixture
def fuse() -> LLMSpendFuse:
    r = fakeredis.FakeRedis()
    return LLMSpendFuse(redis=r, daily_cap_usd=1.0, key_prefix="test:llm")


def test_charge_below_cap_allowed(fuse: LLMSpendFuse) -> None:
    fuse.charge(0.30)
    fuse.charge(0.40)
    assert fuse.spent_today() == pytest.approx(0.70)


def test_charge_over_cap_trips(fuse: LLMSpendFuse) -> None:
    fuse.charge(0.60)
    fuse.charge(0.30)
    with pytest.raises(FuseTripped):
        fuse.charge(0.20)


def test_guard_check_only_does_not_increment(fuse: LLMSpendFuse) -> None:
    fuse.charge(0.60)
    fuse.guard(0.30)  # would still be under cap; no increment
    assert fuse.spent_today() == pytest.approx(0.60)


def test_guard_raises_when_would_exceed(fuse: LLMSpendFuse) -> None:
    fuse.charge(0.80)
    with pytest.raises(FuseTripped):
        fuse.guard(0.30)


def test_spent_today_empty_returns_zero(fuse: LLMSpendFuse) -> None:
    assert fuse.spent_today() == 0.0


def test_charge_sets_ttl_on_counter() -> None:
    r = fakeredis.FakeRedis()
    fuse = LLMSpendFuse(redis=r, daily_cap_usd=10.0, key_prefix="ttl-check")
    fuse.charge(0.10)
    # find the day-keyed counter and verify TTL was set
    keys = list(r.scan_iter("ttl-check:*"))
    assert len(keys) == 1
    ttl = r.ttl(keys[0])
    # TTL should be the 26-hour ceiling we set; allow a slack of 60 seconds.
    assert 26 * 3600 - 60 <= ttl <= 26 * 3600
