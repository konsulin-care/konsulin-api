package research

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/konsulin-care/konsulin-api/internal/app/drivers/sqlite"

	"github.com/konsulin-care/gateway-sdk-go/envelope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// export test parties and fixtures: this party serves the export, the
// requester receives the sealed archive, study-1 is published.
const (
	selfPartyID   = "party-self"
	requesterID   = "party-requester"
	exportStudyID = "study-1"
	draftStudyID  = "study-draft"
)

// exportTicket builds a compact JWS carrying sub/scope/jti claims; the fake
// verifier re-parses it the way the SDK verifier reads real tickets.
func exportTicket(t *testing.T, sub, scope, jti string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"sub": sub, "scope": scope, "jti": jti})
	require.NoError(t, err)
	return "eyJhbGciOiJFUzI1NiJ9." + base64.RawURLEncoding.EncodeToString(payload) + ".c2ln"
}

// fakeExportVerifier checks the scope claim against the expected scope and
// enforces jti single use the way the SDK ticket rule set does.
type fakeExportVerifier struct {
	usedJti map[string]bool
}

// VerifyTicket re-parses the token, compares its scope claim with the
// expected scope and consumes the jti once.
func (f *fakeExportVerifier) VerifyTicket(_ context.Context, token, expectedScope string) error {
	claims, err := parseExportClaims(token)
	if err != nil {
		return err
	}
	if claims.Scope != expectedScope {
		return errors.New("scope mismatch")
	}
	if f.usedJti == nil {
		f.usedJti = map[string]bool{}
	}
	if f.usedJti[claims.JTI] {
		return errors.New("jti replay")
	}
	f.usedJti[claims.JTI] = true
	return nil
}

// fakeExportStudies serves canned ResearchStudy resources by id.
type fakeExportStudies struct {
	byID map[string]map[string]any
}

// GetStudy returns the canned study, or an error when unknown.
func (f *fakeExportStudies) GetStudy(_ context.Context, id string) (map[string]any, error) {
	study, ok := f.byID[id]
	if !ok {
		return nil, errors.New("study not found")
	}
	return study, nil
}

// fakeExportRows serves canned databank rows.
type fakeExportRows struct {
	rows []sqlite.ResearchResource
}

// ListByStudy returns the canned rows belonging to studyID.
func (f *fakeExportRows) ListByStudy(_ context.Context, studyID string) ([]sqlite.ResearchResource, error) {
	out := make([]sqlite.ResearchResource, 0, len(f.rows))
	for _, row := range f.rows {
		if row.StudyID == studyID {
			out = append(out, row)
		}
	}
	return out, nil
}

// fakeExportKeys serves resolved requester use:enc keys by party id.
type fakeExportKeys struct {
	pubs map[string]*ecdh.PublicKey
	kids map[string]string
}

// ResolveKey returns the canned party key and kid, or an error when absent.
func (f *fakeExportKeys) ResolveKey(_ context.Context, partyID string) (*ecdh.PublicKey, string, error) {
	pub, ok := f.pubs[partyID]
	if !ok {
		return nil, "", errors.New("party JWKS has no use:enc key")
	}
	return pub, f.kids[partyID], nil
}

// exportEnv is one constructed exporter with its key material and fakes.
type exportEnv struct {
	exporter      *Exporter
	requesterPriv *ecdh.PrivateKey
	ownPub        *ecdh.PublicKey
	ownKid        string
}

// newExportEnv builds an exporter with fresh X25519 keys, a published
// study, a draft study and two databank rows under the published study.
func newExportEnv(t *testing.T, maxBytes int) *exportEnv {
	t.Helper()
	selfPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	require.NoError(t, err)
	reqPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	require.NoError(t, err)
	ownJWK, err := envelope.EncJWK(selfPriv.PublicKey())
	require.NoError(t, err)
	reqJWK, err := envelope.EncJWK(reqPriv.PublicKey())
	require.NoError(t, err)
	rows := []sqlite.ResearchResource{
		{
			ID: "Observation/o-1", StudyID: exportStudyID,
			Payload: `{"resourceType":"Observation","id":"o-1"}`,
		},
		{
			ID: "Condition/c-1", StudyID: exportStudyID,
			Payload: `{"resourceType":"Condition","id":"c-1"}`,
		},
	}
	exporter := NewExporter(ExportDeps{
		Tickets: &fakeExportVerifier{},
		Studies: &fakeExportStudies{byID: map[string]map[string]any{
			exportStudyID: {
				"status": "active",
				"meta":   map[string]any{"tag": []any{map[string]any{"code": "publishable"}}},
			},
			draftStudyID: {"status": "draft"},
		}},
		Rows: &fakeExportRows{rows: rows},
		Keys: &fakeExportKeys{
			pubs: map[string]*ecdh.PublicKey{requesterID: reqPriv.PublicKey()},
			kids: map[string]string{requesterID: reqJWK.Kid},
		},
		EncPrivateKey: selfPriv,
		PartyID:       selfPartyID,
		MaxBytes:      maxBytes,
	})
	return &exportEnv{
		exporter:      exporter,
		requesterPriv: reqPriv,
		ownPub:        selfPriv.PublicKey(),
		ownKid:        ownJWK.Kid,
	}
}

// sealExportRequest seals the request payload to this party the way the
// requester's POST /research/exports body arrives.
func (e *exportEnv) sealExportRequest(t *testing.T, study string) string {
	t.Helper()
	body, err := json.Marshal(map[string]string{"study": study})
	require.NoError(t, err)
	sealed, err := envelope.SealBound(body, e.ownKid, e.ownPub, envelope.Binding{
		From: requesterID, To: selfPartyID, Service: exportService,
	})
	require.NoError(t, err)
	return sealed
}

