package jobs

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/queue"
)

// ── Task creation tests ─────────────────────────────────────────────────────

func TestNewRecrawlTask(t *testing.T) {
	p := RecrawlPayload{OrgID: "org-1"}
	task, err := NewRecrawlTask(p)
	require.NoError(t, err)
	assert.Equal(t, TypeRecrawlSources, task.Type())

	var got RecrawlPayload
	err = json.Unmarshal(task.Payload(), &got)
	require.NoError(t, err)
	assert.Equal(t, p, got)
}

func TestNewRecrawlTaskEmptyPayload(t *testing.T) {
	p := RecrawlPayload{}
	task, err := NewRecrawlTask(p)
	require.NoError(t, err)
	assert.Equal(t, TypeRecrawlSources, task.Type())

	var got RecrawlPayload
	err = json.Unmarshal(task.Payload(), &got)
	require.NoError(t, err)
	assert.Empty(t, got.OrgID)
}

func TestNewCleanupTask(t *testing.T) {
	p := CleanupPayload{
		SessionMaxAgeDays:  15,
		EventRetentionDays: 60,
	}
	task, err := NewCleanupTask(p)
	require.NoError(t, err)
	assert.Equal(t, TypeCleanupSessions, task.Type())

	var got CleanupPayload
	err = json.Unmarshal(task.Payload(), &got)
	require.NoError(t, err)
	assert.Equal(t, p, got)
}

func TestNewCleanupTaskDefaults(t *testing.T) {
	p := CleanupPayload{}
	task, err := NewCleanupTask(p)
	require.NoError(t, err)

	var got CleanupPayload
	err = json.Unmarshal(task.Payload(), &got)
	require.NoError(t, err)
	assert.Equal(t, 0, got.SessionMaxAgeDays)
	assert.Equal(t, 0, got.EventRetentionDays)
}

func TestNewUsageAggregationTask(t *testing.T) {
	p := UsageAggregationPayload{
		OrgID:         "org-42",
		WindowMinutes: 120,
	}
	task, err := NewUsageAggregationTask(p)
	require.NoError(t, err)
	assert.Equal(t, TypeUsageAggregation, task.Type())

	var got UsageAggregationPayload
	err = json.Unmarshal(task.Payload(), &got)
	require.NoError(t, err)
	assert.Equal(t, p, got)
}

func TestNewUsageAggregationTaskEmptyPayload(t *testing.T) {
	p := UsageAggregationPayload{}
	task, err := NewUsageAggregationTask(p)
	require.NoError(t, err)

	var got UsageAggregationPayload
	err = json.Unmarshal(task.Payload(), &got)
	require.NoError(t, err)
	assert.Empty(t, got.OrgID)
	assert.Equal(t, 0, got.WindowMinutes)
}

// ── Payload JSON round-trip tests ───────────────────────────────────────────

func TestRecrawlPayloadJSON(t *testing.T) {
	p := RecrawlPayload{OrgID: "org-abc"}
	data, err := json.Marshal(p)
	require.NoError(t, err)

	var decoded RecrawlPayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, p, decoded)
}

func TestCleanupPayloadJSON(t *testing.T) {
	p := CleanupPayload{SessionMaxAgeDays: 7, EventRetentionDays: 30}
	data, err := json.Marshal(p)
	require.NoError(t, err)

	var decoded CleanupPayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, p, decoded)
}

func TestUsageAggregationPayloadJSON(t *testing.T) {
	p := UsageAggregationPayload{OrgID: "org-x", WindowMinutes: 30}
	data, err := json.Marshal(p)
	require.NoError(t, err)

	var decoded UsageAggregationPayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, p, decoded)
}

// ── Task type constant tests ────────────────────────────────────────────────

func TestTaskTypeConstants(t *testing.T) {
	// Verify task types follow the "scheduled:" prefix convention.
	assert.Equal(t, "scheduled:recrawl_sources", TypeRecrawlSources)
	assert.Equal(t, "scheduled:cleanup_sessions", TypeCleanupSessions)
	assert.Equal(t, "scheduled:usage_aggregation", TypeUsageAggregation)
}

// ── Cron expression tests ───────────────────────────────────────────────────

func TestCronExpressions(t *testing.T) {
	assert.Equal(t, "0 */6 * * *", CronRecrawl)
	assert.Equal(t, "0 2 * * *", CronCleanup)
	assert.Equal(t, "5 * * * *", CronUsageAggregation)
}

// ── Scheduler creation tests ────────────────────────────────────────────────

