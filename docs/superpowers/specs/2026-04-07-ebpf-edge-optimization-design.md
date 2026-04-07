# eBPF Edge Optimization — Design Spec

**Date:** 2026-04-07
**Issues:** #120 (XDP Pre-filtering), #122 (Kernel Observability), #123 (Security Audit Trail)
**Milestone:** Edge Optimization (M10)
**Tier:** Enterprise

---

## Overview

Three eBPF-based features built on a shared Go foundation inside the Raven API binary. No separate monitoring agents, no extra processes on edge nodes. All features degrade gracefully to no-op when eBPF is unavailable (missing capabilities, non-Linux, kernel too old).

**Implementation order:**
1. Shared foundation (`internal/ebpf/`)
2. #122 Kernel Observability — validates foundation, lowest risk
3. #123 Security Audit Trail — ring buffer pattern, builds on #122
4. #120 XDP Pre-filtering — most complex, isolated blast radius

---

## Architecture

### Shared Foundation: `internal/ebpf/`

One package owns the eBPF lifecycle. All three features depend on it.

**Files:**

| File | Responsibility |
|------|---------------|
| `capabilities.go` | Check `CAP_BPF` / `CAP_NET_ADMIN` at startup; detect kernel version (floor: 5.8); return typed error if missing; calls `setrlimit(RLIMIT_MEMLOCK)` on kernels < 5.11 |
| `loader.go` | Wrap `cilium/ebpf` collection loading; detect BTF availability; return typed handles |
| `manager.go` | Lifecycle: `Start()` / `Stop()` / `io.Closer`; detach all probes on shutdown; hooked into API server SIGTERM |
| `maps.go` | Typed helpers for BPF hash maps and ring buffers shared between kernel and userspace |
| `programs/` | All `.c` BPF programs; compiled by `bpf2go` at build time into embedded `_bpfel.go`/`_bpfeb.go` |

**Key invariant:** If eBPF is unavailable at runtime, all features self-disable with a structured `slog` warning. The API server starts and serves normally.

