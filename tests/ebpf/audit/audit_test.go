//go:build ebpf

// Package audit_test contains privileged eBPF integration tests for the
// security audit trail consumer. Tests require CAP_BPF/CAP_SYS_ADMIN and a
// Linux kernel >= 5.8 with BTF enabled. ClickHouse persistence tests use a
// testcontainers-go sidecar.
package audit_test

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cilium/ebpf/ringbuf"
	dockercontainer "github.com/moby/moby/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/otel/metric/noop"

	_ "github.com/ClickHouse/clickhouse-go/v2"

	"github.com/ravencloak-org/Raven/internal/ebpf/audit"
	"github.com/ravencloak-org/Raven/tests/ebpf/helpers"
)

// --- Mock ring buffer reader ---

// mockRingBufReader implements audit.RingBufReader for unit testing the consumer.
// `closed` is atomic because the consumer goroutine calls Read() while the
// test goroutine can concurrently call Close() — `go test -race` correctly
// flags plain-bool access from the two goroutines as a data race.
type mockRingBufReader struct {
	records []ringbuf.Record
	index   int
	closed  atomic.Bool
}

func newMockReader(records []ringbuf.Record) *mockRingBufReader {
	return &mockRingBufReader{records: records}
}

func (m *mockRingBufReader) Read() (ringbuf.Record, error) {
	if m.closed.Load() {
		return ringbuf.Record{}, ringbuf.ErrClosed
	}
	if m.index >= len(m.records) {
		// No more records — simulate the reader draining by closing and
		// returning ErrClosed so the consumer's event loop exits cleanly.
		m.closed.Store(true)
		return ringbuf.Record{}, ringbuf.ErrClosed
	}
	rec := m.records[m.index]
	m.index++
	return rec, nil
}

func (m *mockRingBufReader) Close() error {
	m.closed.Store(true)
	return nil
}

// --- Audit event binary builders ---

// Byte layout constants (matching consumer.go offsets).
const (
	offType = 0
	offPID  = 4
	offTS   = 16
	offComm = 24
	endComm = 40
	offExec = 40
	endExec = 168
	offNet  = 40

	auditExec    = 1
	auditTCP     = 2
	auditConnect = 3
)

func buildExecEvent(pid uint32, comm, path string) ringbuf.Record {
	buf := make([]byte, endExec)
	buf[offType] = auditExec
	binary.LittleEndian.PutUint32(buf[offPID:], pid)
	binary.LittleEndian.PutUint64(buf[offTS:], uint64(time.Now().UnixNano()))
	copy(buf[offComm:endComm], comm)
	copy(buf[offExec:endExec], path)
	return ringbuf.Record{RawSample: buf}
}

func buildTCPEvent(pid uint32, comm string, srcIP, dstIP [4]byte) ringbuf.Record {
	buf := make([]byte, offNet+8)
	buf[offType] = auditTCP
	binary.LittleEndian.PutUint32(buf[offPID:], pid)
	binary.LittleEndian.PutUint64(buf[offTS:], uint64(time.Now().UnixNano()))
	copy(buf[offComm:endComm], comm)
	copy(buf[offNet:offNet+4], srcIP[:])
	copy(buf[offNet+4:offNet+8], dstIP[:])
	return ringbuf.Record{RawSample: buf}
}

func buildConnectEvent(pid uint32, comm string) ringbuf.Record {
	buf := make([]byte, endComm)
	buf[offType] = auditConnect
	binary.LittleEndian.PutUint32(buf[offPID:], pid)
	binary.LittleEndian.PutUint64(buf[offTS:], uint64(time.Now().UnixNano()))
	copy(buf[offComm:endComm], comm)
	return ringbuf.Record{RawSample: buf}
}

// --- Consumer unit tests ---

// TestNewConsumer_NilReader verifies the consumer degrades gracefully.
func TestNewConsumer_NilReader(t *testing.T) {
	helpers.RequirePrivileged(t)

	mp := noop.NewMeterProvider()
	c, err := audit.NewConsumer(nil, mp.Meter("ebpf-test"), audit.Config{})
	require.NoError(t, err)
	require.NotNil(t, c)

	assert.NoError(t, c.Close())
}

// TestNewConsumer_InvalidCIDR verifies that an invalid CIDR in the allowlist
// is rejected at construction time.
func TestNewConsumer_InvalidCIDR(t *testing.T) {
	helpers.RequirePrivileged(t)

	mp := noop.NewMeterProvider()
	_, err := audit.NewConsumer(nil, mp.Meter("ebpf-test"), audit.Config{
		IPAllowlist: []string{"not-a-cidr"},
	})
	assert.Error(t, err)
}

