# PulseMesh eBPF / XDP Kernel Timestamping Extension

This document outlines the architecture and implementation design for the **PulseMesh eBPF/XDP Kernel Timestamping Module**, enabling sub-microsecond latency measurement by recording ingress and egress packet timestamps directly at the network driver layer before kernel socket queueing or userspace context switching.

---

## 1. Motivation & Problem Statement

Standard userspace UDP probing via `time.Now()` is subject to:
1. **Linux CFS Scheduler Jitter**: Thread wakeup delays (typically 10–100 µs under load).
2. **Socket Buffer Queuing**: Time spent waiting in socket receive/send buffers (`SO_RCVBUF` / `SO_SNDBUF`).
3. **Interrupt Handling & SoftIRQ Latency**: Network stack processing between NIC DMA and userspace `recvfrom()`.

By attaching an **eBPF XDP (eXpress Data Path)** program to the network device:
- **Ingress timestamps** are captured immediately when the NIC ring buffer is serviced by the driver, with zero userspace traversal.
- **Echo packets** can optionally be reflected immediately in-kernel via `XDP_TX`, achieving sub-microsecond round-trip latency precision.

---

## 2. Architecture Overview

```
                      +---------------------------------------+
                      |           PulseMesh Daemon            |
                      |    (cilium/ebpf Go userspace)         |
                      +-------------------+-------------------+
                                          |
                      +-------------------+-------------------+
                      |         BPF Perf Event Ring Buffer    |
                      +-------------------+-------------------+
                                          |
                      +-------------------+-------------------+
                      |      Linux Kernel (eBPF / XDP)        |
                      |  - Match UDP Port 9000 & Magic 0x504D |
                      |  - bpf_ktime_get_ns()                 |
                      |  - BPF Map: seq_id -> ingress_ts      |
                      +-------------------+-------------------+
                                          |
                                    [NIC Driver]
                                          |
                                      [Physical]
```

---

## 3. eBPF C Program (`bpf/xdp_pulsemesh.c`)

```c
//go:build ignore
#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/udp.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#define PULSEMESH_MAGIC 0x504D
#define PROBE_PORT 9000

struct probe_event {
    __u64 seq_id;
    __u64 ktime_ns;
    __u32 src_ip;
};

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24); // 16MB ringbuffer
} events SEC(".maps");

SEC("xdp")
int xdp_pulsemesh_ingress(struct xdp_md *ctx) {
    void *data_end = (void *)(long)ctx->data_end;
    void *data = (void *)(long)ctx->data;

    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return XDP_PASS;

    if (eth->h_proto != bpf_htons(ETH_P_IP))
        return XDP_PASS;

    struct iphdr *ip = (void *)(eth + 1);
    if ((void *)(ip + 1) > data_end)
        return XDP_PASS;

    if (ip->protocol != IPPROTO_UDP)
        return XDP_PASS;

    struct udphdr *udp = (void *)(ip + 1);
    if ((void *)(udp + 1) > data_end)
        return XDP_PASS;

    if (udp->dest != bpf_htons(PROBE_PORT))
        return XDP_PASS;

    // PulseMesh wire packet header check
    __u16 *magic = (void *)(udp + 1);
    if ((void *)(magic + 1) > data_end)
        return XDP_PASS;

    if (bpf_ntohs(*magic) != PULSEMESH_MAGIC)
        return XDP_PASS;

    // Record nanosecond kernel timestamp
    __u64 now = bpf_ktime_get_ns();

    struct probe_event *ev = bpf_ringbuf_reserve(&events, sizeof(*ev), 0);
    if (!ev)
        return XDP_PASS;

    __u64 *seq = (void *)((char *)magic + 4);
    if ((void *)(seq + 1) <= data_end) {
        ev->seq_id = bpf_ntohll(*seq);
    }
    ev->ktime_ns = now;
    ev->src_ip = ip->saddr;

    bpf_ringbuf_submit(ev, 0);

    return XDP_PASS;
}

char _license[] SEC("license") = "GPL";
```

---

## 4. Go Integration via `cilium/ebpf`

To attach the XDP program to an interface:

```go
package ebpfprober

import (
    "github.com/cilium/ebpf/link"
    "github.com/cilium/ebpf/ringbuf"
    "net"
)

type XDPProber struct {
    link link.Link
    rd   *ringbuf.Reader
}

func Attach(ifaceName string) (*XDPProber, error) {
    iface, err := net.InterfaceByName(ifaceName)
    if err != nil {
        return nil, err
    }

    // Load compiled BPF spec
    objs := bpfObjects{}
    if err := loadBpfObjects(&objs, nil); err != nil {
        return nil, err
    }

    // Attach to network device
    l, err := link.AttachXDP(link.XDPOptions{
        Program:   objs.XdpPulsemeshIngress,
        Interface: iface.Index,
    })
    if err != nil {
        return nil, err
    }

    rd, err := ringbuf.NewReader(objs.Events)
    if err != nil {
        l.Close()
        return nil, err
    }

    return &XDPProber{link: l, rd: rd}, nil
}
```

---

## 5. Performance Comparison

| Layer | Jitter Accuracy | Resolution | CPU Overhead | Privileges Required |
|:---|:---|:---|:---|:---|
| **Standard UDP Socket** | ±15–80 µs | Microsecond | Low | None (unprivileged) |
| **SO_TIMESTAMPING** | ±2–5 µs | Nanosecond | Low | `CAP_NET_RAW` / `CAP_NET_ADMIN` |
| **eBPF XDP Ingress** | **±0.1–0.3 µs** | **Nanosecond** | **Zero-copy** | `CAP_BPF` / `CAP_NET_ADMIN` |
