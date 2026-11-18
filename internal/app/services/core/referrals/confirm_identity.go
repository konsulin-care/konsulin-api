package referrals

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/konsulin-care/konsulin-api/internal/app/contracts"
	"github.com/konsulin-care/konsulin-api/internal/app/services/core/federation"
	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_dto"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_http_client"

	"github.com/konsulin-care/gateway-sdk-go/identity"
)

// resolveGlobalKey reads the confirmed local Patient, hashes its contact
// values with the federation pepper (the gateway "N/A" marker for an
// absent field) and resolves them to the confirmed candidate's global
// key: a candidate row other than the local patient's own is preferred,
// any candidate carrying a global key is accepted, and no candidate
// fails closed before any link.
//
// Parameters:
//   - ctx bounds the Patient read and the gateway resolve.
//   - patientID is the confirmed candidate Patient id.
//
// Returns the global key and nil, or "" and the error. Never panics.
func (c *Confirmer) resolveGlobalKey(ctx context.Context, patientID string) (string, error) {
	patient, err := fhir_http_client.GetResource[fhir_dto.Patient](ctx, c.log, c.fhir,
		c.fhirBaseURL+"/"+constvars.ResourcePatient, patientID,
		constvars.ResourcePatient, "patient_id")
	if err != nil {
		return "", fmt.Errorf("referrals: read patient: %w", err)
	}
	pepper, err := federation.LoadPepper()
	if err != nil {
		return "", fmt.Errorf("referrals: load pepper: %w", err)
	}
	email, phone := referralContact(patient.Telecom)
	if email == "" && phone == "" {
		return "", errors.New("referrals: confirmed patient carries no contact to resolve")
	}
	emailHash, phoneHash, err := hashContact(email, phone, pepper)
	if err != nil {
		return "", err
	}
	candidates, err := c.identities.Resolve(ctx, emailHash, phoneHash)
	if err != nil {
		return "", fmt.Errorf("referrals: resolve identity: %w", err)
	}
	return pickGlobalKey(candidates, constvars.FHIRRefPrefixPatient+patientID)
}

// hashContact hashes the confirmed patient's contact values with the
// federation pepper, using the gateway "N/A" marker for an absent
// field.
//
// Parameters:
//   - email is the patient's email address, empty when absent.
//   - phone is the patient's phone number, empty when absent.
//   - pepper is the federation pepper to hash with.
//
// Returns the email hash, the phone hash and nil, or "", "" and the
// error. Never panics.
func hashContact(email, phone string, pepper []byte) (emailHash, phoneHash string, err error) {
	emailHash, phoneHash = identity.NAMarker, identity.NAMarker
	if email != "" {
		emailHash, err = identity.HashEmail(email, pepper)
		if err != nil {
			return "", "", fmt.Errorf("referrals: hash email: %w", err)
		}
	}
	if phone != "" {
		phoneHash, err = identity.HashPhone(phone, pepper)
		if err != nil {
			return "", "", fmt.Errorf("referrals: hash phone: %w", err)
		}
	}
	return emailHash, phoneHash, nil
}

// pickGlobalKey chooses the confirmed candidate's global key from the
// resolved identity rows: a row other than the local patient's own is
// preferred, any row carrying a global key is accepted, and no
// candidate fails closed before any link.
//
// Parameters:
//   - candidates are the identity rows resolved at the gateway.
//   - ownRef is the local patient reference to prefer skipping.
//
// Returns the global key and nil, or "" and the error. Never panics.
func pickGlobalKey(candidates []contracts.IdentityCandidate, ownRef string) (string, error) {
	for _, cand := range candidates {
		if cand.LocalResourceID != ownRef && strings.TrimSpace(cand.GlobalKey) != "" {
			return cand.GlobalKey, nil
		}
	}
	for _, cand := range candidates {
		if strings.TrimSpace(cand.GlobalKey) != "" {
			return cand.GlobalKey, nil
		}
	}
	return "", errors.New("referrals: gateway reports no identity candidate")
}
