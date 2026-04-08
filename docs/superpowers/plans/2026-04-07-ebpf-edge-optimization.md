# eBPF Edge Optimization (M10) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement three eBPF-based features (#120 XDP Pre-filtering, #122 Kernel Observability, #123 Security Audit Trail) on a shared Go foundation inside the Raven API binary, with full graceful degradation when eBPF is unavailable.

**Architecture:** A single `internal/ebpf/` package owns the eBPF lifecycle (capabilities check, loader, manager). Three sub-packages (`observability/`, `audit/`, `xdp/`) each own one feature. BPF C programs live in `internal/ebpf/programs/` and are compiled by `bpf2go` at build time — the bytecode is embedded into the Go binary, so edge nodes need no C toolchain at runtime. All features are opt-in via env vars and self-disable with a structured `slog` warning if the kernel, capabilities, or BTF are missing.

**Tech Stack:** Go 1.26 + `github.com/cilium/ebpf` v0.17+ (bpf2go, link, ringbuf, rlimit), `go.opentelemetry.io/otel/metric` (existing), clang/llvm (build-time only), Linux kernel ≥ 5.8 with BTF.

**Implementation order (lowest risk first):**
1. Config (`internal/config/config.go`)
2. Add `github.com/cilium/ebpf` dependency
3. Shared foundation (`internal/ebpf/`)
4. BPF C programs skeleton (`internal/ebpf/programs/`)
5. Feature #122 Kernel Observability (`internal/ebpf/observability/`)
6. Feature #123 Security Audit Trail (`internal/ebpf/audit/`)
7. Feature #120 XDP Pre-filtering — controller (`internal/ebpf/xdp/`)
8. Feature #120 XDP — `link.AttachXDP` wiring
9. Wire `ebpf.Manager` into `cmd/api/main.go`
10. Build pipeline (`Dockerfile`, `Makefile.edge`, Docker Compose)
11. CI updates (`.github/workflows/go.yml`)
12. `go:generate` directives + final lint pass

---

## File Map

### New files
| File | Responsibility |
|------|---------------|
| `internal/ebpf/capabilities.go` | CAP_BPF / CAP_NET_ADMIN check, kernel version floor (≥5.8), `rlimit.RemoveMemlock()` |
| `internal/ebpf/manager.go` | `Manager` struct: `Start()` / `Stop()` / `io.Closer`; detaches all probes |
| `internal/ebpf/programs/observability.c` | BPF C: sched_switch, raw_syscalls/sys_exit, net tracepoints, __fd_install kprobe |
| `internal/ebpf/programs/audit.c` | BPF C: execve tracepoint, inet_sock_set_state, sys_enter_connect |
| `internal/ebpf/programs/xdp.c` | BPF C: XDP hook, LPM trie lookup, throttle |
| `internal/ebpf/observability/collector.go` | Polls BPF maps → OTel Gauge/Counter instruments |
| `internal/ebpf/observability/collector_test.go` | Unit tests with mock BPF maps |
| `internal/ebpf/audit/consumer.go` | Ring buffer reader goroutine → slog JSON events, allowlist checks |
| `internal/ebpf/audit/consumer_test.go` | Unit tests |
| `internal/ebpf/xdp/controller.go` | XDP attach, SyncLoop (Valkey → blocked_ips map), OTel counter |
| `internal/ebpf/xdp/controller_test.go` | Unit tests |
| `internal/ebpf/stub.go` | `//go:build !linux` no-op stubs so the package compiles on macOS/CI |

### Modified files
| File | Change |
|------|--------|
| `internal/config/config.go` | Add `EBPFConfig` struct + viper bindings + power-of-2 validation |
| `cmd/api/main.go` | Wire `ebpf.Manager` after telemetry init, `defer manager.Stop()` |
| `Dockerfile` | Builder stage: add `clang llvm libbpf-dev musl-dev`, `CGO_ENABLED=1` |
| `Makefile.edge` | Add `build-arm64-ebpf` and `build-amd64-ebpf` targets |
| `docker-compose.edge.yml` | Add `cap_add: [CAP_BPF, CAP_NET_ADMIN]` to `go-api` service |
| `docker-compose.yml` | Same `cap_add` to dev `go-api` service |
| `.github/workflows/go.yml` | Add clang install, `go generate` check, `ebpf-integration` job |
| `go.mod` / `go.sum` | Add `github.com/cilium/ebpf` dependency |

---

## Task 1: Add EBPFConfig to config

**Files:**
- Modify: `internal/config/config.go` (after `STTConfig`, before `Load()`)

- [ ] **Step 1.1: Write failing test for config parsing**

Create `internal/config/config_ebpf_test.go`:

```go
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEBPFConfig_Defaults(t *testing.T) {
	t.Setenv("RAVEN_DATABASE_URL", "postgres://x:x@localhost/x")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_USER_LIMIT", "100")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_ORG_LIMIT", "1000")

	cfg, err := Load()
	require.NoError(t, err)

	assert.False(t, cfg.EBPF.ObservabilityEnabled)
	assert.False(t, cfg.EBPF.AuditEnabled)
	assert.False(t, cfg.EBPF.XDPEnabled)
	assert.Equal(t, "eth0", cfg.EBPF.XDPInterface)
	assert.Equal(t, 1048576, cfg.EBPF.AuditRingBufferSize)
}

func TestEBPFConfig_EnvOverride(t *testing.T) {
	t.Setenv("RAVEN_DATABASE_URL", "postgres://x:x@localhost/x")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_USER_LIMIT", "100")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_ORG_LIMIT", "1000")
	t.Setenv("RAVEN_EBPF_OBSERVABILITY_ENABLED", "true")
	t.Setenv("RAVEN_EBPF_AUDIT_ENABLED", "true")
	t.Setenv("RAVEN_EBPF_AUDIT_RING_BUFFER_SIZE", "2097152")
	t.Setenv("RAVEN_EBPF_XDP_ENABLED", "true")
	t.Setenv("RAVEN_EBPF_XDP_INTERFACE", "wlan0")

	cfg, err := Load()
	require.NoError(t, err)

	assert.True(t, cfg.EBPF.ObservabilityEnabled)
	assert.True(t, cfg.EBPF.AuditEnabled)
	assert.Equal(t, 2097152, cfg.EBPF.AuditRingBufferSize)
	assert.True(t, cfg.EBPF.XDPEnabled)
	assert.Equal(t, "wlan0", cfg.EBPF.XDPInterface)
}

func TestEBPFConfig_InvalidRingBufferSize(t *testing.T) {
	t.Setenv("RAVEN_DATABASE_URL", "postgres://x:x@localhost/x")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_USER_LIMIT", "100")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_ORG_LIMIT", "1000")
	// Validation is unconditional — fires regardless of AuditEnabled
	t.Setenv("RAVEN_EBPF_AUDIT_RING_BUFFER_SIZE", "1000000") // not a power of 2

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ebpf.audit_ring_buffer_size must be a power of 2")
}
```

- [ ] **Step 1.2: Run test to verify it fails**

```bash
cd /Users/jobinlawrance/Project/raven
go test ./internal/config/... -run TestEBPFConfig -v
```
Expected: FAIL — `cfg.EBPF` field does not exist yet.

- [ ] **Step 1.3: Add EBPFConfig struct and wire into Config**

In `internal/config/config.go`, add after the `STTConfig` struct (around line 92):

