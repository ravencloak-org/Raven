//go:build linux

// Package ebpf owns the eBPF lifecycle for the Raven API.
// All features degrade gracefully to no-op when eBPF is unavailable.
package ebpf

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/cilium/ebpf/rlimit"
)

// ErrUnsupported is returned when the runtime environment cannot support eBPF.
type ErrUnsupported struct {
	Reason string
}

func (e *ErrUnsupported) Error() string {
	return fmt.Sprintf("eBPF unavailable: %s", e.Reason)
}

// minKernelVersion is the minimum supported kernel (5.8).
const minKernelMajor, minKernelMinor = 5, 8

// CheckCapabilities verifies that the runtime environment supports eBPF.
// It checks the kernel version floor, calls rlimit.RemoveMemlock() on kernels
// < 5.11, and returns ErrUnsupported when the environment cannot run eBPF.
func CheckCapabilities() error {
	if runtime.GOOS != "linux" {
		return &ErrUnsupported{Reason: "non-Linux OS: " + runtime.GOOS}
	}

	major, minor, err := kernelVersion()
	if err != nil {
		return &ErrUnsupported{Reason: "cannot determine kernel version: " + err.Error()}
	}
	if major < minKernelMajor || (major == minKernelMajor && minor < minKernelMinor) {
		return &ErrUnsupported{
			Reason: fmt.Sprintf("kernel %d.%d < required %d.%d", major, minor, minKernelMajor, minKernelMinor),
		}
	}

	// Kernels < 5.11 require RLIMIT_MEMLOCK to be raised for BPF map allocation.
	if major < 5 || (major == 5 && minor < 11) {
		if err := rlimit.RemoveMemlock(); err != nil {
			slog.Warn("eBPF: failed to remove RLIMIT_MEMLOCK; map allocation may fail", "error", err)
		}
	}

	// Check BTF availability — required for CO-RE and tp_btf programs.
	if _, err := os.Stat("/sys/kernel/btf/vmlinux"); os.IsNotExist(err) {
		return &ErrUnsupported{Reason: "BTF not available (/sys/kernel/btf/vmlinux missing); ensure CONFIG_DEBUG_INFO_BTF=y"}
	}

	return nil
}

// kernelVersion parses /proc/version and returns the major and minor kernel version numbers.
func kernelVersion() (major, minor int, err error) {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return 0, 0, err
	}
	// Format: "Linux version X.Y.Z ..."
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, fmt.Errorf("unexpected /proc/version format")
	}
	parts := strings.SplitN(fields[2], ".", 3)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("cannot parse kernel version %q", fields[2])
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parsing major: %w", err)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parsing minor: %w", err)
	}
	return major, minor, nil
}
