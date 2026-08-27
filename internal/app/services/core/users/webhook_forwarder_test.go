package users

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"konsulin-service/internal/app/config"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func newTestUsecaseWithForwarder(forwardFn func(ctx context.Context, service, method string, body []byte, contentType string) (int, []byte, error)) *userUsecase {
	return &userUsecase{
		Log: zap.NewNop(),
		InternalConfig: &config.InternalConfig{
			App: config.App{
				BaseUrl:                    "http://localhost:3200",
				WebhookInstantiateBasePath: "/api/v1/hook",
			},
			Webhook: config.AppWebhook{
				HTTPTimeoutInSeconds: 5,
			},
		},
		webhookForwardFn: forwardFn,
	}
}

func TestUserFHIRInitializer_WebhookForwardFn_Called_WhenSet(t *testing.T) {
	var recordedService, recordedMethod string
	var recordedContentType string

	forwardFn := func(_ context.Context, service, method string, _ []byte, contentType string) (int, []byte, error) {
		recordedService = service
		recordedMethod = method
		recordedContentType = contentType

		raw := []callWebhookSvcKonsulinOmnichannelRawOutput{
			{ChatwootID: 42, Email: "test@test.com", PhoneNumber: strPtr("+6281234567890")},
		}
		respBytes, _ := json.Marshal(raw)
		return http.StatusOK, respBytes, nil
	}

	uc := newTestUsecaseWithForwarder(forwardFn)

	result, err := uc.callWebhookSvcKonsulinOmnichannel(context.Background(), callWebhookSvcKonsulinOmnichannelInput{
		Email: "test@test.com",
	})
	assert.NoError(t, err)
	assert.Equal(t, 42, result.ChatwootID)
	assert.Equal(t, "test@test.com", result.Email)
	assert.Equal(t, "+6281234567890", result.PhoneNumber)

	assert.Equal(t, "modify-profile", recordedService)
	assert.Equal(t, http.MethodPost, recordedMethod)
	assert.Equal(t, "application/json", recordedContentType)
}

func TestUserFHIRInitializer_WebhookForwardFn_Error_Propagates(t *testing.T) {
	forwardFn := func(_ context.Context, _, _ string, _ []byte, _ string) (int, []byte, error) {
		return 0, nil, errors.New("forwarder error")
	}

	uc := newTestUsecaseWithForwarder(forwardFn)
	_, err := uc.callWebhookSvcKonsulinOmnichannel(context.Background(), callWebhookSvcKonsulinOmnichannelInput{
		Email: "test@test.com",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forwarder error")
}

func TestUserFHIRInitializer_WebhookForwardFn_NonOKStatus_ReturnsError(t *testing.T) {
	forwardFn := func(_ context.Context, _, _ string, _ []byte, _ string) (int, []byte, error) {
		return http.StatusForbidden, []byte("Forbidden"), nil
	}

	uc := newTestUsecaseWithForwarder(forwardFn)
	_, err := uc.callWebhookSvcKonsulinOmnichannel(context.Background(), callWebhookSvcKonsulinOmnichannelInput{
		Email: "test@test.com",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to call webhook svc konsulin omnichannel")
}

func TestUserFHIRInitializer_WebhookForwardFn_EmptyResponse_ReturnsError(t *testing.T) {
	forwardFn := func(_ context.Context, _, _ string, _ []byte, _ string) (int, []byte, error) {
		return http.StatusOK, []byte("[]"), nil
	}

	uc := newTestUsecaseWithForwarder(forwardFn)
	_, err := uc.callWebhookSvcKonsulinOmnichannel(context.Background(), callWebhookSvcKonsulinOmnichannelInput{
		Email: "test@test.com",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty response")
}

func strPtr(s string) *string {
	return &s
}

func TestUserFHIRInitializer_SetWebhookForwarder_SetsField(t *testing.T) {
	uc := &userUsecase{Log: zap.NewNop()}
	called := false
	fn := func(_ context.Context, _, _ string, _ []byte, _ string) (int, []byte, error) {
		called = true
		return http.StatusOK, nil, nil
	}
	uc.SetWebhookForwarder(fn)
	assert.NotNil(t, uc.webhookForwardFn)

	// Verify it works
	uc.webhookForwardFn(context.Background(), "svc", "POST", nil, "ct")
	assert.True(t, called)
}
