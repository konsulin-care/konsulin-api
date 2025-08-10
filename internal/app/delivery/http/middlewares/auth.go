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
		ctxIface := r.Context()
		roles, _ := ctxIface.Value(keyRoles).([]string)
		uid, _ := ctxIface.Value(keyUID).(string)

		fhirRole, fhirID, err := m.resolveFHIRIdentity(ctxIface, uid)

		if err != nil {
			m.Log.Error("Auth.resolveFHIRIdentity", zap.Error(err))
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrAuthInvalidRole(err))
			return
		}

		ctx := context.WithValue(ctxIface, keyFHIRRole, fhirRole)
		ctx = context.WithValue(ctx, keyFHIRID, fhirID)
		r = r.WithContext(ctx)

		if isBundle(r) {
			body, _ := io.ReadAll(r.Body)
			r.Body.Close()

			if err := scanBundle(ctx, m.Enforcer, body, roles, fhirID); err != nil {
				utils.BuildErrorResponse(m.Log, w, exceptions.ErrAuthInvalidRole(err))
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		fullURL := r.URL.RequestURI()
		if err := checkSingle(ctx, m.Enforcer, r.Method, fullURL, roles, fhirID); err != nil {
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrAuthInvalidRole(err))
			return
		}
		next.ServeHTTP(w, r)
	})
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
		return "practitioner", pracs[0].ID, nil
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
	return "patient", pats[0].ID, nil
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

	path = strings.TrimPrefix(path, "/fhir/")
	parts := strings.Split(path, "/")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return ""
}

func ownsResource(uid, rawURL, role string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(parts) >= 2 {
		res, id := parts[0], parts[1]
		switch role {
		case constvars.KonsulinRolePatient:
			if res == constvars.ResourcePatient && id == uid {
				return true
			}
		case constvars.KonsulinRolePractitioner:
			if res == constvars.ResourcePractitioner && id == uid {
				return true
			}
		}
	}

	q := u.Query()
	switch role {
	case constvars.KonsulinRolePatient:
		if p := q.Get("patient"); p != "" {
			id := strings.TrimPrefix(p, "Patient/")
			return id == uid
		}
	case constvars.KonsulinRolePractitioner:
		if p := q.Get("practitioner"); p != "" {
			id := strings.TrimPrefix(p, "Practitioner/")
			return id == uid
		}
	}
	return false
}

func isBundle(r *http.Request) bool {
	if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch && r.Header.Get("Content-Type") != "application/fhir+json" {
		return false
	}
	var peek [512]byte
	n, _ := r.Body.Read(peek[:])
	r.Body.Close()
	r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(peek[:n]), r.Body))
	return bytes.Contains(peek[:n], []byte(`"resourceType":"Bundle"`))
}
