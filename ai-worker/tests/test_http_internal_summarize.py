"""Tests for the /internal/summarize FastAPI endpoint.

Covers the deterministic helpers (subject-line builder, bullet parser) and
exercises the router with a stubbed Anthropic client so the happy path,
missing-messages, empty-key, and upstream-error branches are all covered
without hitting the real API.
"""

from __future__ import annotations

from typing import Any

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from raven_worker.http_internal import app as app_module
from raven_worker.http_internal.summarize import (
    _build_subject,
    _parse_bullets,
    build_router,
)


# ---------------------------------------------------------------------------
# pure-function helpers
# ---------------------------------------------------------------------------


class TestParseBullets:
    def test_valid_json_array(self) -> None:
        raw = '["You asked about A", "We covered B", "You learned C"]'
        assert _parse_bullets(raw) == [
            "You asked about A",
            "We covered B",
            "You learned C",
        ]

    def test_json_array_in_code_fence(self) -> None:
        raw = '```json\n["one", "two"]\n```'
        assert _parse_bullets(raw) == ["one", "two"]

    def test_json_array_bare_code_fence(self) -> None:
        raw = '```\n["alpha"]\n```'
        assert _parse_bullets(raw) == ["alpha"]

    def test_json_array_clamped_to_five(self) -> None:
        raw = '["a", "b", "c", "d", "e", "f", "g"]'
        assert _parse_bullets(raw) == ["a", "b", "c", "d", "e"]

    def test_non_list_json_falls_back(self) -> None:
        # A single string is valid JSON but not a list — fallback to split path.
        assert _parse_bullets('"just a string"') == ['"just a string"']

    def test_plain_bulleted_fallback(self) -> None:
        raw = "- first thing\n- second thing\n* third thing"
        assert _parse_bullets(raw) == ["first thing", "second thing", "third thing"]

    def test_numbered_fallback(self) -> None:
        raw = "1. alpha\n2. beta\n3. gamma"
        assert _parse_bullets(raw) == ["alpha", "beta", "gamma"]

    def test_empty_string(self) -> None:
        assert _parse_bullets("") == []

    def test_malformed_json_falls_through_to_lines(self) -> None:
        raw = "not json {broken\nalso not json"
        assert _parse_bullets(raw) == ["not json {broken", "also not json"]

    def test_strips_blank_entries(self) -> None:
        raw = '["a", "", "b", "   "]'
        assert _parse_bullets(raw) == ["a", "b"]


class TestBuildSubject:
    def test_chat_with_kb(self) -> None:
        assert _build_subject("chat", "Acme Docs") == "Your chat session recap — Acme Docs"

    def test_voice_no_kb(self) -> None:
        assert _build_subject("voice", "") == "Your voice call recap"

    def test_webrtc_maps_to_voice(self) -> None:
        assert _build_subject("webrtc", "Foo") == "Your voice call recap — Foo"

    def test_unknown_channel_defaults_to_session(self) -> None:
        assert _build_subject("smoke-signal", "") == "Your session recap"


# ---------------------------------------------------------------------------
# router: happy path, 400 empty, 503 no-key, 502 upstream, 500 no-bullets
# ---------------------------------------------------------------------------


class _StubBlock:
    def __init__(self, text: str) -> None:
        self.text = text


class _StubMessage:
    def __init__(self, text: str) -> None:
        self.content = [_StubBlock(text)]


class _StubClient:
    """Minimal anthropic.Anthropic stand-in."""

    def __init__(self, response: str | Exception) -> None:
        self._response = response

        class _Messages:
            def __init__(outer, resp: str | Exception) -> None:  # noqa: N805
                outer._resp = resp

            def create(outer, **_kwargs: Any) -> _StubMessage:  # noqa: N805
                if isinstance(outer._resp, Exception):
                    raise outer._resp
                return _StubMessage(outer._resp)

        self.messages = _Messages(response)


