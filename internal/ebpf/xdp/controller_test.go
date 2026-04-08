//go:build linux

package xdp

import (
	"testing"

	"github.com/cilium/ebpf/link"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestController_NewWithNilObjects(t *testing.T) {
	mp := noop.NewMeterProvider()
	c, err := NewController(nil, nil, mp.Meter("test"), Config{Interface: "lo"})
	require.NoError(t, err)
	assert.NotNil(t, c)
}

func TestController_Stop(t *testing.T) {
	mp := noop.NewMeterProvider()
	c, err := NewController(nil, nil, mp.Meter("test"), Config{Interface: "lo"})
	require.NoError(t, err)
	assert.NoError(t, c.Close())
}

func TestController_ParseCIDR_Valid(t *testing.T) {
	_, err := parseLPMKey("192.168.1.0/24")
	assert.NoError(t, err)
}

func TestController_ParseCIDR_Invalid(t *testing.T) {
	_, err := parseLPMKey("not-a-cidr")
	assert.Error(t, err)
}

func TestController_XDPOptions_NativeWithGenericFallback(t *testing.T) {
	assert.Equal(t, xdpAttachFlags(), link.XDPGenericMode)
}
