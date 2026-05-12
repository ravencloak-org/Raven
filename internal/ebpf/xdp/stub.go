//go:build !linux

package xdp

import "go.opentelemetry.io/otel/metric"

// Objects is a no-op interface on non-Linux.
type Objects interface {
	Close() error
}

// Config configures the XDP controller.
type Config struct {
	Interface string
}

// Controller is a no-op on non-Linux.
type Controller struct{}

// NewController is a no-op constructor on non-Linux targets so callers can
// share the same wiring code across platforms without build-tag gymnastics.
func NewController(_ Objects, _ any, _ metric.Meter, _ Config) (*Controller, error) {
	return &Controller{}, nil
}

// SyncBlocklist is a no-op on non-Linux targets.
func (c *Controller) SyncBlocklist(_ []string) {}

// Close is a no-op on non-Linux targets and always returns nil.
func (c *Controller) Close() error { return nil }
