package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/konsulin-care/konsulin-api/internal/app/config"
	"github.com/konsulin-care/konsulin-api/internal/app/contracts"
)

// gatewayMagicLinkListing is the gateway listing served by the automation
// party that carries the magic link to the end user.
const gatewayMagicLinkListing = "send-magiclink"

// GatewayWebhookClient is the gateway surface the magic-link sender needs:
// the webhook enqueue route (contracts.WebhookClient) plus the adapter's
// enabled status. The gateway driver's *gateway.Client implements it.
type GatewayWebhookClient interface {
	contracts.WebhookClient
	// Enabled reports whether the gateway adapter is configured (a
	// non-empty APP_GATEWAY_URL).
	Enabled() bool
}

// gatewayMagicLinkSender delivers passwordless magic links by enqueueing
// the send-magiclink listing on the automation party through the gateway
// webhook client. It replaces the old in-process webhook forwarder at
// bootstrap time.
type gatewayMagicLinkSender struct {
	client  GatewayWebhookClient
	gateway config.AppGateway
}

// NewGatewayMagicLinkSender constructs the gateway-backed magic-link
// sender.
// Parameters: client is the gateway webhook client built by newGateway
// (nil is treated as a disabled gateway); gatewayCfg carries
// APP_AUTOMATION_PARTY_ID.
// Returns a contracts.MagicLinkSender that fails closed with
// contracts.ErrMagicLinkUnavailable while the gateway is disabled or the
// automation party is unset. Never panics.
func NewGatewayMagicLinkSender(client GatewayWebhookClient, gatewayCfg config.AppGateway) contracts.MagicLinkSender {
	return &gatewayMagicLinkSender{client: client, gateway: gatewayCfg}
}

// SendMagicLink enqueues the link on the automation party's send-magiclink
// listing with {url, email} or {url, phoneNumber}. It returns
// contracts.ErrMagicLinkUnavailable without enqueueing when the gateway is
// disabled or APP_AUTOMATION_PARTY_ID is unset; the input is validated
// (URL required, exactly one of email/phone) before any enqueue.
// Parameters: ctx bounds the enqueue; in carries the link and destination.
// Returns nil once the gateway accepts the enqueue, or the error. Never
// panics.
func (s *gatewayMagicLinkSender) SendMagicLink(ctx context.Context, in contracts.SendMagicLinkInput) error {
	automation := strings.TrimSpace(s.gateway.AutomationPartyID)
	if s.client == nil || !s.client.Enabled() || automation == "" {
		return contracts.ErrMagicLinkUnavailable
	}
	if err := validateGatewayMagicLinkInput(in); err != nil {
		return err
	}

	payload := struct {
		URL   string `json:"url"`
		Email string `json:"email,omitempty"`
		Phone string `json:"phoneNumber,omitempty"`
	}{
		URL:   strings.TrimSpace(in.URL),
		Email: strings.TrimSpace(in.Email),
		Phone: strings.TrimSpace(in.Phone),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return errors.New("marshal magiclink payload failed")
	}
	if err := s.client.Enqueue(ctx, automation, gatewayMagicLinkListing, "", data); err != nil {
		return err
	}
	return nil
}

// validateGatewayMagicLinkInput enforces the SendMagicLinkInput invariants:
// a non-empty URL and exactly one destination (email or phone).
// Parameters: in is the input to check.
// Returns nil when valid, or an error naming the violated invariant.
func validateGatewayMagicLinkInput(in contracts.SendMagicLinkInput) error {
	if strings.TrimSpace(in.URL) == "" {
		return errors.New("url is required")
	}
	hasEmail := strings.TrimSpace(in.Email) != ""
	hasPhone := strings.TrimSpace(in.Phone) != ""
	if hasEmail && hasPhone {
		return errors.New("email and phone are mutually exclusive")
	}
	if !hasEmail && !hasPhone {
		return errors.New("either email or phone is required")
	}
	return nil
}