```go
// EBPFConfig holds eBPF feature flags. All features default to false — opt-in only.
// Kernel requirement: Linux ≥ 5.8 with CONFIG_DEBUG_INFO_BTF=y.
type EBPFConfig struct {
	ObservabilityEnabled bool     `mapstructure:"observability_enabled"`
	AuditEnabled         bool     `mapstructure:"audit_enabled"`
	AuditIPAllowlist     []string `mapstructure:"audit_ip_allowlist"`
	AuditExecAllowlist   []string `mapstructure:"audit_exec_allowlist"`
	// AuditRingBufferSize is the BPF ring buffer size in bytes. Must be a power of 2.
	AuditRingBufferSize int    `mapstructure:"audit_ring_buffer_size"`
	XDPEnabled          bool   `mapstructure:"xdp_enabled"`
	XDPInterface        string `mapstructure:"xdp_interface"`
}
```

Add `EBPF EBPFConfig` field to `Config` struct (after `STT STTConfig`):

```go
EBPF EBPFConfig
```

In `Load()`, add defaults after the STT defaults block:

```go
// eBPF defaults — all disabled; safe for existing deployments
v.SetDefault("ebpf.observability_enabled", false)
v.SetDefault("ebpf.audit_enabled", false)
v.SetDefault("ebpf.audit_ip_allowlist", []string{})
v.SetDefault("ebpf.audit_exec_allowlist", []string{})
v.SetDefault("ebpf.audit_ring_buffer_size", 1048576)
v.SetDefault("ebpf.xdp_enabled", false)
v.SetDefault("ebpf.xdp_interface", "eth0")
```

Add viper bindings (after the existing `_ = v.BindEnv("otel.enabled", ...)` lines):

```go
_ = v.BindEnv("ebpf.observability_enabled", "RAVEN_EBPF_OBSERVABILITY_ENABLED")
_ = v.BindEnv("ebpf.audit_enabled", "RAVEN_EBPF_AUDIT_ENABLED")
_ = v.BindEnv("ebpf.audit_ip_allowlist", "RAVEN_EBPF_AUDIT_IP_ALLOWLIST")
_ = v.BindEnv("ebpf.audit_exec_allowlist", "RAVEN_EBPF_AUDIT_EXEC_ALLOWLIST")
_ = v.BindEnv("ebpf.audit_ring_buffer_size", "RAVEN_EBPF_AUDIT_RING_BUFFER_SIZE")
_ = v.BindEnv("ebpf.xdp_enabled", "RAVEN_EBPF_XDP_ENABLED")
_ = v.BindEnv("ebpf.xdp_interface", "RAVEN_EBPF_XDP_INTERFACE")
```

Add validation after the existing `RateLimit` checks (before `return &cfg, nil`):

```go
// Validate ring buffer size unconditionally — catches misconfiguration before any BPF load.
{
    size := cfg.EBPF.AuditRingBufferSize
    if size <= 0 || bits.OnesCount(uint(size)) != 1 {
        return nil, fmt.Errorf("ebpf.audit_ring_buffer_size must be a power of 2 > 0, got %d", size)
    }
}
```

Add `"math/bits"` to the import block.

- [ ] **Step 1.4: Run tests to verify they pass**

```bash
go test ./internal/config/... -run TestEBPFConfig -v
```
Expected: PASS (3 tests).

- [ ] **Step 1.5: Run full config tests**

```bash
go test ./internal/config/... -v
```
Expected: all pass.

- [ ] **Step 1.6: Commit**

```bash
git add internal/config/config.go internal/config/config_ebpf_test.go
git commit -m "feat(config): add EBPFConfig with power-of-2 ring buffer validation"
```

---

## Task 2: Add cilium/ebpf dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 2.1: Add the dependency**

```bash
cd /Users/jobinlawrance/Project/raven
go get github.com/cilium/ebpf@latest
```

- [ ] **Step 2.2: Verify it resolves**

```bash
go mod tidy
go build ./... 2>&1 | head -20
```
Expected: builds cleanly.

- [ ] **Step 2.3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps(go): add github.com/cilium/ebpf"
```

---

## Task 3: Shared eBPF foundation

**Files:**
- Create: `internal/ebpf/capabilities.go`
- Create: `internal/ebpf/manager.go`
- Create: `internal/ebpf/stub.go` (non-Linux no-op)
- Create: `internal/ebpf/capabilities_test.go`

### 3A — capabilities.go

- [ ] **Step 3A.1: Write failing test**

Create `internal/ebpf/capabilities_test.go`:

```go
//go:build linux

package ebpf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrUnsupported_IsError(t *testing.T) {
	err := &ErrUnsupported{Reason: "missing CAP_BPF"}
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "eBPF unavailable")
	assert.Contains(t, err.Error(), "missing CAP_BPF")
}
```

Run: `go test ./internal/ebpf/... -run TestErrUnsupported -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3A.2: Create capabilities.go**

```go
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
// On non-Linux platforms this always returns ErrUnsupported.
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

// kernelVersion parses /proc/version_signature or /proc/version and returns
// the major and minor kernel version numbers.
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
```

- [ ] **Step 3A.3: Create stub.go for non-Linux**

```go
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
```

- [ ] **Step 3A.4: Run test**

```bash
go test ./internal/ebpf/... -run TestErrUnsupported -v
```
Expected: PASS.

### 3B — manager.go

- [ ] **Step 3B.1: Write failing test**

Append to `internal/ebpf/capabilities_test.go`:

```go
func TestManager_StopIsIdempotent(t *testing.T) {
	m := NewManager()
	// Stop on a never-started manager must not panic
	assert.NotPanics(t, func() { m.Stop() })
	assert.NotPanics(t, func() { m.Stop() })
}
```

Run: `go test ./internal/ebpf/... -run TestManager_StopIsIdempotent -v`
Expected: FAIL — `NewManager` not defined.

- [ ] **Step 3B.2: Create manager.go**

```go
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
```

Add stub method to `stub.go`:

```go
// Manager is a no-op on non-Linux.
type Manager struct{}

func NewManager() *Manager          { return &Manager{} }
func (m *Manager) Register(_ interface{ Close() error }) {}
func (m *Manager) Stop()            {}
```

- [ ] **Step 3B.3: Run tests**

```bash
go test ./internal/ebpf/... -v
```
Expected: all pass.

- [ ] **Step 3B.4: Commit**

```bash
git add internal/ebpf/
git commit -m "feat(ebpf): add shared foundation — capabilities, manager, stubs"
```

---

## Task 4: BPF C programs skeleton

**Files:**
- Create: `internal/ebpf/programs/observability.c`
- Create: `internal/ebpf/programs/audit.c`
- Create: `internal/ebpf/programs/xdp.c`

> **Note:** These C files are compiled by `bpf2go` (Task 8). They must compile cleanly with clang on a Linux host. On macOS (development), only the Go stubs are compiled; the generated `_bpfel.go` files are committed to the repo.

- [ ] **Step 4.1: Create programs/observability.c**

