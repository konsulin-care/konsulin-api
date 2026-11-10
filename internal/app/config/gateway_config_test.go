package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// gatewayEnvKeys lists every gateway/party env var so tests can pin them all
// to empty and prove which ones validation actually requires.
var gatewayEnvKeys = []string{
	"APP_GATEWAY_URL",
	"APP_PARTY_ID",
	"APP_PARTY_PRIVATE_KEY",
	"APP_PARTY_ENC_PRIVATE_KEY",
	"APP_PARTY_ENROLLMENT_TOKEN",
	"APP_PARTY_NEXT_PRIVATE_KEY",
	"APP_PARTY_NEXT_ENC_PRIVATE_KEY",
}

// setEmptyGatewayEnv pins APP_ENV and every gateway/party var to empty so
// ambient environment cannot leak into the assertions.
func setEmptyGatewayEnv(t *testing.T, env string) {
	t.Helper()
	t.Setenv("APP_ENV", env)
	for _, key := range gatewayEnvKeys {
		t.Setenv(key, "")
	}
}

// TestLoadInternalConfig_gatewayDisabledWithoutURL asserts that an empty
// APP_GATEWAY_URL disables the gateway (standalone mode) and that none of the
// APP_PARTY_* variables are required in that state.
func TestLoadInternalConfig_gatewayDisabledWithoutURL(t *testing.T) {
	setEmptyGatewayEnv(t, "local")

	cfg, err := loadInternalConfigWithEnv()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.False(t, cfg.Gateway.Enabled)
	assert.Empty(t, cfg.Gateway.URL)
}
