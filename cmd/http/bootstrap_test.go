package main

import (
	"strings"
	"testing"

	"konsulin-service/internal/app/config"

	"go.uber.org/zap"
)

// TestBootstrapingTheAppStopsWhenJWTManagerFails covers the client wiring that runs
// before the first fallible dependency, without reaching Redis, FHIR or SuperTokens.
func TestBootstrapingTheAppStopsWhenJWTManagerFails(t *testing.T) {
	err := bootstrapingTheApp(&config.Bootstrap{
		InternalConfig: &config.InternalConfig{Xendit: config.AppXendit{APIKey: "synthetic-test-key", BaseURL: "http://127.0.0.1:1"}},
		DriverConfig:   &config.DriverConfig{},
		Logger:         zap.NewNop(),
	})
	if err == nil || !strings.Contains(err.Error(), "JWT_HOOK_KEY is empty") {
		t.Fatalf("bootstrap error = %v, want missing JWT_HOOK_KEY", err)
	}
}
