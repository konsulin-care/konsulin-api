package middlewares

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/ownership"
	"konsulin-service/internal/pkg/utils"

	"github.com/casbin/casbin/v2"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// handleAuthPostBody reads and validates the POST request body, restoring it for downstream use.
func handleAuthPostBody(ctxIface context.Context, r *http.Request, fhirRole, fhirID string) error {
	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()

	if err := validatePostRequestBody(ctxIface, body, fhirRole, fhirID); err != nil {
		return err
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	return nil
}

// handleAuthBundle scans the bundle request body for authorization and returns whether it was a bundle.
func (m *Middlewares) handleAuthBundle(ctxIface context.Context, r *http.Request, roles []string) (bool, error) {
	if !isBundle(r) {
		return false, nil
	}
	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()

	fhirID, _ := ctxIface.Value(keyFHIRID).(string)
	err := scanBundle(ctxIface, m.Enforcer, body, roles, fhirID, nil, m.rbacClients())
	if err != nil {
		// Lazy dual-identity retry: only sessions holding both Patient and
		// Practitioner roles pay for the secondary FHIR lookup, and only when
		// the single active-role identity could not authorize the bundle.
		if ids := m.secondaryFHIRIDsByRole(ctxIface, roles); ids != nil {
			err = scanBundle(ctxIface, m.Enforcer, body, roles, fhirID, ids, m.rbacClients())
		}
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return true, err
}

// handleAuthSingleResource validates a single FHIR resource request.
func (m *Middlewares) handleAuthSingleResource(ctxIface context.Context, r *http.Request, roles []string) error {
	fullURL := r.URL.RequestURI()
	fhirID, _ := ctxIface.Value(keyFHIRID).(string)

	var resourceBody []byte
	if r.Method == constvars.MethodPut || r.Method == constvars.MethodPost {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		resourceBody = body
		r.Body = io.NopCloser(bytes.NewReader(body))
	}

	// B3: referral Communications are PUT-only, deterministic resources. A POST
	// create carrying a referral- id would let a caller forge an edge id.
	if err := rejectReferralPOST(r.Method, gjson.GetBytes(resourceBody, "id").String()); err != nil {
		return err
	}

	// B3: validate referral Communication PUTs before any RBAC/ownership dispatch.
	if r.Method == constvars.MethodPut {
		if res, id := extractPathResourceID(fullURL); res == constvars.ResourceCommunication && isReferralID(id) {
			if err := m.validateReferralCommunication(ctxIface, roles, fhirID, id, resourceBody); err != nil {
				return err
			}
		}
	}

	req := rbacRequest{
		method:         r.Method,
		normalizedPath: normalizePath(fullURL),
		fhirID:         fhirID,
		url:            fullURL,
		resource:       resourceBody,
	}
	err := checkSingle(ctxIface, m.Enforcer, req, roles, m.rbacClients())
	if err != nil {
		// Lazy dual-identity retry, mirroring handleAuthBundle: the referral
		// pre-checks above run once; only the RBAC/ownership dispatch retries.
		if ids := m.secondaryFHIRIDsByRole(ctxIface, roles); ids != nil {
			req.fhirIDsByRole = ids
			err = checkSingle(ctxIface, m.Enforcer, req, roles, m.rbacClients())
		}
	}
	return err
}

func (m *Middlewares) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxIface := r.Context()
		roles, _ := ctxIface.Value(keyRoles).([]string)
		uid, _ := ctxIface.Value(keyUID).(string)

		fhirRole, fhirID, err := m.ResolveUserRoles(ctxIface, roles, uid)
		if err != nil {
			m.Log.Error("Auth.resolveUserRoles", zap.Error(err))
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrAuthInvalidRole(err))
			return
		}

		ctxIface = context.WithValue(ctxIface, keyFHIRRole, fhirRole)
		ctxIface = context.WithValue(ctxIface, keyFHIRID, fhirID)
		r = r.WithContext(ctxIface)

		if r.Method == constvars.MethodPost {
			if err := handleAuthPostBody(ctxIface, r, fhirRole, fhirID); err != nil {
				utils.BuildErrorResponse(m.Log, w, exceptions.ErrAuthInvalidRole(err))
				return
			}
		}

		if isBundle, err := m.handleAuthBundle(ctxIface, r, roles); err != nil {
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrAuthInvalidRole(err))
			return
		} else if isBundle {
			next.ServeHTTP(w, r)
			return
		}

		if err := m.handleAuthSingleResource(ctxIface, r, roles); err != nil {
			// Referral validation rejections are client-forbidden (403); all other
			// authorization failures keep the default unauthorized (401) mapping.
			var rfErr *referralForbiddenError
			if errors.As(err, &rfErr) {
				utils.BuildErrorResponse(m.Log, w, exceptions.BuildNewCustomError(err, constvars.StatusForbidden, constvars.ErrClientNotAuthorized, "forbidden"))
				return
			}
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrAuthInvalidRole(err))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isOnlyGuest returns true if the only role in the slice is Guest.
func isOnlyGuest(roles []string) bool {
	if len(roles) != 1 {
		return false
	}
	return strings.EqualFold(roles[0], constvars.KonsulinRoleGuest)
}

