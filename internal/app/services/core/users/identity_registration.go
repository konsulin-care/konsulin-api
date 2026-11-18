package users

import (
	"context"

	"github.com/konsulin-care/konsulin-api/internal/app/contracts"
	"github.com/konsulin-care/konsulin-api/internal/app/services/core/federation"

	"github.com/konsulin-care/gateway-sdk-go/identity"
	"go.uber.org/zap"
)

// IdentityClient is the gateway identity-plane surface the signup identity
// registration (T44) needs: contracts.IdentityClient (register, resolve,
// link) plus the adapter's enabled status, so standalone mode (a disabled
// adapter) is skipped before any I/O. The gateway driver's *gateway.Client
// implements it.
type IdentityClient interface {
	contracts.IdentityClient
	// Enabled reports whether the gateway adapter is configured (a
	// non-empty APP_GATEWAY_URL).
	Enabled() bool
}

// registerNewUserIdentities registers the new user's peppered email and
// phone hashes with the gateway identity plane after the FHIR resources are
// initialized (T44). It is best-effort by contract: a missing client, a
// standalone (disabled) gateway, a pepper that cannot be loaded or a
// registration failure is logged and never fails the signup. One Register
// runs per created subject (Patient and/or Practitioner); a non-applicable
// contact field is sent as the gateway "N/A" hashfmt marker, so plaintext
// PII never reaches the wire.
// Parameters: ctx bounds the gateway calls, input carries the email and
// phone, output carries the created resource IDs.
// Returns nothing; never panics, never surfaces an error.
func (uc *userUsecase) registerNewUserIdentities(ctx context.Context, input *contracts.InitializeNewUserFHIRResourcesInput, output *contracts.InitializeNewUserFHIRResourcesOutput) {
	if uc.identityClient == nil {
		uc.Log.Info(logPrefix + "identity registration skipped: no gateway identity client")
		return
	}
	if !uc.identityClient.Enabled() {
		uc.Log.Info(logPrefix + "identity registration skipped: gateway disabled")
		return
	}
	pepper, err := federation.LoadPepper()
	if err != nil {
		uc.Log.Error(logPrefix+"identity registration failed: pepper load", zap.Error(err))
		return
	}
	emailHash, err := identity.HashEmail(input.Email, pepper)
	if err != nil {
		uc.Log.Error(logPrefix+"identity registration failed: email hash", zap.Error(err))
		return
	}
	phoneHash, err := identity.HashPhone(input.Phone, pepper)
	if err != nil {
		uc.Log.Error(logPrefix+"identity registration failed: phone hash", zap.Error(err))
		return
	}
	if output.PatientID != "" {
		uc.registerIdentity(ctx, "patient", "Patient/"+output.PatientID, emailHash, phoneHash)
	}
	if output.PractitionerID != "" {
		uc.registerIdentity(ctx, "practitioner", "Practitioner/"+output.PractitionerID, emailHash, phoneHash)
	}
}

// registerIdentity performs one idempotent identity registration for a
// created subject. A gateway failure is logged (with the subject reference)
// and swallowed: the signup result must not depend on the identity plane.
// Parameters: ctx bounds the call, resourceType the lowercase local resource
// type, localResourceID the FHIR reference of the created resource,
// emailHash and phoneHash the peppered hashfmt values.
// Returns nothing; never panics.
func (uc *userUsecase) registerIdentity(ctx context.Context, resourceType, localResourceID, emailHash, phoneHash string) {
	globalKey, err := uc.identityClient.Register(ctx, resourceType, localResourceID, emailHash, phoneHash)
	if err != nil {
		uc.Log.Error(logPrefix+"identity registration failed",
			zap.String("resource_type", resourceType),
			zap.String("local_resource_id", localResourceID),
			zap.Error(err),
		)
		return
	}
	uc.Log.Info(logPrefix+"identity registered",
		zap.String("resource_type", resourceType),
		zap.String("local_resource_id", localResourceID),
		zap.String("global_key", globalKey),
	)
}
