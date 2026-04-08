// SPDX-License-Identifier: GPL-2.0
// BPF programs for Feature #123: Security Audit Trail.
// Compiled by bpf2go; do not build directly on macOS.

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
    __u8  type;
    __u32 pid;
    __u32 ppid;
    __u64 timestamp_ns;
    char  comm[TASK_COMM_LEN];
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

// Ring buffer for audit events
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 20);
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

    const char *filename = (const char *)ctx->args[0];
    bpf_probe_read_user_str(e->exec.path, sizeof(e->exec.path), filename);

    bpf_ringbuf_submit(e, 0);
    return 0;
}

SEC("tracepoint/sock/inet_sock_set_state")
int handle_tcp_state(struct trace_event_raw_inet_sock_set_state *ctx)
{
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
