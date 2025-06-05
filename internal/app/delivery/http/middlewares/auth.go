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
	"strings"
	"time"

	"slices"

	"github.com/tidwall/gjson"
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
		ctx := r.Context()
		roles := ctx.Value(keyRoles).([]string)
		uid := ctx.Value(keyUID).(string)

		if isBundle(r) {
			body, _ := io.ReadAll(r.Body)
			r.Body.Close()

			if err := scanBundle(ctx, body, roles, uid); err != nil {
				utils.BuildErrorResponse(m.Log, w, exceptions.ErrAuthInvalidRole(err))
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		if err := checkSingle(ctx, r.Method, r.URL.Path, roles, uid); err != nil {
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrAuthInvalidRole(err))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func scanBundle(ctx context.Context, raw []byte, roles []string, uid string) error {
	if gjson.GetBytes(raw, "resourceType").String() != "Bundle" {
		return fmt.Errorf("invalid bundle")
	}
	entries := gjson.GetBytes(raw, "entry").Array()
	for _, e := range entries {
		method := e.Get("request.method").String()
		url := e.Get("request.url").String()
		if err := checkSingle(ctx, method, url, roles, uid); err != nil {
			return err
		}
	}
	return nil
}

func checkSingle(ctx context.Context, method, url string, roles []string, uid string) error {
	res := firstSeg(url)

	if contains(roles, "Patient") && res == "Patient" && !ownsPatient(uid, url) {
		return fmt.Errorf("patient cannot access other patients' data")
	}

	for _, role := range roles {
		if allowed(role, res, method) {
			return nil
		}
	}
	return fmt.Errorf("forbidden")
}

func allowed(role, res, verb string) bool {
	verbs := rolePerms[role][res]
	return slices.Contains(verbs, verb)
}

func firstSeg(path string) string {
	path = strings.TrimPrefix(path, "/fhir/")
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func contains(s []string, target string) bool {
	for _, v := range s {
		if v == target {
			return true
		}
	}
	return false
}

func ownsPatient(uid, path string) bool {
	if uid == "" {
		return false
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) >= 2 && parts[1] == uid
}

func isBundle(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	var peek [512]byte
	n, _ := r.Body.Read(peek[:])
	r.Body.Close()
	r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(peek[:n]), r.Body))
	return bytes.Contains(peek[:n], []byte(`"resourceType":"Bundle"`))
}
