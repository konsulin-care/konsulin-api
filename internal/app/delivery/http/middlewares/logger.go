package middlewares

import (
	"konsulin-service/internal/app/config"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

func (m *Middlewares) RequestLogger(appConfig config.App, log *logrus.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			duration := time.Since(start)

			tz, err := time.LoadLocation(appConfig.Timezone)
			if err != nil {
				log.Printf("Invalid time zone: %v", err)
				tz = time.UTC
			}

			log.Printf(`{%s} | {%s} | {%s} ==> {%s} | {%s} | {%d}`, time.Now().In(tz).Format(time.RFC850), r.RemoteAddr, r.Method, r.RequestURI, duration)
		})
	}
}
