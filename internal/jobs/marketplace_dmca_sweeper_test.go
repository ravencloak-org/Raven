package jobs

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewMarketplaceDMCASweepTask pins the task-type constant and payload
// round-trip. Mirrors TestNewCleanupTask in scheduler_test.go.
func TestNewMarketplaceDMCASweepTask(t *testing.T) {
	p := MarketplaceDMCASweepPayload{}
	task, err := NewMarketplaceDMCASweepTask(p)
	require.NoError(t, err)
	assert.Equal(t, TypeMarketplaceDMCASweep, task.Type())

	var got MarketplaceDMCASweepPayload
	err = json.Unmarshal(task.Payload(), &got)
	require.NoError(t, err)
	assert.Equal(t, p, got)
}

// TestDMCASweeper_NilServiceIsNoop verifies the defensive check inside
// ProcessTask: a missing DMCAService is treated as a wiring bug, logged,
// and returns nil so Asynq does not accumulate retry pressure.
func TestDMCASweeper_NilServiceIsNoop(t *testing.T) {
	h := NewDMCASweeper(nil, nil)
	task, err := NewMarketplaceDMCASweepTask(MarketplaceDMCASweepPayload{})
	require.NoError(t, err)

	if err := h.ProcessTask(context.Background(), task); err != nil {
		t.Errorf("nil svc should no-op, got %v", err)
	}
}

// TestDMCASweeper_BadPayloadReturnsError verifies the payload-parse
// failure surfaces as an Asynq-retryable error.
func TestDMCASweeper_BadPayloadReturnsError(t *testing.T) {
	h := NewDMCASweeper(nil, nil)
	bad := asynq.NewTask(TypeMarketplaceDMCASweep, []byte("not json"))
	if err := h.ProcessTask(context.Background(), bad); err == nil {
		t.Error("expected error on malformed payload, got nil")
	}
}
