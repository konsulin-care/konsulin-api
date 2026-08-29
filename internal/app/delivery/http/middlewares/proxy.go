package middlewares

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/ownership"
	"konsulin-service/internal/pkg/utils"

	"github.com/casbin/casbin/v2"
	"go.uber.org/zap"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// bodyEncoding represents the original Content-Encoding of the proxied response body.
type bodyEncoding string

const (
	bodyEncodingIdentity bodyEncoding = "identity"
	bodyEncodingBrotli   bodyEncoding = "br"
	bodyEncodingGzip     bodyEncoding = "gzip"
	bodyEncodingZstd     bodyEncoding = "zstd"
)

// maxPostFHIRHookErrorHeaderLen is the maximum length (in bytes) for the
// X-Konsulin-Post-FHIR-Hook-Error response header value. Keeps the header
// within typical server/proxy limits (often 4K–8K per header) while still
// allowing multiple hook error messages to be included.
const maxPostFHIRHookErrorHeaderLen = 2048

// headerPostFHIRHookError is the response header key set when any Post-FHIR-proxy hook returns an error.
const headerPostFHIRHookError = "X-Konsulin-Post-FHIR-Hook-Error"

// decodeBodyForFiltering decodes the body according to the Content-Encoding header.
// Any decoding failure results in an error so the caller can fail closed.
func decodeBodyForFiltering(body []byte, contentEncoding string) ([]byte, bodyEncoding, error) {
	ce := strings.ToLower(strings.TrimSpace(contentEncoding))

	switch ce {
	case "br":
		br := brotli.NewReader(bytes.NewReader(body))
		decoded, err := io.ReadAll(br)
		if err != nil {
			return nil, "", err
		}
		return decoded, bodyEncodingBrotli, nil
	case "gzip":
		gr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, "", err
		}
		decoded, rerr := io.ReadAll(gr)
		_ = gr.Close()
		if rerr != nil {
			return nil, "", rerr
		}
		return decoded, bodyEncodingGzip, nil
	case "identity", "":
		return body, bodyEncodingIdentity, nil
	case "zstd":
		zr, err := zstd.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, "", err
		}
		defer zr.Close()
		decoded, rerr := io.ReadAll(zr)
		if rerr != nil {
			return nil, "", rerr
		}
		return decoded, bodyEncodingZstd, nil
	default:
		// unknown encoding -> return error to preserve fail closed behaviour
		return nil, "", fmt.Errorf("unknown content encoding: %s", ce)
	}
}

// encodeBodyFromFiltering re-applies the original encoding to a filtered body.
// Any encoding failure results in an error so the caller can fail closed.
func encodeBodyFromFiltering(body []byte, enc bodyEncoding) ([]byte, error) {
	switch enc {
	case bodyEncodingBrotli:
		var buf bytes.Buffer
		bw := brotli.NewWriterLevel(&buf, brotli.BestCompression)
		if _, err := bw.Write(body); err != nil {
			_ = bw.Close()
			return nil, err
		}
		if err := bw.Close(); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	case bodyEncodingGzip:
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		if _, err := gw.Write(body); err != nil {
			_ = gw.Close()
			return nil, err
		}
		if err := gw.Close(); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	case bodyEncodingZstd:
		var buf bytes.Buffer
		zw, err := zstd.NewWriter(&buf)
		if err != nil {
			return nil, err
		}
		if _, err := zw.Write(body); err != nil {
			_ = zw.Close()
			return nil, err
		}
		if err := zw.Close(); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	default:
		// unknown encoding -> return original body
		return body, nil
	}
}

// doFHIRProxyRequest builds the FHIR proxy URL, creates the HTTP request, and executes it.
// sanitizeProxyPath validates and sanitizes the proxied path to prevent injection.
func sanitizeProxyPath(path string) (string, error) {
	if path == "/fhir" {
		return "", nil
	}
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("invalid path: path traversal detected")
	}
	if strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("invalid path: unexpected leading slash")
	}
	return path, nil
}

