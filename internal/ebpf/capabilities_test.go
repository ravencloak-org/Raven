package ebpf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrUnsupported_IsError(t *testing.T) {
	err := &ErrUnsupported{Reason: "missing CAP_BPF"}
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "eBPF unavailable")
	assert.Contains(t, err.Error(), "missing CAP_BPF")
}

func TestManager_StopIsIdempotent(t *testing.T) {
	m := NewManager()
	// Stop on a never-started manager must not panic
	assert.NotPanics(t, func() { m.Stop() })
	assert.NotPanics(t, func() { m.Stop() })
}
