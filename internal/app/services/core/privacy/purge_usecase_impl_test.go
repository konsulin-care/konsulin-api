package privacy

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/fhir_http_client"

	bundlepkg "konsulin-service/internal/app/services/fhir_spark/bundle"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockPurgeClient is a configurable contracts.PurgeFhirClient for usecase tests.
type mockPurgeClient struct {
	everything   []contracts.ResourceRef
	everythingErr error

	deleteErr   error
	deleteCalls int

	stripErr   error
	stripCalls int

	commRefs   []contracts.ResourceRef
	commErr    error
	qrRefs     []contracts.ResourceRef
	qrErr      error
}

func (m *mockPurgeClient) GetPatientEverything(_ context.Context, _ string) ([]contracts.ResourceRef, error) {
	return m.everything, m.everythingErr
}

func (m *mockPurgeClient) DeletePatient(_ context.Context, _ string) error {
	m.deleteCalls++
	return m.deleteErr
}

func (m *mockPurgeClient) StripPatientPII(_ context.Context, _ string) error {
	m.stripCalls++
	return m.stripErr
}

func (m *mockPurgeClient) FindCommunicationRefs(_ context.Context, _ string) ([]contracts.ResourceRef, error) {
	return m.commRefs, m.commErr
}

func (m *mockPurgeClient) FindQuestionnaireResponseRefsByAuthor(_ context.Context, _ string) ([]contracts.ResourceRef, error) {
	return m.qrRefs, m.qrErr
}

// mockBundleClient captures the bundles posted by the usecase.
type mockBundleClient struct {
	posted []map[string]any
	err    error
}

func (m *mockBundleClient) PostTransactionBundle(_ context.Context, bundle map[string]any) (*fhir_dto.FHIRBundle, error) {
	m.posted = append(m.posted, bundle)
	if m.err != nil {
		return nil, m.err
	}
	return &fhir_dto.FHIRBundle{ResourceType: "Bundle", Type: "batch"}, nil
}

// mockAccountDeletion records account-deletion calls.
type mockAccountDeletion struct {
	deleted []string
	err     error
}

func (m *mockAccountDeletion) DeleteUserAccount(_ context.Context, userID string) error {
	m.deleted = append(m.deleted, userID)
	return m.err
}

func newTestUsecase(purge *mockPurgeClient, bundle *mockBundleClient, acct *mockAccountDeletion) *purgeUsecaseImpl {
	return &purgeUsecaseImpl{
		purgeClient:     purge,
		bundleClient:    bundle,
		accountDeletion: acct,
		log:             zap.NewNop(),
		batchSize:       100,
	}
}

func TestPurgePatientData_Success_DeletesLinkedResourcesAndPatient(t *testing.T) {
	purge := &mockPurgeClient{
		everything: []contracts.ResourceRef{
			{ResourceType: "QuestionnaireResponse", ID: "qr-1"},
			{ResourceType: "Communication", ID: "comm-1"},
			{ResourceType: "Patient", ID: "pat-1"}, // the patient itself — excluded from the batch
		},
	}
	bundle := &mockBundleClient{}
	acct := &mockAccountDeletion{}

	uc := newTestUsecase(purge, bundle, acct)
	err := uc.PurgePatientData(context.Background(), "pat-1", "user-123")
	require.NoError(t, err)

	require.Len(t, bundle.posted, 1, "one batch bundle expected")
	entries := bundle.posted[0]["entry"].([]map[string]any)
	require.Len(t, entries, 2, "patient resource must not be in the delete batch")
	urls := []string{
		entries[0]["request"].(map[string]any)["url"].(string),
		entries[1]["request"].(map[string]any)["url"].(string),
	}
	assert.ElementsMatch(t, []string{"QuestionnaireResponse/qr-1", "Communication/comm-1"}, urls)
	assert.Equal(t, "batch", bundle.posted[0]["type"])
	assert.Equal(t, 1, purge.deleteCalls, "patient resource must be deleted separately")
	assert.Equal(t, 0, purge.stripCalls)
	assert.Equal(t, []string{"user-123"}, acct.deleted, "account deleted only after FHIR purge succeeds")
}

func TestPurgePatientData_ChunksLargeBatches(t *testing.T) {
	refs := make([]contracts.ResourceRef, 0, 250)
	for i := 0; i < 250; i++ {
		refs = append(refs, contracts.ResourceRef{ResourceType: "QuestionnaireResponse", ID: "qr"})
	}
	purge := &mockPurgeClient{everything: refs}
	bundle := &mockBundleClient{}
	acct := &mockAccountDeletion{}

	uc := newTestUsecase(purge, bundle, acct)
	require.NoError(t, uc.PurgePatientData(context.Background(), "pat-1", ""))

	require.Len(t, bundle.posted, 3, "250 refs must be split into 3 bundles of 100")
	assert.Len(t, bundle.posted[0]["entry"].([]map[string]any), 100)
	assert.Len(t, bundle.posted[1]["entry"].([]map[string]any), 100)
	assert.Len(t, bundle.posted[2]["entry"].([]map[string]any), 50)
	assert.Empty(t, acct.deleted, "account deletion must be skipped when no supertokens user id is provided")
}

