package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestCollector_NewDoesNotPanic(t *testing.T) {
	mp := noop.NewMeterProvider()
	c, err := NewCollector(mp.Meter("test"), nil)
	require.NoError(t, err)
	assert.NotNil(t, c)
}

func TestCollector_Close(t *testing.T) {
	mp := noop.NewMeterProvider()
	c, err := NewCollector(mp.Meter("test"), nil)
	require.NoError(t, err)
	assert.NoError(t, c.Close())
}

func TestCollector_StartWithNilMaps(t *testing.T) {
	mp := noop.NewMeterProvider()
	c, err := NewCollector(mp.Meter("test"), nil)
	require.NoError(t, err)
	// Start with nil maps must not panic or block
	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx)
	cancel()
	assert.NoError(t, c.Close())
}
