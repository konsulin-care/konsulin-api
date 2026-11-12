package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/supertokens/supertokens-golang/ingredients/emaildelivery"
	"github.com/supertokens/supertokens-golang/ingredients/smsdelivery"
	"github.com/supertokens/supertokens-golang/supertokens"
	"go.uber.org/zap"
)

// TestBuildPasswordlessConfig_ContactMethodByGatewayMode verifies T41: the
// passwordless recipe enables email-only contact in standalone mode (gateway
// not configured) and email-or-phone contact in gateway mode.
func TestBuildPasswordlessConfig_ContactMethodByGatewayMode(t *testing.T) {
	standalone := &authUsecase{
		InternalConfig: &config.InternalConfig{},
		Log:            zap.NewNop(),
	}
	cfg := standalone.buildPasswordlessConfig()
	assert.True(t, cfg.ContactMethodEmail.Enabled,
		"standalone mode must enable the email contact method only")
	assert.False(t, cfg.ContactMethodEmailOrPhone.Enabled,
		"standalone mode must not enable email-or-phone contact")

	gatewayMode := &authUsecase{
		InternalConfig: &config.InternalConfig{
			Gateway: config.AppGateway{Enabled: true, URL: "https://gateway.example.test"},
		},
		Log: zap.NewNop(),
	}
	cfg = gatewayMode.buildPasswordlessConfig()
	assert.False(t, cfg.ContactMethodEmail.Enabled,
		"gateway mode must not enable the email-only contact method")
	assert.True(t, cfg.ContactMethodEmailOrPhone.Enabled,
		"gateway mode must keep email-or-phone contact")
}

// TestHandleSupertokensAPIError_WritesJSON500 verifies that recipe API errors
// produce a real 500 JSON response instead of letting net/http emit the
// implicit 200 with an empty body (which breaks the SuperTokens frontend SDK's
// response.json() call).
func TestHandleSupertokensAPIError_WritesJSON500(t *testing.T) {
	rec := httptest.NewRecorder()

	handleSupertokensAPIError(errors.New("magiclink webhook returned status 500"), nil, rec)

	res := rec.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	assert.Equal(t, "application/json", res.Header.Get("Content-Type"))

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	assert.NotEmpty(t, body)

	var payload map[string]string
	require.NoError(t, json.Unmarshal(body, &payload))
	msg, ok := payload["message"]
	assert.True(t, ok, "response body must carry a message in the GeneralErrorResponse shape")
	assert.NotEmpty(t, msg)
}

// TestHandleSupertokensAPIError_NilErrorStillWritesResponse verifies the
// handler is defensive: even a nil error must not leave the response unwritten.
func TestHandleSupertokensAPIError_NilErrorStillWritesResponse(t *testing.T) {
	rec := httptest.NewRecorder()

	handleSupertokensAPIError(nil, nil, rec)

	res := rec.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	assert.NotEmpty(t, body)
}

// stubMagicLinkSender is a controllable contracts.MagicLinkSender for the
// passwordless delivery override tests.
type stubMagicLinkSender struct {
	err error
	fn  func(ctx context.Context, in contracts.SendMagicLinkInput) error
}

// SendMagicLink returns the configured error, or delegates to fn when set.
func (s *stubMagicLinkSender) SendMagicLink(ctx context.Context, in contracts.SendMagicLinkInput) error {
	if s.fn != nil {
		return s.fn(ctx, in)
	}
	return s.err
}

// TestBuildEmailDeliveryConfig_FallsBackToOriginalSenderWhenMagicLinkFails
// verifies contract line: passwordless email delivery falls back to the
// SuperTokens built-in sender when the magic-link sender fails.
func TestBuildEmailDeliveryConfig_FallsBackToOriginalSenderWhenMagicLinkFails(t *testing.T) {
	magicLinkErr := errors.New("magiclink webhook down")
	uc := &authUsecase{
		MagicLinkDelivery: &stubMagicLinkSender{err: magicLinkErr},
		Log:               zap.NewNop(),
	}

	cfg := uc.buildEmailDeliveryConfig()
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Override)

	originalCalled := false
	originalErr := errors.New("supertokens original sender invoked")
	sendEmail := func(input emaildelivery.EmailType, _ supertokens.UserContext) error {
		originalCalled = true
		require.NotNil(t, input.PasswordlessLogin)
		return originalErr
	}
	overridden := cfg.Override(emaildelivery.EmailDeliveryInterface{SendEmail: &sendEmail})
	require.NotNil(t, overridden.SendEmail)

	linkURL := "https://example.com/link"
	sendErr := (*overridden.SendEmail)(emaildelivery.EmailType{
		PasswordlessLogin: &emaildelivery.PasswordlessLoginType{
			Email:           "user@example.com",
			UrlWithLinkCode: &linkURL,
		},
	}, nil)

	assert.True(t, originalCalled, "built-in SuperTokens sender must be used as fallback")
	assert.ErrorIs(t, sendErr, originalErr, "fallback sender result must be returned")
}

