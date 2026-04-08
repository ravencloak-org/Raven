// SPDX-License-Identifier: GPL-2.0
// BPF programs for Feature #122: Kernel Observability.
// Compiled by bpf2go; do not build directly on macOS.

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

char LICENSE[] SEC("license") = "GPL";

// Per-PID CPU time accumulator (nanoseconds)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 4096);
    __type(key, u32);
    __type(value, u64);
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

// Syscall error counts by syscall number
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 512);
    __type(key, u32);
    __type(value, u64);
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

// net/netif_receive_skb: bytes in per PID
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
