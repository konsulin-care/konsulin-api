package research

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/konsulin-care/konsulin-api/internal/app/services/core/federation"

	"github.com/konsulin-care/gateway-sdk-go/envelope"
)

// Export request wire constants: the partner's direct export route and its
// single body field, the ticket purpose, the polled status values, the
// download suffix, and the requester's default poll interval and timeout.
const (
	exportRequestPath     = "/api/v1/federation/exports"
	exportRequestField    = "payload"
	exportPurpose         = exportService
	exportStatusPending   = "pending"
	exportStatusReady     = "ready"
	exportDownloadSuffix  = "/download"
	defaultPollInterval   = 500 * time.Millisecond
	defaultRequestTimeout = 30 * time.Second
)

// Export request failure sentinels, matchable with errors.Is.
var (
	// ErrRequestNotConfigured reports missing requester collaborators.
	ErrRequestNotConfigured = errors.New("research: export requester not configured")
	// ErrRequestFailed reports a partner that refused, failed in transport
	// or served an unusable answer anywhere in request, poll or download.
	ErrRequestFailed = errors.New("research: export request failed")
	// ErrResultRejected reports a downloaded archive that will not open
	// under this party's use:enc key or violates the expected binding.
	ErrResultRejected = errors.New("research: export result rejected")
)

// ExportTicketMinter mints the gateway approval ticket handed to the
// partner (the T62 seam; contracts.ApprovalClient satisfies it).
type ExportTicketMinter interface {
	// Mint mints the ticket for the recipient party, allowlisted scope,
	// literal purpose and the base64 32-byte researcher hash.
	Mint(ctx context.Context, to, scope, purpose, researcherHash string) (string, error)
}

// ExportSealer seals the request payload to the partner's use:enc key
// through the SDK bound envelope; the driver adapter satisfies it. A
// partner without a usable use:enc key fails closed before the POST.
type ExportSealer interface {
	// SealBound returns the compact JWE of data for recipientParty under
	// the named service binding.
	SealBound(ctx context.Context, recipientParty, service string, data []byte) (string, error)
}

// ExportPartnerResolver resolves the partner party to its direct endpoint
// address (the T61 directory seam; the referral Directory satisfies it).
type ExportPartnerResolver interface {
	// ResolvePartner returns the active Endpoint address of the partner
	// Organization carrying the recipient party identifier.
	ResolvePartner(ctx context.Context, partyID string) (string, error)
}

// ExportArchiveStore keeps the opened archive for the requesting
// researcher until the researcher fetches it (the researcher-facing
// download seam).
type ExportArchiveStore interface {
	// StoreExport stores the opened archive bytes for the researcher's
	// export of study.
	StoreExport(ctx context.Context, researcher, study string, archive []byte) error
}

// exportAccepted is the partner's 2xx answer to the request POST: the
// partner-issued export id ("id" tolerated as an alias).
type exportAccepted struct {
	Token string `json:"token"`
	ID    string `json:"id"`
}

// exportAnswer is one polled status answer: "pending" while the partner's
// async job runs, "ready" once the download is served ("state" tolerated
// as an alias).
type exportAnswer struct {
	Status string `json:"status"`
	State  string `json:"state"`
}

// statusValue returns the non-empty status field of the answer.
func (a exportAnswer) statusValue() string {
	if a.Status != "" {
		return a.Status
	}
	return a.State
}

// RequestDeps bundles the collaborators the export requester needs; the
// required ones surface as ErrRequestNotConfigured at RequestExport.
type RequestDeps struct {
	Tickets       ExportTicketMinter
	Sealer        ExportSealer
	Directory     ExportPartnerResolver
	Store         ExportArchiveStore
	EncPrivateKey *ecdh.PrivateKey
	PartyID       string
	HTTPClient    *http.Client
	PollInterval  time.Duration
}

// Requester runs researcher-side export requests (BK-DATABANK-004): it
// mints the export:<study> ticket carrying the researcher hash, seals and
// posts the request to the partner, polls until the partner's job is
// ready, downloads the sealed archive, opens it under this party's
// use:enc key and stores the plaintext archive for the researcher.
type Requester struct {
	tickets      ExportTicketMinter
	sealer       ExportSealer
	directory    ExportPartnerResolver
	store        ExportArchiveStore
	encKey       *ecdh.PrivateKey
	partyID      string
	httpClient   *http.Client
	pollInterval time.Duration
}

