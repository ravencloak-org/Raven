//go:build !linux

// Package observability provides no-op stubs on non-Linux platforms.
package observability

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

// Maps is empty on non-Linux.
type Maps struct{}

// Collector is a no-op on non-Linux.
type Collector struct{}

// NewCollector returns a no-op collector on non-Linux.
func NewCollector(_ metric.Meter, _ *Maps) (*Collector, error) {
	return &Collector{}, nil
}

// Start is a no-op on non-Linux.
func (c *Collector) Start(_ context.Context) {}

// Close is a no-op on non-Linux.
func (c *Collector) Close() error { return nil }
