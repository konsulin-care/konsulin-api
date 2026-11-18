package users

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/exceptions"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_dto"
)

// lookupPractitioner searches for an existing Practitioner by email, phone, or supertoken identifier.
func (uc *userUsecase) lookupPractitioner(ctx context.Context, email, phone, superTokenUserID string) ([]fhir_dto.Practitioner, error) {
	// The supertoken uid identifier is the stable per-user key; match it first
	// so createPractitionerIfNotExists stays idempotent per user (email-first
	// lookup accumulates duplicate Practitioners when the email search
	// parameter is not reliably indexed). Email/phone remain as fallbacks for
	// legacy resources created before the identifier scheme.
	if strings.TrimSpace(superTokenUserID) != "" {
		pracs, err := uc.PractitionerFhirClient.FindPractitionerByIdentifier(ctx, constvars.FhirSupertokenSystemIdentifier, superTokenUserID)
		if err != nil {
			return nil, err
		}
		if len(pracs) > 0 {
			return pracs, nil
		}
	}
	if strings.TrimSpace(email) != "" {
		return uc.PractitionerFhirClient.FindPractitionerByEmail(ctx, email)
	}
	if strings.TrimSpace(phone) != "" {
		return uc.PractitionerFhirClient.FindPractitionerByPhone(ctx, phone)
	}
	return nil, nil
}

// lookupPatient searches for an existing Patient by supertoken uid identifier
// first (the stable per-user key), then falls back to email and phone for
// legacy resources created before the identifier scheme. The uid-first order
// keeps createPatientIfNotExists idempotent per user: repeated magic-link
// create/consume calls converge on one Patient instead of accumulating
// duplicates when the email search parameter is not reliably indexed.
func (uc *userUsecase) lookupPatient(ctx context.Context, email, phone, superTokenUserID string) ([]fhir_dto.Patient, error) {
	if strings.TrimSpace(superTokenUserID) != "" {
		pats, err := uc.PatientFhirClient.FindPatientByIdentifier(
			ctx,
			fmt.Sprintf("%s|%s", constvars.FhirSupertokenSystemIdentifier, superTokenUserID),
		)
		if err != nil {
			return nil, err
		}
		if len(pats) > 0 {
			return pats, nil
		}
	}
	if strings.TrimSpace(email) != "" {
		return uc.PatientFhirClient.FindPatientByEmail(ctx, email)
	}
	if strings.TrimSpace(phone) != "" {
		return uc.PatientFhirClient.FindPatientByPhone(ctx, phone)
	}
	return nil, nil
}

// createNewPractitioner creates a new Practitioner FHIR resource.
//
//nolint:dupl
func (uc *userUsecase) createNewPractitioner(ctx context.Context, email, phone, superTokenUserID string) (*fhir_dto.Practitioner, error) {
	if superTokenUserID == "" {
		return nil, exceptions.ErrInvalidFormat(nil, "superTokenUserID is required for creating a user with Practitioner role")
	}

	userChatwootContact, chatwootErr := uc.callChatwootWithFallback(ctx, email, phone, "")
	chatwootID := strconv.Itoa(userChatwootContact.ChatwootID)
	telecom := buildContactPoints(email, phone)

	newPractitionerInput := &fhir_dto.Practitioner{
		ResourceType: constvars.ResourcePractitioner,
		Active:       true,
		Identifier: []fhir_dto.Identifier{
			{System: constvars.FhirSupertokenSystemIdentifier, Value: superTokenUserID},
		},
		Telecom: telecom,
	}

	if chatwootErr == nil && userChatwootContact.ChatwootID != 0 {
		newPractitionerInput.Identifier = append(newPractitionerInput.Identifier, fhir_dto.Identifier{
			System: constvars.KonsulinOmnichannelSystemIdentifier,
			Value:  chatwootID,
		})
	}

	return uc.PractitionerFhirClient.CreatePractitioner(ctx, newPractitionerInput)
}

// createNewPatient creates a new Patient FHIR resource.
//
//nolint:dupl
func (uc *userUsecase) createNewPatient(ctx context.Context, email, phone, superTokenUserID string) (*fhir_dto.Patient, error) {
	userChatwootContact, chatwootErr := uc.callChatwootWithFallback(ctx, email, phone, "")
	chatwootID := strconv.Itoa(userChatwootContact.ChatwootID)
	telecom := buildContactPoints(email, phone)

	newPatientInput := &fhir_dto.Patient{
		ResourceType: constvars.ResourcePatient,
		Active:       true,
		Identifier:   []fhir_dto.Identifier{},
		Telecom:      telecom,
	}

	if superTokenUserID != "" {
		newPatientInput.Identifier = append(newPatientInput.Identifier, fhir_dto.Identifier{
			System: constvars.FhirSupertokenSystemIdentifier,
			Value:  superTokenUserID,
		})
	}

	if chatwootErr == nil && userChatwootContact.ChatwootID != 0 {
		newPatientInput.Identifier = append(newPatientInput.Identifier, fhir_dto.Identifier{
			System: constvars.KonsulinOmnichannelSystemIdentifier,
			Value:  chatwootID,
		})
	}

	return uc.PatientFhirClient.CreatePatient(ctx, newPatientInput)
}

//nolint:dupl
func (uc *userUsecase) createPractitionerIfNotExists(ctx context.Context, email string, phone string, superTokenUserID string) (*fhir_dto.Practitioner, error) {
	practitioners, err := uc.lookupPractitioner(ctx, email, phone, superTokenUserID)
	if err != nil {
		return nil, err
	}
	if len(practitioners) > 0 {
		return uc.ensurePractitionerIdentifiers(ctx, &practitioners[0], email, phone, superTokenUserID)
	}
	return uc.createNewPractitioner(ctx, email, phone, superTokenUserID)
}

//nolint:dupl
func (uc *userUsecase) createPatientIfNotExists(ctx context.Context, email string, phone string, superTokenUserID string) (*fhir_dto.Patient, error) {
	patients, err := uc.lookupPatient(ctx, email, phone, superTokenUserID)
	if err != nil {
		return nil, err
	}
	if len(patients) > 0 {
		return uc.ensurePatientIdentifiers(ctx, &patients[0], email, phone, superTokenUserID)
	}
	return uc.createNewPatient(ctx, email, phone, superTokenUserID)
}

// buildContactPoints builds a slice of FHIR ContactPoint from email and phone.
// Non-empty values are included; empty values are skipped.
func buildContactPoints(email, phone string) []fhir_dto.ContactPoint {
	var telecom []fhir_dto.ContactPoint
	if strings.TrimSpace(email) != "" {
		telecom = append(telecom, fhir_dto.ContactPoint{
			System: fhir_dto.ContactPointSystemEmail,
			Value:  email,
			Use:    constvars.FhirAddressUseWork,
		})
	}
	if strings.TrimSpace(phone) != "" {
		telecom = append(telecom, fhir_dto.ContactPoint{
			System: fhir_dto.ContactPointSystemPhone,
			Value:  phone,
			Use:    constvars.FhirAddressUseWork,
		})
	}
	return telecom
}