func doFHIRProxyRequest(r *http.Request, target string, client *http.Client) (resp *http.Response, respBody []byte, bodyBytes []byte, err error) {
	path, err := sanitizeProxyPath(strings.TrimPrefix(r.URL.Path, "/fhir/"))
	if err != nil {
		return nil, nil, nil, exceptions.ErrCreateHTTPRequest(err)
	}

	// Build the URL using url.URL semantics to prevent URL injection.
	// This properly encodes special characters in the path and separates
	// query parameters from the path, unlike string concatenation.
	targetURL, pErr := url.Parse(target)
	if pErr != nil {
		return nil, nil, nil, exceptions.ErrCreateHTTPRequest(pErr)
	}
	if path != "" {
		targetURL = targetURL.JoinPath(path)
	}
	targetURL.RawQuery = r.URL.RawQuery
	fullURL := targetURL.String()

	// SSRF protection: verify the final URL host matches the expected target.
	if verr := validateProxyURLHost(fullURL, target); verr != nil {
		return nil, nil, nil, exceptions.ErrCreateHTTPRequest(verr)
	}

	bodyBytes, _ = r.Context().Value(constvars.CONTEXT_RAW_BODY).([]byte)
	if bodyBytes == nil {
		bodyBytes = []byte{}
	}

	var req *http.Request
	// #nosec — SSRF-safe: fullURL host is validated by validateProxyURLHost
	// on lines 172-175 and URL is constructed via url.URL.JoinPath which makes
	// the host immutable. allowedRedirectHost (CheckRedirect) adds transport-level
	// protection. See sanitizeProxyPath, validateProxyURLHost, allowedRedirectHost.
	req, err = http.NewRequestWithContext(r.Context(), r.Method, fullURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, nil, nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header = r.Header.Clone()
	if r.Method == constvars.MethodPost || r.Method == constvars.MethodPut || r.Method == constvars.MethodPatch {
		req.Header.Set("Content-Type", "application/fhir+json")
	}
	req.Header.Set("Accept", "application/fhir+json")

	resp, err = client.Do(req)
	if err != nil {
		return nil, nil, bodyBytes, exceptions.ErrSendHTTPRequest(err)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		_ = resp.Body.Close()
		return nil, nil, bodyBytes, exceptions.ErrReadBody(readErr)
	}
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(respBody))

	return resp, respBody, bodyBytes, nil
}

// filterFHIRResponse applies RBAC and ownership filtering to the FHIR response body.
func (m *Middlewares) filterFHIRResponse(ctx context.Context, resp *http.Response, respBody []byte) ([]byte, bodyEncoding, bool, error) {
	roles, _ := ctx.Value(keyRoles).([]string)
	fhirRole, _ := ctx.Value(keyFHIRRole).(string)
	fhirID, _ := ctx.Value(keyFHIRID).(string)

	decoded, enc, derr := decodeBodyForFiltering(respBody, resp.Header.Get("Content-Encoding"))
	if derr != nil {
		m.Log.Warn("failed to decode response body for filtering; failing closed", zap.Error(derr))
		return nil, "", false, exceptions.ErrServerProcess(derr)
	}

	bodyAfterFilters := decoded
	mutated := false

	filteringRole := determineFilteringRole(roles)
	if filteringRole == constvars.KonsulinRoleSuperadmin {
		b, _, err := m.filterResponseResourceAgainstRBAC(bodyAfterFilters, roles)
		if err != nil {
			m.Log.Warn("RBAC response filtering failed; failing closed", zap.Error(err))
			return nil, "", false, exceptions.ErrServerProcess(err)
		}
		bodyAfterFilters = b
		mutated = true
	}

	if rMethod := resp.Request.Method; rMethod == http.MethodGet && fhirID != "" {
		var ownershipErr error
		bodyAfterFilters, mutated, ownershipErr = m.applyOwnershipFilter(ctx, bodyAfterFilters, roles, fhirRole, fhirID)
		if ownershipErr != nil {
			return nil, "", false, ownershipErr
		}
	}

	// Researcher and Superadmin reads of Communication resources are reduced
	// to the rule's RedactKeep fields (sender/recipient/sent/received +
	// envelope). Patient reads scoped to their own sender/recipient keep full
	// fields. The decision is driven by the ownership engine (rule.RedactKeep).
	if rMethod := resp.Request.Method; rMethod == http.MethodGet &&
		shouldStripCommunicationFields(roles, fhirID, resp.Request.URL) {
		bodyAfterFilters, mutated = stripCommunicationFields(bodyAfterFilters)
	}

	return bodyAfterFilters, enc, mutated, nil
}

// shouldStripCommunicationFields reports whether a Communication GET response
// must be reduced to the ownership rule's RedactKeep fields. Patients scoped
// to their own sender/recipient keep full fields (ownership-checked);
// Researcher-coded and Superadmin callers are redacted (the engine decides via
// the rule's RedactKeep list).
func shouldStripCommunicationFields(roles []string, fhirID string, u *url.URL) bool {
	if fhirID != "" && queryHasOwnRef(u.Query(), fhirID, "sender", "recipient") {
		return false
	}
	oc := ownership.NewContext()
	for _, r := range roles {
		switch {
		case strings.EqualFold(r, constvars.KonsulinRoleSuperadmin):
			oc.HasSuperadminRole = true
		case strings.EqualFold(r, constvars.KonsulinRoleResearcher):
			oc.AddCoding(constvars.FhirPractitionerRoleSystemHL7, constvars.FhirPractitionerRoleCodeResearcher)
		}
	}
	return ownership.ShouldRedact(constvars.ResourceCommunication, oc)
}

