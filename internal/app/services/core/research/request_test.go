package research

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/konsulin-care/gateway-sdk-go/envelope"
	"github.com/konsulin-care/gateway-sdk-go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// request test fixtures: the partner party, the researcher's email, the
// canned ticket and export id, and the fixed 32-byte test-only pepper.
const (
	partnerID       = "party-partner"
	requestEmail    = "researcher@example.com"
	requestExportID = "exp-1"
	requestTicket   = "ticket-abc"
)

var requestPepper = []byte("0123456789abcdef0123456789abcdef")

// fakeRequestMinter records the mint inputs and serves a canned ticket.
type fakeRequestMinter struct {
	ticket  string
	to      string
	scope   string
	purpose string
	hash    string
}

// Mint records the inputs and returns the canned ticket.
func (f *fakeRequestMinter) Mint(_ context.Context, to, scope, purpose, researcherHash string) (string, error) {
	f.to, f.scope, f.purpose, f.hash = to, scope, purpose, researcherHash
	return f.ticket, nil
}

// fakeRequestSealer really seals to the partner's use:enc key under the
// requester-party binding.
type fakeRequestSealer struct {
	partnerPub *ecdh.PublicKey
	partnerKid string
}

// SealBound seals data to the partner key bound from this party.
func (f *fakeRequestSealer) SealBound(_ context.Context, recipientParty, service string, data []byte) (string, error) {
	return envelope.SealBound(data, f.partnerKid, f.partnerPub, envelope.Binding{
		From: requesterID, To: recipientParty, Service: service,
	})
}

// fakeRequestDirectory resolves the partner party to the fake server URL.
type fakeRequestDirectory struct {
	baseURL string
	party   string
}

// ResolvePartner returns the fake server URL and records the party.
func (f *fakeRequestDirectory) ResolvePartner(_ context.Context, partyID string) (string, error) {
	f.party = partyID
	return f.baseURL, nil
}

// fakeRequestStore records the archive stored for the researcher.
type fakeRequestStore struct {
	researcher string
	study      string
	archive    []byte
}

// StoreExport records the stored archive and succeeds.
func (f *fakeRequestStore) StoreExport(_ context.Context, researcher, study string, archive []byte) error {
	f.researcher, f.study, f.archive = researcher, study, archive
	return nil
}

// requestPartner is the fake partner party serving the export protocol: it
// opens the posted request under its own use:enc key, answers one pending
// then one ready status, and serves the sealed archive.
type requestPartner struct {
	priv    *ecdh.PrivateKey
	archive string

	mu      sync.Mutex
	auth    string
	openErr error
	study   string
	polls   int
}

// handler serves the partner's export route, status route and download.
func (p *requestPartner) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == exportRequestPath:
			p.handleRequest(w, r)
		case r.Method == http.MethodGet && r.URL.Path == exportRequestPath+"/"+requestExportID:
			p.handleStatus(w)
		case r.Method == http.MethodGet && r.URL.Path == exportRequestPath+"/"+requestExportID+exportDownloadSuffix:
			w.Header().Set("Content-Type", "application/jose")
			_, _ = io.WriteString(w, p.archive)
		default:
			http.NotFound(w, r)
		}
	})
}

