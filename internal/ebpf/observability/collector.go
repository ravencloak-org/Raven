//go:build linux

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 Observability ../programs/observability.c

// Package observability implements Feature #122: kernel-level metrics via eBPF.
// When maps is nil (no BPF objects loaded), the collector is a no-op —
// safe on kernels without eBPF support.
package observability

import (
	"context"
	"io"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/metric"
)

// Maps holds the BPF map handles needed by the collector.
// When nil, the collector runs as a no-op.
type Maps struct{}

// Collector polls BPF maps on a configurable interval and reports OTel metrics.
type Collector struct {
	meter    metric.Meter
	maps     *Maps
	interval time.Duration
	stop     chan struct{}
	done     chan struct{}

	cpuTime     metric.Int64Counter
	netBytesIn  metric.Int64Counter
	netBytesOut metric.Int64Counter
	syscallErrs metric.Int64Counter
}

// NewCollector creates a Collector that reports metrics via the given Meter.
// Pass nil maps to get a no-op collector (graceful degrade).
func NewCollector(meter metric.Meter, maps *Maps) (*Collector, error) {
	c := &Collector{
		meter:    meter,
		maps:     maps,
		interval: 15 * time.Second,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}

	var err error
	c.cpuTime, err = meter.Int64Counter(
		"ebpf.process.cpu_time",
		metric.WithDescription("Accumulated CPU time per process (ms)"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}
	c.netBytesIn, err = meter.Int64Counter(
		"ebpf.net.bytes_in",
		metric.WithDescription("Network bytes received per PID"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}
	c.netBytesOut, err = meter.Int64Counter(
		"ebpf.net.bytes_out",
		metric.WithDescription("Network bytes sent per PID"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}
	c.syscallErrs, err = meter.Int64Counter(
		"ebpf.syscall.errors",
		metric.WithDescription("Syscall error rate by syscall number"),
		metric.WithUnit("{count}"),
	)
	if err != nil {
		return nil, err
	}
	fdGauge, err := meter.Int64ObservableGauge(
		"ebpf.fd.count",
		metric.WithDescription("Number of open file descriptors per process"),
		metric.WithUnit("{fd}"),
	)
	if err != nil {
		return nil, err
	}

	_, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		// Stub: real implementation reads from BPF fd_count_map
		return nil
	}, fdGauge)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// Start begins polling BPF maps in a background goroutine.
// Safe to call when maps is nil (no-op).
func (c *Collector) Start(ctx context.Context) {
	go c.run(ctx)
}

func (c *Collector) run(ctx context.Context) {
	defer close(c.done)
	if c.maps == nil {
		slog.Debug("ebpf/observability: no BPF maps; collector is a no-op")
		return
	}
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.collect(ctx)
		case <-c.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *Collector) collect(_ context.Context) {
	// Real map iteration wired after bpf2go generates typed accessors.
	slog.Debug("ebpf/observability: collect tick")
}

// Close stops the collector. Implements io.Closer.
func (c *Collector) Close() error {
	select {
	case <-c.stop:
	default:
		close(c.stop)
	}
	return nil
}

var _ io.Closer = (*Collector)(nil)