// stripCommunicationFields reduces Communication resources in a response body
// to communicationAllowedFields. A single Communication resource or each
// Communication entry inside a Bundle is stripped; non-Communication resources
// are left untouched. Returns (body, mutated) where mutated reports whether any
// resource was rewritten.
func stripCommunicationFields(body []byte) ([]byte, bool) {
	switch extractResourceTypeFromJSON(body) {
	case constvars.ResourceCommunication:
		stripped, ok := stripSingleCommunication(body)
		if !ok {
			return body, false
		}
		return stripped, true
	case constvars.ResourceBundle:
		return stripCommunicationBundle(body)
	default:
		return body, false
	}
}

// stripCommunicationBundle reduces each Communication entry inside a Bundle
// to communicationAllowedFields; non-Communication entries are left untouched.
// Returns (body, mutated) where mutated reports whether any resource was
// rewritten, mirroring stripCommunicationFields' contract for bundles.
func stripCommunicationBundle(body []byte) ([]byte, bool) {
	var bundle Bundle
	if err := json.Unmarshal(body, &bundle); err != nil {
		return body, false
	}
	mutated := false
	for i := range bundle.Entry {
		if extractResourceTypeFromJSON(bundle.Entry[i].Resource) != constvars.ResourceCommunication {
			continue
		}
		stripped, ok := stripSingleCommunication(bundle.Entry[i].Resource)
		if !ok {
			continue
		}
		bundle.Entry[i].Resource = stripped
		mutated = true
	}
	if !mutated {
		return body, false
	}
	out, err := json.Marshal(bundle)
	if err != nil {
		return body, false
	}
	return out, true
}

// stripSingleCommunication keeps only the ownership rule's RedactKeep fields
// from a Communication resource. Returns (nil, false) on parse or marshal
// failure.
func stripSingleCommunication(raw json.RawMessage) ([]byte, bool) {
	keep := ownership.RedactKeepFields(constvars.ResourceCommunication)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, false
	}
	out := make(map[string]json.RawMessage, len(keep))
	for _, k := range keep {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	res, err := json.Marshal(out)
	if err != nil {
		return nil, false
	}
	return res, true
}

// applyOwnershipFilter handles bundle and single-resource ownership filtering.
func (m *Middlewares) applyOwnershipFilter(ctx context.Context, body []byte, roles []string, fhirRole, fhirID string) ([]byte, bool, error) {
	if bundle, isBundle, _ := decodeBundle(body); isBundle {
		removed := m.applyOwnershipFilterToBundle(ctx, bundle, roles, fhirRole, fhirID)
		if removed > 0 {
			if bundle.Total != nil {
				v := len(bundle.Entry)
				bundle.Total = &v
			}
			filtered, _ := encodeBundle(bundle)
			return filtered, true, nil
		}
		return body, false, nil
	}

	filteredBody, allowed, ferr := m.filterSingleResourceByOwnership(ctx, body, roles, fhirRole, fhirID)
	if ferr != nil {
		return nil, false, exceptions.ErrServerProcess(ferr)
	}
	if !allowed {
		return nil, false, exceptions.ErrAuthInvalidRole(fmt.Errorf("forbidden: ownership cannot be proven"))
	}
	if filteredBody != nil {
		return filteredBody, true, nil
	}
	return body, false, nil
}

// runPostFHIRProxyHooks executes all registered post-proxy hooks and returns error messages.
func (m *Middlewares) runPostFHIRProxyHooks(r *http.Request, respBody, bodyBytes []byte) []string {
	var msgs []string
	for _, hook := range m.PostFHIRProxyHooks {
		reqDetail := PostFHIRProxyUserRequestDetail{
			Context: r.Context(),
			Method:  r.Method,
			Path:    r.URL.Path,
			Body:    bodyBytes,
		}
		respDetail := PostFHIRProxyFHIRServerResponse{
			StatusCode: http.StatusOK,
			Body:       respBody,
		}
		if err := hook(reqDetail, respDetail); err != nil {
			m.Log.Warn("PostFHIRProxyHook error", zap.Error(err))
			msgs = append(msgs, err.Error())
		}
	}
	return msgs
}

// writeBridgeResponse writes the final response headers and body.
// When mutated is true, the ETag header is stripped to prevent cache poisoning
// — the response body differs from the upstream original, so the original ETag
// is no longer valid.
func (m *Middlewares) writeBridgeResponse(w http.ResponseWriter, resp *http.Response, finalBody []byte, postHookErrMsgs []string, mutated bool) {
	for k, v := range resp.Header {
		if strings.EqualFold(k, "Content-Length") {
			continue
		}
		// Strip ETag when the body was mutated: the response differs from
		// the upstream original, so the original ETag is no longer valid.
		// Keeping it would cause downstream caches to serve the mutated body
		// with a stale ETag (cache poisoning).
		if mutated && strings.EqualFold(k, "ETag") {
			continue
		}
		w.Header()[k] = v
	}
	if len(postHookErrMsgs) > 0 {
		joined := strings.Join(postHookErrMsgs, "; ")
		if len(joined) > maxPostFHIRHookErrorHeaderLen {
			joined = joined[:maxPostFHIRHookErrorHeaderLen-3] + "..."
		}
		w.Header().Set(headerPostFHIRHookError, joined)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(finalBody)))
	w.WriteHeader(resp.StatusCode)
	if _, err := w.Write(finalBody); err != nil {
		m.Log.Warn("failed writing response body", zap.Error(err))
	}
}

