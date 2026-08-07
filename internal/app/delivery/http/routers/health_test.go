package routers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

// TestHealthRoute verifies the /health liveness endpoint responds with 200 and
// the agreed JSON shape using the default (non-ldflags) build metadata.
func TestHealthRoute(t *testing.T) {
	r := chi.NewRouter()
	attachHealthRoute(r)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.JSONEq(t,
		`{"status":"OK","version":"develop","tag":"0.0.1-rc","hash":"unknown"}`,
		w.Body.String())
}
