package middlewares

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

func (m *Middlewares) OptionalAuthenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get(constvars.HeaderAuthorization)
		if authHeader == "" {
			next.ServeHTTP(w, r)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		sessionID, err := utils.ParseJWT(token, m.InternalConfig.JWT.Secret)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		sessionData, err := m.SessionService.GetSessionData(ctx, sessionID)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		ctx = context.WithValue(r.Context(), constvars.CONTEXT_SESSION_DATA_KEY, sessionData)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middlewares) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get(constvars.HeaderAuthorization)
		if authHeader == "" {
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrTokenMissing(nil))
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		sessionID, err := utils.ParseJWT(token, m.InternalConfig.JWT.Secret)
		if err != nil {
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrTokenInvalidOrExpired(err))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		sessionData, err := m.SessionService.GetSessionData(ctx, sessionID)
		if err != nil {
			if err == context.DeadlineExceeded {
				utils.BuildErrorResponse(m.Log, w, exceptions.ErrServerDeadlineExceeded(err))
				return
			}
			utils.BuildErrorResponse(m.Log, w, err)
			return
		}

		ctx = context.WithValue(r.Context(), constvars.CONTEXT_SESSION_DATA_KEY, sessionData)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middlewares) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		ctxIface := r.Context()
		roles, _ := ctxIface.Value(keyRoles).([]string)
		uid, _ := ctxIface.Value(keyUID).(string)

		fhirRole, fhirID, err := m.resolveUserRoles(ctxIface, roles, uid)
		if err != nil {
			m.Log.Error("Auth.resolveUserRoles", zap.Error(err))
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrAuthInvalidRole(err))
			return
		}

		ctxIface = context.WithValue(ctxIface, keyFHIRRole, fhirRole)
		ctxIface = context.WithValue(ctxIface, keyFHIRID, fhirID)

		r = r.WithContext(ctxIface)

		if r.Method == "POST" {
			body, _ := io.ReadAll(r.Body)
			r.Body.Close()

			if err := m.validatePostRequestBody(ctxIface, body, fhirRole, fhirID); err != nil {
				utils.BuildErrorResponse(m.Log, w, exceptions.ErrAuthInvalidRole(err))
				return
			}

			r.Body = io.NopCloser(bytes.NewReader(body))
		}

		if isBundle(r) {
			body, _ := io.ReadAll(r.Body)
			r.Body.Close()

			if err := scanBundle(ctxIface, m.Enforcer, body, roles, ctxIface.Value(keyFHIRID).(string), m.PatientFhirClient, m.PractitionerFhirClient, m.PractitionerRoleFhirClient, m.ScheduleFhirClient, m.QuestionnaireResponseFhirClient); err != nil {
				utils.BuildErrorResponse(m.Log, w, exceptions.ErrAuthInvalidRole(err))
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		fullURL := r.URL.RequestURI()

		var resourceBody []byte
		if r.Method == "PUT" || r.Method == "POST" {
			body, _ := io.ReadAll(r.Body)
			r.Body.Close()
			resourceBody = body
			r.Body = io.NopCloser(bytes.NewReader(body))
		}

		if err := checkSingle(ctxIface, m.Enforcer, r.Method, fullURL, roles, ctxIface.Value(keyFHIRID).(string), m.PatientFhirClient, m.PractitionerFhirClient, m.PractitionerRoleFhirClient, m.ScheduleFhirClient, m.QuestionnaireResponseFhirClient, resourceBody); err != nil {
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
// identity resolution (Patient or Practitioner). Roles like Clinic Admin,
// Researcher, and Guest do not have FHIR identities.
func needsFHIRResolution(roles []string) bool {
	for _, role := range roles {
		if role == constvars.KonsulinRolePatient || role == constvars.KonsulinRolePractitioner {
			return true
		}
	}
	return false
}

// resolveUserRoles determines the FHIR role and ID to use for authorization.
// Returns (fhirRole, fhirID, error). For Guest and non-FHIR roles, fhirID is empty.
func (m *Middlewares) resolveUserRoles(ctx context.Context, roles []string, uid string) (string, string, error) {
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

func (m *Middlewares) validatePostRequestBody(ctx context.Context, body []byte, fhirRole, fhirID string) error {
	if fhirRole == constvars.KonsulinRoleSuperadmin || fhirRole == constvars.KonsulinRoleGuest {
		return nil
	}

	resourceType := gjson.GetBytes(body, "resourceType").String()
	if resourceType == "" {
		return nil
	}

	resourceTypeFromPath := utils.ExtractResourceTypeFromPath("/fhir/" + resourceType)

	if utils.RequiresPatientOwnership(resourceTypeFromPath) && fhirRole == constvars.KonsulinRolePatient {
		return m.validatePatientOwnershipInBody(body, fhirID)
	}

	if utils.RequiresPractitionerOwnership(resourceTypeFromPath) && fhirRole == constvars.KonsulinRolePractitioner {
		return m.validatePractitionerOwnershipInBody(body, fhirID)
	}

	return nil
}

func (m *Middlewares) validatePatientOwnershipInBody(body []byte, patientID string) error {
	if err := validateBodySubjectRef(body, patientID, "Patient"); err != nil {
		return err
	}
	if err := validateBodyArrayRefs(body, "performer", patientID, "Patient"); err != nil {
		return err
	}
	return validateBodyArrayRefs(body, "actor", patientID, "Patient")
}

// validateBodySubjectRef checks that the subject.reference matches the given prefix and ID.
func validateBodySubjectRef(body []byte, id, prefix string) error {
	subject := gjson.GetBytes(body, "subject.reference").String()
	if subject == "" {
		return nil
	}
	if !strings.HasPrefix(subject, prefix+"/") {
		return fmt.Errorf("invalid subject reference format: %s", subject)
	}
	subjectID := strings.TrimPrefix(subject, prefix+"/")
	if subjectID != id {
		return fmt.Errorf("%s %s is trying to create resource for different %s %s", prefix, id, prefix, subjectID)
	}
	return nil
}

// validateBodyArrayRefs checks that all references in a JSON array field match the given prefix and ID.
func validateBodyArrayRefs(body []byte, field, id, prefix string) error {
	for _, item := range gjson.GetBytes(body, field).Array() {
		ref := item.Get("reference").String()
		if ref == "" {
			continue
		}
		if strings.HasPrefix(ref, prefix+"/") {
			itemID := strings.TrimPrefix(ref, prefix+"/")
			if itemID != id {
				return fmt.Errorf("%s %s is trying to create resource with different %s %s", prefix, id, prefix, itemID)
			}
		}
	}
	return nil
}

func (m *Middlewares) validatePractitionerOwnershipInBody(body []byte, practitionerID string) error {
	performers := gjson.GetBytes(body, "performer").Array()
	for _, performer := range performers {
		if ref := performer.Get("reference").String(); ref != "" {
			if strings.HasPrefix(ref, "Practitioner/") {
				performerID := strings.TrimPrefix(ref, "Practitioner/")
				if performerID != practitionerID {
					return fmt.Errorf("practitioner %s is trying to create resource with different practitioner performer %s", practitionerID, performerID)
				}
			}
		}
	}

	actors := gjson.GetBytes(body, "actor").Array()
	for _, actor := range actors {
		if ref := actor.Get("reference").String(); ref != "" {
			if strings.HasPrefix(ref, "Practitioner/") {
				actorID := strings.TrimPrefix(ref, "Practitioner/")
				if actorID != practitionerID {
					return fmt.Errorf("practitioner %s is trying to create resource with different practitioner actor %s", practitionerID, actorID)
				}
			}
		}
	}

	return nil
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

func resolveIdentifierToPatientID(ctx context.Context, identifier string, patientClient contracts.PatientFhirClient) (string, error) {
	patients, err := patientClient.FindPatientByIdentifier(ctx, identifier)
	if err != nil {
		return "", fmt.Errorf("failed to search patients by identifier: %w", err)
	}
	if len(patients) == 0 {
		return "", fmt.Errorf("no patient found with identifier %s", identifier)
	}

	// supress error for multiple patients found
	// if len(patients) > 1 {
	// 	return "", fmt.Errorf("multiple patients found with identifier %s", identifier)
	// }
	return patients[0].ID, nil
}

func scanBundle(ctx context.Context, e *casbin.Enforcer, raw []byte, roles []string, uid string, patientClient contracts.PatientFhirClient, practitionerClient contracts.PractitionerFhirClient, practitionerRoleClient contracts.PractitionerRoleFhirClient, scheduleClient contracts.ScheduleFhirClient, questionnaireResponseClient contracts.QuestionnaireResponseFhirClient) error {
	if gjson.GetBytes(raw, "resourceType").String() != "Bundle" {
		return fmt.Errorf("invalid bundle")
	}
	entries := gjson.GetBytes(raw, "entry").Array()
	for _, entry := range entries {
		method := entry.Get("request.method").String()
		url := entry.Get("request.url").String()
		resource := entry.Get("resource").Raw
		if err := checkSingle(ctx, e, method, url, roles, uid, patientClient, practitionerClient, practitionerRoleClient, scheduleClient, questionnaireResponseClient, []byte(resource)); err != nil {
			return err
		}
	}
	return nil
}

func checkSingle(ctx context.Context, e *casbin.Enforcer, method, url string, roles []string, fhirID string, patientClient contracts.PatientFhirClient, practitionerClient contracts.PractitionerFhirClient, practitionerRoleClient contracts.PractitionerRoleFhirClient, scheduleClient contracts.ScheduleFhirClient, questionnaireResponseClient contracts.QuestionnaireResponseFhirClient, resource []byte) error {
	normalizedPath := normalizePath(url)
	resourceType := utils.ExtractResourceTypeFromPath(normalizedPath)

	// direct request to public resource is allowed to bypass RBAC checks
	// but only for GET requests to avoid unwanted modifications
	if utils.IsPublicResource(resourceType) && method == http.MethodGet {
		return nil
	}

	for _, role := range roles {
		if allowed(e, role, method, normalizedPath) {

			if role == constvars.KonsulinRolePatient || role == constvars.KonsulinRolePractitioner {
				ok := ownsResource(ctx, fhirID, url, role, method, patientClient, practitionerClient, practitionerRoleClient, scheduleClient, questionnaireResponseClient, resource)
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

func allowed(e *casbin.Enforcer, role, method, path string) bool {
	ok, err := e.Enforce(role, method, path)
	if err != nil {
		return false
	}
	return ok
}

func normalizePath(rawURL string) string {
	return utils.NormalizePath(rawURL)
}
func firstSeg(raw string) string {
	path := strings.SplitN(raw, "?", 2)[0]

	path = strings.TrimPrefix(path, "/")
	path = strings.TrimPrefix(path, "fhir/")
	path = strings.TrimPrefix(path, "/")

	parts := strings.Split(path, "/")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return ""
}
func validateResourceOwnership(ctx context.Context, fhirID, role, resourceType string, resource []byte, practitionerRoleClient contracts.PractitionerRoleFhirClient, scheduleClient contracts.ScheduleFhirClient, questionnaireResponseClient contracts.QuestionnaireResponseFhirClient) bool {
	if role == constvars.KonsulinRolePatient {
		return validatePatientResourceOwnership(ctx, fhirID, resourceType, resource, questionnaireResponseClient)
	}
	if role == constvars.KonsulinRolePractitioner {
		return validatePractitionerResourceOwnership(ctx, fhirID, resourceType, resource, practitionerRoleClient, scheduleClient)
	}
	return false
}

func validatePatientResourceOwnership(ctx context.Context, fhirID, resourceType string, resource []byte, questionnaireResponseClient contracts.QuestionnaireResponseFhirClient) bool {
	resourceStr := string(resource)
	switch resourceType {
	case "Condition":
		return validatePatientConditionResource(resourceStr, fhirID)
	case "Appointment":
		return validatePatientAppointmentResource(resourceStr, fhirID)
	case "Slot":
		return validatePatientSlotResource(resourceStr)
	case constvars.ResourceQuestionnaireResponse:
		return validateQuestionnaireResponseOwner(ctx, fhirID, resourceStr, questionnaireResponseClient)
	case constvars.ResourcePatient:
		return gjson.Get(resourceStr, "id").String() == fhirID
	default:
		return checkPatientRefs(resourceStr, fhirID)
	}
}

func validatePatientConditionResource(resourceStr, fhirID string) bool {
	subjectRef := gjson.Get(resourceStr, "subject.reference").String()
	return strings.HasPrefix(subjectRef, "Patient/") && strings.TrimPrefix(subjectRef, "Patient/") == fhirID
}

func validatePatientAppointmentResource(resourceStr, fhirID string) bool {
	for _, participant := range gjson.Get(resourceStr, "participant").Array() {
		actorRef := participant.Get("actor.reference").String()
		if strings.HasPrefix(actorRef, "Patient/") && strings.TrimPrefix(actorRef, "Patient/") == fhirID {
			return true
		}
	}
	return false
}

func validatePatientSlotResource(resourceStr string) bool {
	status := gjson.Get(resourceStr, "status").String()
	return status == "busy" || status == "busy-unavailable"
}

func checkPatientRefs(resourceStr, fhirID string) bool {
	for _, ref := range []string{
		gjson.Get(resourceStr, "subject.reference").String(),
		gjson.Get(resourceStr, "patient.reference").String(),
		gjson.Get(resourceStr, "actor.reference").String(),
	} {
		if strings.HasPrefix(ref, "Patient/") && strings.TrimPrefix(ref, "Patient/") == fhirID {
			return true
		}
	}
	return false
}

func validatePractitionerResourceOwnership(ctx context.Context, fhirID, resourceType string, resource []byte, practitionerRoleClient contracts.PractitionerRoleFhirClient, scheduleClient contracts.ScheduleFhirClient) bool {
	resourceStr := string(resource)
	switch resourceType {
	case "Invoice":
		return validatePractitionerInvoiceResource(resourceStr, fhirID)
	case constvars.ResourcePractitioner:
		return gjson.Get(resourceStr, "id").String() == fhirID
	case constvars.ResourceSchedule:
		return validateScheduleOwnership(ctx, fhirID, resourceStr, practitionerRoleClient, scheduleClient)
	default:
		return checkPractitionerRefs(resourceStr, fhirID)
	}
}

func validatePractitionerInvoiceResource(resourceStr, fhirID string) bool {
	for _, participant := range gjson.Get(resourceStr, "participant").Array() {
		actorRef := participant.Get("actor.reference").String()
		if strings.HasPrefix(actorRef, "PractitionerRole/") {
			return true
		}
		if strings.HasPrefix(actorRef, "Practitioner/") && strings.TrimPrefix(actorRef, "Practitioner/") == fhirID {
			return true
		}
	}
	return false
}

func checkPractitionerRefs(resourceStr, fhirID string) bool {
	for _, ref := range []string{
		gjson.Get(resourceStr, "practitioner.reference").String(),
		gjson.Get(resourceStr, "actor.reference").String(),
		gjson.Get(resourceStr, "performer.reference").String(),
		gjson.Get(resourceStr, "author.reference").String(),
	} {
		if strings.HasPrefix(ref, "Practitioner/") && strings.TrimPrefix(ref, "Practitioner/") == fhirID {
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
	sameOwner := true
	if strings.HasPrefix(authorRef, "Patient/") && strings.TrimPrefix(authorRef, "Patient/") != fhirID {
		sameOwner = false
	}
	if strings.HasPrefix(subjectRef, "Patient/") && strings.TrimPrefix(subjectRef, "Patient/") != fhirID {
		sameOwner = false
	}
	return sameOwner
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
	sch := schedules[0]
	for _, actor := range sch.Actor {
		actorRef := actor.Reference
		if strings.HasPrefix(actorRef, "PractitionerRole/") {
			roleID := strings.TrimPrefix(actorRef, "PractitionerRole/")
			pr, err := practitionerRoleClient.FindPractitionerRoleByID(ctx, roleID)
			if err != nil {
				continue
			}
			pracRef := pr.Practitioner.Reference
			if strings.HasPrefix(pracRef, "Practitioner/") && strings.TrimPrefix(pracRef, "Practitioner/") == fhirID {
				return true
			}
		}
		if strings.HasPrefix(actorRef, "Practitioner/") && strings.TrimPrefix(actorRef, "Practitioner/") == fhirID {
			return true
		}
	}
	return false
}

func ownsResource(ctx context.Context, fhirID, rawURL, role, method string, patientClient contracts.PatientFhirClient, practitionerClient contracts.PractitionerFhirClient, practitionerRoleClient contracts.PractitionerRoleFhirClient, scheduleClient contracts.ScheduleFhirClient, questionnaireResponseClient contracts.QuestionnaireResponseFhirClient, resource []byte) bool {
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

	if method == "POST" {
		return true
	}

	if method == "PUT" && len(resource) > 0 {
		return validateResourceOwnership(ctx, fhirID, role, resourceType, resource, practitionerRoleClient, scheduleClient, questionnaireResponseClient)
	}

	if role == constvars.KonsulinRolePatient {
		return ownsPatientQuery(ctx, fhirID, u, resourceType, patientClient, practitionerClient)
	}

	if role == constvars.KonsulinRolePractitioner {
		return ownsPractitionerQuery(ctx, fhirID, u, resourceType, patientClient, practitionerClient)
	}

	return false
}

func ownsPatientQuery(ctx context.Context, fhirID string, u *url.URL, resourceType string, patientClient contracts.PatientFhirClient, practitionerClient contracts.PractitionerFhirClient) bool {
	if utils.IsPublicResource(resourceType) {
		return true
	}

	if !utils.RequiresPatientOwnership(resourceType) {
		return false
	}

	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(parts) >= 2 {
		var res, id string
		if strings.EqualFold(parts[0], "fhir") {
			if len(parts) >= 3 {
				res, id = parts[1], parts[2]
			}
		} else {
			res, id = parts[0], parts[1]
		}
		if res == "Patient" && id == fhirID {
			return true
		}
	}

	q := u.Query()

	if checkPatientQueryRefs(q, resourceType, fhirID) {
		return true
	}

	if checkPatientIdentifierOwnership(ctx, q, fhirID, patientClient, practitionerClient) {
		return true
	}
	if checkPatientEmailOwnership(ctx, q, fhirID, patientClient, practitionerClient) {
		return true
	}

	return false
}

// checkPatientQueryRefs checks common patient query parameters for ownership.
func checkPatientQueryRefs(q url.Values, resourceType, fhirID string) bool {
	for _, param := range []string{"patient", "subject", "actor"} {
		if val := q.Get(param); val != "" && strings.TrimPrefix(val, "Patient/") == fhirID {
			return true
		}
	}
	// questionnaire is always allowed for patients
	if q.Get("questionnaire") != "" {
		return true
	}
	if resourceType == constvars.ResourcePatient {
		if val := q.Get("_id"); val != "" && val == fhirID {
			return true
		}
	}
	return false
}

func checkPatientIdentifierOwnership(ctx context.Context, q url.Values, fhirID string, patientClient contracts.PatientFhirClient, practitionerClient contracts.PractitionerFhirClient) bool {
	identifier := q.Get("identifier")
	if identifier == "" {
		return false
	}
	patientID, err := resolveIdentifierToPatientID(ctx, identifier, patientClient)
	if err != nil {
		return false
	}
	if patientID == fhirID {
		return true
	}
	roles, _ := ctx.Value(keyRoles).([]string)
	isPractitioner := false
	for _, r := range roles {
		if strings.EqualFold(r, constvars.KonsulinRolePractitioner) {
			isPractitioner = true
			break
		}
	}
	if !isPractitioner {
		return false
	}
	practitioner, err := practitionerClient.FindPractitionerByID(ctx, fhirID)
	if err != nil || practitioner == nil {
		return false
	}
	patient, err := patientClient.FindPatientByID(ctx, patientID)
	if err != nil || patient == nil {
		return false
	}
	practEmails := practitioner.GetEmailAddresses()
	patEmails := patient.GetEmailAddresses()
	if len(practEmails) == 0 || len(patEmails) == 0 {
		return false
	}
	for _, pe := range practEmails {
		for _, qe := range patEmails {
			if pe == qe {
				return true
			}
		}
	}
	return false
}

func checkPatientEmailOwnership(ctx context.Context, q url.Values, fhirID string, patientClient contracts.PatientFhirClient, practitionerClient contracts.PractitionerFhirClient) bool {
	email := q.Get("email")
	if email == "" {
		return false
	}
	patients, err := patientClient.FindPatientByEmail(ctx, email)
	if err != nil {
		return false
	}
	for _, p := range patients {
		if p.ID == fhirID {
			return true
		}
	}
	roles, _ := ctx.Value(keyRoles).([]string)
	hasPractRole := false
	for _, r := range roles {
		if strings.EqualFold(r, constvars.KonsulinRolePractitioner) {
			hasPractRole = true
			break
		}
	}
	if !hasPractRole {
		return false
	}
	practitioner, err := practitionerClient.FindPractitionerByID(ctx, fhirID)
	if err != nil || practitioner == nil {
		return false
	}
	practEmails := practitioner.GetEmailAddresses()
	if len(practEmails) == 0 {
		return false
	}
	for _, pe := range practEmails {
		if pe == email {
			return true
		}
	}
	return false
}

func ownsPractitionerQuery(ctx context.Context, fhirID string, u *url.URL, resourceType string, patientClient contracts.PatientFhirClient, practitionerClient contracts.PractitionerFhirClient) bool {
	if utils.IsPublicResource(resourceType) {
		return checkPractitionerPublicResourceQuery(fhirID, u)
	}

	if utils.RequiresPractitionerOwnership(resourceType) {
		return checkPractitionerOwnershipParams(ctx, fhirID, u, resourceType, practitionerClient)
	}

	if checkPerResourceTypeOwnership(fhirID, u, resourceType) {
		return true
	}

	return false
}

func checkPerResourceTypeOwnership(fhirID string, u *url.URL, resourceType string) bool {
	q := u.Query()
	if resourceType == constvars.ResourcePractitionerRole {
		practitioner := q.Get("practitioner")
		return practitioner != "" && strings.TrimPrefix(practitioner, "Practitioner/") == fhirID
	}
	if resourceType == constvars.ResourceSchedule {
		actor := q.Get("actor")
		return actor != "" && strings.TrimPrefix(actor, "Practitioner/") == fhirID
	}
	if resourceType == constvars.ResourceSlot {
		if q.Get("schedule.actor:Practitioner") != "" || q.Get("schedule.actor") != "" || q.Get("practitioner") != "" {
			return true
		}
	}
	if resourceType == constvars.ResourceQuestionnaireResponse {
		author := q.Get("author")
		return author != "" && strings.TrimPrefix(author, "Practitioner/") == fhirID
	}
	if resourceType == "Appointment" {
		q := u.Query()

		if p := q.Get("practitioner"); p != "" {
			id := strings.TrimPrefix(p, "Practitioner/")
			return id == fhirID
		}

		if a := q.Get("actor"); a != "" {
			id := strings.TrimPrefix(a, "Practitioner/")
			return id == fhirID
		}

		return false
	}

	return false
}

func checkPractitionerPublicResourceQuery(fhirID string, u *url.URL) bool {
	q := u.Query()
	hasOwnershipParams := q.Get("practitioner") != "" || q.Get("actor") != ""
	if !hasOwnershipParams {
		for key := range q {
			if strings.HasPrefix(key, "_has") && strings.Contains(key, "practitioner") {
				hasOwnershipParams = true
				break
			}
		}
	}
	if !hasOwnershipParams {
		return true
	}

	if checkPractitionerQueryPath(fhirID, u) {
		return true
	}
	if checkPractitionerQueryParams(fhirID, q) {
		return true
	}
	return checkPractitionerHasQuery(fhirID, q)
}

func checkPractitionerQueryPath(fhirID string, u *url.URL) bool {
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(parts) < 2 {
		return false
	}
	var res, id string
	if strings.EqualFold(parts[0], "fhir") && len(parts) >= 3 {
		res, id = parts[1], parts[2]
	} else {
		res, id = parts[0], parts[1]
	}
	return res == "Practitioner" && id == fhirID
}

func checkPractitionerQueryParams(fhirID string, q url.Values) bool {
	if p := q.Get("practitioner"); p != "" && strings.TrimPrefix(p, "Practitioner/") == fhirID {
		return true
	}
	if a := q.Get("actor"); a != "" && strings.TrimPrefix(a, "Practitioner/") == fhirID {
		return true
	}
	return q.Get("patient") != "" || strings.HasPrefix(q.Get("subject"), "Patient/")
}

func checkPractitionerHasQuery(fhirID string, q url.Values) bool {
	for key, values := range q {
		if strings.HasPrefix(key, "_has") && strings.Contains(key, "practitioner") {
			for _, value := range values {
				if value == fhirID {
					return true
				}
			}
		}
	}
	return false
}

func checkPractitionerOwnershipParams(ctx context.Context, fhirID string, u *url.URL, resourceType string, practitionerClient contracts.PractitionerFhirClient) bool {
	if checkPractitionerPathOwnership(fhirID, u) {
		return true
	}
	q := u.Query()
	if checkPractitionerIdentifierOwnership(ctx, q, fhirID) {
		return true
	}
	if checkPractitionerQueryOwnership(q, resourceType, fhirID) {
		return true
	}
	if checkPractitionerHasQueryOwnership(q, fhirID) {
		return true
	}
	return checkPractitionerEmailOwnership(ctx, q, fhirID, practitionerClient)
}

func checkPractitionerPathOwnership(fhirID string, u *url.URL) bool {
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(parts) < 2 {
		return false
	}
	var res, id string
	if strings.EqualFold(parts[0], "fhir") && len(parts) >= 3 {
		res, id = parts[1], parts[2]
	} else {
		res, id = parts[0], parts[1]
	}
	return res == "Practitioner" && id == fhirID
}

func checkPractitionerIdentifierOwnership(ctx context.Context, q url.Values, fhirID string) bool {
	ids, ok := q["identifier"]
	if !ok {
		return false
	}
	uidCtx, _ := ctx.Value(keyUID).(string)
	for _, idv := range ids {
		parts := strings.SplitN(idv, "|", 2)
		if len(parts) == 2 {
			if parts[0] == constvars.FhirSupertokenSystemIdentifier && parts[1] == uidCtx {
				return true
			}
		} else if idv == uidCtx {
			return true
		}
	}
	return false
}

func checkPractitionerQueryOwnership(q url.Values, resourceType, fhirID string) bool {
	if p := q.Get("practitioner"); p != "" && strings.TrimPrefix(p, "Practitioner/") == fhirID {
		return true
	}
	if resourceType == constvars.ResourcePractitioner && q.Get("_id") == fhirID {
		return true
	}
	if a := q.Get("actor"); a != "" && strings.TrimPrefix(a, "Practitioner/") == fhirID {
		return true
	}
	if q.Get("patient") != "" || strings.HasPrefix(q.Get("subject"), "Patient/") {
		return true
	}
	if participant := q.Get("participant"); participant != "" {
		if strings.HasPrefix(participant, "PractitionerRole/") {
			return true
		}
		if strings.HasPrefix(participant, "Practitioner/") && strings.TrimPrefix(participant, "Practitioner/") == fhirID {
			return true
		}
	}
	return false
}

func checkPractitionerHasQueryOwnership(q url.Values, fhirID string) bool {
	for key, values := range q {
		if strings.HasPrefix(key, "_has") && strings.Contains(key, "practitioner") {
			for _, value := range values {
				if value == fhirID {
					return true
				}
			}
		}
	}
	return false
}

func checkPractitionerEmailOwnership(ctx context.Context, q url.Values, fhirID string, practitionerClient contracts.PractitionerFhirClient) bool {
	email := q.Get("email")
	if email == "" {
		return false
	}
	practitioners, err := practitionerClient.FindPractitionerByEmail(ctx, email)
	if err != nil || len(practitioners) != 1 {
		return false
	}
	return practitioners[0].ID == fhirID
}

func isBundle(r *http.Request) bool {
	if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
		return false
	}

	if bodyBytes, ok := r.Context().Value(constvars.CONTEXT_RAW_BODY).([]byte); ok && len(bodyBytes) > 0 {
		return strings.EqualFold(gjson.GetBytes(bodyBytes, "resourceType").String(), "Bundle")
	}

	var peek [2048]byte
	n, _ := r.Body.Read(peek[:])
	r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(peek[:n]), r.Body))
	return strings.EqualFold(gjson.GetBytes(peek[:n], "resourceType").String(), "Bundle")
}