// allowedRedirectHost returns a CheckRedirect function that only allows redirects
// to the exact same host:port as expectedHost. This prevents SSRF via redirect following.
func allowedRedirectHost(expectedHost string) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if req.URL.Host != expectedHost {
			return fmt.Errorf("redirect to %q blocked: host must be %q", req.URL.Host, expectedHost)
		}
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	}
}

// validateProxyURLHost checks that the constructed proxy URL's host matches the target.
// Returns an error if the host does not match, preventing SSRF via URL injection.
func validateProxyURLHost(fullURL, target string) error {
	parsedURL, err := url.Parse(fullURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}
	parsedTarget, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}
	if parsedURL.Host != parsedTarget.Host {
		return fmt.Errorf("SSRF: proxy URL host %q does not match target host %q", parsedURL.Host, parsedTarget.Host)
	}
	return nil
}

func (m *Middlewares) Bridge(target string) http.Handler {
	parsedTarget, err := url.Parse(target)
	if err != nil {
		panic(fmt.Sprintf("invalid FHIR target URL: %v", err))
	}
	client := &http.Client{
		Timeout:       15 * time.Second,
		Transport:     &http.Transport{MaxIdleConnsPerHost: 100},
		CheckRedirect: allowedRedirectHost(parsedTarget.Host),
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, respBody, bodyBytes, err := doFHIRProxyRequest(r, target, client)
		if err != nil {
			utils.BuildErrorResponse(m.Log, w, err)
			return
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode >= http.StatusBadRequest {
			fhirErr := exceptions.BuildNewCustomError(fmt.Errorf("%s", string(respBody)), resp.StatusCode, string(respBody), constvars.ErrDevServerProcess)
			utils.BuildErrorResponse(m.Log, w, fhirErr)
			return
		}

		postHookErrMsgs := m.runPostFHIRProxyHooks(r, respBody, bodyBytes)

		bodyAfterOwnership, encForFilters, mutated, ferr := m.filterFHIRResponse(r.Context(), resp, respBody)
		if ferr != nil {
			utils.BuildErrorResponse(m.Log, w, ferr)
			return
		}

		finalBody := respBody
		if mutated {
			encoded, eerr := encodeBodyFromFiltering(bodyAfterOwnership, encForFilters)
			if eerr != nil {
				m.Log.Warn("failed to encode filtered response body; failing closed", zap.Error(eerr))
				utils.BuildErrorResponse(m.Log, w, exceptions.ErrServerProcess(eerr))
				return
			}
			finalBody = encoded
		}

		m.writeBridgeResponse(w, resp, finalBody, postHookErrMsgs, mutated)
	})
}

func determineFilteringRole(roles []string) string {
	for _, role := range roles {
		if strings.EqualFold(role, constvars.KonsulinRoleSuperadmin) {
			return constvars.KonsulinRoleSuperadmin
		}
	}
	return ""
}

func (m *Middlewares) filterResponseResourceAgainstRBAC(body []byte, roles []string) ([]byte, int, error) {
	shouldFilter := false
	for _, role := range roles {
		if strings.EqualFold(role, constvars.KonsulinRoleSuperadmin) {
			shouldFilter = true
			break
		}
	}
	if !shouldFilter {
		return body, 0, nil
	}

	if !strings.EqualFold(extractResourceTypeFromJSON(body), constvars.ResourceBundle) {
		return body, 0, nil
	}

	var bundle struct {
		ResourceType string        `json:"resourceType"`
		ID           string        `json:"id,omitempty"`
		Type         string        `json:"type,omitempty"`
		Total        *int          `json:"total,omitempty"`
		Link         any           `json:"link,omitempty"`
		Entry        []BundleEntry `json:"entry"`
	}

	if err := json.Unmarshal(body, &bundle); err != nil {
		return body, 0, nil
	}

	filtered, removed := filterBundleEntriesByRBAC(bundle.Entry, roles, m.Enforcer)

	if removed == 0 {
		return body, 0, nil
	}

	bundle.Entry = filtered
	filteredJSON, err := json.Marshal(bundle)
	if err != nil {
		return body, 0, err
	}

	return filteredJSON, removed, nil
}

