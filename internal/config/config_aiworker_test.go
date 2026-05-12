package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIWorkerBreakerHalfOpenMax_Default(t *testing.T) {
	t.Setenv("RAVEN_DATABASE_URL", "postgres://x:x@localhost/x")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_USER_LIMIT", "100")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_ORG_LIMIT", "1000")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, uint32(1), cfg.Server.AIWorkerBreakerHalfOpenMax,
		"AIWorkerBreakerHalfOpenMax must default to 1 when env is unset")
}

func TestAIWorkerBreakerHalfOpenMax_EnvOverride(t *testing.T) {
	t.Setenv("RAVEN_DATABASE_URL", "postgres://x:x@localhost/x")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_USER_LIMIT", "100")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_ORG_LIMIT", "1000")
	t.Setenv("RAVEN_AI_WORKER_BREAKER_HALF_OPEN_MAX", "3")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, uint32(3), cfg.Server.AIWorkerBreakerHalfOpenMax,
		"AIWorkerBreakerHalfOpenMax must reflect RAVEN_AI_WORKER_BREAKER_HALF_OPEN_MAX=3")
}
