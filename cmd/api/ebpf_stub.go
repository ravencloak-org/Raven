//go:build !linux

package main

import (
	"fmt"
	"log/slog"

	"github.com/ravencloak-org/Raven/internal/config"
	"github.com/ravencloak-org/Raven/internal/ebpf"
)

func initEBPF(cfg *config.EBPFConfig) (*ebpf.Manager, error) {
	if cfg.ObservabilityEnabled || cfg.AuditEnabled || cfg.XDPEnabled {
		slog.Warn("eBPF features requested but not available on this platform")
		return ebpf.NewManager(), fmt.Errorf("eBPF unsupported on non-Linux platform")
	}
	slog.Debug("eBPF disabled: non-Linux platform")
	return ebpf.NewManager(), nil
}