// filterBundleEntriesByRBAC filters Bundle entries by RBAC permission for each resourceType.
func filterBundleEntriesByRBAC(entries []BundleEntry, roles []string, enf *casbin.Enforcer) ([]BundleEntry, int) {
	removed := 0
	filtered := make([]BundleEntry, 0, len(entries))
	for _, e := range entries {
		resourceType := extractResourceTypeFromJSON(e.Resource)
		if resourceType == "" {
			filtered = append(filtered, e)
			continue
		}
		if rbacAllowedAnyRole(enf, roles, resourceType) {
			filtered = append(filtered, e)
		} else {
			removed++
		}
	}
	return filtered, removed
}

// rbacAllowedAnyRole checks if any role can access a given FHIR resource type.
func rbacAllowedAnyRole(enf *casbin.Enforcer, roles []string, resourceType string) bool {
	for _, role := range roles {
		if allowed(enf, role, http.MethodGet, "/fhir/"+resourceType) {
			return true
		}
	}
	return false
}

// extractResourceTypeFromJSON parses the resource type from a FHIR JSON body.
func extractResourceTypeFromJSON(body []byte) string {
	var env struct {
		ResourceType string `json:"resourceType"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return ""
	}
	return env.ResourceType
}

// BundleEntry and Bundle represent a minimal FHIR Bundle envelope for filtering.
type BundleEntry struct {
	FullURL  string          `json:"fullUrl,omitempty"`
	Resource json.RawMessage `json:"resource"`
	Search   map[string]any  `json:"search,omitempty"`
}

type Bundle struct {
	ResourceType string        `json:"resourceType"`
	ID           string        `json:"id,omitempty"`
	Type         string        `json:"type,omitempty"`
	Total        *int          `json:"total,omitempty"`
	Link         any           `json:"link,omitempty"`
	Entry        []BundleEntry `json:"entry"`
}

// decodeBundle assumes body is already uncompressed JSON and tries to unmarshal a FHIR Bundle.
// It returns (bundle, isBundle, error). Errors are only returned for unexpected failures;
// JSON parse failures simply mean "not a bundle".
func decodeBundle(body []byte) (*Bundle, bool, error) {
	var envelope struct {
		ResourceType string `json:"resourceType"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, false, nil
	}

	if !strings.EqualFold(envelope.ResourceType, constvars.ResourceBundle) {
		return nil, false, nil
	}

	var bundle Bundle
	if err := json.Unmarshal(body, &bundle); err != nil {
		return nil, false, nil
	}
	return &bundle, true, nil
}

// encodeBundle marshals a Bundle into JSON. Content-Encoding is handled by the caller.
func encodeBundle(bundle *Bundle) ([]byte, error) {
	filteredJSON, err := json.Marshal(bundle)
	if err != nil {
		return nil, err
	}

	return filteredJSON, nil
}

// ownershipContextFromRoles builds a minimal OwnershipContext from session
// roles, the resolved FHIR role, and the caller's FHIR id. Session roles map
// to role flags and to the Phase 1 practitioner-role codings (a Researcher
// session holds the researcher coding; a Clinic Admin session holds the SNOMED
// administrative-staff coding). No FHIR I/O is performed.
func ownershipContextFromRoles(roles []string, fhirRole, fhirID string) *ownership.OwnershipContext {
	oc := ownership.NewContext()
	for _, r := range roles {
		switch {
		case strings.EqualFold(r, constvars.KonsulinRolePatient):
			oc.HasPatientRole = true
		case strings.EqualFold(r, constvars.KonsulinRolePractitioner):
			oc.HasPractitionerRole = true
			oc.HoldsPractitionerRole = true
		case strings.EqualFold(r, constvars.KonsulinRoleSuperadmin):
			oc.HasSuperadminRole = true
		case strings.EqualFold(r, constvars.KonsulinRoleResearcher):
			oc.HasPractitionerRole = true
			oc.AddCoding(constvars.FhirPractitionerRoleSystemHL7, constvars.FhirPractitionerRoleCodeResearcher)
		case strings.EqualFold(r, constvars.KonsulinRoleClinicAdmin):
			oc.HasPractitionerRole = true
			oc.AddCoding(constvars.FhirPractitionerRoleSystemSnomed, constvars.FhirPractitionerRoleCodeAdministrativeStaff)
		}
	}
	if fhirRole == constvars.KonsulinRolePatient && fhirID != "" {
		oc.AddPatientID(fhirID)
	}
	if fhirRole == constvars.KonsulinRolePractitioner && fhirID != "" {
		oc.AddPractitionerID(fhirID)
	}
	return oc
}

