package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"konsulin-service/internal/app/services/shared/jwtmanager"
	"konsulin-service/internal/pkg/constvars"
)

const (
	// magicLinkWebhookURL is intentionally hardcoded for now per requirements.
	// Make this configurable later via internal config / env.
	magicLinkWebhookURL = "https://flow.konsulin.care/webhook/staging/send-magiclink"

	// magicLinkExpMinutes is intentionally arbitrary. The *actual* magic-link expiry is controlled externally.
	magicLinkExpMinutes = 15
)

type SendMagicLinkInput struct {
	// URL is the magic link URL (required).
	URL string

	// Email is the destination email address. Mutually exclusive with Phone.
	Email string

	// Phone is the WhatsApp phone number without '+' prefix. Mutually exclusive with Email.
	// For now, accept any given phone number (no normalization/validation).
	Phone string
}

func (in SendMagicLinkInput) validate() error {
	url := strings.TrimSpace(in.URL)
	email := strings.TrimSpace(in.Email)
	phone := strings.TrimSpace(in.Phone)

	if url == "" {
		return fmt.Errorf("url is required")
	}

	hasEmail := email != ""
	hasPhone := phone != ""

	if hasEmail && hasPhone {
		return fmt.Errorf("email and phone are mutually exclusive")
	}
	if !hasEmail && !hasPhone {
		return fmt.Errorf("either email or phone is required")
	}

	return nil
}

// SendMagicLink calls the external webhook service that sends passwordless magic links.
// It attaches Authorization: Bearer <JWT> similarly to other webhook forwarders.
// Success is indicated by HTTP 204.
func SendMagicLink(ctx context.Context, jwt *jwtmanager.JWTManager, input SendMagicLinkInput) error {
	if err := input.validate(); err != nil {
		return err
	}
	if jwt == nil {
		return fmt.Errorf("jwt manager is required")
	}

	payload := struct {
		URL   string `json:"url"`
		Exp   int    `json:"exp"`
		Email string `json:"email,omitempty"`
		Phone string `json:"phone,omitempty"`
	}{
		URL:   strings.TrimSpace(input.URL),
		Exp:   magicLinkExpMinutes,
		Email: strings.TrimSpace(input.Email),
		Phone: strings.TrimSpace(input.Phone),
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, magicLinkWebhookURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationJSON)

	// Match existing webhook behavior: subject identifies the target service.
	tokenOut, err := jwt.CreateToken(ctx, &jwtmanager.CreateTokenInput{Subject: "send-magiclink"})
	if err != nil {
		return err
	}
	req.Header.Set(constvars.HeaderAuthorization, "Bearer "+tokenOut.Token)

	client := &http.Client{
		// We still rely on ctx timeout; this is just a safety net.
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("call magiclink webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	// Best-effort error body capture (bounded).
	const maxBody = 4096
	b, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if len(b) == 0 {
		return fmt.Errorf("magiclink webhook returned status %d", resp.StatusCode)
	}
	return fmt.Errorf("magiclink webhook returned status %d: %s", resp.StatusCode, string(b))
}
