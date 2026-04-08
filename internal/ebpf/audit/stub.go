//go:build !linux

// Package audit provides no-op stubs on non-Linux platforms.
package audit

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

// Config configures the audit consumer.
type Config struct {
	IPAllowlist   []string
	ExecAllowlist []string
}

// RingBufReader is a no-op interface on non-Linux.
type RingBufReader interface {
	Close() error
}

// Consumer is a no-op on non-Linux.
type Consumer struct{}

// NewConsumer returns a no-op consumer on non-Linux.
func NewConsumer(_ RingBufReader, _ metric.Meter, _ Config) (*Consumer, error) {
	return &Consumer{}, nil
}

// Start is a no-op on non-Linux.
func (c *Consumer) Start(_ context.Context) {}

// Close is a no-op on non-Linux.
func (c *Consumer) Close() error { return nil }
