package middlewares

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

		// Check if this is API key authenticated superadmin
		if len(roles) == 1 && roles[0] == constvars.KonsulinRoleSuperadmin && uid == "api-key-superadmin" {
			// API key authenticated superadmin - skip FHIR identity resolution
			fhirRole = constvars.KonsulinRoleSuperadmin
			fhirID = "" // Superadmin has no specific FHIR ID
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
		if err := checkSingle(ctxIface, m.Enforcer, r.Method, fullURL, roles, ctxIface.Value(keyFHIRID).(string)); err != nil {
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

func scanBundle(ctx context.Context, e *casbin.Enforcer, raw []byte, roles []string, uid string) error {
	if gjson.GetBytes(raw, "resourceType").String() != "Bundle" {
		return fmt.Errorf("invalid bundle")
	}
	entries := gjson.GetBytes(raw, "entry").Array()
	for _, entry := range entries {
		method := entry.Get("request.method").String()
		url := entry.Get("request.url").String()
		if err := checkSingle(ctx, e, method, url, roles, uid); err != nil {
			return err
		}
	}
	return nil
}

func checkSingle(ctx context.Context, e *casbin.Enforcer, method, url string, roles []string, fhirID string) error {
	res := firstSeg(url)

	if res == "" {
		return fmt.Errorf("invalid or empty resource in request")
	}

	for _, role := range roles {
		if allowed(e, role, res, method) {
			if role == constvars.KonsulinRolePatient || role == constvars.KonsulinRolePractitioner {
				if !ownsResource(fhirID, url, role) {
					return fmt.Errorf("%s cannot access other %s resources", role, role)
				}
			}
			return nil
		}
	}

	return fmt.Errorf("forbidden")
}

func allowed(e *casbin.Enforcer, role, res, verb string) bool {
	ok, err := e.Enforce(role, res, verb)
	if err != nil {
		return false
	}
	return ok
}
func firstSeg(raw string) string {
	path := strings.SplitN(raw, "?", 2)[0]

	// Normalize leading slash and optional fhir prefix so that
	// paths like "/Observation", "Observation", "/fhir/Observation", "fhir/Observation" all work
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimPrefix(path, "fhir/")
	path = strings.TrimPrefix(path, "/")

	parts := strings.Split(path, "/")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return ""
}

func ownsResource(fhirID, rawURL, role string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(parts) >= 2 {
		var res, id string
		// Support both "/fhir/Resource/ID" and "/Resource/ID" shapes
		if strings.EqualFold(parts[0], "fhir") {
			if len(parts) >= 3 {
				res, id = parts[1], parts[2]
			}
		} else {
			res, id = parts[0], parts[1]
		}

		switch role {
		case constvars.KonsulinRolePatient:
			if res == constvars.ResourcePatient && id == fhirID {
				return true
			}
		case constvars.KonsulinRolePractitioner:
			if res == constvars.ResourcePractitioner && id == fhirID {
				return true
			}
		}
	}

	q := u.Query()
	switch role {
	case constvars.KonsulinRolePatient:
		if p := q.Get("patient"); p != "" {
			id := strings.TrimPrefix(p, "Patient/")
			return id == fhirID
		}
	case constvars.KonsulinRolePractitioner:
		if p := q.Get("practitioner"); p != "" {
			id := strings.TrimPrefix(p, "Practitioner/")
			return id == fhirID
		}
	}
	return false
}

func isBundle(r *http.Request) bool {
	if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
		return false
	}

	// Prefer raw body from context if available
	if bodyBytes, ok := r.Context().Value(constvars.CONTEXT_RAW_BODY).([]byte); ok && len(bodyBytes) > 0 {
		return strings.EqualFold(gjson.GetBytes(bodyBytes, "resourceType").String(), "Bundle")
	}

	// Fallback: peek into request body and parse resourceType using gjson
	var peek [2048]byte
	n, _ := r.Body.Read(peek[:])
	r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(peek[:n]), r.Body))
	return strings.EqualFold(gjson.GetBytes(peek[:n], "resourceType").String(), "Bundle")
}
