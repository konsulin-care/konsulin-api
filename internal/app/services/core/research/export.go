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
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/konsulin-care/konsulin-api/internal/app/drivers/sqlite"

	"github.com/konsulin-care/gateway-sdk-go/envelope"
)

// export wire constants: the approval-scope prefix (BK-DATABANK-004), the
// bound-envelope service label, the archive's single member and the
// download-token lifetime.
const (
	exportScopePrefix = "export:"
	exportService     = "research-export"
	exportMemberName  = "research.ndjson"
	exportTokenTTL    = 24 * time.Hour
)

// Export failure sentinels, matchable with errors.Is.
var (
	// ErrExportRejected reports a missing or unverifiable ticket or envelope.
	ErrExportRejected = errors.New("research: export rejected")
	// ErrExportScopeMismatch reports a ticket scope other than export:<study>.
	ErrExportScopeMismatch = errors.New("research: export scope mismatch")
	// ErrExportNotPublished reports an unpublished, non-exportable study.
	ErrExportNotPublished = errors.New("research: study not published")
	// ErrExportTooLarge reports an export over APP_RESEARCH_EXPORT_MAX_BYTES.
	ErrExportTooLarge = errors.New("research: export exceeds APP_RESEARCH_EXPORT_MAX_BYTES")
	// ErrExportNotConfigured reports missing exporter collaborators.
	ErrExportNotConfigured = errors.New("research: exporter not configured")
	// ErrDownloadNotFound reports an unknown, consumed or expired token.
	ErrDownloadNotFound = errors.New("research: download token not found")
)

// TicketVerifier checks one presented approval ticket for the expected
// scope; contracts.ApprovalClient satisfies it (the T62 seam).
type TicketVerifier interface {
	// VerifyTicket returns nil once the ticket verifies for expectedScope
	// (signature, audience, expiry, scope and single-use jti), or the error.
	VerifyTicket(ctx context.Context, token, expectedScope string) error
}

// StudyReader loads one raw FHIR ResearchStudy by id; fhir_spark's
// ResourceClient satisfies it in production wiring.
type StudyReader interface {
	// GetStudy returns the raw ResearchStudy, or an error when absent.
	GetStudy(ctx context.Context, id string) (map[string]any, error)
}

// StudyRowLister lists the databank rows of one study; *sqlite.ResearchStore
// satisfies it.
type StudyRowLister interface {
	// ListByStudy returns every stored row belonging to the given study.
	ListByStudy(ctx context.Context, studyID string) ([]sqlite.ResearchResource, error)
}

// ExportKeyResolver resolves a requester party's use:enc key; the gateway
// adapter's party JWKS resolution satisfies it in production wiring.
type ExportKeyResolver interface {
	// ResolveKey returns the party's use:enc public key and its kid, or an
	// error when the party has no usable use:enc key.
	ResolveKey(ctx context.Context, partyID string) (*ecdh.PublicKey, string, error)
}

// exportRequest is the payload of the sealed request envelope.
type exportRequest struct {
	Study string `json:"study"`
}

// exportClaims is the ticket payload read after presentation: the sending
// party, the approval scope and the single-use jti the download binds to.
type exportClaims struct {
	Sub   string `json:"sub"`
	Scope string `json:"scope"`
	JTI   string `json:"jti"`
}

// parseExportClaims decodes the compact ticket's payload segment (claims
// only; signature validity is proven by VerifyTicket).
// Returns the claims and nil, or an error when the segment is not
// decodable JSON. Never panics.
func parseExportClaims(ticket string) (exportClaims, error) {
	parts := strings.Split(ticket, ".")
	if len(parts) != 3 {
		return exportClaims{}, errors.New("ticket is not a compact JWS")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return exportClaims{}, fmt.Errorf("ticket payload decode: %w", err)
	}
	var claims exportClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return exportClaims{}, fmt.Errorf("ticket payload parse: %w", err)
	}
	return claims, nil
}

// ExportDeps bundles the collaborators the research exporter needs.
type ExportDeps struct {
	// Tickets verifies the presented approval ticket (required).
	Tickets TicketVerifier
	// Studies loads the ResearchStudy whose publication gates export (required).
	Studies StudyReader
	// Rows lists the study's databank rows (required).
	Rows StudyRowLister
	// Keys resolves the requester's use:enc key (required).
	Keys ExportKeyResolver
	// EncPrivateKey is this party's use:enc key opening the request (required).
	EncPrivateKey *ecdh.PrivateKey
	// PartyID is this party's own id, the envelope's expected "to" (required).
	PartyID string
	// AutoPublish is the APP_AUTOPUBLISH flag feeding IsPublished.
	AutoPublish bool
	// MaxBytes is APP_RESEARCH_EXPORT_MAX_BYTES, the export size cap.
	MaxBytes int
}

