package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestConsumer_NewWithNilReader(t *testing.T) {
	mp := noop.NewMeterProvider()
	c, err := NewConsumer(nil, mp.Meter("test"), Config{})
	require.NoError(t, err)
	assert.NotNil(t, c)
}

func TestConsumer_Close(t *testing.T) {
	mp := noop.NewMeterProvider()
	c, err := NewConsumer(nil, mp.Meter("test"), Config{})
	require.NoError(t, err)
	assert.NoError(t, c.Close())
}

func TestConsumer_IPAllowlist_ParsesCIDRs(t *testing.T) {
	cfg := Config{
		IPAllowlist: []string{"192.168.0.0/16", "10.0.0.0/8"},
	}
	nets, err := parseCIDRs(cfg.IPAllowlist)
	require.NoError(t, err)
	assert.Len(t, nets, 2)
}

func TestConsumer_IPAllowlist_InvalidCIDR(t *testing.T) {
	_, err := parseCIDRs([]string{"not-a-cidr"})
	assert.Error(t, err)
}
