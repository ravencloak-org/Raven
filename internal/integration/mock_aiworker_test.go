//go:build integration

package integration

import (
	"context"
	"fmt"
	"sync"

	pb "github.com/ravencloak-org/Raven/internal/grpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockAIWorker struct {
	pb.UnimplementedAIWorkerServer

	mu             sync.Mutex
	parseRequests  []*pb.ParseRequest
	failParseEmbed bool
	parseResponses map[string]*pb.ParseResponse
	embeddings     map[string]*pb.EmbeddingResponse
}

func newMockAIWorker() *mockAIWorker {
	return &mockAIWorker{
		parseResponses: make(map[string]*pb.ParseResponse),
		embeddings:     make(map[string]*pb.EmbeddingResponse),
	}
}

func (m *mockAIWorker) ParseAndEmbed(_ context.Context, req *pb.ParseRequest) (*pb.ParseResponse, error) {
	m.mu.Lock()
	m.parseRequests = append(m.parseRequests, req)
	shouldFail := m.failParseEmbed
	m.mu.Unlock()

	if shouldFail {
		return nil, status.Errorf(codes.Internal, "mock: simulated parse failure")
	}

	if resp, ok := m.parseResponses[req.GetDocumentId()]; ok {
		return resp, nil
	}

	return &pb.ParseResponse{
		DocumentId: req.GetDocumentId(),
		ChunkCount: 0,
		Status:     "ready",
	}, nil
}

func (m *mockAIWorker) GetEmbedding(_ context.Context, req *pb.EmbeddingRequest) (*pb.EmbeddingResponse, error) {
	if resp, ok := m.embeddings[req.GetText()]; ok {
		return resp, nil
	}
	return &pb.EmbeddingResponse{
		Embedding:  make([]float32, 1536),
		Dimensions: 1536,
	}, nil
}

func (m *mockAIWorker) QueryRAG(req *pb.RAGRequest, stream grpc.ServerStreamingServer[pb.RAGChunk]) error {
	return stream.Send(&pb.RAGChunk{
		Text:    fmt.Sprintf("Mock response for: %s", req.GetQuery()),
		IsFinal: true,
		Sources: []*pb.Source{},
	})
}

func (m *mockAIWorker) getParseRequests() []*pb.ParseRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	cpy := make([]*pb.ParseRequest, len(m.parseRequests))
	copy(cpy, m.parseRequests)
	return cpy
}
