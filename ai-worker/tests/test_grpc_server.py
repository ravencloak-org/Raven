"""Tests for AIWorkerServicer — the combined gRPC server servicer.

Covers ParseAndEmbed, QueryRAG, and GetEmbedding entry points by
mocking the underlying EmbeddingServicer and RAGServicer delegates.

ParseAndEmbed is currently UNIMPLEMENTED (returns UNIMPLEMENTED abort)
until LiteParse integration is wired in.
"""

from __future__ import annotations

from unittest.mock import AsyncMock, MagicMock, patch

import grpc

from raven_worker.generated import ai_worker_pb2
from tests.conftest import make_embedding_request, make_parse_request, make_rag_request


class TestParseAndEmbed:
    """ParseAndEmbed is currently UNIMPLEMENTED — verify it aborts correctly."""

    async def test_unimplemented_aborts_with_correct_status(self, grpc_context):
        """ParseAndEmbed must abort with UNIMPLEMENTED until LiteParse is wired in."""
        from raven_worker.server import AIWorkerServicer

        servicer = AIWorkerServicer()
        req = make_parse_request(content=b"Test document content")
        await servicer.ParseAndEmbed(req, grpc_context)
        grpc_context.abort.assert_awaited_once()
        status_code = grpc_context.abort.call_args.args[0]
        assert status_code == grpc.StatusCode.UNIMPLEMENTED

    async def test_unimplemented_includes_helpful_message(self, grpc_context):
        """The abort message should mention ParseAndEmbed or LiteParse."""
        from raven_worker.server import AIWorkerServicer

        servicer = AIWorkerServicer()
        req = make_parse_request(content=b"hello")
        await servicer.ParseAndEmbed(req, grpc_context)
        message = grpc_context.abort.call_args.args[1]
        assert "ParseAndEmbed" in message or "LiteParse" in message

    async def test_parse_request_with_pdf_mime_type_aborts(self, grpc_context):
        """ParseAndEmbed should abort even for PDF mime type (not yet implemented)."""
        from raven_worker.server import AIWorkerServicer

        servicer = AIWorkerServicer()
        req = make_parse_request(
            content=b"%PDF-1.4 fake pdf content",
            mime_type="application/pdf",
            file_name="test.pdf",
        )
        await servicer.ParseAndEmbed(req, grpc_context)
        grpc_context.abort.assert_awaited_once()

    async def test_empty_document_aborts(self, grpc_context):
        """ParseAndEmbed with empty content should still abort (UNIMPLEMENTED path)."""
        from raven_worker.server import AIWorkerServicer

        servicer = AIWorkerServicer()
        req = make_parse_request(content=b"")
        await servicer.ParseAndEmbed(req, grpc_context)
        grpc_context.abort.assert_awaited_once()


class TestQueryRAG:
    """QueryRAG delegates to RAGServicer.query — verify the delegation."""

    async def test_query_rag_delegates_to_rag_servicer(self, grpc_context):
        """QueryRAG must delegate to the RAGServicer and yield its chunks."""

        async def _fake_query(req, ctx):
            yield ai_worker_pb2.RAGChunk(text="Token 1", is_final=False)
            yield ai_worker_pb2.RAGChunk(text="", is_final=True)

        with patch("raven_worker.server.RAGServicer") as mock_rag_servicer_cls:
            mock_instance = MagicMock()
            mock_instance.query = _fake_query
            mock_rag_servicer_cls.return_value = mock_instance

            from raven_worker.server import AIWorkerServicer

            servicer = AIWorkerServicer()
            req = make_rag_request(query="What is Raven?")
            chunks = []
            async for chunk in servicer.QueryRAG(req, grpc_context):
                chunks.append(chunk)

        assert len(chunks) == 2
        assert chunks[0].text == "Token 1"
        assert chunks[0].is_final is False
        assert chunks[1].is_final is True

    async def test_query_rag_passes_request_and_context(self, grpc_context):
        """The request and context passed to QueryRAG must reach RAGServicer.query."""
        received_req = []
        received_ctx = []

        async def _fake_query(req, ctx):
            received_req.append(req)
            received_ctx.append(ctx)
            yield ai_worker_pb2.RAGChunk(text="done", is_final=True)

        with patch("raven_worker.server.RAGServicer") as mock_rag_servicer_cls:
            mock_instance = MagicMock()
            mock_instance.query = _fake_query
            mock_rag_servicer_cls.return_value = mock_instance

            from raven_worker.server import AIWorkerServicer

            servicer = AIWorkerServicer()
            req = make_rag_request(query="test query", org_id="org-xyz")
            async for _ in servicer.QueryRAG(req, grpc_context):
                pass

        assert received_req[0].query == "test query"
        assert received_req[0].org_id == "org-xyz"
        assert received_ctx[0] is grpc_context

    async def test_query_rag_empty_result_no_error(self, grpc_context):
        """QueryRAG must handle zero chunks from the delegate gracefully."""

        async def _empty_query(req, ctx):
            return
            yield  # make it an async generator

        with patch("raven_worker.server.RAGServicer") as mock_rag_servicer_cls:
            mock_instance = MagicMock()
            mock_instance.query = _empty_query
            mock_rag_servicer_cls.return_value = mock_instance

            from raven_worker.server import AIWorkerServicer

            servicer = AIWorkerServicer()
            chunks = [c async for c in servicer.QueryRAG(make_rag_request(), grpc_context)]

        assert chunks == []
        grpc_context.abort.assert_not_awaited()

    async def test_query_rag_multiple_token_chunks(self, grpc_context):
        """QueryRAG streams multiple tokens as individual chunks."""

        async def _multi_token_query(req, ctx):
            for token in ["Hello", " world", "!"]:
                yield ai_worker_pb2.RAGChunk(text=token, is_final=False)
            yield ai_worker_pb2.RAGChunk(text="", is_final=True)

        with patch("raven_worker.server.RAGServicer") as mock_rag_servicer_cls:
            mock_instance = MagicMock()
            mock_instance.query = _multi_token_query
            mock_rag_servicer_cls.return_value = mock_instance

            from raven_worker.server import AIWorkerServicer

            servicer = AIWorkerServicer()
            chunks = [c async for c in servicer.QueryRAG(make_rag_request(), grpc_context)]

        assert len(chunks) == 4
        token_chunks = [c for c in chunks if not c.is_final]
        assert [c.text for c in token_chunks] == ["Hello", " world", "!"]