// needsFHIRResolution returns true if any role in the slice requires FHIR
// identity resolution. Clinic Admin and Researcher resolve to their
// Practitioner FHIR identity (they are practitioners with a specialized
// PractitionerRole coding); Guest has no FHIR identity.
func needsFHIRResolution(roles []string) bool {
	for _, role := range roles {
		switch role {
		case constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner,
			constvars.KonsulinRoleResearcher, constvars.KonsulinRoleClinicAdmin:
			return true
		}
	}
	return false
}

// ResolveUserRoles determines the FHIR role and ID to use for authorization.
// Returns (fhirRole, fhirID, error). For Guest and non-FHIR roles, fhirID is empty.
// Backend routes outside the /fhir proxy mount (e.g. the purge endpoint) use
// it to resolve the caller's FHIR identity from the session context values.
func (m *Middlewares) ResolveUserRoles(ctx context.Context, roles []string, uid string) (string, string, error) {
	if len(roles) == 1 && roles[0] == constvars.KonsulinRoleSuperadmin && uid == "api-key-superadmin" {
		return constvars.KonsulinRoleSuperadmin, "", nil
	}
	if !isOnlyGuest(roles) && needsFHIRResolution(roles) {
		fhirRole, fhirID, err := m.resolveFHIRIdentity(ctx, uid)
		if err != nil {
			return "", "", err
		}
		return fhirRole, fhirID, nil
	}
	if !isOnlyGuest(roles) {
		for _, role := range roles {
			if role != constvars.KonsulinRoleGuest {
				return role, "", nil
			}
		}
		return constvars.KonsulinRoleGuest, "", nil
	}
	return constvars.KonsulinRoleGuest, "", nil
}

func validatePostRequestBody(ctx context.Context, body []byte, fhirRole, fhirID string) error {
	if fhirRole == "" || fhirRole == constvars.KonsulinRoleGuest {
		return nil
	}

	resourceType := gjson.GetBytes(body, "resourceType").String()
	if resourceType == "" {
		return nil
	}

	resourceTypeFromPath := utils.ExtractResourceTypeFromPath("/fhir/" + resourceType)

	// Transaction bundles are authorized per-entry by handleAuthBundle's
	// scanBundle; the single-resource body check must not reject the Bundle
	// wrapper itself (the ownership spec carries no "Bundle" rule).
	if resourceTypeFromPath == constvars.ResourceBundle {
		return nil
	}

	roles, _ := ctx.Value(keyRoles).([]string)
	oc := ownershipContextFromRoles(roles, fhirRole, fhirID)
	if err := ownership.ValidateWriteBody(body, resourceTypeFromPath, oc, false); err != nil {
		return err
	}

	// Body-only named write checkers apply to POST creates too. The
	// canonical-state checkers (schedule, questionnaire_response) stay PUT-only:
	// they verify the stored resource by id, which a create cannot satisfy.
	rule, ok := ownership.Rule(resourceTypeFromPath)
	if !ok || rule.WriteCheckerName == "" || writeCheckBypassed(oc, rule) {
		return nil
	}
	switch rule.WriteCheckerName {
	case "slot":
		if fhirRole == constvars.KonsulinRolePatient && !validatePatientSlotResource(string(body)) {
			return errors.New("slot POST by patient must have status busy or busy-unavailable")
		}
	case "invoice":
		if !validatePractitionerInvoiceResource(string(body), fhirID) {
			return errors.New("invoice POST must reference the caller's own practitioner or a PractitionerRole actor")
		}
	}
	return nil
}

