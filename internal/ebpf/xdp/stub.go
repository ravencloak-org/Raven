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

func NewController(_ Objects, _ any, _ metric.Meter, _ Config) (*Controller, error) {
	return &Controller{}, nil
}
func (c *Controller) SyncBlocklist(_ []string) {}
func (c *Controller) Close() error              { return nil }
