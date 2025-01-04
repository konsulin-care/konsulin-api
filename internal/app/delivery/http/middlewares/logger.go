package middlewares

// type responseRecorder struct {
// 	http.ResponseWriter
// 	statusCode int
// }

// func (rec *responseRecorder) WriteHeader(code int) {
// 	rec.statusCode = code
// 	rec.ResponseWriter.WriteHeader(code)
// }

// func (m *Middlewares) Logging(logger *zap.Logger) func(http.Handler) http.Handler {
// 	return func(next http.Handler) http.Handler {
// 		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 			start := time.Now()

// 			var requestBody []byte
// 			if r.Body != nil {
// 				bodyBytes, err := io.ReadAll(r.Body)
// 				if err == nil {
// 					requestBody = bodyBytes
// 				}
// 				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
// 			}

// 			rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}

// 			next.ServeHTTP(rec, r)

// 			logger.Info("API request",
// 				zap.String("method", r.Method),
// 				zap.String("url", r.URL.String()),
// 				zap.String("remote_addr", r.RemoteAddr),
// 				zap.String("user_agent", r.UserAgent()),
// 				zap.ByteString("request_body", requestBody),
// 				zap.Int("status_code", rec.statusCode),
// 				zap.Duration("duration", time.Since(start)),
// 			)
// 		})
// 	}
// }