// validateCommunicationSenderInBody enforces that a Communication POST body's
// sender references the caller's own Patient ID, driven by the ownership
// engine's Communication rule. A missing sender is lenient (nil); a malformed
// reference or a mismatched patient is an error.
func validateCommunicationSenderInBody(body []byte, patientID string) error {
	oc := ownership.NewContext()
	oc.HasPatientRole = true
	oc.AddPatientID(patientID)
	return ownership.ValidateWriteBody(body, constvars.ResourceCommunication, oc, false)
}

// validatePatientCommunicationSender reports whether a Communication resource
// is owned by the given patient via its sender reference, driven by the
// ownership engine's Communication rule (strict PUT semantics).
func validatePatientCommunicationSender(resourceStr, fhirID string) bool {
	oc := ownership.NewContext()
	oc.HasPatientRole = true
	oc.AddPatientID(fhirID)
	return ownership.ValidateWriteBody([]byte(resourceStr), constvars.ResourceCommunication, oc, true) == nil
}

// ownsPostBody validates POST create bodies for patient-scoped resources.
// Consent (patient.reference) and ResearchSubject (individual.reference) must
// point at the caller's own Patient ID so a patient cannot consent on behalf
// of another patient. Non-patient roles, unrelated resource types, empty
// bodies, and missing reference fields stay lenient.
func ownsPostBody(fhirID, role, resourceType string, resource []byte) bool {
	if role != constvars.KonsulinRolePatient || len(resource) == 0 {
		return true
	}
	switch resourceType {
	case constvars.ResourceConsent, constvars.ResourceResearchSubject:
		oc := ownership.NewContext()
		oc.HasPatientRole = true
		oc.AddPatientID(fhirID)
		return ownership.ValidateWriteBody(resource, resourceType, oc, false) == nil
	default:
		return true
	}
}

// lookupPatient looks up a Patient FHIR resource by Supertoken UID.
// Returns the Patient ID if found, or an empty string if not found.
// Returns an error if the FHIR query fails or if multiple Patient resources match.
func (m *Middlewares) lookupPatient(ctx context.Context, uid string) (string, error) {
	pats, err := m.PatientFhirClient.FindPatientByIdentifier(
		ctx,
		fmt.Sprintf("%s|%s", constvars.FhirSupertokenSystemIdentifier, uid),
	)
	if err != nil {
		return "", err
	}
	if len(pats) > 1 {
		return "", fmt.Errorf("multiple Patient resources for uid %s", uid)
	}
	if len(pats) == 1 {
		return pats[0].ID, nil
	}
	return "", nil
}

// holdsDualFHIRRoles returns true when the session holds both the Patient and
// Practitioner roles, i.e. it may own FHIR identities of both types.
func holdsDualFHIRRoles(roles []string) bool {
	return hasRole(roles, constvars.KonsulinRolePatient) &&
		hasRole(roles, constvars.KonsulinRolePractitioner)
}

// secondaryRoleFor returns the FHIR role that is NOT the given primary role,
// or "" when the primary role is not a FHIR identity role.
func secondaryRoleFor(primaryRole string) string {
	switch primaryRole {
	case constvars.KonsulinRolePatient:
		return constvars.KonsulinRolePractitioner
	case constvars.KonsulinRolePractitioner:
		return constvars.KonsulinRolePatient
	}
	return ""
}

// resolveSecondaryFHIRID resolves the FHIR ID of the identity the caller is
// NOT actively using. With active role Patient it looks up the Practitioner;
// with active role Practitioner it looks up the Patient. Resolution is
// lenient: errors, multiple matches, and missing identities all yield "".
func (m *Middlewares) resolveSecondaryFHIRID(ctx context.Context, uid, primaryRole string) string {
	switch primaryRole {
	case constvars.KonsulinRolePatient:
		pracs, err := m.PractitionerFhirClient.FindPractitionerByIdentifier(
			ctx,
			constvars.FhirSupertokenSystemIdentifier,
			uid,
		)
		if err != nil || len(pracs) != 1 {
			return ""
		}
		return pracs[0].ID
	case constvars.KonsulinRolePractitioner:
		patID, err := m.lookupPatient(ctx, uid)
		if err != nil {
			return ""
		}
		return patID
	}
	return ""
}

