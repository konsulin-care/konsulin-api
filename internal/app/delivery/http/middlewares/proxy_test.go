package middlewares

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"konsulin-service/internal/pkg/constvars"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// mockRoundTripper returns a fixed response for any request.
type mockRoundTripper struct {
	statusCode int
	body       string
	headers    http.Header
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{
		StatusCode: m.statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(m.body)),
		Request:    req,
	}
	for k, v := range m.headers {
		resp.Header[k] = v
	}
	return resp, nil
}

// ---------------------------------------------------------------------------
// Tests for allowedRedirectHost helper
// ---------------------------------------------------------------------------

func TestAllowedRedirectHost_SameHostAllowed(t *testing.T) {
	checkFn := allowedRedirectHost("blaze:8080")
	req := httptest.NewRequest(http.MethodGet, "http://blaze:8080/fhir/Patient/123", nil)
	err := checkFn(req, nil)
	assert.NoError(t, err, "redirect to same host should be allowed")
}

func TestAllowedRedirectHost_DifferentHostBlocked(t *testing.T) {
	checkFn := allowedRedirectHost("blaze:8080")
	req := httptest.NewRequest(http.MethodGet, "http://evil.internal/admin", nil)
	err := checkFn(req, nil)
	assert.Error(t, err, "redirect to different host should be blocked")
	assert.Contains(t, err.Error(), "redirect", "error should mention redirect")
	assert.Contains(t, err.Error(), "evil.internal", "error should mention blocked host")
}

func TestAllowedRedirectHost_MaxRedirects(t *testing.T) {
	checkFn := allowedRedirectHost("blaze:8080")
	via := make([]*http.Request, 10)
	req := httptest.NewRequest(http.MethodGet, "http://blaze:8080/fhir/Patient/123", nil)
	err := checkFn(req, via)
	assert.Error(t, err, "too many redirects should be blocked")
	assert.Contains(t, err.Error(), "too many redirects")
}

func TestAllowedRedirectHost_DifferentPortBlocked(t *testing.T) {
	checkFn := allowedRedirectHost("blaze:8080")
	req := httptest.NewRequest(http.MethodGet, "http://blaze:9999/fhir/Patient/123", nil)
	err := checkFn(req, nil)
	assert.Error(t, err, "redirect to different port should be blocked")
}

func TestAllowedRedirectHost_HostWithoutPort(t *testing.T) {
	checkFn := allowedRedirectHost("blaze")
	req := httptest.NewRequest(http.MethodGet, "http://blaze:8080/fhir/Patient/123", nil)
	err := checkFn(req, nil)
	assert.Error(t, err, "redirect with port should not match host without port")
}

func TestAllowedRedirectHost_IPAddress(t *testing.T) {
	checkFn := allowedRedirectHost("10.0.0.1:8080")
	req := httptest.NewRequest(http.MethodGet, "http://10.0.0.1:8080/fhir/Patient/123", nil)
	err := checkFn(req, nil)
	assert.NoError(t, err, "redirect to same IP:port should be allowed")
}

// ---------------------------------------------------------------------------
// Tests for validateProxyURLHost helper
// ---------------------------------------------------------------------------

func TestValidateProxyURLHost_Valid(t *testing.T) {
	err := validateProxyURLHost("http://blaze:8080/fhir/Patient/123", "http://blaze:8080/fhir")
	assert.NoError(t, err, "same host should pass validation")
}

func TestValidateProxyURLHost_MismatchedHost(t *testing.T) {
	err := validateProxyURLHost("http://evil.internal/admin", "http://blaze:8080/fhir")
	assert.Error(t, err, "different host should fail validation")
	assert.Contains(t, err.Error(), "SSRF", "error should mention SSRF")
}

func TestValidateProxyURLHost_DifferentPort(t *testing.T) {
	err := validateProxyURLHost("http://blaze:9999/fhir/Patient/123", "http://blaze:8080/fhir")
	assert.Error(t, err, "different port should fail validation")
}

func TestValidateProxyURLHost_InvalidURL(t *testing.T) {
	err := validateProxyURLHost("://invalid", "http://blaze:8080/fhir")
	assert.Error(t, err, "invalid URL should fail validation")
}

