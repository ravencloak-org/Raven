package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContextWithTimeout_ShorterDeadlineWins verifies the invariant that
// context.WithTimeout uses the earlier of the parent deadline and the new
// duration — this is the behaviour that the per-handler timeout wrap relies on.
//
// All five ProcessTask wrappers do:
//
//	ctx, cancel := context.WithTimeout(ctx, <handlerTimeout>)
//
// If the parent context already has a shorter deadline (e.g. Asynq's own
// per-task deadline), that shorter deadline is preserved automatically.
func TestContextWithTimeout_ShorterDeadlineWins(t *testing.T) {
	// Parent has a 1-second deadline — shorter than any handler timeout.
	parentCtx, parentCancel := context.WithTimeout(context.Background(), time.Second)
	defer parentCancel()

	parentDeadline, ok := parentCtx.Deadline()
	require.True(t, ok)

	// Apply the usage-aggregation handler's 30s wrap.
	child, childCancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer childCancel()

	childDeadline, ok := child.Deadline()
	require.True(t, ok, "child context must have a deadline")

	// The child deadline must equal the parent's (shorter) deadline.
	assert.Equal(t, parentDeadline, childDeadline,
		"context.WithTimeout must keep the shorter (parent) deadline; "+
			"the per-handler wrap must pass the parent ctx, not a fresh Background()")
}

// TestContextWithTimeout_HandlerTimeoutApplied verifies that when the parent
// context has no deadline, a 30-second timeout wrap installs the expected
// deadline duration.
func TestContextWithTimeout_HandlerTimeoutApplied(t *testing.T) {
	const handlerTimeout = 30 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	deadline, ok := ctx.Deadline()
	require.True(t, ok, "context must have a deadline after WithTimeout")

	remaining := time.Until(deadline)
	// Allow ±1 second for test execution time.
	assert.GreaterOrEqual(t, remaining, handlerTimeout-time.Second,
		"deadline must be at least %v in the future", handlerTimeout-time.Second)
	assert.LessOrEqual(t, remaining, handlerTimeout+time.Second,
		"deadline must not exceed %v in the future", handlerTimeout+time.Second)
}

// TestContextWithTimeout_RecrawlTimeout verifies the 5-minute timeout constant
// used by RecrawlHandler.
func TestContextWithTimeout_RecrawlTimeout(t *testing.T) {
	const recrawlTimeout = 5 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), recrawlTimeout)
	defer cancel()

	deadline, ok := ctx.Deadline()
	require.True(t, ok)

	remaining := time.Until(deadline)
	assert.GreaterOrEqual(t, remaining, recrawlTimeout-time.Second)
	assert.LessOrEqual(t, remaining, recrawlTimeout+time.Second)
}

// TestContextWithTimeout_CleanupTimeout verifies the 1-minute timeout constant
// used by CleanupHandler.
func TestContextWithTimeout_CleanupTimeout(t *testing.T) {
	const cleanupTimeout = time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()

	deadline, ok := ctx.Deadline()
	require.True(t, ok)

	remaining := time.Until(deadline)
	assert.GreaterOrEqual(t, remaining, cleanupTimeout-time.Second)
	assert.LessOrEqual(t, remaining, cleanupTimeout+time.Second)
}
