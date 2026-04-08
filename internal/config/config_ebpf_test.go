package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEBPFConfig_Defaults(t *testing.T) {
	t.Setenv("RAVEN_DATABASE_URL", "postgres://x:x@localhost/x")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_USER_LIMIT", "100")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_ORG_LIMIT", "1000")

	cfg, err := Load()
	require.NoError(t, err)

	assert.False(t, cfg.EBPF.Enabled, "master toggle must default to false")
	assert.False(t, cfg.EBPF.ObservabilityEnabled)
	assert.False(t, cfg.EBPF.AuditEnabled)
	assert.False(t, cfg.EBPF.XDPEnabled)
	assert.Equal(t, "eth0", cfg.EBPF.XDPInterface)
	assert.Equal(t, 1048576, cfg.EBPF.AuditRingBufferSize)
}

func TestEBPFConfig_EnvOverride(t *testing.T) {
	t.Setenv("RAVEN_DATABASE_URL", "postgres://x:x@localhost/x")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_USER_LIMIT", "100")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_ORG_LIMIT", "1000")
	t.Setenv("RAVEN_EBPF_ENABLED", "true")
	t.Setenv("RAVEN_EBPF_OBSERVABILITY_ENABLED", "true")
	t.Setenv("RAVEN_EBPF_AUDIT_ENABLED", "true")
	t.Setenv("RAVEN_EBPF_AUDIT_RING_BUFFER_SIZE", "2097152")
	t.Setenv("RAVEN_EBPF_XDP_ENABLED", "true")
	t.Setenv("RAVEN_EBPF_XDP_INTERFACE", "wlan0")

	cfg, err := Load()
	require.NoError(t, err)

	assert.True(t, cfg.EBPF.Enabled)
	assert.True(t, cfg.EBPF.ObservabilityEnabled)
	assert.True(t, cfg.EBPF.AuditEnabled)
	assert.Equal(t, 2097152, cfg.EBPF.AuditRingBufferSize)
	assert.True(t, cfg.EBPF.XDPEnabled)
	assert.Equal(t, "wlan0", cfg.EBPF.XDPInterface)
}

func TestEBPFConfig_InvalidRingBufferSize(t *testing.T) {
	t.Setenv("RAVEN_DATABASE_URL", "postgres://x:x@localhost/x")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_USER_LIMIT", "100")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_ORG_LIMIT", "1000")
	// Validation fires when master toggle is enabled
	t.Setenv("RAVEN_EBPF_ENABLED", "true")
	t.Setenv("RAVEN_EBPF_AUDIT_RING_BUFFER_SIZE", "1000000") // not a power of 2

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ebpf.audit_ring_buffer_size must be a power of 2")
}

// TestEBPFConfig_MasterToggle_DefaultOff verifies that individual feature flags are
// irrelevant when the master toggle is off — the subsystem won't start.
func TestEBPFConfig_MasterToggle_DefaultOff(t *testing.T) {
	t.Setenv("RAVEN_DATABASE_URL", "postgres://x:x@localhost/x")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_USER_LIMIT", "100")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_ORG_LIMIT", "1000")
	t.Setenv("RAVEN_EBPF_OBSERVABILITY_ENABLED", "true")
	t.Setenv("RAVEN_EBPF_XDP_ENABLED", "true")

	cfg, err := Load()
	require.NoError(t, err)
	assert.False(t, cfg.EBPF.Enabled, "master toggle must default to false regardless of sub-flags")
}

// TestEBPFConfig_MasterToggle_ExplicitEnable verifies RAVEN_EBPF_ENABLED=true is respected.
func TestEBPFConfig_MasterToggle_ExplicitEnable(t *testing.T) {
	t.Setenv("RAVEN_DATABASE_URL", "postgres://x:x@localhost/x")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_USER_LIMIT", "100")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_ORG_LIMIT", "1000")
	t.Setenv("RAVEN_EBPF_ENABLED", "true")

	cfg, err := Load()
	require.NoError(t, err)
	assert.True(t, cfg.EBPF.Enabled)
}