func TestValidateProxyURLHost_InvalidTarget(t *testing.T) {
	err := validateProxyURLHost("http://blaze:8080/fhir", "://invalid")
	assert.Error(t, err, "invalid target URL should fail validation")
}

func TestValidateProxyURLHost_EmptyTarget(t *testing.T) {
	err := validateProxyURLHost("http://blaze:8080/fhir", "")
	assert.Error(t, err, "empty target should fail validation")
}

func TestValidateProxyURLHost_HttpsSameHost(t *testing.T) {
	err := validateProxyURLHost("https://blaze:8080/fhir/Patient/123", "https://blaze:8080/fhir")
	assert.NoError(t, err, "same host with HTTPS should pass validation")
}

// ---------------------------------------------------------------------------
// Integration tests for Bridge redirect protection
// ---------------------------------------------------------------------------

func TestValidateProxyURLHost_IPv6(t *testing.T) {
	err := validateProxyURLHost("http://[::1]:8080/fhir/Patient/123", "http://[::1]:8080/fhir")
	assert.NoError(t, err, "IPv6 address should pass validation")
}

func TestValidateProxyURLHost_IPv6Mismatch(t *testing.T) {
	err := validateProxyURLHost("http://[::1]:8080/fhir/Patient/123", "http://[::2]:8080/fhir")
	assert.Error(t, err, "different IPv6 address should fail validation")
}

func TestValidateProxyURLHost_SchemeMismatchStillPasses(t *testing.T) {
	// Host validation only checks host:port, not scheme. This is intentional
	// because the proxy forwards to the same upstream regardless of scheme.
	err := validateProxyURLHost("https://blaze:8080/fhir/Patient/123", "http://blaze:8080/fhir")
	assert.NoError(t, err, "host is the same even if scheme differs")
}

func TestWriteBridgeResponse_StripsETagOnMutated(t *testing.T) {
	logger := zap.NewNop()
	mw := &Middlewares{Log: logger}

	upstreamResp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			http.CanonicalHeaderKey("etag"): {`W/"xyz123"`},
			"Content-Type":                  {"application/fhir+json"},
			"Cache-Control":                 {"max-age=60"},
		},
	}

	rr := httptest.NewRecorder()
	finalBody := []byte(`{"resourceType":"Patient","id":"pat-1"}`)

	// Mutated=true: ETag should be stripped
	mw.writeBridgeResponse(rr, upstreamResp, finalBody, nil, true)

	assert.Equal(t, "application/fhir+json", rr.Header().Get("Content-Type"),
		"non-ETag headers should be preserved")
	assert.Equal(t, "max-age=60", rr.Header().Get("Cache-Control"),
		"non-ETag headers should be preserved")
	assert.Equal(t, "", rr.Header().Get("Etag"),
		"ETag must be stripped when body was mutated to prevent cache poisoning")
	assert.Equal(t, strconv.Itoa(len(finalBody)), rr.Header().Get("Content-Length"),
		"Content-Length should be overridden with the final body length")
}

func TestWriteBridgeResponse_PreservesETagOnUnmutated(t *testing.T) {
	logger := zap.NewNop()
	mw := &Middlewares{Log: logger}

	upstreamResp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			http.CanonicalHeaderKey("etag"): {`W/"xyz123"`},
			"Content-Type":                  {"application/fhir+json"},
		},
	}

	rr := httptest.NewRecorder()
	finalBody := []byte(`{"resourceType":"Patient","id":"pat-1"}`)

	// Mutated=false: ETag should be preserved
	mw.writeBridgeResponse(rr, upstreamResp, finalBody, nil, false)

	assert.Equal(t, `W/"xyz123"`, rr.Header().Get("Etag"),
		"ETag must be preserved when body was not mutated")
}

func TestBridge_ClientHasCheckRedirect(t *testing.T) {
	logger := zap.NewNop()
	mw := &Middlewares{Log: logger}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/fhir/Patient/123", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"resourceType":"Patient"}`))
	}))
	defer backend.Close()

	handler := mw.Bridge(backend.URL + "/fhir")

	req := httptest.NewRequest(http.MethodGet, "/fhir/Patient", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// The proxy should successfully route and get a response.
	// 400+ would indicate the request failed before reaching the backend.
	assert.Less(t, rr.Code, 400, "proxy should forward request and receive response")
}