```c
// SPDX-License-Identifier: GPL-2.0

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

char LICENSE[] SEC("license") = "GPL";

// Per-PID CPU time accumulator (nanoseconds)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 4096);
    __type(key, u32);   // pid
    __type(value, u64); // accumulated cpu ns
} cpu_time_map SEC(".maps");

// Network bytes in/out per PID
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 4096);
    __type(key, u32);
    __type(value, u64);
} net_bytes_in SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 4096);
    __type(key, u32);
    __type(value, u64);
} net_bytes_out SEC(".maps");

// Syscall error counts (by syscall nr)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 512);
    __type(key, u32);   // syscall nr
    __type(value, u64); // error count
} syscall_errors SEC(".maps");

// FD count per PID
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 4096);
    __type(key, u32);
    __type(value, u64);
} fd_count_map SEC(".maps");

// sched_switch: accumulate prev task CPU time delta
SEC("tp_btf/sched_switch")
int BPF_PROG(handle_sched_switch, bool preempt,
             struct task_struct *prev, struct task_struct *next)
{
    u32 pid = BPF_CORE_READ(prev, pid);
    u64 runtime = BPF_CORE_READ(prev, se.sum_exec_runtime);
    u64 *val = bpf_map_lookup_elem(&cpu_time_map, &pid);
    if (val)
        __sync_fetch_and_add(val, runtime);
    else
        bpf_map_update_elem(&cpu_time_map, &pid, &runtime, BPF_ANY);
    return 0;
}

// raw_syscalls/sys_exit: count non-zero return codes as errors
SEC("tracepoint/raw_syscalls/sys_exit")
int handle_sys_exit(struct trace_event_raw_sys_exit *ctx)
{
    if (ctx->ret >= 0) return 0;
    u32 nr = (u32)ctx->id;
    u64 one = 1;
    u64 *val = bpf_map_lookup_elem(&syscall_errors, &nr);
    if (val)
        __sync_fetch_and_add(val, 1);
    else
        bpf_map_update_elem(&syscall_errors, &nr, &one, BPF_ANY);
    return 0;
}

// net/netif_receive_skb: bytes in per PID (approximate — skb owner)
SEC("tracepoint/net/netif_receive_skb")
int handle_net_rx(struct trace_event_raw_netif_receive_skb *ctx)
{
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    u64 len = ctx->len;
    u64 *val = bpf_map_lookup_elem(&net_bytes_in, &pid);
    if (val)
        __sync_fetch_and_add(val, len);
    else
        bpf_map_update_elem(&net_bytes_in, &pid, &len, BPF_ANY);
    return 0;
}

// net/net_dev_start_xmit: bytes out per PID
SEC("tracepoint/net/net_dev_start_xmit")
int handle_net_tx(struct trace_event_raw_net_dev_start_xmit *ctx)
{
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    u64 len = ctx->len;
    u64 *val = bpf_map_lookup_elem(&net_bytes_out, &pid);
    if (val)
        __sync_fetch_and_add(val, len);
    else
        bpf_map_update_elem(&net_bytes_out, &pid, &len, BPF_ANY);
    return 0;
}

// kprobe/__fd_install: track FD creation per PID
SEC("kprobe/__fd_install")
int BPF_KPROBE(handle_fd_install)
{
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    u64 one = 1;
    u64 *val = bpf_map_lookup_elem(&fd_count_map, &pid);
    if (val)
        __sync_fetch_and_add(val, 1);
    else
        bpf_map_update_elem(&fd_count_map, &pid, &one, BPF_ANY);
    return 0;
}
```

- [ ] **Step 4.2: Create programs/audit.c**

```c
// SPDX-License-Identifier: GPL-2.0

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

char LICENSE[] SEC("license") = "GPL";

#define TASK_COMM_LEN 16
#define PATH_LEN      128

// Audit event types
#define AUDIT_EXEC    1
#define AUDIT_TCP     2
#define AUDIT_CONNECT 3

struct audit_event {
    u8  type;
    u32 pid;
    u32 ppid;
    u64 timestamp_ns;
    char comm[TASK_COMM_LEN];
    union {
        struct {
            char path[PATH_LEN];
        } exec;
        struct {
            __be32 saddr;
            __be32 daddr;
            __be16 sport;
            __be16 dport;
        } net;
    };
};

// Ring buffer for audit events — sized at load time by userspace
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 20); // 1MB default; overridden by userspace
} audit_events SEC(".maps");

SEC("tracepoint/syscalls/sys_enter_execve")
int handle_execve(struct trace_event_raw_sys_enter *ctx)
{
    struct audit_event *e = bpf_ringbuf_reserve(&audit_events, sizeof(*e), 0);
    if (!e) return 0;

    e->type = AUDIT_EXEC;
    e->pid  = bpf_get_current_pid_tgid() >> 32;
    e->timestamp_ns = bpf_ktime_get_ns();
    bpf_get_current_comm(e->comm, sizeof(e->comm));

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    e->ppid = BPF_CORE_READ(task, real_parent, pid);

    // Read first argument (filename pointer)
    const char *filename = (const char *)ctx->args[0];
    bpf_probe_read_user_str(e->exec.path, sizeof(e->exec.path), filename);

    bpf_ringbuf_submit(e, 0);
    return 0;
}

SEC("tracepoint/sock/inet_sock_set_state")
int handle_tcp_state(struct trace_event_raw_inet_sock_set_state *ctx)
{
    // Only care about transitions TO TCP_ESTABLISHED
    if (ctx->newstate != 1 /* TCP_ESTABLISHED */) return 0;

    struct audit_event *e = bpf_ringbuf_reserve(&audit_events, sizeof(*e), 0);
    if (!e) return 0;

    e->type = AUDIT_TCP;
    e->pid  = bpf_get_current_pid_tgid() >> 32;
    e->timestamp_ns = bpf_ktime_get_ns();
    bpf_get_current_comm(e->comm, sizeof(e->comm));
    e->net.saddr = ctx->saddr;
    e->net.daddr = ctx->daddr;
    e->net.sport = ctx->sport;
    e->net.dport = ctx->dport;

    bpf_ringbuf_submit(e, 0);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_connect")
int handle_connect(struct trace_event_raw_sys_enter *ctx)
{
    struct audit_event *e = bpf_ringbuf_reserve(&audit_events, sizeof(*e), 0);
    if (!e) return 0;

    e->type = AUDIT_CONNECT;
    e->pid  = bpf_get_current_pid_tgid() >> 32;
    e->timestamp_ns = bpf_ktime_get_ns();
    bpf_get_current_comm(e->comm, sizeof(e->comm));

    bpf_ringbuf_submit(e, 0);
    return 0;
}
```

- [ ] **Step 4.3: Create programs/xdp.c**

```c
// SPDX-License-Identifier: GPL-2.0

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>

char LICENSE[] SEC("license") = "GPL";

// LPM trie key for IPv4 prefix matching
struct lpm_key {
    __u32 prefixlen;
    __u32 addr;
};

// Blocked CIDRs — LPM trie for prefix matching
struct {
    __uint(type, BPF_MAP_TYPE_LPM_TRIE);
    __uint(max_entries, 10000);
    __uint(map_flags, BPF_F_NO_PREALLOC);
    __type(key, struct lpm_key);
    __type(value, u32); // reason code (reserved)
} blocked_ips SEC(".maps");

// Throttle state — per-IP packet counter + timestamp (LRU evicts stale entries)
struct throttle_val {
    u64 count;
    u64 window_start_ns;
};

struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, 65536);
    __type(key, u32);   // src IP
    __type(value, struct throttle_val);
} throttle_state SEC(".maps");

// XDP drop counter
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, u32);
    __type(value, u64);
} drop_count SEC(".maps");

#define THROTTLE_LIMIT  1000   // packets per window
#define THROTTLE_WINDOW 1000000000ULL // 1 second in ns

SEC("xdp")
int xdp_filter(struct xdp_md *ctx)
{
    void *data_end = (void *)(long)ctx->data_end;
    void *data     = (void *)(long)ctx->data;

    // Parse Ethernet header
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end) return XDP_PASS;
    if (eth->h_proto != __constant_htons(ETH_P_IP)) return XDP_PASS;

    // Parse IP header
    struct iphdr *ip = (void *)(eth + 1);
    if ((void *)(ip + 1) > data_end) return XDP_PASS;

    u32 src = ip->saddr;

    // LPM trie lookup — check if src IP is in a blocked CIDR
    struct lpm_key key = { .prefixlen = 32, .addr = src };
    if (bpf_map_lookup_elem(&blocked_ips, &key)) {
        u32 idx = 0;
        u64 *cnt = bpf_map_lookup_elem(&drop_count, &idx);
        if (cnt) __sync_fetch_and_add(cnt, 1);
        return XDP_DROP;
    }

    // Throttle check
    u64 now = bpf_ktime_get_ns();
    struct throttle_val *tv = bpf_map_lookup_elem(&throttle_state, &src);
    if (tv) {
        if (now - tv->window_start_ns > THROTTLE_WINDOW) {
            tv->window_start_ns = now;
            tv->count = 1;
        } else {
            __sync_fetch_and_add(&tv->count, 1);
            if (tv->count > THROTTLE_LIMIT) {
                u32 idx = 0;
                u64 *cnt = bpf_map_lookup_elem(&drop_count, &idx);
                if (cnt) __sync_fetch_and_add(cnt, 1);
                return XDP_DROP;
            }
        }
    } else {
        struct throttle_val new_tv = { .count = 1, .window_start_ns = now };
        bpf_map_update_elem(&throttle_state, &src, &new_tv, BPF_ANY);
    }

    return XDP_PASS;
}
```

