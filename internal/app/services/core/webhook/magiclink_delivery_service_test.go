package webhook

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/shared/jwtmanager"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// newTestMagicLinkDelivery creates a magicLinkDeliveryService with a given forwardFn.
func newTestMagicLinkDelivery(forwardFn func(ctx context.Context, service, method string, body []byte, contentType string) (int, []byte, error)) *magicLinkDeliveryService {
	logger := zap.NewNop()
	key := generateTestECKey()
	cfg := &config.InternalConfig{
		App: config.App{
			Env:                        "test",
			BaseUrl:                    "http://localhost:3200",
			WebhookInstantiateBasePath: "/api/v1/hook",
		},
		Webhook: config.AppWebhook{
			JWTAlg:              "ES256",
			JWTHookKey:          key,
			URL:                 "http://localhost:9999", // not used when forwardFn is set
			HTTPTimeoutInSeconds: 5,
		},
	}
	jwtMgr, err := jwtmanager.NewJWTManager(cfg, logger)
	if err != nil {
		panic("NewJWTManager: " + err.Error())
	}
	s := &magicLinkDeliveryService{
		log:        logger,
		cfg:        cfg,
		jwtManager: jwtMgr,
		httpClient: &http.Client{},
		forwardFn:  forwardFn,
	}
	return s
}

func TestMagicLinkDelivery_ForwardFnUsed_WhenSet(t *testing.T) {
	var recordedService, recordedMethod string
	var recordedBody []byte
	var recordedContentType string

	forwardFn := func(ctx context.Context, service, method string, body []byte, contentType string) (int, []byte, error) {
		recordedService = service
		recordedMethod = method
		recordedBody = body
		recordedContentType = contentType
		return http.StatusOK, []byte("ok"), nil
	}

	s := newTestMagicLinkDelivery(forwardFn)

	err := s.SendMagicLink(context.Background(), contracts.SendMagicLinkInput{
		URL:   "https://example.com/magic-link",
		Email: "user@test.com",
	})
	assert.NoError(t, err)
	assert.Equal(t, "send-magiclink", recordedService)
	assert.Equal(t, http.MethodPost, recordedMethod)
	assert.Equal(t, "application/json", recordedContentType)
	assert.Contains(t, string(recordedBody), "https://example.com/magic-link")
	assert.Contains(t, string(recordedBody), "user@test.com")
}

func TestMagicLinkDelivery_ForwardFn_ReturnsError_WhenForwarderFails(t *testing.T) {
	forwardFn := func(ctx context.Context, service, method string, body []byte, contentType string) (int, []byte, error) {
		return 0, nil, errors.New("forwarder error")
	}
	s := newTestMagicLinkDelivery(forwardFn)

	err := s.SendMagicLink(context.Background(), contracts.SendMagicLinkInput{
		URL:   "https://example.com/magic-link",
		Email: "user@test.com",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forward magiclink")
}

func TestMagicLinkDelivery_ForwardFn_ReturnsError_OnNonOKStatusCode(t *testing.T) {
	forwardFn := func(ctx context.Context, service, method string, body []byte, contentType string) (int, []byte, error) {
		return http.StatusForbidden, []byte("Forbidden"), nil
	}
	s := newTestMagicLinkDelivery(forwardFn)

	err := s.SendMagicLink(context.Background(), contracts.SendMagicLinkInput{
		URL:   "https://example.com/magic-link",
		Email: "user@test.com",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestMagicLinkDelivery_ForwardFn_Empty_DefaultsToHTTP(t *testing.T) {
	// When forwardFn is nil, SendMagicLink should fall back to HTTP.
	// We can't test the full HTTP path here, but we can verify it doesn't panic.
	s := newTestMagicLinkDelivery(nil) // no forwardFn
	err := s.SendMagicLink(context.Background(), contracts.SendMagicLinkInput{
		URL:   "https://example.com/magic-link",
		Email: "user@test.com",
	})
	// Should fail trying to connect to localhost:3200 (nothing listening)
	assert.Error(t, err)
}
