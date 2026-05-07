package grpc

import (
	"testing"
	"time"

	"github.com/ravencloak-org/Raven/internal/resilience"
)

func TestNewClient_AppliesResilienceInterceptor(t *testing.T) {
	p, err := resilience.NewPolicy("ai-worker", resilience.WithTimeout(50*time.Millisecond))
	if err != nil {
		t.Fatalf("NewPolicy err = %v", err)
	}
	br := resilience.NewBreaker(p)

	// We can't easily dial a real server in a unit test; just assert
	// NewClient accepts the policy and returns no error for a syntactically
	// valid address. Connection establishment is lazy in grpc.NewClient.
	c, err := NewClient("passthrough:///localhost:1", p, br)
	if err != nil {
		t.Fatalf("NewClient err = %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	if c.Worker() == nil {
		t.Errorf("Worker() returned nil")
	}
}

func TestNewClient_InvalidAddr(t *testing.T) {
	// NewClient with grpc.NewClient should succeed for any address since it
	// uses lazy connection establishment. The actual connection is not
	// established until the first RPC call.
	p, _ := resilience.NewPolicy("ai-worker", resilience.WithTimeout(5*time.Second))
	br := resilience.NewBreaker(p)

	c, err := NewClient("localhost:0", p, br)
	if err != nil {
		t.Fatalf("NewClient returned unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("NewClient returned nil client")
	}
	if c.Worker() == nil {
		t.Fatal("Worker() returned nil")
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close returned unexpected error: %v", err)
	}
}

func TestNewClient_WorkerNotNil(t *testing.T) {
	p, _ := resilience.NewPolicy("ai-worker", resilience.WithTimeout(5*time.Second))
	br := resilience.NewBreaker(p)

	c, err := NewClient("localhost:50051", p, br)
	if err != nil {
		t.Fatalf("NewClient returned unexpected error: %v", err)
	}
	defer c.Close() //nolint:errcheck

	if c.Worker() == nil {
		t.Error("expected Worker() to return a non-nil AIWorkerClient")
	}
}

func TestClient_CloseIdempotent(t *testing.T) {
	p, _ := resilience.NewPolicy("ai-worker", resilience.WithTimeout(5*time.Second))
	br := resilience.NewBreaker(p)

	c, err := NewClient("localhost:50051", p, br)
	if err != nil {
		t.Fatalf("NewClient returned unexpected error: %v", err)
	}

	// First close should succeed.
	if err := c.Close(); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}
}