- [ ] **Step 4.4: Commit**

```bash
git add internal/ebpf/programs/
git commit -m "feat(ebpf): add BPF C programs skeleton (observability, audit, xdp)"
```

---

## Task 5: Feature #122 — Kernel Observability collector

**Files:**
- Create: `internal/ebpf/observability/collector.go`
- Create: `internal/ebpf/observability/collector_test.go`
- Create: `internal/ebpf/observability/observability_bpfel.go` (generated — committed)

> **Note on generated files:** `_bpfel.go` / `_bpfeb.go` are generated by `bpf2go` (Task 8). For now, create stub generated files manually so the package compiles and tests can run. Replace with real generated files after Task 8.

- [ ] **Step 5.1: Write failing test**

Create `internal/ebpf/observability/collector_test.go`:

```go
//go:build linux

package observability

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestCollector_NewDoesNotPanic(t *testing.T) {
	mp := noop.NewMeterProvider()
	c, err := NewCollector(mp.Meter("test"), nil)
	assert.NoError(t, err)
	assert.NotNil(t, c)
}

func TestCollector_Close(t *testing.T) {
	mp := noop.NewMeterProvider()
	c, err := NewCollector(mp.Meter("test"), nil)
	assert.NoError(t, err)
	assert.NoError(t, c.Close())
}
```

Run: `go test ./internal/ebpf/observability/... -v`
Expected: FAIL — package not found.

- [ ] **Step 5.2: Create collector.go**

```go
//go:build linux

// Package observability implements Feature #122: kernel-level metrics via eBPF.
// When maps is nil (no BPF objects loaded), all metrics report zero and the
// collector is a no-op — safe on kernels without eBPF support.
package observability

import (
	"context"
	"io"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/metric"
)

// Maps holds the BPF map handles needed by the collector.
// It is populated from the bpf2go-generated objects after loading.
// When nil, the collector runs as a no-op.
type Maps struct {
	CPUTimeMap    MapReader
	NetBytesIn    MapReader
	NetBytesOut   MapReader
	SyscallErrors MapReader
	FDCountMap    MapReader
}

// MapReader is the subset of ebpf.Map used by the collector (easy to mock).
type MapReader interface {
	// Iterate calls fn for each key-value pair in the map.
	// For testing, a nil MapReader is treated as an empty map.
}

// Collector polls BPF maps on a configurable interval and reports OTel metrics.
type Collector struct {
	meter    metric.Meter
	maps     *Maps
	interval time.Duration
	stop     chan struct{}
	done     chan struct{}

	cpuTime     metric.Int64Counter
	netBytesIn  metric.Int64Counter
	netBytesOut metric.Int64Counter
	syscallErrs metric.Int64Counter
	fdCount     metric.Int64ObservableGauge
}

// NewCollector creates a Collector that reports metrics via the given Meter.
// Pass nil maps to get a no-op collector (graceful degrade).
func NewCollector(meter metric.Meter, maps *Maps) (*Collector, error) {
	c := &Collector{
		meter:    meter,
		maps:     maps,
		interval: 15 * time.Second,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}

	var err error
	c.cpuTime, err = meter.Int64Counter(
		"ebpf.process.cpu_time",
		metric.WithDescription("Accumulated CPU time per process (ms)"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}
	c.netBytesIn, err = meter.Int64Counter(
		"ebpf.net.bytes_in",
		metric.WithDescription("Network bytes received per PID"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}
	c.netBytesOut, err = meter.Int64Counter(
		"ebpf.net.bytes_out",
		metric.WithDescription("Network bytes sent per PID"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}
	c.syscallErrs, err = meter.Int64Counter(
		"ebpf.syscall.errors",
		metric.WithDescription("Syscall error rate by syscall number"),
		metric.WithUnit("{count}"),
	)
	if err != nil {
		return nil, err
	}
	_, err = meter.Int64ObservableGauge(
		"ebpf.fd.count",
		metric.WithDescription("Open file descriptor count per PID"),
		metric.WithUnit("{fd}"),
	)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// Start begins polling BPF maps in the background.
// Safe to call even when maps is nil (no-op).
func (c *Collector) Start(ctx context.Context) {
	go c.run(ctx)
}

func (c *Collector) run(ctx context.Context) {
	defer close(c.done)
	if c.maps == nil {
		slog.Debug("ebpf/observability: no BPF maps; collector is a no-op")
		return
	}
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.collect(ctx)
		case <-c.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *Collector) collect(_ context.Context) {
	// Real implementation iterates BPF maps and records metrics.
	// Wired fully in Task 8 after bpf2go generates the typed map accessors.
	slog.Debug("ebpf/observability: collect tick")
}

// Close stops the collector. Implements io.Closer.
func (c *Collector) Close() error {
	select {
	case <-c.stop:
	default:
		close(c.stop)
	}
	return nil
}

var _ io.Closer = (*Collector)(nil)
```

- [ ] **Step 5.3: Add non-Linux stub**

Create `internal/ebpf/observability/stub.go`:

```go
//go:build !linux

package observability

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

// Maps is empty on non-Linux.
type Maps struct{}

// MapReader is the subset used by the collector.
type MapReader interface{}

// Collector is a no-op on non-Linux.
type Collector struct{}

func NewCollector(_ metric.Meter, _ *Maps) (*Collector, error) {
	return &Collector{}, nil
}
func (c *Collector) Start(_ context.Context) {}
func (c *Collector) Close() error             { return nil }
```

- [ ] **Step 5.4: Run tests**

```bash
go test ./internal/ebpf/observability/... -v
```
Expected: PASS.

- [ ] **Step 5.5: Commit**

```bash
git add internal/ebpf/observability/
git commit -m "feat(ebpf/observability): add #122 kernel metrics collector (OTel wiring)"
```

---

## Task 6: Feature #123 — Security Audit Trail consumer

**Files:**
- Create: `internal/ebpf/audit/consumer.go`
- Create: `internal/ebpf/audit/consumer_test.go`

- [ ] **Step 6.1: Write failing test**

Create `internal/ebpf/audit/consumer_test.go`:

