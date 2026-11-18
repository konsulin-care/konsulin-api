package users

import (
	"context"
	"strconv"

	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_dto"
)

// identifierScanResult holds the result of scanning identifiers for supertoken and Chatwoot IDs.
type identifierScanResult struct {
	foundSupertoken      bool
	foundSupertokenIdx   int
	supertokenExactMatch bool
	foundChatwoot        bool
	foundChatwootIdx     int
	chatwootExactMatch   bool
}

// scanIdentifiers iterates over identifiers and records match status for both
// supertoken and Chatwoot omnichannel identifier systems.
// Returns -1 indices when not found.
func scanIdentifiers(identifiers []fhir_dto.Identifier, superTokenUserID, chatwootID string) identifierScanResult {
	result := identifierScanResult{
		foundSupertokenIdx: -1,
		foundChatwootIdx:   -1,
	}
	for idx, identifier := range identifiers {
		if identifier.System == constvars.FhirSupertokenSystemIdentifier {
			result.foundSupertoken = true
			result.foundSupertokenIdx = idx
			result.supertokenExactMatch = identifier.Value == superTokenUserID
		}
		if identifier.System == constvars.KonsulinOmnichannelSystemIdentifier {
			result.foundChatwoot = true
			result.foundChatwootIdx = idx
			result.chatwootExactMatch = identifier.Value == chatwootID
		}
	}
	return result
}

// ensurePractitionerIdentifiers updates the practitioner's identifiers with the
// supertoken user ID and Chatwoot contact ID if they differ from what's stored.
// Returns the (possibly updated) practitioner.
//
//nolint:dupl
func (uc *userUsecase) ensurePractitionerIdentifiers(ctx context.Context, practitioner *fhir_dto.Practitioner, email, phone, superTokenUserID string) (*fhir_dto.Practitioner, error) {
	userChatwootContact, chatwootCallErr := uc.callChatwootWithFallback(ctx, email, phone, practitioner.FullName())
	chatwootID := strconv.Itoa(userChatwootContact.ChatwootID)

	scanResult := scanIdentifiers(practitioner.Identifier, superTokenUserID, chatwootID)
	mustUpdate := false

	if superTokenUserID != "" {
		if scanResult.foundSupertoken && !scanResult.supertokenExactMatch {
			mustUpdate = true
			practitioner.Identifier[scanResult.foundSupertokenIdx] = fhir_dto.Identifier{
				System: constvars.FhirSupertokenSystemIdentifier,
				Value:  superTokenUserID,
			}
		}
		if !scanResult.foundSupertoken {
			mustUpdate = true
			practitioner.Identifier = append(practitioner.Identifier, fhir_dto.Identifier{
				System: constvars.FhirSupertokenSystemIdentifier,
				Value:  superTokenUserID,
			})
		}
	}

	if chatwootCallErr == nil && userChatwootContact.ChatwootID != 0 {
		if !scanResult.foundChatwoot {
			mustUpdate = true
			practitioner.Identifier = append(practitioner.Identifier, fhir_dto.Identifier{
				System: constvars.KonsulinOmnichannelSystemIdentifier,
				Value:  chatwootID,
			})
		}
		if scanResult.foundChatwoot && !scanResult.chatwootExactMatch {
			mustUpdate = true
			practitioner.Identifier[scanResult.foundChatwootIdx] = fhir_dto.Identifier{
				System: constvars.KonsulinOmnichannelSystemIdentifier,
				Value:  chatwootID,
			}
		}
	}

	if mustUpdate {
		return uc.PractitionerFhirClient.UpdatePractitioner(ctx, practitioner)
	}
	return practitioner, nil
}

// ensurePatientIdentifiers updates the patient's identifiers with the
// supertoken user ID and Chatwoot contact ID if they differ from what's stored.
//
//nolint:dupl
func (uc *userUsecase) ensurePatientIdentifiers(ctx context.Context, patient *fhir_dto.Patient, email, phone, superTokenUserID string) (*fhir_dto.Patient, error) {
	userChatwootContact, chatwootCallErr := uc.callChatwootWithFallback(ctx, email, phone, patient.FullName())
	chatwootID := strconv.Itoa(userChatwootContact.ChatwootID)

	scanResult := scanIdentifiers(patient.Identifier, superTokenUserID, chatwootID)
	mustUpdate := false

	if superTokenUserID != "" {
		if scanResult.foundSupertoken && !scanResult.supertokenExactMatch {
			mustUpdate = true
			patient.Identifier[scanResult.foundSupertokenIdx] = fhir_dto.Identifier{
				System: constvars.FhirSupertokenSystemIdentifier,
				Value:  superTokenUserID,
			}
		}
		if !scanResult.foundSupertoken {
			mustUpdate = true
			patient.Identifier = append(patient.Identifier, fhir_dto.Identifier{
				System: constvars.FhirSupertokenSystemIdentifier,
				Value:  superTokenUserID,
			})
		}
	}

	if chatwootCallErr == nil && userChatwootContact.ChatwootID != 0 {
		if !scanResult.foundChatwoot {
			mustUpdate = true
			patient.Identifier = append(patient.Identifier, fhir_dto.Identifier{
				System: constvars.KonsulinOmnichannelSystemIdentifier,
				Value:  chatwootID,
			})
		}
		if scanResult.foundChatwoot && !scanResult.chatwootExactMatch {
			mustUpdate = true
			patient.Identifier[scanResult.foundChatwootIdx] = fhir_dto.Identifier{
				System: constvars.KonsulinOmnichannelSystemIdentifier,
				Value:  chatwootID,
			}
		}
	}

	if mustUpdate {
		return uc.PatientFhirClient.UpdatePatient(ctx, patient)
	}
	return patient, nil
}
