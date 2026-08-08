package webhook

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/services/shared/jwtmanager"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// generateTestECKey PEM-encodes a throwaway ES256 private key.
func generateTestECKey() string {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(fmt.Sprintf("generate EC key: %v", err))
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		panic(fmt.Sprintf("marshal EC key: %v", err))
	}
	block := &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: der,
	}
	return string(pem.EncodeToMemory(block))
}

// newTestUsecase returns a usecase with a functional JWTManager (ES256) and minimal config.
func newTestUsecase() *usecase {
	logger := zap.NewNop()
	key := generateTestECKey()
	cfg := &config.InternalConfig{
		App: config.App{Env: "test"},
		Webhook: config.AppWebhook{
			JWTAlg:     "ES256",
			JWTHookKey: key,
		},
	}
	jwtMgr, err := jwtmanager.NewJWTManager(cfg, logger)
	if err != nil {
		panic("NewJWTManager: " + err.Error())
	}
	return &usecase{
		log:        logger,
		cfg:        cfg,
		jwtManager: jwtMgr,
		httpClient: &http.Client{Timeout: time.Second},
	}
}

func TestForwardSynchronousInternal_DelegatesToForwardSynchronous(t *testing.T) {
	u := newTestUsecase()
	// Webhook.URL is empty -> forwardSynchronous builds a relative URL that fails
	out, err := u.ForwardSynchronousInternal(
		context.Background(), "test-svc", "POST", []byte(`{}`), "application/json",
	)
	require.Error(t, err, "should fail with empty Webhook.URL")
	assert.Nil(t, out)
}

func TestForwardSynchronousInternal_UnreachableURL(t *testing.T) {
	u := newTestUsecase()
	u.cfg.Webhook.URL = "http://127.0.0.1:1" // unreachable
	out, err := u.ForwardSynchronousInternal(
		context.Background(), "test-svc", "POST", []byte(`{}`), "application/json",
	)
	require.Error(t, err, "should fail when URL is unreachable")
	assert.Nil(t, out)
}

func TestForwardSynchronousInternal_ContextCancelled(t *testing.T) {
	u := newTestUsecase()
	u.cfg.Webhook.URL = "http://127.0.0.1:1"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err := u.ForwardSynchronousInternal(
		ctx, "test-svc", "POST", []byte(`{}`), "application/json",
	)
	require.Error(t, err, "should propagate cancellation")
	assert.Nil(t, out)
}

// TestForwardSynchronousInternal_EmptyInput: the same input validation as the
// HTTP path must reject an empty/whitespace service name before any transport
// attempt. NO allowlist gate is imposed on this internal path.
func TestForwardSynchronousInternal_EmptyInput(t *testing.T) {
	u := newTestUsecase()
	out, err := u.ForwardSynchronousInternal(
		context.Background(), "   ", "POST", []byte(`{}`), "application/json",
	)
	require.Error(t, err, "empty service must be rejected by input validation")
	assert.Nil(t, out)

	var ce *exceptions.CustomError
	require.True(t, errors.As(err, &ce), "err must be a CustomError")
	assert.Equal(t, constvars.StatusBadRequest, ce.StatusCode)
	assert.True(t, strings.Contains(ce.DevMessage, constvars.ErrDevValidationFailed),
		"validation marker expected, got: %s", ce.DevMessage)
}

// TestForwardSynchronousInternal_FailurePolicyReturnError: with the default
// return_error failure policy, an unreachable upstream surfaces the same
// failure-policy error as the HTTP synchronous path. The enqueue_request
// branch is not exercised here because it requires a queue stub.
func TestForwardSynchronousInternal_FailurePolicyReturnError(t *testing.T) {
	u := newTestUsecase()
	u.cfg.Webhook.URL = "http://127.0.0.1:1" // unreachable upstream
	u.failurePolicy = SyncFailurePolicyReturnError

	out, err := u.ForwardSynchronousInternal(
		context.Background(), "test-svc", "POST", []byte(`{}`), "application/json",
	)
	require.Error(t, err, "upstream failure must surface an error under return_error")
	assert.Nil(t, out)

	var ce *exceptions.CustomError
	require.True(t, errors.As(err, &ce), "custom must be a CustomError")
	assert.True(t, strings.Contains(ce.DevMessage, "WEBHOOK_SYNC_FAILURE_POLICY_RETURN_ERROR"),
		"failure policy error marker expected, got: %s", ce.DevMessage)
}
