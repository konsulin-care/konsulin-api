package contracts

import "context"

// SendMagicLinkInput is the payload used by internal magic-link delivery.
// Exactly one of Email or Phone must be provided.
type SendMagicLinkInput struct {
	// URL is the magic link URL (required).
	URL string

	// Email is the destination email address. Mutually exclusive with Phone.
	Email string

	// Phone is the WhatsApp phone number without '+' prefix. Mutually exclusive with Email.
	Phone string
}

// MagicLinkDeliveryService sends passwordless magic links via the internal webhook service.
//
// SECURITY NOTE:
// This service is meant to be used internally by backend components (e.g. SuperTokens delivery overrides).
// Do NOT expose this as a public HTTP endpoint unless there is an explicit feature request and proper
// authorization, rate limiting, and abuse prevention are implemented. This is a sensitive capability
// that can be used to spam users if exposed.
type MagicLinkDeliveryService interface {
	SendMagicLink(ctx context.Context, in SendMagicLinkInput) error
}
