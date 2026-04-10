//go:build ebpf && linux

package helpers

import (
	"testing"

	"golang.org/x/sys/unix"
)

// RequirePrivileged skips the test unless the process has CAP_BPF / CAP_SYS_ADMIN.
// It probes the kernel by issuing a no-op BPF syscall (cmd=0, attr=NULL, size=0).
// EPERM means we lack privileges; any other error (EINVAL, EFAULT) means eBPF
// is reachable and we can proceed.
func RequirePrivileged(t *testing.T) {
	t.Helper()

	// BPF_MAP_CREATE = 0. With a nil attr and zero size the kernel returns
	// EFAULT or EINVAL when the caller is privileged, and EPERM when not.
	_, _, errno := unix.Syscall(
		unix.SYS_BPF,
		0, // BPF_MAP_CREATE
		0, // attr pointer = NULL
		0, // size = 0
	)

	if errno == unix.EPERM {
		t.Skip("skipping: insufficient privileges for eBPF (need CAP_BPF or CAP_SYS_ADMIN)")
	}
	// EINVAL or EFAULT are expected — they mean the syscall is reachable.
}

// RequireKernelBTF skips the test if /sys/kernel/btf/vmlinux is not available.
func RequireKernelBTF(t *testing.T) {
	t.Helper()

	var stat unix.Stat_t
	if err := unix.Stat("/sys/kernel/btf/vmlinux", &stat); err != nil {
		t.Skip("skipping: BTF not available (/sys/kernel/btf/vmlinux missing)")
	}
}