**Library:** `github.com/cilium/ebpf` (Cilium's ebpf-go). BPF C programs compiled via `bpf2go` — bytecode embedded into the Go binary at build time. No runtime C toolchain required on edge nodes.

---

## Feature #122: Kernel-level Observability

**Package:** `internal/ebpf/observability/`
**Config:** `RAVEN_EBPF_OBSERVABILITY_ENABLED=true`

### BPF Programs (`programs/observability.c`)

| Program type | Hook | Metric |
|---|---|---|
| `tp_btf` | `sched_switch` | Per-process CPU time (via `prev_sum_exec_runtime` delta) |
| `tracepoint` | `raw_syscalls/sys_exit` | Syscall error rates by syscall nr (all syscalls, single tracepoint) |
| `tracepoint` | `net/net_dev_start_xmit` + `net/netif_receive_skb` | Network bytes in/out per PID |
| `kprobe` | `__fd_install` | File descriptor count (kernel-internal symbol; monitor for rename across kernel upgrades) |

### Userspace (`observability/collector.go`)

- Polls BPF maps on configurable interval (default 15s)
- Converts raw BPF counters → OTel `Gauge`/`Counter` instruments
- Registers with existing `MeterProvider` from `internal/telemetry/`
- `InitProvider()` in `internal/telemetry/telemetry.go` optionally initialises collector after OTel is ready

**New OTel metrics exported:**

| Metric | Type | Unit |
|--------|------|------|
| `ebpf.process.cpu_time` | Counter | milliseconds |
| `ebpf.net.bytes_in` | Counter | bytes |
| `ebpf.net.bytes_out` | Counter | bytes |
| `ebpf.syscall.errors` | Counter | count |
| `ebpf.fd.count` | Gauge | count |

**Replaces:** Prometheus node exporter on edge nodes — zero additional process on Pi.

---

## Feature #123: Security Audit Trail

**Package:** `internal/ebpf/audit/`
**Config:** `RAVEN_EBPF_AUDIT_ENABLED=true`

### BPF Programs (`programs/audit.c`)

| Program type | Hook | Event |
|---|---|---|
| `tracepoint` | `syscalls/sys_enter_execve` | Process spawn: PID, binary path, parent PID, timestamp |
| `tracepoint` | `sock/inet_sock_set_state` | TCP connection established: src/dst IP, port |
| `tracepoint` | `syscalls/sys_enter_connect` | Outbound connect: destination IP + port |

Events written to a **BPF ring buffer** — efficient, ordered, no polling overhead.

**Ring buffer sizing:** `RAVEN_EBPF_AUDIT_RING_BUFFER_SIZE` (default `1048576` = 1MB, must be a power of 2). On overflow, the kernel drops events and increments a lost-event counter.

### Userspace (`audit/consumer.go`)

- Reads ring buffer in dedicated goroutine via `ringbuf.Reader`
- On `ringbuf.ErrRingbufferFull`: increments `ebpf.audit.dropped_events` OTel counter (Counter, unit: count) and logs a warning — no panic, no crash
- Emits structured `slog` JSON log entries into existing logging pipeline (same OTLP endpoint)
- Configurable IP allowlist in a BPF hash map — alerts on connections outside the list
- Configurable exec path allowlist — alerts on unexpected binary spawns
- All alerts tagged with `audit.violation=true` for downstream SIEM filtering

**Config vars:**

| Var | Default | Purpose |
|-----|---------|---------|
| `RAVEN_EBPF_AUDIT_IP_ALLOWLIST` | `""` | Comma-separated CIDRs allowed for outbound |
| `RAVEN_EBPF_AUDIT_EXEC_ALLOWLIST` | `""` | Comma-separated binary paths allowed to exec |
| `RAVEN_EBPF_AUDIT_RING_BUFFER_SIZE` | `1048576` | Ring buffer size in bytes (power of 2) |

**New OTel metrics exported (Feature #123):**

| Metric | Type | Unit |
|--------|------|------|
| `ebpf.audit.dropped_events` | Counter | count |

**Power-of-2 validation:** `internal/config/config.go` validates `AuditRingBufferSize` on load using `bits.OnesCount(uint(size)) == 1 && size > 0`. A non-power-of-2 value returns a clear config error before any BPF load is attempted, preventing the opaque kernel error from `ringbuf.NewReader`.

**Compliance target:** GDPR/SOC2 audit log of all process activity and network connections on edge nodes.

---

## Feature #120: XDP Pre-filtering

**Package:** `internal/ebpf/xdp/`
**Config:** `RAVEN_EBPF_XDP_ENABLED=true`, `RAVEN_EBPF_XDP_INTERFACE=eth0`

### BPF Program (`programs/xdp.c`)

```
XDP hook on primary NIC
→ Parse Ethernet/IP header
→ Lookup src IP in blocked_ips BPF hash map
   → BLOCKED:   XDP_DROP  (never reaches TCP stack)
   → THROTTLED: XDP_DROP if rate exceeded, else XDP_PASS
   → DEFAULT:   XDP_PASS
```

**Two BPF maps:**

| Map | Type | Content |
|-----|------|---------|
| `blocked_ips` | `BPF_MAP_TYPE_LPM_TRIE` | Permanently blocked CIDRs; uses LPM for prefix matching. Go-side insert uses `struct bpf_lpm_trie_key` + 4-byte IPv4 prefix data. |
| `throttle_state` | LRU Hash | Per-IP packet counters + timestamps |

### Userspace (`xdp/controller.go`)

- Attaches XDP program to interface on startup (native mode → generic fallback if driver unsupported)
- `SyncLoop()` polls Valkey for blocklist updates every 30s and writes to `blocked_ips` BPF map
- No Valkey dependency at packet drop time — BPF map is the runtime source of truth
- Exports `ebpf.xdp.dropped_packets` OTel counter
- Detaches cleanly on `SIGTERM` / `Stop()`

**Relationship to existing rate limiting:** Complements (does not replace) the Valkey sliding-window rate limiter in `internal/middleware/`. XDP acts before TCP; middleware acts after HTTP parse.

**Fallback:** If XDP attach fails (missing `CAP_NET_ADMIN`, unsupported NIC driver), logs warning and falls back to app-layer rate limiting only. Nothing crashes.

---

## Configuration

All flags live in a new `EBPFConfig` struct added to `internal/config/config.go`, loaded via Viper.

```go
type EBPFConfig struct {
    ObservabilityEnabled  bool
    AuditEnabled          bool
    AuditIPAllowlist      []string
    AuditExecAllowlist    []string
    AuditRingBufferSize   int      // bytes, must be power of 2
    XDPEnabled            bool
    XDPInterface          string
}
```

| Env var | Default | Feature |
|---------|---------|---------|
| `RAVEN_EBPF_OBSERVABILITY_ENABLED` | `false` | #122 kprobe metrics |
| `RAVEN_EBPF_AUDIT_ENABLED` | `false` | #123 audit trail |
| `RAVEN_EBPF_AUDIT_IP_ALLOWLIST` | `""` | Comma-sep CIDRs |
| `RAVEN_EBPF_AUDIT_EXEC_ALLOWLIST` | `""` | Comma-sep binary paths |
| `RAVEN_EBPF_AUDIT_RING_BUFFER_SIZE` | `1048576` | Ring buffer bytes (power of 2) |
| `RAVEN_EBPF_XDP_ENABLED` | `false` | #120 XDP drop |
| `RAVEN_EBPF_XDP_INTERFACE` | `eth0` | NIC to attach XDP to |

All features default to `false` — opt-in only, safe for existing deployments.

---

## Build Pipeline Changes

### Dockerfile (builder stage)

Change the builder stage (currently `CGO_ENABLED=0`):
```dockerfile
RUN apk add --no-cache clang llvm linux-headers libbpf-dev musl-dev
RUN go install github.com/cilium/ebpf/cmd/bpf2go@latest
# CGO_ENABLED=1 required for cilium/ebpf userspace bindings
# -extldflags "-static" preserves fully static binary for Alpine runtime stage
RUN CGO_ENABLED=1 go build -ldflags="-s -w -extldflags '-static'" -o /api ./cmd/api
```

Runtime stage unchanged — BPF bytecode embedded at build time, binary remains fully static.

### CGO & Makefile.edge

`CGO_ENABLED=0` (current) breaks `cilium/ebpf`. `Makefile.edge` must split targets:

```makefile
# Existing non-eBPF build (unchanged)
build-arm64:
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
        go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-linux-arm64 ./cmd/api

# New eBPF-enabled build
build-arm64-ebpf:
    CC=aarch64-linux-musl-gcc CGO_ENABLED=1 GOOS=linux GOARCH=arm64 \
        go build -ldflags="$(LDFLAGS) -extldflags '-static'" \
        -o $(BUILD_DIR)/$(APP_NAME)-linux-arm64-ebpf ./cmd/api
```

Non-eBPF edge builds are unaffected.

### go generate & Generated Files

Each feature package contains a `//go:generate` directive:
```go
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 Observability ../programs/observability.c
```

**Generated `_bpfel.go`/`_bpfeb.go` files are committed to the repository** (standard cilium/ebpf practice). Edge nodes require no C toolchain at runtime.

CI workflow (`.github/workflows/go.yml`) additions needed:
- Install `clang` on the runner: `sudo apt-get install -y clang llvm libbpf-dev`
- Add a `make generate && git diff --exit-code` step to verify generated files are up-to-date
- eBPF integration tests run on a separate `ebpf-integration` job with `--privileged` Docker

---

## Docker / Deployment Changes

Both `docker-compose.yml` and `docker-compose.edge.yml` — `go-api` service:

```yaml
cap_add:
  - CAP_BPF        # eBPF program loading, map creation (kernel >= 5.8)
  - CAP_NET_ADMIN  # XDP attachment only; omit if XDP disabled
```

`CAP_SYS_ADMIN` is **not** added — it is excessively broad. Minimum supported kernel is 5.8 where `CAP_BPF` suffices. `capabilities.go` detects the kernel version at startup and logs a clear error if the floor is not met rather than relying on container capabilities to paper over it.

**Kernel floor:** >= 5.8 with `CONFIG_DEBUG_INFO_BTF=y`. Raspberry Pi OS Bullseye (5.15 LTS) and Bookworm (6.1 LTS) both qualify.

**Prerequisite check on edge nodes:**
```bash
uname -r                          # must be >= 5.8
ls /sys/kernel/btf/vmlinux        # must exist (BTF enabled)
```

---

## Testing Strategy

| Layer | Approach |
|-------|----------|
| Unit | Mock `cilium/ebpf` interfaces; test config parsing, map helpers, graceful-degrade paths |
| Integration | Linux VM (GitHub Actions runner or local Docker with `--privileged`); load real BPF programs, verify maps populated |
| Observability | Assert OTel metrics appear in test meter provider after collector poll |
| XDP | Use `AF_XDP` loopback test or `veth` pair to send synthetic packets; verify drop counters |
| Audit | Exec a known binary in test; verify ring buffer emits correct event |

All eBPF integration tests are gated behind `//go:build linux,ebpf` build tag — skipped automatically on macOS and in environments without capabilities.

---

## Non-Goals

- No Windows or macOS eBPF support (Linux-only feature, gracefully no-ops elsewhere)
- No replacement of the Python AI worker observability (separate concern)
- No user-facing UI for audit logs in this milestone (raw OTLP pipeline output only)
- XDP does not replace the Valkey rate limiter — they are complementary layers
