package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLoadInternalConfigWithEnv_ShutdownTimeoutDefault verifies the env loader
// falls back to a 30-second default when APP_SHUTDOWN_TIMEOUT_IN_SECONDS is unset.
func TestLoadInternalConfigWithEnv_ShutdownTimeoutDefault(t *testing.T) {
	t.Setenv("APP_SHUTDOWN_TIMEOUT_IN_SECONDS", "")
	cfg, err := loadInternalConfigWithEnv()
	assert.NoError(t, err)
	assert.Equal(t, 30, cfg.App.ShutdownTimeoutInSeconds)
}

// TestLoadInternalConfigWithEnv_ShutdownTimeoutFromEnv verifies the env loader
// binds APP_SHUTDOWN_TIMEOUT_IN_SECONDS into the App struct.
func TestLoadInternalConfigWithEnv_ShutdownTimeoutFromEnv(t *testing.T) {
	t.Setenv("APP_SHUTDOWN_TIMEOUT_IN_SECONDS", "45")
	cfg, err := loadInternalConfigWithEnv()
	assert.NoError(t, err)
	assert.Equal(t, 45, cfg.App.ShutdownTimeoutInSeconds)
}
