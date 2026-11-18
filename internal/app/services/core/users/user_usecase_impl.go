package users

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/konsulin-care/konsulin-api/internal/app/config"
	"github.com/konsulin-care/konsulin-api/internal/app/contracts"
	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/exceptions"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_dto"

	"go.uber.org/zap"
)

// logPrefix namespaces every log entry emitted by this use case.
const logPrefix = "userUsecase."

// gatewayOmnichannelListing is the gateway listing served by the automation
// party that carries the omnichannel profile sync (T42A).
const gatewayOmnichannelListing = "modify-profile"

// GatewayWebhookClient is the gateway surface the omnichannel profile sync
// needs: the webhook routes (contracts.WebhookClient) plus the adapter's
// enabled status. The gateway driver's *gateway.Client implements it.
type GatewayWebhookClient interface {
	contracts.WebhookClient
	// Enabled reports whether the gateway adapter is configured (a
	// non-empty APP_GATEWAY_URL).
	Enabled() bool
}

type userUsecase struct {
	UserFHIRInitializerDeps

	// Log is retained for backwards compatibility with callers that read the
	// logger field directly; internal code uses the embedded Logger.
	Log *zap.Logger
}

var (
	userUsecaseInstance     contracts.UserFHIRInitializer
	onceUserFHIRInitializer sync.Once
)

// UserFHIRInitializerDeps bundles the dependencies for NewUserFHIRInitializer.
type UserFHIRInitializerDeps struct {
	PatientFhirClient          contracts.PatientFhirClient
	PractitionerFhirClient     contracts.PractitionerFhirClient
	PractitionerRoleFhirClient contracts.PractitionerRoleFhirClient
	OrganizationFhirClient     contracts.OrganizationFhirClient
	RedisRepository            contracts.RedisRepository
	InternalConfig             *config.InternalConfig
	Logger                     *zap.Logger
	LockerService              contracts.LockerService
	// GatewayWebhooks is the optional gateway adapter for the profile sync
	// (nil skips the sync with no error; also skipped-with-no-error when
	// disabled).
	GatewayWebhooks GatewayWebhookClient
	// GatewayAutomationPartyID is APP_AUTOMATION_PARTY_ID (the party serving
	// the modify-profile listing).
	GatewayAutomationPartyID string
	// IdentityClient is the optional gateway identity-plane client for the
	// post-initialization identity registration (T44); nil or a disabled
	// adapter skips the registration with no error.
	IdentityClient IdentityClient
}

func NewUserFHIRInitializer(deps UserFHIRInitializerDeps) contracts.UserFHIRInitializer {
	onceUserFHIRInitializer.Do(func() {
		instance := &userUsecase{
			UserFHIRInitializerDeps: deps,
			Log:                     deps.Logger,
		}
		userUsecaseInstance = instance
	})
	return userUsecaseInstance
}

func (uc *userUsecase) InitializeNewUserFHIRResources(ctx context.Context, input *contracts.InitializeNewUserFHIRResourcesInput) (*contracts.InitializeNewUserFHIRResourcesOutput, error) {
	if err := input.Validate(); err != nil {
		return nil, exceptions.ErrInvalidFormat(err, "email_or_phone")
	}

	output := &contracts.InitializeNewUserFHIRResourcesOutput{}
	practitionerID := ""

	for _, plan := range input.Resources() {
		id, err := uc.applyResourcePlan(ctx, input, plan, practitionerID, output)
		if err != nil {
			return nil, err
		}
		practitionerID = id
	}

	// T44: register the new user's peppered email/phone hashes with the
	// gateway identity plane. Best-effort: failures are logged and never
	// fail the signup; skipped entirely without a (enabled) client.
	uc.registerNewUserIdentities(ctx, input, output)
	return output, nil
}

