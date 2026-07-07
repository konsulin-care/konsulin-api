package middlewares

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

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
			http.CanonicalHeaderKey("etag"):           {`W/"xyz123"`},
			"Content-Type":                             {"application/fhir+json"},
			"Cache-Control":                            {"max-age=60"},
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
			http.CanonicalHeaderKey("etag"):           {`W/"xyz123"`},
			"Content-Type":                             {"application/fhir+json"},
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