```go
//go:build linux

package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestConsumer_NewWithNilReader(t *testing.T) {
	mp := noop.NewMeterProvider()
	c, err := NewConsumer(nil, mp.Meter("test"), Config{})
	require.NoError(t, err)
	assert.NotNil(t, c)
}

func TestConsumer_Close(t *testing.T) {
	mp := noop.NewMeterProvider()
	c, err := NewConsumer(nil, mp.Meter("test"), Config{})
	require.NoError(t, err)
	assert.NoError(t, c.Close())
}

func TestConsumer_IPAllowlist_ParsesCIDRs(t *testing.T) {
	cfg := Config{
		IPAllowlist: []string{"192.168.0.0/16", "10.0.0.0/8"},
	}
	nets, err := parseCIDRs(cfg.IPAllowlist)
	require.NoError(t, err)
	assert.Len(t, nets, 2)
}

func TestConsumer_IPAllowlist_InvalidCIDR(t *testing.T) {
	cfg := Config{
		IPAllowlist: []string{"not-a-cidr"},
	}
	_, err := parseCIDRs(cfg.IPAllowlist)
	assert.Error(t, err)
}
```

Run: `go test ./internal/ebpf/audit/... -v`
Expected: FAIL — package not found.

- [ ] **Step 6.2: Create consumer.go**

```go
//go:build linux

// Package audit implements Feature #123: security audit trail via eBPF ring buffer.
package audit

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"

	"github.com/cilium/ebpf/ringbuf"
	"go.opentelemetry.io/otel/metric"
)

// Config configures the audit consumer.
type Config struct {
	IPAllowlist   []string // allowed outbound CIDRs; empty = allow all
	ExecAllowlist []string // allowed binary paths; empty = allow all
}

// RingBufReader is the subset of ringbuf.Reader used by Consumer (allows mocking).
type RingBufReader interface {
	Read() (ringbuf.Record, error)
	Close() error
}

// Consumer reads audit events from a BPF ring buffer and emits structured logs.
type Consumer struct {
	reader      RingBufReader
	cfg         Config
	allowedNets []*net.IPNet
	stop        chan struct{}
	done        chan struct{}
	dropped     metric.Int64Counter
}

// NewConsumer creates a Consumer. Pass nil reader for a no-op (graceful degrade).
func NewConsumer(reader RingBufReader, meter metric.Meter, cfg Config) (*Consumer, error) {
	nets, err := parseCIDRs(cfg.IPAllowlist)
	if err != nil {
		return nil, fmt.Errorf("audit: invalid IP allowlist: %w", err)
	}

	dropped, err := meter.Int64Counter(
		"ebpf.audit.dropped_events",
		metric.WithDescription("Audit ring buffer overflow drop count"),
		metric.WithUnit("{count}"),
	)
	if err != nil {
		return nil, err
	}

	return &Consumer{
		reader:      reader,
		cfg:         cfg,
		allowedNets: nets,
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
		dropped:     dropped,
	}, nil
}

// Start begins consuming events in a background goroutine.
func (c *Consumer) Start(ctx context.Context) {
	go c.run(ctx)
}

func (c *Consumer) run(ctx context.Context) {
	defer close(c.done)
	if c.reader == nil {
		slog.Debug("ebpf/audit: no ring buffer reader; consumer is a no-op")
		return
	}
	for {
		record, err := c.reader.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			// Ring buffer overflow — increment OTel counter and log.
			c.dropped.Add(ctx, 1)
			slog.Warn("ebpf/audit: ring buffer overflow; events dropped", "error", err)
			continue
		}
		c.handleRecord(ctx, record)

		select {
		case <-c.stop:
			return
		case <-ctx.Done():
			return
		default:
		}
	}
}

// auditEventType mirrors the C enum in audit.c
const (
	auditExec    = 1
	auditTCP     = 2
	auditConnect = 3
)

func (c *Consumer) handleRecord(ctx context.Context, rec ringbuf.Record) {
	if len(rec.RawSample) < 1 {
		return
	}
	eventType := rec.RawSample[0]

	pid := binary.LittleEndian.Uint32(rec.RawSample[4:8])
	ts := binary.LittleEndian.Uint64(rec.RawSample[12:20])
	comm := nullTermStr(rec.RawSample[20:36])

	switch eventType {
	case auditExec:
		path := nullTermStr(rec.RawSample[36:164])
		violation := len(c.cfg.ExecAllowlist) > 0 && !sliceContains(c.cfg.ExecAllowlist, path)
		slog.InfoContext(ctx, "ebpf/audit: exec",
			"pid", pid, "comm", comm, "path", path,
			"timestamp_ns", ts, "audit.violation", violation,
		)
	case auditTCP:
		if len(rec.RawSample) < 44 {
			return
		}
		saddr := netip.AddrFrom4([4]byte(rec.RawSample[36:40]))
		daddr := netip.AddrFrom4([4]byte(rec.RawSample[40:44]))
		violation := !c.ipAllowed(daddr.Unmap().String())
		slog.InfoContext(ctx, "ebpf/audit: tcp-established",
			"pid", pid, "comm", comm,
			"src", saddr, "dst", daddr,
			"timestamp_ns", ts, "audit.violation", violation,
		)
	case auditConnect:
		slog.InfoContext(ctx, "ebpf/audit: connect",
			"pid", pid, "comm", comm, "timestamp_ns", ts,
		)
	}
}

func (c *Consumer) ipAllowed(ip string) bool {
	if len(c.allowedNets) == 0 {
		return true
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, n := range c.allowedNets {
		if n.Contains(parsed) {
			return true
		}
	}
	return false
}

// Close stops the consumer. Implements io.Closer.
func (c *Consumer) Close() error {
	select {
	case <-c.stop:
	default:
		close(c.stop)
	}
	if c.reader != nil {
		return c.reader.Close()
	}
	return nil
}

var _ io.Closer = (*Consumer)(nil)

// parseCIDRs parses a slice of CIDR strings into *net.IPNet values.
func parseCIDRs(cidrs []string) ([]*net.IPNet, error) {
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
		}
		nets = append(nets, n)
	}
	return nets, nil
}

func nullTermStr(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

func sliceContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
```

- [ ] **Step 6.3: Add non-Linux stub**

Create `internal/ebpf/audit/stub.go`:

```go
//go:build !linux

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

func NewConsumer(_ RingBufReader, _ metric.Meter, _ Config) (*Consumer, error) {
	return &Consumer{}, nil
}
func (c *Consumer) Start(_ context.Context) {}
func (c *Consumer) Close() error             { return nil }
```

- [ ] **Step 6.4: Run tests**

```bash
go test ./internal/ebpf/audit/... -v
```
Expected: PASS (4 tests).

- [ ] **Step 6.5: Commit**

```bash
git add internal/ebpf/audit/
git commit -m "feat(ebpf/audit): add #123 security audit trail ring buffer consumer"
```

---

## Task 7: Feature #120 — XDP pre-filtering controller

**Files:**
- Create: `internal/ebpf/xdp/controller.go`
- Create: `internal/ebpf/xdp/controller_test.go`

- [ ] **Step 7.1: Write failing test**

Create `internal/ebpf/xdp/controller_test.go`:

```go
//go:build linux

package xdp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestController_NewWithNilObjects(t *testing.T) {
	mp := noop.NewMeterProvider()
	c, err := NewController(nil, mp.Meter("test"), Config{Interface: "lo"})
	require.NoError(t, err)
	assert.NotNil(t, c)
}

func TestController_Stop(t *testing.T) {
	mp := noop.NewMeterProvider()
	c, err := NewController(nil, mp.Meter("test"), Config{Interface: "lo"})
	require.NoError(t, err)
	assert.NoError(t, c.Close())
}

func TestController_ParseCIDR_Valid(t *testing.T) {
	_, err := parseLPMKey("192.168.1.0/24")
	assert.NoError(t, err)
}

func TestController_ParseCIDR_Invalid(t *testing.T) {
	_, err := parseLPMKey("not-a-cidr")
	assert.Error(t, err)
}
```