func TestPurgePatientData_AccountDeletionFailurePropagates(t *testing.T) {
	purge := &mockPurgeClient{
		everything: []contracts.ResourceRef{{ResourceType: "QuestionnaireResponse", ID: "qr-1"}},
	}
	bundle := &mockBundleClient{}
	acct := &mockAccountDeletion{err: errors.New("supertokens core unreachable")}

	uc := newTestUsecase(purge, bundle, acct)
	err := uc.PurgePatientData(context.Background(), "pat-1", "user-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete account")
	assert.Equal(t, 1, purge.deleteCalls, "FHIR purge must complete before account deletion is attempted")
}

func TestPurgePatientData_PatientDeleteFailureStripsPII(t *testing.T) {
	purge := &mockPurgeClient{
		everything: []contracts.ResourceRef{
			{ResourceType: "QuestionnaireResponse", ID: "qr-1"},
			{ResourceType: "Patient", ID: "pat-1"},
		},
		deleteErr: errors.New("blaze refused delete"),
	}
	bundle := &mockBundleClient{}
	acct := &mockAccountDeletion{}

	uc := newTestUsecase(purge, bundle, acct)
	err := uc.PurgePatientData(context.Background(), "pat-1", "user-123")
	require.NoError(t, err, "purge must still report success when the patient row cannot be deleted")
	assert.Equal(t, 1, purge.stripCalls, "PII-free shell must be written when the patient delete fails")
	assert.Equal(t, []string{"user-123"}, acct.deleted)
}

func TestPurgePatientData_PatientAlreadyDeletedIsNoOp(t *testing.T) {
	purge := &mockPurgeClient{
		everything: nil, // $everything 404 → empty
		deleteErr: &fhir_http_client.FHIRHTTPError{
			StatusCode: http.StatusNotFound,
			Err:        errors.New("patient not found"),
		}, // second run: patient already gone
	}
	bundle := &mockBundleClient{}
	acct := &mockAccountDeletion{}

	uc := newTestUsecase(purge, bundle, acct)
	err := uc.PurgePatientData(context.Background(), "pat-1", "")
	require.NoError(t, err, "repeat purge must be a no-op success")
	assert.Empty(t, bundle.posted)
	assert.Equal(t, 0, purge.stripCalls, "a missing patient must not trigger a PII strip")
}

func TestPurgePatientData_OrphanCommunicationFails(t *testing.T) {
	purge := &mockPurgeClient{
		everything: []contracts.ResourceRef{{ResourceType: "Communication", ID: "comm-1"}},
		commRefs:   []contracts.ResourceRef{{ResourceType: "Communication", ID: "comm-1"}},
	}
	bundle := &mockBundleClient{}
	acct := &mockAccountDeletion{}

	uc := newTestUsecase(purge, bundle, acct)
	err := uc.PurgePatientData(context.Background(), "pat-1", "user-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "orphan")
	assert.Empty(t, acct.deleted, "account deletion must never run when the purge did not fully succeed")
}

func TestPurgePatientData_OrphanQuestionnaireResponseFails(t *testing.T) {
	purge := &mockPurgeClient{
		everything: []contracts.ResourceRef{{ResourceType: "QuestionnaireResponse", ID: "qr-1"}},
		qrRefs:     []contracts.ResourceRef{{ResourceType: "QuestionnaireResponse", ID: "qr-1"}},
	}
	bundle := &mockBundleClient{}
	acct := &mockAccountDeletion{}

	uc := newTestUsecase(purge, bundle, acct)
	err := uc.PurgePatientData(context.Background(), "pat-1", "user-123")
	require.Error(t, err)
	assert.Empty(t, acct.deleted)
}

func TestPurgePatientData_EnumerationErrorAborts(t *testing.T) {
	purge := &mockPurgeClient{everythingErr: errors.New("fhir down")}
	bundle := &mockBundleClient{}
	acct := &mockAccountDeletion{}

	uc := newTestUsecase(purge, bundle, acct)
	err := uc.PurgePatientData(context.Background(), "pat-1", "user-123")
	require.Error(t, err)
	assert.Empty(t, bundle.posted)
	assert.Equal(t, 0, purge.deleteCalls)
	assert.Empty(t, acct.deleted)
}

var _ contracts.PurgeFhirClient = (*mockPurgeClient)(nil)
var _ contracts.AccountDeletionService = (*mockAccountDeletion)(nil)
var _ bundlepkg.BundleFhirClient = (*mockBundleClient)(nil)
