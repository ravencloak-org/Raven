//go:build linux

package main

import (
	"testing"

	"github.com/ravencloak-org/Raven/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestEBPFManager_GracefulDegradeOnCI(t *testing.T) {
	cfg := &config.EBPFConfig{
		Enabled:              true,
		ObservabilityEnabled: true,
		AuditEnabled:         true,
		XDPEnabled:           true,
		XDPInterface:         "lo",
		AuditRingBufferSize:  1048576,
	}
	manager, err := initEBPF(cfg)
	assert.NotNil(t, manager)
	// On CI without CAP_BPF, initEBPF returns a non-nil manager with a degradation error
	if err != nil {
		t.Logf("eBPF gracefully degraded: %v", err)
	}
	manager.Stop()
}
