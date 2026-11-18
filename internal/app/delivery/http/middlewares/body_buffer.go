package middlewares

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/exceptions"
	"github.com/konsulin-care/konsulin-api/internal/pkg/utils"
)

// BodyBuffer reads the request body, stores the raw bytes in the context and
// replaces the request body with a new reader so it can be consumed again by
// subsequent middlewares or handlers.
func (m *Middlewares) BodyBuffer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrReadBody(err))
			return
		}

		ctx := context.WithValue(r.Context(), constvars.CONTEXT_RAW_BODY, bodyBytes)
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
