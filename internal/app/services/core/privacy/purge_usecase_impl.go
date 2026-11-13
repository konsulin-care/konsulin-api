package privacy

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/fhir_spark/bundle"

	"go.uber.org/zap"
)

// purgeBatchSize bounds the number of DELETE entries per batch bundle to stay
// within Blaze request limits.
const purgeBatchSize = 100

// ResearchRowDeleter removes a subject's derived research-databank rows
// (*sqlite.ResearchStore), so a patient purge erases the derived copies too.
type ResearchRowDeleter interface {
	// DeleteBySubject deletes every stored row for subjectRef and returns how
	// many were removed.
	DeleteBySubject(ctx context.Context, subjectRef string) (int64, error)
}

type purgeUsecaseImpl struct {
	purgeClient     contracts.PurgeFhirClient
	bundleClient    bundle.BundleFhirClient
	accountDeletion contracts.UserAccountDeleter
	research        ResearchRowDeleter
	log             *zap.Logger
	batchSize       int
}

// NewPatientDataPurger returns a PatientDataPurger that erases a patient's actively-owned
// FHIR data via registry-driven enumeration + batch DELETE bundles, strips the
// Patient resource to a shell, verifies erasure, and only then deletes the
// associated SuperTokens account. research (optional, at most the first
// argument is used) additionally deletes the patient's derived research
// databank rows so no de-identified copy outlives the purge.
func NewPatientDataPurger(
	purgeClient contracts.PurgeFhirClient,
	bundleClient bundle.BundleFhirClient,
	accountDeletion contracts.UserAccountDeleter,
	logger *zap.Logger,
	research ...ResearchRowDeleter,
) contracts.PatientDataPurger {
	var researchDeleter ResearchRowDeleter
	if len(research) > 0 {
		researchDeleter = research[0]
	}
	return &purgeUsecaseImpl{
		purgeClient:     purgeClient,
		bundleClient:    bundleClient,
		accountDeletion: accountDeletion,
		research:        researchDeleter,
		log:             logger,
		batchSize:       purgeBatchSize,
	}
}

// PurgePatientData erases every actively-owned FHIR resource linked to fhirID,
// strips the Patient resource to a PII-free shell, verifies that no
// actively-owned resource survives, and only then deletes the SuperTokens
// account. Shared resources (those referencing other patients or
// practitioners) are excluded by the client and remain referencing the shell.
// An empty enumeration (already purged) is a no-op success.
func (uc *purgeUsecaseImpl) PurgePatientData(ctx context.Context, fhirID, supertokensUserID string) error {
	// Research rows are deleted first and fail closed: derived copies must
	// not survive a purge (BK-DATABANK-003), and a broken databank store
	// aborts the erasure before any irreversible FHIR delete.
	if uc.research != nil {
		if _, err := uc.research.DeleteBySubject(ctx, patientSubjectRef(fhirID)); err != nil {
			return fmt.Errorf("purge: delete research rows: %w", err)
		}
	}

	deletable, err := uc.purgeClient.FindActivelyOwnedResources(ctx, fhirID)
	if err != nil {
		return fmt.Errorf("purge: enumerate actively owned resources: %w", err)
	}

	if err := uc.postDeleteBundles(ctx, deletable); err != nil {
		return fmt.Errorf("purge: delete linked resources: %w", err)
	}

	if err := uc.purgeClient.StripPatientToShell(ctx, fhirID); err != nil {
		return fmt.Errorf("purge: strip patient to shell: %w", err)
	}

	// Fail-closed verification: any remaining actively-owned resource means the
	// erasure is incomplete and the account must not be deleted.
	leftovers, err := uc.purgeClient.FindActivelyOwnedResources(ctx, fhirID)
	if err != nil {
		return fmt.Errorf("purge: verify erasure: %w", err)
	}
	if len(leftovers) > 0 {
		return fmt.Errorf("purge: %d actively-owned resource(s) still remain", len(leftovers))
	}

	// The FHIR purge fully succeeded — only now may the account be deleted.
	if supertokensUserID != "" && uc.accountDeletion != nil {
		if err := uc.accountDeletion.DeleteUserAccount(ctx, supertokensUserID); err != nil {
			return fmt.Errorf("purge: delete account: %w", err)
		}
	}
	return nil
}

// patientSubjectRef normalizes a patient FHIR id to the Patient/{id}
// reference the research databank rows are keyed by.
func patientSubjectRef(fhirID string) string {
	if strings.Contains(fhirID, "/") {
		return fhirID
	}
	return "Patient/" + fhirID
}

// postDeleteBundles posts chunked batch bundles of DELETE entries for refs.
func (uc *purgeUsecaseImpl) postDeleteBundles(ctx context.Context, refs []contracts.ResourceRef) error {
	for start := 0; start < len(refs); start += uc.batchSize {
		end := min(start+uc.batchSize, len(refs))
		entries := make([]map[string]any, 0, end-start)
		for _, ref := range refs[start:end] {
			entries = append(entries, map[string]any{
				"request": map[string]any{
					"method": http.MethodDelete,
					"url":    fmt.Sprintf("%s/%s", ref.ResourceType, ref.ID),
				},
			})
		}
		batch := map[string]any{
			"resourceType": "Bundle",
			"type":         "batch",
			"entry":        entries,
		}
		if _, err := uc.bundleClient.PostTransactionBundle(ctx, batch); err != nil {
			return err
		}
	}
	return nil
}
