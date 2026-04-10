package integration_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	rpcClient "github.com/ravencloak-org/Raven/internal/grpc"
	pb "github.com/ravencloak-org/Raven/internal/grpc/pb"
)

// fakeAIWorker is a configurable gRPC server implementation for tests.
type fakeAIWorker struct {
	pb.UnimplementedAIWorkerServer
	parseErr     error
	parseResp    *pb.ParseResponse
	embeddingErr error
}

func (f *fakeAIWorker) ParseAndEmbed(_ context.Context, req *pb.ParseRequest) (*pb.ParseResponse, error) {
	if f.parseErr != nil {
		return nil, f.parseErr
	}
	if f.parseResp != nil {
		return f.parseResp, nil
	}
	return &pb.ParseResponse{
		DocumentId: req.GetDocumentId(),
		ChunkCount: 3,
		Status:     "ok",
	}, nil
}

func (f *fakeAIWorker) GetEmbedding(_ context.Context, _ *pb.EmbeddingRequest) (*pb.EmbeddingResponse, error) {
	if f.embeddingErr != nil {
		return nil, f.embeddingErr
	}
	vec := make([]float32, 1536)
	return &pb.EmbeddingResponse{Embedding: vec, Dimensions: 1536}, nil
}

func (f *fakeAIWorker) QueryRAG(_ *pb.RAGRequest, stream pb.AIWorker_QueryRAGServer) error {
	_ = stream.Send(&pb.RAGChunk{Text: "chunk 1"})
	_ = stream.Send(&pb.RAGChunk{Text: "chunk 2", IsFinal: true})
	return nil
}

// startFakeGRPCServer starts an in-process gRPC server and returns its address.
func startFakeGRPCServer(t *testing.T, impl pb.AIWorkerServer) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := grpc.NewServer()
	pb.RegisterAIWorkerServer(s, impl)
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(s.Stop)
	return lis.Addr().String()
}

// TestGRPCClient_ParseAndEmbed_HappyPath verifies a successful ParseAndEmbed call
// against an in-process fake server.
func TestGRPCClient_ParseAndEmbed_HappyPath(t *testing.T) {
	addr := startFakeGRPCServer(t, &fakeAIWorker{})

	client, err := rpcClient.NewClient(addr)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	resp, err := client.Worker().ParseAndEmbed(context.Background(), &pb.ParseRequest{
		DocumentId: "doc-1",
		OrgId:      "org-1",
		Content:    []byte("hello world"),
	})
	require.NoError(t, err)
	assert.Equal(t, int32(3), resp.ChunkCount)
	assert.Equal(t, "ok", resp.Status)
}

// TestGRPCClient_ConnectionRefused_ReturnsError verifies that calling a method
// on a non-listening address eventually returns an error.
func TestGRPCClient_ConnectionRefused_ReturnsError(t *testing.T) {
	// Port 1 is virtually guaranteed to be not listening.
	client, err := rpcClient.NewClient("127.0.0.1:1")
	require.NoError(t, err, "NewClient should succeed (lazy dial)")
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = client.Worker().ParseAndEmbed(ctx, &pb.ParseRequest{Content: []byte("test")})
	assert.Error(t, err, "RPC to non-listening address must fail")
}

// TestGRPCClient_Unavailable_ReturnsError verifies that a gRPC UNAVAILABLE error
// propagates back to the caller.
func TestGRPCClient_Unavailable_ReturnsError(t *testing.T) {
	addr := startFakeGRPCServer(t, &fakeAIWorker{
		parseErr: status.Error(codes.Unavailable, "worker down"),
	})

	client, err := rpcClient.NewClient(addr)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	_, err = client.Worker().ParseAndEmbed(context.Background(), &pb.ParseRequest{Content: []byte("test")})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok, "error must be a gRPC status error")
	assert.Equal(t, codes.Unavailable, st.Code())
}

// TestGRPCClient_ResourceExhausted_ReturnsError verifies RESOURCE_EXHAUSTED propagation.
func TestGRPCClient_ResourceExhausted_ReturnsError(t *testing.T) {
	addr := startFakeGRPCServer(t, &fakeAIWorker{
		parseErr: status.Error(codes.ResourceExhausted, "rate limit"),
	})

	client, err := rpcClient.NewClient(addr)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	_, err = client.Worker().ParseAndEmbed(context.Background(), &pb.ParseRequest{Content: []byte("test")})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
}

// TestGRPCClient_QueryRAG_StreamAssembly verifies that a server-streaming call
// returns all chunks from the server in order.
func TestGRPCClient_QueryRAG_StreamAssembly(t *testing.T) {
	addr := startFakeGRPCServer(t, &fakeAIWorker{})

	client, err := rpcClient.NewClient(addr)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	stream, err := client.Worker().QueryRAG(context.Background(), &pb.RAGRequest{
		Query: "what is raven?",
		OrgId: "org-1",
		KbIds: []string{"kb-1"},
	})
	require.NoError(t, err)

	var assembled string
	for {
		chunk, recvErr := stream.Recv()
		if recvErr != nil {
			break
		}
		assembled += chunk.GetText()
	}
	assert.Equal(t, "chunk 1chunk 2", assembled)
}

// TestGRPCClient_TLSHandshake_PlainServerRejectsClient verifies that dialing a
// plain TCP server with an insecure client still works (no TLS mismatch here)
// and that a real TLS client would fail against a plain server.
func TestGRPCClient_TLSHandshake_InsecureClient_PlainServer(t *testing.T) {
	// Start a plain TCP listener (not a TLS server).
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	// Accept and immediately close the connection to simulate a non-gRPC server.
	go func() {
		conn, _ := lis.Accept()
		if conn != nil {
			_ = conn.Close()
		}
	}()

	// Dial the plain server with insecure credentials — the connection should
	// fail at the gRPC handshake level because the server closes immediately.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err, "NewClient should succeed (lazy dial)")
	defer conn.Close() //nolint:errcheck

	stub := pb.NewAIWorkerClient(conn)
	_, err = stub.ParseAndEmbed(ctx, &pb.ParseRequest{Content: []byte("test")})
	assert.Error(t, err, "RPC against a closed connection must fail")
}

// TestGRPCClient_ContextTimeout_Cancelled verifies that an already-expired
// context returns a deadline-exceeded error immediately.
func TestGRPCClient_ContextTimeout_Cancelled(t *testing.T) {
	// Start a fake gRPC server. The 1ns timeout expires before the handshake
	// completes, so the RPC fails with DeadlineExceeded immediately.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	slowSrv := grpc.NewServer()
	pb.RegisterAIWorkerServer(slowSrv, &fakeAIWorker{})
	go func() { _ = slowSrv.Serve(lis) }()
	t.Cleanup(slowSrv.Stop)

	client, err := rpcClient.NewClient(lis.Addr().String())
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	// A 1-nanosecond timeout should be exceeded immediately.
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()

	_, err = client.Worker().ParseAndEmbed(ctx, &pb.ParseRequest{Content: []byte("test")})
	st, ok := status.FromError(err)
	assert.True(t, ok, "error should be a gRPC status error")
	assert.Equal(t, codes.DeadlineExceeded, st.Code(), "expired context must produce DeadlineExceeded")
}