// handleRequest opens the posted request under the partner's own key and
// answers the accepted export id.
func (p *requestPartner) handleRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Payload string `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	opened, err := envelope.OpenBound(r.Context(), body.Payload, p.priv, envelope.Expectations{
		From: requesterID, To: partnerID, Service: exportService,
	}, nil, time.Now)
	var req exportRequest
	_ = json.Unmarshal(opened, &req)
	p.mu.Lock()
	p.auth = r.Header.Get("Authorization")
	p.openErr = err
	p.study = req.Study
	p.mu.Unlock()
	if err != nil {
		http.Error(w, "bad envelope", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	_, _ = io.WriteString(w, `{"token":"`+requestExportID+`"}`)
}

// handleStatus answers one pending poll, then every later poll ready.
func (p *requestPartner) handleStatus(w http.ResponseWriter) {
	p.mu.Lock()
	p.polls++
	n := p.polls
	p.mu.Unlock()
	if n == 1 {
		_, _ = io.WriteString(w, `{"status":"pending"}`)
		return
	}
	_, _ = io.WriteString(w, `{"status":"ready"}`)
}

// snapshot returns the recorded request fields and poll count.
func (p *requestPartner) snapshot() (auth string, openErr error, study string, polls int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.auth, p.openErr, p.study, p.polls
}

// requestEnv is the constructed requester over fakes and the partner server.
type requestEnv struct {
	requester *Requester
	minter    *fakeRequestMinter
	directory *fakeRequestDirectory
	store     *fakeRequestStore
	partner   *requestPartner
	archive   []byte
}

// newRequestEnv builds the requester over a live fake partner server with
// fresh X25519 keys, the fallback pepper and a canned sealed archive.
func newRequestEnv(t *testing.T) *requestEnv {
	t.Helper()
	t.Setenv("APP_DATA_DIR", t.TempDir())
	t.Setenv("APP_PARTY_ENC_PRIVATE_KEY", "")
	t.Setenv("APP_PEPPER", base64.StdEncoding.EncodeToString(requestPepper))
	ownPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	require.NoError(t, err)
	partnerPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	require.NoError(t, err)
	ownJWK, err := envelope.EncJWK(ownPriv.PublicKey())
	require.NoError(t, err)
	partnerJWK, err := envelope.EncJWK(partnerPriv.PublicKey())
	require.NoError(t, err)
	archiveBuf, err := buildTarGz([]byte(`{"resourceType":"Observation","id":"o-1"}`))
	require.NoError(t, err)
	archive := archiveBuf.Bytes()
	sealedArchive, err := envelope.SealBound(archive, ownJWK.Kid, ownPriv.PublicKey(), envelope.Binding{
		From: partnerID, To: requesterID, Service: exportService,
	})
	require.NoError(t, err)
	partner := &requestPartner{priv: partnerPriv, archive: sealedArchive}
	server := httptest.NewServer(partner.handler())
	t.Cleanup(server.Close)
	sealer := &fakeRequestSealer{partnerPub: partnerPriv.PublicKey(), partnerKid: partnerJWK.Kid}
	minter := &fakeRequestMinter{ticket: requestTicket}
	directory := &fakeRequestDirectory{baseURL: server.URL}
	store := &fakeRequestStore{}
	requester := NewRequester(RequestDeps{
		Tickets:       minter,
		Sealer:        sealer,
		Directory:     directory,
		Store:         store,
		EncPrivateKey: ownPriv,
		PartyID:       requesterID,
		PollInterval:  time.Millisecond,
	})
	return &requestEnv{
		requester: requester,
		minter:    minter,
		directory: directory,
		store:     store,
		partner:   partner,
		archive:   archive,
	}
}

// TestRequestExportPostsSealedRequestWithResearcherHash covers T75
// assertion 1: a researcher request mints the export ticket carrying
// researcher_hash = UserRef(researcher email) and POSTs the sealed {study}
// request to the partner behind that ticket.
func TestRequestExportPostsSealedRequestWithResearcherHash(t *testing.T) {
	env := newRequestEnv(t)

	exportID, err := env.requester.RequestExport(context.Background(),
		"  Researcher@Example.COM  ", partnerID, exportStudyID)
	require.NoError(t, err, "the end-to-end request must succeed")
	assert.Equal(t, requestExportID, exportID, "the partner-issued export id must come back")

	auth, openErr, study, _ := env.partner.snapshot()
	assert.Equal(t, "Bearer "+requestTicket, auth, "the POST must carry the minted ticket")
	require.NoError(t, openErr, "the posted payload must open under the partner enc key")
	assert.Equal(t, exportStudyID, study, "the sealed request must carry the study")
	assert.Equal(t, partnerID, env.directory.party, "the partner party must be resolved")

	wantHash, err := identity.HashEmail(requestEmail, requestPepper)
	require.NoError(t, err)
	assert.Equal(t, partnerID, env.minter.to, "the ticket must target the partner")
	assert.Equal(t, exportScopePrefix+exportStudyID, env.minter.scope, "the scope must be export:<study>")
	assert.Equal(t, wantHash, env.minter.hash, "the ticket must carry researcher_hash = UserRef(email)")
}

// TestRequestExportDownloadsAndStoresArchiveForResearcher covers T75
// assertion 2: the requester polls the partner, downloads the sealed
// archive, opens it with this party's use:enc key and stores the
// plaintext archive for the researcher.
func TestRequestExportDownloadsAndStoresArchiveForResearcher(t *testing.T) {
	env := newRequestEnv(t)

	exportID, err := env.requester.RequestExport(context.Background(), requestEmail, partnerID, exportStudyID)
	require.NoError(t, err, "the end-to-end request must succeed")
	assert.Equal(t, requestExportID, exportID, "the export id must come back")

	_, _, _, polls := env.partner.snapshot()
	assert.GreaterOrEqual(t, polls, 2, "the requester must poll the status before downloading")

	require.NotEmpty(t, env.store.archive, "the archive must be stored for the researcher")
	assert.Equal(t, requestEmail, env.store.researcher, "the archive must be stored for the researcher")
	assert.Equal(t, exportStudyID, env.store.study, "the stored archive must name the study")
	assert.Equal(t, env.archive, env.store.archive, "the stored bytes must be the archive opened under the party enc key")
}
