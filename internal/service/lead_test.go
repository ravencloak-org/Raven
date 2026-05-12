package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/model"
)

// fakeLeadDispatcher captures Dispatch calls so tests can assert that
// LeadService fires the right event with the right payload.
type fakeLeadDispatcher struct {
	mu     sync.Mutex
	calls  []dispatchCall
	doneCh chan struct{}
	err    error
}

type dispatchCall struct {
	orgID     string
	eventType string
	payload   map[string]any
}

func newFakeLeadDispatcher() *fakeLeadDispatcher {
	return &fakeLeadDispatcher{doneCh: make(chan struct{}, 1)}
}

func (f *fakeLeadDispatcher) Dispatch(_ context.Context, orgID, eventType string, payload map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, dispatchCall{orgID: orgID, eventType: eventType, payload: payload})
	select {
	case f.doneCh <- struct{}{}:
	default:
	}
	return f.err
}

func (f *fakeLeadDispatcher) waitForCall(t *testing.T) {
	t.Helper()
	select {
	case <-f.doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for dispatcher to be called")
	}
}

func (f *fakeLeadDispatcher) snapshot() []dispatchCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]dispatchCall, len(f.calls))
	copy(out, f.calls)
	return out
}

func TestLeadService_dispatchLeadGenerated_FiresEvent(t *testing.T) {
	disp := newFakeLeadDispatcher()
	svc := (&LeadService{}).WithWebhookDispatcher(disp)

	lead := &model.LeadProfile{
		ID:              "lead-uuid",
		OrgID:           "org-uuid",
		KnowledgeBaseID: "kb-uuid",
		Email:           "alice@example.com",
		Name:            "Alice",
		SessionIDs:      []string{"sess-1", "sess-2"},
	}
	svc.dispatchLeadGenerated(context.Background(), "org-uuid", lead)
	disp.waitForCall(t)

	calls := disp.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "org-uuid", calls[0].orgID)
	assert.Equal(t, string(model.WebhookEventLeadGenerated), calls[0].eventType)
	assert.Equal(t, "lead-uuid", calls[0].payload["lead_id"])
	assert.Equal(t, "alice@example.com", calls[0].payload["email"])
	assert.Equal(t, "Alice", calls[0].payload["name"])
	assert.Equal(t, "kb-uuid", calls[0].payload["knowledge_base_id"])
	assert.Equal(t, []string{"sess-1", "sess-2"}, calls[0].payload["session_ids"])
}

func TestLeadService_dispatchLeadGenerated_NoDispatcherIsNoOp(t *testing.T) {
	svc := &LeadService{}
	// Must not panic and must not block when no dispatcher is configured.
	svc.dispatchLeadGenerated(context.Background(), "org", &model.LeadProfile{ID: "lead"})
}

func TestLeadService_dispatchLeadGenerated_DispatcherErrorSwallowed(t *testing.T) {
	disp := newFakeLeadDispatcher()
	disp.err = errors.New("queue unavailable")
	svc := (&LeadService{}).WithWebhookDispatcher(disp)

	// The producer must not panic or block even when Dispatch returns an
	// error — the error path only logs and carries on.
	svc.dispatchLeadGenerated(context.Background(), "org", &model.LeadProfile{ID: "lead"})
	disp.waitForCall(t)
	assert.Len(t, disp.snapshot(), 1)
}

func TestLeadService_dispatchLeadGenerated_NilLeadIsNoOp(t *testing.T) {
	disp := newFakeLeadDispatcher()
	svc := (&LeadService{}).WithWebhookDispatcher(disp)
	svc.dispatchLeadGenerated(context.Background(), "org", nil)
	// No call expected.
	select {
	case <-disp.doneCh:
		t.Fatal("dispatcher should not be called when lead is nil")
	case <-time.After(50 * time.Millisecond):
	}
	assert.Empty(t, disp.snapshot())
}