// buildOwnershipContext resolves owned Patient / Practitioner identities and
// practitioner-role codings once per request into the ownership engine's
// OwnershipContext. Session roles map directly to role flags and to the
// Phase 1 practitioner-role codings (a Researcher session holds the researcher
// coding, a Clinic Admin session holds the SNOMED administrative-staff coding).
func (m *Middlewares) buildOwnershipContext(
	ctx context.Context,
	roles []string,
	fhirRole, fhirID string,
) *ownership.OwnershipContext {
	oc := ownershipContextFromRoles(roles, fhirRole, fhirID)

	// Relationship-based seeding (own Patient record via the SuperTokens
	// identifier, PractitionerRole ids and codings) runs only for sessions that
	// genuinely hold the Practitioner role. Researcher and Clinic Admin sessions
	// resolve as practitioners but never inherit the practitioner↔patient
	// relationship: their reads are code-conditioned instead.
	if oc.HoldsPractitionerRole {
		m.resolvePractitionerPatientIDs(ctx, oc, fhirID)
	}

	// Dual-identity sessions may read resources owned under either identity.
	// Seed the non-active identity's own resource ID so GET responses pass the
	// ownership filter under either active role. The secondary lookup runs
	// only when the other role is actually held (single-role sessions pay
	// nothing), and it never re-runs the practitioner related-patient/role
	// resolution above.
	if uid, _ := ctx.Value(keyUID).(string); uid != "" {
		if oc.HasPractitionerRole && fhirRole == constvars.KonsulinRolePatient {
			if pracID := m.resolveSecondaryFHIRID(ctx, uid, fhirRole); pracID != "" {
				oc.AddPractitionerID(pracID)
			}
		}
		if oc.HasPatientRole && fhirRole == constvars.KonsulinRolePractitioner {
			if patID := m.resolveSecondaryFHIRID(ctx, uid, fhirRole); patID != "" {
				oc.AddPatientID(patID)
			}
		}
	}

	return oc
}

// resolvePractitionerPatientIDs populates PatientIDs and PractitionerRoleIDs
// for a practitioner.
// resolveOwnPatientBySupertoken seeds the caller's own Patient record — the
// dual-identity case where the same SuperTokens user holds both Patient and
// Practitioner identities. Matching by the FhirSupertokenSystemIdentifier (the
// uid) is exact: one uid maps to at most one Patient record, and email
// collisions cannot widen access.
func (m *Middlewares) resolveOwnPatientBySupertoken(ctx context.Context, oc *ownership.OwnershipContext) {
	uid, _ := ctx.Value(keyUID).(string)
	if uid == "" {
		return
	}
	pats, err := m.PatientFhirClient.FindPatientByIdentifier(
		ctx,
		fmt.Sprintf("%s|%s", constvars.FhirSupertokenSystemIdentifier, uid),
	)
	if err != nil {
		m.Log.Warn("failed to resolve own patient by supertoken identifier; skipping patient seeding",
			zap.String("uid", uid), zap.Error(err))
		return
	}
	for _, p := range pats {
		oc.AddPatientID(p.ID)
	}
}

// resolvePractitionerRoleIDs populates PractitionerRoleIDs and the
// practitioner-role codings for the given practitioner.
func (m *Middlewares) resolvePractitionerRoleIDs(ctx context.Context, oc *ownership.OwnershipContext, fhirID string) {
	practitionerRoles, err := m.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerID(ctx, fhirID)
	if err != nil {
		m.Log.Warn("failed to find practitioner roles by practitioner ID. skipping practitioner role population", zap.String("practitionerID", fhirID), zap.Error(err))
		return
	}
	for _, pr := range practitionerRoles {
		oc.AddPractitionerRoleID(pr.ID)
		for _, cc := range pr.Code {
			for _, c := range cc.Coding {
				oc.AddCoding(c.System, c.Code)
			}
		}
	}
}

func (m *Middlewares) resolvePractitionerPatientIDs(ctx context.Context, oc *ownership.OwnershipContext, fhirID string) {
	if !oc.HasPractitionerRole || len(oc.PatientIDs) > 0 || fhirID == "" {
		return
	}
	m.resolveOwnPatientBySupertoken(ctx, oc)
	m.resolvePractitionerRoleIDs(ctx, oc, fhirID)
}

// resourceOwnedByContext decides ownership for a single FHIR resource using the
// ownership engine's declarative rules (internal/pkg/ownership). Unclassified
// and unparseable resources deny (fail-closed flip); per-type strategies
// (invoice) live in the engine's checker registry.
func (m *Middlewares) resourceOwnedByContext(
	raw json.RawMessage,
	resourceType string,
	_ string,
	oc *ownership.OwnershipContext,
) bool {
	owned, err := ownership.OwnedBy(raw, resourceType, oc)
	if err != nil {
		m.Log.Warn("ownership check failed; denying",
			zap.String("resourceType", resourceType),
			zap.Error(err))
		return false
	}
	return owned
}

// applyOwnershipFilterToBundle mutates bundle.Entry in-place, keeping only owned resources.
// bundleEntryInfo caches resource metadata for ownership filtering.
type bundleEntryInfo struct {
	idx          int
	owned        bool
	resourceType string
	id           string
}