// untarGz extracts the archive's single NDJSON member.
func untarGz(t *testing.T, archive []byte) string {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	require.NoError(t, err, "the result must be gzip")
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	hdr, err := tr.Next()
	require.NoError(t, err, "the archive must hold a member")
	assert.Equal(t, exportMemberName, hdr.Name, "the member must be the NDJSON file")
	body, err := io.ReadAll(tr)
	require.NoError(t, err)
	return string(body)
}

// TestStartExportProducesSealedTarGzJobResult covers T74 assertion 1: a
// valid export ticket for a published study issues a download token bound
// to the ticket jti, and its job result opens under the requester's
// use:enc key as a tar.gz of NDJSON.
func TestStartExportProducesSealedTarGzJobResult(t *testing.T) {
	env := newExportEnv(t, 1<<20)
	ctx := context.Background()
	ticket := exportTicket(t, requesterID, "export:"+exportStudyID, "jti-t74-1")

	token, err := env.exporter.StartExport(ctx, ticket,
		env.sealExportRequest(t, exportStudyID))
	require.NoError(t, err, "a valid ticket for a published study must be accepted")
	require.NotEmpty(t, token, "a download token must be issued")

	env.exporter.mu.Lock()
	job := env.exporter.jobs[token]
	env.exporter.mu.Unlock()
	require.NotNil(t, job, "the token must be bound to a stored job")
	assert.Equal(t, "jti-t74-1", job.ticketJTI, "the job must be bound to the ticket jti")

	sealedResult, err := env.exporter.Download(ctx, token)
	require.NoError(t, err, "the finished job must be downloadable")
	opened, err := envelope.OpenBound(ctx, sealedResult, env.requesterPriv, envelope.Expectations{
		From: selfPartyID, To: requesterID, Service: exportService,
	}, nil, time.Now)
	require.NoError(t, err, "the result must open under the requester use:enc key")
	ndjson := untarGz(t, opened)
	lines := strings.Split(strings.TrimSpace(ndjson), "\n")
	require.Len(t, lines, 2, "one NDJSON line per stored row")
	for _, line := range lines {
		assert.True(t, json.Valid([]byte(line)), "every archive line must be JSON: %s", line)
	}
}

// TestDownloadTokenWorksOnceThenNotFound covers T74 assertion 2: the
// download token serves exactly one download; the second attempt is not
// found.
func TestDownloadTokenWorksOnceThenNotFound(t *testing.T) {
	env := newExportEnv(t, 1<<20)
	ctx := context.Background()
	ticket := exportTicket(t, requesterID, "export:"+exportStudyID, "jti-t74-2")
	token, err := env.exporter.StartExport(ctx, ticket,
		env.sealExportRequest(t, exportStudyID))
	require.NoError(t, err)

	first, err := env.exporter.Download(ctx, token)
	require.NoError(t, err, "the first download must succeed")
	assert.NotEmpty(t, first, "the first download must serve the sealed result")

	second, err := env.exporter.Download(ctx, token)
	require.Error(t, err, "the second download must fail")
	assert.ErrorIs(t, err, ErrDownloadNotFound, "the consumed token must be not found")
	assert.Empty(t, second, "no result must come back on the second download")
}

// TestStartExportRejectsUnpublishedMismatchedScopeAndOversize covers T74
// assertion 3: an unpublished study, a mismatched scope or an export over
// APP_RESEARCH_EXPORT_MAX_BYTES is rejected.
func TestStartExportRejectsUnpublishedMismatchedScopeAndOversize(t *testing.T) {
	ctx := context.Background()

	t.Run("unpublished study is rejected", func(t *testing.T) {
		env := newExportEnv(t, 1<<20)
		ticket := exportTicket(t, requesterID, "export:"+draftStudyID, "jti-draft")
		_, err := env.exporter.StartExport(ctx, ticket, env.sealExportRequest(t, draftStudyID))
		require.Error(t, err, "a draft study must not be exported")
		assert.ErrorIs(t, err, ErrExportNotPublished, "publication is the gate")
	})

	t.Run("mismatched scope is rejected", func(t *testing.T) {
		env := newExportEnv(t, 1<<20)
		ticket := exportTicket(t, requesterID, "export:"+draftStudyID, "jti-scope")
		_, err := env.exporter.StartExport(ctx, ticket, env.sealExportRequest(t, exportStudyID))
		require.Error(t, err, "a ticket for another study must not export")
		assert.ErrorIs(t, err, ErrExportScopeMismatch, "scope must equal export:<study>")
	})

	t.Run("non-export scope is rejected", func(t *testing.T) {
		env := newExportEnv(t, 1<<20)
		ticket := exportTicket(t, requesterID, "referral", "jti-referral")
		_, err := env.exporter.StartExport(ctx, ticket, env.sealExportRequest(t, exportStudyID))
		require.Error(t, err, "a referral-scope ticket must not export")
		assert.ErrorIs(t, err, ErrExportScopeMismatch, "only export:<study> scopes pass")
	})

	t.Run("oversized export is rejected", func(t *testing.T) {
		env := newExportEnv(t, 8)
		ticket := exportTicket(t, requesterID, "export:"+exportStudyID, "jti-big")
		_, err := env.exporter.StartExport(ctx, ticket, env.sealExportRequest(t, exportStudyID))
		require.Error(t, err, "an export over the cap must not start")
		assert.ErrorIs(t, err, ErrExportTooLarge, "APP_RESEARCH_EXPORT_MAX_BYTES is the cap")
	})
}
