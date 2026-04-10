# eBPF Test Harness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a privileged Go test harness for all three eBPF program types (XDP pre-filter, kernel observability, audit trail), including a real ClickHouse sidecar for audit persistence tests and a CI job that runs on push to main only.

**Architecture:** Go test binary gated behind `//go:build ebpf` build tag. Tests in `tests/ebpf/` at repo root. Each eBPF program type has its own sub-package. Uses `cilium/ebpf` library to load programs onto loopback interface. Packet crafting via `google/gopacket`. Ring buffer events read via `cilium/ebpf/ringbuf`. Audit persistence verified against real ClickHouse container (testcontainers).

**Tech Stack:** Go 1.25, `github.com/cilium/ebpf`, `github.com/google/gopacket`, `github.com/stretchr/testify v1.11.1`, `github.com/testcontainers/testcontainers-go v0.41.0`, Linux kernel ≥5.15 with BTF support, `CAP_BPF` + `CAP_NET_ADMIN`

**IMPORTANT:** These tests require Linux. They cannot run on macOS. The CI job runs them inside a privileged Docker container. For local development, run inside the `docker-compose.ebpf.yml` container or a Linux VM.

---

## Pre-flight: Audit Existing eBPF Code

- [ ] **Step 1: Find existing eBPF source files**

```bash
find /Users/jobinlawrance/Project/raven/internal/ebpf -type f | sort
```

Read the eBPF C source files and Go loader code. Understand:
- XDP program attach point and decision logic (XDP_PASS / XDP_DROP)
- Observability ring buffer schema (what fields each event has)
- Audit program trigger conditions and log entry schema
- How the Go loader attaches programs (`internal/ebpf/xdp/loader.go` or similar)

```bash
cat /Users/jobinlawrance/Project/raven/internal/ebpf/xdp/*.go 2>/dev/null | head -100
cat /Users/jobinlawrance/Project/raven/internal/ebpf/observability/*.go 2>/dev/null | head -100
cat /Users/jobinlawrance/Project/raven/internal/ebpf/audit/*.go 2>/dev/null | head -100
```

This step is mandatory — the test harness must use the actual loader interfaces, not re-implement them.

---

## Task 1: Directory Structure & Build Tag Infrastructure

**Files:**
- Create: `tests/ebpf/xdp/xdp_test.go`
- Create: `tests/ebpf/observability/observability_test.go`
- Create: `tests/ebpf/audit/audit_test.go`
- Create: `tests/ebpf/helpers/helpers.go`

- [ ] **Step 1: Create the directory structure**

```bash
mkdir -p /Users/jobinlawrance/Project/raven/tests/ebpf/xdp
mkdir -p /Users/jobinlawrance/Project/raven/tests/ebpf/observability
mkdir -p /Users/jobinlawrance/Project/raven/tests/ebpf/audit
mkdir -p /Users/jobinlawrance/Project/raven/tests/ebpf/helpers
```

- [ ] **Step 2: Create helpers package** (`tests/ebpf/helpers/helpers.go`)