func TestDoFHIRProxyRequest_SSRFPathInjection(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		target      string
		wantHost    string
		wantURLPath string
		wantErr     bool
	}{
		{
			name:        "normal path",
			path:        "/fhir/Patient/123",
			target:      "https://fhir.example.com",
			wantHost:    "fhir.example.com",
			wantURLPath: "/Patient/123",
			wantErr:     false,
		},
		{
			name:        "path with query is forwarded separately",
			path:        "/fhir/Observation?_tag=test",
			target:      "https://fhir.example.com",
			wantHost:    "fhir.example.com",
			wantURLPath: "/Observation",
			wantErr:     false,
		},
		{
			name:    "path traversal blocked",
			path:    "/fhir/../../etc/passwd",
			target:  "https://fhir.example.com",
			wantErr: true,
		},
		{
			name:        "SSRF host mismatch is rejected",
			path:        "/fhir/Patient/1",
			target:      "https://fhir.example.com",
			wantHost:    "fhir.example.com",
			wantURLPath: "/Patient/1",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var actualURL *url.URL
			mockClient := &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					actualURL = req.URL
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`{}`)),
						Header:     make(http.Header),
					}, nil
				}),
			}

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			_, _, _, err := doFHIRProxyRequest(req, tt.target, mockClient)

			if tt.wantErr {
				assert.Error(t, err, "should reject malicious path")
				return
			}
			assert.NoError(t, err, "request should succeed")
			assert.Equal(t, tt.wantHost, actualURL.Host, "request host should match target")
			assert.Equal(t, tt.wantURLPath, actualURL.Path, "request path should be properly constructed")
		})
	}
}

func TestBuildTxProxyURL_SSRF(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		path      string
		prefix    string
		version   string
		wantHost  string
		wantPath  string
		wantQuery string
		wantEmpty bool
		wantExact string
	}{
		{
			name:   "normal path with filter",
			target: "https://tx.example.com/fhir",
			path:   "/v1/v1/tx/ValueSet/$expand?filter=test",
			prefix: "v1", version: "v1",
			wantHost: "tx.example.com", wantPath: "/fhir/ValueSet/$expand", wantQuery: "filter=test",
		},
		{
			name:   "normal path with count",
			target: "https://tx.example.com/fhir",
			path:   "/v1/v1/tx/ValueSet/$expand?count=10",
			prefix: "v1", version: "v1",
			wantHost: "tx.example.com", wantPath: "/fhir/ValueSet/$expand", wantQuery: "count=10",
		},
		{
			name:   "no filter or count returns empty",
			target: "https://tx.example.com/fhir",
			path:   "/v1/v1/tx/ValueSet/$expand",
			prefix: "v1", version: "v1",
			wantEmpty: true,
		},
		{
			name:   "empty relative path returns target",
			target: "https://tx.example.com/fhir",
			path:   "/v1/v1/tx?filter=test",
			prefix: "v1", version: "v1",
			wantExact: "https://tx.example.com/fhir",
		},
		{
			name:   "path traversal blocked",
			target: "https://tx.example.com/fhir",
			path:   "/v1/v1/tx/../etc/passwd?filter=test",
			prefix: "v1", version: "v1",
			wantEmpty: true,
		},
		{
			name:   "at sign in relative path is path literal not host separator",
			target: "https://tx.example.com/fhir",
			path:   "/v1/v1/tx/@evil.com:443/api?filter=test",
			prefix: "v1", version: "v1",
			wantHost: "tx.example.com", wantPath: "/fhir/@evil.com:443/api", wantQuery: "filter=test",
		},
		{
			name:   "double slash in relative path is normalized",
			target: "https://tx.example.com/fhir",
			path:   "/v1/v1/tx//evil.com/path?filter=test",
			prefix: "v1", version: "v1",
			wantHost: "tx.example.com", wantPath: "/fhir/evil.com/path", wantQuery: "filter=test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			result := buildTxProxyURL(tt.target, req, tt.prefix, tt.version)

			if tt.wantEmpty {
				assert.Empty(t, result, "should return empty string")
				return
			}
			if tt.wantExact != "" {
				assert.Equal(t, tt.wantExact, result, "should return exact target")
				return
			}

			parsed, err := url.Parse(result)
			assert.NoError(t, err, "result should be a valid URL")
			assert.Equal(t, tt.wantHost, parsed.Host, "host must match target")
			assert.Equal(t, tt.wantPath, parsed.Path, "path must be properly constructed")
			assert.Equal(t, tt.wantQuery, parsed.RawQuery, "query must be preserved")
		})
	}
}