def _mount(client_factory: Any) -> TestClient:
    """Build a FastAPI app whose summarize endpoint uses the given client."""
    app = FastAPI()
    router = build_router(api_key="test-key", model="claude-haiku-4-5")
    # Replace the Depends function with our stub. The router was built with a
    # closure, so we override via FastAPI's dependency_overrides.
    router_deps = next(iter(router.routes))
    dep = router_deps.dependant.dependencies[0].call  # type: ignore[attr-defined]
    app.include_router(router)
    app.dependency_overrides[dep] = client_factory
    return TestClient(app)


def _payload() -> dict[str, Any]:
    return {
        "session_id": "11111111-1111-1111-1111-111111111111",
        "channel": "chat",
        "kb_name": "Acme Docs",
        "user_name": "Jobin",
        "messages": [
            {"role": "user", "content": "How do refunds work?"},
            {"role": "assistant", "content": "Refunds are processed within 7 days."},
        ],
    }


class TestSummarizeRouter:
    def test_happy_path(self) -> None:
        raw = '["You asked about refunds.", "We covered the 7-day window."]'
        client = _mount(lambda: _StubClient(raw))
        res = client.post("/internal/summarize", json=_payload())
        assert res.status_code == 200, res.text
        body = res.json()
        assert body["subject"] == "Your chat session recap — Acme Docs"
        assert body["bullets"] == [
            "You asked about refunds.",
            "We covered the 7-day window.",
        ]

    def test_empty_messages_returns_400(self) -> None:
        client = _mount(lambda: _StubClient("irrelevant"))
        payload = _payload()
        payload["messages"] = []
        res = client.post("/internal/summarize", json=payload)
        assert res.status_code == 400
        assert "empty" in res.json()["detail"].lower()

    def test_upstream_error_returns_502(self) -> None:
        client = _mount(lambda: _StubClient(RuntimeError("rate limited")))
        res = client.post("/internal/summarize", json=_payload())
        assert res.status_code == 502
        assert "upstream" in res.json()["detail"].lower()

    def test_no_bullets_falls_back_to_single(self) -> None:
        client = _mount(lambda: _StubClient(""))
        res = client.post("/internal/summarize", json=_payload())
        assert res.status_code == 200
        bullets = res.json()["bullets"]
        assert len(bullets) == 1
        assert bullets[0]  # non-empty

    def test_missing_api_key_returns_503(self) -> None:
        # Build a router with no key and call without overriding dependencies.
        empty_router = build_router(api_key="", model="claude-haiku-4-5")
        app = FastAPI()
        app.include_router(empty_router)
        res = TestClient(app).post("/internal/summarize", json=_payload())
        assert res.status_code == 503
        assert "ANTHROPIC_API_KEY" in res.json()["detail"]


# ---------------------------------------------------------------------------
# app factory wiring
# ---------------------------------------------------------------------------


class TestAppFactory:
    def test_healthz(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.setenv("ANTHROPIC_API_KEY", "")
        app = app_module.build_app()
        res = TestClient(app).get("/internal/healthz")
        assert res.status_code == 200
        assert res.json() == {"status": "ok"}

    def test_docs_are_disabled(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.setenv("ANTHROPIC_API_KEY", "")
        app = app_module.build_app()
        client = TestClient(app)
        assert client.get("/docs").status_code == 404
        assert client.get("/redoc").status_code == 404
        assert client.get("/openapi.json").status_code == 404

    def test_model_env_override(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.setenv("ANTHROPIC_API_KEY", "")
        monkeypatch.setenv("RAVEN_SUMMARY_MODEL", "custom-model")
        # build_app should not error even with an exotic model name
        app = app_module.build_app()
        assert app is not None

    def test_build_app_runs_without_anthropic_key(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.delenv("ANTHROPIC_API_KEY", raising=False)
        app = app_module.build_app()
        # Posting to summarize without a key must 503, proving the endpoint
        # responded instead of crashing.
        res = TestClient(app).post("/internal/summarize", json=_payload())
        assert res.status_code == 503
