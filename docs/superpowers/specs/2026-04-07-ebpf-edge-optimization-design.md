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
| `capabilities.go` | Check `CAP_BPF` / `CAP_SYS_ADMIN` at startup; return typed error if missing |
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
| `kprobe` | `finish_task_switch` | Per-process CPU time |
| `tracepoint` | `sys_exit` | Syscall error rates by syscall nr |
| `tracepoint` | `net/net_dev_xmit` + `netif_receive_skb` | Network bytes in/out per PID |
| `kprobe` | `__fd_install` | File descriptor count |

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
| `kprobe` | `sys_execve` | Process spawn: PID, binary path, parent PID, timestamp |
| `tracepoint` | `sock/inet_sock_set_state` | TCP connection established: src/dst IP, port |
| `tracepoint` | `syscalls/sys_enter_connect` | Outbound connect: destination IP + port |

Events written to a **BPF ring buffer** — efficient, ordered, no polling overhead.

### Userspace (`audit/consumer.go`)

- Reads ring buffer in dedicated goroutine
- Emits structured `slog` JSON log entries into existing logging pipeline (same OTLP endpoint)
- Configurable IP allowlist in a BPF hash map — alerts on connections outside the list
- Configurable exec path allowlist — alerts on unexpected binary spawns
- All alerts tagged with `audit.violation=true` for downstream SIEM filtering

**Config vars:**

| Var | Default | Purpose |
|-----|---------|---------|
| `RAVEN_EBPF_AUDIT_IP_ALLOWLIST` | `""` | Comma-separated CIDRs allowed for outbound |
| `RAVEN_EBPF_AUDIT_EXEC_ALLOWLIST` | `""` | Comma-separated binary paths allowed to exec |

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
| `blocked_ips` | Hash | Permanently blocked CIDRs from Valkey blocklist |
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
    ObservabilityEnabled bool
    AuditEnabled         bool
    AuditIPAllowlist     []string
    AuditExecAllowlist   []string
    XDPEnabled           bool
    XDPInterface         string
}
```

| Env var | Default | Feature |
|---------|---------|---------|
| `RAVEN_EBPF_OBSERVABILITY_ENABLED` | `false` | #122 kprobe metrics |
| `RAVEN_EBPF_AUDIT_ENABLED` | `false` | #123 audit trail |
| `RAVEN_EBPF_AUDIT_IP_ALLOWLIST` | `""` | Comma-sep CIDRs |
| `RAVEN_EBPF_AUDIT_EXEC_ALLOWLIST` | `""` | Comma-sep binary paths |
| `RAVEN_EBPF_XDP_ENABLED` | `false` | #120 XDP drop |
| `RAVEN_EBPF_XDP_INTERFACE` | `eth0` | NIC to attach XDP to |

All features default to `false` — opt-in only, safe for existing deployments.

---

## Build Pipeline Changes

### Dockerfile (builder stage)

Add to the builder stage:
```dockerfile
RUN apk add --no-cache clang llvm linux-headers libbpf-dev
RUN go install github.com/cilium/ebpf/cmd/bpf2go@latest
```

Runtime stage unchanged — BPF bytecode is embedded at build time.

### CGO

`CGO_ENABLED` must be `1` for `cilium/ebpf`. Cross-compilation for ARM64 in `Makefile.edge`:
```makefile
CC=aarch64-linux-musl-gcc CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build ...
```

### go generate

Each feature package contains a `//go:generate` directive:
```go
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 Observability ../programs/observability.c
```

Run `make generate` before `make build`.

---

## Docker / Deployment Changes

Both `docker-compose.yml` and `docker-compose.edge.yml` — `go-api` service:

```yaml
cap_add:
  - CAP_BPF
  - CAP_NET_ADMIN      # XDP only
  - CAP_SYS_ADMIN      # fallback for kernels < 5.8
security_opt:
  - no-new-privileges:false
```

**Prerequisite check on edge nodes:**
```bash
# Verify BTF is available (kernel >= 5.2 with CONFIG_DEBUG_INFO_BTF=y)
ls /sys/kernel/btf/vmlinux
```

Raspberry Pi OS 6.x confirmed BTF-compatible per research doc.

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
