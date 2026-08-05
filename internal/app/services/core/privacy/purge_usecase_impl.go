package privacy

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/fhir_spark/bundle"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_http_client"

	"go.uber.org/zap"
)

// purgeBatchSize bounds the number of DELETE entries per batch bundle to stay
// within Blaze request limits.
const purgeBatchSize = 100

type purgeUsecaseImpl struct {
	purgeClient     contracts.PurgeFhirClient
	bundleClient    bundle.BundleFhirClient
	accountDeletion contracts.AccountDeletionService
	log             *zap.Logger
	batchSize       int
}

// NewPurgeUsecase returns a PurgeUsecase that erases a patient's FHIR data via
// $everything enumeration + batch DELETE bundles and, on success, deletes the
// associated SuperTokens account.
func NewPurgeUsecase(
	purgeClient contracts.PurgeFhirClient,
	bundleClient bundle.BundleFhirClient,
	accountDeletion contracts.AccountDeletionService,
	logger *zap.Logger,
) contracts.PurgeUsecase {
	return &purgeUsecaseImpl{
		purgeClient:     purgeClient,
		bundleClient:    bundleClient,
		accountDeletion: accountDeletion,
		log:             logger,
		batchSize:       purgeBatchSize,
	}
}

// PurgePatientData erases all FHIR resources linked to fhirID, deletes (or
// PII-strips) the Patient resource itself, asserts no orphan edges remain, and
// only then deletes the SuperTokens account. Empty enumeration (already purged)
// is a no-op success.
func (uc *purgeUsecaseImpl) PurgePatientData(ctx context.Context, fhirID, supertokensUserID string) error {
	refs, err := uc.purgeClient.GetPatientEverything(ctx, fhirID)
	if err != nil {
		return fmt.Errorf("purge: enumerate patient resources: %w", err)
	}

	// Delete every linked resource except the Patient row itself, which is
	// handled separately so a failed delete can fall back to a PII strip.
	deleteRefs := make([]contracts.ResourceRef, 0, len(refs))
	for _, ref := range refs {
		if ref.ResourceType == constvars.ResourcePatient && ref.ID == fhirID {
			continue
		}
		deleteRefs = append(deleteRefs, ref)
	}
	if err := uc.postDeleteBundles(ctx, deleteRefs); err != nil {
		return fmt.Errorf("purge: delete linked resources: %w", err)
	}

	if err := uc.deleteOrStripPatient(ctx, fhirID); err != nil {
		return err
	}

	if err := uc.assertNoOrphans(ctx, fhirID); err != nil {
		return err
	}

	// The FHIR purge fully succeeded — only now may the account be deleted.
	if supertokensUserID != "" && uc.accountDeletion != nil {
		if err := uc.accountDeletion.DeleteUserAccount(ctx, supertokensUserID); err != nil {
			return fmt.Errorf("purge: delete account: %w", err)
		}
	}
	return nil
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

// deleteOrStripPatient deletes the Patient resource; when the delete fails for
// a real reason (not a 404 from an already-purged patient), it honors erasure
// by replacing the resource with a PII-free shell carrying only its id.
func (uc *purgeUsecaseImpl) deleteOrStripPatient(ctx context.Context, fhirID string) error {
	if err := uc.purgeClient.DeletePatient(ctx, fhirID); err != nil {
		var httpErr *fhir_http_client.FHIRHTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
			uc.log.Info("purge: patient already deleted; nothing to strip",
				zap.String(constvars.LoggingPatientIDKey, fhirID))
			return nil
		}
		uc.log.Warn("purge: patient delete failed; stripping PII instead",
			zap.String(constvars.LoggingPatientIDKey, fhirID),
			zap.Error(err))
		if stripErr := uc.purgeClient.StripPatientPII(ctx, fhirID); stripErr != nil {
			return fmt.Errorf("purge: patient delete failed (%v) and PII strip failed: %w", err, stripErr)
		}
	}
	return nil
}

// assertNoOrphans verifies no Communication (sender or recipient) or
// QuestionnaireResponse (author) still references the purged identity.
func (uc *purgeUsecaseImpl) assertNoOrphans(ctx context.Context, fhirID string) error {
	commRefs, err := uc.purgeClient.FindCommunicationRefs(ctx, fhirID)
	if err != nil {
		return fmt.Errorf("purge: orphan check (communications): %w", err)
	}
	if len(commRefs) > 0 {
		return fmt.Errorf("purge: %d orphan Communication(s) still reference %s", len(commRefs), fhirID)
	}

	qrRefs, err := uc.purgeClient.FindQuestionnaireResponseRefsByAuthor(ctx, fhirID)
	if err != nil {
		return fmt.Errorf("purge: orphan check (questionnaire responses): %w", err)
	}
	if len(qrRefs) > 0 {
		return fmt.Errorf("purge: %d orphan QuestionnaireResponse(s) still reference %s", len(qrRefs), fhirID)
	}
	return nil
}
