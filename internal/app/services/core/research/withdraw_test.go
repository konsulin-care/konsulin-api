package research

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"testing"

	"konsulin-service/internal/pkg/constvars"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeUpdate records one PUT issued by the withdrawal flow.
type fakeUpdate struct {
	resourceType string
	id           string
	body         map[string]any
}

// fakeEditor serves canned search results and records PUT updates.
type fakeEditor struct {
	byType    map[string][]json.RawMessage
	queries   map[string]url.Values
	searchErr error
	updates   []fakeUpdate
	updateErr error
}

// Search returns the canned entries for resourceType and records params.
func (f *fakeEditor) Search(_ context.Context, resourceType string, params url.Values) ([]json.RawMessage, error) {
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	if f.queries == nil {
		f.queries = map[string]url.Values{}
	}
	f.queries[resourceType] = params
	return f.byType[resourceType], nil
}

// Update decodes and records the PUT body, then echoes it back.
func (f *fakeEditor) Update(_ context.Context, resourceType, id string, resource json.RawMessage) (json.RawMessage, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	var body map[string]any
	if err := json.Unmarshal(resource, &body); err != nil {
		return nil, err
	}
	f.updates = append(f.updates, fakeUpdate{resourceType: resourceType, id: id, body: body})
	return resource, nil
}

// findUpdate returns the recorded update for resourceType/id, or nil.
func (f *fakeEditor) findUpdate(resourceType, id string) *fakeUpdate {
	for i := range f.updates {
		if f.updates[i].resourceType == resourceType && f.updates[i].id == id {
			return &f.updates[i]
		}
	}
	return nil
}

// fakeRowDeleter records DeleteBySubject calls.
type fakeRowDeleter struct {
	deleted []string
	err     error
}

// DeleteBySubject records subjectRef and returns the configured error.
func (f *fakeRowDeleter) DeleteBySubject(_ context.Context, subjectRef string) (int64, error) {
	f.deleted = append(f.deleted, subjectRef)
	if f.err != nil {
		return 0, f.err
	}
	return 1, nil
}

// TestWithdraw_MarksSubjectWithdrawnConsentInactiveAndDeletesRows covers the
// first contract line: Withdraw marks the ResearchSubject withdrawn, the
// research Consent inactive, and deletes the subject's rows immediately.
func TestWithdraw_MarksSubjectWithdrawnConsentInactiveAndDeletesRows(t *testing.T) {
	editor := &fakeEditor{byType: map[string][]json.RawMessage{
		"ResearchSubject": {rawJSON(t, onStudySubject("Patient/pat-1", "ResearchStudy/st-1"))},
		"Consent":         {rawJSON(t, activeResearchConsent("Patient/pat-1"))},
	}}
	store := &fakeRowDeleter{}

	w := NewWithdrawer(editor, store)
	require.NoError(t, w.Withdraw(context.Background(), "pat-1", "st-1"))

	rs := editor.findUpdate("ResearchSubject", "rs-1")
	require.NotNil(t, rs, "the ResearchSubject must be updated")
	assert.Equal(t, "withdrawn", rs.body["status"], "ResearchSubject must be marked withdrawn")

	consent := editor.findUpdate("Consent", "c-1")
	require.NotNil(t, consent, "the research Consent must be updated")
	assert.Equal(t, "inactive", consent.body["status"], "Consent must be marked inactive")

	assert.Equal(t, []string{"Patient/pat-1"}, store.deleted,
		"the subject's research rows must be deleted immediately")
	assert.Equal(t, "Patient/pat-1", editor.queries[constvars.ResourceResearchSubject].Get("individual"),
		"the subject search must be scoped to the patient")
}

