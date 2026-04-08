//go:build linux

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 XDP ../programs/xdp.c -- -I/usr/include/$(shell uname -m)-linux-gnu

// Package xdp implements Feature #120: XDP pre-filtering at the network driver level.
package xdp

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"go.opentelemetry.io/otel/metric"
)

// LPMKey mirrors the C struct bpf_lpm_trie_key + 4-byte IPv4 data.
type LPMKey struct {
	Prefixlen uint32
	Addr      [4]byte
}

// parseLPMKey parses a CIDR string into an LPMKey for the BPF LPM trie.
func parseLPMKey(cidr string) (LPMKey, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return LPMKey{}, fmt.Errorf("xdp: invalid CIDR %q: %w", cidr, err)
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return LPMKey{}, fmt.Errorf("xdp: only IPv4 CIDRs supported, got %q", cidr)
	}
	ones, _ := ipNet.Mask.Size()
	var addr [4]byte
	binary.BigEndian.PutUint32(addr[:], binary.BigEndian.Uint32(ip4))
	return LPMKey{Prefixlen: uint32(ones), Addr: addr}, nil
}

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
	xdpLink link.Link
}

// NewController creates a Controller. Pass nil objects for a no-op (graceful degrade).
func NewController(objects XDPObjects, prog *ebpf.Program, meter metric.Meter, cfg Config) (*Controller, error) {
	dropped, err := meter.Int64Counter(
		"ebpf.xdp.dropped_packets",
		metric.WithDescription("Packets dropped by XDP pre-filter"),
		metric.WithUnit("{packet}"),
	)
	if err != nil {
		return nil, err
	}

	c := &Controller{
		objects: objects,
		cfg:     cfg,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		dropped: dropped,
	}

	if prog != nil {
		iface, err := net.InterfaceByName(cfg.Interface)
		if err != nil {
			slog.Warn("ebpf/xdp: interface not found; XDP disabled", "interface", cfg.Interface, "error", err)
			return c, nil
		}
		// Try native XDP mode first; fall back to generic if driver doesn't support it.
		xdpLink, err := link.AttachXDP(link.XDPOptions{
			Program:   prog,
			Interface: iface.Index,
			Flags:     link.XDPDriverMode,
		})
		if err != nil {
			slog.Warn("ebpf/xdp: native XDP attach failed, falling back to generic mode", "error", err)
			xdpLink, err = link.AttachXDP(link.XDPOptions{
				Program:   prog,
				Interface: iface.Index,
				Flags:     link.XDPGenericMode,
			})
			if err != nil {
				slog.Warn("ebpf/xdp: generic XDP attach also failed; XDP disabled", "error", err)
				return c, nil
			}
		}
		c.xdpLink = xdpLink
		slog.Info("ebpf/xdp: program attached", "interface", cfg.Interface)
	}

	return c, nil
}

// Close detaches the XDP program and stops the sync loop.
func (c *Controller) Close() error {
	select {
	case <-c.stop:
	default:
		close(c.stop)
	}
	if c.xdpLink != nil {
		if err := c.xdpLink.Close(); err != nil {
			slog.Warn("ebpf/xdp: error detaching XDP program", "error", err)
		}
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

// xdpAttachFlags returns the preferred XDP attach flags for testing.
func xdpAttachFlags() link.XDPAttachFlags {
	return link.XDPGenericMode
}