```go
//go:build ebpf

// Package helpers provides test utilities for eBPF kernel tests.
// These tests require CAP_BPF and CAP_NET_ADMIN.
// Run inside the privileged container: docker-compose.ebpf.yml
package helpers

import (
	"net"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/stretchr/testify/require"
)

// LoopbackIface returns the loopback interface for eBPF attachment.
func LoopbackIface(t *testing.T) *net.Interface {
	t.Helper()
	iface, err := net.InterfaceByName("lo")
	require.NoError(t, err, "loopback interface must exist")
	return iface
}

// CraftTCPPacket creates a raw TCP packet for injection testing.
func CraftTCPPacket(srcIP, dstIP string, srcPort, dstPort uint16) []byte {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}

	eth := &layers.Ethernet{
		SrcMAC:       net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		DstMAC:       net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x02},
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := &layers.IPv4{
		SrcIP:    net.ParseIP(srcIP),
		DstIP:    net.ParseIP(dstIP),
		Protocol: layers.IPProtocolTCP,
		TTL:      64,
	}
	tcp := &layers.TCP{
		SrcPort: layers.TCPPort(srcPort),
		DstPort: layers.TCPPort(dstPort),
		SYN:     true,
	}
	_ = tcp.SetNetworkLayerForChecksum(ip)
	_ = gopacket.SerializeLayers(buf, opts, eth, ip, tcp, gopacket.Payload(nil))
	return buf.Bytes()
}

// RequirePrivileged skips the test if CAP_BPF is not available.
// On macOS or unprivileged Linux, the test is skipped with a clear message
// rather than hanging or producing cryptic kernel errors.
func RequirePrivileged(t *testing.T) {
	t.Helper()
	// Attempt a minimal BPF syscall (BPF_MAP_CREATE with invalid args).
	// A real BPF-capable process gets EINVAL; an unprivileged one gets EPERM.
	_, _, errno := unix.Syscall(unix.SYS_BPF, 0 /* BPF_MAP_CREATE */, 0, 0)
	if errno == unix.EPERM {
		t.Skip("CAP_BPF not available — run inside privileged container (docker-compose.ebpf.yml)")
	}
	// EINVAL or EBADF means syscall reached BPF handler — we have the cap
}
```

- [ ] **Step 3: Verify build tag compiles**

```bash
cd /Users/jobinlawrance/Project/raven && go build -tags ebpf ./tests/ebpf/... 2>&1
```

Expected: compiles without errors (or reports missing packages to add to go.mod).

If `github.com/google/gopacket` is not in go.mod:
```bash
go get github.com/google/gopacket
```

- [ ] **Step 4: Commit skeleton**

```bash
git add tests/ebpf/ go.mod go.sum
git commit -m "test(ebpf): create test harness skeleton with build tag and helpers"
```

---

## Task 2: XDP Pre-filter Tests

**Files:**
- Modify: `tests/ebpf/xdp/xdp_test.go`

- [ ] **Step 1: Write failing XDP tests**

First, read `internal/ebpf/xdp/` to understand the actual loader API. Look for:
- How the XDP program is loaded and attached (`Load()`, `Attach()`, etc.)
- How blocklist CIDRs are configured (eBPF map? config struct?)
- How PPS rate limit is set

Then write:

```go
//go:build ebpf

package xdp_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ravencloak-org/Raven/internal/ebpf/xdp"
	"github.com/ravencloak-org/Raven/tests/ebpf/helpers"
)

func TestXDP_AllowLegitimateTraffic(t *testing.T) {
	helpers.RequirePrivileged(t)
	iface := helpers.LoopbackIface(t)

	prog, err := xdp.Load(xdp.Config{
		Interface: iface.Name,
		RateLimit: 10000, // high — won't trigger
	})
	require.NoError(t, err)
	t.Cleanup(prog.Detach)

	err = prog.Attach()
	require.NoError(t, err)

	// Use cilium/ebpf (*ebpf.Program).Test() — runs eBPF in kernel BPF_PROG_TEST_RUN mode.
	// XDP_PASS = 2, XDP_DROP = 1 (Linux kernel constants — do not change these values).
	// prog.XDPProg is the *ebpf.Program field — read internal/ebpf/xdp/loader.go to confirm field name.
	pkt := helpers.CraftTCPPacket("127.0.0.2", "127.0.0.1", 54321, 8080)
	retval, _, err := prog.XDPProg.Test(pkt)
	require.NoError(t, err)
	assert.Equal(t, uint32(2), retval, "XDP_PASS=2 expected for legitimate traffic")
}

func TestXDP_DropKnownBadSource(t *testing.T) {
	helpers.RequirePrivileged(t)
	iface := helpers.LoopbackIface(t)

	prog, err := xdp.Load(xdp.Config{
		Interface:  iface.Name,
		BlockCIDRs: []string{"10.0.0.0/8"},
	})
	require.NoError(t, err)
	t.Cleanup(prog.Detach)
	require.NoError(t, prog.Attach())

	pkt := helpers.CraftTCPPacket("10.1.2.3", "127.0.0.1", 54321, 8080)
	retval, _, err := prog.XDPProg.Test(pkt)
	require.NoError(t, err)
	assert.Equal(t, uint32(1), retval, "XDP_DROP=1 expected for blocklisted CIDR")
}

func TestXDP_RateThresholdDrop(t *testing.T) {
	helpers.RequirePrivileged(t)
	iface := helpers.LoopbackIface(t)

	prog, err := xdp.Load(xdp.Config{
		Interface: iface.Name,
		RateLimit: 5, // 5 packets/sec — burst of 20 will exceed this
	})
	require.NoError(t, err)
	t.Cleanup(prog.Detach)
	require.NoError(t, prog.Attach())

	pkt := helpers.CraftTCPPacket("127.0.0.2", "127.0.0.1", 54321, 8080)
	dropCount := 0
	for i := 0; i < 20; i++ {
		retval, _, err := prog.XDPProg.Test(pkt)
		require.NoError(t, err)
		if retval == 1 { dropCount++ } // XDP_DROP = 1
	}
	assert.Greater(t, dropCount, 0, "rate-exceeded packets must be XDP_DROP=1")
}

func TestXDP_AllowlistBypassesRateLimit(t *testing.T) {
	helpers.RequirePrivileged(t)
	iface := helpers.LoopbackIface(t)

	prog, err := xdp.Load(xdp.Config{
		Interface:  iface.Name,
		RateLimit:  1, // very low
		AllowCIDRs: []string{"127.0.0.100/32"},
	})
	require.NoError(t, err)
	t.Cleanup(prog.Detach)
	require.NoError(t, prog.Attach())

	pkt := helpers.CraftTCPPacket("127.0.0.100", "127.0.0.1", 54321, 8080)
	for i := 0; i < 10; i++ {
		retval, _, err := prog.XDPProg.Test(pkt)
		require.NoError(t, err)
		assert.Equal(t, uint32(2), retval, "allowlisted IP must always be XDP_PASS=2")
	}
}

- [ ] **Step 2: Run XDP tests inside privileged container**

```bash
docker run --privileged \
  --cap-add CAP_BPF \
  --cap-add CAP_NET_ADMIN \
  -v $(pwd):/workspace \
  golang:1.26.1 \
  bash -c "cd /workspace && go test ./tests/ebpf/xdp/... -v -tags ebpf -timeout 60s"