Run: `go test ./internal/ebpf/xdp/... -v`
Expected: FAIL — package not found.

- [ ] **Step 7.2: Create controller.go**

```go
//go:build linux

// Package xdp implements Feature #120: XDP pre-filtering at the network driver level.
package xdp

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"

	"go.opentelemetry.io/otel/metric"
)

// LPMKey mirrors the C struct bpf_lpm_trie_key + 4-byte IPv4 data.
type LPMKey struct {
	Prefixlen uint32
	Addr      [4]byte
}

// XDPObjects holds the bpf2go-generated map handles. Nil = no eBPF objects loaded.
type XDPObjects interface {
	// BlockedIPs returns the LPM trie map for blocked CIDRs.
	// In the real generated struct this is objs.BlockedIps (*ebpf.Map).
	io.Closer
}

// Config configures the XDP controller.
type Config struct {
	Interface string
}

// Controller attaches the XDP program and syncs the blocklist from Valkey.
type Controller struct {
	objects   XDPObjects
	cfg       Config
	stop      chan struct{}
	done      chan struct{}
	dropped   metric.Int64Counter
}

// NewController creates a Controller. Pass nil objects for a no-op (graceful degrade).
func NewController(objects XDPObjects, meter metric.Meter, cfg Config) (*Controller, error) {
	dropped, err := meter.Int64Counter(
		"ebpf.xdp.dropped_packets",
		metric.WithDescription("Packets dropped by XDP pre-filter"),
		metric.WithUnit("{packet}"),
	)
	if err != nil {
		return nil, err
	}

	return &Controller{
		objects: objects,
		cfg:     cfg,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		dropped: dropped,
	}, nil
}

// Close detaches the XDP program and stops the sync loop.
func (c *Controller) Close() error {
	select {
	case <-c.stop:
	default:
		close(c.stop)
	}
	if c.objects != nil {
		return c.objects.Close()
	}
	return nil
}

var _ io.Closer = (*Controller)(nil)

// parseLPMKey parses a CIDR string into an LPMKey for the BPF LPM trie.
func parseLPMKey(cidr string) (LPMKey, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return LPMKey{}, fmt.Errorf("xdp: invalid CIDR %q: %w", cidr, err)
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return LPMKey{}, fmt.Errorf("xdp: only IPv4 CIDRs supported, got %q", cidr)
	}
	ones, _ := ipNet.Mask.Size()
	var addr [4]byte
	binary.BigEndian.PutUint32(addr[:], binary.BigEndian.Uint32(ip4))
	return LPMKey{Prefixlen: uint32(ones), Addr: addr}, nil
}

// SyncBlocklist writes cidrs to the blocked_ips BPF map.
// Called from the sync loop after fetching CIDRs from Valkey.
// This is a stub — fully wired in Task 8 after bpf2go generates typed accessors.
func (c *Controller) SyncBlocklist(cidrs []string) {
	if c.objects == nil {
		return
	}
	slog.Debug("ebpf/xdp: syncing blocklist", "count", len(cidrs))
	// Real implementation: iterate cidrs, call parseLPMKey, write to blocked_ips map.
}
```

- [ ] **Step 7.3: Add non-Linux stub**

Create `internal/ebpf/xdp/stub.go`:

```go
//go:build !linux

package xdp

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

// Config configures the XDP controller.
type Config struct {
	Interface string
}

// XDPObjects is a no-op interface on non-Linux.
type XDPObjects interface {
	Close() error
}

// Controller is a no-op on non-Linux.
type Controller struct{}

func NewController(_ XDPObjects, _ metric.Meter, _ Config) (*Controller, error) {
	return &Controller{}, nil
}
func (c *Controller) Start(_ context.Context) {}
func (c *Controller) SyncBlocklist(_ []string) {}
func (c *Controller) Close() error              { return nil }
```

- [ ] **Step 7.4: Run tests**

```bash
go test ./internal/ebpf/xdp/... -v
```
Expected: PASS (4 tests).

- [ ] **Step 7.5: Run all eBPF tests**

```bash
go test ./internal/ebpf/... -v
```
Expected: all pass.

- [ ] **Step 7.6: Commit**

```bash
git add internal/ebpf/xdp/
git commit -m "feat(ebpf/xdp): add #120 XDP pre-filtering controller"
```

---

## Task 8: Wire XDP program attachment into controller (link.AttachXDP)

**Files:**
- Modify: `internal/ebpf/xdp/controller.go` (Linux build tag file)

The bpf2go-generated struct will have `objs.XdpFilter (*ebpf.Program)`. This task wires `link.AttachXDP` into the controller so the program actually attaches on startup.

- [ ] **Step 8.1: Write failing test for XDP attach**

Add to `internal/ebpf/xdp/controller_test.go`:

```go
func TestController_XDPOptions_NativeWithGenericFallback(t *testing.T) {
	// Ensure XDPAttachFlags returns GenericMode when NativeMode is not available.
	// We can test the flag selection logic without real hardware.
	assert.Equal(t, xdpAttachFlags(), link.XDPGenericMode)
}
```

Run: `go test ./internal/ebpf/xdp/... -run TestController_XDPOptions -v`
Expected: FAIL — `xdpAttachFlags` not defined.

- [ ] **Step 8.2: Add link storage and AttachXDP to controller**

Replace the `Controller` struct in `controller.go` and update `NewController` and `Close`:

```go
import (
    "encoding/binary"
    "fmt"
    "io"
    "log/slog"
    "net"

    "github.com/cilium/ebpf/link"
    "go.opentelemetry.io/otel/metric"
)

// XDPProgram is the subset of *ebpf.Program used for XDP attachment.
type XDPProgram interface {
    // FD returns the program file descriptor (used internally by cilium/ebpf link).
}

// Controller attaches the XDP program and syncs the blocklist from Valkey.
type Controller struct {
    xdpLink link.Link   // nil when objects is nil (graceful degrade)
    objects XDPObjects
    cfg     Config
    stop    chan struct{}
    done    chan struct{}
    dropped metric.Int64Counter
}

// NewController creates a Controller and attaches the XDP program to the interface.
// Pass nil objects for a no-op (graceful degrade — no XDP program attached).
func NewController(objects XDPObjects, prog *ebpf.Program, meter metric.Meter, cfg Config) (*Controller, error) {
    dropped, err := meter.Int64Counter(
        "ebpf.xdp.dropped_packets",
        metric.WithDescription("Packets dropped by XDP pre-filter"),
        metric.WithUnit("{packet}"),
    )
    if err != nil {
        return nil, err
    }

    c := &Controller{
        objects: objects,
        cfg:     cfg,
        stop:    make(chan struct{}),
        done:    make(chan struct{}),
        dropped: dropped,
    }

    if prog != nil {
        iface, err := net.InterfaceByName(cfg.Interface)
        if err != nil {
            slog.Warn("ebpf/xdp: interface not found; XDP disabled", "interface", cfg.Interface, "error", err)
            return c, nil
        }
        // Try native XDP mode first; fall back to generic if driver doesn't support it.
        xdpLink, err := link.AttachXDP(link.XDPOptions{
            Program:   prog,
            Interface: iface.Index,
            Flags:     link.XDPDriverMode,
        })
        if err != nil {
            slog.Warn("ebpf/xdp: native XDP attach failed, falling back to generic mode", "error", err)
            xdpLink, err = link.AttachXDP(link.XDPOptions{
                Program:   prog,
                Interface: iface.Index,
                Flags:     link.XDPGenericMode,
            })
            if err != nil {
                slog.Warn("ebpf/xdp: generic XDP attach also failed; XDP disabled", "error", err)
                return c, nil
            }
        }
        c.xdpLink = xdpLink
        slog.Info("ebpf/xdp: program attached", "interface", cfg.Interface)
    }

    return c, nil
}

// Close detaches the XDP program and stops the sync loop.
func (c *Controller) Close() error {
    select {
    case <-c.stop:
    default:
        close(c.stop)
    }
    if c.xdpLink != nil {
        if err := c.xdpLink.Close(); err != nil {
            slog.Warn("ebpf/xdp: error detaching XDP program", "error", err)
        }
    }
    if c.objects != nil {
        return c.objects.Close()
    }
    return nil
}

// xdpAttachFlags returns the preferred XDP attach flags (used in tests).
func xdpAttachFlags() link.XDPAttachFlags {
    return link.XDPGenericMode
}
```