// applyResourcePlan creates the FHIR resource described by a single plan entry,
// enforcing the Practitioner dedup guard and the PractitionerRole ordering
// constraint. It returns the practitioner FHIR ID to carry forward: the newly
// created ID when this entry created a Practitioner, or the input value otherwise.
func (uc *userUsecase) applyResourcePlan(ctx context.Context, input *contracts.InitializeNewUserFHIRResourcesInput, plan contracts.FHIRResourcePlan, practitionerID string, output *contracts.InitializeNewUserFHIRResourcesOutput) (string, error) {
	switch plan.ResourceType {
	case constvars.ResourcePractitioner:
		if practitionerID != "" {
			// The plan carries at most one Practitioner entry; guard anyway.
			return practitionerID, nil
		}
		id, err := uc.createPractitionerResource(ctx, input.Email, input.Phone, input.SuperTokenUserID)
		if err != nil {
			return practitionerID, err
		}
		output.PractitionerID = id
		return id, nil
	case constvars.ResourcePatient:
		id, err := uc.createPatientResource(ctx, input.Email, input.Phone, input.SuperTokenUserID)
		if err != nil {
			return practitionerID, err
		}
		output.PatientID = id
		return practitionerID, nil
	case constvars.ResourcePractitionerRole:
		if practitionerID == "" {
			return practitionerID, errors.New("practitioner must be created before PractitionerRole")
		}
		id, err := uc.createPractitionerRoleResource(ctx, practitionerID, input.OrganizationID,
			plan.CodingSystem, plan.CodingCode, plan.CodingDisplay)
		if err != nil {
			return practitionerID, err
		}
		output.PractitionerRoleIDs = append(output.PractitionerRoleIDs, id)
		return practitionerID, nil
	}
	return practitionerID, nil
}

// createPractitionerResource creates the user's Practitioner resource
// (create-if-not-exists) and returns its FHIR ID.
func (uc *userUsecase) createPractitionerResource(ctx context.Context, email, phone, superTokenUserID string) (string, error) {
	practitioner, err := uc.createPractitionerIfNotExists(ctx, email, phone, superTokenUserID)
	if err != nil {
		return "", err
	}
	return practitioner.ID, nil
}

// createPatientResource creates the user's Patient resource (create-if-not-exists)
// and returns its FHIR ID.
func (uc *userUsecase) createPatientResource(ctx context.Context, email, phone, superTokenUserID string) (string, error) {
	patient, err := uc.createPatientIfNotExists(ctx, email, phone, superTokenUserID)
	if err != nil {
		return "", err
	}
	return patient.ID, nil
}

// createPractitionerRoleResource creates the PractitionerRole for the given
// practitioner and coding (create-if-not-exists) and returns its FHIR ID.
func (uc *userUsecase) createPractitionerRoleResource(ctx context.Context, practitionerID, organizationID, system, code, display string) (string, error) {
	role, err := uc.createPractitionerRoleIfNotExists(ctx, practitionerID, organizationID, system, code, display)
	if err != nil {
		return "", err
	}
	return role.ID, nil
}

// createPractitionerRoleIfNotExists looks up an existing PractitionerRole for the
// practitioner carrying the given system|code coding and returns it; otherwise it
// creates a new active role, optionally linked to an organization.
func (uc *userUsecase) createPractitionerRoleIfNotExists(ctx context.Context, practitionerID, organizationID, system, code, display string) (*fhir_dto.PractitionerRole, error) {
	existing, err := uc.PractitionerRoleFhirClient.Search(ctx, contracts.PractitionerRoleSearchParams{
		PractitionerID: practitionerID,
		Code:           fmt.Sprintf("%s|%s", system, code),
	})
	if err != nil {
		return nil, err
	}
	// Some servers (e.g. Blaze) do not index the code search parameter and return
	// every role for the practitioner; filter client-side for the exact system|code
	// match to keep create-if-not-exists idempotent.
	for i := range existing {
		if roleHasCoding(existing[i], system, code) {
			return &existing[i], nil
		}
	}

	role := &fhir_dto.PractitionerRole{
		ResourceType: constvars.ResourcePractitionerRole,
		Active:       true,
		Practitioner: fhir_dto.Reference{Reference: "Practitioner/" + practitionerID},
		Code: []fhir_dto.CodeableConcept{{
			Coding: []fhir_dto.Coding{{
				System:  system,
				Code:    code,
				Display: display,
			}},
		}},
	}
	if organizationID != "" {
		role.Organization = fhir_dto.Reference{Reference: "Organization/" + organizationID}
	}

	return uc.PractitionerRoleFhirClient.CreatePractitionerRole(ctx, role)
}

// roleHasCoding reports whether the role's code element carries the given
// system|code coding.
func roleHasCoding(role fhir_dto.PractitionerRole, system, code string) bool {
	for _, cc := range role.Code {
		for _, c := range cc.Coding {
			if c.System == system && c.Code == code {
				return true
			}
		}
	}
	return false
}