// TestNewConsumer_ValidCIDRAllowlist verifies valid CIDRs are accepted.
func TestNewConsumer_ValidCIDRAllowlist(t *testing.T) {
	helpers.RequirePrivileged(t)

	mp := noop.NewMeterProvider()
	c, err := audit.NewConsumer(nil, mp.Meter("ebpf-test"), audit.Config{
		IPAllowlist: []string{"10.0.0.0/8", "192.168.0.0/16"},
	})
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.NoError(t, c.Close())
}

// TestConsumer_CloseIdempotent verifies Close() can be called multiple times.
func TestConsumer_CloseIdempotent(t *testing.T) {
	helpers.RequirePrivileged(t)

	mp := noop.NewMeterProvider()
	c, err := audit.NewConsumer(nil, mp.Meter("ebpf-test"), audit.Config{})
	require.NoError(t, err)

	assert.NoError(t, c.Close())
	assert.NoError(t, c.Close())
}

// TestConsumer_ProcessesExecEvent verifies the consumer can read and process
// an exec audit event from a mock ring buffer.
func TestConsumer_ProcessesExecEvent(t *testing.T) {
	helpers.RequirePrivileged(t)

	records := []ringbuf.Record{
		buildExecEvent(1234, "bash", "/usr/bin/curl"),
	}
	reader := newMockReader(records)

	mp := noop.NewMeterProvider()
	c, err := audit.NewConsumer(reader, mp.Meter("ebpf-test"), audit.Config{
		ExecAllowlist: []string{"/usr/bin/bash", "/usr/bin/ls"},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	c.Start(ctx)

	// Wait for the consumer to process and exit.
	<-ctx.Done()
	time.Sleep(50 * time.Millisecond)

	assert.NoError(t, c.Close())
}

// TestConsumer_ProcessesTCPEvent verifies TCP-established event handling.
func TestConsumer_ProcessesTCPEvent(t *testing.T) {
	helpers.RequirePrivileged(t)

	srcIP := [4]byte{192, 168, 1, 10}
	dstIP := [4]byte{10, 0, 0, 1}
	records := []ringbuf.Record{
		buildTCPEvent(5678, "nginx", srcIP, dstIP),
	}
	reader := newMockReader(records)

	mp := noop.NewMeterProvider()
	c, err := audit.NewConsumer(reader, mp.Meter("ebpf-test"), audit.Config{
		IPAllowlist: []string{"10.0.0.0/8"},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	c.Start(ctx)

	<-ctx.Done()
	time.Sleep(50 * time.Millisecond)
	assert.NoError(t, c.Close())
}

// TestConsumer_ProcessesConnectEvent verifies connect event handling.
func TestConsumer_ProcessesConnectEvent(t *testing.T) {
	helpers.RequirePrivileged(t)

	records := []ringbuf.Record{
		buildConnectEvent(9999, "curl"),
	}
	reader := newMockReader(records)

	mp := noop.NewMeterProvider()
	c, err := audit.NewConsumer(reader, mp.Meter("ebpf-test"), audit.Config{})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	c.Start(ctx)

	<-ctx.Done()
	time.Sleep(50 * time.Millisecond)
	assert.NoError(t, c.Close())
}

// TestConsumer_MultipleEvents verifies the consumer processes a sequence of
// mixed event types without error.
func TestConsumer_MultipleEvents(t *testing.T) {
	helpers.RequirePrivileged(t)

	records := []ringbuf.Record{
		buildExecEvent(100, "sh", "/bin/sh"),
		buildTCPEvent(200, "go", [4]byte{127, 0, 0, 1}, [4]byte{8, 8, 8, 8}),
		buildConnectEvent(300, "curl"),
		buildExecEvent(400, "python", "/usr/bin/python3"),
	}
	reader := newMockReader(records)

	mp := noop.NewMeterProvider()
	c, err := audit.NewConsumer(reader, mp.Meter("ebpf-test"), audit.Config{})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	c.Start(ctx)

	<-ctx.Done()
	time.Sleep(50 * time.Millisecond)
	assert.NoError(t, c.Close())
}

// TestConsumer_ShortRecord verifies the consumer handles truncated records
// gracefully (no panic).
func TestConsumer_ShortRecord(t *testing.T) {
	helpers.RequirePrivileged(t)

	// Record too short to contain even the type + comm fields.
	shortRecord := ringbuf.Record{RawSample: []byte{auditExec}}
	records := []ringbuf.Record{shortRecord}
	reader := newMockReader(records)

	mp := noop.NewMeterProvider()
	c, err := audit.NewConsumer(reader, mp.Meter("ebpf-test"), audit.Config{})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	c.Start(ctx)

	<-ctx.Done()
	time.Sleep(50 * time.Millisecond)
	assert.NoError(t, c.Close())
}

// --- ClickHouse sidecar integration test ---

const clickhouseAuditDDL = `
CREATE TABLE IF NOT EXISTS audit_events (
    timestamp DateTime64(9),
    event_type UInt8,
    pid UInt32,
    comm String,
    path String,
    src_ip String,
    dst_ip String
) ENGINE = MergeTree()
ORDER BY timestamp
`

// startClickHouse spins up a ClickHouse container using testcontainers-go.
func startClickHouse(ctx context.Context, t *testing.T) (testcontainers.Container, string) {
	t.Helper()

	// When this test runs inside a Docker-in-Docker privileged CI harness,
	// the nested ClickHouse container inherits the HOST's seccomp profile
	// instead of the outer container's `--privileged` relaxation. That
	// denies `get_mempolicy`/`set_mempolicy`, which ClickHouse calls at
	// startup for NUMA bookkeeping, and the image spins indefinitely
	// printing "Operation not permitted" instead of serving connections.
	//
	// Passing `seccomp=unconfined` + `apparmor=unconfined` via HostConfig
	// lifts that restriction so ClickHouse boots normally. It is safe for
	// the test context because we only talk to it over the loopback-bound
	// exposed port.
	req := testcontainers.ContainerRequest{
		Image:        "clickhouse/clickhouse-server:24-alpine",
		ExposedPorts: []string{"9000/tcp", "8123/tcp"},
		HostConfigModifier: func(hc *dockercontainer.HostConfig) {
			hc.SecurityOpt = append(hc.SecurityOpt,
				"seccomp=unconfined",
				"apparmor=unconfined",
			)
		},
		WaitingFor: wait.ForLog("Ready for connections").
			WithStartupTimeout(90 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start ClickHouse container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get ClickHouse host: %v", err)
	}

	port, err := container.MappedPort(ctx, "9000")
	if err != nil {
		t.Fatalf("failed to get ClickHouse port: %v", err)
	}

	dsn := fmt.Sprintf("clickhouse://%s:%s/default", host, port.Port())
	return container, dsn
}

// TestAuditClickHousePersistence tests that audit events can be persisted to
// a ClickHouse sidecar. This validates the full write path: event construction,
// DDL, and INSERT.
func TestAuditClickHousePersistence(t *testing.T) {
	helpers.RequirePrivileged(t)

	if testing.Short() {
		t.Skip("skipping ClickHouse integration test in short mode")
	}

	ctx := context.Background()

	container, dsn := startClickHouse(ctx, t)
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate ClickHouse container: %v", err)
		}
	}()

	// Connect to ClickHouse.
	db, err := sql.Open("clickhouse", dsn)
	require.NoError(t, err)
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close ClickHouse connection: %v", err)
		}
	}()

	// Wait for ClickHouse to be ready.
	for i := 0; i < 30; i++ {
		if err := db.PingContext(ctx); err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	require.NoError(t, db.PingContext(ctx), "ClickHouse not reachable")

	// Create the audit_events table.
	_, err = db.ExecContext(ctx, clickhouseAuditDDL)
	require.NoError(t, err, "DDL creation must succeed")

	// Insert a synthetic audit event.
	_, err = db.ExecContext(ctx,
		`INSERT INTO audit_events (timestamp, event_type, pid, comm, path, src_ip, dst_ip)
		 VALUES (now64(9), ?, ?, ?, ?, ?, ?)`,
		auditExec, 1234, "bash", "/usr/bin/curl", "", "",
	)
	require.NoError(t, err, "INSERT must succeed")

	// Insert a TCP event.
	_, err = db.ExecContext(ctx,
		`INSERT INTO audit_events (timestamp, event_type, pid, comm, path, src_ip, dst_ip)
		 VALUES (now64(9), ?, ?, ?, ?, ?, ?)`,
		auditTCP, 5678, "nginx", "", "192.168.1.10", "10.0.0.1",
	)
	require.NoError(t, err, "INSERT for TCP event must succeed")

	// Verify row count.
	var count uint64
	err = db.QueryRowContext(ctx, "SELECT count() FROM audit_events").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), count, "expected exactly 2 audit events in ClickHouse")

	// Verify we can query back specific fields.
	var comm string
	var eventType uint8
	err = db.QueryRowContext(ctx,
		"SELECT event_type, comm FROM audit_events WHERE pid = 1234").Scan(&eventType, &comm)
	require.NoError(t, err)
	assert.Equal(t, uint8(auditExec), eventType)
	assert.Equal(t, "bash", comm)
}
