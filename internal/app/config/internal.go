package config

import (
	"fmt"
	"strings"

	"konsulin-service/internal/pkg/constvars"
)

type InternalConfig struct {
	App        App           `mapstructure:"app"`
	FHIR       AppFHIR       `mapstructure:"fhir"`
	JWT        AppJWT        `mapstructure:"jwt"`
	Supertoken AppSupertoken `mapstructure:"supertoken"`
	Webhook    AppWebhook    `mapstructure:"webhook"`
	Gateway    AppGateway    `mapstructure:"gateway"`
}

type App struct {
	Env                         string `mapstructure:"env"`
	Port                        string `mapstructure:"port"`
	Version                     string `mapstructure:"version"`
	Address                     string `mapstructure:"address"`
	BaseUrl                     string `mapstructure:"base_url"`
	Timezone                    string `mapstructure:"timezone"`
	FrontendDomain              string `mapstructure:"frontend_domain"`
	EndpointPrefix              string `mapstructure:"endpoint_prefix"`
	MaxRequests                 int    `mapstructure:"max_requests"`
	ShutdownTimeoutInSeconds    int    `mapstructure:"shutdown_timeout_in_seconds"`
	PaymentExpiredTimeInMinutes int    `mapstructure:"payment_expired_time_in_minutes"`
	SuperadminAPIKey            string `mapstructure:"superadmin_api_key"`
	SuperadminAPIKeyRateLimit   int    `mapstructure:"superadmin_api_key_rate_limit"`
	RBACPolicyPath              string `mapstructure:"rbac_policy_path"`
	OrganizationName            string `mapstructure:"organization_name"`
	DataDir                     string `mapstructure:"data_dir"`
	Pepper                      string `mapstructure:"pepper"` // Sensitive — never logged
}

type AppFHIR struct {
	BaseUrl string `mapstructure:"base_url"`
}

type AppJWT struct {
	Secret string `mapstructure:"secret"`
}

type AppSupertoken struct {
	KonsulinTenantID           string `mapstructure:"konsulin_tenant_id"`
	KonsulinDasboardAdminEmail string `mapstructure:"konsulin_dashboard_admin_email"`
}

// AppWebhook retains only the knobs still consumed outside config: the
// services rate-limit window read by the T37 limiter in ServicesController
// and the delivery timeout placeholder pinned by the auth test fixture.
// All HOOK_* environment reads were removed with the legacy webhook stack.
type AppWebhook struct {
	// HTTPTimeoutInSeconds is retained as a fixture pin for the auth tests
	// (the magic-link sender uses a fixed timeout and ignores it)
	HTTPTimeoutInSeconds int `mapstructure:"http_timeout_in_seconds"`
	// SynchronousServiceRateLimit controls requests allowed per window for synchronous services
	SynchronousServiceRateLimit int `mapstructure:"synchronous_service_rate_limit"`
	// SynchronousServiceWindowSeconds is the limiter window duration in seconds for synchronous services
	SynchronousServiceWindowSeconds int `mapstructure:"synchronous_service_window_seconds"`
}

// AppGateway holds gateway/party configuration for federation mode.
// An empty URL means standalone mode: no gateway and no APP_PARTY_* required.
type AppGateway struct {
	// Enabled is true when APP_GATEWAY_URL is non-empty
	Enabled bool `mapstructure:"enabled"`
	// URL is the gateway base URL; https:// required outside dev-like environments
	URL string `mapstructure:"url"`
	// PartyID is the party ID issued out of band by the gateway admin
	PartyID string `mapstructure:"party_id"`
	// PrivateKey is the signing key PEM; sensitive, never logged
	PrivateKey string `mapstructure:"private_key"`
	// EncPrivateKey is the X25519 key PEM opening JWE envelopes; sensitive, never logged
	EncPrivateKey string `mapstructure:"enc_private_key"`
	// EnrollmentToken is the single-use enrollment token; sensitive, never logged
	EnrollmentToken string `mapstructure:"enrollment_token"`
	// NextPrivateKey is the next signing key for self-service rotation; sensitive, never logged
	NextPrivateKey string `mapstructure:"next_private_key"`
	// NextEncPrivateKey is the next enc key for self-service rotation; sensitive, never logged
	NextEncPrivateKey string `mapstructure:"next_enc_private_key"`
	// AutomationPartyID is the automation provider party ID
	AutomationPartyID string `mapstructure:"automation_party_id"`
	// ServiceProviderAliases is a comma-separated alias=partyId list
	ServiceProviderAliases string `mapstructure:"service_provider_aliases"`
}

// validateGatewayConfig validates gateway/party configuration in
// loadInternalConfigWithEnv. Errors name the offending variable only and
// never include key material.
func validateGatewayConfig(cfg *InternalConfig) error {
	gw := &cfg.Gateway
	gw.URL = strings.TrimSpace(gw.URL)
	gw.Enabled = gw.URL != ""
	if !gw.Enabled {
		return nil // standalone mode: no APP_PARTY_* required
	}
	if strings.HasPrefix(strings.ToLower(gw.URL), "http://") &&
		cfg.App.Env != constvars.EnvLocal && cfg.App.Env != constvars.EnvDev &&
		cfg.App.Env != constvars.EnvDevelopment && cfg.App.Env != constvars.EnvTest {
		return fmt.Errorf("env var APP_GATEWAY_URL must use https:// in %s environment", cfg.App.Env)
	}
	for _, required := range []struct{ value, name string }{
		{gw.PartyID, "APP_PARTY_ID"},
		{gw.PrivateKey, "APP_PARTY_PRIVATE_KEY"},
		{gw.EncPrivateKey, "APP_PARTY_ENC_PRIVATE_KEY"},
	} {
		if required.value == "" {
			return fmt.Errorf("env var %s is required when APP_GATEWAY_URL is set", required.name)
		}
	}
	return nil
}
