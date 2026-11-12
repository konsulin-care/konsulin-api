package federation

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"konsulin-service/internal/app/contracts"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// pepperListing is the active federation-pepper webhook row the fetch looks
// for (BK-PEPPER-001).
var pepperListing = contracts.Listing{
	ListingID:       "lst-pepper-001",
	ProviderPartyID: "party-konsulin",
	Name:            pepperListingName,
	Invocation:      "webhook",
	Price:           0,
}

// pepperSyncReply is the exact reply bytes the fake Sync returns: whatever
// the relay hands back must land on disk verbatim.
var pepperSyncReply = []byte("eyJhbGciOiJFQ0RSIiwidHlwIjoiSldUIn0.ciphertext.signature\n")

// pepperDirectory is a contracts.ListingDirectory double serving a
// configured webhook listing set and recording the discovery read.
type pepperDirectory struct {
	listed     []contracts.Listing
	listErr    error
	listCalls  int
	findCalls  int
	enlistCall int
}

var _ contracts.ListingDirectory = (*pepperDirectory)(nil)

// FindListing records the call and reports no match — the fetch discovers
// through ListWebhookListings only.
// Parameters: ctx bounds the lookup; provider and name are the arguments.
// Returns nil and contracts.ErrListingNotFound. Never panics.
func (d *pepperDirectory) FindListing(context.Context, string, string) (*contracts.Listing, error) {
	d.findCalls++
	return nil, contracts.ErrListingNotFound
}

// ListWebhookListings records the discovery read and serves the configured
// rows.
// Parameters: ctx bounds the read.
// Returns the configured listings and the configured error. Never panics.
func (d *pepperDirectory) ListWebhookListings(context.Context) ([]contracts.Listing, error) {
	d.listCalls++
	return d.listed, d.listErr
}

// Enlist records the call and reports no created listing — unused by the
// fetch.
// Parameters: ctx bounds the call; req is the listing to enlist.
// Returns "" and contracts.ErrGatewayNotConfigured. Never panics.
func (d *pepperDirectory) Enlist(context.Context, contracts.EnlistRequest) (string, error) {
	d.enlistCall++
	return "", contracts.ErrGatewayNotConfigured
}

// pepperSyncCall records one Sync invocation's routing arguments.
type pepperSyncCall struct {
	to      string
	service string
}

// pepperWebhooks is a contracts.WebhookClient double returning a canned
// reply and recording every Sync invocation.
type pepperWebhooks struct {
	reply []byte
	err   error
	syncs []pepperSyncCall
}

var _ contracts.WebhookClient = (*pepperWebhooks)(nil)

// Enqueue records nothing and reports success — the fetch invokes sync only.
// Parameters: ctx bounds the call; to, service, userRef and data are the
// invocation arguments.
// Returns nil. Never panics.
func (w *pepperWebhooks) Enqueue(context.Context, string, string, string, []byte) error {
	return nil
}

// Sync records the routing arguments and returns the canned reply.
// Parameters: ctx bounds the call; to, service, userRef and data are the
// invocation arguments.
// Returns the canned reply and the canned error. Never panics.
func (w *pepperWebhooks) Sync(_ context.Context, to, service, _ string, _ []byte) ([]byte, error) {
	w.syncs = append(w.syncs, pepperSyncCall{to: to, service: service})
	return w.reply, w.err
}

// Status reports the delivery state — unused by the fetch.
// Parameters: ctx bounds the read; id is the enqueued webhook id.
// Returns "" and contracts.ErrGatewayNotConfigured. Never panics.
func (w *pepperWebhooks) Status(context.Context, string) (string, error) {
	return "", contracts.ErrGatewayNotConfigured
}