// NewRequester constructs the researcher-side export requester over its
// collaborators. A nil HTTP client or non-positive poll interval uses the
// 30s client and the 500ms default.
// Parameters: deps carries the ticket minter, request sealer, partner
// resolver, archive store, this party's use:enc key and id, plus the
// optional HTTP client and poll interval.
// Returns the requester. Never panics.
func NewRequester(deps RequestDeps) *Requester {
	client := deps.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultRequestTimeout}
	}
	interval := deps.PollInterval
	if interval <= 0 {
		interval = defaultPollInterval
	}
	return &Requester{
		tickets:      deps.Tickets,
		sealer:       deps.Sealer,
		directory:    deps.Directory,
		store:        deps.Store,
		encKey:       deps.EncPrivateKey,
		partyID:      strings.TrimSpace(deps.PartyID),
		httpClient:   client,
		pollInterval: interval,
	}
}

// ready fails closed with ErrRequestNotConfigured when any required
// collaborator is missing.
// Returns nil when configured, or ErrRequestNotConfigured. Never panics.
func (r *Requester) ready() error {
	if r == nil || r.tickets == nil || r.sealer == nil || r.directory == nil ||
		r.store == nil || r.encKey == nil || r.partyID == "" || r.httpClient == nil {
		return ErrRequestNotConfigured
	}
	return nil
}

// RequestExport runs one researcher's cross-party export request
// (BK-DATABANK-004) end to end: it hashes the researcher's email into
// researcher_hash (federation.UserRef over the federation pepper), mints
// the export:<study> ticket carrying that hash, seals {study} to the
// partner's use:enc key, POSTs the sealed request, polls until the export
// is ready, downloads the sealed archive, opens it under this party's
// use:enc key and stores the plaintext archive for the researcher.
// Parameters: ctx bounds every partner call and the poll loop; researcher
// is the researcher's email; partnerParty the recipient party id; study
// the ResearchStudy id to export.
// Returns the partner-issued export id and nil, or "" and the error —
// ErrRequestNotConfigured, ErrRequestFailed or ErrResultRejected.
// Never panics.
func (r *Requester) RequestExport(ctx context.Context, researcher, partnerParty, study string) (string, error) {
	if err := r.ready(); err != nil {
		return "", err
	}
	researcher = strings.TrimSpace(researcher)
	partnerParty = strings.TrimSpace(partnerParty)
	study = strings.TrimSpace(study)
	if researcher == "" || partnerParty == "" || study == "" {
		return "", fmt.Errorf("%w: missing researcher, partner or study", ErrRequestFailed)
	}
	endpoint, err := r.directory.ResolvePartner(ctx, partnerParty)
	if err != nil {
		return "", fmt.Errorf("research: export request: resolve partner %q: %w", partnerParty, err)
	}
	researcherHash, err := federation.UserRef(researcher, "")
	if err != nil {
		return "", fmt.Errorf("research: export request: researcher hash: %w", err)
	}
	ticket, err := r.tickets.Mint(ctx, partnerParty, exportScopePrefix+study, exportPurpose, researcherHash)
	if err != nil {
		return "", fmt.Errorf("research: export request: mint ticket: %w", err)
	}
	data, err := json.Marshal(exportRequest{Study: study})
	if err != nil {
		return "", fmt.Errorf("research: export request: encode request: %w", err)
	}
	sealed, err := r.sealer.SealBound(ctx, partnerParty, exportService, data)
	if err != nil {
		return "", fmt.Errorf("research: export request: seal request: %w", err)
	}
	exportID, err := r.post(ctx, endpoint, ticket, sealed)
	if err != nil {
		return "", err
	}
	if err := r.waitReady(ctx, endpoint, exportID); err != nil {
		return "", err
	}
	sealedArchive, err := r.download(ctx, endpoint, exportID)
	if err != nil {
		return "", err
	}
	archive, err := envelope.OpenBound(ctx, sealedArchive, r.encKey, envelope.Expectations{
		From: partnerParty, To: r.partyID, Service: exportService,
	}, nil, time.Now)
	if err != nil {
		return "", fmt.Errorf("%w: open archive: %w", ErrResultRejected, err)
	}
	if err := r.store.StoreExport(ctx, researcher, study, archive); err != nil {
		return "", fmt.Errorf("research: export request: store archive: %w", err)
	}
	return exportID, nil
}

