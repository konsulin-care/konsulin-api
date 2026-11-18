package config

import (
	"fmt"
	"strings"

	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
)

// devLikeEnvs lists the environments where a plain http:// gateway URL is
// allowed. Everywhere else APP_GATEWAY_URL must use https://.
var devLikeEnvs = []string{
	constvars.EnvLocal,
	constvars.EnvDev,
	constvars.EnvDevelopment,
	constvars.EnvTest,
}

// isDevLikeEnv reports whether env is one of the dev/test environments where
// non-TLS gateway URLs are permitted.
//
// Parameters:
//   - env: the application environment name (APP_ENV).
//
// Returns true for local, dev, development and test.
func isDevLikeEnv(env string) bool {
	for _, candidate := range devLikeEnvs {
		if env == candidate {
			return true
		}
	}
	return false
}

// validateGatewayURLScheme checks APP_GATEWAY_URL. An empty value (standalone
// mode) always passes; an http:// URL passes only in dev-like environments;
// any other scheme is accepted.
//
// Parameters:
//   - rawURL: the configured gateway URL, possibly with surrounding spaces.
//   - env: the application environment name (APP_ENV).
//
// Returns nil when the URL is acceptable, otherwise an error naming
// APP_GATEWAY_URL.
func validateGatewayURLScheme(rawURL, env string) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil
	}
	if !strings.HasPrefix(strings.ToLower(trimmed), "http://") {
		return nil
	}
	if isDevLikeEnv(env) {
		return nil
	}
	return fmt.Errorf("env var APP_GATEWAY_URL must use https:// in %s environment", env)
}

// validateGatewayPartyVars checks that the party variables required while
// APP_GATEWAY_URL is set are present, naming the first missing one. It reads
// no environment directly: values arrive via cfg.
//
// Parameters:
//   - cfg: the internal configuration populated from the environment.
//
// Returns nil when the gateway is disabled or every required party variable
// is set, otherwise an error naming the missing variable. Key material is
// never included in the error.
func validateGatewayPartyVars(cfg *InternalConfig) error {
	if strings.TrimSpace(cfg.Gateway.URL) == "" {
		return nil // standalone mode: no APP_PARTY_* required
	}
	values := []string{cfg.Gateway.PartyID, cfg.Gateway.PrivateKey, cfg.Gateway.EncPrivateKey}
	names := []string{"APP_PARTY_ID", "APP_PARTY_PRIVATE_KEY", "APP_PARTY_ENC_PRIVATE_KEY"}
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("env var %s is required when APP_GATEWAY_URL is set", names[i])
		}
	}
	return nil
}

// validateGatewaySettings validates the gateway/party configuration built by
// loadInternalConfigWithEnv. It performs no direct environment reads (all
// values arrive via cfg) and errors only ever name the offending variable.
//
// Parameters:
//   - cfg: the internal configuration populated from the environment.
//
// Returns nil when the gateway is disabled or fully and validly configured,
// otherwise an error naming the offending environment variable.
func validateGatewaySettings(cfg *InternalConfig) error {
	if err := validateGatewayURLScheme(cfg.Gateway.URL, cfg.App.Env); err != nil {
		return err
	}
	return validateGatewayPartyVars(cfg)
}