// secondaryFHIRIDsByRole resolves the non-active FHIR identity for a dual-role
// session into a per-role ownership map (at most one entry). Returns nil for
// single-identity sessions so callers keep today's single-ID behavior.
func (m *Middlewares) secondaryFHIRIDsByRole(ctx context.Context, roles []string) map[string]string {
	if !holdsDualFHIRRoles(roles) {
		return nil
	}
	uid, _ := ctx.Value(keyUID).(string)
	fhirRole, _ := ctx.Value(keyFHIRRole).(string)
	secondary := secondaryRoleFor(fhirRole)
	if uid == "" || secondary == "" {
		return nil
	}
	return map[string]string{secondary: m.resolveSecondaryFHIRID(ctx, uid, fhirRole)}
}

func (m *Middlewares) resolveFHIRIdentity(ctx context.Context, uid string) (role, id string, err error) {
	activeRole, _ := ctx.Value(keyActiveRole).(string)

	if activeRole == constvars.KonsulinRolePatient {
		patID, err := m.lookupPatient(ctx, uid)
		if err != nil {
			return "", "", err
		}
		if patID != "" {
			return constvars.KonsulinRolePatient, patID, nil
		}
	}

	pracs, err := m.PractitionerFhirClient.FindPractitionerByIdentifier(
		ctx,
		constvars.FhirSupertokenSystemIdentifier,
		uid,
	)
	if err != nil {
		return "", "", err
	}

	if len(pracs) > 0 {
		if len(pracs) > 1 {
			return "", "", fmt.Errorf("multiple Practitioner resources for uid %s", uid)
		}
		return constvars.KonsulinRolePractitioner, pracs[0].ID, nil
	}

	if activeRole != constvars.KonsulinRolePatient {
		patID, err := m.lookupPatient(ctx, uid)
		if err != nil {
			return "", "", err
		}
		if patID != "" {
			return constvars.KonsulinRolePatient, patID, nil
		}
	}
	return "", "", fmt.Errorf("no Practitioner/Patient found for uid %s", uid)
}

// rbacClients bundles the FHIR clients needed for RBAC ownership checks.
type rbacClients struct {
	patient               contracts.PatientFhirClient
	practitioner          contracts.PractitionerFhirClient
	practitionerRole      contracts.PractitionerRoleFhirClient
	schedule              contracts.ScheduleFhirClient
	questionnaireResponse contracts.QuestionnaireResponseFhirClient
}

// rbacRequest bundles the request data needed for RBAC and ownership checks.
type rbacRequest struct {
	method         string
	normalizedPath string
	fhirID         string
	url            string
	resource       []byte
	// fhirIDsByRole carries a per-role ownership baseline for dual-identity
	// sessions (Patient AND Practitioner). It is populated lazily, only when
	// the single active-role identity cannot authorize the request.
	fhirIDsByRole map[string]string
}

// fhirIDForRole returns the FHIR ID to use as the ownership baseline for the
// given role. When the per-role map is present and names the role, that ID
// wins; otherwise the single active-role fhirID is used (nil map keeps today's
// behavior for single-identity sessions).
func (r rbacRequest) fhirIDForRole(role string) string {
	if id, ok := r.fhirIDsByRole[role]; ok {
		return id
	}
	return r.fhirID
}

// rbacClients returns the middleware's FHIR clients bundled for RBAC checks.
func (m *Middlewares) rbacClients() rbacClients {
	return rbacClients{
		patient:               m.PatientFhirClient,
		practitioner:          m.PractitionerFhirClient,
		practitionerRole:      m.PractitionerRoleFhirClient,
		schedule:              m.ScheduleFhirClient,
		questionnaireResponse: m.QuestionnaireResponseFhirClient,
	}
}

