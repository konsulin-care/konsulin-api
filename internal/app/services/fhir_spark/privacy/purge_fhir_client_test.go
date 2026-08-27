package privacy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"konsulin-service/internal/app/services/fhir_spark/base"
	"konsulin-service/internal/pkg/constvars"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestPurgeClient(baseURL string, logger *zap.Logger) *purgeFhirClient {
	// Mirror the production APP_FHIR_BASE_URL shape (trailing slash).
	if baseURL[len(baseURL)-1] != '/' {
		baseURL += "/"
	}
	return &purgeFhirClient{
		ResourceClient: base.New(baseURL, constvars.ResourcePatient, logger),
		pageSize:       2,
		baseFHIRURL:    baseURL,
	}
}

// emptySearchBundle is served for any registry type a test does not exercise.
// The implicit 200 status comes from writing the body; no explicit WriteHeader
// so handlers that already set one don't trigger superfluous-call warnings.
func emptySearchBundle(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/fhir+json")
	_, _ = w.Write([]byte(`{"resourceType":"Bundle","type":"searchset","total":0,"entry":[]}`))
}

func TestFindActivelyOwnedResources_ReturnsDeletableAcrossTypes(t *testing.T) {
	logger := zap.NewNop()
	var queries []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Path+"?"+r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		switch r.URL.Path {
		case "/QuestionnaireResponse":
			_, _ = w.Write([]byte(`{"resourceType":"Bundle","type":"searchset","entry":[{"resource":{"resourceType":"QuestionnaireResponse","id":"qr-1","author":{"reference":"Patient/pat-1"}}}]}`))
		case "/Consent":
			_, _ = w.Write([]byte(`{"resourceType":"Bundle","type":"searchset","entry":[{"resource":{"resourceType":"Consent","id":"c-1","status":"active","patient":{"reference":"Patient/pat-1"}}}]}`))
		case "/Appointment":
			_, _ = w.Write([]byte(`{"resourceType":"Bundle","type":"searchset","entry":[{"resource":{"resourceType":"Appointment","id":"apt-1","status":"booked","participant":[{"actor":{"reference":"Patient/pat-1"}}]}}]}`))
		default:
			emptySearchBundle(w)
		}
	}))
	defer server.Close()

	client := newTestPurgeClient(server.URL, logger)
	refs, err := client.FindActivelyOwnedResources(context.Background(), "pat-1")
	require.NoError(t, err)
	require.Len(t, refs, 3)

	byType := map[string]string{}
	for _, ref := range refs {
		byType[ref.ResourceType] = ref.ID
	}
	assert.Equal(t, "qr-1", byType["QuestionnaireResponse"])
	assert.Equal(t, "c-1", byType["Consent"])
	assert.Equal(t, "apt-1", byType["Appointment"])

	// Every registry rule must be queried exactly once with its ownership param.
	require.Len(t, queries, len(constvars.PurgeRules))
	for _, rule := range constvars.PurgeRules {
		found := false
		for _, q := range queries {
			if strings.HasPrefix(q, "/"+rule.ResourceType+"?") &&
				strings.Contains(q, rule.Params[0]+"=Patient%2Fpat-1") {
				found = true
				break
			}
		}
		assert.Truef(t, found, "missing query for %s", rule.ResourceType)
	}
}

func TestFindActivelyOwnedResources_SafetyPartitionExcludesShared(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/Communication" {
			_, _ = w.Write([]byte(`{
				"resourceType":"Bundle","type":"searchset","entry":[
					{"resource":{"resourceType":"Communication","id":"comm-1","status":"completed","sender":{"reference":"Patient/pat-1"}}},
					{"resource":{"resourceType":"Communication","id":"comm-2","status":"completed","sender":{"reference":"Patient/pat-1"},"recipient":[{"reference":"Patient/pat-2"}]}}
				]}`))
			return
		}
		emptySearchBundle(w)
	}))
	defer server.Close()

	client := newTestPurgeClient(server.URL, logger)
	refs, err := client.FindActivelyOwnedResources(context.Background(), "pat-1")
	require.NoError(t, err)
	require.Len(t, refs, 1, "shared referral communication must be excluded")
	assert.Equal(t, "Communication", refs[0].ResourceType)
	assert.Equal(t, "comm-1", refs[0].ID)
}