// TestBuildSMSDeliveryConfig_ReturnsErrorWhenMagicLinkFails verifies contract
// line: passwordless SMS delivery returns an error when the magic-link sender
// fails, with no fallback to the original sender.
func TestBuildSMSDeliveryConfig_ReturnsErrorWhenMagicLinkFails(t *testing.T) {
	magicLinkErr := errors.New("magiclink webhook down")
	uc := &authUsecase{
		MagicLinkDelivery: &stubMagicLinkSender{err: magicLinkErr},
		Log:               zap.NewNop(),
	}

	cfg := uc.buildSMSDeliveryConfig()
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Override)

	originalCalled := false
	sendSms := func(_ smsdelivery.SmsType, _ supertokens.UserContext) error {
		originalCalled = true
		return nil
	}
	overridden := cfg.Override(smsdelivery.SmsDeliveryInterface{SendSms: &sendSms})
	require.NotNil(t, overridden.SendSms)

	linkURL := "https://example.com/link"
	sendErr := (*overridden.SendSms)(smsdelivery.SmsType{
		PasswordlessLogin: &smsdelivery.PasswordlessLoginType{
			PhoneNumber:     "6281234567890",
			UrlWithLinkCode: &linkURL,
		},
	}, nil)

	assert.ErrorIs(t, sendErr, magicLinkErr, "SMS delivery must surface the magic-link sender error")
	assert.False(t, originalCalled, "SMS delivery must not fall back to the original sender")
}

// TestPasswordlessDelivery_UsesFixedTenSecondTimeout verifies that both email
// and SMS delivery build their context with a fixed 10s timeout, ignoring
// InternalConfig.Webhook.HTTPTimeoutInSeconds (set to 1 here on purpose).
func TestPasswordlessDelivery_UsesFixedTenSecondTimeout(t *testing.T) {
	var emailDeadline, smsDeadline time.Duration
	uc := &authUsecase{
		MagicLinkDelivery: &stubMagicLinkSender{fn: func(ctx context.Context, _ contracts.SendMagicLinkInput) error {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Error("delivery context must carry a deadline")
				return nil
			}
			remaining := time.Until(deadline)
			if emailDeadline == 0 {
				emailDeadline = remaining
			} else {
				smsDeadline = remaining
			}
			return nil
		}},
		InternalConfig: &config.InternalConfig{Webhook: config.AppWebhook{HTTPTimeoutInSeconds: 1}},
		Log:            zap.NewNop(),
	}

	linkURL := "https://example.com/link"

	emailSend := func(_ emaildelivery.EmailType, _ supertokens.UserContext) error { return nil }
	emailCfg := uc.buildEmailDeliveryConfig()
	overriddenEmail := emailCfg.Override(emaildelivery.EmailDeliveryInterface{SendEmail: &emailSend})
	require.NoError(t, (*overriddenEmail.SendEmail)(emaildelivery.EmailType{
		PasswordlessLogin: &emaildelivery.PasswordlessLoginType{
			Email:           "user@example.com",
			UrlWithLinkCode: &linkURL,
		},
	}, nil))

	smsSend := func(_ smsdelivery.SmsType, _ supertokens.UserContext) error { return nil }
	smsCfg := uc.buildSMSDeliveryConfig()
	overriddenSms := smsCfg.Override(smsdelivery.SmsDeliveryInterface{SendSms: &smsSend})
	require.NoError(t, (*overriddenSms.SendSms)(smsdelivery.SmsType{
		PasswordlessLogin: &smsdelivery.PasswordlessLoginType{
			PhoneNumber:     "6281234567890",
			UrlWithLinkCode: &linkURL,
		},
	}, nil))

	assert.InDelta(t, float64(10*time.Second), float64(emailDeadline), float64(2*time.Second),
		"email delivery must use a fixed 10s timeout")
	assert.InDelta(t, float64(10*time.Second), float64(smsDeadline), float64(2*time.Second),
		"SMS delivery must use a fixed 10s timeout")
}
