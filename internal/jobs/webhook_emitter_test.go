package jobs

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/model"
)

// fakeJobDispatcher captures Dispatch calls so tests can verify that the
// document and Airbyte job handlers fan out the right webhook events with
// the right payloads.
type fakeJobDispatcher struct {
	mu     sync.Mutex
	calls  []jobDispatchCall
	doneCh chan struct{}
	err    error
}

type jobDispatchCall struct {
	orgID     string
	eventType string
	payload   map[string]any
}

func newFakeJobDispatcher() *fakeJobDispatcher {
	return &fakeJobDispatcher{doneCh: make(chan struct{}, 1)}
}

func (f *fakeJobDispatcher) Dispatch(_ context.Context, orgID, eventType string, payload map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, jobDispatchCall{orgID: orgID, eventType: eventType, payload: payload})
	select {
	case f.doneCh <- struct{}{}:
	default:
	}
	return f.err
}

func (f *fakeJobDispatcher) waitForCall(t *testing.T) {
	t.Helper()
	select {
	case <-f.doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for dispatcher to be called")
	}
}

func (f *fakeJobDispatcher) snapshot() []jobDispatchCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]jobDispatchCall, len(f.calls))
	copy(out, f.calls)
	return out
}

func TestDispatchAsync_FiresDocumentProcessedEvent(t *testing.T) {
	disp := newFakeJobDispatcher()
	logger := slog.New(slog.DiscardHandler)

	dispatchAsync(context.Background(), disp, logger, "org-1",
		string(model.WebhookEventDocumentProcessed),
		map[string]any{
			"document_id":       "doc-1",
			"knowledge_base_id": "kb-1",
			"status":            "ready",
			"chunk_count":       7,
		})
	disp.waitForCall(t)

	calls := disp.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "org-1", calls[0].orgID)
	assert.Equal(t, "document.processed", calls[0].eventType)
	assert.Equal(t, "doc-1", calls[0].payload["document_id"])
	assert.Equal(t, "kb-1", calls[0].payload["knowledge_base_id"])
	assert.Equal(t, "ready", calls[0].payload["status"])
	assert.Equal(t, 7, calls[0].payload["chunk_count"])
}

func TestDispatchAsync_FiresSyncCompletedEvent(t *testing.T) {
	disp := newFakeJobDispatcher()
	logger := slog.New(slog.DiscardHandler)

	dispatchAsync(context.Background(), disp, logger, "org-2",
		string(model.WebhookEventSyncCompleted),
		map[string]any{
			"connector_id":   "conn-1",
			"source_id":      "kb-2",
			"sync_run_id":    "run-1",
			"records_synced": 0,
			"status":         "completed",
		})
	disp.waitForCall(t)

	calls := disp.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "org-2", calls[0].orgID)
	assert.Equal(t, "sync.completed", calls[0].eventType)
	assert.Equal(t, "conn-1", calls[0].payload["connector_id"])
	assert.Equal(t, "kb-2", calls[0].payload["source_id"])
	assert.Equal(t, "run-1", calls[0].payload["sync_run_id"])
	assert.Equal(t, 0, calls[0].payload["records_synced"])
	assert.Equal(t, "completed", calls[0].payload["status"])
}

func TestDispatchAsync_NilDispatcherIsNoOp(t *testing.T) {
	// A nil dispatcher must not panic and must not start a goroutine.
	dispatchAsync(context.Background(), nil, slog.New(slog.DiscardHandler),
		"org", "document.processed", map[string]any{"k": "v"})
}

func TestDispatchAsync_ErrorIsSwallowed(t *testing.T) {
	disp := newFakeJobDispatcher()
	disp.err = errors.New("downstream failure")
	logger := slog.New(slog.DiscardHandler)

	// Producer must not panic or block even when Dispatch returns an
	// error — the error path only logs and the goroutine completes.
	dispatchAsync(context.Background(), disp, logger, "org",
		"document.processed", map[string]any{"document_id": "d"})
	disp.waitForCall(t)
	assert.Len(t, disp.snapshot(), 1)
}

// TestDispatchAsync_DetachesFromCallerContext verifies that cancelling the
// caller's context does not prevent Dispatch from running. This is the
// `context.WithoutCancel` guarantee documented in the dispatcher.
func TestDispatchAsync_DetachesFromCallerContext(t *testing.T) {
	disp := newFakeJobDispatcher()
	logger := slog.New(slog.DiscardHandler)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled

	dispatchAsync(ctx, disp, logger, "org",
		"document.processed", map[string]any{"document_id": "d"})
	disp.waitForCall(t)
	require.Len(t, disp.snapshot(), 1)
}
