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
	"strconv"
	"strings"
	"time"

	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"

	"github.com/casbin/casbin/v2"
	"go.uber.org/zap"

	"slices"

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
func (m *Middlewares) doFHIRProxyRequest(r *http.Request, target string, client *http.Client) (resp *http.Response, respBody []byte, bodyBytes []byte, err error) {
	path := strings.TrimPrefix(r.URL.Path, "/fhir/")
	if path == "/fhir" {
		path = ""
	}

	fullURL := target
	if path != "" {
		if !strings.HasSuffix(target, "/") && !strings.HasPrefix(path, "/") {
			fullURL += "/"
		}
		fullURL += path
	}
	if r.URL.RawQuery != "" {
		fullURL += "?" + r.URL.RawQuery
	}

	bodyBytes, _ = r.Context().Value(constvars.CONTEXT_RAW_BODY).([]byte)
	if bodyBytes == nil {
		bodyBytes = []byte{}
	}

	var req *http.Request
	req, err = http.NewRequestWithContext(r.Context(), r.Method, fullURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, nil, nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header = r.Header.Clone()
	if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
		req.Header.Set("Content-Type", "application/fhir+json")
	}
	req.Header.Set("Accept", "application/fhir+json")

	resp, err = client.Do(req)
	if err != nil {
		return nil, nil, bodyBytes, exceptions.ErrSendHTTPRequest(err)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		resp.Body.Close()
		return nil, nil, bodyBytes, exceptions.ErrReadBody(readErr)
	}
	resp.Body.Close()
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

	return bodyAfterFilters, enc, mutated, nil
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
func (m *Middlewares) writeBridgeResponse(w http.ResponseWriter, resp *http.Response, finalBody []byte, postHookErrMsgs []string) {
	for k, v := range resp.Header {
		if strings.EqualFold(k, "Content-Length") {
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

func (m *Middlewares) Bridge(target string) http.Handler {
	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: &http.Transport{MaxIdleConnsPerHost: 100},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, respBody, bodyBytes, err := m.doFHIRProxyRequest(r, target, client)
		if err != nil {
			utils.BuildErrorResponse(m.Log, w, err)
			return
		}
		defer resp.Body.Close()

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

		m.writeBridgeResponse(w, resp, finalBody, postHookErrMsgs)
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

	if !strings.EqualFold(extractResourceTypeFromJSON(body), "Bundle") {
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

	if !strings.EqualFold(envelope.ResourceType, "Bundle") {
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

// ownershipContext describes what FHIR resources (Patient / Practitioner) the caller owns.
type ownershipContext struct {
	HasPatientRole      bool
	HasPractitionerRole bool
	PatientIDs          map[string]struct{}
	PractitionerIDs     map[string]struct{}
	PractitionerRoleIDs []string
}

// buildOwnershipContext resolves owned Patient / Practitioner IDs once per request.
func (m *Middlewares) buildOwnershipContext(
	ctx context.Context,
	roles []string,
	fhirRole, fhirID string,
) *ownershipContext {
	oc := &ownershipContext{
		PatientIDs:          make(map[string]struct{}),
		PractitionerIDs:     make(map[string]struct{}),
		PractitionerRoleIDs: make([]string, 0),
	}

	for _, r := range roles {
		if strings.EqualFold(r, constvars.KonsulinRolePatient) {
			oc.HasPatientRole = true
		}
		if strings.EqualFold(r, constvars.KonsulinRolePractitioner) {
			oc.HasPractitionerRole = true
		}
	}

	if fhirRole == constvars.KonsulinRolePatient && fhirID != "" {
		oc.PatientIDs[fhirID] = struct{}{}
	}
	if fhirRole == constvars.KonsulinRolePractitioner && fhirID != "" {
		oc.PractitionerIDs[fhirID] = struct{}{}
	}

	m.resolvePractitionerPatientIDs(ctx, oc, fhirID)

	return oc
}

// resolvePractitionerPatientIDs populates PatientIDs and PractitionerRoleIDs for a practitioner.
func (m *Middlewares) resolvePractitionerPatientIDs(ctx context.Context, oc *ownershipContext, fhirID string) {
	if !oc.HasPractitionerRole || len(oc.PatientIDs) > 0 || fhirID == "" {
		return
	}
	prac, err := m.PractitionerFhirClient.FindPractitionerByID(ctx, fhirID)
	if err == nil && prac != nil {
		for _, em := range prac.GetEmailAddresses() {
			pats, err := m.PatientFhirClient.FindPatientByEmail(ctx, em)
			if err != nil {
				continue
			}
			for _, p := range pats {
				if p.ID != "" {
					oc.PatientIDs[p.ID] = struct{}{}
				}
			}
		}
	}
	practitionerRoles, err := m.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerID(ctx, fhirID)
	if err != nil {
		m.Log.Warn("failed to find practitioner roles by practitioner ID. skipping practitioner role population", zap.String("practitionerID", fhirID), zap.Error(err))
		return
	}
	for _, pr := range practitionerRoles {
		if pr.ID != "" {
			oc.PractitionerRoleIDs = append(oc.PractitionerRoleIDs, pr.ID)
		}
	}
}

// ownershipChecker is a resource-specific, last-resort ownership function.
type ownershipChecker func(raw json.RawMessage, oc *ownershipContext) (bool, error)

// resourceSpecificOwnershipCheckers holds resource-specific ownership logic.
// add your own custom ownership checkers here if needed
var resourceSpecificOwnershipCheckers = map[string]ownershipChecker{
	constvars.ResourceInvoice: func(raw json.RawMessage, oc *ownershipContext) (bool, error) {
		// Invoice is public only if ALL references point to whitelisted resource types.
		publicResourceIfOwnedByTheseActors := map[string]struct{}{
			constvars.ResourcePractitioner:     {},
			constvars.ResourcePractitionerRole: {},
			constvars.ResourceDevice:           {},
		}

		var resMap map[string]any
		if err := json.Unmarshal(raw, &resMap); err != nil {
			return false, err
		}

		var refs []string
		collectReferences(resMap, &refs, 0)
		if len(refs) == 0 {
			return false, nil
		}

		for _, ref := range refs {
			parts := strings.SplitN(ref, "/", 2)
			if len(parts) == 0 {
				return false, nil
			}

			if _, ok := publicResourceIfOwnedByTheseActors[parts[0]]; !ok {
				// Found a non-whitelisted reference
				return false, nil
			}
		}

		// All references are whitelisted means the invoice is public.
		return true, nil
	},
}

// resourceOwnedByContext centralizes ownership checks for a single FHIR resource.
// It is used by both bundle-level and single-resource filters.
func (m *Middlewares) resourceOwnedByContext(
	raw json.RawMessage,
	resourceType string,
	id string,
	oc *ownershipContext,
) bool {
	if utils.IsPublicResource(resourceType) {
		return true
	}

	requiresPatient := utils.RequiresPatientOwnership(resourceType)
	requiresPract := utils.RequiresPractitionerOwnership(resourceType)
	if !requiresPatient && !requiresPract {
		return true
	}

	// If a resource requires *only* patient or *only* practitioner ownership,
	// and we lack the corresponding IDs/roles, we can't prove ownership.
	if requiresPatient && !requiresPract && len(oc.PatientIDs) == 0 && !oc.HasPatientRole {
		return false
	}
	if requiresPract && !requiresPatient && len(oc.PractitionerIDs) == 0 && !oc.HasPractitionerRole {
		return false
	}

	if simpleOwnershipCheck(resourceType, id, oc) {
		return true
	}

	if ok, err := genericOwnershipPatterns(raw, oc); err == nil && ok {
		return true
	} else if err != nil && !m.handleOwnershipCheckError(resourceType, id, err) {
		return false
	}

	if checker, ok := resourceSpecificOwnershipCheckers[resourceType]; ok {
		if ok2, err := checker(raw, oc); err == nil && ok2 {
			return true
		} else if err != nil {
			m.handleOwnershipCheckError(resourceType, id, err)
		}
	}

	// If we reach here, we couldn't prove ownership.
	return false
}

// failClosedOnErrorFromResource is a function that determines if we should fail closed on error from a resource.
// this function behaviour comes from this discussion: https://github.com/konsulin-care/konsulin-api/pull/250#discussion_r2559068460
// This function must be used to determine if we should fail closed on error from a resource.
func (m *Middlewares) failClosedOnErrorFromResource(resourceType string, resourceID string) bool {
	if resourceType == "" {
		return true
	}

	defaultDenyResources := []string{
		constvars.ResourcePatient,
		constvars.ResourceCondition,
		constvars.ResourceObservation,
		constvars.ResourceMedicationRequest,
		constvars.ResourceAllergyIntolerance,
		constvars.ResourceProcedure,
		constvars.ResourceCarePlan,
		constvars.ResourceMedicationAdministration,
	}

	if slices.Contains(defaultDenyResources, resourceType) {
		// if the resource is in the default deny list, we fail closed
		m.Log.Info(fmt.Sprintf("Denying an unauthorized request to {%s/%s}", resourceType, resourceID),
			zap.String("resourceType", resourceType),
			zap.String("resourceID", resourceID),
		)
		return true
	}

	return false
}

// handleOwnershipCheckError decides whether to fail closed or open on an ownership check error.
// Returns true if we should proceed (fail open), false if we should deny (fail closed).
func (m *Middlewares) handleOwnershipCheckError(resourceType, id string, err error) bool {
	if m.failClosedOnErrorFromResource(resourceType, id) {
		return false
	}
	m.Log.Warn("resorting to fail open on error from resource",
		zap.String("resourceType", resourceType),
		zap.String("id", id),
		zap.Error(err),
	)
	return true
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
	return filterBundleEntries(bundle, infos, allowedRefs, m.Log)
}

// evaluateBundleOwnership determines direct ownership for each bundle entry and collects allowed refs.
func (m *Middlewares) evaluateBundleOwnership(ctx context.Context, bundle *Bundle, oc *ownershipContext) ([]bundleEntryInfo, map[string]struct{}) {
	infos := make([]bundleEntryInfo, len(bundle.Entry))
	allowedRefs := make(map[string]struct{})

	for i, e := range bundle.Entry {
		var env struct {
			ResourceType string `json:"resourceType"`
			ID           string `json:"id,omitempty"`
		}
		if err := json.Unmarshal(e.Resource, &env); err != nil || env.ResourceType == "" {
			owned := !m.failClosedOnErrorFromResource(env.ResourceType, env.ID)
			infos[i] = bundleEntryInfo{idx: i, owned: owned}
			continue
		}
		owned := m.resourceOwnedByContext(e.Resource, env.ResourceType, env.ID, oc)
		infos[i] = bundleEntryInfo{idx: i, owned: owned, resourceType: env.ResourceType, id: env.ID}

		if owned && oc.HasPractitionerRole {
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

// filterBundleEntries removes entries that are neither owned nor referenced by owned resources.
func filterBundleEntries(bundle *Bundle, infos []bundleEntryInfo, allowedRefs map[string]struct{}, log *zap.Logger) int {
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
			filtered = append(filtered, e)
		} else {
			log.Info("removing resource from bundle", zap.String("resourceType", info.resourceType), zap.String("resourceID", info.id))
			removed++
		}
	}
	bundle.Entry = filtered
	return removed
}

// simpleOwnershipCheck performs direct ownership based on resourceType + id.
func simpleOwnershipCheck(resourceType, id string, oc *ownershipContext) bool {
	if id == "" {
		return false
	}

	switch resourceType {
	case constvars.ResourcePatient:
		_, ok := oc.PatientIDs[id]
		return ok
	case constvars.ResourcePractitioner:
		_, ok := oc.PractitionerIDs[id]
		return ok
	default:
		return false
	}
}

// genericOwnershipPatterns covers:
// - subject.reference
// - patient.reference
// - recipient.reference
// - actor.reference
// - participant[*].actor.reference
// - plus a full recursive "reference" walk as a safety net.
func genericOwnershipPatterns(raw json.RawMessage, oc *ownershipContext) (bool, error) {
	var res map[string]any
	if err := json.Unmarshal(raw, &res); err != nil {
		return false, err
	}

	// Check well-known reference fields first.
	for _, field := range []string{"subject", "patient", "recipient", "actor"} {
		if ref := extractReference(res[field]); ref != "" && matchesOwnedRef(ref, oc) {
			return true, nil
		}
	}

	// participant[*].actor.reference
	if checkParticipantOwnership(res, oc) {
		return true, nil
	}

	// Fallback: recursive scan of all "reference" fields.
	var refs []string
	collectReferences(res, &refs, 0)
	for _, ref := range refs {
		if matchesOwnedRef(ref, oc) {
			return true, nil
		}
	}
	return false, nil
}

// checkParticipantOwnership searches participant[*].actor.reference for a match.
func checkParticipantOwnership(res map[string]any, oc *ownershipContext) bool {
	parts, ok := res["participant"]
	if !ok {
		return false
	}
	arr, ok := parts.([]any)
	if !ok {
		return false
	}
	for _, item := range arr {
		pm, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if ref := extractReference(pm["actor"]); ref != "" && matchesOwnedRef(ref, oc) {
			return true
		}
	}
	return false
}

// extractReference gets the "reference" field from a FHIR reference object.
func extractReference(v any) string {
	if m, ok := v.(map[string]any); ok {
		if s, ok := m["reference"].(string); ok {
			return s
		}
	}
	return ""
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

// matchesOwnedRef checks "Patient/{id}" and "Practitioner/{id}" against ownershipContext.
func matchesOwnedRef(ref string, oc *ownershipContext) bool {
	if strings.HasPrefix(ref, "Patient/") {
		id := strings.TrimPrefix(ref, "Patient/")
		_, ok := oc.PatientIDs[id]
		return ok
	}
	if strings.HasPrefix(ref, "Practitioner/") {
		id := strings.TrimPrefix(ref, "Practitioner/")
		_, ok := oc.PractitionerIDs[id]
		return ok
	}

	if strings.HasPrefix(ref, "PractitionerRole/") {
		id := strings.TrimPrefix(ref, "PractitionerRole/")
		if slices.Contains(oc.PractitionerRoleIDs, id) {
			return true
		}
	}

	return false
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
func buildTxProxyURL(target string, r *http.Request, prefix, version string) string {
	q := r.URL.Query()
	hasFilter := strings.TrimSpace(q.Get("filter")) != ""
	hasCount := strings.TrimSpace(q.Get("count")) != ""
	if !hasFilter && !hasCount {
		return ""
	}
	relativePath := strings.TrimPrefix(r.URL.Path, fmt.Sprintf("/%s/%s/tx", prefix, version))
	fullURL := target + relativePath
	if r.URL.RawQuery != "" {
		fullURL += "?" + r.URL.RawQuery
	}
	return fullURL
}

func (m *Middlewares) TxProxy(target string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := m.authTxProxyAccess(w, r); err != nil {
			return
		}

		fullURL := buildTxProxyURL(target, r, m.InternalConfig.App.EndpointPrefix, m.InternalConfig.App.Version)
		if fullURL == "" {
			w.WriteHeader(http.StatusAccepted)
			return
		}

		bodyBytes, _ := r.Context().Value(constvars.CONTEXT_RAW_BODY).([]byte)
		if bodyBytes == nil {
			bodyBytes = []byte{}
		}

		m.doTxProxyRequest(w, r, fullURL, bodyBytes)
	})
}

// doTxProxyRequest executes a proxy request to the terminology server and streams the response.
func (m *Middlewares) doTxProxyRequest(w http.ResponseWriter, r *http.Request, fullURL string, bodyBytes []byte) {
	proxyCtx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(proxyCtx, r.Method, fullURL, bytes.NewReader(bodyBytes))
	if err != nil {
		utils.BuildErrorResponse(m.Log, w, exceptions.ErrCreateHTTPRequest(err))
		return
	}

	req.Header.Set("Accept", "application/fhir+json")
	if contentType := r.Header.Get("Content-Type"); contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := m.HTTPClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(proxyCtx.Err(), context.DeadlineExceeded) {
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(m.Log, w, exceptions.ErrSendHTTPRequest(err))
		return
	}
	defer resp.Body.Close()

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
func collectReferences(v any, out *[]string, depth int) {
	// prevent infinite recursion. 30 is arbitrary.
	if depth > 30 {
		return
	}

	switch t := v.(type) {
	case map[string]any:
		for k, vv := range t {
			if k == "reference" {
				if s, ok := vv.(string); ok {
					*out = append(*out, s)
				}
			} else {
				collectReferences(vv, out, depth+1)
			}
		}
	case []any:
		for _, vv := range t {
			collectReferences(vv, out, depth+1)
		}
	}
}
