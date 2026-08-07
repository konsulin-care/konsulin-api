package contracts

import (
	"context"
)

// ResourceRef identifies a single FHIR resource by type and id. It is the
// minimal payload needed to build DELETE entries for a purge batch — full
// resource bodies are never decoded.
type ResourceRef struct {
	ResourceType string `json:"resourceType"`
	ID           string `json:"id"`
}

// PurgeFhirClient enumerates and mutates a patient's FHIR resources for the
// erasure (purge) flow. All requests go through FHIRHTTPClient.Do.
type PurgeFhirClient interface {
	// FindActivelyOwnedResources returns ResourceRefs for every resource the
	// patient actively owns and that is safe to delete. Enumeration is driven by
	// the constvars.PurgeRules registry; each candidate passes the shared-resource
	// safety check (no Patient/{x != patient} or Practitioner/{x} reference) and
	// results are paginated and deduped. Passively owned and shared resources are
	// intentionally excluded.
	FindActivelyOwnedResources(ctx context.Context, patientID string) ([]ResourceRef, error)

	// StripPatientToShell replaces the Patient resource with a PII-free shell
	// carrying only resourceType, id, meta, and active:false. A missing or
	// already-purged patient is a no-op success.
	StripPatientToShell(ctx context.Context, patientID string) error
}

// AccountDeletionService removes a user's SuperTokens account (and its
// sessions). It is invoked only after a successful FHIR purge.
type AccountDeletionService interface {
	// DeleteUserAccount permanently deletes the SuperTokens account for userID.
	DeleteUserAccount(ctx context.Context, userID string) error
}

// PurgeUsecase orchestrates erasure: it removes all FHIR data linked to the
// session patient and, on success, deletes the associated SuperTokens account.
type PurgeUsecase interface {
	// PurgePatientData erases all FHIR resources linked to fhirID, then (and only
	// then) deletes the SuperTokens account for supertokensUserID. When
	// supertokensUserID is empty, the account step is skipped. A second run is a
	// no-op success.
	PurgePatientData(ctx context.Context, fhirID, supertokensUserID string) error
}
