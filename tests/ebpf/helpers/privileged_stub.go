//go:build ebpf && !linux

package helpers

import "testing"

// RequirePrivileged always skips on non-Linux platforms — eBPF is Linux-only.
func RequirePrivileged(t *testing.T) {
	t.Helper()
	t.Skip("skipping: eBPF tests require Linux")
}

// RequireKernelBTF always skips on non-Linux platforms.
func RequireKernelBTF(t *testing.T) {
	t.Helper()
	t.Skip("skipping: BTF not available on non-Linux platforms")
}