// Exporter runs sealed cross-party research exports (BK-DATABANK-004): it
// verifies the export ticket and study publication, then asynchronously
// tar.gz's the study's NDJSON rows, seals the archive to the requester's
// use:enc key and serves it once through a 24h download token.
type Exporter struct {
	tickets     TicketVerifier
	studies     StudyReader
	rows        StudyRowLister
	keys        ExportKeyResolver
	encKey      *ecdh.PrivateKey
	partyID     string
	autoPublish bool
	maxBytes    int

	mu   sync.Mutex
	jobs map[string]*exportJob
}

// exportJob is one export awaiting its download: the ticket jti that
// authorized it, the completion signal and the sealed result (or the
// failure) the first Download serves.
type exportJob struct {
	ticketJTI string
	created   time.Time
	done      chan struct{}
	err       error
	sealed    string
}

// NewExporter constructs the research exporter over its collaborators.
// Missing required deps surface as ErrExportNotConfigured at StartExport.
// Parameters: deps carries the ticket verifier, study reader, row lister,
// requester key resolver, this party's use:enc key and id, the APP_AUTOPUBLISH
// flag and the APP_RESEARCH_EXPORT_MAX_BYTES cap.
// Returns the exporter. Never panics.
func NewExporter(deps ExportDeps) *Exporter {
	return &Exporter{
		tickets:     deps.Tickets,
		studies:     deps.Studies,
		rows:        deps.Rows,
		keys:        deps.Keys,
		encKey:      deps.EncPrivateKey,
		partyID:     strings.TrimSpace(deps.PartyID),
		autoPublish: deps.AutoPublish,
		maxBytes:    deps.MaxBytes,
		jobs:        map[string]*exportJob{},
	}
}

// ready fails closed with ErrExportNotConfigured when any required
// collaborator is missing.
// Returns nil when configured, or ErrExportNotConfigured. Never panics.
func (e *Exporter) ready() error {
	if e == nil || e.tickets == nil || e.studies == nil || e.rows == nil ||
		e.keys == nil || e.encKey == nil || e.partyID == "" {
		return ErrExportNotConfigured
	}
	return nil
}

// openRequest trims the presented inputs, decodes the ticket claims, opens
// the bound request envelope under this party's use:enc key and compares
// the ticket scope against export:<study>.
// Parameters: ctx bounds the envelope open; ticket is the compact
// ks-approval+jwt; sealed is the compact bound JWE carrying {study}.
// Returns the claims, the decoded request and nil, or zero values and the
// error — ErrExportRejected or ErrExportScopeMismatch. Never panics.
func (e *Exporter) openRequest(ctx context.Context, ticket, sealed string) (exportClaims, exportRequest, error) {
	ticket = strings.TrimSpace(ticket)
	sealed = strings.TrimSpace(sealed)
	if ticket == "" || sealed == "" {
		return exportClaims{}, exportRequest{}, fmt.Errorf("%w: missing ticket or envelope", ErrExportRejected)
	}
	claims, err := parseExportClaims(ticket)
	if err != nil {
		return exportClaims{}, exportRequest{}, fmt.Errorf("%w: ticket claims: %w", ErrExportRejected, err)
	}
	if claims.Sub == "" {
		return exportClaims{}, exportRequest{}, fmt.Errorf("%w: ticket carries no requester party", ErrExportRejected)
	}
	opened, err := envelope.OpenBound(ctx, sealed, e.encKey, envelope.Expectations{
		From: claims.Sub, To: e.partyID, Service: exportService,
	}, nil, time.Now)
	if err != nil {
		return exportClaims{}, exportRequest{}, fmt.Errorf("%w: open envelope: %w", ErrExportRejected, err)
	}
	var req exportRequest
	if err := json.Unmarshal(opened, &req); err != nil {
		return exportClaims{}, exportRequest{}, fmt.Errorf("%w: request decode: %w", ErrExportRejected, err)
	}
	if req.Study == "" {
		return exportClaims{}, exportRequest{}, fmt.Errorf("%w: request carries no study", ErrExportRejected)
	}
	expectedScope := exportScopePrefix + req.Study
	if claims.Scope != expectedScope {
		return exportClaims{}, exportRequest{}, fmt.Errorf("%w: ticket scope %q, want %q",
			ErrExportScopeMismatch, claims.Scope, expectedScope)
	}
	return claims, req, nil
}

