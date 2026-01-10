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

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/shared/jwtmanager"
	"konsulin-service/internal/pkg/constvars"

	"go.uber.org/zap"
)

const (
	// magicLinkServiceName is appended to InternalConfig.Webhook.URL.
	// With HOOK_URL="https://flow.konsulin.care/webhook/staging", this becomes:
	//   https://flow.konsulin.care/webhook/staging/send-magiclink
	magicLinkServiceName = "send-magiclink"

	// magicLinkExpMinutes is intentionally arbitrary. The *actual* magic-link expiry is controlled externally.
	magicLinkExpMinutes = 15
)

type magicLinkDeliveryService struct {
	log        *zap.Logger
	cfg        *config.InternalConfig
	jwtManager *jwtmanager.JWTManager
	httpClient *http.Client
}

// NewMagicLinkDeliveryService constructs an internal-only delivery service for passwordless magic links.
// It is NOT exposed as an HTTP endpoint; intended usage is via internal components like SuperTokens overrides.
func NewMagicLinkDeliveryService(cfg *config.InternalConfig, jwtManager *jwtmanager.JWTManager, logger *zap.Logger) contracts.MagicLinkDeliveryService {
	timeoutSeconds := 15
	if cfg != nil && cfg.Webhook.HTTPTimeoutInSeconds > 0 {
		timeoutSeconds = cfg.Webhook.HTTPTimeoutInSeconds
	}

	return &magicLinkDeliveryService{
		cfg:        cfg,
		log:        logger,
		jwtManager: jwtManager,
		httpClient: &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
	}
}

func (s *magicLinkDeliveryService) SendMagicLink(ctx context.Context, in contracts.SendMagicLinkInput) error {
	if s.cfg == nil {
		return fmt.Errorf("internal config is required")
	}
	if strings.TrimSpace(s.cfg.Webhook.URL) == "" {
		return fmt.Errorf("webhook base url (InternalConfig.Webhook.URL) is required")
	}
	if s.jwtManager == nil {
		return fmt.Errorf("jwt manager is required")
	}

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

	targetURL := fmt.Sprintf("%s/%s", strings.TrimRight(s.cfg.Webhook.URL, "/"), magicLinkServiceName)

	payload := struct {
		URL   string `json:"url"`
		Exp   int    `json:"exp"`
		Email string `json:"email,omitempty"`
		Phone string `json:"phone,omitempty"`
	}{
		URL:   url,
		Exp:   magicLinkExpMinutes,
		Email: email,
		Phone: phone,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationJSON)

	tokenOut, err := s.jwtManager.CreateToken(ctx, &jwtmanager.CreateTokenInput{Subject: magicLinkServiceName})
	if err != nil {
		return err
	}

	req.Header.Set(constvars.HeaderAuthorization, "Bearer "+tokenOut.Token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call magiclink webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
		s.log.Info("magiclink webhook sent successfully",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil
	}

	// Best-effort error body capture (bounded).
	const maxBody = 4096
	b, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if len(b) == 0 {
		s.log.Error("magiclink webhook returned status %d", zap.Int("status_code", resp.StatusCode))
		return fmt.Errorf("magiclink webhook returned status %d", resp.StatusCode)
	}

	s.log.Error("magiclink webhook returned status %d: %s", zap.Int("status_code", resp.StatusCode), zap.String("body", string(b)))
	return fmt.Errorf("magiclink webhook returned status %d: %s", resp.StatusCode, string(b))
}