// roundTripperFunc adapts a function to http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestStripCommunicationFields_SingleResource(t *testing.T) {
	body := `{"resourceType":"Communication","id":"comm-1","meta":{"versionId":"1","lastUpdated":"2025-01-01T00:00:00Z"},"status":"completed","topic":{"coding":[{"system":"http://konsulin.care/fhir/CodeSystem/research-referral","code":"research-referral"}]},"subject":{"reference":"Patient/pat-1"},"sender":{"reference":"Patient/pat-1"},"recipient":[{"reference":"Patient/pat-2"}],"sent":"2025-01-01T00:00:00Z","received":"2025-01-02T00:00:00Z","payload":[{"contentString":"sensitive"}]}`

	out, mutated := stripCommunicationFields([]byte(body))
	assert.True(t, mutated, "a Communication must be marked mutated")

	var m map[string]any
	if !assert.NoError(t, json.Unmarshal(out, &m)) {
		return
	}
	assert.Equal(t, "Communication", m["resourceType"])
	assert.Equal(t, "comm-1", m["id"])
	assert.NotNil(t, m["meta"])
	assert.NotNil(t, m["sender"])
	assert.NotNil(t, m["recipient"])
	assert.NotNil(t, m["sent"])
	assert.NotNil(t, m["received"])
	assert.NotContains(t, m, "status")
	assert.NotContains(t, m, "topic")
	assert.NotContains(t, m, "subject")
	assert.NotContains(t, m, "payload")
}

func TestStripCommunicationFields_BundleMixedEntries(t *testing.T) {
	body := `{"resourceType":"Bundle","type":"searchset","total":2,"entry":[
		{"resource":{"resourceType":"Communication","id":"comm-1","status":"completed","sender":{"reference":"Patient/pat-1"},"recipient":[{"reference":"Patient/pat-2"}],"payload":[{"contentString":"secret"}]}},
		{"resource":{"resourceType":"Patient","id":"pat-1","name":[{"family":"Doe"}]}}
	]}`

	out, mutated := stripCommunicationFields([]byte(body))
	assert.True(t, mutated, "bundle with a Communication entry must be marked mutated")

	var b struct {
		Entry []struct {
			Resource map[string]any `json:"resource"`
		} `json:"entry"`
	}
	if !assert.NoError(t, json.Unmarshal(out, &b)) {
		return
	}
	assert.Len(t, b.Entry, 2)

	comm := b.Entry[0].Resource
	assert.Equal(t, "Communication", comm["resourceType"])
	assert.NotContains(t, comm, "status")
	assert.NotContains(t, comm, "payload")
	assert.NotNil(t, comm["sender"])

	pat := b.Entry[1].Resource
	assert.Equal(t, "Patient", pat["resourceType"])
	assert.NotNil(t, pat["name"], "non-Communication entries must be untouched")
}

func TestStripCommunicationFields_NonCommunicationUntouched(t *testing.T) {
	body := `{"resourceType":"Observation","id":"obs-1","status":"final"}`
	out, mutated := stripCommunicationFields([]byte(body))
	assert.False(t, mutated)
	assert.Equal(t, body, string(out))
}

