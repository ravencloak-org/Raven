# eBPF Edge Optimization — Research Notes

> Status: **Parked / Future Milestone**
> Assessed: 2026-03-28
> Reference: https://sazak.io/articles/an-applied-introduction-to-ebpf-with-go-2024-06-06

## What Is eBPF

eBPF (Extended Berkeley Packet Filter) lets you inject sandboxed programs into the Linux kernel at runtime — no kernel recompile, no reboot. Programs are event-driven: they attach to **hooks** in the kernel and fire on matching events. Results surface to user space via **BPF maps** (shared kernel↔userspace data structures).

Go integration: `github.com/cilium/ebpf` (Cilium's `ebpf-go`). eBPF kernel-side programs are written in restricted C, compiled via `bpf2go`, then loaded and managed from Go.

Key hook types:
| Hook | When it fires |
|------|---------------|
| `xdp` | Right after NIC receives a packet — before the kernel networking stack |
| `kprobe` / `kretprobe` | Before/after any kernel function call |
| `tracepoint` | Predefined kernel trace points (stable API) |
| `uprobe` / `uretprobe` | Before/after any user-space function (Go binary, Python AI worker) |

---

## Use Case 1 — XDP Pre-filtering (Rate Limit Offload)

**Problem:** Current rate limiting lives in Valkey at the application layer. Every packet — including junk — consumes CPU on the Pi before the Go server even sees it.

**eBPF solution:** Attach an XDP program to the network interface. It runs before the TCP/IP stack. Malicious or excess-rate IPs get `XDP_DROP`'d at the NIC — zero kernel networking overhead, zero Go overhead, zero Valkey hit.

**Raven fit:**
- Complements (not replaces) the Valkey sliding-window rate limiter — XDP handles burst/DDoS, Valkey handles per-user quotas
- Most impactful on Raspberry Pi under load or adversarial traffic
- Aligns with the "minimal footprint, native speed" edge constraint

**Implementation sketch:**
1. XDP C program: parse IP src, lookup BPF hash map of blocked/throttled IPs, return `XDP_DROP` or `XDP_PASS`
2. Go control plane: populate the BPF map from Valkey state or a blocklist feed
3. Attach to the Pi's primary interface (e.g., `eth0`) on startup

---

## Use Case 2 — Kernel-level Observability (Zero-agent Metrics)

**Problem:** Running a Prometheus node exporter or any monitoring sidecar on the Pi wastes RAM and CPU. OTel middleware covers application-layer traces but not kernel/system metrics.

**eBPF solution:** Use kprobes and tracepoints to collect CPU scheduling events, syscall latencies, network socket stats, and memory pressure — all from within the Go binary via BPF maps. No separate process needed.

**Raven fit:**
- Directly eliminates a monitoring agent from the edge node
- Data can be exported via the existing OTel pipeline (custom metric source)
- Pairs well with the Phase 2 analytics/observability work

**What you get without an agent:**
- Per-process CPU time (kprobe on scheduler)
- Network bytes in/out per connection (socket tracepoints)
- Syscall error rates (tracepoint on syscall exit)
- File descriptor exhaustion warnings

---

## Use Case 3 — Security Audit Trail (Process + Syscall Monitoring)

**Problem:** GDPR/SOC2 requires audit logs of significant system events. Currently there's no low-level audit trail for what runs on the Pi.

**eBPF solution:** Attach a kprobe to `sys_execve` to log every process spawn with timestamp, PID, binary path, and parent PID. Attach socket tracepoints to log outbound connections.

**Raven fit:**
- Detects anomalous behavior: AI worker (Python) spawning unexpected subprocesses, unexpected outbound calls
- Feeds into compliance audit log without a separate auditd daemon
- Lightweight — only fires on exec and connect events, not on every syscall

**Implementation sketch:**
1. C program on `sys_execve` kprobe → emit event to ring buffer BPF map
2. Go consumer reads events, writes to structured log (JSON) → existing logging pipeline
3. Alert on: exec from unexpected paths, connections to non-allowlisted IPs

---

## Prerequisites & Caveats

| Item | Detail |
|------|--------|
| **Kernel version** | Requires Linux ≥ 4.18 for eBPF; CO-RE (BTF) needs ≥ 5.2. Raspberry Pi OS on Pi 4/5 ships kernel 6.x — confirmed compatible. Pi 3 or custom kernels need verification. |
| **ARM64 support** | `ebpf-go` + CO-RE works on ARM64. Confirmed by Cilium. |
| **Build pipeline** | `bpf2go` needs `clang` + `linux-headers` at build time. Docker build image needs updating. Cross-compilation (x86 → ARM64) is supported but adds CI complexity. |
| **BTF requirement** | Kernel must be compiled with `CONFIG_DEBUG_INFO_BTF=y`. Check: `ls /sys/kernel/btf/vmlinux`. Raspberry Pi OS kernels enable this by default from 6.1+. |
| **Privileges** | eBPF programs require `CAP_BPF` (or `CAP_SYS_ADMIN` on older kernels). The Go service needs this capability in its Docker security context. |
| **Verifier constraints** | eBPF C programs have no unbounded loops, limited stack size (512 bytes), restricted pointer arithmetic. Keeps programs safe but limits complexity. |

---

## Recommended Implementation Order (When Ready)

1. **Use Case 2 first** — observability is additive, lowest risk, easy to validate
2. **Use Case 3 second** — kprobe on execve is simple and high compliance value
3. **Use Case 1 last** — XDP modifies the network path; test thoroughly on the Pi before enabling in production

---

## Libraries & References

- `github.com/cilium/ebpf` — Go eBPF library (load, attach, read maps)
- `github.com/cilium/ebpf/cmd/bpf2go` — compile C eBPF → Go embed
- `github.com/aquasecurity/libbpfgo` — alternative Go wrapper around libbpf
- Cilium docs: https://ebpf-go.dev/
- Linux kernel BTF docs: https://www.kernel.org/doc/html/latest/bpf/btf.html
- eBPF verifier: https://www.kernel.org/doc/html/latest/bpf/verifier.html
