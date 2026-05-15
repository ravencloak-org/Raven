"""Daily LLM spend circuit breaker backed by Redis/Valkey.

The counter is keyed by UTC date and given a 26-hour TTL so the value
expires cleanly even across DST transitions. Reads use ``GET`` (and
parse the returned bytes as a float); writes use ``INCRBYFLOAT``.

Two entry points:

* :meth:`LLMSpendFuse.guard` — check whether charging ``estimated_cost``
  would exceed the cap. Raises :class:`FuseTripped` without mutating
  state. Call this *before* issuing the LLM request.
* :meth:`LLMSpendFuse.charge` — atomically add ``actual_cost`` to the
  counter and raise :class:`FuseTripped` if the new total crossed the
  cap. Call this *after* the LLM response is received, with the real
  observed cost.

Using both gives correct behaviour for bursty concurrent callers: guard
prevents new requests from starting once the cap is near, and charge is
the source-of-truth for the running total.
"""

from __future__ import annotations

from datetime import UTC, datetime
from typing import Protocol


class FuseTripped(Exception):  # noqa: N818 — domain term, "Tripped" reads more naturally than "TrippedError"
    """Raised when the daily spend cap would be exceeded."""


class _Redis(Protocol):
    """Minimal Redis client surface used by LLMSpendFuse."""

    def incrbyfloat(self, key: str, amount: float) -> float: ...
    def expire(self, key: str, seconds: int) -> bool: ...
    def get(self, key: str) -> bytes | None: ...


class LLMSpendFuse:
    """Daily $-cap circuit breaker.

    Args:
        redis: A Redis/Valkey client implementing :class:`_Redis`.
        daily_cap_usd: Hard ceiling in dollars for the current UTC day.
        key_prefix: Base for the counter key. The current ``YYYYMMDD``
            is appended so a new counter starts at UTC midnight.
    """

    # 26 hours covers DST shifts and clock skew while still guaranteeing
    # the counter expires before the next-next day's writes.
    _TTL_SECONDS = 60 * 60 * 26

    def __init__(
        self,
        redis: _Redis,
        daily_cap_usd: float,
        key_prefix: str = "raven:llm:spend",
    ) -> None:
        self.redis = redis
        self.cap = daily_cap_usd
        self.key_prefix = key_prefix

    def _key(self) -> str:
        day = datetime.now(UTC).strftime("%Y%m%d")
        return f"{self.key_prefix}:{day}"

    def spent_today(self) -> float:
        """Return the running total in USD for the current UTC day."""
        raw = self.redis.get(self._key())
        return float(raw) if raw else 0.0

    def guard(self, estimated_cost_usd: float) -> None:
        """Raise :class:`FuseTripped` if charging would exceed the cap.

        Does not mutate state. Safe to call from many concurrent callers
        — only the actual :meth:`charge` is authoritative.
        """
        if self.spent_today() + estimated_cost_usd > self.cap:
            raise FuseTripped(
                f"Daily LLM cap ${self.cap:.2f} would be exceeded "
                f"(spent ${self.spent_today():.4f}, "
                f"+${estimated_cost_usd:.4f})"
            )

    def charge(self, actual_cost_usd: float) -> None:
        """Add ``actual_cost_usd`` to the counter.

        Raises :class:`FuseTripped` if the new total crossed the cap.
        The increment is applied even when the fuse trips — the caller
        already issued the LLM request and was billed.
        """
        new_total = self.redis.incrbyfloat(self._key(), actual_cost_usd)
        self.redis.expire(self._key(), self._TTL_SECONDS)
        if float(new_total) > self.cap:
            raise FuseTripped(
                f"Daily LLM cap ${self.cap:.2f} exceeded "
                f"(total ${new_total:.4f})"
            )