func scanBundle(ctx context.Context, e *casbin.Enforcer, raw []byte, roles []string, uid string, fhirIDsByRole map[string]string, clients rbacClients) error {
	if gjson.GetBytes(raw, "resourceType").String() != constvars.ResourceBundle {
		return fmt.Errorf("invalid bundle")
	}
	entries := gjson.GetBytes(raw, "entry").Array()
	for _, entry := range entries {
		method := entry.Get("request.method").String()
		url := entry.Get("request.url").String()
		resource := entry.Get("resource").Raw
		req := rbacRequest{
			method:         method,
			normalizedPath: normalizePath(url),
			fhirID:         uid,
			fhirIDsByRole:  fhirIDsByRole,
			url:            url,
			resource:       []byte(resource),
		}
		if err := checkSingle(ctx, e, req, roles, clients); err != nil {
			return err
		}
	}
	return nil
}

func checkSingle(ctx context.Context, e *casbin.Enforcer, req rbacRequest, roles []string, clients rbacClients) error {
	resourceType := utils.ExtractResourceTypeFromPath(req.normalizedPath)

	// C3: entry-level QuestionnaireResponse / Communication GET reads must carry
	// an identity scope (aggregate _summary=count stays public). Closes the
	// open-endpoint hole where a bare query returned every response. The
	// per-role map carries the secondary identity for dual-role sessions.
	if err := checkScopedEntryRead(ctx, req.method, resourceType, roles, req.fhirID, req.url, req.fhirIDsByRole); err != nil {
		return err
	}

	// B3: referral Communication PUTs are fully validated in
	// handleAuthSingleResource (deterministic body + live sender/batch checks,
	// patient-only). Allow them through the RBAC/ownership dispatch — the policy
	// grants nobody a PUT on Communication, and non-referral Communication
	// writes stay rejected below.
	if isReferralCommunicationPut(req.method, resourceType, req.normalizedPath) {
		return nil
	}

	// direct request to public resource is allowed to bypass RBAC checks
	// but only for GET requests to avoid unwanted modifications
	if isPublicRule(resourceType) && req.method == http.MethodGet {
		return nil
	}

	return enforceRBAC(ctx, e, req, roles, clients)
}

// isPublicRule reports whether the ownership spec classifies the resource type
// as public (readable by any caller without ownership proof).
func isPublicRule(resourceType string) bool {
	rule, ok := ownership.Rule(resourceType)
	return ok && rule.Scope == ownership.ScopePublic
}

// checkScopedEntryRead enforces that entry-level QuestionnaireResponse /
// Communication GET reads carry an identity scope, driven by the ownership
// engine's ValidSearchQuery (rule SearchParams + code-conditioned search
// allowances). Returns nil when the request needs no scope or satisfies it.
// For dual-identity sessions the fhirIDsByRole map seeds the non-active
// identity's own Patient/Practitioner id, mirroring buildOwnershipContext so
// the pre-request gate accepts the same patient-self scopes the response
// filter would allow.
func checkScopedEntryRead(ctx context.Context, method, resourceType string, roles []string, fhirID, rawURL string, fhirIDsByRole map[string]string) error {
	if method != http.MethodGet || !isScopedEntryResource(resourceType) {
		return nil
	}
	fhirRole, _ := ctx.Value(keyFHIRRole).(string)
	oc := ownershipContextFromRoles(roles, fhirRole, fhirID)
	if id, ok := fhirIDsByRole[constvars.KonsulinRolePatient]; ok {
		oc.AddPatientID(id)
	}
	if id, ok := fhirIDsByRole[constvars.KonsulinRolePractitioner]; ok {
		oc.AddPractitionerID(id)
	}
	if ownership.ValidSearchQuery(rawURL, resourceType, oc) {
		return nil
	}
	return fmt.Errorf("forbidden: entry-level %s read requires an identity scope", resourceType)
}

// isReferralCommunicationPut reports whether the request is a referral
// Communication PUT, which bypasses the RBAC/ownership dispatch (it is fully
// validated in handleAuthSingleResource instead).
func isReferralCommunicationPut(method, resourceType, normalizedPath string) bool {
	if method != constvars.MethodPut || resourceType != constvars.ResourceCommunication {
		return false
	}
	_, id := extractPathResourceID(normalizedPath)
	return isReferralID(id)
}

