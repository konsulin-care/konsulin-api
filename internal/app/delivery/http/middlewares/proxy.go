package middlewares

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"

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

func (m *Middlewares) Bridge(target string) http.Handler {
	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: &http.Transport{MaxIdleConnsPerHost: 100},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		bodyBytes, _ := r.Context().Value(constvars.CONTEXT_RAW_BODY).([]byte)
		if bodyBytes == nil {
			bodyBytes = []byte{}
		}

		req, err := http.NewRequestWithContext(r.Context(), r.Method, fullURL, bytes.NewReader(bodyBytes))
		if err != nil {
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrCreateHTTPRequest(err))
			return
		}
		req.Header = r.Header.Clone()

		if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
			req.Header.Set("Content-Type", "application/fhir+json")
		}
		req.Header.Set("Accept", "application/fhir+json")

		resp, err := client.Do(req)
		if err != nil {
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrSendHTTPRequest(err))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= http.StatusBadRequest {
			body, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				utils.BuildErrorResponse(m.Log, w, exceptions.ErrReadBody(readErr))
				return
			}

			fhirErr := exceptions.BuildNewCustomError(fmt.Errorf("%s", string(body)), resp.StatusCode, string(body), constvars.ErrDevServerProcess)
			utils.BuildErrorResponse(m.Log, w, fhirErr)
			return
		}

		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrReadBody(readErr))
			return
		}

		roles, _ := r.Context().Value(keyRoles).([]string)
		fhirRole, _ := r.Context().Value(keyFHIRRole).(string)
		fhirID, _ := r.Context().Value(keyFHIRID).(string)

		originalBody := respBody
		bodyForFilters := respBody
		encForFilters := bodyEncodingIdentity

		filteringRole := determineFilteringRole(roles)
		needsRBAC := filteringRole != ""
		needsOwnership := r.Method == http.MethodGet && fhirID != ""

		if needsRBAC || needsOwnership {
			decoded, enc, derr := decodeBodyForFiltering(respBody, resp.Header.Get("Content-Encoding"))
			if derr != nil {
				m.Log.Warn("failed to decode response body for filtering; failing closed", zap.Error(derr))
				utils.BuildErrorResponse(m.Log, w, exceptions.ErrServerProcess(derr))
				return
			}
			bodyForFilters = decoded
			encForFilters = enc
		}

		bodyAfterRBAC := bodyForFilters
		filteredRBAC := false
		removedRBAC := 0

		if needsRBAC {
			switch filteringRole {
			case constvars.KonsulinRoleSuperadmin:
				b, removed, err := m.filterResponseResourceAgainstRBAC(bodyAfterRBAC, roles)
				if err != nil {
					m.Log.Warn("RBAC response filtering failed; failing closed", zap.Error(err))
					utils.BuildErrorResponse(m.Log, w, exceptions.ErrServerProcess(err))
					return
				}

				bodyAfterRBAC = b
				removedRBAC = removed
				if removed > 0 {
					filteredRBAC = true
				}
			default:
				// other roles: no RBAC response filtering
			}
		}

		// Ownership-based filtering
		bodyAfterOwnership := bodyAfterRBAC
		filteredOwnership := false
		removedOwnership := 0

		if needsOwnership {
			if bundle, isBundle, _ := decodeBundle(bodyAfterRBAC); isBundle {
				removedOwnership = m.applyOwnershipFilterToBundle(r.Context(), bundle, roles, fhirRole, fhirID)
				if removedOwnership > 0 {
					filteredOwnership = true

					if bundle.Total != nil {
						v := len(bundle.Entry)
						bundle.Total = &v
					}

					fb, eerr := encodeBundle(bundle)
					if eerr != nil {
						m.Log.Warn("encodeBundle after ownership filtering failed; failing closed", zap.Error(eerr))
						utils.BuildErrorResponse(m.Log, w, exceptions.ErrServerProcess(eerr))
						return
					}

					bodyAfterOwnership = fb
				}
			} else {
				filteredBody, allowed, ferr := m.filterSingleResourceByOwnership(r.Context(), bodyAfterRBAC, roles, fhirRole, fhirID)
				if ferr != nil {
					m.Log.Info(fmt.Sprintf("single-resource ownership filtering failed for {%s/%s}; failing closed", fhirRole, fhirID), zap.Error(ferr))
					utils.BuildErrorResponse(m.Log, w, exceptions.ErrServerProcess(ferr))
					return
				}

				if !allowed {
					// Deny access when ownership cannot be proven.
					utils.BuildErrorResponse(m.Log, w, exceptions.ErrAuthInvalidRole(fmt.Errorf("forbidden: ownership cannot be proven")))
					return
				}

				if filteredBody != nil {
					bodyAfterOwnership = filteredBody
				}
			}
		}

		if filteredRBAC && removedRBAC > 0 {
			m.Log.Info("RBAC filtered response entries",
				zap.Int("removed", removedRBAC),
				zap.String("method", r.Method),
				zap.String("url", r.URL.RequestURI()),
				zap.Strings("roles", roles),
			)
		}
		if filteredOwnership && removedOwnership > 0 {
			m.Log.Info("Ownership filtered response entries",
				zap.Int("removed", removedOwnership),
				zap.String("method", r.Method),
				zap.String("url", r.URL.RequestURI()),
				zap.String("fhirRole", fhirRole),
				zap.String("fhirID", fhirID),
			)
		}

		mutated := filteredRBAC || filteredOwnership

		finalBody := originalBody
		if mutated {
			encoded, eerr := encodeBodyFromFiltering(bodyAfterOwnership, encForFilters)
			if eerr != nil {
				m.Log.Warn("failed to encode filtered response body; failing closed", zap.Error(eerr))
				utils.BuildErrorResponse(m.Log, w, exceptions.ErrServerProcess(eerr))
				return
			}
			finalBody = encoded
		}

		for k, v := range resp.Header {

			if strings.EqualFold(k, "Content-Length") {
				continue
			}
			w.Header()[k] = v
		}

		if mutated {
			w.Header().Del("ETag")
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(finalBody)))
		w.WriteHeader(resp.StatusCode)
		if _, err := w.Write(finalBody); err != nil {
			m.Log.Warn("failed writing response body", zap.Error(err))
		}
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

	var envelope struct {
		ResourceType string `json:"resourceType"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		// cannot inspect safely -> skip RBAC filtering
		return body, 0, nil
	}

	if !strings.EqualFold(envelope.ResourceType, "Bundle") {
		return body, 0, nil
	}

	type entry struct {
		FullURL  string          `json:"fullUrl,omitempty"`
		Resource json.RawMessage `json:"resource"`
		Search   map[string]any  `json:"search,omitempty"`
	}
	var bundle struct {
		ResourceType string  `json:"resourceType"`
		ID           string  `json:"id,omitempty"`
		Type         string  `json:"type,omitempty"`
		Total        *int    `json:"total,omitempty"`
		Link         any     `json:"link,omitempty"`
		Entry        []entry `json:"entry"`
	}

	if err := json.Unmarshal(body, &bundle); err != nil {
		return body, 0, nil
	}

	removed := 0
	filtered := make([]entry, 0, len(bundle.Entry))
	for _, e := range bundle.Entry {
		var resEnv struct {
			ResourceType string `json:"resourceType"`
		}
		if err := json.Unmarshal(e.Resource, &resEnv); err != nil {
			filtered = append(filtered, e)
			continue
		}
		if resEnv.ResourceType == "" {
			filtered = append(filtered, e)
			continue
		}

		allowedForAnyRole := false
		for _, role := range roles {

			if allowed(m.Enforcer, role, http.MethodGet, "/fhir/"+resEnv.ResourceType) {
				allowedForAnyRole = true
				break
			}
		}
		if allowedForAnyRole {
			filtered = append(filtered, e)
		} else {
			removed++
		}
	}

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

	if oc.HasPractitionerRole && len(oc.PatientIDs) == 0 && fhirID != "" {
		prac, err := m.PractitionerFhirClient.FindPractitionerByID(ctx, fhirID)
		if err == nil && prac != nil {
			emails := prac.GetEmailAddresses()
			for _, em := range emails {
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
			return oc
		}

		for _, pr := range practitionerRoles {
			if pr.ID != "" {
				oc.PractitionerRoleIDs = append(oc.PractitionerRoleIDs, pr.ID)
			}
		}
	}

	return oc
}

// ownershipChecker is a resource-specific, last-resort ownership function.
type ownershipChecker func(raw json.RawMessage, oc *ownershipContext) (bool, error)

// resourceSpecificOwnershipCheckers holds resource-specific ownership logic.
// add your own custom ownership checkers here if needed
var resourceSpecificOwnershipCheckers = map[string]ownershipChecker{}

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
	} else if err != nil {
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

	if checker, ok := resourceSpecificOwnershipCheckers[resourceType]; ok {
		if ok2, err := checker(raw, oc); err == nil && ok2 {
			return true
		} else if err != nil {
			if m.failClosedOnErrorFromResource(resourceType, id) {
				return false
			}

			m.Log.Warn("resorting to fail open on error from resource",
				zap.String("resourceType", resourceType),
				zap.String("id", id),
				zap.Error(err),
			)
		}
	}

	// If we reach here, we couldn't prove ownership.
	return false
}

// failClosedOnErrorFromResource is a function that determines if we should fail closed on error from a resource.
// this function behaviour comes from this discussion: https://github.com/konsulin-care/konsulin-api/pull/250#discussion_r2559068460
// This function must be used to determine if we should fail closed on error from a resource.
func (m *Middlewares) failClosedOnErrorFromResource(resourceType string, resourceID string) bool {
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

// applyOwnershipFilterToBundle mutates bundle.Entry in-place, keeping only owned resources.
func (m *Middlewares) applyOwnershipFilterToBundle(
	ctx context.Context,
	bundle *Bundle,
	roles []string,
	fhirRole, fhirID string,
) int {
	oc := m.buildOwnershipContext(ctx, roles, fhirRole, fhirID)

	removed := 0
	filtered := make([]BundleEntry, 0, len(bundle.Entry))

	for _, e := range bundle.Entry {
		keep := false
		var env struct {
			ResourceType string `json:"resourceType"`
			ID           string `json:"id,omitempty"`
		}
		if err := json.Unmarshal(e.Resource, &env); err != nil || env.ResourceType == "" {
			// can't inspect safely → keep
			filtered = append(filtered, e)
			continue
		}
		rt := env.ResourceType

		owned := m.resourceOwnedByContext(e.Resource, rt, env.ID, oc)
		if owned {
			keep = true
		}

		if keep {
			filtered = append(filtered, e)
		} else {
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

	extractRef := func(v any) string {
		if m, ok := v.(map[string]any); ok {
			if s, ok := m["reference"].(string); ok {
				return s
			}
		}
		return ""
	}

	// subject.reference
	if subj, ok := res["subject"]; ok {
		if ref := extractRef(subj); ref != "" && matchesOwnedRef(ref, oc) {
			return true, nil
		}
	}

	// patient.reference
	if pat, ok := res["patient"]; ok {
		if ref := extractRef(pat); ref != "" && matchesOwnedRef(ref, oc) {
			return true, nil
		}
	}

	// recipient.reference
	if rec, ok := res["recipient"]; ok {
		if ref := extractRef(rec); ref != "" && matchesOwnedRef(ref, oc) {
			return true, nil
		}
	}

	// actor.reference at root
	if act, ok := res["actor"]; ok {
		if ref := extractRef(act); ref != "" && matchesOwnedRef(ref, oc) {
			return true, nil
		}
	}

	// participant[*].actor.reference
	if parts, ok := res["participant"]; ok {
		if arr, ok := parts.([]any); ok {
			for _, item := range arr {
				if pm, ok := item.(map[string]any); ok {
					if actor, ok := pm["actor"]; ok {
						if ref := extractRef(actor); ref != "" && matchesOwnedRef(ref, oc) {
							return true, nil
						}
					}
				}
			}
		}
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
