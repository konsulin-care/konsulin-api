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

func TestGetPatientEverything_SinglePage(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/Patient/pat-1/$everything", r.URL.Path)
		assert.Equal(t, "2", r.URL.Query().Get("_count"))
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"resourceType":"Bundle","type":"searchset","total":2,
			"link":[{"relation":"self","url":"http://blaze/fhir/Patient/pat-1/$everything?_count=2"}],
			"entry":[
				{"resource":{"resourceType":"QuestionnaireResponse","id":"qr-1","author":{"reference":"Patient/pat-1"}}},
				{"resource":{"resourceType":"Communication","id":"comm-1","sender":{"reference":"Patient/pat-1"}}}
			]
		}`))
	}))
	defer server.Close()

	client := newTestPurgeClient(server.URL, logger)
	refs, err := client.GetPatientEverything(context.Background(), "pat-1")
	require.NoError(t, err)
	require.Len(t, refs, 2)
	assert.Equal(t, "QuestionnaireResponse", refs[0].ResourceType)
	assert.Equal(t, "qr-1", refs[0].ID)
	assert.Equal(t, "Communication", refs[1].ResourceType)
	assert.Equal(t, "comm-1", refs[1].ID)
}

func TestGetPatientEverything_FollowsNextLink(t *testing.T) {
	logger := zap.NewNop()
	page := 0

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		if page == 1 {
			_, _ = w.Write([]byte(fmt.Sprintf(`{
				"resourceType":"Bundle","type":"searchset",
				"link":[
					{"relation":"self","url":"%s"},
					{"relation":"next","url":"%s/next"}
				],
				"entry":[{"resource":{"resourceType":"Condition","id":"cond-1"}}]
			}`, server.URL, server.URL)))
			return
		}
		_, _ = w.Write([]byte(fmt.Sprintf(`{
			"resourceType":"Bundle","type":"searchset",
			"link":[{"relation":"self","url":"%s/next"}],
			"entry":[{"resource":{"resourceType":"Observation","id":"obs-1"}}]
		}`, server.URL)))
	}))
	defer server.Close()

	client := newTestPurgeClient(server.URL, logger)
	refs, err := client.GetPatientEverything(context.Background(), "pat-1")
	require.NoError(t, err)
	require.Len(t, refs, 2)
	assert.Equal(t, "Condition", refs[0].ResourceType)
	assert.Equal(t, "Observation", refs[1].ResourceType)
	assert.Equal(t, 2, page, "expected the next link to be followed")
}

func TestGetPatientEverything_EmptyBundle(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"resourceType":"Bundle","type":"searchset","total":0,"entry":[]}`))
	}))
	defer server.Close()

	client := newTestPurgeClient(server.URL, logger)
	refs, err := client.GetPatientEverything(context.Background(), "pat-1")
	require.NoError(t, err)
	assert.Empty(t, refs)
}

func TestGetPatientEverything_NotFoundIsNoOp(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"resourceType":"OperationOutcome","issue":[{"severity":"error","code":"not-found","diagnostics":"Patient pat-1 not found"}]}`))
	}))
	defer server.Close()

	client := newTestPurgeClient(server.URL, logger)
	refs, err := client.GetPatientEverything(context.Background(), "pat-1")
	require.NoError(t, err, "a missing/already-purged patient must not error")
	assert.Empty(t, refs)
}

func TestDeletePatient_Success(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/Patient/pat-1", r.URL.Path)
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"resourceType":"OperationOutcome","issue":[{"severity":"information","code":"informational","diagnostics":"Deleted 1 resource"}]}`))
	}))
	defer server.Close()

	client := newTestPurgeClient(server.URL, logger)
	assert.NoError(t, client.DeletePatient(context.Background(), "pat-1"))
}

func TestDeletePatient_Error(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"resourceType":"OperationOutcome","issue":[{"severity":"error","code":"exception","diagnostics":"delete failed"}]}`))
	}))
	defer server.Close()

	client := newTestPurgeClient(server.URL, logger)
	assert.Error(t, client.DeletePatient(context.Background(), "pat-1"))
}

func TestStripPatientPII_WritesEmptyShell(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/Patient/pat-1", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		assert.JSONEq(t, `{"resourceType":"Patient","id":"pat-1"}`, string(body), "shell must carry no PII")
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"resourceType":"Patient","id":"pat-1"}`))
	}))
	defer server.Close()

	client := newTestPurgeClient(server.URL, logger)
	assert.NoError(t, client.StripPatientPII(context.Background(), "pat-1"))
}

func TestFindCommunicationRefs_SenderAndRecipient(t *testing.T) {
	logger := zap.NewNop()
	var paths []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		if strings.Contains(r.URL.RawQuery, "sender=") {
			_, _ = w.Write([]byte(`{"resourceType":"Bundle","type":"searchset","entry":[{"resource":{"resourceType":"Communication","id":"comm-1"}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"resourceType":"Bundle","type":"searchset","entry":[{"resource":{"resourceType":"Communication","id":"comm-1"}},{"resource":{"resourceType":"Communication","id":"comm-2"}}]}`))
	}))
	defer server.Close()

	client := newTestPurgeClient(server.URL, logger)
	refs, err := client.FindCommunicationRefs(context.Background(), "pat-1")
	require.NoError(t, err)
	require.Len(t, refs, 2, "sender and recipient hits must be deduped")
	assert.Equal(t, "Communication", refs[0].ResourceType)
	assert.Contains(t, []string{"comm-1", "comm-2"}, refs[0].ID)
	assert.Contains(t, []string{"comm-1", "comm-2"}, refs[1].ID)
	assert.Len(t, paths, 2, "both sender and recipient queries must run")
}

func TestFindQuestionnaireResponseRefsByAuthor(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "author=Patient%2Fpat-1")
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"resourceType":"Bundle","type":"searchset","entry":[{"resource":{"resourceType":"QuestionnaireResponse","id":"qr-1"}}]}`))
	}))
	defer server.Close()

	client := newTestPurgeClient(server.URL, logger)
	refs, err := client.FindQuestionnaireResponseRefsByAuthor(context.Background(), "pat-1")
	require.NoError(t, err)
	require.Len(t, refs, 1)
	assert.Equal(t, "QuestionnaireResponse", refs[0].ResourceType)
	assert.Equal(t, "qr-1", refs[0].ID)
}