func TestFindActivelyOwnedResources_FollowsNextLink(t *testing.T) {
	logger := zap.NewNop()
	page := 0

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		switch r.URL.Path {
		case "/Observation", "/Observation/page2":
			page++
			if r.URL.Path == "/Observation" {
				// Mock FHIR bundle served by the httptest server — no user input, no HTML.
				// nosemgrep: no-printf-in-responsewriter, no-direct-write-to-responsewriter
				_, _ = w.Write([]byte(fmt.Sprintf(`{
				"resourceType":"Bundle","type":"searchset",
				"link":[
					{"relation":"self","url":"%s"},
					{"relation":"next","url":"%s/Observation/page2"}
				],
				"entry":[{"resource":{"resourceType":"Observation","id":"obs-1","status":"final","subject":{"reference":"Patient/pat-1"}}}]
			}`, server.URL, server.URL)))
				return
			}
			// Mock FHIR bundle served by the httptest server — no user input, no HTML.
			// nosemgrep: no-printf-in-responsewriter, no-direct-write-to-responsewriter
			_, _ = w.Write([]byte(fmt.Sprintf(`{
				"resourceType":"Bundle","type":"searchset",
				"link":[{"relation":"self","url":"%s/Observation/page2"}],
				"entry":[{"resource":{"resourceType":"Observation","id":"obs-2","status":"final","subject":{"reference":"Patient/pat-1"}}}]
			}`, server.URL)))
		default:
			emptySearchBundle(w)
		}
	}))
	defer server.Close()

	client := newTestPurgeClient(server.URL, logger)
	refs, err := client.FindActivelyOwnedResources(context.Background(), "pat-1")
	require.NoError(t, err)
	require.Len(t, refs, 2)
	assert.Equal(t, "obs-1", refs[0].ID)
	assert.Equal(t, "obs-2", refs[1].ID)
	assert.Equal(t, 2, page, "expected the next link to be followed")
}

func TestFindActivelyOwnedResources_DedupsById(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/Condition" {
			_, _ = w.Write([]byte(`{
				"resourceType":"Bundle","type":"searchset","entry":[
					{"resource":{"resourceType":"Condition","id":"cond-1","clinicalStatus":{"coding":[{"code":"active"}]},"subject":{"reference":"Patient/pat-1"}}},
					{"resource":{"resourceType":"Condition","id":"cond-1","clinicalStatus":{"coding":[{"code":"active"}]},"subject":{"reference":"Patient/pat-1"}}}
				]}`))
			return
		}
		emptySearchBundle(w)
	}))
	defer server.Close()

	client := newTestPurgeClient(server.URL, logger)
	refs, err := client.FindActivelyOwnedResources(context.Background(), "pat-1")
	require.NoError(t, err)
	require.Len(t, refs, 1, "duplicate entries must be deduped")
	assert.Equal(t, "cond-1", refs[0].ID)
}

func TestFindActivelyOwnedResources_EmptyWhenNothingOwned(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		emptySearchBundle(w)
	}))
	defer server.Close()

	client := newTestPurgeClient(server.URL, logger)
	refs, err := client.FindActivelyOwnedResources(context.Background(), "pat-1")
	require.NoError(t, err)
	assert.Empty(t, refs)
}

func TestStripPatientToShell_PreservesMetaAndDeactivates(t *testing.T) {
	logger := zap.NewNop()
	var putBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		switch r.Method {
		case http.MethodGet:
			assert.Equal(t, "/Patient/pat-1", r.URL.Path)
			_, _ = w.Write([]byte(`{
				"resourceType":"Patient","id":"pat-1","active":true,
				"name":[{"use":"official","family":"Smith"}],
				"meta":{"versionId":"7","lastUpdated":"2025-01-15T10:00:00Z"}
			}`))
		case http.MethodPut:
			assert.Equal(t, "/Patient/pat-1", r.URL.Path)
			body, _ := io.ReadAll(r.Body)
			putBody = string(body)
			_, _ = w.Write([]byte(`{"resourceType":"Patient","id":"pat-1"}`))
		}
	}))
	defer server.Close()

	client := newTestPurgeClient(server.URL, logger)
	require.NoError(t, client.StripPatientToShell(context.Background(), "pat-1"))
	assert.JSONEq(t, `{
		"resourceType":"Patient",
		"id":"pat-1",
		"active":false,
		"meta":{"versionId":"7","lastUpdated":"2025-01-15T10:00:00Z"}
	}`, putBody, "shell must keep meta and set active=false")
}

func TestStripPatientToShell_NotFoundIsNoOp(t *testing.T) {
	logger := zap.NewNop()
	putCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			putCalls++
		}
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"resourceType":"OperationOutcome","issue":[{"severity":"error","code":"not-found","diagnostics":"patient not found"}]}`))
	}))
	defer server.Close()

	client := newTestPurgeClient(server.URL, logger)
	require.NoError(t, client.StripPatientToShell(context.Background(), "pat-1"))
	assert.Zero(t, putCalls, "a missing patient must not trigger a PUT")
}