// StartExport starts one sealed export (BK-DATABANK-004): it opens the
// bound request envelope under this party's use:enc key, requires the
// ticket scope to equal export:<study>, verifies the ticket (single-use
// jti), refuses an unpublished study or a payload over
// APP_RESEARCH_EXPORT_MAX_BYTES, then launches the async job that builds
// the tar.gz and seals it to the requester's use:enc key.
// Parameters: ctx bounds the envelope open, ticket check and databank
// reads; ticket is the compact ks-approval+jwt; sealed is the compact
// bound JWE carrying {study}.
// Returns the single-use 24h download token bound to the ticket jti and
// nil, or "" and the error — ErrExportRejected, ErrExportScopeMismatch,
// ErrExportNotPublished, ErrExportTooLarge or ErrExportNotConfigured.
// Never panics.
func (e *Exporter) StartExport(ctx context.Context, ticket, sealed string) (string, error) {
	if err := e.ready(); err != nil {
		return "", err
	}
	ticket = strings.TrimSpace(ticket)
	claims, req, err := e.openRequest(ctx, ticket, sealed)
	if err != nil {
		return "", err
	}
	if err := e.tickets.VerifyTicket(ctx, ticket, exportScopePrefix+req.Study); err != nil {
		return "", fmt.Errorf("%w: verify ticket: %w", ErrExportRejected, err)
	}
	study, err := e.studies.GetStudy(ctx, req.Study)
	if err != nil {
		return "", fmt.Errorf("research: export: read study %q: %w", req.Study, err)
	}
	if !IsPublished(study, e.autoPublish) {
		return "", fmt.Errorf("%w: %s", ErrExportNotPublished, req.Study)
	}
	payload, err := studyNDJSON(ctx, e.rows, req.Study)
	if err != nil {
		return "", err
	}
	if e.maxBytes > 0 && len(payload) > e.maxBytes {
		return "", fmt.Errorf("%w: NDJSON is %d bytes, cap %d",
			ErrExportTooLarge, len(payload), e.maxBytes)
	}
	token, err := newDownloadToken()
	if err != nil {
		return "", err
	}
	job := &exportJob{ticketJTI: claims.JTI, created: time.Now(), done: make(chan struct{})}
	e.mu.Lock()
	e.jobs[token] = job
	e.mu.Unlock()
	go e.runExport(job, payload, claims.Sub)
	return token, nil
}

// studyNDJSON renders the study's non-purged databank rows as NDJSON (one
// de-identified resource JSON per line); purged rows never leave the bank.
// Returns the encoded bytes and nil, or the list error. Never panics.
func studyNDJSON(ctx context.Context, rows StudyRowLister, studyID string) ([]byte, error) {
	list, err := rows.ListByStudy(ctx, studyID)
	if err != nil {
		return nil, fmt.Errorf("research: export: list study %q: %w", studyID, err)
	}
	var buf bytes.Buffer
	for _, row := range list {
		if row.PurgedAt != nil {
			continue
		}
		buf.WriteString(row.Payload)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

// newDownloadToken draws a 256-bit single-use download token (base64url).
// Returns the token and nil, or the randomness error. Never panics.
func newDownloadToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("research: export: download token randomness: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// runExport is the async job: it tar.gz's the NDJSON payload, refuses an
// archive over the cap, seals it to the requester's use:enc key and stores
// the sealed result on the job; job.done closes exactly once, success or
// failure.
// Parameters: job the stored download entry, payload the study NDJSON,
// to the requester party id.
// Returns nothing. Never panics.
func (e *Exporter) runExport(job *exportJob, payload []byte, to string) {
	defer close(job.done)
	archive, err := buildTarGz(payload)
	if err != nil {
		job.err = err
		return
	}
	if e.maxBytes > 0 && archive.Len() > e.maxBytes {
		job.err = fmt.Errorf("%w: archive is %d bytes, cap %d",
			ErrExportTooLarge, archive.Len(), e.maxBytes)
		return
	}
	recipient, kid, err := e.keys.ResolveKey(context.Background(), to)
	if err != nil {
		job.err = fmt.Errorf("research: export: requester use:enc key: %w", err)
		return
	}
	sealed, err := envelope.SealBound(archive.Bytes(), kid, recipient, envelope.Binding{
		From: e.partyID, To: to, Service: exportService,
	})
	if err != nil {
		job.err = fmt.Errorf("research: export: seal archive: %w", err)
		return
	}
	job.sealed = sealed
}

// buildTarGz packs the NDJSON payload as the archive's single member and
// gzips it.
// Returns the archive buffer and nil, or the write error. Never panics.
func buildTarGz(payload []byte) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     exportMemberName,
		Mode:     0o600,
		Size:     int64(len(payload)),
		ModTime:  time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, fmt.Errorf("research: export: tar header: %w", err)
	}
	if _, err := tw.Write(payload); err != nil {
		return nil, fmt.Errorf("research: export: tar write: %w", err)
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("research: export: tar close: %w", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("research: export: gzip close: %w", err)
	}
	return &buf, nil
}

// Download consumes one single-use download token (24h, bound to the
// ticket jti): the token is looked up and deleted first — a second,
// unknown or expired token reports ErrDownloadNotFound — then it waits for
// the async job and serves the sealed tar.gz result or the job's error.
// Parameters: ctx bounds the wait for the async job; token is the download
// token issued by StartExport.
// Returns the compact sealed archive and nil, or "" and the error.
// Never panics.
func (e *Exporter) Download(ctx context.Context, token string) (string, error) {
	if e == nil {
		return "", ErrDownloadNotFound
	}
	token = strings.TrimSpace(token)
	e.mu.Lock()
	job, ok := e.jobs[token]
	if ok {
		delete(e.jobs, token) // consume before serving: single use
	}
	expired := ok && time.Since(job.created) > exportTokenTTL
	e.mu.Unlock()
	if !ok || expired {
		return "", ErrDownloadNotFound
	}
	select {
	case <-job.done:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if job.err != nil {
		return "", job.err
	}
	return job.sealed, nil
}
