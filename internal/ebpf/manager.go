//go:build linux

package ebpf

import (
	"io"
	"log/slog"
	"sync"
)

// Closer is implemented by any eBPF feature that needs cleanup on shutdown.
type Closer interface {
	io.Closer
}

// Manager owns the shared eBPF lifecycle. Features register their Closer with
// Register; Stop() calls Close() on all of them in LIFO order.
type Manager struct {
	mu      sync.Mutex
	closers []Closer
	stopped bool
}

// NewManager returns a new Manager.
func NewManager() *Manager {
	return &Manager{}
}

// Register adds a Closer that will be called on Stop().
func (m *Manager) Register(c Closer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closers = append(m.closers, c)
}

// Stop closes all registered features in reverse registration order.
// It is safe to call multiple times.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped {
		return
	}
	m.stopped = true
	for i := len(m.closers) - 1; i >= 0; i-- {
		if err := m.closers[i].Close(); err != nil {
			slog.Warn("eBPF: error closing feature", "error", err)
		}
	}
}
