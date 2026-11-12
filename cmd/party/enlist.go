package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/drivers/gateway"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/utils"
)

// enlistTimeout bounds one enlist call against the gateway.
const enlistTimeout = 30 * time.Second

// enlist submits one marketplace listing through the adapter's
// ListingDirectory Enlist (BK-MARKET-001): it reads the gateway URL, party
// id and current signing key from the environment, forwards the flag
// values as the listing to create, and writes listing_id=<id> to stdout on
// success. Any gateway rejection (e.g. a duplicate name, HTTP 409) returns
// the mapped error so the CLI exits non-zero with the gateway's message.
// Parameters: args are the CLI arguments after "enlist"; stdout and stderr
// are the output sinks (stdout receives the listing id only).
// Returns: nil once the listing id is written, otherwise the flag, config
// or mapped gateway error. Never panics.
func enlist(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("enlist", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "listing title shown to buyers")
	description := fs.String("description", "", "optional marketplace detail")
	endpoint := fs.String("endpoint", "", "URL where the buyer reaches the provider")
	price := fs.Int64("price", 0, "amount in IDR minor units; 0 marks a free listing")
	invocation := fs.String("invocation", "", `invocation mode: "webhook" or "commodity"`)
	priceScope := fs.String("price-scope", "", `price scope: "party" or "user"`)
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("party enlist: %w", err)
	}
	if strings.TrimSpace(*name) == "" {
		return errors.New("party enlist: --name is required")
	}
	if strings.TrimSpace(*endpoint) == "" {
		return errors.New("party enlist: --endpoint is required")
	}

	gwURL := strings.TrimSpace(utils.GetEnvString("APP_GATEWAY_URL", ""))
	partyID := strings.TrimSpace(utils.GetEnvString("APP_PARTY_ID", ""))
	currentKey := strings.TrimSpace(utils.GetEnvString("APP_PARTY_PRIVATE_KEY", ""))
	env := utils.GetEnvString("APP_ENV", constvars.EnvLocal)

	adapter, err := gateway.New(config.AppGateway{
		Enabled:    gwURL != "",
		URL:        gwURL,
		PartyID:    partyID,
		PrivateKey: currentKey,
	}, env, nil)
	if err != nil {
		return fmt.Errorf("party enlist: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), enlistTimeout)
	defer cancel()
	listingID, err := adapter.Enlist(ctx, contracts.EnlistRequest{
		Name:        strings.TrimSpace(*name),
		Description: strings.TrimSpace(*description),
		EndpointURL: strings.TrimSpace(*endpoint),
		Price:       *price,
		Invocation:  strings.TrimSpace(*invocation),
		PriceScope:  strings.TrimSpace(*priceScope),
	})
	if err != nil {
		return gateway.MapError(err)
	}
	if _, err := fmt.Fprintf(stdout, "listing_id=%s\n", listingID); err != nil {
		return fmt.Errorf("party enlist: write output: %w", err)
	}
	return nil
}
