package middlewares

import (
	"context"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *responseRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

func (m *Middlewares) Logging(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rec, r)

			requestID := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY)
			isClientRequestID := r.Context().Value(constvars.CONTEXT_IS_CLIENT_REQUEST_ID_KEY)

			logger.Info("API request completed",
				zap.Int("status_code", rec.statusCode),
				zap.Any("request_id", requestID),
				zap.Any("is_client_request_id", isClientRequestID),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
				zap.Duration("response_time", time.Since(start)),
			)
		})
	}
}

func (m *Middlewares) RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(constvars.HeaderXRequestID)
		isClientRequestID := true

		if requestID == "" {
			requestID = utils.GenerateRequestID()
			isClientRequestID = false
		}

		ctx := context.WithValue(r.Context(), constvars.CONTEXT_REQUEST_ID_KEY, requestID)
		ctx = context.WithValue(ctx, constvars.CONTEXT_IS_CLIENT_REQUEST_ID_KEY, isClientRequestID)
		ctx = context.WithValue(ctx, constvars.CONTEXT_STEPS_KEY, 1)

		w.Header().Set(constvars.HeaderXRequestID, requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