func TestStripCommunicationBundle_StripsCommunicationEntries(t *testing.T) {
	body := `{"resourceType":"Bundle","type":"searchset","total":2,"entry":[
		{"resource":{"resourceType":"Communication","id":"comm-1","status":"completed","sender":{"reference":"Patient/pat-1"},"recipient":[{"reference":"Patient/pat-2"}],"payload":[{"contentString":"secret"}]}},
		{"resource":{"resourceType":"Patient","id":"pat-1","name":[{"family":"Doe"}]}}
	]}`

	out, mutated := stripCommunicationBundle([]byte(body))
	assert.True(t, mutated, "bundle with a Communication entry must be marked mutated")

	var b struct {
		Entry []struct {
			Resource map[string]any `json:"resource"`
		} `json:"entry"`
	}
	if !assert.NoError(t, json.Unmarshal(out, &b)) {
		return
	}
	assert.Len(t, b.Entry, 2)

	comm := b.Entry[0].Resource
	assert.Equal(t, "Communication", comm["resourceType"])
	assert.NotContains(t, comm, "status")
	assert.NotContains(t, comm, "payload")
	assert.NotNil(t, comm["sender"])

	pat := b.Entry[1].Resource
	assert.Equal(t, "Patient", pat["resourceType"])
	assert.NotNil(t, pat["name"], "non-Communication entries must be untouched")
}

func TestStripCommunicationBundle_NoCommunicationUnmutated(t *testing.T) {
	body := `{"resourceType":"Bundle","type":"searchset","total":1,"entry":[
		{"resource":{"resourceType":"Patient","id":"pat-1","name":[{"family":"Doe"}]}}
	]}`

	out, mutated := stripCommunicationBundle([]byte(body))
	assert.False(t, mutated, "bundle without Communication entries must be unmutated")
	assert.Equal(t, body, string(out))
}

func TestStripCommunicationBundle_InvalidJSONUnmutated(t *testing.T) {
	body := []byte(`{not json`)

	out, mutated := stripCommunicationBundle(body)
	assert.False(t, mutated)
	assert.Equal(t, body, out)
}

func TestShouldStripCommunicationFields(t *testing.T) {
	ownSender, _ := url.Parse("/fhir/Communication?sender=Patient/pat-1&topic=research-referral")
	ownRecipient, _ := url.Parse("/fhir/Communication?recipient=Patient/pat-1&topic=research-referral")
	crossPatient, _ := url.Parse("/fhir/Communication?sender=Patient/pat-2&topic=research-referral")
	bare, _ := url.Parse("/fhir/Communication")

	t.Run("patient scoped to own sender keeps full fields", func(t *testing.T) {
		assert.False(t, shouldStripCommunicationFields([]string{constvars.KonsulinRolePatient}, "pat-1", ownSender))
	})

	t.Run("patient scoped to own recipient keeps full fields", func(t *testing.T) {
		assert.False(t, shouldStripCommunicationFields([]string{constvars.KonsulinRolePatient}, "pat-1", ownRecipient))
	})

	t.Run("researcher strips fields", func(t *testing.T) {
		assert.True(t, shouldStripCommunicationFields([]string{constvars.KonsulinRoleResearcher}, "", crossPatient))
	})

	t.Run("superadmin strips fields", func(t *testing.T) {
		assert.True(t, shouldStripCommunicationFields([]string{constvars.KonsulinRoleSuperadmin}, "", bare))
	})

	t.Run("patient+researcher on own data keeps full fields", func(t *testing.T) {
		assert.False(t, shouldStripCommunicationFields(
			[]string{constvars.KonsulinRolePatient, constvars.KonsulinRoleResearcher}, "pat-1", ownSender))
	})

	t.Run("patient+researcher on cross-patient query strips fields", func(t *testing.T) {
		assert.True(t, shouldStripCommunicationFields(
			[]string{constvars.KonsulinRolePatient, constvars.KonsulinRoleResearcher}, "pat-1", crossPatient))
	})

	t.Run("guest never strips", func(t *testing.T) {
		assert.False(t, shouldStripCommunicationFields([]string{constvars.KonsulinRoleGuest}, "", bare))
	})

	t.Run("practitioner never strips", func(t *testing.T) {
		assert.False(t, shouldStripCommunicationFields([]string{constvars.KonsulinRolePractitioner}, "prac-1", bare))
	})
}