// enforceRBAC applies the Casbin policy for each session role and, for the
// FHIR identity roles (Patient, Practitioner, Researcher, Clinic Admin), the
// ownership check on the target resource. Researcher and Clinic Admin resolve
// as Practitioner identities; code-conditioned ownership rules gate their
// reads/writes. Returns an error when no role authorizes the request.
func enforceRBAC(ctx context.Context, e *casbin.Enforcer, req rbacRequest, roles []string, clients rbacClients) error {
	for _, role := range roles {
		if allowed(e, role, req.method, req.normalizedPath) {
			if isFHIRIdentityRole(role) {
				ok := ownsResource(ctx, req.fhirIDForRole(role), req.url, role, req.method, clients, req.resource)
				if ok {
					return nil
				}
				continue
			}
			return nil
		}
	}
	return fmt.Errorf("forbidden")
}

// isFHIRIdentityRole reports whether the role carries a FHIR identity that the
// ownership engine scopes (Clinic Admin and Researcher are practitioners).
func isFHIRIdentityRole(role string) bool {
	switch role {
	case constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner,
		constvars.KonsulinRoleResearcher, constvars.KonsulinRoleClinicAdmin:
		return true
	}
	return false
}

func allowed(e *casbin.Enforcer, role, method, path string) bool {
	ok, err := e.Enforce(role, method, path)
	if err != nil {
		return false
	}
	return ok
}

// isScopedEntryResource reports whether the resource type is one whose
// entry-level (search) GET reads must carry an identity scope.
func isScopedEntryResource(resourceType string) bool {
	return resourceType == constvars.ResourceQuestionnaireResponse ||
		resourceType == constvars.ResourceCommunication
}

// queryHasOwnRef reports whether any of the given query params reference the
// provided FHIR id, tolerating the "Patient/" prefix. Every value of a param
// is scanned, including comma-separated FHIR reference lists.
func queryHasOwnRef(q url.Values, fhirID string, params ...string) bool {
	for _, key := range params {
		for _, val := range q[key] {
			for _, v := range strings.Split(val, ",") {
				v = strings.TrimPrefix(v, constvars.FHIRRefPrefixPatient)
				if v == fhirID {
					return true
				}
			}
		}
	}
	return false
}

// allowScopedEntryRead enforces that entry-level QuestionnaireResponse /
// Communication GET reads carry an identity scope, driven by the ownership
// engine's ValidSearchQuery (rule SearchParams + code-conditioned search
// allowances). Aggregate `_summary=count` and single-resource reads stay
// public; non-scoped resource types are exempt. The fhirIDsByRole map seeds
// the non-active identity for dual-role sessions, mirroring the request path.
func allowScopedEntryRead(roles []string, fhirID, rawURL, resourceType string, fhirIDsByRole map[string]string) bool {
	if !isScopedEntryResource(resourceType) {
		return true
	}
	oc := ownershipContextFromRoles(roles, inferredFHIRRole(roles), fhirID)
	if id, ok := fhirIDsByRole[constvars.KonsulinRolePatient]; ok {
		oc.AddPatientID(id)
	}
	if id, ok := fhirIDsByRole[constvars.KonsulinRolePractitioner]; ok {
		oc.AddPractitionerID(id)
	}
	return ownership.ValidSearchQuery(rawURL, resourceType, oc)
}

// inferredFHIRRole derives the active FHIR role from a single-role session
// (used by allowScopedEntryRead; the request path passes the resolved role).
func inferredFHIRRole(roles []string) string {
	hasPatient, hasPractitioner := false, false
	for _, r := range roles {
		hasPatient = hasPatient || strings.EqualFold(r, constvars.KonsulinRolePatient)
		hasPractitioner = hasPractitioner || strings.EqualFold(r, constvars.KonsulinRolePractitioner)
	}
	switch {
	case hasPatient && !hasPractitioner:
		return constvars.KonsulinRolePatient
	case hasPractitioner:
		return constvars.KonsulinRolePractitioner
	}
	return ""
}

func normalizePath(rawURL string) string {
	return utils.NormalizePath(rawURL)
}

