package testutil

import (
	"context"
	"io"

	"google.golang.org/grpc"

	pb "github.com/ravencloak-org/Raven/internal/grpc/pb"
)

// StubAIWorker is a deterministic gRPC AI worker stub for tests.
// Each Fn field can be overridden per-test; nil Fn fields use sensible defaults.
type StubAIWorker struct {
	ParseAndEmbedFn func(ctx context.Context, req *pb.ParseRequest) (*pb.ParseResponse, error)
	QueryRAGFn      func(ctx context.Context, req *pb.RAGRequest) ([]*pb.RAGChunk, error)
	GetEmbeddingFn  func(ctx context.Context, req *pb.EmbeddingRequest) (*pb.EmbeddingResponse, error)
}

// ParseAndEmbed implements pb.AIWorkerClient.
func (s *StubAIWorker) ParseAndEmbed(ctx context.Context, req *pb.ParseRequest, _ ...grpc.CallOption) (*pb.ParseResponse, error) {
	if s.ParseAndEmbedFn != nil {
		return s.ParseAndEmbedFn(ctx, req)
	}
	return &pb.ParseResponse{ChunkCount: 3, Status: "ok"}, nil
}

// GetEmbedding implements pb.AIWorkerClient.
func (s *StubAIWorker) GetEmbedding(ctx context.Context, req *pb.EmbeddingRequest, _ ...grpc.CallOption) (*pb.EmbeddingResponse, error) {
	if s.GetEmbeddingFn != nil {
		return s.GetEmbeddingFn(ctx, req)
	}
	vec := make([]float32, 1536)
	return &pb.EmbeddingResponse{Embedding: vec, Dimensions: 1536}, nil
}

// QueryRAG implements pb.AIWorkerClient.
// Returns a StubRAGStream that yields 2 deterministic text chunks.
func (s *StubAIWorker) QueryRAG(ctx context.Context, req *pb.RAGRequest, _ ...grpc.CallOption) (pb.AIWorker_QueryRAGClient, error) {
	var chunks []*pb.RAGChunk
	if s.QueryRAGFn != nil {
		var err error
		chunks, err = s.QueryRAGFn(ctx, req)
		if err != nil {
			return nil, err
		}
	} else {
		chunks = []*pb.RAGChunk{
			{Text: "First chunk of the answer"},
			{Text: " second chunk", IsFinal: true},
		}
	}
	return &StubRAGStream{chunks: chunks}, nil
}

// StubRAGStream is a minimal server-streaming client stub.
type StubRAGStream struct {
	chunks []*pb.RAGChunk
	idx    int
	grpc.ClientStream
}

// Recv returns the next chunk or io.EOF when exhausted.
func (s *StubRAGStream) Recv() (*pb.RAGChunk, error) {
	if s.idx >= len(s.chunks) {
		return nil, io.EOF
	}
	c := s.chunks[s.idx]
	s.idx++
	return c, nil
}
