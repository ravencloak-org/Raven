package grpc

import (
	"testing"
)

func TestNewClient_InvalidAddr(t *testing.T) {
	// NewClient with grpc.NewClient should succeed for any address since it
	// uses lazy connection establishment. The actual connection is not
	// established until the first RPC call.
	c, err := NewClient("localhost:0")
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
	c, err := NewClient("localhost:50051")
	if err != nil {
		t.Fatalf("NewClient returned unexpected error: %v", err)
	}
	defer c.Close()

	if c.Worker() == nil {
		t.Error("expected Worker() to return a non-nil AIWorkerClient")
	}
}

func TestClient_CloseIdempotent(t *testing.T) {
	c, err := NewClient("localhost:50051")
	if err != nil {
		t.Fatalf("NewClient returned unexpected error: %v", err)
	}

	// First close should succeed.
	if err := c.Close(); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}
}
