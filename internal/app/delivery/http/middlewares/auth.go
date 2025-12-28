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
		var fhirRole, fhirID string
		var err error
		ctxIface := r.Context()
		roles, _ := ctxIface.Value(keyRoles).([]string)
		uid, _ := ctxIface.Value(keyUID).(string)

		if len(roles) == 1 && roles[0] == constvars.KonsulinRoleSuperadmin && uid == "api-key-superadmin" {

			fhirRole = constvars.KonsulinRoleSuperadmin
			fhirID = ""
		} else if !isOnlyGuest(roles) {
			fhirRole, fhirID, err = m.resolveFHIRIdentity(ctxIface, uid)
			if err != nil {
				m.Log.Error("Auth.resolveFHIRIdentity", zap.Error(err))
				utils.BuildErrorResponse(m.Log, w, exceptions.ErrAuthInvalidRole(err))
				return
			}
		} else {
			fhirRole = constvars.KonsulinRoleGuest
			fhirID = ""
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

func isOnlyGuest(roles []string) bool {
	if len(roles) != 1 {
		return false
	}
	return strings.EqualFold(roles[0], constvars.KonsulinRoleGuest)
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
	if subject := gjson.GetBytes(body, "subject.reference").String(); subject != "" {
		if !strings.HasPrefix(subject, "Patient/") {
			return fmt.Errorf("invalid subject reference format: %s", subject)
		}
		subjectID := strings.TrimPrefix(subject, "Patient/")
		if subjectID != patientID {
			return fmt.Errorf("patient %s is trying to create resource for different patient %s", patientID, subjectID)
		}
	}

	performers := gjson.GetBytes(body, "performer").Array()
	for _, performer := range performers {
		if ref := performer.Get("reference").String(); ref != "" {
			if strings.HasPrefix(ref, "Patient/") {
				performerID := strings.TrimPrefix(ref, "Patient/")
				if performerID != patientID {
					return fmt.Errorf("patient %s is trying to create resource with different patient performer %s", patientID, performerID)
				}
			}
		}
	}

	actors := gjson.GetBytes(body, "actor").Array()
	for _, actor := range actors {
		if ref := actor.Get("reference").String(); ref != "" {
			if strings.HasPrefix(ref, "Patient/") {
				actorID := strings.TrimPrefix(ref, "Patient/")
				if actorID != patientID {
					return fmt.Errorf("patient %s is trying to create resource with different patient actor %s", patientID, actorID)
				}
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

func (m *Middlewares) resolveFHIRIdentity(ctx context.Context, uid string) (role string, id string, err error) {
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

	pats, err := m.PatientFhirClient.FindPatientByIdentifier(
		ctx,
		fmt.Sprintf("%s|%s", constvars.FhirSupertokenSystemIdentifier, uid),
	)

	if err != nil {
		return "", "", err
	}
	if len(pats) == 0 {
		return "", "", fmt.Errorf("no Practitioner/Patient found for uid %s", uid)
	}

	// supress error for multiple patients found
	// if len(pats) > 1 {
	// 	fmt.Println("MULTIPLE PATIENTS FOUND FOR UID", uid)
	// 	fmt.Println("PATIENTS", pats)
	// 	return "", "", fmt.Errorf("multiple Patient resources for uid %s", uid)
	// }
	return constvars.KonsulinRolePatient, pats[0].ID, nil
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
		resourceStr := string(resource)

		if resourceType == "Condition" {
			subjectRef := gjson.Get(resourceStr, "subject.reference").String()
			if strings.HasPrefix(subjectRef, "Patient/") {
				patientID := strings.TrimPrefix(subjectRef, "Patient/")
				return patientID == fhirID
			}
		}

		if resourceType == "Appointment" {
			participants := gjson.Get(resourceStr, "participant").Array()
			for _, participant := range participants {
				actorRef := participant.Get("actor.reference").String()
				if strings.HasPrefix(actorRef, "Patient/") {
					patientID := strings.TrimPrefix(actorRef, "Patient/")
					if patientID == fhirID {
						return true
					}
				}
			}
		}

		if resourceType == "Slot" {
			status := gjson.Get(resourceStr, "status").String()

			if status == "busy" || status == "busy-unavailable" {
				return true
			}
		}

		if resourceType == constvars.ResourceQuestionnaireResponse {
			questionnaireResponseID := gjson.Get(resourceStr, "id").String()

			// will directly reject the request if the questionnaire response id is not found
			if questionnaireResponseID == "" {
				return false
			}

			questionnaireResponse, err := questionnaireResponseClient.FindQuestionnaireResponseByID(ctx, questionnaireResponseID)
			if err != nil {
				return false
			}

			authorRef := questionnaireResponse.Author.Reference
			subjectRef := questionnaireResponse.Subject.Reference

			if authorRef == "" && subjectRef == "" {
				return true
			}

			sameOwner := true
			if strings.HasPrefix(authorRef, "Patient/") {
				authorID := strings.TrimPrefix(authorRef, "Patient/")
				if authorID != fhirID {
					sameOwner = false
				}
			}

			if strings.HasPrefix(subjectRef, "Patient/") {
				subjectID := strings.TrimPrefix(subjectRef, "Patient/")
				if subjectID != fhirID {
					sameOwner = false
				}
			}

			if sameOwner {
				return true
			}
		}

		// this checks below is to allow patient to update their own patient resource
		if resourceType == constvars.ResourcePatient {
			patientID := gjson.Get(resourceStr, "id").String()
			if patientID == fhirID {
				return true
			}
		}

		patientRefs := []string{
			gjson.Get(resourceStr, "subject.reference").String(),
			gjson.Get(resourceStr, "patient.reference").String(),
			gjson.Get(resourceStr, "actor.reference").String(),
		}

		for _, ref := range patientRefs {
			if strings.HasPrefix(ref, "Patient/") {
				patientID := strings.TrimPrefix(ref, "Patient/")
				if patientID == fhirID {
					return true
				}
			}
		}
	}

	if role == constvars.KonsulinRolePractitioner {
		resourceStr := string(resource)
		if resourceType == "Invoice" {
			participants := gjson.Get(resourceStr, "participant").Array()
			for _, participant := range participants {
				actorRef := participant.Get("actor.reference").String()
				if strings.HasPrefix(actorRef, "PractitionerRole/") {
					return true
				}
				if strings.HasPrefix(actorRef, "Practitioner/") {
					practitionerID := strings.TrimPrefix(actorRef, "Practitioner/")
					if practitionerID == fhirID {
						return true
					}
				}
			}
		}

		// this checks below is to allow practitioner to update their own practitioner resource
		if resourceType == constvars.ResourcePractitioner {
			practitionerID := gjson.Get(resourceStr, "id").String()
			if practitionerID == fhirID {
				return true
			}
		}

		// schedule ownership check via first actor -> PractitionerRole -> Practitioner
		if resourceType == constvars.ResourceSchedule {
			scheduleID := gjson.Get(resourceStr, "id").String()
			if scheduleID == "" {
				return false
			}

			schedules, err := scheduleClient.Search(ctx, contracts.ScheduleSearchParams{ID: scheduleID})
			if err != nil {
				return false
			}
			if len(schedules) != 1 {
				return false
			}
			sch := schedules[0]
			if len(sch.Actor) < 1 {
				return false
			}

			for _, actor := range sch.Actor {
				actorRef := actor.Reference

				if strings.HasPrefix(actorRef, "PractitionerRole/") {
					roleID := strings.TrimPrefix(actorRef, "PractitionerRole/")
					pr, err := practitionerRoleClient.FindPractitionerRoleByID(ctx, roleID)
					if err != nil {
						continue
					}
					pracRef := pr.Practitioner.Reference
					if strings.HasPrefix(pracRef, "Practitioner/") {
						pid := strings.TrimPrefix(pracRef, "Practitioner/")
						if pid == fhirID {
							return true
						}
					}
				}

				if strings.HasPrefix(actorRef, "Practitioner/") {
					practitionerID := strings.TrimPrefix(actorRef, "Practitioner/")
					if practitionerID == fhirID {
						return true
					}
				}
			}

		}

		practitionerRefs := []string{
			gjson.Get(resourceStr, "practitioner.reference").String(),
			gjson.Get(resourceStr, "actor.reference").String(),
			gjson.Get(resourceStr, "performer.reference").String(),
			gjson.Get(resourceStr, "author.reference").String(),
		}
		for _, ref := range practitionerRefs {
			if strings.HasPrefix(ref, "Practitioner/") {
				practitionerID := strings.TrimPrefix(ref, "Practitioner/")
				if practitionerID == fhirID {
					return true
				}
			}
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

		if utils.IsPublicResource(resourceType) {
			return true
		}

		if utils.RequiresPatientOwnership(resourceType) {

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

			if p := q.Get("patient"); p != "" {
				id := strings.TrimPrefix(p, "Patient/")
				return id == fhirID
			}

			if s := q.Get("subject"); s != "" {
				id := strings.TrimPrefix(s, "Patient/")
				return id == fhirID
			}

			if a := q.Get("actor"); a != "" {
				id := strings.TrimPrefix(a, "Patient/")
				return id == fhirID
			}

			if qr := q.Get("questionnaire"); qr != "" {
				return true
			}

			if resourceType == constvars.ResourcePatient {
				if val := q.Get("_id"); val != "" {
					return val == fhirID
				}
			}

			if identifier := q.Get("identifier"); identifier != "" {

				patientID, err := resolveIdentifierToPatientID(ctx, identifier, patientClient)
				if err != nil {

					return false
				}

				if patientID == fhirID {
					return true
				}

				// Fallback: if requester has Practitioner role, allow when practitioner's email matches patient's
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

				// Fetch Practitioner and Patient resources and compare emails (exact match)
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

			// add support email based query
			if email := q.Get("email"); email != "" {
				patients, err := patientClient.FindPatientByEmail(ctx, email)
				if err != nil {
					return false
				}

				// the check below will be turned of for now
				// to temporarily allow multiple patients found
				// // guard against no patients found or multiple patients found
				// if len(patients) != 1 {
				// 	return false
				// }

				// If any patient resolved by email matches current fhirID, allow
				for _, p := range patients {
					if p.ID == fhirID {
						return true
					}
				}

				// Require Practitioner role for email intersection fallback
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

				// Verify practitioner's emails intersect with requested email
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

			return false
		}

		return false
	}

	if role == constvars.KonsulinRolePractitioner {

		if utils.IsPublicResource(resourceType) {
			q := u.Query()
			hasOwnershipParams := false

			if q.Get("practitioner") != "" || q.Get("actor") != "" {
				hasOwnershipParams = true
			}

			for key := range q {
				if strings.HasPrefix(key, "_has") && strings.Contains(key, "practitioner") {
					hasOwnershipParams = true
					break
				}
			}

			if !hasOwnershipParams {
				return true
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

				if res == "Practitioner" && id == fhirID {
					return true
				}
			}

			if p := q.Get("practitioner"); p != "" {
				id := strings.TrimPrefix(p, "Practitioner/")
				return id == fhirID
			}

			if a := q.Get("actor"); a != "" {
				id := strings.TrimPrefix(a, "Practitioner/")
				return id == fhirID
			}

			if patient := q.Get("patient"); patient != "" {
				return true
			}

			if subject := q.Get("subject"); subject != "" {
				if strings.HasPrefix(subject, "Patient/") {
					return true
				}
			}

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

		if utils.RequiresPractitionerOwnership(resourceType) {

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

				if res == "Practitioner" && id == fhirID {
					return true
				}
			}

			q := u.Query()

			// support identifier-based lookup for own Practitioner record using SuperTokens UID
			if ids, ok := q["identifier"]; ok {
				uidCtx, _ := ctx.Value(keyUID).(string)
				for _, idv := range ids {
					parts := strings.SplitN(idv, "|", 2)
					if len(parts) == 2 {
						sys, val := parts[0], parts[1]
						if sys == constvars.FhirSupertokenSystemIdentifier && val == uidCtx {
							return true
						}
					} else {
						// optionally allow raw UID without system prefix
						if idv == uidCtx {
							return true
						}
					}
				}
			}

			if p := q.Get("practitioner"); p != "" {
				id := strings.TrimPrefix(p, "Practitioner/")
				return id == fhirID
			}

			if resourceType == constvars.ResourcePractitioner {
				if val := q.Get("_id"); val != "" {
					return val == fhirID
				}
			}

			if a := q.Get("actor"); a != "" {
				id := strings.TrimPrefix(a, "Practitioner/")
				return id == fhirID
			}

			if patient := q.Get("patient"); patient != "" {
				return true
			}

			if subject := q.Get("subject"); subject != "" {
				if strings.HasPrefix(subject, "Patient/") {
					return true
				}
			}

			if participant := q.Get("participant"); participant != "" {
				if strings.HasPrefix(participant, "PractitionerRole/") {
					return true
				}
				if strings.HasPrefix(participant, "Practitioner/") {
					id := strings.TrimPrefix(participant, "Practitioner/")
					return id == fhirID
				}
			}

			for key, values := range q {
				if strings.HasPrefix(key, "_has") && strings.Contains(key, "practitioner") {
					for _, value := range values {
						if value == fhirID {
							return true
						}
					}
				}
			}

			// add support email based query
			if email := q.Get("email"); email != "" {
				practitioners, err := practitionerClient.FindPractitionerByEmail(ctx, email)

				if err != nil {
					return false
				}

				// guard against multiple practitioners found
				// or no practitioners found at all
				if len(practitioners) != 1 {
					return false
				}

				return practitioners[0].ID == fhirID
			}
			return false
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

	return false
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
