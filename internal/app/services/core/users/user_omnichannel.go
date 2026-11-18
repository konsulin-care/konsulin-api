package users

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/konsulin-care/konsulin-api/internal/app/services/core/federation"
	"github.com/konsulin-care/konsulin-api/internal/pkg/utils"

	"go.uber.org/zap"
)

// callChatwootWithFallback calls the omnichannel webhook to create/update a Chatwoot contact.
// If email and phone are both empty, it returns immediately with no error.
// If the call fails, the error is logged and returned for callers to decide how to handle it.
func (uc *userUsecase) callChatwootWithFallback(ctx context.Context, email, phone, username string) (callWebhookSvcKonsulinOmnichannelOutput, error) {
	if strings.TrimSpace(email) == "" && strings.TrimSpace(phone) == "" {
		return callWebhookSvcKonsulinOmnichannelOutput{}, nil
	}
	result, err := uc.callWebhookSvcKonsulinOmnichannel(ctx, callWebhookSvcKonsulinOmnichannelInput{
		Email:    email,
		Username: username,
		Phone:    phone,
	})
	if err != nil {
		uc.Logger.Error(logPrefix+"callChatwootWithFallback error calling webhook svc konsulin omnichannel",
			zap.Error(err),
		)
		return callWebhookSvcKonsulinOmnichannelOutput{}, err
	}
	return result, nil
}

type callWebhookSvcKonsulinOmnichannelOutput struct {
	ChatwootID  int    `json:"chatwoot_id"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phoneNumber"`
}

type callWebhookSvcKonsulinOmnichannelInput struct {
	Email    string
	Username string
	Phone    string
}

type callWebhookSvcKonsulinOmnichannelRawOutput struct {
	ChatwootID  int     `json:"chatwoot_id"`
	Email       string  `json:"email"`
	PhoneNumber *string `json:"phoneNumber"`
}

func (uc *userUsecase) callWebhookSvcKonsulinOmnichannel(ctx context.Context, input callWebhookSvcKonsulinOmnichannelInput) (callWebhookSvcKonsulinOmnichannelOutput, error) {
	// T42B: the legacy in-process forwarder and HTTP loopback paths are
	// removed; the profile sync runs only through the gateway. Without
	// gateway deps the sync is skipped with a log and no error, matching
	// the disabled-gateway behavior.
	if uc.GatewayWebhooks == nil {
		uc.Logger.Info(logPrefix + "callWebhookSvcKonsulinOmnichannel skipped: no gateway client")
		return callWebhookSvcKonsulinOmnichannelOutput{}, nil
	}
	return uc.callGatewayOmnichannel(ctx, input)
}

// marshalOmnichannelBody builds the existing profile-sync body: the username
// falls back to the email local part and the phone is normalized to E.164
// with a leading '+' (a detail kept internal to this use case).
// Parameters: input carries the email, username and phone.
// Returns the JSON body bytes, or nil and the marshal error. Never panics.
func marshalOmnichannelBody(input callWebhookSvcKonsulinOmnichannelInput) ([]byte, error) {
	lastUsername := input.Username
	if lastUsername == "" {
		if strings.TrimSpace(input.Email) != "" {
			lastUsername = strings.Split(input.Email, "@")[0]
		}
	}

	phoneE164 := utils.FormatE164WithPlus(input.Phone)

	body := struct {
		Email string `json:"email,omitempty"`
		Name  string `json:"name"`
		Phone string `json:"phoneNumber,omitempty"`
	}{
		Email: input.Email,
		Name:  lastUsername,
		Phone: phoneE164,
	}

	return json.Marshal(body)
}

// callGatewayOmnichannel performs the T42A gateway profile sync: it invokes
// the automation party's modify-profile listing through the gateway
// WebhookClient (data = the existing body, user_ref = the user's peppered
// hash) and parses the chatwoot reply. It is skipped with a log and no error
// when the gateway is disabled or APP_AUTOMATION_PARTY_ID is empty — no
// legacy path runs in that case either.
// Parameters: ctx bounds the sync; input carries email, username and phone.
// Returns the parsed chatwoot output, or the zero output with nil when the
// sync is skipped. Never panics.
func (uc *userUsecase) callGatewayOmnichannel(ctx context.Context, input callWebhookSvcKonsulinOmnichannelInput) (callWebhookSvcKonsulinOmnichannelOutput, error) {
	if !uc.GatewayWebhooks.Enabled() {
		uc.Logger.Info(logPrefix + "callWebhookSvcKonsulinOmnichannel skipped: gateway disabled")
		return callWebhookSvcKonsulinOmnichannelOutput{}, nil
	}
	automation := strings.TrimSpace(uc.GatewayAutomationPartyID)
	if automation == "" {
		uc.Logger.Info(logPrefix + "callWebhookSvcKonsulinOmnichannel skipped: APP_AUTOMATION_PARTY_ID is empty")
		return callWebhookSvcKonsulinOmnichannelOutput{}, nil
	}

	bodyBytes, err := marshalOmnichannelBody(input)
	if err != nil {
		return callWebhookSvcKonsulinOmnichannelOutput{}, err
	}
	userRef, err := federation.UserRef(input.Email, input.Phone)
	if err != nil {
		return callWebhookSvcKonsulinOmnichannelOutput{}, err
	}

	reply, err := uc.GatewayWebhooks.Sync(ctx, automation, gatewayOmnichannelListing, userRef, bodyBytes)
	if err != nil {
		return callWebhookSvcKonsulinOmnichannelOutput{}, err
	}
	return parseOmnichannelResponse(reply)
}

// parseOmnichannelResponse parses the raw response body from the omnichannel webhook
// into a structured output. It handles nil phone number fields safely.
func parseOmnichannelResponse(body []byte) (callWebhookSvcKonsulinOmnichannelOutput, error) {
	var rawOutputs []callWebhookSvcKonsulinOmnichannelRawOutput
	if err := json.Unmarshal(body, &rawOutputs); err != nil {
		return callWebhookSvcKonsulinOmnichannelOutput{}, err
	}
	if len(rawOutputs) == 0 {
		return callWebhookSvcKonsulinOmnichannelOutput{}, errors.New("webhook svc konsulin omnichannel returned empty response")
	}

	raw := rawOutputs[0]
	output := callWebhookSvcKonsulinOmnichannelOutput{
		ChatwootID:  raw.ChatwootID,
		Email:       raw.Email,
		PhoneNumber: "",
	}

	// the upstream server might omit the phone number or assigning null to it
	// this was made to ensure no nil pointer dereference happen when
	// the downstream code try to access the phone number
	if raw.PhoneNumber != nil {
		output.PhoneNumber = *raw.PhoneNumber
	}
	return output, nil
}
