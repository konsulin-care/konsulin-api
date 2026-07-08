package config

import (
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
)

// isLower initial reports whether the first rune of s is a lowercase letter.
func isLower(s string) bool {
	for _, r := range s {
		return unicode.IsLower(r)
	}
	return false
}

func TestValidateNonDevConfig_errorStringsAreLowercase(t *testing.T) {
	tests := []struct {
		name string
		cfg  *InternalConfig
	}{
		{
			name: "production missing APP_JWT_SECRET",
			cfg: &InternalConfig{
				App: App{Env: "production"},
				// JWT.Secret is zero value -> "" triggers error
			},
		},
		{
			name: "production missing JWT_HOOK_KEY",
			cfg: &InternalConfig{
				App: App{Env: "production"},
				JWT: AppJWT{Secret: "set"},
				// Webhook.JWTHookKey is zero value -> "" triggers error
			},
		},
		{
			name: "production missing payment gateway creds",
			cfg: &InternalConfig{
				App:     App{Env: "production"},
				JWT:     AppJWT{Secret: "set"},
				Webhook: AppWebhook{JWTHookKey: "set"},
				// PaymentGateway.Username and ApiKey are zero value -> triggers error
			},
		},
		{
			name: "production missing APP_XENDIT_API_KEY",
			cfg: &InternalConfig{
				App:            App{Env: "production"},
				JWT:            AppJWT{Secret: "set"},
				Webhook:        AppWebhook{JWTHookKey: "set"},
				PaymentGateway: AppPaymentGateway{Username: "u", ApiKey: "k"},
				// Xendit.APIKey is zero value -> triggers error
			},
		},
		{
			name: "production missing APP_PAYMENT_GATEWAY_BASE_URL",
			cfg: &InternalConfig{
				App:            App{Env: "production"},
				JWT:            AppJWT{Secret: "set"},
				Webhook:        AppWebhook{JWTHookKey: "set"},
				PaymentGateway: AppPaymentGateway{Username: "u", ApiKey: "k"},
				Xendit:         AppXendit{APIKey: "set"},
				// PaymentGateway.BaseUrl is zero value -> triggers error
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNonDevConfig(tt.cfg)
			assert.Error(t, err)
			msg := err.Error()
			assert.True(t, isLower(msg),
				"error string should start with a lowercase letter, got: %q", msg)
		})
	}
}

func TestValidateNonDevDriverConfig_errorStringsAreLowercase(t *testing.T) {
	tests := []struct {
		name string
		cfg  *DriverConfig
		env  string
	}{
		{
			name: "production missing REDIS_PASSWORD",
			cfg:  &DriverConfig{
				// Redis.Password is zero value -> "" triggers error
			},
			env: "production",
		},
		{
			name: "production missing RabbitMQ creds",
			cfg: &DriverConfig{
				Redis: Redis{Password: "set"},
				// RabbitMQ.Username and Password are zero value -> triggers error
			},
			env: "production",
		},
		{
			name: "production missing Supertoken API key",
			cfg: &DriverConfig{
				Redis:    Redis{Password: "set"},
				RabbitMQ: RabbitMQ{Username: "u", Password: "p"},
				// Supertoken.APIKey is zero value -> triggers error
			},
			env: "production",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNonDevDriverConfig(tt.cfg, tt.env)
			assert.Error(t, err)
			msg := err.Error()
			assert.True(t, isLower(msg),
				"error string should start with a lowercase letter, got: %q", msg)
		})
	}
}
