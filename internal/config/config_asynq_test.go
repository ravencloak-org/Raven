package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsynqConfig_Defaults(t *testing.T) {
	t.Setenv("RAVEN_DATABASE_URL", "postgres://x:x@localhost/x")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_USER_LIMIT", "100")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_ORG_LIMIT", "1000")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 5*time.Minute, cfg.Asynq.DocumentProcessBudget)
	assert.Equal(t, 5*time.Minute, cfg.Asynq.AirbyteSyncBudget)
	assert.Equal(t, 2*time.Minute, cfg.Asynq.EmailSummaryBudget)
	assert.Equal(t, 30*time.Second, cfg.Asynq.SendEmailBudget)
	assert.Equal(t, 30*time.Second, cfg.Asynq.VoiceUsageAggBudget)
	assert.Equal(t, 30*time.Second, cfg.Asynq.UsageAggBudget)
	assert.Equal(t, 30*time.Second, cfg.Asynq.WebhookDeliveryBudget)
	assert.Equal(t, 2*time.Minute, cfg.Asynq.RecrawlSourcesBudget)
	assert.Equal(t, 2*time.Minute, cfg.Asynq.CleanupSessionsBudget)
	assert.Equal(t, 1*time.Minute, cfg.Asynq.DefaultBudget)
}

func TestAsynqConfig_EnvOverride(t *testing.T) {
	t.Setenv("RAVEN_DATABASE_URL", "postgres://x:x@localhost/x")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_USER_LIMIT", "100")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_ORG_LIMIT", "1000")
	t.Setenv("RAVEN_ASYNQ_SEND_EMAIL_BUDGET", "10s")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 10*time.Second, cfg.Asynq.SendEmailBudget, "RAVEN_ASYNQ_SEND_EMAIL_BUDGET=10s should be read")
	// Other fields keep their defaults.
	assert.Equal(t, 5*time.Minute, cfg.Asynq.DocumentProcessBudget)
	assert.Equal(t, 1*time.Minute, cfg.Asynq.DefaultBudget)
}
