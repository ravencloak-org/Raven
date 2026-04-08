//go:build linux

package main

import (
	"log/slog"

	"go.opentelemetry.io/otel"

	"github.com/ravencloak-org/Raven/internal/config"
	"github.com/ravencloak-org/Raven/internal/ebpf"
	"github.com/ravencloak-org/Raven/internal/ebpf/audit"
	"github.com/ravencloak-org/Raven/internal/ebpf/observability"
	"github.com/ravencloak-org/Raven/internal/ebpf/xdp"
)

// initEBPF starts the eBPF subsystem based on cfg.
// It always returns a non-nil Manager — features that fail to start are logged
// and skipped; the API server continues regardless.
func initEBPF(cfg *config.EBPFConfig) (*ebpf.Manager, error) {
	manager := ebpf.NewManager()

	if err := ebpf.CheckCapabilities(); err != nil {
		slog.Warn("eBPF subsystem disabled", "reason", err)
		return manager, err
	}

	meter := otel.GetMeterProvider().Meter("raven/ebpf")

	if cfg.ObservabilityEnabled {
		col, err := observability.NewCollector(meter, nil)
		if err != nil {
			slog.Warn("eBPF observability failed to start", "error", err)
		} else {
			if err := manager.Register(col); err != nil {
				slog.Warn("eBPF: failed to register component", "error", err)
			}
			slog.Info("eBPF observability enabled")
		}
	}

	if cfg.AuditEnabled {
		con, err := audit.NewConsumer(nil, meter, audit.Config{
			IPAllowlist:   cfg.AuditIPAllowlist,
			ExecAllowlist: cfg.AuditExecAllowlist,
		})
		if err != nil {
			slog.Warn("eBPF audit consumer failed to start", "error", err)
		} else {
			if err := manager.Register(con); err != nil {
				slog.Warn("eBPF: failed to register component", "error", err)
			}
			slog.Info("eBPF audit trail enabled")
		}
	}

	if cfg.XDPEnabled {
		ctrl, err := xdp.NewController(nil, nil, meter, xdp.Config{
			Interface: cfg.XDPInterface,
		})
		if err != nil {
			slog.Warn("eBPF XDP controller failed to start", "error", err)
		} else {
			if err := manager.Register(ctrl); err != nil {
				slog.Warn("eBPF: failed to register component", "error", err)
			}
			slog.Info("eBPF XDP pre-filtering enabled", "interface", cfg.XDPInterface)
		}
	}

	return manager, nil
}