// TestWithdraw_IgnoresOtherSubjectsAndNonResearchConsents pins the
// client-side re-checks: resources of another patient (or a non-research
// Consent) are never updated, while the caller's rows are still deleted.
func TestWithdraw_IgnoresOtherSubjectsAndNonResearchConsents(t *testing.T) {
	treatmentConsent := activeResearchConsent("Patient/pat-1")
	treatmentConsent["category"] = []any{map[string]any{
		"coding": []any{map[string]any{"code": "treatment"}},
	}}
	editor := &fakeEditor{byType: map[string][]json.RawMessage{
		"ResearchSubject": {rawJSON(t, onStudySubject("Patient/pat-2", "ResearchStudy/st-1"))},
		"Consent":         {rawJSON(t, treatmentConsent)},
	}}
	store := &fakeRowDeleter{}

	w := NewWithdrawer(editor, store)
	require.NoError(t, w.Withdraw(context.Background(), "pat-1", "st-1"))

	assert.Empty(t, editor.updates, "mismatched or non-research resources must not be updated")
	assert.Equal(t, []string{"Patient/pat-1"}, store.deleted, "row deletion must still run")
}

// TestWithdraw_AlreadyWithdrawnIsIdempotentNoOpUpdate asserts a re-run skips
// the PUTs but still deletes rows (fail-closed cleanup of strays).
func TestWithdraw_AlreadyWithdrawnIsIdempotentNoOpUpdate(t *testing.T) {
	withdrawn := onStudySubject("Patient/pat-1", "ResearchStudy/st-1")
	withdrawn["status"] = researchSubjectStatusWithdrawn
	inactive := activeResearchConsent("Patient/pat-1")
	inactive["status"] = consentStatusInactive
	editor := &fakeEditor{byType: map[string][]json.RawMessage{
		"ResearchSubject": {rawJSON(t, withdrawn)},
		"Consent":         {rawJSON(t, inactive)},
	}}
	store := &fakeRowDeleter{}

	w := NewWithdrawer(editor, store)
	require.NoError(t, w.Withdraw(context.Background(), "pat-1", "st-1"))

	assert.Empty(t, editor.updates, "already-withdrawn resources must not be rewritten")
	assert.Equal(t, []string{"Patient/pat-1"}, store.deleted)
}

// TestWithdraw_SearchErrorAbortsBeforeRowDeletion asserts a FHIR search
// failure fails the withdrawal and no rows are deleted while the source
// status is unknown (fail closed).
func TestWithdraw_SearchErrorAbortsBeforeRowDeletion(t *testing.T) {
	editor := &fakeEditor{searchErr: errors.New("fhir down")}
	store := &fakeRowDeleter{}

	w := NewWithdrawer(editor, store)
	err := w.Withdraw(context.Background(), "pat-1", "st-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "search ResearchSubject")
	assert.Empty(t, store.deleted, "rows must not be deleted before the FHIR marks succeed")
}

// TestWithdraw_UpdateErrorAbortsBeforeRowDeletion asserts a failed PUT fails
// the withdrawal and no rows are deleted while extraction could re-ingest.
func TestWithdraw_UpdateErrorAbortsBeforeRowDeletion(t *testing.T) {
	editor := &fakeEditor{
		byType:    map[string][]json.RawMessage{"ResearchSubject": {rawJSON(t, onStudySubject("Patient/pat-1", "ResearchStudy/st-1"))}},
		updateErr: errors.New("blaze refused put"),
	}
	store := &fakeRowDeleter{}

	w := NewWithdrawer(editor, store)
	err := w.Withdraw(context.Background(), "pat-1", "st-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update ResearchSubject/rs-1")
	assert.Empty(t, store.deleted, "rows must not be deleted while the status still allows re-ingest")
}

// TestWithdraw_DeleteErrorPropagates asserts a store failure is reported to
// the caller instead of being swallowed.
func TestWithdraw_DeleteErrorPropagates(t *testing.T) {
	editor := &fakeEditor{byType: map[string][]json.RawMessage{
		"ResearchSubject": {rawJSON(t, onStudySubject("Patient/pat-1", "ResearchStudy/st-1"))},
		"Consent":         {rawJSON(t, activeResearchConsent("Patient/pat-1"))},
	}}
	store := &fakeRowDeleter{err: errors.New("sqlite locked")}

	w := NewWithdrawer(editor, store)
	err := w.Withdraw(context.Background(), "pat-1", "st-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete rows")
	assert.Equal(t, []string{"Patient/pat-1"}, store.deleted)
}

var (
	_ ResourceEditor    = (*fakeEditor)(nil)
	_ SubjectRowDeleter = (*fakeRowDeleter)(nil)
)
