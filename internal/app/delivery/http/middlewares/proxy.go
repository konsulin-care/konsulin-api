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
)

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

		bodyAfterRBAC := respBody
		filteredRBAC := false
		removedRBAC := 0

		filteringRole := determineFilteringRole(roles)
		if filteringRole != "" {
			switch filteringRole {
			case constvars.KonsulinRoleSuperadmin:
				b, removed, err := m.filterResponseResourceAgainstRBAC(bodyAfterRBAC, roles)
				if err != nil {
					m.Log.Warn("RBAC response filtering failed; resorting to fail closed on error", zap.Error(err))
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

		if r.Method == http.MethodGet && fhirRole != "" {
			if bundle, enc, isBundle, derr := decodeBundle(bodyAfterRBAC); derr == nil && isBundle {
				removedOwnership = m.applyOwnershipFilterToBundle(r.Context(), bundle, roles, fhirRole, fhirID)
				if removedOwnership > 0 {
					filteredOwnership = true

					if bundle.Total != nil {
						v := len(bundle.Entry)
						bundle.Total = &v
					}

					fb, eerr := encodeBundle(bundle, enc)
					if eerr != nil {
						m.Log.Warn("encodeBundle after ownership filtering failed; resorting to fail closed on error", zap.Error(err))
						utils.BuildErrorResponse(m.Log, w, exceptions.ErrServerProcess(err))
						return
					}

					bodyAfterOwnership = fb
				}
			} else {
				filteredBody, allowed, ferr := m.filterSingleResourceByOwnership(r.Context(), bodyAfterRBAC, roles, fhirRole, fhirID)
				if ferr != nil {
					m.Log.Info(fmt.Sprintf("single-resource ownership filtering failed for {%s/%s}; resorting to fail closed on error", fhirRole, fhirID), zap.Error(ferr))
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

		finalBody := bodyAfterOwnership
		mutated := filteredRBAC || filteredOwnership

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

	type originalEncoding string
	const (
		encodingIdentity originalEncoding = "identity"
		encodingBrotli   originalEncoding = "br"
		encodingGzip     originalEncoding = "gzip"
	)

	bodyForFiltering := body
	encDetected := encodingIdentity

	tryUnmarshal := func(b []byte, v any) error { return json.Unmarshal(b, v) }

	var envelope struct {
		ResourceType string `json:"resourceType"`
	}
	if err := tryUnmarshal(bodyForFiltering, &envelope); err != nil {

		if brReader := brotli.NewReader(bytes.NewReader(body)); brReader != nil {
			if decompressed, derr := io.ReadAll(brReader); derr == nil {
				if jerr := tryUnmarshal(decompressed, &envelope); jerr == nil {
					bodyForFiltering = decompressed
					encDetected = encodingBrotli
				} else {

					if gr, gerr := gzip.NewReader(bytes.NewReader(body)); gerr == nil {
						decompressedGz, rerr := io.ReadAll(gr)
						_ = gr.Close()
						if rerr == nil && tryUnmarshal(decompressedGz, &envelope) == nil {
							bodyForFiltering = decompressedGz
							encDetected = encodingGzip
						} else {

							return body, 0, nil
						}
					} else {

						return body, 0, nil
					}
				}
			} else {

				if gr, gerr := gzip.NewReader(bytes.NewReader(body)); gerr == nil {
					decompressedGz, rerr := io.ReadAll(gr)
					_ = gr.Close()
					if rerr == nil && tryUnmarshal(decompressedGz, &envelope) == nil {
						bodyForFiltering = decompressedGz
						encDetected = encodingGzip
					} else {
						return body, 0, nil
					}
				} else {
					return body, 0, nil
				}
			}
		}
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

	if err := json.Unmarshal(bodyForFiltering, &bundle); err != nil {
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

	switch encDetected {
	case encodingIdentity:
		return filteredJSON, removed, nil
	case encodingBrotli:
		var buf bytes.Buffer
		bw := brotli.NewWriterLevel(&buf, brotli.BestCompression)
		if _, err := bw.Write(filteredJSON); err != nil {
			_ = bw.Close()
			return body, 0, err
		}
		if err := bw.Close(); err != nil {
			return body, 0, err
		}
		return buf.Bytes(), removed, nil
	case encodingGzip:
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		if _, err := gw.Write(filteredJSON); err != nil {
			_ = gw.Close()
			return body, 0, err
		}
		if err := gw.Close(); err != nil {
			return body, 0, err
		}
		return buf.Bytes(), removed, nil
	default:
		return filteredJSON, removed, nil
	}
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

// decodeBundle attempts to detect encoding (identity, br, gzip) and unmarshal a FHIR Bundle.
// It returns (bundle, encoding, isBundle, error).
func decodeBundle(body []byte) (*Bundle, string, bool, error) {
	const (
		encodingIdentity = "identity"
		encodingBrotli   = "br"
		encodingGzip     = "gzip"
	)

	enc := encodingIdentity
	bodyForFiltering := body

	tryUnmarshal := func(b []byte, v any) error { return json.Unmarshal(b, v) }

	var envelope struct {
		ResourceType string `json:"resourceType"`
	}
	if err := tryUnmarshal(bodyForFiltering, &envelope); err != nil {

		if brReader := brotli.NewReader(bytes.NewReader(body)); brReader != nil {
			if decompressed, derr := io.ReadAll(brReader); derr == nil {
				if jerr := tryUnmarshal(decompressed, &envelope); jerr == nil {
					bodyForFiltering = decompressed
					enc = encodingBrotli
				} else {

					if gr, gerr := gzip.NewReader(bytes.NewReader(body)); gerr == nil {
						decompressedGz, rerr := io.ReadAll(gr)
						_ = gr.Close()
						if rerr == nil && tryUnmarshal(decompressedGz, &envelope) == nil {
							bodyForFiltering = decompressedGz
							enc = encodingGzip
						} else {

							return nil, "", false, nil
						}
					} else {

						return nil, "", false, nil
					}
				}
			} else {

				if gr, gerr := gzip.NewReader(bytes.NewReader(body)); gerr == nil {
					decompressedGz, rerr := io.ReadAll(gr)
					_ = gr.Close()
					if rerr == nil && tryUnmarshal(decompressedGz, &envelope) == nil {
						bodyForFiltering = decompressedGz
						enc = encodingGzip
					} else {
						return nil, "", false, nil
					}
				} else {
					return nil, "", false, nil
				}
			}
		}

	}

	if !strings.EqualFold(envelope.ResourceType, "Bundle") {
		return nil, "", false, nil
	}

	var bundle Bundle
	if err := json.Unmarshal(bodyForFiltering, &bundle); err != nil {
		return nil, "", false, nil
	}
	return &bundle, enc, true, nil
}

// encodeBundle marshals a Bundle and re-applies the original encoding.
func encodeBundle(bundle *Bundle, enc string) ([]byte, error) {
	const (
		encodingIdentity = "identity"
		encodingBrotli   = "br"
		encodingGzip     = "gzip"
	)

	filteredJSON, err := json.Marshal(bundle)
	if err != nil {
		return nil, err
	}

	switch enc {
	case encodingIdentity, "":
		return filteredJSON, nil
	case encodingBrotli:
		var buf bytes.Buffer
		bw := brotli.NewWriterLevel(&buf, brotli.BestCompression)
		if _, err := bw.Write(filteredJSON); err != nil {
			_ = bw.Close()
			return nil, err
		}
		if err := bw.Close(); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	case encodingGzip:
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		if _, err := gw.Write(filteredJSON); err != nil {
			_ = gw.Close()
			return nil, err
		}
		if err := gw.Close(); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	default:
		return filteredJSON, nil
	}
}

// ownershipContext describes what FHIR resources (Patient / Practitioner) the caller owns.
type ownershipContext struct {
	HasPatientRole      bool
	HasPractitionerRole bool
	PatientIDs          map[string]struct{}
	PractitionerIDs     map[string]struct{}
}

// buildOwnershipContext resolves owned Patient / Practitioner IDs once per request.
func (m *Middlewares) buildOwnershipContext(
	ctx context.Context,
	roles []string,
	fhirRole, fhirID string,
) *ownershipContext {
	oc := &ownershipContext{
		PatientIDs:      make(map[string]struct{}),
		PractitionerIDs: make(map[string]struct{}),
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

	// Attempt to unmarshal directly; if it fails, try brotli / gzip like decodeBundle.
	bodyForFiltering := body

	tryUnmarshal := func(b []byte, v any) error { return json.Unmarshal(b, v) }

	var env struct {
		ResourceType string `json:"resourceType"`
		ID           string `json:"id,omitempty"`
	}
	if err := tryUnmarshal(bodyForFiltering, &env); err != nil {
		if brReader := brotli.NewReader(bytes.NewReader(body)); brReader != nil {
			if decompressed, derr := io.ReadAll(brReader); derr == nil {
				if jerr := tryUnmarshal(decompressed, &env); jerr == nil {
					bodyForFiltering = decompressed
				} else {
					if gr, gerr := gzip.NewReader(bytes.NewReader(body)); gerr == nil {
						decompressedGz, rerr := io.ReadAll(gr)
						_ = gr.Close()
						if rerr == nil && tryUnmarshal(decompressedGz, &env) == nil {
							bodyForFiltering = decompressedGz
						} else {
							// cannot inspect safely → allow
							return body, true, nil
						}
					} else {
						return body, true, nil
					}
				}
			} else {
				if gr, gerr := gzip.NewReader(bytes.NewReader(body)); gerr == nil {
					decompressedGz, rerr := io.ReadAll(gr)
					_ = gr.Close()
					if rerr == nil && tryUnmarshal(decompressedGz, &env) == nil {
						bodyForFiltering = decompressedGz
					} else {
						return body, true, nil
					}
				} else {
					return body, true, nil
				}
			}
		}
	}

	if env.ResourceType == "" {
		// Not a FHIR resource or no type → do not filter.
		return body, true, nil
	}

	owned := m.resourceOwnedByContext(bodyForFiltering, env.ResourceType, env.ID, oc)
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
