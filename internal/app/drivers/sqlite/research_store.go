// Package sqlite implements the research databank store: a pure-Go SQLite
// database file at <APP_DATA_DIR>/research.db holding the derived,
// consent-gated research resource index plus the extraction watermark.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, registered as "sqlite"
)

// schemaSQL is the idempotent migration applied on every open. It creates the
// derived resource index (BK-DATABANK-001) and the nightly-extraction
// watermark table.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS research_resources (
	id            TEXT PRIMARY KEY,
	resource_type TEXT NOT NULL,
	source_id     TEXT NOT NULL,
	party_id      TEXT NOT NULL,
	subject_ref   TEXT NOT NULL,
	study_id      TEXT NOT NULL,
	consent_state TEXT NOT NULL,
	payload       TEXT NOT NULL,
	created_at    TEXT NOT NULL,
	purged_at     TEXT
);
CREATE TABLE IF NOT EXISTS research_watermarks (
	resource_type TEXT PRIMARY KEY,
	watermark     TEXT NOT NULL,
	updated_at    TEXT NOT NULL
);
`

// ResearchResource is one derived row of the research databank. Payload holds
// the de-identified resource JSON; PurgedAt is nil until the row is purged.
type ResearchResource struct {
	ID           string
	ResourceType string
	SourceID     string
	PartyID      string
	SubjectRef   string
	StudyID      string
	ConsentState string
	Payload      string
	CreatedAt    time.Time
	PurgedAt     *time.Time
}

// ResearchStore is the SQLite-backed research databank store. It is safe for
// concurrent use by multiple goroutines.
type ResearchStore struct {
	db *sql.DB
}

// OpenResearchStore opens (creating the data directory and the database file
// if needed) <dataDir>/research.db and applies the schema migration.
// Parameters: dataDir is the APP_DATA_DIR value; the database file lives at
// <dataDir>/research.db. Returns the opened store and nil, or a descriptive
// error when the directory or file cannot be created or migrated.
func OpenResearchStore(dataDir string) (*ResearchStore, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("sqlite: create data dir %q: %w", dataDir, err)
	}
	dbPath := filepath.Join(dataDir, "research.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %q: %w", dbPath, err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: migrate %q: %w", dbPath, err)
	}
	return &ResearchStore{db: db}, nil
}

// Close releases the underlying database connection.
// Parameters: none. Returns nil on success or the close error.
func (s *ResearchStore) Close() error {
	return s.db.Close()
}

// Insert stores one derived research resource row.
// Parameters: ctx is the query context, r is the row to store.
// Returns: nil on success, or the insert error (including duplicate IDs).
func (s *ResearchStore) Insert(ctx context.Context, r ResearchResource) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO research_resources
	(id, resource_type, source_id, party_id, subject_ref, study_id,
	 consent_state, payload, created_at, purged_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.ResourceType, r.SourceID, r.PartyID, r.SubjectRef, r.StudyID,
		r.ConsentState, r.Payload, formatTime(r.CreatedAt), formatNullTime(r.PurgedAt))
	if err != nil {
		return fmt.Errorf("sqlite: insert research resource %q: %w", r.ID, err)
	}
	return nil
}

// ListByStudy returns every stored row belonging to the given study, ordered
// by creation time then ID.
// Parameters: ctx is the query context, studyID selects the study.
// Returns: the matching rows and nil, or the query error.
func (s *ResearchStore) ListByStudy(ctx context.Context, studyID string) ([]ResearchResource, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, resource_type, source_id, party_id, subject_ref, study_id,
       consent_state, payload, created_at, purged_at
FROM research_resources
WHERE study_id = ?
ORDER BY created_at, id`, studyID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list study %q: %w", studyID, err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]ResearchResource, 0)
	for rows.Next() {
		var (
			r         ResearchResource
			createdAt string
			purgedAt  sql.NullString
		)
		if err := rows.Scan(&r.ID, &r.ResourceType, &r.SourceID, &r.PartyID,
			&r.SubjectRef, &r.StudyID, &r.ConsentState, &r.Payload,
			&createdAt, &purgedAt); err != nil {
			return nil, fmt.Errorf("sqlite: list study %q: scan: %w", studyID, err)
		}
		created, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("sqlite: list study %q: parse created_at: %w", studyID, err)
		}
		r.CreatedAt = created
		if purgedAt.Valid {
			purged, err := time.Parse(time.RFC3339Nano, purgedAt.String)
			if err != nil {
				return nil, fmt.Errorf("sqlite: list study %q: parse purged_at: %w", studyID, err)
			}
			r.PurgedAt = &purged
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: list study %q: %w", studyID, err)
	}
	return out, nil
}

// DeleteBySubject enforces withdrawal (BK-DATABANK-003): it deletes every row
// of the given subject reference.
// Parameters: ctx is the query context, subjectRef is the FHIR reference
// (e.g. "Patient/<id>"). Returns the number of deleted rows and nil, or the
// delete error.
func (s *ResearchStore) DeleteBySubject(ctx context.Context, subjectRef string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM research_resources WHERE subject_ref = ?`, subjectRef)
	if err != nil {
		return 0, fmt.Errorf("sqlite: delete subject %q: %w", subjectRef, err)
	}
	deleted, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("sqlite: delete subject %q: %w", subjectRef, err)
	}
	return deleted, nil
}

// Watermark reads the stored extraction watermark for a resource type.
// Parameters: ctx is the query context, resourceType selects the watermark.
// Returns: the stored watermark ("" when never set) and nil, or the query
// error.
func (s *ResearchStore) Watermark(ctx context.Context, resourceType string) (string, error) {
	var wm string
	err := s.db.QueryRowContext(ctx,
		`SELECT watermark FROM research_watermarks WHERE resource_type = ?`,
		resourceType).Scan(&wm)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("sqlite: read watermark %q: %w", resourceType, err)
	}
	return wm, nil
}

// SetWatermark stores the extraction watermark for a resource type, replacing
// any previous value.
// Parameters: ctx is the query context, resourceType selects the watermark,
// watermark is the new cursor value. Returns nil on success or the upsert
// error.
func (s *ResearchStore) SetWatermark(ctx context.Context, resourceType, watermark string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO research_watermarks (resource_type, watermark, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(resource_type) DO UPDATE SET
	watermark = excluded.watermark,
	updated_at = excluded.updated_at`,
		resourceType, watermark, formatTime(time.Now()))
	if err != nil {
		return fmt.Errorf("sqlite: write watermark %q: %w", resourceType, err)
	}
	return nil
}

// formatTime renders a timestamp as UTC RFC3339Nano for text storage.
// Parameters: t is the timestamp. Returns the RFC3339Nano string.
func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// formatNullTime renders an optional timestamp as RFC3339Nano, or nil for a
// nil pointer (stored as NULL).
// Parameters: t is the optional timestamp. Returns the RFC3339Nano string,
// or nil.
func formatNullTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return formatTime(*t)
}