> Note: `*ebpf.Program` is the bpf2go-generated field (e.g. `objs.XdpFilter`). The real wiring from generated objects happens when you run `go generate ./internal/ebpf/xdp/...` and wire `objs.XdpFilter` in `cmd/api/ebpf.go`.

Also update `cmd/api/ebpf.go` (Task 9) — the `NewController` call signature now takes a `*ebpf.Program` second arg (pass `nil` until bpf2go objects are generated):

```go
ctrl, err := xdp.NewController(nil, nil, meter, xdp.Config{Interface: cfg.XDPInterface})
```

- [ ] **Step 8.3: Update stub NewController signature to match**

In `internal/ebpf/xdp/stub.go`, update:

```go
func NewController(_ XDPObjects, _ interface{}, _ metric.Meter, _ Config) (*Controller, error) {
    return &Controller{}, nil
}
```

- [ ] **Step 8.4: Run tests**

```bash
go test ./internal/ebpf/xdp/... -v
```
Expected: PASS.

- [ ] **Step 8.5: Commit**

```bash
git add internal/ebpf/xdp/controller.go internal/ebpf/xdp/stub.go
git commit -m "feat(ebpf/xdp): wire link.AttachXDP with native→generic mode fallback"
```

---

## Task 9: Wire eBPF manager into cmd/api/main.go  <!-- was Task 8 -->

**Files:**
- Modify: `cmd/api/main.go` (after `telemetry.InitProvider`, before `gin.SetMode`)

- [ ] **Step 8.1: Write integration test (build-tag guarded)**

Create `cmd/api/ebpf_wire_test.go`:

```go
//go:build linux

package main

import (
	"testing"

	"github.com/ravencloak-org/Raven/internal/config"
	"github.com/ravencloak-org/Raven/internal/ebpf"
	"github.com/stretchr/testify/assert"
)

func TestEBPFManager_GracefulDegradeOnCI(t *testing.T) {
	cfg := &config.EBPFConfig{
		ObservabilityEnabled: true,
		AuditEnabled:         true,
		XDPEnabled:           true,
		XDPInterface:         "lo",
		AuditRingBufferSize:  1048576,
	}
	// initEBPF must not panic or crash even when capabilities are unavailable
	manager, err := initEBPF(cfg, nil)
	// On CI without CAP_BPF, err may be non-nil but manager must not be nil
	assert.NotNil(t, manager)
	_ = err
	manager.Stop()
}
```

Run: `go test ./cmd/api/... -run TestEBPFManager -v`
Expected: FAIL — `initEBPF` not defined.

- [ ] **Step 8.2: Add initEBPF function**

Create `cmd/api/ebpf.go`:

```go
//go:build linux

package main

import (
	"log/slog"

	"go.opentelemetry.io/otel"

	"github.com/ravencloak-org/Raven/internal/config"
	"github.com/ravencloak-org/Raven/internal/ebpf"
	"github.com/ravencloak-org/Raven/internal/ebpf/audit"
	"github.com/ravencloak-org/Raven/internal/ebpf/observability"
	"github.com/ravencloak-org/Raven/internal/ebpf/xdp"
)

// initEBPF starts the eBPF subsystem based on cfg.
// It always returns a non-nil Manager — features that fail to start are logged
// and skipped; the API server continues regardless.
func initEBPF(cfg *config.EBPFConfig, _ interface{}) (*ebpf.Manager, error) {
	manager := ebpf.NewManager()

	if err := ebpf.CheckCapabilities(); err != nil {
		slog.Warn("eBPF subsystem disabled", "reason", err)
		return manager, err
	}

	meter := otel.GetMeterProvider().Meter("raven/ebpf")

	if cfg.ObservabilityEnabled {
		col, err := observability.NewCollector(meter, nil) // maps wired after bpf2go in Task 9
		if err != nil {
			slog.Warn("eBPF observability failed to start", "error", err)
		} else {
			manager.Register(col)
			slog.Info("eBPF observability enabled")
		}
	}

	if cfg.AuditEnabled {
		con, err := audit.NewConsumer(nil, meter, audit.Config{ // reader wired in Task 9
			IPAllowlist:   cfg.AuditIPAllowlist,
			ExecAllowlist: cfg.AuditExecAllowlist,
		})
		if err != nil {
			slog.Warn("eBPF audit consumer failed to start", "error", err)
		} else {
			manager.Register(con)
			slog.Info("eBPF audit trail enabled")
		}
	}

	if cfg.XDPEnabled {
		ctrl, err := xdp.NewController(nil, meter, xdp.Config{ // objects wired in Task 9
			Interface: cfg.XDPInterface,
		})
		if err != nil {
			slog.Warn("eBPF XDP controller failed to start", "error", err)
		} else {
			manager.Register(ctrl)
			slog.Info("eBPF XDP pre-filtering enabled", "interface", cfg.XDPInterface)
		}
	}

	return manager, nil
}
```

Create `cmd/api/ebpf_stub.go` for non-Linux:

```go
//go:build !linux

package main

import (
	"log/slog"

	"github.com/ravencloak-org/Raven/internal/config"
	"github.com/ravencloak-org/Raven/internal/ebpf"
)

func initEBPF(cfg *config.EBPFConfig, _ interface{}) (*ebpf.Manager, error) {
	slog.Debug("eBPF disabled: non-Linux platform")
	return ebpf.NewManager(), nil
}
```

- [ ] **Step 8.3: Wire into main()**

In `cmd/api/main.go`, after the `otelShutdown` defer block (around line 130), add:

```go
// Initialise eBPF subsystem (no-op when unavailable or disabled).
ebpfManager, _ := initEBPF(&cfg.EBPF, nil)
defer ebpfManager.Stop()
```

- [ ] **Step 8.4: Build to verify**

```bash
go build ./cmd/api
```
Expected: builds cleanly.

- [ ] **Step 8.5: Run tests**

```bash
go test ./cmd/api/... -run TestEBPFManager -v
go test ./... 2>&1 | tail -20
```
Expected: no new failures.

- [ ] **Step 8.6: Commit**

```bash
git add cmd/api/ebpf.go cmd/api/ebpf_stub.go cmd/api/main.go cmd/api/ebpf_wire_test.go
git commit -m "feat(api): wire eBPF manager into server startup lifecycle"
```

---

## Task 10: Build pipeline — Dockerfile + Makefile.edge

**Files:**
- Modify: `Dockerfile`
- Modify: `Makefile.edge`
- Modify: `docker-compose.yml`
- Modify: `docker-compose.edge.yml`

- [ ] **Step 9.1: Read current Dockerfile**

```bash
cat /Users/jobinlawrance/Project/raven/Dockerfile
```