func (m *Middlewares) applyOwnershipFilterToBundle(
	ctx context.Context,
	bundle *Bundle,
	roles []string,
	fhirRole, fhirID string,
) int {
	oc := m.buildOwnershipContext(ctx, roles, fhirRole, fhirID)
	infos, allowedRefs := m.evaluateBundleOwnership(ctx, bundle, oc)
	return filterBundleEntries(bundle, infos, allowedRefs, oc, m.Log)
}

// evaluateBundleOwnership determines direct ownership for each bundle entry and collects allowed refs.
func (m *Middlewares) evaluateBundleOwnership(_ context.Context, bundle *Bundle, oc *ownership.OwnershipContext) ([]bundleEntryInfo, map[string]struct{}) {
	infos := make([]bundleEntryInfo, len(bundle.Entry))
	allowedRefs := make(map[string]struct{})

	for i, e := range bundle.Entry {
		var env struct {
			ResourceType string `json:"resourceType"`
			ID           string `json:"id,omitempty"`
		}
		if err := json.Unmarshal(e.Resource, &env); err != nil || env.ResourceType == "" {
			// Unparseable entry: deny (fail-closed flip).
			infos[i] = bundleEntryInfo{idx: i, owned: false}
			continue
		}
		owned := m.resourceOwnedByContext(e.Resource, env.ResourceType, env.ID, oc)
		infos[i] = bundleEntryInfo{idx: i, owned: owned, resourceType: env.ResourceType, id: env.ID}

		if owned && oc.HoldsPractitionerRole {
			var resMap map[string]any
			if err := json.Unmarshal(e.Resource, &resMap); err == nil {
				var refs []string
				collectReferences(resMap, &refs, 0)
				for _, r := range refs {
					allowedRefs[r] = struct{}{}
				}
			}
		}
	}
	return infos, allowedRefs
}

// filterBundleEntries removes entries that are neither owned nor provably kept:
// referenced entries survive only when OwnedBy confirms the caller owns them
// (strict referenced-bundle-keep). Public-scope entries are already owned;
// unknown and internal types fail closed. A co-worker's scoped resource
// referenced by an owned resource is therefore dropped.
func filterBundleEntries(bundle *Bundle, infos []bundleEntryInfo, allowedRefs map[string]struct{}, oc *ownership.OwnershipContext, log *zap.Logger) int {
	removed := 0
	filtered := make([]BundleEntry, 0, len(bundle.Entry))
	for i, e := range bundle.Entry {
		info := infos[i]
		if info.owned {
			filtered = append(filtered, e)
			continue
		}
		refKey := fmt.Sprintf("%s/%s", info.resourceType, info.id)
		if _, isReferenced := allowedRefs[refKey]; isReferenced {
			if owned, err := ownership.OwnedBy(e.Resource, info.resourceType, oc); err == nil && owned {
				filtered = append(filtered, e)
				continue
			}
		}
		log.Info("removing resource from bundle", zap.String("resourceType", info.resourceType), zap.String("resourceID", info.id))
		removed++
	}
	bundle.Entry = filtered
	return removed
}

