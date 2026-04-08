//go:build linux

// Package xdp implements Feature #120: XDP pre-filtering at the network driver level.
package xdp

import (
	"io"
	"log/slog"

	"go.opentelemetry.io/otel/metric"
)

// XDPObjects holds the bpf2go-generated map handles. Nil = no eBPF objects loaded.
type XDPObjects interface {
	// BlockedIPs returns the LPM trie map for blocked CIDRs.
	io.Closer
}

// Config configures the XDP controller.
type Config struct {
	Interface string
}

// Controller attaches the XDP program and syncs the blocklist from Valkey.
type Controller struct {
	objects XDPObjects
	cfg     Config
	stop    chan struct{}
	done    chan struct{}
	dropped metric.Int64Counter
}

// NewController creates a Controller. Pass nil objects for a no-op (graceful degrade).
func NewController(objects XDPObjects, meter metric.Meter, cfg Config) (*Controller, error) {
	dropped, err := meter.Int64Counter(
		"ebpf.xdp.dropped_packets",
		metric.WithDescription("Packets dropped by XDP pre-filter"),
		metric.WithUnit("{packet}"),
	)
	if err != nil {
		return nil, err
	}

	return &Controller{
		objects: objects,
		cfg:     cfg,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		dropped: dropped,
	}, nil
}

// Close detaches the XDP program and stops the sync loop.
func (c *Controller) Close() error {
	select {
	case <-c.stop:
	default:
		close(c.stop)
	}
	if c.objects != nil {
		return c.objects.Close()
	}
	return nil
}

var _ io.Closer = (*Controller)(nil)

// SyncBlocklist writes cidrs to the blocked_ips BPF map.
// Called from the sync loop after fetching CIDRs from Valkey.
// This is a stub — fully wired in Task 8 after bpf2go generates typed accessors.
func (c *Controller) SyncBlocklist(cidrs []string) {
	if c.objects == nil {
		return
	}
	slog.Debug("ebpf/xdp: syncing blocklist", "count", len(cidrs))
}
