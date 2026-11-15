package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/drivers/gateway"
)

const onlineTimeout = 15 * time.Second

// verifyOnline checks that the party's key-derived kid is enrolled at the gateway.
// Parameters: values is the parsed env file; stdout is the finding sink.
// Returns: the number of FAIL findings printed, or a write error.
func verifyOnline(values map[string]string, stdout io.Writer) (int, error) {
	cfg := config.AppGateway{
		Enabled:       true,
		URL:           strings.TrimSpace(values["APP_GATEWAY_URL"]),
		PartyID:       strings.TrimSpace(values["APP_PARTY_ID"]),
		PrivateKey:    values["APP_PARTY_PRIVATE_KEY"],
		EncPrivateKey: values["APP_PARTY_ENC_PRIVATE_KEY"],
	}
	line := func(level, v, msg string) error {
		_, err := fmt.Fprintf(stdout, "%s %s: %s\n", level, v, msg)
		return err
	}
	adapter, err := gateway.New(cfg, values["APP_ENV"], nil)
	if err != nil {
		return 1, line("FAIL", "APP_GATEWAY_URL", "cannot build gateway client for the online check")
	}
	ctx, cancel := context.WithTimeout(context.Background(), onlineTimeout)
	defer cancel()
	ok, err := adapter.IsEnrolled(ctx)
	if err != nil {
		return 1, line("FAIL", "APP_GATEWAY_URL", "gateway unreachable during the online check")
	}
	if !ok {
		return 1, line("FAIL", "APP_PARTY_PRIVATE_KEY", "JWKS kid mismatch: the party JWKS at the gateway does not contain the kid derived from this key")
	}
	if strings.TrimSpace(values["APP_PARTY_ENROLLMENT_TOKEN"]) != "" {
		return 0, line("WARN", "APP_PARTY_ENROLLMENT_TOKEN", "party is enrolled; the single-use token should be removed")
	}
	return 0, nil
}
