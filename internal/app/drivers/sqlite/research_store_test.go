package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenCreatesSchemaInDataDirectory covers assertion 1: opening the store
// creates research.db in the data directory with the research_resources and
// watermark tables (and the declared columns) in place.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestOpenCreatesSchemaInDataDirectory(t *testing.T) {
	dataDir := t.TempDir()

	store, err := OpenResearchStore(dataDir)
	require.NoError(t, err, "opening the store in a fresh data dir must succeed")
	t.Cleanup(func() { _ = store.Close() })

	_, err = os.Stat(filepath.Join(dataDir, "research.db"))
	require.NoError(t, err, "research.db must exist in the data directory")

	tables, err := store.tableNames(context.Background())
	require.NoError(t, err)
	assert.Contains(t, tables, "research_resources",
		"the research_resources table must be created on open")
	assert.Contains(t, tables, "research_watermarks",
		"the watermark table must be created on open")

	cols, err := store.columnNames(context.Background(), "research_resources")
	require.NoError(t, err)
	for _, want := range []string{
		"id", "resource_type", "source_id", "party_id", "subject_ref",
		"study_id", "consent_state", "payload", "created_at", "purged_at",
	} {
		assert.Contains(t, cols, want, "research_resources must declare column %q", want)
	}

	// Re-opening the same file is idempotent (migration is CREATE IF NOT EXISTS).
	again, err := OpenResearchStore(dataDir)
	require.NoError(t, err, "re-opening an existing store must succeed")
	require.NoError(t, again.Close())
}

// TestListByStudyReturnsInsertedRows covers assertion 2: rows inserted under a
// study are listed by that study with every field round-tripped, and rows of a
// different study stay out of the result.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestListByStudyReturnsInsertedRows(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	created := time.Now().UTC().Truncate(time.Second)
	purged := created.Add(time.Hour)

	inside := ResearchResource{
		ID:           "res-1",
		ResourceType: "Observation",
		SourceID:     "obs-1",
		PartyID:      "party-a",
		SubjectRef:   "Patient/sub-1",
		StudyID:      "study-1",
		ConsentState: "active",
		Payload:      `{"resourceType":"Observation"}`,
		CreatedAt:    created,
	}
	other := ResearchResource{
		ID:           "res-2",
		ResourceType: "Condition",
		SourceID:     "cond-9",
		PartyID:      "party-a",
		SubjectRef:   "Patient/sub-2",
		StudyID:      "study-2",
		ConsentState: "withdrawn",
		Payload:      `{"resourceType":"Condition"}`,
		CreatedAt:    created,
		PurgedAt:     &purged,
	}
	require.NoError(t, store.Insert(ctx, inside))
	require.NoError(t, store.Insert(ctx, other))

	got, err := store.ListByStudy(ctx, "study-1")
	require.NoError(t, err)
	require.Len(t, got, 1, "only the study's own rows must be listed")

	row := got[0]
	assert.Equal(t, inside.ID, row.ID)
	assert.Equal(t, inside.ResourceType, row.ResourceType)
	assert.Equal(t, inside.SourceID, row.SourceID)
	assert.Equal(t, inside.PartyID, row.PartyID)
	assert.Equal(t, inside.SubjectRef, row.SubjectRef)
	assert.Equal(t, inside.StudyID, row.StudyID)
	assert.Equal(t, inside.ConsentState, row.ConsentState)
	assert.Equal(t, inside.Payload, row.Payload)
	assert.True(t, inside.CreatedAt.Equal(row.CreatedAt), "created_at must round-trip")
	assert.Nil(t, row.PurgedAt, "an un-purged row must read purged_at as nil")

	// The purged_at column round-trips for purged rows, too.
	got2, err := store.ListByStudy(ctx, "study-2")
	require.NoError(t, err)
	require.Len(t, got2, 1)
	require.NotNil(t, got2[0].PurgedAt)
	assert.True(t, purged.Equal(*got2[0].PurgedAt), "purged_at must round-trip")
	assert.Equal(t, "withdrawn", got2[0].ConsentState)

	empty, err := store.ListByStudy(ctx, "study-missing")
	require.NoError(t, err)
	assert.Empty(t, empty, "an unknown study lists no rows")
}

// TestDeleteBySubjectRemovesOnlyThatSubjectsRows covers assertion 3: deleting
// a subject removes exactly that subject's rows and keeps every other
// subject's rows in the same study.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestDeleteBySubjectRemovesOnlyThatSubjectsRows(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	created := time.Now().UTC().Truncate(time.Second)

	mine := []ResearchResource{
		{
			ID:           "r-1",
			ResourceType: "Observation",
			SourceID:     "obs-1",
			PartyID:      "party-a",
			SubjectRef:   "Patient/alice",
			StudyID:      "study-1",
			ConsentState: "active",
			Payload:      "{}",
			CreatedAt:    created,
		},
		{
			ID:           "r-2",
			ResourceType: "Condition",
			SourceID:     "cond-1",
			PartyID:      "party-a",
			SubjectRef:   "Patient/alice",
			StudyID:      "study-1",
			ConsentState: "active",
			Payload:      "{}",
			CreatedAt:    created,
		},
	}
	keep := ResearchResource{
		ID:           "r-3",
		ResourceType: "Observation",
		SourceID:     "obs-2",
		PartyID:      "party-a",
		SubjectRef:   "Patient/bob",
		StudyID:      "study-1",
		ConsentState: "active",
		Payload:      "{}",
		CreatedAt:    created,
	}
	for _, r := range append(mine, keep) {
		require.NoError(t, store.Insert(ctx, r))
	}

	deleted, err := store.DeleteBySubject(ctx, "Patient/alice")
	require.NoError(t, err)
	assert.EqualValues(t, 2, deleted, "both of the subject's rows must be deleted")

	remaining, err := store.ListByStudy(ctx, "study-1")
	require.NoError(t, err)
	require.Len(t, remaining, 1, "other subjects' rows must survive")
	assert.Equal(t, "Patient/bob", remaining[0].SubjectRef)
	assert.Equal(t, "r-3", remaining[0].ID)

	// Deleting an unknown subject is a no-op, not an error.
	none, err := store.DeleteBySubject(ctx, "Patient/nobody")
	require.NoError(t, err)
	assert.EqualValues(t, 0, none)
}

// tableNames lists the names of every user table in the store's database.
// Parameters: ctx is the query context.
// Returns: the table names and any query error.
func (s *ResearchStore) tableNames(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// columnNames lists the column names of one table in the store's database.
// Parameters: ctx is the query context, table is the table name.
// Returns: the column names and any query error.
func (s *ResearchStore) columnNames(ctx context.Context, table string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var (
			cid     int
			name    string
			typ     string
			notnull int
			dflt    *string
			pk      int
		)
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// newTestStore opens a research store in a per-test temp data directory and
// closes it when the test finishes.
// Parameters: t is the test harness.
// Returns: the opened store.
func newTestStore(t *testing.T) *ResearchStore {
	t.Helper()
	store, err := OpenResearchStore(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	return store
}