// TestEnsureSealedPepper_FetchesAndStoresVerbatim covers the T45 assertion:
// without a sealed file the pepper is fetched through the unique active
// federation-pepper listing and the reply bytes are stored verbatim at
// <APP_DATA_DIR>/federation-pepper.jwe with mode 0600.
// Parameters: t is the test harness.
// Returns nothing; fails on any deviation.
func TestEnsureSealedPepper_FetchesAndStoresVerbatim(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("APP_DATA_DIR", dataDir)

	directory := &pepperDirectory{listed: []contracts.Listing{
		{ListingID: "lst-other", ProviderPartyID: "party-x", Name: "send-magiclink", Invocation: "webhook"},
		pepperListing,
	}}
	hooks := &pepperWebhooks{reply: pepperSyncReply}
	core, logs := observer.New(zap.WarnLevel)

	EnsureSealedPepper(context.Background(), directory, hooks, zap.New(core))

	stored, err := os.ReadFile(filepath.Join(dataDir, pepperFileName))
	require.NoError(t, err, "the fetched sealed pepper must be stored")
	assert.Equal(t, pepperSyncReply, stored, "the reply bytes must be stored verbatim")
	info, err := os.Stat(filepath.Join(dataDir, pepperFileName))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "the sealed file must be mode 0600")

	require.Len(t, hooks.syncs, 1, "exactly one sync invocation must run")
	assert.Equal(t, pepperSyncCall{to: "party-konsulin", service: pepperListingName},
		hooks.syncs[0], "the sync must target the listing provider and name")
	assert.Equal(t, 1, directory.listCalls, "the listing discovery must run once")
	assert.Zero(t, logs.Len(), "a successful fetch must not log a warning")
}

// TestEnsureSealedPepper_ExistingSealedFilePreventsFetch covers the T45
// assertion: an existing sealed file prevents any listing read or sync
// invocation and leaves the stored bytes untouched.
// Parameters: t is the test harness.
// Returns nothing; fails on any deviation.
func TestEnsureSealedPepper_ExistingSealedFilePreventsFetch(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("APP_DATA_DIR", dataDir)
	path := filepath.Join(dataDir, pepperFileName)
	existing := []byte("already-sealed.jwe")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	require.NoError(t, os.WriteFile(path, existing, 0o600))

	directory := &pepperDirectory{listed: []contracts.Listing{pepperListing}}
	hooks := &pepperWebhooks{reply: pepperSyncReply}

	EnsureSealedPepper(context.Background(), directory, hooks, zap.NewNop())

	stored, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, existing, stored, "the existing sealed file must stay untouched")
	assert.Zero(t, directory.listCalls, "an existing sealed file must prevent the listing read")
	assert.Empty(t, hooks.syncs, "an existing sealed file must prevent any fetch")
}

// TestEnsureSealedPepper_ZeroOrMultipleListingsWriteNoFile covers the T45
// assertion: zero or multiple federation-pepper listings log a warning,
// write no file and keep the APP_PEPPER fallback loadable.
// Parameters: t is the test harness.
// Returns nothing; fails on any deviation.
func TestEnsureSealedPepper_ZeroOrMultipleListingsWriteNoFile(t *testing.T) {
	cases := map[string][]contracts.Listing{
		"zero listings":     {},
		"no pepper row":     {{ListingID: "lst-other", ProviderPartyID: "party-x", Name: "send-magiclink", Invocation: "webhook"}},
		"multiple listings": {pepperListing, {ListingID: "lst-pepper-002", ProviderPartyID: "party-y", Name: pepperListingName, Invocation: "webhook"}},
	}
	for name, listed := range cases {
		t.Run(name, func(t *testing.T) {
			dataDir := t.TempDir()
			fallback := []byte("0123456789abcdef0123456789abcdef")
			t.Setenv("APP_DATA_DIR", dataDir)
			t.Setenv("APP_PEPPER", string(fallback))

			directory := &pepperDirectory{listed: listed}
			hooks := &pepperWebhooks{reply: pepperSyncReply}
			core, logs := observer.New(zap.WarnLevel)

			EnsureSealedPepper(context.Background(), directory, hooks, zap.New(core))

			_, err := os.Stat(filepath.Join(dataDir, pepperFileName))
			assert.ErrorIs(t, err, os.ErrNotExist, "no unique listing must write no file")
			assert.Equal(t, 1, directory.listCalls, "the listing discovery must run")
			assert.Empty(t, hooks.syncs, "without exactly one listing nothing must be invoked")
			assert.NotZero(t, logs.Len(), "the fallback must be logged")

			got, err := LoadPepper()
			require.NoError(t, err, "the APP_PEPPER fallback must stay loadable")
			assert.Equal(t, fallback, got, "the APP_PEPPER fallback must be kept")
		})
	}
}
