//go:build linux

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 Audit ../programs/audit.c -- -I/usr/include/$(shell uname -m)-linux-gnu

// Package audit implements Feature #123: security audit trail via eBPF ring buffer.
package audit

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"slices"

	"github.com/cilium/ebpf/ringbuf"
	"go.opentelemetry.io/otel/metric"
)

// Config configures the audit consumer.
type Config struct {
	IPAllowlist   []string // allowed outbound CIDRs; empty = allow all
	ExecAllowlist []string // allowed binary paths; empty = allow all
}

// RingBufReader is the subset of ringbuf.Reader used by Consumer (allows mocking).
type RingBufReader interface {
	Read() (ringbuf.Record, error)
	Close() error
}

// Consumer reads audit events from a BPF ring buffer and emits structured logs.
type Consumer struct {
	reader      RingBufReader
	cfg         Config
	allowedNets []*net.IPNet
	stop        chan struct{}
	done        chan struct{}
	dropped     metric.Int64Counter
}

// NewConsumer creates a Consumer. Pass nil reader for a no-op (graceful degrade).
func NewConsumer(reader RingBufReader, meter metric.Meter, cfg Config) (*Consumer, error) {
	nets, err := parseCIDRs(cfg.IPAllowlist)
	if err != nil {
		return nil, fmt.Errorf("audit: invalid IP allowlist: %w", err)
	}

	dropped, err := meter.Int64Counter(
		"ebpf.audit.dropped_events",
		metric.WithDescription("Audit ring buffer overflow drop count"),
		metric.WithUnit("{count}"),
	)
	if err != nil {
		return nil, err
	}

	return &Consumer{
		reader:      reader,
		cfg:         cfg,
		allowedNets: nets,
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
		dropped:     dropped,
	}, nil
}

// Start begins consuming events in a background goroutine.
func (c *Consumer) Start(ctx context.Context) {
	go c.run(ctx)
}

func (c *Consumer) run(ctx context.Context) {
	defer close(c.done)
	if c.reader == nil {
		slog.Debug("ebpf/audit: no ring buffer reader; consumer is a no-op")
		return
	}
	for {
		record, err := c.reader.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			c.dropped.Add(ctx, 1)
			slog.Warn("ebpf/audit: ring buffer overflow; events dropped", "error", err)
			continue
		}
		c.handleRecord(ctx, record)

		select {
		case <-c.stop:
			return
		case <-ctx.Done():
			return
		default:
		}
	}
}

// auditEventType mirrors the C enum in audit.c
const (
	auditExec    = 1
	auditTCP     = 2
	auditConnect = 3
)

func (c *Consumer) handleRecord(ctx context.Context, rec ringbuf.Record) {
	if len(rec.RawSample) < 1 {
		return
	}
	eventType := rec.RawSample[0]
	if len(rec.RawSample) < 20 {
		return
	}

	pid := binary.LittleEndian.Uint32(rec.RawSample[4:8])
	ts := binary.LittleEndian.Uint64(rec.RawSample[12:20])
	comm := ""
	if len(rec.RawSample) >= 36 {
		comm = nullTermStr(rec.RawSample[20:36])
	}

	switch eventType {
	case auditExec:
		path := ""
		if len(rec.RawSample) >= 164 {
			path = nullTermStr(rec.RawSample[36:164])
		}
		violation := len(c.cfg.ExecAllowlist) > 0 && !slices.Contains(c.cfg.ExecAllowlist, path)
		slog.InfoContext(ctx, "ebpf/audit: exec",
			"pid", pid, "comm", comm, "path", path,
			"timestamp_ns", ts, "audit.violation", violation,
		)
	case auditTCP:
		if len(rec.RawSample) < 44 {
			return
		}
		saddr := netip.AddrFrom4([4]byte(rec.RawSample[36:40]))
		daddr := netip.AddrFrom4([4]byte(rec.RawSample[40:44]))
		violation := !c.ipAllowed(daddr.Unmap().String())
		slog.InfoContext(ctx, "ebpf/audit: tcp-established",
			"pid", pid, "comm", comm,
			"src", saddr, "dst", daddr,
			"timestamp_ns", ts, "audit.violation", violation,
		)
	case auditConnect:
		slog.InfoContext(ctx, "ebpf/audit: connect",
			"pid", pid, "comm", comm, "timestamp_ns", ts,
		)
	}
}

func (c *Consumer) ipAllowed(ip string) bool {
	if len(c.allowedNets) == 0 {
		return true
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, n := range c.allowedNets {
		if n.Contains(parsed) {
			return true
		}
	}
	return false
}

// Close stops the consumer. Implements io.Closer.
func (c *Consumer) Close() error {
	select {
	case <-c.stop:
	default:
		close(c.stop)
	}
	if c.reader != nil {
		return c.reader.Close()
	}
	return nil
}

var _ io.Closer = (*Consumer)(nil)

// nullTermStr converts a null-terminated byte slice to a string.
func nullTermStr(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