// validateResourceOwnership validates a PUT body via the ownership engine's
// declarative WriteRefs (strict semantics) and, for exotic types, the named
// write checkers preserved from the legacy per-type logic (schedule, slot,
// invoice, questionnaire_response).
func validateResourceOwnership(ctx context.Context, fhirID, role, resourceType string, resource []byte, clients rbacClients) bool {
	oc := ownershipContextForRole(role, fhirID)
	rule, ok := ownership.Rule(resourceType)
	if !ok {
		return false
	}
	if err := ownership.ValidateWriteBody(resource, resourceType, oc, true); err != nil {
		return false
	}
	if rule.WriteCheckerName == "" || writeCheckBypassed(oc, rule) {
		return true
	}
	switch rule.WriteCheckerName {
	case "schedule":
		return validateScheduleOwnership(ctx, fhirID, string(resource), clients.practitionerRole, clients.schedule)
	case "slot":
		if role == constvars.KonsulinRolePatient {
			return validatePatientSlotResource(string(resource))
		}
		return true
	case "invoice":
		return validatePractitionerInvoiceResource(string(resource), fhirID)
	case "questionnaire_response":
		return validateQuestionnaireResponseOwner(ctx, fhirID, string(resource), clients.questionnaireResponse)
	}
	return true
}

// ownershipContextForRole builds a minimal OwnershipContext for the write path
// from the role being authorized and the caller's FHIR id. Researcher and
// Clinic Admin roles are practitioners carrying their specialized coding.
func ownershipContextForRole(role, fhirID string) *ownership.OwnershipContext {
	oc := ownership.NewContext()
	switch role {
	case constvars.KonsulinRolePatient:
		oc.HasPatientRole = true
		oc.AddPatientID(fhirID)
	case constvars.KonsulinRolePractitioner:
		oc.HasPractitionerRole = true
		oc.AddPractitionerID(fhirID)
	case constvars.KonsulinRoleResearcher:
		oc.HasPractitionerRole = true
		oc.AddPractitionerID(fhirID)
		oc.AddCoding(constvars.FhirPractitionerRoleSystemHL7, constvars.FhirPractitionerRoleCodeResearcher)
	case constvars.KonsulinRoleClinicAdmin:
		oc.HasPractitionerRole = true
		oc.AddPractitionerID(fhirID)
		oc.AddCoding(constvars.FhirPractitionerRoleSystemSnomed, constvars.FhirPractitionerRoleCodeAdministrativeStaff)
	}
	return oc
}

// writeCheckBypassed reports whether a code-exempt caller (e.g. a clinic admin
// managing invoices or roles for other practitioners) skips the rule's named
// write checker.
func writeCheckBypassed(oc *ownership.OwnershipContext, rule ownership.ResourceRule) bool {
	for _, c := range rule.WriteBypassCodes {
		if oc.HoldsCoding(c) {
			return true
		}
	}
	return false
}

func validatePatientSlotResource(resourceStr string) bool {
	status := gjson.Get(resourceStr, "status").String()
	return status == "busy" || status == "busy-unavailable"
}

func validatePractitionerInvoiceResource(resourceStr, fhirID string) bool {
	for _, participant := range gjson.Get(resourceStr, "participant").Array() {
		actorRef := participant.Get("actor.reference").String()
		if strings.HasPrefix(actorRef, "PractitionerRole/") {
			return true
		}
		if strings.HasPrefix(actorRef, constvars.FHIRRefPrefixPractitioner) && strings.TrimPrefix(actorRef, constvars.FHIRRefPrefixPractitioner) == fhirID {
			return true
		}
	}
	return false
}

func validateQuestionnaireResponseOwner(ctx context.Context, fhirID, resourceStr string, client contracts.QuestionnaireResponseFhirClient) bool {
	id := gjson.Get(resourceStr, "id").String()
	if id == "" {
		return false
	}
	qr, err := client.FindQuestionnaireResponseByID(ctx, id)
	if err != nil {
		return false
	}
	authorRef := qr.Author.Reference
	subjectRef := qr.Subject.Reference
	if authorRef == "" && subjectRef == "" {
		return true
	}
	if strings.HasPrefix(authorRef, constvars.FHIRRefPrefixPatient) && strings.TrimPrefix(authorRef, constvars.FHIRRefPrefixPatient) != fhirID {
		return false
	}
	if strings.HasPrefix(subjectRef, constvars.FHIRRefPrefixPatient) && strings.TrimPrefix(subjectRef, constvars.FHIRRefPrefixPatient) != fhirID {
		return false
	}
	return true
}