// filterSingleResourceByOwnership applies the same ownership rules as the bundle
// filter, but for a single FHIR resource response body.
//
// Returns:
//   - filteredBody: body to send back (usually the original body)
//   - allowed: whether the caller is allowed to see this resource
//   - err: real errors; (nil, false, nil) means "not owned"
func (m *Middlewares) filterSingleResourceByOwnership(
	ctx context.Context,
	body []byte,
	roles []string,
	fhirRole, fhirID string,
) ([]byte, bool, error) {
	// Only filter when we have a resolved FHIR identity.
	if fhirRole == "" {
		return body, true, nil
	}

	oc := m.buildOwnershipContext(ctx, roles, fhirRole, fhirID)

	var env struct {
		ResourceType string `json:"resourceType"`
		ID           string `json:"id,omitempty"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, false, err
	}

	if env.ResourceType == "" {
		// Not a FHIR resource or no type → do not filter.
		return body, true, nil
	}

	owned := m.resourceOwnedByContext(body, env.ResourceType, env.ID, oc)
	if !owned {
		// Not owned → deny.
		return nil, false, nil
	}

	// Owned → allow, and we can safely return original body bytes.
	return body, true, nil
}

// authTxProxyAccess checks RBAC and query params for TxProxy access.
func (m *Middlewares) authTxProxyAccess(w http.ResponseWriter, r *http.Request) error {
	roles, ok := r.Context().Value(keyRoles).([]string)
	if !ok || len(roles) == 0 {
		utils.BuildErrorResponse(m.Log, w, exceptions.ErrTokenMissing(nil))
		return fmt.Errorf("no roles")
	}
	for _, role := range roles {
		if ok, _ := m.Enforcer.Enforce(role, r.Method, r.URL.Path); ok {
			return nil
		}
	}
	utils.BuildErrorResponse(m.Log, w, exceptions.ErrAuthInvalidRole(fmt.Errorf("forbidden: role not allowed to access terminology service")))
	return fmt.Errorf("access denied")
}

// buildTxProxyURL builds the full URL for the terminology server proxy.
// Uses url.URL semantics to prevent URL injection, mirroring the same
// SSRF-hardened pattern used in doFHIRProxyRequest. Path segments are
// properly encoded and any special characters (//, @, etc.) are handled
// as literal path content, not host/authority separators.
func buildTxProxyURL(target string, r *http.Request, prefix, version string) string {
	q := r.URL.Query()
	hasFilter := strings.TrimSpace(q.Get("filter")) != ""
	hasCount := strings.TrimSpace(q.Get("count")) != ""
	if !hasFilter && !hasCount {
		return ""
	}
	relativePath := strings.TrimPrefix(r.URL.Path, fmt.Sprintf("/%s/%s/tx", prefix, version))
	if relativePath == "" {
		return target
	}
	if strings.Contains(relativePath, "..") {
		return ""
	}
	targetURL, err := url.Parse(target)
	if err != nil {
		return ""
	}
	targetURL = targetURL.JoinPath(relativePath)
	targetURL.RawQuery = r.URL.RawQuery
	return targetURL.String()
}

func (m *Middlewares) TxProxy(target string) http.Handler {
	parsedTarget, err := url.Parse(target)
	if err != nil {
		panic(fmt.Sprintf("invalid TxProxy target URL: %v", err))
	}
	client := &http.Client{
		Timeout:       15 * time.Second,
		Transport:     &http.Transport{MaxIdleConnsPerHost: 100},
		CheckRedirect: allowedRedirectHost(parsedTarget.Host),
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := m.authTxProxyAccess(w, r); err != nil {
			return
		}

		fullURL := buildTxProxyURL(target, r, m.InternalConfig.App.EndpointPrefix, m.InternalConfig.App.Version)
		if fullURL == "" {
			w.WriteHeader(http.StatusAccepted)
			return
		}

		// SSRF protection: verify the final URL host matches the expected target.
		if verr := validateProxyURLHost(fullURL, target); verr != nil {
			m.Log.Error("TxProxy URL validation failed", zap.Error(verr))
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrCreateHTTPRequest(verr))
			return
		}

		bodyBytes, _ := r.Context().Value(constvars.CONTEXT_RAW_BODY).([]byte)
		if bodyBytes == nil {
			bodyBytes = []byte{}
		}

		m.doTxProxyRequest(w, r, fullURL, bodyBytes, client)
	})
}

// doTxProxyRequest executes a proxy request to the terminology server and streams the response.
func (m *Middlewares) doTxProxyRequest(w http.ResponseWriter, r *http.Request, fullURL string, bodyBytes []byte, client *http.Client) {
	proxyCtx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	// #nosec — SSRF-safe: fullURL host is validated by validateProxyURLHost
	// (called in TxProxy before doTxProxyRequest) and URL is constructed via
	// buildTxProxyURL using url.URL.JoinPath which makes the host immutable.
	// allowedRedirectHost (CheckRedirect) adds transport-level protection.
	req, err := http.NewRequestWithContext(proxyCtx, r.Method, fullURL, bytes.NewReader(bodyBytes))
	if err != nil {
		utils.BuildErrorResponse(m.Log, w, exceptions.ErrCreateHTTPRequest(err))
		return
	}

	req.Header.Set("Accept", "application/fhir+json")
	if contentType := r.Header.Get("Content-Type"); contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(proxyCtx.Err(), context.DeadlineExceeded) {
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(m.Log, w, exceptions.ErrSendHTTPRequest(err))
		return
	}
	defer func() { _ = resp.Body.Close() }()

	for k, v := range resp.Header {
		if strings.HasPrefix(k, "Access-Control-") || k == "Content-Length" || k == "Connection" {
			continue
		}
		for _, val := range v {
			w.Header().Add(k, val)
		}
	}

	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		m.Log.Warn("failed to copy response body", zap.Error(err))
	}
}

// collectReferences walks arbitrary JSON and collects all "reference" string fields.
// collectMapReferences walks a JSON object and collects "reference" fields into out.
func collectMapReferences(m map[string]any, out *[]string, depth int) {
	if depth > 30 {
		return
	}
	for k, vv := range m {
		if k == "reference" {
			if s, ok := vv.(string); ok {
				*out = append(*out, s)
			}
		} else {
			collectReferences(vv, out, depth+1)
		}
	}
}

// collectArrayReferences walks a JSON array and collects "reference" fields into out.
func collectArrayReferences(arr []any, out *[]string, depth int) {
	if depth > 30 {
		return
	}
	for _, vv := range arr {
		collectReferences(vv, out, depth+1)
	}
}

func collectReferences(v any, out *[]string, depth int) {
	switch t := v.(type) {
	case map[string]any:
		collectMapReferences(t, out, depth)
	case []any:
		collectArrayReferences(t, out, depth)
	}
}
