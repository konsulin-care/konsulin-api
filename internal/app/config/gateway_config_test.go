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

// TestLoadInternalConfig_gatewayHTTPRejectedInProduction asserts that an
// http:// APP_GATEWAY_URL fails validation when APP_ENV is production. The
// non-dev mandatory vars are set so the failure must come from the gateway
// scheme check and name APP_GATEWAY_URL.
func TestLoadInternalConfig_gatewayHTTPRejectedInProduction(t *testing.T) {
	setEmptyGatewayEnv(t, "production")
	t.Setenv("APP_JWT_SECRET", "jwt-secret")
	t.Setenv("JWT_HOOK_KEY", "hook-key")
	t.Setenv("APP_XENDIT_API_KEY", "xendit-key")
	t.Setenv("APP_GATEWAY_URL", "http://gateway.example.test")
	t.Setenv("APP_PARTY_ID", "pty-1")
	t.Setenv("APP_PARTY_PRIVATE_KEY", "signing-key")
	t.Setenv("APP_PARTY_ENC_PRIVATE_KEY", "enc-key")

	_, err := loadInternalConfigWithEnv()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "APP_GATEWAY_URL")
}

// TestLoadInternalConfig_gatewayHTTPAcceptedInDevLikeEnvs asserts that an
// http:// APP_GATEWAY_URL is accepted in local, dev, development and test.
func TestLoadInternalConfig_gatewayHTTPAcceptedInDevLikeEnvs(t *testing.T) {
	for _, env := range []string{"local", "dev", "development", "test"} {
		t.Run(env, func(t *testing.T) {
			setEmptyGatewayEnv(t, env)
			t.Setenv("APP_GATEWAY_URL", "http://gateway.example.test")
			t.Setenv("APP_PARTY_ID", "pty-1")
			t.Setenv("APP_PARTY_PRIVATE_KEY", "signing-key")
			t.Setenv("APP_PARTY_ENC_PRIVATE_KEY", "enc-key")

			cfg, err := loadInternalConfigWithEnv()

			assert.NoError(t, err)
			if assert.NotNil(t, cfg) {
				assert.True(t, cfg.Gateway.Enabled)
				assert.Equal(t, "http://gateway.example.test", cfg.Gateway.URL)
			}
		})
	}
}

// TestLoadInternalConfig_gatewayPartyVarsRequiredWithURL asserts that each of
// APP_PARTY_ID, APP_PARTY_PRIVATE_KEY and APP_PARTY_ENC_PRIVATE_KEY is
// required when APP_GATEWAY_URL is set, with the error naming the missing one.
func TestLoadInternalConfig_gatewayPartyVarsRequiredWithURL(t *testing.T) {
	missingKeys := []string{
		"APP_PARTY_ID",
		"APP_PARTY_PRIVATE_KEY",
		"APP_PARTY_ENC_PRIVATE_KEY",
	}
	for _, missing := range missingKeys {
		t.Run(missing, func(t *testing.T) {
			setEmptyGatewayEnv(t, "local")
			t.Setenv("APP_GATEWAY_URL", "https://gateway.example.test")
			t.Setenv("APP_PARTY_ID", "pty-1")
			t.Setenv("APP_PARTY_PRIVATE_KEY", "signing-key")
			t.Setenv("APP_PARTY_ENC_PRIVATE_KEY", "enc-key")
			t.Setenv(missing, "")

			_, err := loadInternalConfigWithEnv()

			assert.Error(t, err)
			assert.Contains(t, err.Error(), missing)
		})
	}
}
