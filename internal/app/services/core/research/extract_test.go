package research

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"konsulin-service/internal/app/drivers/sqlite"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test-only HMAC key for extraction fixtures.
const extractPseudonymKey = "extract-test-key"

// testPartyID is the owning party recorded on extracted rows.
const testPartyID = "party-1"

// oldWatermark pins the pre-extraction cursor so advancement is observable.
const oldWatermark = "2026-01-01T00:00:00Z"

// fakeSearcher serves canned search results per resource type and records
// the query parameters it was called with.
type fakeSearcher struct {
	byType  map[string][]json.RawMessage
	queries map[string]url.Values
}

// Search returns the canned entries for resourceType and records params.
func (f *fakeSearcher) Search(_ context.Context, resourceType string, params url.Values) ([]json.RawMessage, error) {
	if f.queries == nil {
		f.queries = map[string]url.Values{}
	}
	f.queries[resourceType] = params
	return f.byType[resourceType], nil
}

// fakeStore keeps extracted rows and watermarks in memory.
type fakeStore struct {
	rows map[string]sqlite.ResearchResource
	wm   map[string]string
}

// Insert stores the row by ID.
func (f *fakeStore) Insert(_ context.Context, r sqlite.ResearchResource) error {
	f.rows[r.ID] = r
	return nil
}

// Watermark returns the stored watermark ("" when unset).
func (f *fakeStore) Watermark(_ context.Context, resourceType string) (string, error) {
	return f.wm[resourceType], nil
}

// SetWatermark replaces the stored watermark.
func (f *fakeStore) SetWatermark(_ context.Context, resourceType, watermark string) error {
	f.wm[resourceType] = watermark
	return nil
}

// rawJSON marshals a fixture map for the fake searcher.
func rawJSON(t *testing.T, m map[string]any) json.RawMessage {
	t.Helper()
	out, err := json.Marshal(m)
	require.NoError(t, err)
	return out
}

// onStudySubject returns an on-study ResearchSubject for the patient.
func onStudySubject(patientRef, studyRef string) map[string]any {
	return map[string]any{
		"resourceType": "ResearchSubject",
		"id":           "rs-1",
		"status":       "on-study",
		"study":        map[string]any{"reference": studyRef},
		"individual":   map[string]any{"reference": patientRef},
	}
}

// activeResearchConsent returns an active research Consent for the patient.
func activeResearchConsent(patientRef string) map[string]any {
	return map[string]any{
		"resourceType": "Consent",
		"id":           "c-1",
		"status":       "active",
		"category": []any{map[string]any{
			"coding": []any{map[string]any{
				"system": "http://terminology.hl7.org/CodeSystem/consentcategorycodes",
				"code":   "research",
			}},
		}},
		"patient": map[string]any{"reference": patientRef},
	}
}

// changedObservation returns an Observation with a direct identifier and a
// display-bearing subject reference that must not survive de-identification.
func changedObservation(id, patientRef string) map[string]any {
	return map[string]any{
		"resourceType": "Observation",
		"id":           id,
		"status":       "final",
		"name":         []any{map[string]any{"text": "Heart rate"}},
		"subject": map[string]any{
			"reference": patientRef,
			"display":   "Ada Lovelace",
		},
		"meta": map[string]any{"lastUpdated": "2026-02-01T10:00:00Z"},
	}
}

