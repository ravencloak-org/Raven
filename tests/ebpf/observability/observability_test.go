//go:build ebpf

// Package observability_test contains privileged eBPF integration tests for the
// kernel observability collector. Tests require CAP_BPF/CAP_SYS_ADMIN and a
// Linux kernel >= 5.8 with BTF enabled.
package observability_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"

	"github.com/ravencloak-org/Raven/internal/ebpf/observability"
	"github.com/ravencloak-org/Raven/tests/ebpf/helpers"
)

// TestNewCollector_NilMaps verifies the collector gracefully degrades to a
// no-op when no BPF maps are loaded.
func TestNewCollector_NilMaps(t *testing.T) {
	helpers.RequirePrivileged(t)

	mp := noop.NewMeterProvider()
	c, err := observability.NewCollector(mp.Meter("ebpf-test"), nil)
	require.NoError(t, err)
	require.NotNil(t, c)

	assert.NoError(t, c.Close())
}

// TestNewCollector_MetricDescriptors verifies that all five metric instruments
// (cpu_time, net_bytes_in, net_bytes_out, syscall_errors, fd.count) are created
// without error. Uses t.Fatal (not t.Skip) because metadata accuracy is critical.
func TestNewCollector_MetricDescriptors(t *testing.T) {
	helpers.RequirePrivileged(t)

	mp := noop.NewMeterProvider()
	meter := mp.Meter("ebpf-observability-metrics")

	c, err := observability.NewCollector(meter, nil)
	if err != nil {
		t.Fatalf("NewCollector must succeed with noop meter: %v", err)
	}
	if c == nil {
		t.Fatal("NewCollector returned nil collector")
	}

	assert.NoError(t, c.Close())
}

// TestCollector_StartStop_NilMaps verifies Start/Close lifecycle with nil maps.
// The collector must not block or panic.
func TestCollector_StartStop_NilMaps(t *testing.T) {
	helpers.RequirePrivileged(t)

	mp := noop.NewMeterProvider()
	c, err := observability.NewCollector(mp.Meter("ebpf-test"), nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx)

	// Give the goroutine time to start and exit (nil maps = immediate return).
	time.Sleep(50 * time.Millisecond)
	cancel()

	assert.NoError(t, c.Close())
}

// TestCollector_CloseIdempotent verifies Close() can be called multiple times.
func TestCollector_CloseIdempotent(t *testing.T) {
	helpers.RequirePrivileged(t)

	mp := noop.NewMeterProvider()
	c, err := observability.NewCollector(mp.Meter("ebpf-test"), nil)
	require.NoError(t, err)

	assert.NoError(t, c.Close())
	assert.NoError(t, c.Close())
}

// TestCollector_ContextCancellation verifies the collector stops when the
// context is cancelled.
func TestCollector_ContextCancellation(t *testing.T) {
	helpers.RequirePrivileged(t)

	mp := noop.NewMeterProvider()
	c, err := observability.NewCollector(mp.Meter("ebpf-test"), nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	c.Start(ctx)

	// Wait for context to expire.
	<-ctx.Done()
	// Small grace period for goroutine cleanup.
	time.Sleep(50 * time.Millisecond)

	assert.NoError(t, c.Close())
}

// TestNewCollector_WithMaps verifies the collector creates successfully with
// a non-nil Maps struct (even though no real BPF maps are populated).
func TestNewCollector_WithMaps(t *testing.T) {
	helpers.RequirePrivileged(t)

	mp := noop.NewMeterProvider()
	maps := &observability.Maps{}
	c, err := observability.NewCollector(mp.Meter("ebpf-test"), maps)
	if err != nil {
		t.Fatalf("NewCollector with non-nil Maps must succeed: %v", err)
	}
	if c == nil {
		t.Fatal("NewCollector returned nil collector with non-nil Maps")
	}

	// Start and immediately stop — the collect tick should not panic even
	// though the maps have no real BPF backing.
	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx)
	cancel()

	assert.NoError(t, c.Close())
}