func TestNewScheduler(t *testing.T) {
	mr := miniredis.RunT(t)
	qc := queue.NewClient(mr.Addr(), queue.WithMaxRetry(1))
	t.Cleanup(func() { _ = qc.Close() })

	s, err := NewScheduler(SchedulerConfig{
		RedisAddr:   mr.Addr(),
		Pool:        nil, // No real DB needed for construction test.
		QueueClient: qc,
	})
	require.NoError(t, err)
	assert.NotNil(t, s)
	assert.NotNil(t, s.Handlers())
}

func TestNewSchedulerWithLogger(t *testing.T) {
	mr := miniredis.RunT(t)
	qc := queue.NewClient(mr.Addr())
	t.Cleanup(func() { _ = qc.Close() })

	s, err := NewScheduler(SchedulerConfig{
		RedisAddr:   mr.Addr(),
		QueueClient: qc,
	})
	require.NoError(t, err)
	assert.NotNil(t, s)
}

// ── Handler construction tests ──────────────────────────────────────────────

func TestNewRecrawlHandler(t *testing.T) {
	mr := miniredis.RunT(t)
	qc := queue.NewClient(mr.Addr())
	t.Cleanup(func() { _ = qc.Close() })

	h := NewRecrawlHandler(nil, qc, nil)
	assert.NotNil(t, h)
	assert.NotNil(t, h.logger)
}

func TestNewCleanupHandler(t *testing.T) {
	h := NewCleanupHandler(nil, nil)
	assert.NotNil(t, h)
	assert.NotNil(t, h.logger)
}

func TestNewUsageAggregationHandler(t *testing.T) {
	h := NewUsageAggregationHandler(nil, nil)
	assert.NotNil(t, h)
	assert.NotNil(t, h.logger)
}

// ── Handler ProcessTask tests (invalid payload) ─────────────────────────────

func TestRecrawlHandlerInvalidPayload(t *testing.T) {
	mr := miniredis.RunT(t)
	qc := queue.NewClient(mr.Addr())
	t.Cleanup(func() { _ = qc.Close() })

	h := NewRecrawlHandler(nil, qc, nil)
	task := asynq.NewTask(TypeRecrawlSources, []byte("not-json"))
	err := h.ProcessTask(context.Background(), task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal RecrawlPayload")
}

func TestCleanupHandlerInvalidPayload(t *testing.T) {
	h := NewCleanupHandler(nil, nil)
	task := asynq.NewTask(TypeCleanupSessions, []byte("not-json"))
	err := h.ProcessTask(context.Background(), task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal CleanupPayload")
}

func TestUsageAggregationHandlerInvalidPayload(t *testing.T) {
	h := NewUsageAggregationHandler(nil, nil)
	task := asynq.NewTask(TypeUsageAggregation, []byte("not-json"))
	err := h.ProcessTask(context.Background(), task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal UsageAggregationPayload")
}

// ── frequencyToDuration tests ───────────────────────────────────────────────

func TestFrequencyToDuration(t *testing.T) {
	tests := []struct {
		name     string
		freq     model.CrawlFrequency
		expected string
	}{
		{"daily", model.CrawlFrequencyDaily, "24h0m0s"},
		{"weekly", model.CrawlFrequencyWeekly, "168h0m0s"},
		{"monthly", model.CrawlFrequencyMonthly, "720h0m0s"},
		{"manual returns zero", model.CrawlFrequencyManual, "0s"},
		{"unknown returns zero", model.CrawlFrequency("unknown"), "0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := frequencyToDuration(tt.freq)
			assert.Equal(t, tt.expected, d.String())
		})
	}
}

// ── Handler mux routing tests ───────────────────────────────────────────────

func TestSchedulerHandlersMuxNotNil(t *testing.T) {
	mr := miniredis.RunT(t)
	qc := queue.NewClient(mr.Addr())
	t.Cleanup(func() { _ = qc.Close() })

	s, err := NewScheduler(SchedulerConfig{
		RedisAddr:   mr.Addr(),
		QueueClient: qc,
	})
	require.NoError(t, err)

	mux := s.Handlers()
	assert.NotNil(t, mux)
}

// ── CleanupPayload default value tests ──────────────────────────────────────

func TestCleanupPayloadDefaultValues(t *testing.T) {
	// When zero values are passed, the handler should apply defaults internally.
	p := CleanupPayload{}

	// Verify defaults are applied by the constants (not the struct).
	assert.Equal(t, 30, defaultSessionMaxAgeDays)
	assert.Equal(t, 90, defaultEventRetentionDays)

	// Zero values in the struct mean "use defaults".
	assert.Equal(t, 0, p.SessionMaxAgeDays)
	assert.Equal(t, 0, p.EventRetentionDays)
}

// ── UsageAggregation default window test ────────────────────────────────────

func TestUsageAggregationDefaultWindow(t *testing.T) {
	assert.Equal(t, 60, defaultWindowMinutes)
}
