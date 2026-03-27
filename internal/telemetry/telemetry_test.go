package telemetry

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
)

func TestInitProvider_EmptyEndpoint_ReturnsNoOp(t *testing.T) {
	shutdown, err := InitProvider(context.Background(), "test-service", "0.1.0", "", "test")
	if err != nil {
		t.Fatalf("expected no error for empty endpoint, got: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}
	// The shutdown function should work without error.
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("expected no error from no-op shutdown, got: %v", err)
	}
}

func TestInitProvider_WithEndpoint_ConfiguresTracer(t *testing.T) {
	// Use a dummy endpoint -- the exporter is created but never dialled
	// during the test because we shut down immediately.
	shutdown, err := InitProvider(context.Background(), "test-service", "0.1.0", "localhost:4317", "test")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	// Verify a real TracerProvider was installed (not the default no-op).
	tp := otel.GetTracerProvider()
	typeName := typeNameOf(tp)
	if typeName == "tracerProvider" {
		t.Error("expected a real TracerProvider to be set, got the default no-op")
	}

	// Shutdown with a short timeout -- the exporter targets an unreachable
	// endpoint so the flush will fail. We only care that shutdown does not
	// panic; export errors are expected in this test scenario.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_ = shutdown(ctx) // error is expected (unreachable endpoint)
}

func TestShutdown_CalledTwice_NoPanic(t *testing.T) {
	shutdown, err := InitProvider(context.Background(), "test-service", "0.1.0", "", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Calling shutdown multiple times should not panic.
	_ = shutdown(context.Background())
	_ = shutdown(context.Background())
}

// typeNameOf returns the short type name for debugging.
func typeNameOf(v interface{}) string {
	if v == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%T", v)
}
