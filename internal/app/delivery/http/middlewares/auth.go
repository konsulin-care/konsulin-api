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

			if err := scanBundle(ctxIface, m.Enforcer, body, roles, ctxIface.Value(keyFHIRID).(string)); err != nil {
				utils.BuildErrorResponse(m.Log, w, exceptions.ErrAuthInvalidRole(err))
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		fullURL := r.URL.RequestURI()
		if err := checkSingle(ctxIface, m.Enforcer, r.Method, fullURL, roles, ctxIface.Value(keyFHIRID).(string), m.PatientFhirClient); err != nil {
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
		constvars.FhirSupertokenSystemIdentifier,
		uid,
	)

	if err != nil {
		return "", "", err
	}
	if len(pats) == 0 {
		return "", "", fmt.Errorf("no Practitioner/Patient found for uid %s", uid)
	}
	if len(pats) > 1 {
		return "", "", fmt.Errorf("multiple Patient resources for uid %s", uid)
	}
	return constvars.KonsulinRolePatient, pats[0].ID, nil
}

func resolveIdentifierToPatientID(ctx context.Context, identifier string, patientClient contracts.PatientFhirClient) (string, error) {
	var system, value string
	if strings.Contains(identifier, "|") {
		parts := strings.SplitN(identifier, "|", 2)
		system = parts[0]
		value = parts[1]
		fmt.Printf("Debug: Parsed identifier with system: system=%s, value=%s\n", system, value)
	} else {
		value = identifier
		system = ""
		fmt.Printf("Debug: Parsed identifier without system: system=%s, value=%s\n", system, value)
	}

	fmt.Printf("Debug: Calling FindPatientByIdentifier with system=%s, value=%s\n", system, value)
	patients, err := patientClient.FindPatientByIdentifier(ctx, system, value)
	if err != nil {
		fmt.Printf("Debug: FindPatientByIdentifier failed: %v\n", err)
		return "", fmt.Errorf("failed to search patients by identifier: %w", err)
	}

	fmt.Printf("Debug: Found %d patients with identifier %s\n", len(patients), identifier)
	if len(patients) == 0 {
		return "", fmt.Errorf("no patient found with identifier %s", identifier)
	}

	if len(patients) > 1 {
		return "", fmt.Errorf("multiple patients found with identifier %s", identifier)
	}

	fmt.Printf("Debug: Returning patient ID: %s\n", patients[0].ID)
	return patients[0].ID, nil
}

func scanBundle(ctx context.Context, e *casbin.Enforcer, raw []byte, roles []string, uid string) error {
	if gjson.GetBytes(raw, "resourceType").String() != "Bundle" {
		return fmt.Errorf("invalid bundle")
	}
	entries := gjson.GetBytes(raw, "entry").Array()
	for _, entry := range entries {
		method := entry.Get("request.method").String()
		url := entry.Get("request.url").String()
		if err := checkSingle(ctx, e, method, url, roles, uid, nil); err != nil {
			return err
		}
	}
	return nil
}

func checkSingle(ctx context.Context, e *casbin.Enforcer, method, url string, roles []string, fhirID string, patientClient contracts.PatientFhirClient) error {

	normalizedPath := normalizePath(url)

	for _, role := range roles {
		if allowed(e, role, method, normalizedPath) {

			if role == constvars.KonsulinRolePatient || role == constvars.KonsulinRolePractitioner {
				if !ownsResource(ctx, fhirID, url, role, method, patientClient) {
					return fmt.Errorf("%s is trying to access resource that don't belong to him/her", role)
				}
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

func ownsResource(ctx context.Context, fhirID, rawURL, role, method string, patientClient contracts.PatientFhirClient) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	resourceType := utils.ExtractResourceTypeFromPath(u.Path)

	if method == "POST" {
		return true
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

			if identifier := q.Get("identifier"); identifier != "" {
				fmt.Printf("Debug: Checking identifier %s for patient fhirID %s\n", identifier, fhirID)

				patientID, err := resolveIdentifierToPatientID(ctx, identifier, patientClient)
				if err != nil {
					fmt.Printf("Debug: Failed to resolve identifier %s: %v\n", identifier, err)
					return false
				}

				fmt.Printf("Debug: Resolved identifier %s to patientID %s, comparing with fhirID %s\n", identifier, patientID, fhirID)
				return patientID == fhirID
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

			if p := q.Get("practitioner"); p != "" {
				id := strings.TrimPrefix(p, "Practitioner/")
				return id == fhirID
			}

			if a := q.Get("actor"); a != "" {
				id := strings.TrimPrefix(a, "Practitioner/")
				return id == fhirID
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