// scheduleActorOwnedByPractitioner checks if any actor in the schedule references the practitioner.
func scheduleActorOwnedByPractitioner(ctx context.Context, actors []fhir_dto.Reference, fhirID string, practitionerRoleClient contracts.PractitionerRoleFhirClient) bool {
	for _, actor := range actors {
		actorRef := actor.Reference
		if strings.HasPrefix(actorRef, "PractitionerRole/") {
			roleID := strings.TrimPrefix(actorRef, "PractitionerRole/")
			pr, err := practitionerRoleClient.FindPractitionerRoleByID(ctx, roleID)
			if err != nil {
				continue
			}
			pracRef := pr.Practitioner.Reference
			if strings.HasPrefix(pracRef, constvars.FHIRRefPrefixPractitioner) && strings.TrimPrefix(pracRef, constvars.FHIRRefPrefixPractitioner) == fhirID {
				return true
			}
		}
		if strings.HasPrefix(actorRef, constvars.FHIRRefPrefixPractitioner) && strings.TrimPrefix(actorRef, constvars.FHIRRefPrefixPractitioner) == fhirID {
			return true
		}
	}
	return false
}

func validateScheduleOwnership(ctx context.Context, fhirID, resourceStr string, practitionerRoleClient contracts.PractitionerRoleFhirClient, scheduleClient contracts.ScheduleFhirClient) bool {
	scheduleID := gjson.Get(resourceStr, "id").String()
	if scheduleID == "" {
		return false
	}
	schedules, err := scheduleClient.Search(ctx, contracts.ScheduleSearchParams{ID: scheduleID})
	if err != nil || len(schedules) != 1 {
		return false
	}
	return scheduleActorOwnedByPractitioner(ctx, schedules[0].Actor, fhirID, practitionerRoleClient)
}

func ownsResource(ctx context.Context, fhirID, rawURL, role, method string, clients rbacClients, resource []byte) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	resourceType := utils.ExtractResourceTypeFromPath(u.Path)

	// GET request can bypass pre-request ownership checks
	// however, it might subject to post-request ownership filtering
	if method == http.MethodGet {
		return true
	}

	if method == constvars.MethodPost {
		return ownsPostBody(fhirID, role, resourceType, resource)
	}

	if method == constvars.MethodPut && len(resource) > 0 {
		return validateResourceOwnership(ctx, fhirID, role, resourceType, resource, clients)
	}

	// DELETE/PATCH: scope via the ownership engine's mutating-query rules.
	// ValidWriteQuery fails closed: a query that cannot prove ownership of
	// every scoped identity is denied.
	oc := ownershipContextForRole(role, fhirID)
	return ownership.ValidWriteQuery(rawURL, resourceType, oc)
}

// extractPathResourceID parses a URL path to extract resource type and ID.
// Handles both /fhir/{resource}/{id} and /{resource}/{id} patterns.
// Returns empty strings for paths with insufficient segments.
func extractPathResourceID(path string) (resource, id string) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) >= 2 {
		if strings.EqualFold(parts[0], "fhir") {
			if len(parts) >= 3 {
				return parts[1], parts[2]
			}
			return "", ""
		}
		return parts[0], parts[1]
	}
	return "", ""
}

// hasRole returns true if roles contains the target role (case-insensitive).
func hasRole(roles []string, target string) bool {
	for _, r := range roles {
		if strings.EqualFold(r, target) {
			return true
		}
	}
	return false
}

// matchSharedEmail returns true if any email address is shared between practitioner and patient.

func isBundle(r *http.Request) bool {
	if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
		return false
	}

	if bodyBytes, ok := r.Context().Value(constvars.CONTEXT_RAW_BODY).([]byte); ok && len(bodyBytes) > 0 {
		return strings.EqualFold(gjson.GetBytes(bodyBytes, "resourceType").String(), constvars.ResourceBundle)
	}

	var peek [2048]byte
	n, _ := r.Body.Read(peek[:])
	r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(peek[:n]), r.Body))
	return strings.EqualFold(gjson.GetBytes(peek[:n], "resourceType").String(), constvars.ResourceBundle)
}
