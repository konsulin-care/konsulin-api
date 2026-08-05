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
	// GetPatientEverything returns ResourceRefs for every resource linked to the
	// patient via GET /Patient/{id}/$everything, following pagination until
	// exhausted. A missing or already-purged patient yields an empty, nil result.
	GetPatientEverything(ctx context.Context, patientID string) ([]ResourceRef, error)

	// DeletePatient DELETEs the Patient resource. Returns an error on failure.
	DeletePatient(ctx context.Context, patientID string) error

	// StripPatientPII replaces the Patient resource with a PII-free shell that
	// carries only its id (resourceType + id, no name/identifier/telecom/address).
	StripPatientPII(ctx context.Context, patientID string) error

	// FindCommunicationRefs returns Communications referencing the patient as
	// sender or recipient (sender and recipient searches combined and deduped).
	// Used by the post-purge orphan-edge check.
	FindCommunicationRefs(ctx context.Context, patientID string) ([]ResourceRef, error)

	// FindQuestionnaireResponseRefsByAuthor returns QuestionnaireResponses
	// authored by the patient. Used by the post-purge orphan-edge check.
	FindQuestionnaireResponseRefsByAuthor(ctx context.Context, patientID string) ([]ResourceRef, error)
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
