"""Tests for EmbeddingServicer — the gRPC handler."""

from __future__ import annotations

from unittest.mock import AsyncMock, patch

import grpc
import pytest

from raven_worker.generated import ai_worker_pb2
from raven_worker.services.embedding import EmbeddingServicer


def _make_request(
    text: str = "hello",
    org_id: str = "org-1",
    model: str = "text-embedding-3-small",
    provider: str = "openai",
) -> ai_worker_pb2.EmbeddingRequest:
    return ai_worker_pb2.EmbeddingRequest(
        text=text,
        org_id=org_id,
        model=model,
        provider=provider,
    )


@pytest.fixture()
def servicer() -> EmbeddingServicer:
    return EmbeddingServicer()


# ---------------------------------------------------------------------------
# Happy path
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_embedding_servicer_calls_provider(servicer, grpc_context) -> None:
    """EmbeddingServicer should call get_provider_for_request and return the vector."""
    fake_embedding = [0.1, 0.2, 0.3]
    mock_provider = AsyncMock()
    mock_provider.embed = AsyncMock(return_value=fake_embedding)

    with patch(
        "raven_worker.services.embedding.get_provider_for_request",
        AsyncMock(return_value=mock_provider),
    ):
        response = await servicer.get_embedding(_make_request(), grpc_context)

    # Protobuf stores repeated float as float32, so use approx for comparison
    assert list(response.embedding) == pytest.approx(fake_embedding, rel=1e-5)
    assert response.dimensions == len(fake_embedding)
    grpc_context.abort.assert_not_awaited()


@pytest.mark.asyncio
async def test_embedding_servicer_dimensions_match_vector_length(servicer, grpc_context) -> None:
    """dimensions in the response must equal the actual vector length."""
    vec = [float(i) for i in range(1536)]
    mock_provider = AsyncMock()
    mock_provider.embed = AsyncMock(return_value=vec)

    with patch(
        "raven_worker.services.embedding.get_provider_for_request",
        AsyncMock(return_value=mock_provider),
    ):
        response = await servicer.get_embedding(_make_request(), grpc_context)

    assert response.dimensions == 1536


# ---------------------------------------------------------------------------
# Error paths
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_embedding_servicer_unknown_provider_aborts(servicer, grpc_context) -> None:
    """A ValueError (e.g. unsupported provider) should abort with NOT_FOUND."""
    with patch(
        "raven_worker.services.embedding.get_provider_for_request",
        AsyncMock(side_effect=ValueError("No active 'bad' provider config")),
    ):
        await servicer.get_embedding(_make_request(provider="bad"), grpc_context)

    grpc_context.abort.assert_awaited_once()
    args = grpc_context.abort.call_args.args
    assert args[0] == grpc.StatusCode.NOT_FOUND


@pytest.mark.asyncio
async def test_embedding_servicer_internal_error_aborts(servicer, grpc_context) -> None:
    """An unexpected exception should abort with INTERNAL status."""
    with patch(
        "raven_worker.services.embedding.get_provider_for_request",
        AsyncMock(side_effect=RuntimeError("network error")),
    ):
        await servicer.get_embedding(_make_request(), grpc_context)

    grpc_context.abort.assert_awaited_once()
    args = grpc_context.abort.call_args.args
    assert args[0] == grpc.StatusCode.INTERNAL
    assert "network error" in args[1]
