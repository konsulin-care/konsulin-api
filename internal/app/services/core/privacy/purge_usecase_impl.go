package privacy

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/fhir_spark/bundle"
	"net/http"

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

// NewPurgeUsecase returns a PurgeUsecase that erases a patient's actively-owned
// FHIR data via registry-driven enumeration + batch DELETE bundles, strips the
// Patient resource to a shell, verifies erasure, and only then deletes the
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

// PurgePatientData erases every actively-owned FHIR resource linked to fhirID,
// strips the Patient resource to a PII-free shell, verifies that no
// actively-owned resource survives, and only then deletes the SuperTokens
// account. Shared resources (those referencing other patients or
// practitioners) are excluded by the client and remain referencing the shell.
// An empty enumeration (already purged) is a no-op success.
func (uc *purgeUsecaseImpl) PurgePatientData(ctx context.Context, fhirID, supertokensUserID string) error {
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