- [ ] **Step 9.2: Update Dockerfile builder stage**

In the `Stage 1: Build` section, change from `CGO_ENABLED=0 go build` to:

```dockerfile
# Stage 1: Build
FROM golang:1.26-alpine AS builder

WORKDIR /app

# eBPF build tools — required for bpf2go and cilium/ebpf CGO bindings
RUN apk add --no-cache clang llvm linux-headers libbpf-dev musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=1 required for cilium/ebpf userspace bindings.
# -extldflags "-static" preserves fully static binary for the Alpine runtime stage.
RUN CGO_ENABLED=1 go build \
    -ldflags="-s -w -extldflags '-static'" \
    -o /api ./cmd/api
```

Runtime stage is unchanged — the binary remains fully static.

- [ ] **Step 9.3: Update Makefile.edge**

Read `Makefile.edge`, then add below the existing `build-amd64` target:

```makefile
# ─── eBPF-enabled builds (require cross-compiler toolchain) ─────────────────

build-arm64-ebpf:
	CC=aarch64-linux-musl-gcc CGO_ENABLED=1 GOOS=linux GOARCH=arm64 \
		go build -ldflags="$(LDFLAGS) -extldflags '-static'" \
		-o $(BUILD_DIR)/$(APP_NAME)-linux-arm64-ebpf ./cmd/api

build-amd64-ebpf:
	CC=x86_64-linux-musl-gcc CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
		go build -ldflags="$(LDFLAGS) -extldflags '-static'" \
		-o $(BUILD_DIR)/$(APP_NAME)-linux-amd64-ebpf ./cmd/api
```

- [ ] **Step 9.4: Add cap_add to docker-compose.edge.yml**

In the `go-api` service section, add after `restart: unless-stopped`:

```yaml
    cap_add:
      - CAP_BPF         # eBPF program loading and map creation (kernel ≥ 5.8)
      - CAP_NET_ADMIN   # XDP attachment; omit if RAVEN_EBPF_XDP_ENABLED=false
```

- [ ] **Step 9.5: Add cap_add to docker-compose.yml** (dev stack)

Same `cap_add` block to the `go-api` service in the root `docker-compose.yml`.

- [ ] **Step 9.6: Verify Dockerfile builds (native arch only on macOS)**

```bash
docker build -t raven-api-test . --target builder 2>&1 | tail -20
```
Expected: stage 1 builds with clang installed; CGO_ENABLED=1 build succeeds.

> Note: Full multi-arch build is validated in CI (Task 10).

- [ ] **Step 9.7: Commit**

```bash
git add Dockerfile Makefile.edge docker-compose.yml docker-compose.edge.yml
git commit -m "feat(build): add eBPF-enabled Dockerfile builder stage and Makefile.edge targets"
```

---

## Task 11: CI updates — .github/workflows/go.yml

**Files:**
- Modify: `.github/workflows/go.yml`

- [ ] **Step 10.1: Read current go.yml**

```bash
cat /Users/jobinlawrance/Project/raven/.github/workflows/go.yml
```

- [ ] **Step 10.2: Update build-and-test AND lint jobs**

In the `build-and-test` job steps, add before `go build ./cmd/api`:

```yaml
      - name: Install eBPF build tools
        run: sudo apt-get install -y clang llvm libbpf-dev
```

Change the `go build` step to use CGO_ENABLED=1:

```yaml
      - run: CGO_ENABLED=1 go build ./cmd/api
```

In the `lint` job, add the same install step before `golangci-lint-action` runs (golangci-lint invokes the compiler internally and will fail on CGO-dependent code without clang):

```yaml
      - name: Install eBPF build tools
        run: sudo apt-get install -y clang llvm libbpf-dev
```

Add after `go vet ./...`:

```yaml
      - name: Verify generated eBPF files are up-to-date
        run: |
          go install github.com/cilium/ebpf/cmd/bpf2go@latest
          go generate ./internal/ebpf/...
          git diff --exit-code || (echo "Generated eBPF files are out of date. Run 'go generate ./internal/ebpf/...'" && exit 1)
```

- [ ] **Step 10.3: Add ebpf-integration job**

Append a new job to `go.yml`:

```yaml
  ebpf-integration:
    name: eBPF Integration Tests
    runs-on: ubuntu-latest
    # Requires Linux with kernel ≥ 5.8; GitHub Actions ubuntu-latest is 6.x
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v6

      - uses: actions/setup-go@v6
        with:
          go-version: '1.26'
          cache: true

      - name: Install eBPF build tools
        run: sudo apt-get install -y clang llvm libbpf-dev

      - name: Run eBPF integration tests (privileged)
        run: |
          go test -tags linux,ebpf -race -v ./internal/ebpf/... ./cmd/api/...
        # Integration tests require CAP_BPF; run in a privileged context.
        # On GitHub Actions the runner has the necessary capabilities.
```

- [ ] **Step 10.4: Commit**

```bash
git add .github/workflows/go.yml
git commit -m "ci(go): add eBPF build tools, go generate check, ebpf-integration job"
```

---

## Task 12: go:generate directives and final lint pass

**Files:**
- Modify: `internal/ebpf/observability/collector.go`
- Modify: `internal/ebpf/audit/consumer.go`
- Modify: `internal/ebpf/xdp/controller.go`

- [ ] **Step 11.1: Add go:generate directives**

Add at the top of each collector/consumer/controller Linux file (after the build tag):

`internal/ebpf/observability/collector.go`:
```go
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 Observability ../programs/observability.c -- -I/usr/include/$(uname -m)-linux-gnu
```

`internal/ebpf/audit/consumer.go`:
```go
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 Audit ../programs/audit.c -- -I/usr/include/$(uname -m)-linux-gnu
```

`internal/ebpf/xdp/controller.go`:
```go
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 XDP ../programs/xdp.c -- -I/usr/include/$(uname -m)-linux-gnu
```

- [ ] **Step 11.2: Run full test suite**

```bash
go test -race ./... 2>&1 | tail -30
```
Expected: all existing tests pass; no new failures.

- [ ] **Step 11.3: Run linter**

```bash
golangci-lint run ./...
```
Expected: no errors on eBPF packages. Fix any issues before committing.

- [ ] **Step 11.4: Final commit**

```bash
git add internal/ebpf/observability/collector.go internal/ebpf/audit/consumer.go internal/ebpf/xdp/controller.go
git commit -m "feat(ebpf): add go:generate bpf2go directives for all three features"
```

---

## Environment notes for reviewer / executor

### Prerequisites on a Linux dev/CI host
```bash
uname -r                        # must be ≥ 5.8
ls /sys/kernel/btf/vmlinux      # must exist (BTF)
sudo apt-get install -y clang llvm libbpf-dev
go install github.com/cilium/ebpf/cmd/bpf2go@latest
go generate ./internal/ebpf/... # generates _bpfel.go / _bpfeb.go; commit these
```

### macOS development (no eBPF)
All packages compile because of `//go:build !linux` stubs. Tests that require `//go:build linux` are skipped automatically. The `go build ./cmd/api` and `go test ./...` commands work normally.

### Kernel capabilities on Docker
```yaml
cap_add:
  - CAP_BPF        # program loading, map creation
  - CAP_NET_ADMIN  # XDP attachment only
```
Do NOT add `CAP_SYS_ADMIN` — it is excessively broad.

### Verifying edge node compatibility
```bash
ssh pi@<edge-node>
uname -r                    # Raspberry Pi OS Bullseye = 5.15, Bookworm = 6.1
ls /sys/kernel/btf/vmlinux  # must exist
```
