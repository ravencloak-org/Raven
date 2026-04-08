// SPDX-License-Identifier: GPL-2.0
// BPF program for Feature #120: XDP Pre-filtering.
// Compiled by bpf2go; do not build directly on macOS.

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
    __type(value, u32);
} blocked_ips SEC(".maps");

// Throttle state — per-IP packet counter + timestamp
struct throttle_val {
    __u64 count;
    __u64 window_start_ns;
};

struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, 65536);
    __type(key, u32);
    __type(value, struct throttle_val);
} throttle_state SEC(".maps");

// XDP drop counter
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, u32);
    __type(value, u64);
} drop_count SEC(".maps");

#define THROTTLE_LIMIT  1000
#define THROTTLE_WINDOW 1000000000ULL

SEC("xdp")
int xdp_filter(struct xdp_md *ctx)
{
    void *data_end = (void *)(long)ctx->data_end;
    void *data     = (void *)(long)ctx->data;

    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end) return XDP_PASS;
    if (eth->h_proto != __constant_htons(ETH_P_IP)) return XDP_PASS;

    struct iphdr *ip = (void *)(eth + 1);
    if ((void *)(ip + 1) > data_end) return XDP_PASS;

    u32 src = ip->saddr;

    struct lpm_key key = { .prefixlen = 32, .addr = src };
    if (bpf_map_lookup_elem(&blocked_ips, &key)) {
        u32 idx = 0;
        u64 *cnt = bpf_map_lookup_elem(&drop_count, &idx);
        if (cnt) __sync_fetch_and_add(cnt, 1);
        return XDP_DROP;
    }

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
