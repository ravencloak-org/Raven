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
	t.Setenv("RAVEN_EBPF_OBSERVABILITY_ENABLED", "true")
	t.Setenv("RAVEN_EBPF_AUDIT_ENABLED", "true")
	t.Setenv("RAVEN_EBPF_AUDIT_RING_BUFFER_SIZE", "2097152")
	t.Setenv("RAVEN_EBPF_XDP_ENABLED", "true")
	t.Setenv("RAVEN_EBPF_XDP_INTERFACE", "wlan0")

	cfg, err := Load()
	require.NoError(t, err)

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
	// Validation is unconditional — fires regardless of AuditEnabled
	t.Setenv("RAVEN_EBPF_AUDIT_RING_BUFFER_SIZE", "1000000") // not a power of 2

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ebpf.audit_ring_buffer_size must be a power of 2")
}