class TestGetEmbedding:
    """GetEmbedding delegates to EmbeddingServicer.get_embedding."""

    async def test_get_embedding_returns_vector(self, grpc_context):
        """GetEmbedding must return the vector produced by EmbeddingServicer."""
        fake_response = ai_worker_pb2.EmbeddingResponse(
            embedding=[0.1] * 1536,
            dimensions=1536,
        )

        with patch("raven_worker.server.EmbeddingServicer") as mock_embed_servicer_cls:
            mock_instance = MagicMock()
            mock_instance.get_embedding = AsyncMock(return_value=fake_response)
            mock_embed_servicer_cls.return_value = mock_instance

            from raven_worker.server import AIWorkerServicer

            servicer = AIWorkerServicer()
            req = make_embedding_request(text="Hello Raven")
            resp = await servicer.GetEmbedding(req, grpc_context)

        assert len(resp.embedding) == 1536
        assert resp.dimensions == 1536

    async def test_get_embedding_passes_request_and_context(self, grpc_context):
        """The request and context must be forwarded to EmbeddingServicer."""
        received = []
        fake_response = ai_worker_pb2.EmbeddingResponse(embedding=[0.5] * 3, dimensions=3)

        async def _capture(req, ctx):
            received.append((req, ctx))
            return fake_response

        with patch("raven_worker.server.EmbeddingServicer") as mock_embed_servicer_cls:
            mock_instance = MagicMock()
            mock_instance.get_embedding = _capture
            mock_embed_servicer_cls.return_value = mock_instance

            from raven_worker.server import AIWorkerServicer

            servicer = AIWorkerServicer()
            req = make_embedding_request(text="capture me", org_id="org-capture")
            await servicer.GetEmbedding(req, grpc_context)

        assert received[0][0].text == "capture me"
        assert received[0][0].org_id == "org-capture"
        assert received[0][1] is grpc_context

    async def test_get_embedding_cohere_dimension(self, grpc_context):
        """GetEmbedding for cohere provider should return 1024-dimension vector."""
        fake_response = ai_worker_pb2.EmbeddingResponse(
            embedding=[0.2] * 1024,
            dimensions=1024,
        )

        with patch("raven_worker.server.EmbeddingServicer") as mock_embed_servicer_cls:
            mock_instance = MagicMock()
            mock_instance.get_embedding = AsyncMock(return_value=fake_response)
            mock_embed_servicer_cls.return_value = mock_instance

            from raven_worker.server import AIWorkerServicer

            servicer = AIWorkerServicer()
            req = make_embedding_request(
                text="Cohere test",
                provider="cohere",
                model="embed-english-v3.0",
            )
            resp = await servicer.GetEmbedding(req, grpc_context)

        assert len(resp.embedding) == 1024
        assert resp.dimensions == 1024
