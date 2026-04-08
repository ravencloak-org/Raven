//go:build linux

package main

import (
	"testing"

	"github.com/ravencloak-org/Raven/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestEBPFManager_GracefulDegradeOnCI(t *testing.T) {
	cfg := &config.EBPFConfig{
		ObservabilityEnabled: true,
		AuditEnabled:         true,
		XDPEnabled:           true,
		XDPInterface:         "lo",
		AuditRingBufferSize:  1048576,
	}
	// initEBPF must not panic or crash even when capabilities are unavailable
	manager, err := initEBPF(cfg, nil)
	// On CI without CAP_BPF, err may be non-nil but manager must not be nil
	assert.NotNil(t, manager)
	_ = err
	manager.Stop()
}