// TestExtract_StoresDeidentifiedResourcesAndAdvancesWatermark covers the
// first contract line: resources of an on-study subject with an active
// research Consent are stored de-identified and the watermark advances.
func TestExtract_StoresDeidentifiedResourcesAndAdvancesWatermark(t *testing.T) {
	searcher := &fakeSearcher{byType: map[string][]json.RawMessage{
		"ResearchSubject": {rawJSON(t, onStudySubject("Patient/pat-1", "ResearchStudy/st-1"))},
		"Consent":         {rawJSON(t, activeResearchConsent("Patient/pat-1"))},
		"Observation":     {rawJSON(t, changedObservation("obs-1", "Patient/pat-1"))},
	}}
	store := &fakeStore{
		rows: map[string]sqlite.ResearchResource{},
		wm:   map[string]string{"Observation": oldWatermark},
	}
	ex := NewExtractor(searcher, store, extractPseudonymKey, testPartyID, []string{"Observation"})

	require.NoError(t, ex.Extract(context.Background()))

	row, ok := store.rows["Observation/obs-1"]
	require.True(t, ok, "the consented on-study resource must be stored")
	assert.Equal(t, "Observation", row.ResourceType)
	assert.Equal(t, "obs-1", row.SourceID)
	assert.Equal(t, testPartyID, row.PartyID)
	assert.Equal(t, "Patient/pat-1", row.SubjectRef, "the index keeps the raw ref for withdrawal")
	assert.Equal(t, "ResearchStudy/st-1", row.StudyID)
	assert.Equal(t, "active", row.ConsentState)

	// The payload must be de-identified: no direct identifiers, no display,
	// and a pseudonymized subject reference.
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(row.Payload), &payload))
	assert.NotContains(t, payload, "name", "direct identifiers must be stripped")
	sub, ok := payload["subject"].(map[string]any)
	require.True(t, ok, "subject must survive as a reference")
	assert.NotEqual(t, "Patient/pat-1", sub["reference"], "subject must be pseudonymized")
	assert.NotContains(t, sub, "display", "display must be dropped")
	assert.Equal(t, "final", payload["status"], "clinical fields must survive")

	// The watermark must advance and the next pass must search _since it.
	assert.NotEqual(t, oldWatermark, store.wm["Observation"], "the watermark must advance")
	assert.NotEmpty(t, store.wm["Observation"])
	assert.Equal(t, oldWatermark, searcher.queries["Observation"].Get("_since"),
		"the search must be scoped to resources changed _since the watermark")
}

// TestExtract_SkipsSubjectsWithoutActiveConsent covers the second contract
// line: a resource whose subject is on-study but has no active research
// Consent is not stored.
func TestExtract_SkipsSubjectsWithoutActiveConsent(t *testing.T) {
	// pat-2 is on-study but its Consent is not active.
	inactiveConsent := activeResearchConsent("Patient/pat-2")
	inactiveConsent["status"] = "withdrawn"

	searcher := &fakeSearcher{byType: map[string][]json.RawMessage{
		"ResearchSubject": {rawJSON(t, onStudySubject("Patient/pat-2", "ResearchStudy/st-1"))},
		"Consent":         {rawJSON(t, inactiveConsent)},
		"Observation":     {rawJSON(t, changedObservation("obs-2", "Patient/pat-2"))},
	}}
	store := &fakeStore{
		rows: map[string]sqlite.ResearchResource{},
		wm:   map[string]string{"Observation": oldWatermark},
	}
	ex := NewExtractor(searcher, store, extractPseudonymKey, testPartyID, []string{"Observation"})

	require.NoError(t, ex.Extract(context.Background()))

	assert.Empty(t, store.rows, "resources without an active consent must be skipped")
	// Skipping must not stall the cursor: the watermark still advances.
	assert.NotEqual(t, oldWatermark, store.wm["Observation"])
}

// TestExtract_FailsClosedWithoutPseudonymKey asserts extraction refuses to
// run without APP_RESEARCH_PSEUDONYM_KEY instead of storing reversible data.
func TestExtract_FailsClosedWithoutPseudonymKey(t *testing.T) {
	searcher := &fakeSearcher{}
	store := &fakeStore{rows: map[string]sqlite.ResearchResource{}, wm: map[string]string{}}
	ex := NewExtractor(searcher, store, "  ", testPartyID, nil)

	require.ErrorIs(t, ex.Extract(context.Background()), ErrEmptyPseudonymKey)
	assert.Empty(t, store.rows)
	assert.Empty(t, store.wm)
}

// TestExtract_AdvancesWatermarkToNow pins that the new cursor is a UTC
// RFC3339 timestamp derived from the extraction run.
func TestExtract_AdvancesWatermarkToNow(t *testing.T) {
	searcher := &fakeSearcher{byType: map[string][]json.RawMessage{
		"ResearchSubject": {rawJSON(t, onStudySubject("Patient/pat-1", "ResearchStudy/st-1"))},
		"Consent":         {rawJSON(t, activeResearchConsent("Patient/pat-1"))},
	}}
	store := &fakeStore{rows: map[string]sqlite.ResearchResource{}, wm: map[string]string{}}
	ex := NewExtractor(searcher, store, extractPseudonymKey, testPartyID, []string{"Observation"})

	before := time.Now().UTC().Add(-time.Second)
	require.NoError(t, ex.Extract(context.Background()))

	got, err := time.Parse(time.RFC3339Nano, store.wm["Observation"])
	require.NoError(t, err, "the watermark must be an RFC3339 timestamp")
	assert.True(t, got.After(before), "the watermark must advance to the run time")
}
