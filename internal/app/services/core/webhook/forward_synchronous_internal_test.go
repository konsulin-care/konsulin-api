package webhook

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"testing"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/services/shared/jwtmanager"

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
