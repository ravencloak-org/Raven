package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabaseConfig_AutoMigrate_DefaultsOff(t *testing.T) {
	t.Setenv("RAVEN_DATABASE_URL", "postgres://x:x@localhost/x")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_USER_LIMIT", "100")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_ORG_LIMIT", "1000")

	cfg, err := Load()
	require.NoError(t, err)

	assert.False(t, cfg.Database.AutoMigrate,
		"AutoMigrate must default to false so operators never run migrations by accident")
}

func TestDatabaseConfig_AutoMigrate_EnvOverride(t *testing.T) {
	t.Setenv("RAVEN_DATABASE_URL", "postgres://x:x@localhost/x")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_USER_LIMIT", "100")
	t.Setenv("RAVEN_RATELIMIT_DEFAULT_ORG_LIMIT", "1000")
	t.Setenv("RAVEN_DB_AUTO_MIGRATE", "true")

	cfg, err := Load()
	require.NoError(t, err)

	assert.True(t, cfg.Database.AutoMigrate,
		"RAVEN_DB_AUTO_MIGRATE=true must flip the flag on")
}