// post sends the sealed request with the bearer ticket to the partner's
// export route. Returns the partner-issued export id from the 2xx answer,
// or ErrRequestFailed wrapping the transport error or partner status.
// Never panics.
func (r *Requester) post(ctx context.Context, endpoint, ticket, sealed string) (string, error) {
	body, err := json.Marshal(map[string]string{exportRequestField: sealed})
	if err != nil {
		return "", fmt.Errorf("%w: encode request body: %w", ErrRequestFailed, err)
	}
	target := strings.TrimRight(endpoint, "/") + exportRequestPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("%w: build request: %w", ErrRequestFailed, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ticket)
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: post request: %w", ErrRequestFailed, err)
	}
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
	if readErr != nil {
		return "", fmt.Errorf("%w: read accept answer: %w", ErrRequestFailed, readErr)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("%w: partner answered status %d", ErrRequestFailed, resp.StatusCode)
	}
	var accepted exportAccepted
	if err := json.Unmarshal(raw, &accepted); err != nil {
		return "", fmt.Errorf("%w: decode accept answer: %w", ErrRequestFailed, err)
	}
	exportID := accepted.Token
	if exportID == "" {
		exportID = accepted.ID
	}
	if exportID == "" {
		return "", fmt.Errorf("%w: partner returned no export id", ErrRequestFailed)
	}
	return exportID, nil
}

// waitReady polls the partner's export status until it reports ready; the
// ctx deadline bounds the wait. A status other than pending or ready fails
// closed with ErrRequestFailed.
// Parameters: ctx bounds the poll loop; endpoint the partner base URL;
// exportID the partner-issued export id.
// Returns nil once the export is ready, or the error. Never panics.
func (r *Requester) waitReady(ctx context.Context, endpoint, exportID string) error {
	target := strings.TrimRight(endpoint, "/") + exportRequestPath + "/" + url.PathEscape(exportID)
	for {
		status, err := r.fetchStatus(ctx, target)
		if err != nil {
			return err
		}
		switch status {
		case exportStatusReady:
			return nil
		case exportStatusPending, "":
			// still running: wait out the interval, then poll again
		default:
			return fmt.Errorf("%w: partner reports status %q", ErrRequestFailed, status)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: poll interrupted: %w", ErrRequestFailed, ctx.Err())
		case <-time.After(r.pollInterval):
		}
	}
}

// fetchStatus reads one status answer from target.
// Parameters: ctx bounds the GET; target is the full status URL.
// Returns the reported status value and nil, or "" and ErrRequestFailed
// wrapping the transport error, partner status or undecodable answer.
// Never panics.
func (r *Requester) fetchStatus(ctx context.Context, target string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", fmt.Errorf("%w: build status request: %w", ErrRequestFailed, err)
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: poll status: %w", ErrRequestFailed, err)
	}
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
	if readErr != nil {
		return "", fmt.Errorf("%w: read status answer: %w", ErrRequestFailed, readErr)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("%w: status endpoint answered %d", ErrRequestFailed, resp.StatusCode)
	}
	var answer exportAnswer
	if err := json.Unmarshal(raw, &answer); err != nil {
		return "", fmt.Errorf("%w: decode status answer: %w", ErrRequestFailed, err)
	}
	return answer.statusValue(), nil
}

// download fetches the sealed archive once the partner reports ready.
// Returns the compact JWE and nil, or "" and ErrRequestFailed wrapping the
// transport error, partner status or empty body. Never panics.
func (r *Requester) download(ctx context.Context, endpoint, exportID string) (string, error) {
	target := strings.TrimRight(endpoint, "/") + exportRequestPath + "/" +
		url.PathEscape(exportID) + exportDownloadSuffix
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", fmt.Errorf("%w: build download request: %w", ErrRequestFailed, err)
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: download: %w", ErrRequestFailed, err)
	}
	raw, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return "", fmt.Errorf("%w: read download: %w", ErrRequestFailed, readErr)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("%w: download answered %d", ErrRequestFailed, resp.StatusCode)
	}
	sealed := strings.TrimSpace(string(raw))
	if sealed == "" {
		return "", fmt.Errorf("%w: empty download", ErrRequestFailed)
	}
	return sealed, nil
}
