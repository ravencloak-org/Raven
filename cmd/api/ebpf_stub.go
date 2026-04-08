//go:build !linux

package main

import (
	"log/slog"

	"github.com/ravencloak-org/Raven/internal/config"
	"github.com/ravencloak-org/Raven/internal/ebpf"
)

func initEBPF(_ *config.EBPFConfig, _ any) (*ebpf.Manager, error) {
	slog.Debug("eBPF disabled: non-Linux platform")
	return ebpf.NewManager(), nil
}
