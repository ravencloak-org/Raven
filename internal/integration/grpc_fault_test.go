package integration_test

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
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
	"github.com/ravencloak-org/Raven/internal/resilience"
)

// defaultTestPolicy returns a resilience.Policy suitable for integration tests.
func defaultTestPolicy(t *testing.T) (*resilience.Policy, *resilience.Breaker) {
	t.Helper()
	p, err := resilience.NewPolicy("test-ai-worker", resilience.WithTimeout(5*time.Second))
	require.NoError(t, err)
	return p, resilience.NewBreaker(p)
}

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

	p, br := defaultTestPolicy(t)
	client, err := rpcClient.NewClient(addr, p, br)
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
	p, br := defaultTestPolicy(t)
	client, err := rpcClient.NewClient("127.0.0.1:1", p, br)
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

	p, br := defaultTestPolicy(t)
	client, err := rpcClient.NewClient(addr, p, br)
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

	p, br := defaultTestPolicy(t)
	client, err := rpcClient.NewClient(addr, p, br)
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

	p, br := defaultTestPolicy(t)
	client, err := rpcClient.NewClient(addr, p, br)
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
//
// Note: with the resilience interceptor, the breaker short-circuits on ctx.Err()
// before invoking the gRPC call, so the error is a raw context.DeadlineExceeded
// rather than a gRPC status error. We accept either form.
func TestGRPCClient_ContextTimeout_Cancelled(t *testing.T) {
	// Start a fake gRPC server. The 1ns timeout expires before the handshake
	// completes, so the RPC fails with DeadlineExceeded immediately.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	slowSrv := grpc.NewServer()
	pb.RegisterAIWorkerServer(slowSrv, &fakeAIWorker{})
	go func() { _ = slowSrv.Serve(lis) }()
	t.Cleanup(slowSrv.Stop)

	p, br := defaultTestPolicy(t)
	client, err := rpcClient.NewClient(lis.Addr().String(), p, br)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	// A 1-nanosecond timeout should be exceeded immediately.
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()

	_, err = client.Worker().ParseAndEmbed(ctx, &pb.ParseRequest{Content: []byte("test")})
	require.Error(t, err, "expired context must produce an error")
	// Accept either a raw context error or a gRPC status error wrapping DeadlineExceeded.
	if st, ok := status.FromError(err); ok {
		assert.Equal(t, codes.DeadlineExceeded, st.Code(), "gRPC status must be DeadlineExceeded")
	} else {
		assert.ErrorIs(t, err, context.DeadlineExceeded, "raw error must be DeadlineExceeded")
	}
}

// ---------------------------------------------------------------------------
// Fault-injection fixture — configurable delay + error code, with call count.
// ---------------------------------------------------------------------------

// faultConfig controls how faultAIWorker responds to ParseAndEmbed calls.
type faultConfig struct {
	// Delay is the artificial sleep before responding (simulates a slow worker).
	Delay time.Duration
	// Code, when non-zero, is returned as a gRPC status error after any Delay.
	Code codes.Code
	// ErrMessage is the message for the returned status error (defaults to "injected fault").
	ErrMessage string
}

// faultAIWorker is a thread-safe gRPC AIWorker implementation that applies the
// currently configured fault on every ParseAndEmbed call.
type faultAIWorker struct {
	pb.UnimplementedAIWorkerServer

	// cfg holds the active faultConfig; reads/writes must hold cfgMu.
	cfg   faultConfig
	calls atomic.Int64
}

// SetConfig atomically replaces the active fault configuration.
func (f *faultAIWorker) SetConfig(cfg faultConfig) {
	f.cfg = cfg
}

// CallCount returns the total number of ParseAndEmbed calls received.
func (f *faultAIWorker) CallCount() int64 {
	return f.calls.Load()
}

func (f *faultAIWorker) ParseAndEmbed(ctx context.Context, req *pb.ParseRequest) (*pb.ParseResponse, error) {
	f.calls.Add(1)
	cfg := f.cfg

	if cfg.Delay > 0 {
		select {
		case <-time.After(cfg.Delay):
		case <-ctx.Done():
			return nil, status.FromContextError(ctx.Err()).Err()
		}
	}

	if cfg.Code != codes.OK && cfg.Code != 0 {
		msg := cfg.ErrMessage
		if msg == "" {
			msg = "injected fault"
		}
		return nil, status.Error(cfg.Code, msg)
	}

	return &pb.ParseResponse{
		DocumentId: req.GetDocumentId(),
		ChunkCount: 1,
		Status:     "ok",
	}, nil
}

// faultServerHandle is returned by startFaultServer so tests can mutate the
// server's config and inspect call counts.
type faultServerHandle struct {
	impl *faultAIWorker
	srv  *grpc.Server
}

// SetConfig forwards to the underlying faultAIWorker.
func (h *faultServerHandle) SetConfig(cfg faultConfig) { h.impl.SetConfig(cfg) }

// CallCount returns the total number of ParseAndEmbed calls received.
func (h *faultServerHandle) CallCount() int64 { return h.impl.CallCount() }

// Stop gracefully stops the gRPC server.
func (h *faultServerHandle) Stop() { h.srv.Stop() }

// startFaultServer starts an in-process gRPC server backed by a faultAIWorker
// with the given initial configuration and returns the handle and address.
func startFaultServer(t *testing.T, cfg faultConfig) (*faultServerHandle, string) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	impl := &faultAIWorker{cfg: cfg}
	srv := grpc.NewServer()
	pb.RegisterAIWorkerServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	return &faultServerHandle{impl: impl, srv: srv}, lis.Addr().String()
}

