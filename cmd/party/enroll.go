package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/drivers/gateway"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/utils"
)

// enrollTimeout bounds one enroll call against the gateway.
const enrollTimeout = 30 * time.Second

// enroll redeems the single-use APP_PARTY_ENROLLMENT_TOKEN once through the
// adapter and prints one success line naming the party id. Keys and the
// token are never printed.
// Parameters: args are the CLI arguments after "enroll" (none accepted);
// stdout receives the success line; stderr is unused.
// Returns: nil on success, otherwise the config or mapped gateway error.
func enroll(args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("party enroll: unexpected argument %q", args[0])
	}
	token := strings.TrimSpace(utils.GetEnvString("APP_PARTY_ENROLLMENT_TOKEN", ""))
	if token == "" {
		return errors.New("party enroll: APP_PARTY_ENROLLMENT_TOKEN is required")
	}
	gwURL := strings.TrimSpace(utils.GetEnvString("APP_GATEWAY_URL", ""))
	partyID := strings.TrimSpace(utils.GetEnvString("APP_PARTY_ID", ""))
	env := utils.GetEnvString("APP_ENV", constvars.EnvLocal)

	adapter, err := gateway.New(config.AppGateway{
		Enabled:       gwURL != "",
		URL:           gwURL,
		PartyID:       partyID,
		PrivateKey:    strings.TrimSpace(utils.GetEnvString("APP_PARTY_PRIVATE_KEY", "")),
		EncPrivateKey: strings.TrimSpace(utils.GetEnvString("APP_PARTY_ENC_PRIVATE_KEY", "")),
	}, env, nil)
	if err != nil {
		return fmt.Errorf("party enroll: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), enrollTimeout)
	defer cancel()
	if err := adapter.Enroll(ctx, token); err != nil {
		return gateway.MapError(err)
	}
	if _, err := fmt.Fprintf(stdout, "party %s enrolled\n", partyID); err != nil {
		return fmt.Errorf("party enroll: write output: %w", err)
	}
	return nil
}
