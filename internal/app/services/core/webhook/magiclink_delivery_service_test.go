package webhook

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/shared/jwtmanager"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// defaultTestConfig returns the base config used by most tests.
func defaultTestConfig() *config.InternalConfig {
	key := generateTestECKey()
	return &config.InternalConfig{
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
}

// newTestMagicLinkDelivery creates a magicLinkDeliveryService with a given forwardFn.
func newTestMagicLinkDelivery(forwardFn func(ctx context.Context, service, method string, body []byte, contentType string) (int, []byte, error)) *magicLinkDeliveryService {
	return newTestMagicLinkDeliveryWithConfig(defaultTestConfig(), forwardFn)
}

// newTestMagicLinkDeliveryWithConfig creates a magicLinkDeliveryService from a custom config.
func newTestMagicLinkDeliveryWithConfig(cfg *config.InternalConfig, forwardFn func(ctx context.Context, service, method string, body []byte, contentType string) (int, []byte, error)) *magicLinkDeliveryService {
	logger := zap.NewNop()
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

// TestMagicLinkDelivery_ForwardFn_5xx_TreatedAsDelivered verifies that a 5xx
// response from the webhook after dispatch is not a login failure: the webhook
// received the request and dispatched the message, so SendMagicLink logs and
// returns nil instead of failing createCode.
func TestMagicLinkDelivery_ForwardFn_5xx_TreatedAsDelivered(t *testing.T) {
	forwardFn := func(ctx context.Context, service, method string, body []byte, contentType string) (int, []byte, error) {
		return http.StatusInternalServerError, []byte("upstream exploded after dispatch"), nil
	}
	s := newTestMagicLinkDelivery(forwardFn)

	err := s.SendMagicLink(context.Background(), contracts.SendMagicLinkInput{
		URL:   "https://example.com/magic-link",
		Email: "user@test.com",
	})
	assert.NoError(t, err)
}

// TestMagicLinkDelivery_ForwardFn_4xx_ReturnsError verifies that a 4xx response
// is a definitive non-dispatch (misconfig/payload bug) and must stay loud.
func TestMagicLinkDelivery_ForwardFn_4xx_ReturnsError(t *testing.T) {
	forwardFn := func(ctx context.Context, service, method string, body []byte, contentType string) (int, []byte, error) {
		return http.StatusBadRequest, []byte("bad payload"), nil
	}
	s := newTestMagicLinkDelivery(forwardFn)

	err := s.SendMagicLink(context.Background(), contracts.SendMagicLinkInput{
		URL:   "https://example.com/magic-link",
		Email: "user@test.com",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

// TestMagicLinkDelivery_HTTP_5xx_TreatedAsDelivered verifies the HTTP path
// (forwardFn nil) applies the same post-dispatch classification: a 5xx response
// is logged but does not fail delivery.
func TestMagicLinkDelivery_HTTP_5xx_TreatedAsDelivered(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := defaultTestConfig()
	cfg.App.BaseUrl = srv.URL
	s := newTestMagicLinkDeliveryWithConfig(cfg, nil)

	err := s.SendMagicLink(context.Background(), contracts.SendMagicLinkInput{
		URL:   "https://example.com/magic-link",
		Email: "user@test.com",
	})
	assert.NoError(t, err)
}

// TestMagicLinkDelivery_HTTP_4xx_ReturnsError verifies the HTTP path keeps 4xx
// responses as hard errors.
func TestMagicLinkDelivery_HTTP_4xx_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	cfg := defaultTestConfig()
	cfg.App.BaseUrl = srv.URL
	s := newTestMagicLinkDeliveryWithConfig(cfg, nil)

	err := s.SendMagicLink(context.Background(), contracts.SendMagicLinkInput{
		URL:   "https://example.com/magic-link",
		Email: "user@test.com",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "400")
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
