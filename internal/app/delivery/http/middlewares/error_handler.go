package middlewares

import (
	"errors"
	"net/http"

	"github.com/konsulin-care/konsulin-api/internal/pkg/utils"
)

func (m *Middlewares) ErrorHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				var err error
				switch x := rec.(type) {
				case string:
					err = errors.New(x)
				case error:
					err = x
				default:
					err = errors.New("unknown error")
				}

				utils.BuildErrorResponse(m.Log, w, err)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
