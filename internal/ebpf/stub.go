//go:build !linux

// Package ebpf provides no-op stubs on non-Linux platforms.
package ebpf

import "fmt"

// ErrUnsupported is returned when eBPF is not available.
type ErrUnsupported struct {
	Reason string
}

func (e *ErrUnsupported) Error() string {
	return fmt.Sprintf("eBPF unavailable: %s", e.Reason)
}

// CheckCapabilities always returns ErrUnsupported on non-Linux.
func CheckCapabilities() error {
	return &ErrUnsupported{Reason: "non-Linux platform"}
}

// Closer is implemented by any eBPF feature that needs cleanup on shutdown.
type Closer interface {
	Close() error
}

// Manager is a no-op on non-Linux.
type Manager struct{}

// NewManager returns a new no-op Manager.
func NewManager() *Manager { return &Manager{} }

// Register is a no-op on non-Linux.
func (m *Manager) Register(_ Closer) error { return nil }

// Stop is a no-op on non-Linux.
func (m *Manager) Stop() {}
