package privacy

import (
	"context"
	"errors"
	"testing"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/fhir_dto"

	bundlepkg "konsulin-service/internal/app/services/fhir_spark/bundle"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockPurgeClient is a configurable contracts.PurgeFhirClient for usecase
// tests. The first FindActivelyOwnedResources call (enumeration) returns owned;
// later calls (fail-closed verification) return ownedAfter.
type mockPurgeClient struct {
	owned        []contracts.ResourceRef
	ownedErr     error
	ownedAfter   []contracts.ResourceRef
	ownedErrAfter error
	ownedCalls   int

	stripErr   error
	stripCalls int
}

func (m *mockPurgeClient) FindActivelyOwnedResources(_ context.Context, _ string) ([]contracts.ResourceRef, error) {
	m.ownedCalls++
	if m.ownedCalls == 1 {
		return m.owned, m.ownedErr
	}
	return m.ownedAfter, m.ownedErrAfter
}

func (m *mockPurgeClient) StripPatientToShell(_ context.Context, _ string) error {
	m.stripCalls++
	return m.stripErr
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

func TestPurgePatientData_Success_DeletesOwnedResourcesStripsShellAndAccount(t *testing.T) {
	purge := &mockPurgeClient{
		owned: []contracts.ResourceRef{
			{ResourceType: "QuestionnaireResponse", ID: "qr-1"},
			{ResourceType: "Communication", ID: "comm-1"},
		},
	}
	bundle := &mockBundleClient{}
	acct := &mockAccountDeletion{}

	uc := newTestUsecase(purge, bundle, acct)
	err := uc.PurgePatientData(context.Background(), "pat-1", "user-123")
	require.NoError(t, err)

	require.Len(t, bundle.posted, 1, "one batch bundle expected")
	entries := bundle.posted[0]["entry"].([]map[string]any)
	require.Len(t, entries, 2)
	urls := []string{
		entries[0]["request"].(map[string]any)["url"].(string),
		entries[1]["request"].(map[string]any)["url"].(string),
	}
	assert.ElementsMatch(t, []string{"QuestionnaireResponse/qr-1", "Communication/comm-1"}, urls)
	assert.Equal(t, "batch", bundle.posted[0]["type"])

	assert.Equal(t, 1, purge.stripCalls, "patient must be stripped to its shell")
	assert.Equal(t, 2, purge.ownedCalls, "enumeration must run once and verification once")
	assert.Equal(t, []string{"user-123"}, acct.deleted, "account deleted only after the full purge succeeds")
}

func TestPurgePatientData_ChunksLargeBatches(t *testing.T) {
	refs := make([]contracts.ResourceRef, 0, 250)
	for i := 0; i < 250; i++ {
		refs = append(refs, contracts.ResourceRef{ResourceType: "QuestionnaireResponse", ID: "qr"})
	}
	purge := &mockPurgeClient{owned: refs}
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
		owned: []contracts.ResourceRef{{ResourceType: "QuestionnaireResponse", ID: "qr-1"}},
	}
	bundle := &mockBundleClient{}
	acct := &mockAccountDeletion{err: errors.New("supertokens core unreachable")}

	uc := newTestUsecase(purge, bundle, acct)
	err := uc.PurgePatientData(context.Background(), "pat-1", "user-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete account")
	assert.Equal(t, 1, purge.stripCalls, "FHIR purge must complete before account deletion is attempted")
}

func TestPurgePatientData_StripFailurePropagatesAndBlocksAccountDeletion(t *testing.T) {
	purge := &mockPurgeClient{
		owned:    []contracts.ResourceRef{{ResourceType: "QuestionnaireResponse", ID: "qr-1"}},
		stripErr: errors.New("blaze refused put"),
	}
	bundle := &mockBundleClient{}
	acct := &mockAccountDeletion{}

	uc := newTestUsecase(purge, bundle, acct)
	err := uc.PurgePatientData(context.Background(), "pat-1", "user-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "strip patient")
	assert.Empty(t, acct.deleted, "account deletion must never run when the purge did not fully succeed")
}

func TestPurgePatientData_OnlySharedResourcesRemainIsSuccess(t *testing.T) {
	// The client filters shared (recipient/practitioner-referencing) resources
	// out of the enumeration, so an empty deletable set means nothing to delete
	// while the shared edges keep referencing the Patient shell.
	purge := &mockPurgeClient{}
	bundle := &mockBundleClient{}
	acct := &mockAccountDeletion{}

	uc := newTestUsecase(purge, bundle, acct)
	err := uc.PurgePatientData(context.Background(), "pat-1", "user-123")
	require.NoError(t, err)
	assert.Empty(t, bundle.posted, "shared resources must never reach a delete bundle")
	assert.Equal(t, 1, purge.stripCalls)
	assert.Equal(t, []string{"user-123"}, acct.deleted)
}

func TestPurgePatientData_VerificationFindsSurvivorAborts(t *testing.T) {
	purge := &mockPurgeClient{
		owned:      []contracts.ResourceRef{{ResourceType: "Observation", ID: "obs-1"}},
		ownedAfter: []contracts.ResourceRef{{ResourceType: "Observation", ID: "obs-1"}}, // delete did not take
	}
	bundle := &mockBundleClient{}
	acct := &mockAccountDeletion{}

	uc := newTestUsecase(purge, bundle, acct)
	err := uc.PurgePatientData(context.Background(), "pat-1", "user-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "still remain")
	assert.Empty(t, acct.deleted, "account deletion must never run when the purge did not fully succeed")
}

func TestPurgePatientData_VerificationErrorAborts(t *testing.T) {
	purge := &mockPurgeClient{ownedErrAfter: errors.New("fhir down")}
	bundle := &mockBundleClient{}
	acct := &mockAccountDeletion{}

	uc := newTestUsecase(purge, bundle, acct)
	err := uc.PurgePatientData(context.Background(), "pat-1", "user-123")
	require.Error(t, err)
	assert.Empty(t, acct.deleted)
}

func TestPurgePatientData_EnumerationErrorAborts(t *testing.T) {
	purge := &mockPurgeClient{ownedErr: errors.New("fhir down")}
	bundle := &mockBundleClient{}
	acct := &mockAccountDeletion{}

	uc := newTestUsecase(purge, bundle, acct)
	err := uc.PurgePatientData(context.Background(), "pat-1", "user-123")
	require.Error(t, err)
	assert.Empty(t, bundle.posted)
	assert.Equal(t, 0, purge.stripCalls)
	assert.Empty(t, acct.deleted)
}

func TestPurgePatientData_AlreadyPurgedIsIdempotentNoOp(t *testing.T) {
	purge := &mockPurgeClient{}
	bundle := &mockBundleClient{}
	acct := &mockAccountDeletion{}

	uc := newTestUsecase(purge, bundle, acct)
	err := uc.PurgePatientData(context.Background(), "pat-1", "user-123")
	require.NoError(t, err, "repeat purge must be a no-op success")
	assert.Empty(t, bundle.posted)
	assert.Equal(t, 1, purge.stripCalls)
	assert.Equal(t, []string{"user-123"}, acct.deleted)
}

var _ contracts.PurgeFhirClient = (*mockPurgeClient)(nil)
var _ contracts.AccountDeletionService = (*mockAccountDeletion)(nil)
var _ bundlepkg.BundleFhirClient = (*mockBundleClient)(nil)