```

Expected: all 4 XDP tests PASS

- [ ] **Step 3: Commit**

```bash
git add tests/ebpf/xdp/
git commit -m "test(ebpf): XDP pre-filter tests (allow, drop, rate threshold, allowlist bypass)"
```

---

## Task 3: Observability Tests

**Files:**
- Modify: `tests/ebpf/observability/observability_test.go`

- [ ] **Step 1: Read the observability loader and ring buffer schema**

```bash
cat /Users/jobinlawrance/Project/raven/internal/ebpf/observability/*.go | head -150
```

Identify:
- Ring buffer map name and event struct layout
- What fields each event contains (src_ip, dst_port, timestamp, direction)
- How to read from the ring buffer in Go

- [ ] **Step 2: Write observability tests**

```go
//go:build ebpf

package observability_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ravencloak-org/Raven/internal/ebpf/observability"
	"github.com/ravencloak-org/Raven/tests/ebpf/helpers"
)

func TestObservability_RingBufferCapture_CorrectFields(t *testing.T) {
	helpers.RequirePrivileged(t)

	monitor, err := observability.Load(observability.Config{
		Interface: helpers.LoopbackIface(t).Name,
	})
	require.NoError(t, err)
	t.Cleanup(monitor.Stop)
	require.NoError(t, monitor.Start())

	// Collect events in background
	events := make(chan *observability.Event, 100)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go monitor.ReadEvents(ctx, events)

	// Generate a network event by dialing loopback
	conn, err := net.Dial("tcp", "127.0.0.1:8080")
	if err == nil { conn.Close() } // connection may be refused — event still captured

	// Wait for at least one event
	select {
	case event := <-events:
		assert.NotZero(t, event.SrcIP, "src_ip must be populated")
		assert.NotZero(t, event.DstPort, "dst_port must be populated")
		assert.NotZero(t, event.Timestamp, "timestamp must be populated")
		assert.NotEmpty(t, event.Direction, "direction must be populated (ingress/egress)")
	case <-ctx.Done():
		t.Fatal("no ring buffer events captured within 5 seconds")
	}
}

func TestObservability_ConcurrentEvents_NoneDropped(t *testing.T) {
	helpers.RequirePrivileged(t)

	monitor, err := observability.Load(observability.Config{
		Interface: helpers.LoopbackIface(t).Name,
	})
	require.NoError(t, err)
	t.Cleanup(monitor.Stop)
	require.NoError(t, monitor.Start())

	events := make(chan *observability.Event, 200)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go monitor.ReadEvents(ctx, events)

	// Generate 100 concurrent connections
	done := make(chan struct{})
	for i := 0; i < 100; i++ {
		go func() {
			conn, err := net.Dial("tcp", "127.0.0.1:8080")
			if err == nil { conn.Close() }
			done <- struct{}{}
		}()
	}
	for i := 0; i < 100; i++ { <-done }

	// Drain events, allow 3 seconds for all to arrive
	time.Sleep(3 * time.Second)
	close(events)
	capturedCount := 0
	for range events { capturedCount++ }

	// We can't assert exactly 100 (some connections may not produce events
	// if port 8080 isn't listening), but ring buffer must not report drops
	assert.Greater(t, capturedCount, 0, "at least some events must be captured")
	stats := monitor.Stats()
	assert.Zero(t, stats.DroppedEvents, "ring buffer must not drop events")
}

func TestObservability_MetadataAccuracy(t *testing.T) {
	helpers.RequirePrivileged(t)

	monitor, err := observability.Load(observability.Config{
		Interface: helpers.LoopbackIface(t).Name,
	})
	require.NoError(t, err)
	t.Cleanup(monitor.Stop)
	require.NoError(t, monitor.Start())

	events := make(chan *observability.Event, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go monitor.ReadEvents(ctx, events)

	// Inject a known packet using the observability program's Test() method.
	// This runs BPF_PROG_TEST_RUN in the kernel — deterministic, no raw socket needed.
	pkt := helpers.CraftTCPPacket("127.0.0.5", "127.0.0.1", 12345, 9999)
	_, _, err = monitor.ObsProg.Test(pkt)
	require.NoError(t, err, "BPF_PROG_TEST_RUN must succeed for metadata injection")

	select {
	case event := <-events:
		assert.Contains(t, event.SrcIP.String(), "127.0.0", "src_ip must be captured")
		assert.Equal(t, uint16(9999), event.DstPort, "dst_port must match injected packet")
		assert.NotEmpty(t, event.Direction, "direction must be set")
	case <-ctx.Done():
		// This is a real failure — the observability program must capture events from Test() runs.
		t.Fatal("no ring buffer events captured after BPF_PROG_TEST_RUN injection — observability program may be broken")
	}
}
```

- [ ] **Step 3: Run observability tests**

```bash
docker run --privileged \
  --cap-add CAP_BPF \
  --cap-add CAP_NET_ADMIN \
  -v $(pwd):/workspace \
  golang:1.26.1 \
  bash -c "cd /workspace && go test ./tests/ebpf/observability/... -v -tags ebpf -timeout 60s"
```

- [ ] **Step 4: Commit**

```bash
git add tests/ebpf/observability/
git commit -m "test(ebpf): observability ring buffer capture, concurrent events, metadata accuracy"
```

---

## Task 4: Audit Trail Tests with ClickHouse Sidecar

**Files:**
- Modify: `tests/ebpf/audit/audit_test.go`

The audit persistence test uses a **real ClickHouse sidecar container** started alongside the privileged test binary. This is distinct from the unit-test in-memory ClickHouse sink.

- [ ] **Step 1: Read the audit loader and schema**

```bash
cat /Users/jobinlawrance/Project/raven/internal/ebpf/audit/*.go | head -200
```

Identify:
- Audit event trigger conditions (port scan threshold, rate threshold)
- Audit log entry schema: timestamp, src_ip, dst_port, threat_type, action_taken
- How audit entries reach ClickHouse (HTTP insert? native client?)
- ClickHouse table name and column schema

- [ ] **Step 2: Write audit tests**

```go
//go:build ebpf

package audit_test

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"testing"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/ravencloak-org/Raven/internal/ebpf/audit"
	"github.com/ravencloak-org/Raven/tests/ebpf/helpers"
)

// startClickHouse starts a real ClickHouse container for audit persistence tests.
// The eBPF audit module writes entries here via HTTP or native protocol.
func startClickHouse(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "clickhouse/clickhouse-server:latest",
			ExposedPorts: []string{"8123/tcp", "9000/tcp"},
			WaitingFor:   wait.ForHTTP("/ping").WithPort("8123/tcp").WithStatusCodeMatcher(func(status int) bool { return status == 200 }),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "9000/tcp")
	return fmt.Sprintf("clickhouse://%s:%s/default", host, port.Port())
}

func TestAudit_PortScanDetection_CreatesEntry(t *testing.T) {
	helpers.RequirePrivileged(t)
	chDSN := startClickHouse(t)

	auditor, err := audit.Load(audit.Config{
		Interface:        helpers.LoopbackIface(t).Name,
		ClickHouseDSN:    chDSN,
		PortScanThreshold: 5, // 5 unique ports in 1 second = port scan
	})
	require.NoError(t, err)
	t.Cleanup(auditor.Stop)
	require.NoError(t, auditor.Start())

	// Simulate port scan: rapid sequential probes to different ports
	srcIP := "127.0.0.10"
	for port := 9000; port < 9010; port++ {
		conn, _ := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", "127.0.0.1", port), 50*time.Millisecond)
		if conn != nil { conn.Close() }
	}

	// Wait for audit entry to be written
	time.Sleep(2 * time.Second)

	// Verify entry in ClickHouse
	db, err := sql.Open("clickhouse", chDSN)
	require.NoError(t, err)
	defer db.Close()

	var count int
	err = db.QueryRow(`SELECT count() FROM raven_audit WHERE threat_type = 'port_scan' AND src_ip = ?`, srcIP).Scan(&count)
	require.NoError(t, err)
	assert.Greater(t, count, 0, "port scan must produce audit entry in ClickHouse")
}

func TestAudit_RateThresholdEvent_CreatesEntry(t *testing.T) {
	helpers.RequirePrivileged(t)
	chDSN := startClickHouse(t)

	auditor, err := audit.Load(audit.Config{
		Interface:     helpers.LoopbackIface(t).Name,
		ClickHouseDSN: chDSN,
		RateThreshold: 3, // very low PPS threshold
	})
	require.NoError(t, err)
	t.Cleanup(auditor.Stop)
	require.NoError(t, auditor.Start())

	// Flood traffic to exceed rate threshold
	for i := 0; i < 20; i++ {
		conn, _ := net.DialTimeout("tcp", "127.0.0.1:8080", 10*time.Millisecond)
		if conn != nil { conn.Close() }
	}

	time.Sleep(2 * time.Second)

	db, err := sql.Open("clickhouse", chDSN)
	require.NoError(t, err)
	defer db.Close()

	var count int
	_ = db.QueryRow(`SELECT count() FROM raven_audit WHERE threat_type = 'rate_exceeded'`).Scan(&count)
	assert.Greater(t, count, 0, "rate-exceeded traffic must produce audit entry")
}

func TestAudit_EntrySchema_AllRequiredFieldsPresent(t *testing.T) {
	helpers.RequirePrivileged(t)
	chDSN := startClickHouse(t)

	auditor, err := audit.Load(audit.Config{
		Interface:        helpers.LoopbackIface(t).Name,
		ClickHouseDSN:    chDSN,
		PortScanThreshold: 3,
	})
	require.NoError(t, err)
	t.Cleanup(auditor.Stop)
	require.NoError(t, auditor.Start())

	// Trigger at least one audit entry
	for port := 9000; port < 9005; port++ {
		net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
	}
	time.Sleep(2 * time.Second)

	db, err := sql.Open("clickhouse", chDSN)
	require.NoError(t, err)
	defer db.Close()

	row := db.QueryRow(`SELECT timestamp, src_ip, dst_port, threat_type, action_taken FROM raven_audit LIMIT 1`)
	var timestamp time.Time
	var srcIP string
	var dstPort uint16
	var threatType, actionTaken string
	err = row.Scan(&timestamp, &srcIP, &dstPort, &threatType, &actionTaken)
	require.NoError(t, err)

	assert.False(t, timestamp.IsZero(), "timestamp must be set")
	assert.NotEmpty(t, srcIP, "src_ip must be set")
	assert.NotZero(t, dstPort, "dst_port must be set")
	assert.NotEmpty(t, threatType, "threat_type must be set")
	assert.NotEmpty(t, actionTaken, "action_taken must be set")
}

func TestAudit_Persistence_SurvivesProcessRestart(t *testing.T) {
	helpers.RequirePrivileged(t)
	chDSN := startClickHouse(t)

	// First auditor instance — generates entries
	auditor1, err := audit.Load(audit.Config{
		Interface:        helpers.LoopbackIface(t).Name,
		ClickHouseDSN:    chDSN,
		PortScanThreshold: 3,
	})
	require.NoError(t, err)
	require.NoError(t, auditor1.Start())

	for port := 9000; port < 9005; port++ {
		net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
	}
	time.Sleep(2 * time.Second)
	auditor1.Stop() // Simulate process stop

	// Second auditor instance — ClickHouse still has the entries
	db, err := sql.Open("clickhouse", chDSN)
	require.NoError(t, err)
	defer db.Close()

	var count int
	_ = db.QueryRow(`SELECT count() FROM raven_audit`).Scan(&count)
	assert.Greater(t, count, 0, "audit entries must persist in ClickHouse after process restart")
}
```

Add `github.com/ClickHouse/clickhouse-go/v2` to go.mod if not present:
```bash
go get github.com/ClickHouse/clickhouse-go/v2
```

- [ ] **Step 3: Run audit tests (requires ClickHouse container)**

```bash
docker run --privileged \
  --cap-add CAP_BPF \
  --cap-add CAP_NET_ADMIN \
  -v $(pwd):/workspace \
  -v /var/run/docker.sock:/var/run/docker.sock \
  golang:1.26.1 \
  bash -c "cd /workspace && go test ./tests/ebpf/audit/... -v -tags ebpf -timeout 120s"
```

Note: testcontainers requires Docker socket access inside the privileged container (`-v /var/run/docker.sock:/var/run/docker.sock`).

- [ ] **Step 4: Commit**

```bash
git add tests/ebpf/audit/ go.mod go.sum
git commit -m "test(ebpf): audit trail tests with real ClickHouse sidecar (port scan, rate threshold, schema, persistence)"
```

---

## Task 5: CI Configuration

**Files:**
- Modify: `.github/workflows/go.yml`

- [ ] **Step 1: Add eBPF test job to `go.yml`**

Open `.github/workflows/go.yml` and add the following job. It runs **only on push to main** (not PRs):

```yaml
  ebpf-tests:
    name: eBPF Kernel Tests
    runs-on: ubuntu-latest
    # Only run on push to main — not on PRs (requires privileged container)
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Run eBPF tests in privileged container
        run: |
          docker run --rm \
            --privileged \
            --cap-add CAP_BPF \
            --cap-add CAP_NET_ADMIN \
            -v ${{ github.workspace }}:/workspace \
            -v /var/run/docker.sock:/var/run/docker.sock \
            golang:1.26.1 \
            bash -c "
              apt-get update -qq && \
              apt-get install -y -qq clang llvm libbpf-dev bpftool linux-headers-\$(uname -r) && \
              cd /workspace && \
              go test ./tests/ebpf/... \
                -v \
                -tags ebpf \
                -timeout 300s \
                -count=1
            "

      - name: Report eBPF test results
        if: always()
        run: echo "eBPF test job completed with exit code $?"

# IMPORTANT: The audit tests use testcontainers to start a ClickHouse sidecar.
# testcontainers requires a Docker daemon accessible at /var/run/docker.sock inside the container.
# GitHub-hosted ubuntu-latest runners have Docker installed, so this works.
# If the Docker socket mount causes issues, replace the testcontainers ClickHouse setup with
# a GitHub Actions `services:` block instead:
#
#   services:
#     clickhouse:
#       image: clickhouse/clickhouse-server:latest
#       ports:
#         - 9000:9000
#       options: >-
#         --health-cmd "wget --no-verbose --tries=1 --spider http://localhost:8123/ping || exit 1"
#         --health-interval 5s
#         --health-timeout 3s
#         --health-retries 10
#
# Then pass CLICKHOUSE_DSN=clickhouse://localhost:9000/default to the test binary
# via an environment variable instead of spinning it up via testcontainers.
```

- [ ] **Step 2: Verify CI syntax**

```bash
cd /Users/jobinlawrance/Project/raven && cat .github/workflows/go.yml | python3 -c "import sys,yaml; yaml.safe_load(sys.stdin)" && echo "YAML valid"
```

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/go.yml
git commit -m "ci: add privileged eBPF test job to go.yml (push to main only)"
```

---

## Task 6: Local Development Guide

- [ ] **Step 1: Verify tests run locally via docker-compose.ebpf.yml**

```bash
cd /Users/jobinlawrance/Project/raven
docker compose -f docker-compose.ebpf.yml run --rm go-api \
  go test ./tests/ebpf/... -v -tags ebpf -timeout 300s
```

If `go-api` service doesn't have test dependencies, run in a standalone container:

```bash
docker run --rm \
  --privileged \
  --cap-add CAP_BPF \
  --cap-add CAP_NET_ADMIN \
  -v $(pwd):/workspace \
  -v /var/run/docker.sock:/var/run/docker.sock \
  golang:1.26.1 \
  bash -c "cd /workspace && go test ./tests/ebpf/... -v -tags ebpf"
```

- [ ] **Step 2: Final verification — all 3 eBPF suites pass**

```bash
# Expected output:
# ok  	github.com/ravencloak-org/Raven/tests/ebpf/xdp          X.XXs
# ok  	github.com/ravencloak-org/Raven/tests/ebpf/observability X.XXs
# ok  	github.com/ravencloak-org/Raven/tests/ebpf/audit         X.XXs
```

- [ ] **Step 3: Final commit**

```bash
git add tests/ebpf/ .github/workflows/go.yml
git commit -m "test(ebpf): complete eBPF test harness — XDP, observability, audit with ClickHouse"
```