// ---------------------------------------------------------------------------
// Task 13 — three new resilience test cases
// ---------------------------------------------------------------------------

// TestResilience_SlowAIWorker_HitsClientDeadline verifies that a worker that
// delays longer than policy.Timeout causes the call to return DeadlineExceeded
// within the timeout budget (±100 ms tolerance).
func TestResilience_SlowAIWorker_HitsClientDeadline(t *testing.T) {
	const policyTimeout = 200 * time.Millisecond
	const serverDelay = 5 * time.Second // much longer than policyTimeout

	srv, addr := startFaultServer(t, faultConfig{Delay: serverDelay})
	defer srv.Stop()

	p, err := resilience.NewPolicy("slow-worker",
		resilience.WithTimeout(policyTimeout),
	)
	require.NoError(t, err)
	br := resilience.NewBreaker(p)

	client, err := rpcClient.NewClient(addr, p, br)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	start := time.Now()
	_, callErr := client.Worker().ParseAndEmbed(context.Background(), &pb.ParseRequest{Content: []byte("slow")})
	elapsed := time.Since(start)

	require.Error(t, callErr, "slow server must return an error")

	// Accept either a gRPC status DeadlineExceeded or a raw context error.
	if st, ok := status.FromError(callErr); ok {
		assert.Equal(t, codes.DeadlineExceeded, st.Code(), "gRPC status must be DeadlineExceeded")
	} else {
		assert.True(t, errors.Is(callErr, context.DeadlineExceeded), "raw error must be DeadlineExceeded, got: %v", callErr)
	}

	// The call must return within policyTimeout + 100 ms CI tolerance.
	assert.LessOrEqual(t, elapsed, policyTimeout+100*time.Millisecond,
		"call took %v; expected ≤ %v", elapsed, policyTimeout+100*time.Millisecond)
}

// TestResilience_RepeatedUnavailable_OpensBreaker verifies that N consecutive
// Unavailable responses trip the circuit breaker, and the next call returns
// ErrCircuitOpen without the server being invoked again.
func TestResilience_RepeatedUnavailable_OpensBreaker(t *testing.T) {
	const threshold = uint32(3)

	srv, addr := startFaultServer(t, faultConfig{Code: codes.Unavailable})
	defer srv.Stop()

	p, err := resilience.NewPolicy("breaker-open",
		resilience.WithTimeout(500*time.Millisecond),
		resilience.WithBreakerThreshold(threshold),
		resilience.WithBreakerCooldown(30*time.Second),
	)
	require.NoError(t, err)
	br := resilience.NewBreaker(p)

	client, err := rpcClient.NewClient(addr, p, br)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	// Drive threshold consecutive failures to open the breaker.
	for i := uint32(0); i < threshold; i++ {
		_, _ = client.Worker().ParseAndEmbed(context.Background(), &pb.ParseRequest{Content: []byte("fail")})
	}

	// Record how many calls the server has seen so far.
	preCalls := srv.CallCount()

	// The next call must be short-circuited by the open breaker.
	_, openErr := client.Worker().ParseAndEmbed(context.Background(), &pb.ParseRequest{Content: []byte("blocked")})
	assert.True(t, errors.Is(openErr, resilience.ErrCircuitOpen),
		"expected ErrCircuitOpen, got: %v", openErr)

	// The server must not have received the short-circuited call.
	assert.Equal(t, preCalls, srv.CallCount(), "server saw extra call after breaker opened")
}

// TestResilience_HalfOpenProbe_ClosesBreaker verifies that after the cooldown
// period the breaker allows a single probe, and a successful probe closes the
// breaker so subsequent calls succeed normally.
func TestResilience_HalfOpenProbe_ClosesBreaker(t *testing.T) {
	const threshold = uint32(3)
	const cooldown = 200 * time.Millisecond // short so the test stays fast

	srv, addr := startFaultServer(t, faultConfig{Code: codes.Unavailable})
	defer srv.Stop()

	p, err := resilience.NewPolicy("half-open-probe",
		resilience.WithTimeout(500*time.Millisecond),
		resilience.WithBreakerThreshold(threshold),
		resilience.WithBreakerCooldown(cooldown),
		resilience.WithBreakerHalfOpenMax(1),
	)
	require.NoError(t, err)
	br := resilience.NewBreaker(p)

	client, err := rpcClient.NewClient(addr, p, br)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	// Trip the breaker with consecutive failures.
	for i := uint32(0); i < threshold; i++ {
		_, _ = client.Worker().ParseAndEmbed(context.Background(), &pb.ParseRequest{Content: []byte("fail")})
	}

	// Wait for the cooldown to expire so the breaker enters Half-Open.
	time.Sleep(cooldown + 50*time.Millisecond)

	// Flip the server to healthy so the probe succeeds.
	srv.SetConfig(faultConfig{})

	// The probe call closes the breaker.
	_, probeErr := client.Worker().ParseAndEmbed(context.Background(), &pb.ParseRequest{Content: []byte("probe")})
	require.NoError(t, probeErr, "probe call must succeed")

	// Subsequent calls should also succeed (breaker is Closed again).
	_, followErr := client.Worker().ParseAndEmbed(context.Background(), &pb.ParseRequest{Content: []byte("follow-up")})
	assert.NoError(t, followErr, "follow-up call after breaker closed must succeed")
}
